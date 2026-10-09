package tasks

import (
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/server"
)

func (p *TaskPool) useRemoteRunner(task db.Task, template db.Template, inventory db.Inventory) (bool, error) {
	// Changing the default must not turn a task already assigned to a runner into
	// a local task during recovery, stop, confirmation or progress handling.
	// Explicit template/inventory tags retain their existing routing semantics.
	if task.RunnerID != nil || template.RunnerTag != nil || inventory.RunnerTag != nil {
		return true, nil
	}
	settings, err := server.GetServerSettings(p.store)
	return settings.UseRemoteRunner, err
}
