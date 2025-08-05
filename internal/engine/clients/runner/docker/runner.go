package docker

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/w-h-a/workflow/internal/engine/clients/runner"
)

type dockerRunner struct {
	options runner.Options
	client  *client.Client
	tasks   map[string]string
	mtx     sync.RWMutex
}

func (r *dockerRunner) Run(ctx context.Context, opts ...runner.RunOption) (string, error) {
	options := runner.NewRunOptions(opts...)

	// TODO: validate the options for docker

	reader, err := r.client.ImagePull(ctx, options.Image, image.PullOptions{})
	if err != nil {
		// span
		slog.ErrorContext(ctx, "failed to pull image", "image", options.Image, "error", err)
		return "", err
	}

	if _, err := io.Copy(os.Stdout, reader); err != nil {
		return "", err
	}

	cc := container.Config{
		Image: options.Image,
		Cmd:   options.Cmd,
		Env:   options.Env,
	}

	rp := container.RestartPolicy{
		Name: container.RestartPolicyMode(options.RestartPolicy),
	}

	rs := container.Resources{
		Memory: options.Memory,
	}

	hc := container.HostConfig{
		RestartPolicy:   rp,
		Resources:       rs,
		PublishAllPorts: true,
	}

	rsp, err := r.client.ContainerCreate(ctx, &cc, &hc, nil, nil, options.ID)
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

	out, err := r.client.ContainerLogs(ctx, rsp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", err
	}

	defer func() {
		if err := out.Close(); err != nil {
			// span
			slog.ErrorContext(ctx, "failed to close stdout on container", "containerID", rsp.ID)
		}
	}()

	lr := &io.LimitedReader{R: out, N: 1024}
	buf := &strings.Builder{}

	if _, err := stdcopy.StdCopy(buf, buf, lr); err != nil {
		return "", err
	}

	statusCh, errCh := r.client.ContainerWait(ctx, rsp.ID, container.WaitConditionNotRunning)

	select {
	case err := <-errCh:
		if err != nil {
			return "", err
		}
	case status := <-statusCh:
		// span
		slog.InfoContext(ctx, "done waiting for container", "containerID", rsp.ID, "status", status.StatusCode)
	}

	return buf.String(), nil
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

	if err := r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{RemoveVolumes: true, RemoveLinks: false, Force: true}); err != nil {
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

	dr := &dockerRunner{
		options: options,
		client:  c,
		tasks:   map[string]string{},
		mtx:     sync.RWMutex{},
	}

	return dr
}
