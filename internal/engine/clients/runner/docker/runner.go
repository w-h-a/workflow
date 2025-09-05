package docker

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"maps"
	"os"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
)

type dockerRunner struct {
	options runner.Options
	client  *client.Client
	sem     chan struct{}
	images  map[string]bool
	tasks   map[string]string
	mtx     sync.RWMutex
	exit    chan struct{}
	once    sync.Once
	wg      sync.WaitGroup
}

func (r *dockerRunner) Run(ctx context.Context, opts ...runner.RunOption) (string, error) {
	options := runner.NewRunOptions(opts...)

	// TODO: validate the options for docker

	if err := r.pullImage(ctx, options.Image); err != nil {
		return "", runner.ErrPullingImage
	}

	cc := container.Config{
		Image: options.Image,
		Cmd:   options.Cmd,
		Env:   options.Env,
	}

	var mounts []mount.Mount

	for _, m := range options.Mounts {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Source: m["source"],
			Target: m["target"],
		})
	}

	hc := container.HostConfig{
		PublishAllPorts: true,
		Mounts:          mounts,
	}

	// TODO: network support

	rsp, err := r.client.ContainerCreate(ctx, &cc, &hc, nil, nil, "")
	if err != nil {
		// span
		slog.ErrorContext(ctx, "failed to create container", "image", options.Image, "error", err)
		return "", err
	}

	defer func() {
		cleanupCtx := context.WithoutCancel(ctx)
		if err := r.remove(cleanupCtx, options.ID); err != nil {
			// span
			slog.ErrorContext(cleanupCtx, "failed to remove container", "containerID", rsp.ID, "error", err)
		}
	}()

	r.mtx.Lock()
	r.tasks[options.ID] = rsp.ID
	r.mtx.Unlock()

	if err := r.client.ContainerStart(ctx, rsp.ID, container.StartOptions{}); err != nil {
		return "", err
	}

	out, err := r.client.ContainerLogs(ctx, rsp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return "", err
	}

	defer out.Close()

	rPipe, wPipe := io.Pipe()

	go func() {
		defer wPipe.Close()
		stdcopy.StdCopy(wPipe, wPipe, out)
	}()

	go func() {
		scanner := bufio.NewScanner(rPipe)
		for scanner.Scan() {
			options.LogHandler(scanner.Text())
		}
	}()

	statusCh, errCh := r.client.ContainerWait(ctx, rsp.ID, container.WaitConditionNotRunning)

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case err := <-errCh:
		if err != nil {
			return "", err
		}
	case status := <-statusCh:
		if status.StatusCode != 0 {
			return "", runner.ErrBadExitCode
		}
		// span
		slog.InfoContext(ctx, "done waiting for container", "containerID", rsp.ID, "status", status.StatusCode)
	}

	return "Container finished", nil
}

func (r *dockerRunner) CreateVolume(ctx context.Context, opts ...runner.CreateVolumeOption) error {
	options := runner.NewCreateVolumeOptions(opts...)

	if _, err := r.client.VolumeCreate(ctx, volume.CreateOptions{Name: options.Name}); err != nil {
		return err
	}

	// span
	slog.InfoContext(ctx, "created volume", "name", options.Name)

	return nil
}

func (r *dockerRunner) DeleteVolume(ctx context.Context, opts ...runner.DeleteVolumeOption) error {
	options := runner.NewDeleteVolumeOptions(opts...)

	vs, err := r.client.VolumeList(ctx, volume.ListOptions{Filters: filters.NewArgs(filters.Arg("name", options.Name))})
	if err != nil {
		return err
	}

	if len(vs.Volumes) == 0 {
		return runner.ErrVolumeNotFound
	}

	if err := r.client.VolumeRemove(ctx, options.Name, true); err != nil {
		return err
	}

	// span
	slog.InfoContext(ctx, "removed volume", "name", options.Name)

	return nil
}

func (r *dockerRunner) CheckHealth(ctx context.Context) error {
	_, err := r.client.Ping(ctx)
	return err
}

func (r *dockerRunner) Close(ctx context.Context) error {
	done := make(chan struct{})

	r.once.Do(func() {
		slog.InfoContext(ctx, "starting graceful shutdown of docker client")

		close(r.exit)

		go func() {
			r.wg.Wait()

			toRemove := map[string]string{}

			r.mtx.RLock()
			maps.Copy(toRemove, r.tasks)
			r.mtx.RUnlock()

			for id, containerID := range toRemove {
				if err := r.remove(ctx, id); err != nil {
					slog.ErrorContext(ctx, "failed to remove container during close", "containerID", containerID, "error", err)
				}
			}

			close(r.sem)
			close(done)
		}()
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
	}

	slog.InfoContext(ctx, "graceful shutdown of docker client complete")

	return nil
}

func (r *dockerRunner) pullImage(ctx context.Context, tag string) error {
	r.mtx.RLock()
	if _, ok := r.images[tag]; ok {
		r.mtx.RUnlock()
		return nil
	}
	r.mtx.RUnlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.exit:
		return runner.ErrRunnerClosing
	case r.sem <- struct{}{}:
		r.wg.Add(1)
		defer func() {
			r.wg.Done()
			<-r.sem
		}()

		r.mtx.RLock()
		if _, ok := r.images[tag]; ok {
			r.mtx.RUnlock()
			return nil
		}
		r.mtx.RUnlock()

		images, err := r.client.ImageList(ctx, image.ListOptions{All: true})
		if err != nil {
			return err
		}

		for _, img := range images {
			for _, t := range img.RepoTags {
				if t == tag {
					r.mtx.Lock()
					r.images[t] = true
					r.mtx.Unlock()
					return nil
				}
			}
		}

		authConfig := registry.AuthConfig{
			Username: r.options.RegistryUser,
			Password: r.options.RegistryPass,
		}

		encoded, _ := json.Marshal(authConfig)
		registryAuth := base64.URLEncoding.EncodeToString(encoded)

		reader, err := r.client.ImagePull(ctx, tag, image.PullOptions{RegistryAuth: registryAuth})
		if err != nil {
			return err
		}

		defer reader.Close()

		if _, err := io.Copy(os.Stdout, reader); err != nil {
			return err
		}

		r.mtx.Lock()
		r.images[tag] = true
		r.mtx.Unlock()
	}

	return nil
}

func (r *dockerRunner) remove(ctx context.Context, id string) error {
	r.mtx.Lock()
	containerID, ok := r.tasks[id]
	if !ok {
		r.mtx.Unlock()
		return nil
	}
	delete(r.tasks, id)
	r.mtx.Unlock()

	// span
	slog.InfoContext(ctx, "attempting to remove container", "containerID", containerID)

	if err := r.client.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		// span
		slog.ErrorContext(ctx, "failed to stop container", "containerID", containerID, "error", err)
	}

	if err := r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{RemoveVolumes: false, RemoveLinks: false, Force: true}); err != nil {
		// span
		slog.ErrorContext(ctx, "failed to remove container", "containerID", containerID, "error", err)
		return err
	}

	// span
	slog.InfoContext(ctx, "successfully removed container", "containerID", containerID)

	return nil
}

func (r *dockerRunner) pruneImages() {
	r.wg.Add(1)
	defer r.wg.Done()

	ticker := time.NewTicker(r.options.PruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx := context.Background()
			r.prune(ctx)
		case <-r.exit:
			return
		}
	}
}

func (r *dockerRunner) prune(ctx context.Context) {
	filters := filters.NewArgs()

	filters.Add("until", "24h")

	pruneReport, err := r.client.ImagesPrune(ctx, filters)
	if err != nil {
		slog.ErrorContext(ctx, "failed to prune images", "error", err)
		return
	}

	slog.InfoContext(ctx, "pruning complete", "images-deleted", len(pruneReport.ImagesDeleted), "space-reclaimed", pruneReport.SpaceReclaimed)
}

func NewRunner(opts ...runner.Option) runner.Runner {
	options := runner.NewOptions(opts...)

	// TODO: validate options and use them in the next line
	c, err := client.NewClientWithOpts(client.WithHost(options.Host))
	if err != nil {
		detail := "failed to initialize docker runner client"
		slog.ErrorContext(context.Background(), detail, "error", err)
		panic(err)
	}

	if _, err := c.Ping(context.Background()); err != nil {
		detail := "failed to ping docker daemon"
		slog.ErrorContext(context.Background(), detail, "error", err)
		panic(err)
	}

	dr := &dockerRunner{
		options: options,
		client:  c,
		sem:     make(chan struct{}, 1),
		images:  map[string]bool{},
		tasks:   map[string]string{},
		mtx:     sync.RWMutex{},
		exit:    make(chan struct{}),
		once:    sync.Once{},
		wg:      sync.WaitGroup{},
	}

	go dr.pruneImages()

	return dr
}
