package containerworker

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/stretchr/testify/require"
)

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(data []byte) (int, error) { return f(data) }

func TestAppObservesConfirmationStateBeforeRunner(t *testing.T) {
	var ready atomic.Bool
	output := writerFunc(func(p []byte) (int, error) {
		require.True(t, ready.Load(), "the app must enter the waiting state before publishing it")
		return len(p), nil
	})
	l := &logger{encoder: json.NewEncoder(output)}
	l.AddStatusListener(func(s task_logger.TaskStatus) { ready.Store(s == task_logger.TaskWaitingConfirmation) })
	l.SetStatus(task_logger.TaskWaitingConfirmation)
}

func TestConcurrentWorkerLogsRemainValidJSON(t *testing.T) {
	var output bytes.Buffer
	l := &logger{encoder: json.NewEncoder(&output)}
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Go(func() {
			for j := 0; j < 50; j++ {
				l.Logf("line %d", j)
			}
		})
	}
	wg.Wait()
	decoder := json.NewDecoder(&output)
	count := 0
	for {
		var event Event
		err := decoder.Decode(&event)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		require.Equal(t, "log", event.Type)
		count++
	}
	require.Equal(t, 250, count)
}
func TestCommandOutputDrainsBeforeResult(t *testing.T) {
	var output bytes.Buffer
	l := &logger{encoder: json.NewEncoder(&output)}
	cmd := exec.Command("sh", "-c", "echo stdout; echo stderr >&2")
	finish := l.LogCmd(cmd)
	require.NoError(t, cmd.Run())
	finish()
	finish()
	l.emit(Event{Type: "result"})
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	require.Len(t, lines, 3)
	var result Event
	require.NoError(t, json.Unmarshal([]byte(lines[2]), &result))
	require.Equal(t, "result", result.Type)
}
