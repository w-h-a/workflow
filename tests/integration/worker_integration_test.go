package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/w-h-a/workflow/internal/task"
)

func TestWorker_TaskExecution_EndToEnd(t *testing.T) {
	// Arrange
	testTask := &task.Task{
		Image: "ubuntu:mantic",
		Cmd:   []string{"echo", "integration-test-output"},
	}

	bs, _ := json.Marshal(testTask)

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://localhost:4000/tasks",
		bytes.NewBuffer(bs),
	)
	require.NoError(t, err)

	client := &http.Client{Timeout: 10 * time.Second}

	// Act
	rsp, err := client.Do(req)
	require.NoError(t, err)
	defer rsp.Body.Close()

	var scheduledTask task.Task
	err = json.NewDecoder(rsp.Body).Decode(&scheduledTask)
	require.NoError(t, err)

	// Assert
	var completedTask task.Task

	timeout := time.After(30 * time.Second)

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for task to complete")
		case <-ticker.C:
			rsp, _ := http.Get(
				fmt.Sprintf("http://localhost:4000/tasks/%s", scheduledTask.ID),
			)
			defer rsp.Body.Close()
			json.NewDecoder(rsp.Body).Decode(&completedTask)
			if completedTask.State == task.Completed {
				assert.Equal(t, "Container finished", completedTask.Result)
				return
			}
			if completedTask.State == task.Failed {
				t.Fatalf("task failed unexpectedly: %s", completedTask.Error)
			}
		}
	}
}
