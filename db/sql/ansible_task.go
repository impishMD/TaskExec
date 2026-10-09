package sql

import (
	"errors"

	"github.com/impishMD/taskexec/db"
)

// CreateAnsibleTaskHost stores the latest recap for a host. A repeated runner
// report replaces counters instead of duplicating the host in the summary.
func (d *SqlDb) CreateAnsibleTaskHost(host db.AnsibleTaskHost) error {
	if err := d.validateTask(host.ProjectID, host.TaskID); err != nil {
		return err
	}
	var existingID int
	err := d.selectOne(&existingID,
		"select id from task__ansible_host where project_id=? and task_id=? and host=? order by id limit 1",
		host.ProjectID, host.TaskID, host.Host)
	if err == nil {
		_, err = d.exec("update task__ansible_host set changed=?, failed=?, ignored=?, ok=?, rescued=?, skipped=?, unreachable=? where id=?",
			host.Changed, host.Failed, host.Ignored, host.Ok, host.Rescued, host.Skipped, host.Unreachable, existingID)
		return err
	}
	if !errors.Is(err, db.ErrNotFound) {
		return err
	}
	_, err = d.insert("id", "insert into task__ansible_host "+
		"(task_id, project_id, host, changed, failed, ignored, ok, rescued, skipped, unreachable, created) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		host.TaskID, host.ProjectID, host.Host, host.Changed, host.Failed, host.Ignored,
		host.Ok, host.Rescued, host.Skipped, host.Unreachable, host.Created)
	return err
}

func (d *SqlDb) CreateAnsibleTaskError(item db.AnsibleTaskError) error {
	if err := d.validateTask(item.ProjectID, item.TaskID); err != nil {
		return err
	}
	// Runner retries preserve the original log timestamp. Separate failures of
	// the same task/host remain distinct when they occurred at different times.
	var id int
	err := d.selectOne(&id, "select id from task__ansible_error where project_id=? and task_id=? and host=? and task=? and created=? limit 1",
		item.ProjectID, item.TaskID, item.Host, item.Task, item.Created)
	if err == nil {
		return nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return err
	}
	_, err = d.insert("id", "insert into task__ansible_error (task_id, project_id, host, task, error, created) values (?, ?, ?, ?, ?, ?)",
		item.TaskID, item.ProjectID, item.Host, item.Task, item.Error, item.Created)
	return err
}

func (d *SqlDb) GetAnsibleTaskHosts(projectID, taskID int) ([]db.AnsibleTaskHost, error) {
	res := make([]db.AnsibleTaskHost, 0)
	if err := d.validateTask(projectID, taskID); err != nil {
		return nil, err
	}
	// Older installations may have rows predating the created column.
	_, err := d.selectAll(&res, "select id, task_id, project_id, host, changed, failed, ignored, ok, rescued, skipped, unreachable, created "+
		"from task__ansible_host where project_id=? and task_id=? order by host, id", projectID, taskID)
	return res, err
}

func (d *SqlDb) GetAnsibleTaskErrors(projectID, taskID int) ([]db.AnsibleTaskError, error) {
	res := make([]db.AnsibleTaskError, 0)
	if err := d.validateTask(projectID, taskID); err != nil {
		return nil, err
	}
	_, err := d.selectAll(&res, "select id, task_id, project_id, coalesce(host, '') as host, task, error, created "+
		"from task__ansible_error where project_id=? and task_id=? order by id", projectID, taskID)
	return res, err
}
