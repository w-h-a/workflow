package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/w-h-a/workflow/api/task"
)

func TestCoordinator_ScheduleTask_Success(t *testing.T) {
	// Arrange
	testData, err := os.ReadFile("../testdata/hello.json")
	require.NoError(t, err)
	require.NotNil(t, testData)

	// Act
	rsp, err := http.Post(
		"http://localhost:4000/tasks",
		"application/json",
		bytes.NewBuffer(testData),
	)
	require.NoError(t, err)
	defer rsp.Body.Close()

	// Assert
	assert.Equal(t, http.StatusOK, rsp.StatusCode)

	var scheduledTask task.Task
	err = json.NewDecoder(rsp.Body).Decode(&scheduledTask)
	assert.NoError(t, err)

	assert.NotNil(t, scheduledTask.ID)
	assert.Equal(t, task.Scheduled, scheduledTask.State)
	assert.NotNil(t, scheduledTask.ScheduledAt)
}

func TestCoordinator_CancelTask_Success(t *testing.T) {
	// Arrange
	testData, err := os.ReadFile("../testdata/convert.json")
	require.NoError(t, err)
	require.NotNil(t, testData)

	// Act
	rsp1, err := http.Post(
		"http://localhost:4000/tasks",
		"application/json",
		bytes.NewBuffer(testData),
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
	}, 10*time.Second, 500*time.Millisecond, "timed out waiting for task to start")

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
	}, 10*time.Second, 500*time.Millisecond, "timed out waiting for task to cancel")
}

func TestCoordinator_RestartTask_Success(t *testing.T) {
	// Arrange
	testTask := &task.Task{
		Image: "ubuntu:mantic",
		Cmd:   []string{"sleep", "5"},
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

	// Act
	req, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("http://localhost:4000/tasks/restart/%s", scheduledTask.ID),
		nil,
	)
	require.NoError(t, err)

	rsp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer rsp2.Body.Close()

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
	}, 10*time.Second, 500*time.Millisecond, "timed out waiting for task to start")

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
	}, 30*time.Second, 1*time.Second, "timed out waiting for task to complete again")
}
