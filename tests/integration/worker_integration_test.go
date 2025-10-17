package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/w-h-a/workflow/api/task"
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
