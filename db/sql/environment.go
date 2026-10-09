package sql

import (
	"github.com/impishMD/taskexec/db"
)

func (d *SqlDb) GetEnvironment(projectID int, environmentID int) (environment db.Environment, err error) {
	err = d.getObject(projectID, db.EnvironmentProps, environmentID, &environment)
	if err != nil {
		return
	}

	err = d.fillEnvironmentBindings(&environment)
	return
}

func (d *SqlDb) GetEnvironmentRefs(projectID int, environmentID int) (refs db.ObjectReferrers, err error) {
	refs, err = d.getObjectRefs(projectID, db.EnvironmentProps, environmentID)
	if err != nil {
		return
	}

	var extra []db.ObjectReferrer
	_, err = d.selectAll(
		&extra,
		"select t.id, t.name from project__template t "+
			"join project__template_environment pte "+
			"on pte.template_id = t.id and pte.project_id = t.project_id "+
			"where t.project_id = ? and pte.environment_id = ?",
		projectID,
		environmentID,
	)

	if err != nil {
		return
	}

	seen := make(map[int]bool)
	for _, r := range refs.Templates {
		seen[r.ID] = true
	}
	for _, r := range extra {
		if seen[r.ID] {
			continue
		}
		refs.Templates = append(refs.Templates, r)
	}

	return
}

func (d *SqlDb) GetEnvironments(projectID int, params db.RetrieveQueryParams) ([]db.Environment, error) {
	var environments []db.Environment
	err := d.getObjects(projectID, db.EnvironmentProps, params, nil, &environments)
	if err != nil {
		return environments, err
	}

	for i := range environments {
		if err = d.fillEnvironmentBindings(&environments[i]); err != nil {
			return environments, err
		}
	}

	return environments, nil
}

func (d *SqlDb) UpdateEnvironment(env db.Environment) error {
	if err := env.Validate(); err != nil {
		return err
	}
	tx, err := d.Sql().Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := d.execTx(tx, "update project__environment set name=?, json=?, env=?, password=? where id=? and project_id=?", env.Name, env.JSON, env.ENV, env.Password, env.ID, env.ProjectID)
	if err = validateMutationResult(res, err); err != nil {
		return err
	}
	if err = d.saveEnvironmentBindings(tx, env); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (d *SqlDb) CreateEnvironment(env db.Environment) (newEnv db.Environment, err error) {
	if err = env.Validate(); err != nil {
		return
	}
	tx, err := d.Sql().Begin()
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback() }()
	env.ID = 0
	if err = tx.Insert(&env); err != nil {
		return
	}
	if err = d.saveEnvironmentBindings(tx, env); err != nil {
		return
	}
	if err = tx.Commit(); err != nil {
		return
	}
	newEnv = env
	return
}

func (d *SqlDb) DeleteEnvironment(projectID int, environmentID int) error {
	return d.deleteObject(projectID, db.EnvironmentProps, environmentID)
}

func (d *SqlDb) GetEnvironmentSecrets(projectID int, environmentID int) (keys []db.AccessKey, err error) {
	keys = make([]db.AccessKey, 0)

	q, err := d.makeObjectsQuery(projectID, db.AccessKeyProps, db.RetrieveQueryParams{})

	if err != nil {
		return
	}

	q = q.Where("pe.environment_id = ?", environmentID)

	query, args, err := q.ToSql()

	if err != nil {
		return
	}

	_, err = d.selectAll(&keys, query, args...)

	return
}
