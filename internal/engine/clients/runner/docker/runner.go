package docker

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
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
	ctx     context.Context
	cancel  context.CancelFunc
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

	for _, v := range options.Volumes {
		vol := strings.Split(v, ":")
		if len(vol) != 2 {
			return "", runner.ErrInvalidVolumeName
		}
		mount := mount.Mount{
			Type:   mount.TypeVolume,
			Source: vol[0],
			Target: vol[1],
		}
		mounts = append(mounts, mount)
	}

	hc := container.HostConfig{
		PublishAllPorts: true,
		Mounts:          mounts,
	}

	rsp, err := r.client.ContainerCreate(ctx, &cc, &hc, nil, nil, "")
	if err != nil {
		// span
		slog.ErrorContext(ctx, "failed to create container", "image", options.Image, "error", err)
		return "", err
	}

	defer func() {
		if err := r.remove(ctx, options.ID); err != nil {
			// span
			slog.ErrorContext(ctx, "failed to remove container", "containerID", rsp.ID, "error", err)
		}
	}()

	r.mtx.Lock()
	r.tasks[options.ID] = rsp.ID
	r.mtx.Unlock()

	if err := r.client.ContainerStart(ctx, rsp.ID, container.StartOptions{}); err != nil {
		return "", err
	}

	out, err := r.client.ContainerLogs(ctx, rsp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return "", err
	}

	defer func() {
		if err := out.Close(); err != nil {
			// span
			slog.ErrorContext(ctx, "failed to close stdout on container", "containerID", rsp.ID)
		}
	}()

	lr := &io.LimitedReader{R: out, N: 4096}
	buf := &strings.Builder{}
	multiWriter := io.MultiWriter(os.Stdout, buf)

	if _, err := stdcopy.StdCopy(multiWriter, multiWriter, lr); err != nil {
		return "", err
	}

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

	return buf.String(), nil
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

func (r *dockerRunner) Close() {
	r.cancel()
	r.wg.Wait()
	close(r.sem)
}

func (r *dockerRunner) pullImage(ctx context.Context, tag string) error {
	if r.ctx.Err() != nil {
		return r.ctx.Err()
	}

	r.mtx.RLock()
	if _, ok := r.images[tag]; ok {
		r.mtx.RUnlock()
		return nil
	}
	r.mtx.RUnlock()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-r.ctx.Done():
		return r.ctx.Err()
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

		reader, err := r.client.ImagePull(ctx, tag, image.PullOptions{})
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

	if err := r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{RemoveVolumes: false, RemoveLinks: false, Force: true}); err != nil {
		// span
		return err
	}

	return nil
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

	ctx, cancel := context.WithCancel(context.Background())

	dr := &dockerRunner{
		options: options,
		client:  c,
		sem:     make(chan struct{}, 1),
		images:  map[string]bool{},
		tasks:   map[string]string{},
		mtx:     sync.RWMutex{},
		ctx:     ctx,
		cancel:  cancel,
		wg:      sync.WaitGroup{},
	}

	return dr
}
