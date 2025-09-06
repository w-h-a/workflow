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
	"github.com/w-h-a/workflow/internal/task"
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

	// Act
	time.Sleep(3 * time.Second)

	client := &http.Client{}

	req, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("http://localhost:4000/tasks/cancel/%s", scheduledTask.ID),
		nil,
	)
	require.NoError(t, err)

	rsp2, err := client.Do(req)
	require.NoError(t, err)
	defer rsp2.Body.Close()

	// Assert
	assert.Equal(t, http.StatusOK, rsp2.StatusCode)

	var cancelledTask task.Task
	err = json.NewDecoder(rsp2.Body).Decode(&cancelledTask)
	assert.NoError(t, err)

	assert.Equal(t, scheduledTask.ID, cancelledTask.ID)
	assert.Equal(t, task.Cancelled, cancelledTask.State)
	assert.NotNil(t, cancelledTask.CancelledAt)

	// Act
	rsp3, err := http.Get(
		fmt.Sprintf("http://localhost:4000/tasks/%s", cancelledTask.ID),
	)
	require.NoError(t, err)
	defer rsp3.Body.Close()

	// Assert
	assert.Equal(t, http.StatusOK, rsp3.StatusCode)

	var retrievedTask task.Task
	err = json.NewDecoder(rsp3.Body).Decode(&retrievedTask)
	assert.NoError(t, err)

	assert.Equal(t, task.Cancelled, retrievedTask.State)
}

func TestCoordinator_RestartTask_Success(t *testing.T) {
	// Arrange
	testData, err := os.ReadFile("../testdata/hello.json")
	require.NoError(t, err)
	require.NotNil(t, testData)

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

	// Act
	time.Sleep(3 * time.Second)

	client := &http.Client{}

	req, err := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("http://localhost:4000/tasks/restart/%s", scheduledTask.ID),
		nil,
	)
	require.NoError(t, err)

	rsp2, err := client.Do(req)
	require.NoError(t, err)
	defer rsp2.Body.Close()

	// Assert
	assert.Equal(t, http.StatusOK, rsp2.StatusCode)

	var restartedTask task.Task
	err = json.NewDecoder(rsp2.Body).Decode(&restartedTask)
	assert.NoError(t, err)

	assert.Equal(t, scheduledTask.ID, restartedTask.ID)
	assert.Equal(t, task.Scheduled, restartedTask.State)
	assert.NotNil(t, restartedTask.ScheduledAt)
	assert.Equal(t, "", restartedTask.Result)
	assert.Equal(t, "", restartedTask.Error)
	assert.Nil(t, restartedTask.StartedAt)
}
