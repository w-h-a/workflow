package unit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/w-h-a/workflow/api/task"
	"github.com/w-h-a/workflow/internal/engine/clients/broker"
	"github.com/w-h-a/workflow/internal/engine/clients/broker/memory"
	mockrunner "github.com/w-h-a/workflow/internal/engine/clients/runner/mock"
	"github.com/w-h-a/workflow/internal/engine/services/worker"
)

func TestWorker_HandleTask_Scheduled_Success(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	mockRunner := mockrunner.NewRunner()
	memoryBroker := memory.NewBroker()

	w := worker.New(mockRunner, memoryBroker, map[string]int{string(task.Scheduled): 1})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Start(stop)
	}()

	testTask := &task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Scheduled,
		Image: "ubuntu:latest",
	}

	taskBs, _ := json.Marshal(testTask)

	mockRunner.On("Run", mock.Anything, mock.Anything).Return("Container finished", nil)

	mockRunner.On("Close", mock.Anything).Return(nil)

	// Act
	err := memoryBroker.Publish(context.Background(), taskBs, broker.PublishWithQueue(string(task.Scheduled)))
	require.NoError(t, err)

	// Assert
	startedQueue := memoryBroker.Queue(string(task.Started))
	completedQueue := memoryBroker.Queue(string(task.Completed))

	select {
	case msg := <-startedQueue:
		startedTask, _ := task.Factory(msg)
		assert.Equal(t, task.Started, startedTask.State)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for started message")
	}

	select {
	case msg := <-completedQueue:
		completedTask, _ := task.Factory(msg)
		assert.Equal(t, task.Completed, completedTask.State)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for completed message")
	}

	mockRunner.AssertCalled(t, "Run", mock.Anything, mock.Anything)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestWorker_HandleTask_Scheduled_Failure(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	mockRunner := mockrunner.NewRunner()
	memoryBroker := memory.NewBroker()

	w := worker.New(mockRunner, memoryBroker, map[string]int{string(task.Scheduled): 1})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Start(stop)
	}()

	testTask := &task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Scheduled,
		Image: "ubuntu:latest",
	}

	taskBs, _ := json.Marshal(testTask)

	mockRunner.On("Run", mock.Anything, mock.Anything).Return("", errors.New("container exited with non-zero status"))

	mockRunner.On("Close", mock.Anything).Return(nil)

	// Act
	err := memoryBroker.Publish(context.Background(), taskBs, broker.PublishWithQueue(string(task.Scheduled)))
	require.NoError(t, err)

	// Assert
	startedQueue := memoryBroker.Queue(string(task.Started))
	failedQueue := memoryBroker.Queue(string(task.Failed))

	select {
	case msg := <-startedQueue:
		startedTask, _ := task.Factory(msg)
		assert.Equal(t, task.Started, startedTask.State)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for started message")
	}

	select {
	case msg := <-failedQueue:
		failedTask, _ := task.Factory(msg)
		assert.Equal(t, task.Failed, failedTask.State)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for failed message")
	}

	mockRunner.AssertCalled(t, "Run", mock.Anything, mock.Anything)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestWorker_HandleTask_Cancellation(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	mockRunner := mockrunner.NewRunner()
	memoryBroker := memory.NewBroker()

	w := worker.New(mockRunner, memoryBroker, map[string]int{
		string(task.Scheduled): 1,
		string(task.Cancelled): 1,
	})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Start(stop)
	}()

	testTask := &task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Scheduled,
		Image: "ubuntu:latest",
	}

	taskBs, _ := json.Marshal(testTask)

	cancelledTask := &task.Task{
		ID:    testTask.ID,
		State: task.Cancelled,
		Image: testTask.Image,
	}

	cancelledBs, _ := json.Marshal(cancelledTask)

	runChan := make(chan struct{})

	mockRunner.On("Run", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		<-runChan
	}).Return("Container cancelled", errors.New("context cancelled"))

	mockRunner.On("Close", mock.Anything).Return(nil)

	// Act
	err := memoryBroker.Publish(context.Background(), taskBs, broker.PublishWithQueue(string(task.Scheduled)))
	require.NoError(t, err)

	// Assert
	startedQueue := memoryBroker.Queue(string(task.Started))

	select {
	case msg := <-startedQueue:
		startedTask, _ := task.Factory(msg)
		assert.Equal(t, task.Started, startedTask.State)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for started message")
	}

	// Act
	err = memoryBroker.Publish(context.Background(), cancelledBs, broker.PublishWithQueue(string(task.Cancelled)))
	require.NoError(t, err)

	close(runChan)

	// Assert
	failedQueue := memoryBroker.Queue(string(task.Failed))

	select {
	case msg := <-failedQueue:
		failedTask, _ := task.Factory(msg)
		assert.Equal(t, task.Failed, failedTask.State)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for failed message")
	}

	mockRunner.AssertCalled(t, "Run", mock.Anything, mock.Anything)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestWorker_HandleTask_VolumeCreationFailure(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	mockRunner := mockrunner.NewRunner()
	memoryBroker := memory.NewBroker()

	w := worker.New(mockRunner, memoryBroker, map[string]int{string(task.Scheduled): 1})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Start(stop)
	}()

	testTask := &task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Scheduled,
		Image: "ubuntu:latest",
		Mounts: []*task.Mount{
			{
				Target: "/datums",
			},
		},
	}

	taskBs, _ := json.Marshal(testTask)

	mockRunner.On("Run", mock.Anything, mock.Anything).Return("", nil)

	mockRunner.On("CreateVolume", mock.Anything, mock.Anything).Return(errors.New("volume creation failure"))

	mockRunner.On("Close", mock.Anything).Return(nil)

	// Act
	err := memoryBroker.Publish(context.Background(), taskBs, broker.PublishWithQueue(string(task.Scheduled)))
	require.NoError(t, err)

	// Assert
	startedQueue := memoryBroker.Queue(string(task.Started))
	failedQueue := memoryBroker.Queue(string(task.Failed))

	select {
	case msg := <-startedQueue:
		startedTask, _ := task.Factory(msg)
		assert.Equal(t, task.Started, startedTask.State)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for started message")
	}

	select {
	case msg := <-failedQueue:
		failedTask, _ := task.Factory(msg)
		assert.Equal(t, task.Failed, failedTask.State)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for failed message")
	}

	mockRunner.AssertCalled(t, "CreateVolume", mock.Anything, mock.Anything)

	mockRunner.AssertNotCalled(t, "Run", mock.Anything, mock.Anything)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}

func TestWorker_HandleTask_PreTaskFailure(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	mockRunner := mockrunner.NewRunner()
	memoryBroker := memory.NewBroker()

	w := worker.New(mockRunner, memoryBroker, map[string]int{string(task.Scheduled): 1})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.Start(stop)
	}()

	testTask := &task.Task{
		ID:    strings.ReplaceAll(uuid.NewString(), "-", ""),
		State: task.Scheduled,
		Image: "ubuntu:latest",
		Pre: []*task.Task{
			{
				Image: "pre-task-image",
			},
		},
	}

	taskBs, _ := json.Marshal(testTask)

	mockRunner.On("Run", mock.Anything, mock.Anything).Return("", errors.New("pre-task failure"))

	mockRunner.On("Close", mock.Anything).Return(nil)

	// Act
	err := memoryBroker.Publish(context.Background(), taskBs, broker.PublishWithQueue(string(task.Scheduled)))
	require.NoError(t, err)

	// Assert
	startedQueue := memoryBroker.Queue(string(task.Started))
	failedQueue := memoryBroker.Queue(string(task.Failed))

	select {
	case msg := <-startedQueue:
		startedTask, _ := task.Factory(msg)
		assert.Equal(t, task.Started, startedTask.State)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for started message")
	}

	select {
	case msg := <-failedQueue:
		failedTask, _ := task.Factory(msg)
		assert.Equal(t, task.Failed, failedTask.State)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for failed message")
	}

	mockRunner.AssertCalled(t, "Run", mock.Anything, mock.Anything)

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}
