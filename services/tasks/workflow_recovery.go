package tasks

import (
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/task_logger"
	"github.com/impishMD/taskexec/pkg/tz"
)

// RecoverWorkflowTasks runs once before accepting requests. Local processes
// cannot be reattached after a server restart; replaying their nodes could
// duplicate external changes. Stop those runs and keep their task history.
// Runs waiting only for approvals/delays resume through the reconciler, and
// assigned remote tasks are handled by the existing runner reconciliation.
func (p *TaskPool) RecoverWorkflowTasks() error {
	if p.workflowRepo == nil || p.workflowService == nil {
		return nil
	}
	runs, err := p.workflowRepo.GetActiveWorkflowRuns()
	if err != nil {
		return err
	}
	for _, run := range runs {
		tasks, err := p.workflowRepo.GetWorkflowRunTasks(run.ProjectID, run.ID, db.RetrieveQueryParams{})
		if err != nil {
			return err
		}
		interrupted := false
		for _, task := range tasks {
			if task.RunnerID != nil || task.Status.IsFinished() {
				continue
			}
			task.Status, task.End = task_logger.TaskStoppedStatus, new(tz.Now())
			if err = p.store.UpdateTask(task.Task); err != nil {
				return err
			}
			if _, err = p.store.CreateTaskOutput(db.TaskOutput{TaskID: task.ID, Time: tz.Now(), Output: "Workflow interrupted by server restart; this task was not replayed."}); err != nil {
				return err
			}
			interrupted = true
		}
		if interrupted {
			if _, err = p.workflowService.StopWorkflowRun(run.ProjectID, run.ID, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
