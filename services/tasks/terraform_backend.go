package tasks

import "github.com/impishMD/taskexec/db"

// ResolveTerraformAlias grants a running task access only to its own workspace.
// Task aliases are cryptographically random capabilities removed at completion.
// Read live status from the database, not the concurrently changing runner status.
func (p *TaskPool) ResolveTerraformAlias(alias string) (db.TerraformInventoryAlias, error) {
	runner := p.state.GetByAlias(alias)
	if runner == nil || !runner.Template.App.IsTerraform() {
		return db.TerraformInventoryAlias{}, db.ErrNotFound
	}
	task, err := p.store.GetTask(runner.Task.ProjectID, runner.Task.ID)
	if err != nil {
		return db.TerraformInventoryAlias{}, err
	}
	if task.Status.IsFinished() {
		return db.TerraformInventoryAlias{}, db.ErrNotFound
	}
	return db.TerraformInventoryAlias{ProjectID: task.ProjectID, InventoryID: runner.Inventory.ID, TaskID: &task.ID, Alias: alias}, nil
}
