package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

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

	// Act
	rsp, err := http.Post(
		"http://localhost:4000/tasks",
		"application/json",
		bytes.NewBuffer(bs),
	)
	require.NoError(t, err)
	defer rsp.Body.Close()

	var scheduledTask task.Task
	err = json.NewDecoder(rsp.Body).Decode(&scheduledTask)
	require.NoError(t, err)

	// Assert
	require.Eventually(t, func() bool {
		var completedTask task.Task
		rsp, err := http.Get(
			fmt.Sprintf("http://localhost:4000/tasks/%s", scheduledTask.ID),
		)
		if err != nil {
			return false
		}
		defer rsp.Body.Close()
		if err := json.NewDecoder(rsp.Body).Decode(&completedTask); err != nil {
			return false
		}
		return completedTask.State == task.Completed
	}, 30*time.Second, 1*time.Second, "timed out waiting for task to complete")
}

func TestWorker_CancelTask_EndToEnd(t *testing.T) {
	// Arrange
	testTask := &task.Task{
		Image: "ubuntu:latest",
		Cmd:   []string{"sleep", "30"},
	}

	bs, _ := json.Marshal(testTask)

	// Act
	rsp1, err := http.Post(
		"http://localhost:4000/tasks",
		"application/json",
		bytes.NewBuffer(bs),
	)
	require.NoError(t, err)
	defer rsp1.Body.Close()

	var scheduledTask task.Task
	err = json.NewDecoder(rsp1.Body).Decode(&scheduledTask)
	require.NoError(t, err)

	// Assert
	require.Eventually(t, func() bool {
		var startedTask task.Task
		rsp, err := http.Get(
			fmt.Sprintf("http://localhost:4000/tasks/%s", scheduledTask.ID),
		)
		if err != nil {
			return false
		}
		defer rsp.Body.Close()
		if err := json.NewDecoder(rsp.Body).Decode(&startedTask); err != nil {
			return false
		}
		return startedTask.State == task.Started
	}, 30*time.Second, 1*time.Second, "timed out waiting for task to start")

	// Act
	req, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("http://localhost:4000/tasks/cancel/%s", scheduledTask.ID),
		nil,
	)
	require.NoError(t, err)

	rsp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer rsp2.Body.Close()

	// Assert
	require.Eventually(t, func() bool {
		var cancelledTask task.Task
		rsp, err := http.Get(
			fmt.Sprintf("http://localhost:4000/tasks/%s", scheduledTask.ID),
		)
		if err != nil {
			return false
		}
		defer rsp.Body.Close()
		if err := json.NewDecoder(rsp.Body).Decode(&cancelledTask); err != nil {
			return false
		}
		return cancelledTask.State == task.Failed
	}, 30*time.Second, 1*time.Second, "timed out waiting for task to cancel")
}
