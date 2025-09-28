package unit

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	memorybroker "github.com/w-h-a/workflow/internal/engine/clients/broker/memory"
	"github.com/w-h-a/workflow/internal/engine/clients/notifier/local"
	memorystore "github.com/w-h-a/workflow/internal/engine/clients/readwriter/memory"
	"github.com/w-h-a/workflow/internal/engine/services/coordinator"
	"github.com/w-h-a/workflow/internal/task"
)

func TestCoordinator_ScheduleTask_Success(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memorybroker.NewBroker()
	memoryStore := memorystore.NewReadWriter()
	localNotifier := local.NewNotifier()

	c := coordinator.New(memoryBroker, memoryStore, localNotifier, map[string]int{})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Start(stop)
	}()

	testTask := &task.Task{
		Image: "ubuntu:latest",
		Cmd:   []string{"echo", "hello world"},
	}

	// Action
	scheduledTask, err := c.ScheduleTask(context.Background(), testTask)
	require.NoError(t, err)
	require.NotNil(t, scheduledTask)

	// Assert
	bs, err := memoryStore.ReadById(context.Background(), scheduledTask.ID)
	assert.NoError(t, err)
	assert.NotNil(t, bs)

	persistedTask, _ := task.Factory(bs)
	assert.Equal(t, task.Scheduled, persistedTask.State)

	q := memoryBroker.Queue(string(task.Scheduled))

	select {
	case msg := <-q:
		publishedTask, _ := task.Factory(msg)
		assert.Equal(t, scheduledTask.ID, publishedTask.ID)
		assert.Equal(t, task.Scheduled, publishedTask.State)
		assert.NotEmpty(t, publishedTask.ScheduledAt)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for message")
	}

	assert.NotEmpty(t, scheduledTask.ID)
	assert.Equal(t, task.Scheduled, scheduledTask.State)
	assert.NotEmpty(t, scheduledTask.ScheduledAt)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestCoordinator_CancelTask_Success(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memorybroker.NewBroker()
	memoryStore := memorystore.NewReadWriter()
	localNotifier := local.NewNotifier()

	c := coordinator.New(memoryBroker, memoryStore, localNotifier, map[string]int{})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Start(stop)
	}()

	testTask := &task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Started,
		Image: "ubuntu:latest",
		Cmd:   []string{"echo", "hello world"},
	}

	taskBs, _ := json.Marshal(testTask)

	err := memoryStore.Write(context.Background(), testTask.ID, taskBs)
	require.NoError(t, err)

	// Action
	cancelledTask, err := c.CancelTask(context.Background(), testTask.ID)
	require.NoError(t, err)
	require.NotNil(t, cancelledTask)

	// Assert
	bs, err := memoryStore.ReadById(context.Background(), cancelledTask.ID)
	assert.NoError(t, err)
	assert.NotNil(t, bs)

	persistedTask, _ := task.Factory(bs)
	assert.Equal(t, task.Cancelled, persistedTask.State)

	q := memoryBroker.Queue(string(task.Cancelled))

	select {
	case msg := <-q:
		publishedTask, _ := task.Factory(msg)
		assert.Equal(t, cancelledTask.ID, publishedTask.ID)
		assert.Equal(t, task.Cancelled, publishedTask.State)
		assert.NotEmpty(t, publishedTask.CancelledAt)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for message")
	}

	assert.Equal(t, task.Cancelled, cancelledTask.State)
	assert.NotEmpty(t, cancelledTask.CancelledAt)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestCoordinator_CancelTask_NotCancellable(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memorybroker.NewBroker()
	memoryStore := memorystore.NewReadWriter()
	localNotifier := local.NewNotifier()

	c := coordinator.New(memoryBroker, memoryStore, localNotifier, map[string]int{})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Start(stop)
	}()

	testTask := &task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Failed,
		Image: "ubuntu:latest",
		Cmd:   []string{"echo", "hello world"},
	}

	taskBs, _ := json.Marshal(testTask)

	err := memoryStore.Write(context.Background(), testTask.ID, taskBs)
	require.NoError(t, err)

	// Action
	cancelledTask, err := c.CancelTask(context.Background(), testTask.ID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, cancelledTask)
	assert.Equal(t, task.ErrTaskNotCancellable, err)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestCoordinator_RestartTask_Success(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memorybroker.NewBroker()
	memoryStore := memorystore.NewReadWriter()
	localNotifier := local.NewNotifier()

	c := coordinator.New(memoryBroker, memoryStore, localNotifier, map[string]int{})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Start(stop)
	}()

	testTask := &task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Completed,
		Image: "ubuntu:latest",
		Cmd:   []string{"echo", "hello world"},
	}

	taskBs, _ := json.Marshal(testTask)

	err := memoryStore.Write(context.Background(), testTask.ID, taskBs)
	require.NoError(t, err)

	// Action
	restartedTask, err := c.RestartTask(context.Background(), testTask.ID)
	require.NoError(t, err)
	require.NotNil(t, restartedTask)

	// Assert
	bs, err := memoryStore.ReadById(context.Background(), restartedTask.ID)
	assert.NoError(t, err)
	assert.NotNil(t, bs)

	persistedTask, _ := task.Factory(bs)
	assert.Equal(t, task.Scheduled, persistedTask.State)

	q := memoryBroker.Queue(string(task.Scheduled))

	select {
	case msg := <-q:
		publishedTask, _ := task.Factory(msg)
		assert.Equal(t, restartedTask.ID, publishedTask.ID)
		assert.Equal(t, task.Scheduled, publishedTask.State)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for message")
	}

	assert.Equal(t, task.Scheduled, restartedTask.State)
	assert.NotEmpty(t, restartedTask.ScheduledAt)
	assert.Empty(t, restartedTask.Result)
	assert.Empty(t, restartedTask.Error)
	assert.Empty(t, restartedTask.CompletedAt)
	assert.Empty(t, restartedTask.FailedAt)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestCoordinator_RestartTask_NotRestartable(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memorybroker.NewBroker()
	memoryStore := memorystore.NewReadWriter()
	localNotifier := local.NewNotifier()

	c := coordinator.New(memoryBroker, memoryStore, localNotifier, map[string]int{})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Start(stop)
	}()

	testTask := &task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Scheduled,
		Image: "ubuntu:latest",
		Cmd:   []string{"echo", "hello world"},
	}

	taskBs, _ := json.Marshal(testTask)

	err := memoryStore.Write(context.Background(), testTask.ID, taskBs)
	require.NoError(t, err)

	// Action
	restartedTask, err := c.RestartTask(context.Background(), testTask.ID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, restartedTask)
	assert.Equal(t, task.ErrTaskNotRestartable, err)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestCoordinator_RetrieveTasks_Success(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memorybroker.NewBroker()
	memoryStore := memorystore.NewReadWriter()
	localNotifier := local.NewNotifier()

	c := coordinator.New(memoryBroker, memoryStore, localNotifier, map[string]int{})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Start(stop)
	}()

	testTasks := []*task.Task{
		{
			ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
			State: task.Scheduled,
			Image: "ubuntu:latest",
			Cmd:   []string{"echo", "hello world"},
		},
		{
			ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
			State: task.Completed,
			Image: "ubuntu:latest",
			Cmd:   []string{"echo", "hello world"},
		},
		{
			ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
			State: task.Failed,
			Image: "ubuntu:latest",
			Cmd:   []string{"echo", "hello world"},
		},
	}

	for _, testTask := range testTasks {
		bs, _ := json.Marshal(testTask)
		err := memoryStore.Write(context.Background(), testTask.ID, bs)
		require.NoError(t, err)
	}

	// Action
	tasks, err := c.RetrieveTasks(context.Background(), 1, 10)
	require.NoError(t, err)
	require.NotNil(t, tasks)

	// Assert
	assert.Equal(t, 3, tasks.TotalTasks)
	assert.Equal(t, 3, len(tasks.Tasks))

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestCoordinator_RetrieveTask_Success(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memorybroker.NewBroker()
	memoryStore := memorystore.NewReadWriter()
	localNotifier := local.NewNotifier()

	c := coordinator.New(memoryBroker, memoryStore, localNotifier, map[string]int{})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Start(stop)
	}()

	testTask := &task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Scheduled,
		Image: "ubuntu:latest",
		Cmd:   []string{"echo", "hello world"},
	}

	bs, _ := json.Marshal(testTask)
	err := memoryStore.Write(context.Background(), testTask.ID, bs)
	require.NoError(t, err)

	// Action
	retrievedTask, err := c.RetrieveTask(context.Background(), testTask.ID)
	require.NoError(t, err)
	require.NotNil(t, retrievedTask)

	// Assert
	assert.Equal(t, testTask.ID, retrievedTask.ID)
	assert.Equal(t, testTask.State, retrievedTask.State)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestCoordinator_RetrieveTask_NotFound(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memorybroker.NewBroker()
	memoryStore := memorystore.NewReadWriter()
	localNotifier := local.NewNotifier()

	c := coordinator.New(memoryBroker, memoryStore, localNotifier, map[string]int{})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Start(stop)
	}()

	nonExistentID := strings.ReplaceAll(uuid.NewString(), "-", "")

	// Action
	retrievedTask, err := c.RetrieveTask(context.Background(), nonExistentID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, retrievedTask)
	assert.Equal(t, task.ErrTaskNotFound, err)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestCoordinator_HandleTask_StartedState(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memorybroker.NewBroker()
	memoryStore := memorystore.NewReadWriter()
	localNotifier := local.NewNotifier()

	c := coordinator.New(memoryBroker, memoryStore, localNotifier, map[string]int{
		string(task.Started):   1,
		string(task.Completed): 1,
		string(task.Failed):    1,
	})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Start(stop)
	}()

	testTask := &task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Scheduled,
		Image: "ubuntu:latest",
	}

	bs, _ := json.Marshal(testTask)
	err := memoryStore.Write(context.Background(), testTask.ID, bs)
	require.NoError(t, err)

	// Action
	startedTask := &task.Task{
		ID:    testTask.ID,
		State: task.Started,
		Image: "ubuntu:latest",
	}

	bs, _ = json.Marshal(startedTask)
	err = memoryBroker.Publish(context.Background(), bs, broker.PublishWithQueue(string(task.Started)))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Assert
	bs, err = memoryStore.ReadById(context.Background(), testTask.ID)
	assert.NoError(t, err)
	assert.NotNil(t, bs)

	persistedTask, _ := task.Factory(bs)
	assert.Equal(t, task.Started, persistedTask.State)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestCoordinator_HandleTask_CompletedState(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memorybroker.NewBroker()
	memoryStore := memorystore.NewReadWriter()
	localNotifier := local.NewNotifier()

	c := coordinator.New(memoryBroker, memoryStore, localNotifier, map[string]int{
		string(task.Started):   1,
		string(task.Completed): 1,
		string(task.Failed):    1,
	})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Start(stop)
	}()

	testTask := &task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Started,
		Image: "ubuntu:latest",
	}

	bs, _ := json.Marshal(testTask)
	err := memoryStore.Write(context.Background(), testTask.ID, bs)
	require.NoError(t, err)

	// Action
	completedTask := &task.Task{
		ID:     testTask.ID,
		State:  task.Completed,
		Image:  "ubuntu:latest",
		Result: "success",
	}

	bs, _ = json.Marshal(completedTask)
	err = memoryBroker.Publish(context.Background(), bs, broker.PublishWithQueue(string(task.Completed)))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Assert
	bs, err = memoryStore.ReadById(context.Background(), testTask.ID)
	assert.NoError(t, err)
	assert.NotNil(t, bs)

	persistedTask, _ := task.Factory(bs)
	assert.Equal(t, task.Completed, persistedTask.State)
	assert.Equal(t, "success", persistedTask.Result)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestCoordinator_HandleTask_FailedState(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memorybroker.NewBroker()
	memoryStore := memorystore.NewReadWriter()
	localNotifier := local.NewNotifier()

	c := coordinator.New(memoryBroker, memoryStore, localNotifier, map[string]int{
		string(task.Started):   1,
		string(task.Completed): 1,
		string(task.Failed):    1,
	})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Start(stop)
	}()

	testTask := &task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Started,
		Image: "ubuntu:latest",
	}

	bs, _ := json.Marshal(testTask)
	err := memoryStore.Write(context.Background(), testTask.ID, bs)
	require.NoError(t, err)

	// Action
	failedTask := &task.Task{
		ID:    testTask.ID,
		State: task.Failed,
		Image: "ubuntu:latest",
		Error: "failed with error",
	}

	bs, _ = json.Marshal(failedTask)
	err = memoryBroker.Publish(context.Background(), bs, broker.PublishWithQueue(string(task.Failed)))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// Assert
	bs, err = memoryStore.ReadById(context.Background(), testTask.ID)
	assert.NoError(t, err)
	assert.NotNil(t, bs)

	persistedTask, _ := task.Factory(bs)
	assert.Equal(t, task.Failed, persistedTask.State)
	assert.Equal(t, "failed with error", persistedTask.Error)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}
