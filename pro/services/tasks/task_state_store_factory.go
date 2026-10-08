package tasks

import (
	"github.com/impishMD/jeh/services/tasks"
)

func NewTaskStateStore() tasks.TaskStateStore {
	return tasks.NewMemoryTaskStateStore()
}
