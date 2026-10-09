package tasks

import (
	"sync"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRunner(id, projectID, templateID int) *TaskRunner {
	runnerID := 7
	return &TaskRunner{
		Task: db.Task{
			ID:         id,
			ProjectID:  projectID,
			TemplateID: templateID,
			Status:     task_logger.TaskRunningStatus,
			RunnerID:   &runnerID,
		},
		Username: "admin",
		Alias:    "",
	}
}

func TestMemoryTaskStateStore_ClaimAndDequeue(t *testing.T) {
	s := NewMemoryTaskStateStore()
	s.Enqueue(newTestRunner(1, 10, 100))
	s.Enqueue(newTestRunner(2, 10, 100))

	// First claim succeeds: task leaves the queue and enters the running set.
	assert.True(t, s.ClaimAndDequeue(1))
	assert.Equal(t, 1, s.QueueLen(), "claimed task must be removed from the queue")
	assert.Equal(t, 1, s.RunningCount(), "claimed task must be added to the running set")
	assert.Nil(t, s.QueueGet(1), "only one task should remain queued")

	// Second claim of the same task fails (no longer queued).
	assert.False(t, s.ClaimAndDequeue(1))

	// Unknown task ID fails without side effects.
	assert.False(t, s.ClaimAndDequeue(999))
	assert.Equal(t, 1, s.QueueLen())
}

func TestMemoryTaskStateStore_DequeueByID(t *testing.T) {
	s := NewMemoryTaskStateStore()
	s.Enqueue(newTestRunner(1, 10, 100))
	s.Enqueue(newTestRunner(2, 10, 100))
	s.Enqueue(newTestRunner(3, 10, 100))

	// Remove the middle element by value; the others keep their order.
	s.DequeueByID(2)
	require.Equal(t, 2, s.QueueLen())
	assert.Equal(t, 1, s.QueueGet(0).Task.ID)
	assert.Equal(t, 3, s.QueueGet(1).Task.ID)

	// Removing the running task (not queued) is a no-op.
	s.DequeueByID(99)
	assert.Equal(t, 2, s.QueueLen())
}

// TestMemoryTaskStateStore_ClaimAndDequeue_ExactlyOnce ensures concurrent
// claimers never run the same task twice and never lose a task — the contract
// that the Redis store must also honor across nodes.
func TestMemoryTaskStateStore_ClaimAndDequeue_ExactlyOnce(t *testing.T) {
	s := NewMemoryTaskStateStore()
	const n = 200
	for i := 1; i <= n; i++ {
		s.Enqueue(newTestRunner(i, 10, 100))
	}

	var mu sync.Mutex
	claimed := map[int]int{} // taskID -> times claimed

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := 1; id <= n; id++ {
				if s.ClaimAndDequeue(id) {
					mu.Lock()
					claimed[id]++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, 0, s.QueueLen(), "every task must be dequeued")
	assert.Len(t, claimed, n, "every task must be claimed")
	for id, count := range claimed {
		assert.Equal(t, 1, count, "task %d must be claimed exactly once", id)
	}
}

// TestMemoryTaskStateStore_TryFinalize_ExactlyOnce ensures concurrent
// finalizers never both succeed — the contract Redis-backed stores must honor
// across HA nodes.
func TestMemoryTaskStateStore_TryFinalize_ExactlyOnce(t *testing.T) {
	s := NewMemoryTaskStateStore()
	const n = 200

	var mu sync.Mutex
	finalized := map[int]int{}

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := 1; id <= n; id++ {
				if s.TryFinalize(id) {
					mu.Lock()
					finalized[id]++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	for id := 1; id <= n; id++ {
		s.DeleteFinalize(id)
	}

	assert.Len(t, finalized, n, "every task must be finalized exactly once")
	for id, count := range finalized {
		assert.Equal(t, 1, count, "task %d must be finalized exactly once", id)
	}
}
