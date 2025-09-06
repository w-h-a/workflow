package integration

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if len(os.Getenv("INTEGRATION")) == 0 {
		os.Exit(0)
	}

	if err := setup(); err != nil {
		slog.Error("error during integration tests", "error", err)
		teardown()
		os.Exit(1)
	}

	code := m.Run()

	teardown()

	os.Exit(code)
}

func setup() error {
	fmt.Println("Starting services with docker compose...")

	cmd := exec.Command("docker", "compose", "up", "--build", "-d")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		slog.Error("failed to start docker compose", "error", err)
		return fmt.Errorf("failed to start docker compose: %w", err)
	}

	fmt.Println("Waiting for services to become healthy...")

	if err := waitForServices(); err != nil {
		return err
	}

	fmt.Println("Waiting for migrations to complete...")

	cmd = exec.Command("go", "run", "main.go", "migration", "--location", "postgres://tasks:tasks@localhost:5432/tasks?sslmode=disable")

	cmd.Dir = "../.."
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		slog.Error("failed to run migrations", "error", err)
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	fmt.Println(" - Waiting for migrations... [OK]")

	return nil
}

func waitForServices() error {
	services := []string{"prometheus", "jaeger", "postgres", "rabbitmq", "streamer", "worker", "coordinator"}

	timeout := 120 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, service := range services {
		fmt.Printf(" - Waiting for %s...", service)

		if err := waitForService(ctx, service); err != nil {
			slog.Error("failed to start service", "service", service, "error", err)
			return err
		}

		fmt.Printf(" [OK]\n")
	}

	return nil
}

func waitForService(ctx context.Context, service string) error {
	check := exec.CommandContext(ctx, "docker", "compose", "ps", "-q", service)

	out, err := check.Output()
	if err != nil {
		return fmt.Errorf("failed to get service container ID: %w", err)
	}

	containerID := strings.TrimSpace(string(out))

	if len(containerID) == 0 {
		return errors.New("container ID not found")
	}

	for {
		cmd := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Health.Status}}", containerID)

		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to inspect container health: %w", err)
		}

		status := strings.TrimSpace(string(out))

		if status == "healthy" {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):

		}
	}
}

func teardown() {
	fmt.Println("Tearing down services with docker compose")

	cmd := exec.Command("docker", "compose", "down")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		slog.Error("failed to tear down docker compose", "error", err)
	}
}
