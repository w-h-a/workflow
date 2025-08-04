package docker

import (
	"context"
	"io"
	"log/slog"
	"os"
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

func (r *dockerRunner) Start(ctx context.Context, opts ...runner.StartOption) error {
	options := runner.NewStartOptions(opts...)

	// TODO: validate the options for docker

	reader, err := r.client.ImagePull(ctx, options.Image, image.PullOptions{})
	if err != nil {
		// span
		slog.ErrorContext(ctx, "failed to pull image", "image", options.Image, "error", err)
		return err
	}

	if _, err := io.Copy(os.Stdout, reader); err != nil {
		return err
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
		return err
	}

	if err := r.client.ContainerStart(ctx, rsp.ID, container.StartOptions{}); err != nil {
		return err
	}

	out, err := r.client.ContainerLogs(ctx, rsp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return err
	}

	if _, err := stdcopy.StdCopy(os.Stdout, os.Stderr, out); err != nil {
		return err
	}

	r.mtx.Lock()
	r.tasks[options.ID] = rsp.ID
	r.mtx.Unlock()

	return nil
}

func (r *dockerRunner) Stop(ctx context.Context, opts ...runner.StopOption) error {
	options := runner.NewStopOptions(opts...)

	r.mtx.RLock()
	containerID, ok := r.tasks[options.ID]
	r.mtx.RUnlock()

	if !ok {
		return nil
	}

	// span
	slog.InfoContext(ctx, "attempting to stop container", "containerID", containerID)

	if err := r.client.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		// span
		return err
	}

	if err := r.client.ContainerRemove(ctx, containerID, container.RemoveOptions{RemoveVolumes: true, RemoveLinks: false, Force: false}); err != nil {
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
