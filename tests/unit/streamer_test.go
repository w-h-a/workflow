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
	"github.com/w-h-a/workflow/internal/engine/clients/broker/memory"
	"github.com/w-h-a/workflow/internal/engine/services/streamer"
	"github.com/w-h-a/workflow/internal/log"
)

func TestStreamer_StreamLogs_Success(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memory.NewBroker()

	s := streamer.New(memoryBroker, map[string]int{string(log.Queue): 1})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Start(stop)
	}()

	// Act
	taskID := strings.ReplaceAll(uuid.NewString(), "-", "")

	logStream, err := s.StreamLogs(context.Background(), taskID)
	require.NoError(t, err)
	require.NotNil(t, logStream)

	logEntry1 := &log.Entry{TaskID: taskID, Log: "first log line"}
	logData1, _ := json.Marshal(logEntry1)
	err = memoryBroker.Publish(context.Background(), logData1, broker.PublishWithQueue(string(log.Queue)))
	require.NoError(t, err)

	logEntry2 := &log.Entry{TaskID: taskID, Log: "second log line"}
	logData2, _ := json.Marshal(logEntry2)
	err = memoryBroker.Publish(context.Background(), logData2, broker.PublishWithQueue(string(log.Queue)))
	require.NoError(t, err)

	// Assert
	select {
	case line := <-logStream.Logs:
		assert.Equal(t, "first log line", line)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for the first log line")
	}

	select {
	case line := <-logStream.Logs:
		assert.Equal(t, "second log line", line)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for the second log line")
	}

	t.Cleanup(func() {
		close(logStream.Stop)
		close(stop)
		wg.Wait()
	})
}

func TestStreamer_StreamLogs_MultipleClients(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memory.NewBroker()

	s := streamer.New(memoryBroker, map[string]int{string(log.Queue): 1})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Start(stop)
	}()

	// Act
	taskID := strings.ReplaceAll(uuid.NewString(), "-", "")

	logStream1, err := s.StreamLogs(context.Background(), taskID)
	require.NoError(t, err)
	require.NotNil(t, logStream1)

	logStream2, err := s.StreamLogs(context.Background(), taskID)
	require.NoError(t, err)
	require.NotNil(t, logStream2)

	logEntry := &log.Entry{TaskID: taskID, Log: "broadcast log line"}
	logData, _ := json.Marshal(logEntry)
	err = memoryBroker.Publish(context.Background(), logData, broker.PublishWithQueue(string(log.Queue)))
	require.NoError(t, err)

	// Assert
	select {
	case line := <-logStream1.Logs:
		assert.Equal(t, "broadcast log line", line)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for the log line")
	}

	select {
	case line := <-logStream2.Logs:
		assert.Equal(t, "broadcast log line", line)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for the log line")
	}

	t.Cleanup(func() {
		close(logStream1.Stop)
		close(logStream2.Stop)
		close(stop)
		wg.Wait()
	})
}

func TestStreamer_StreamLogs_Cancelled(t *testing.T) {
	if len(os.Getenv("INTEGRATION")) > 0 {
		t.Log("SKIPPING UNIT TEST")
		return
	}

	// Arrange
	memoryBroker := memory.NewBroker()

	s := streamer.New(memoryBroker, map[string]int{string(log.Queue): 1})

	stop := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Start(stop)
	}()

	// Act
	taskID := strings.ReplaceAll(uuid.NewString(), "-", "")

	logStream, err := s.StreamLogs(context.Background(), taskID)
	require.NoError(t, err)
	require.NotNil(t, logStream)

	close(logStream.Stop)

	select {
	case <-logStream.Stop:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for stream to stop")
	}

	logEntry := &log.Entry{TaskID: taskID, Log: "log line"}
	logData, _ := json.Marshal(logEntry)
	err = memoryBroker.Publish(context.Background(), logData, broker.PublishWithQueue(string(log.Queue)))
	require.NoError(t, err)

	// Assert
	select {
	case _, ok := <-logStream.Logs:
		assert.False(t, ok)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for channel to close")
	}

	t.Cleanup(func() {
		close(stop)
		wg.Wait()
	})
}
