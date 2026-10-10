package sql

import (
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/tz"
)

func (d *SqlDb) GetProjectTokens(projectID int) ([]db.ProjectToken, error) {
	items := []db.ProjectToken{}
	_, err := d.selectAll(&items, "select * from project__token where project_id=? order by created desc, id", projectID)
	if err != nil {
		return nil, err
	}
	// gorp does not invoke pointer PostGet hooks for a slice of values.
	for i := range items {
		if err = items[i].PostGet(d.Sql()); err != nil {
			return nil, err
		}
	}
	return items, err
}
func (d *SqlDb) GetProjectToken(projectID int, id string) (t db.ProjectToken, err error) {
	err = d.selectOne(&t, "select * from project__token where project_id=? and id=?", projectID, id)
	return
}
func (d *SqlDb) GetProjectTokenByID(id string) (t db.ProjectToken, err error) {
	err = d.selectOne(&t, "select * from project__token where id=?", id)
	return
}
func (d *SqlDb) CreateProjectToken(token db.ProjectToken, replaceID string) (db.ProjectToken, error) {
	token.Created = db.GetParsedTime(tz.Now())
	tx, err := d.Sql().Begin()
	if err != nil {
		return token, err
	}
	defer tx.Rollback()
	if replaceID != "" {
		result, e := tx.Exec(d.PrepareQuery("update project__token set revoked_at=? where project_id=? and id=? and revoked_at is null"), token.Created, token.ProjectID, replaceID)
		if e != nil {
			return token, e
		}
		if e = requireDeletedRow(result, nil); e != nil {
			return token, e
		}
	}
	if err = tx.Insert(&token); err != nil {
		return token, err
	}
	return token, tx.Commit()
}
func (d *SqlDb) RevokeProjectToken(projectID int, id string) error {
	result, err := d.exec("update project__token set revoked_at=? where project_id=? and id=? and revoked_at is null", tz.Now(), projectID, id)
	return requireDeletedRow(result, err)
}
func (d *SqlDb) TouchProjectToken(projectID int, id string) error {
	_, err := d.exec("update project__token set last_used_at=? where project_id=? and id=?", tz.Now(), projectID, id)
	return err
}
