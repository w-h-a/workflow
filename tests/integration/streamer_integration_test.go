package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/w-h-a/workflow/internal/task"
)

func TestStreamer_StreamLogs_EndToEnd(t *testing.T) {
	// Arrange
	testTask := &task.Task{
		Image: "alpine",
		Cmd:   []string{"sh", "-c", `sleep 3; for i in $(seq 1 3); do echo "log line $i"; sleep 3; done`},
	}

	bs, _ := json.Marshal(testTask)

	req1, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://localhost:4000/tasks",
		bytes.NewBuffer(bs),
	)
	require.NoError(t, err)

	client := &http.Client{Timeout: 30 * time.Second}

	rsp, err := client.Do(req1)
	require.NoError(t, err)
	defer rsp.Body.Close()

	var scheduledTask task.Task
	err = json.NewDecoder(rsp.Body).Decode(&scheduledTask)
	require.NoError(t, err)

	// Act
	req2, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		fmt.Sprintf("http://localhost:4002/logs/%s", scheduledTask.ID),
		nil,
	)
	require.NoError(t, err)

	req2.Header.Set("Accept", "text/event-stream")

	stream, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	defer stream.Body.Close()

	want := []string{"log line 1", "log line 2", "log line 3"}
	var got []string

	reader := stream.Body
	buf := make([]byte, 1024)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			lines := strings.Split(string(buf[:n]), "\n")
			for _, line := range lines {
				if after, ok := strings.CutPrefix(line, "data: "); ok {
					logLine := after
					got = append(got, logLine)
					t.Logf("Received log: %s\n", logLine)
				}
			}
		}

		if len(got) >= len(want) {
			break
		}

		if err != nil {
			t.Logf("stream ended prematurely: %v", err)
			break
		}
	}

	// Assert
	assert.Equal(t, len(want), len(got))

	for i, expected := range want {
		assert.Equal(t, expected, got[i])
	}
}
