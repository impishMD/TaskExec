package sql

import (
	stdsql "database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/go-gorp/gorp/v3"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/common_errors"
	"github.com/impishMD/taskexec/pkg/tz"
)

// Lock the owning inventory row, even when no protocol lock exists yet. Every
// alias of this workspace and every server process serialize on the same row.
func (d *SqlDb) terraformTransaction(projectID, inventoryID int, fn func(*gorp.Transaction) error) error {
	tx, err := d.Sql().Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = d.execTx(tx, "update project__inventory set name=name where project_id=? and id=?", projectID, inventoryID)
	if err != nil {
		return err
	}
	count, err := tx.SelectInt(d.PrepareQuery("select count(*) from project__inventory where project_id=? and id=? and type in (?, ?, ?)"), projectID, inventoryID, db.InventoryTerraformWorkspace, db.InventoryTofuWorkspace, db.InventoryTerragruntWorkspace)
	if err != nil {
		return err
	}
	if count != 1 {
		return db.ErrNotFound
	}
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *SqlDb) terraformLock(tx *gorp.Transaction, p, i int) (db.TerraformStateLock, error) {
	var raw string
	err := tx.SelectOne(&raw, d.PrepareQuery("select lock_data from project__terraform_inventory_lock where project_id=? and inventory_id=?"), p, i)
	if errors.Is(err, stdsql.ErrNoRows) {
		return db.TerraformStateLock{}, nil
	}
	if err != nil {
		return db.TerraformStateLock{}, err
	}
	var lock db.TerraformStateLock
	err = json.Unmarshal([]byte(raw), &lock)
	return lock, err
}
func (d *SqlDb) checkTerraformLock(tx *gorp.Transaction, p, i int, id string) error {
	lock, err := d.terraformLock(tx, p, i)
	if err != nil {
		return err
	}
	if lock.ID != id {
		return &db.TerraformLockError{Lock: lock}
	}
	return nil
}
func (d *SqlDb) LockTerraformInventoryState(p, i int, lock db.TerraformStateLock) error {
	if strings.TrimSpace(lock.ID) == "" || len(lock.ID) > 128 {
		return common_errors.NewValidationError("a valid lock ID is required")
	}
	raw, err := json.Marshal(lock)
	if err != nil {
		return err
	}
	if len(raw) > 65536 {
		return common_errors.NewValidationError("lock information is too large")
	}
	return d.terraformTransaction(p, i, func(tx *gorp.Transaction) error {
		current, err := d.terraformLock(tx, p, i)
		if err != nil {
			return err
		}
		if current.ID != "" {
			if current.ID == lock.ID {
				return nil
			}
			return &db.TerraformLockError{Lock: current}
		}
		_, err = d.execTx(tx, "insert into project__terraform_inventory_lock (inventory_id, project_id, lock_data) values (?, ?, ?)", i, p, string(raw))
		return err
	})
}
func (d *SqlDb) UnlockTerraformInventoryState(p, i int, id string) error {
	if id == "" {
		return common_errors.NewValidationError("lock ID is required")
	}
	return d.terraformTransaction(p, i, func(tx *gorp.Transaction) error {
		current, err := d.terraformLock(tx, p, i)
		if err != nil {
			return err
		}
		if current.ID == "" {
			return nil
		} // A repeated successful unlock is harmless.
		if current.ID != id {
			return &db.TerraformLockError{Lock: current}
		}
		_, err = d.execTx(tx, "delete from project__terraform_inventory_lock where project_id=? and inventory_id=?", p, i)
		return err
	})
}

func (d *SqlDb) CreateTerraformInventoryAlias(alias db.TerraformInventoryAlias) (db.TerraformInventoryAlias, error) {
	err := d.terraformTransaction(alias.ProjectID, alias.InventoryID, func(tx *gorp.Transaction) error {
		if err := d.validateTerraformAuthKey(tx, alias); err != nil {
			return err
		}
		return tx.Insert(&alias)
	})
	return alias, err
}
func (d *SqlDb) validateTerraformAuthKey(tx *gorp.Transaction, alias db.TerraformInventoryAlias) error {
	count, err := tx.SelectInt(d.PrepareQuery("select count(*) from access_key where id=? and project_id=? and owner=? and type=?"), alias.AuthKeyID, alias.ProjectID, db.AccessKeyShared, db.AccessKeyLoginPassword)
	if err != nil {
		return err
	}
	if count != 1 {
		return common_errors.NewValidationError("select a login/password key belonging to this project")
	}
	return nil
}
func (d *SqlDb) UpdateTerraformInventoryAlias(alias db.TerraformInventoryAlias) error {
	return d.terraformTransaction(alias.ProjectID, alias.InventoryID, func(tx *gorp.Transaction) error {
		if err := d.validateTerraformAuthKey(tx, alias); err != nil {
			return err
		}
		count, err := tx.SelectInt(d.PrepareQuery("select count(*) from project__terraform_inventory_alias where project_id=? and inventory_id=? and alias=?"), alias.ProjectID, alias.InventoryID, alias.Alias)
		if err != nil {
			return err
		}
		if count != 1 {
			return db.ErrNotFound
		}
		_, err = d.execTx(tx, "update project__terraform_inventory_alias set auth_key_id=? where project_id=? and inventory_id=? and alias=?", alias.AuthKeyID, alias.ProjectID, alias.InventoryID, alias.Alias)
		return err
	})
}
func (d *SqlDb) GetTerraformInventoryAliasByAlias(name string) (alias db.TerraformInventoryAlias, err error) {
	err = d.selectOne(&alias, "select * from project__terraform_inventory_alias where alias=?", name)
	return
}
func (d *SqlDb) GetTerraformInventoryAlias(p, i int, name string) (alias db.TerraformInventoryAlias, err error) {
	err = d.selectOne(&alias, "select * from project__terraform_inventory_alias where project_id=? and inventory_id=? and alias=?", p, i, name)
	return
}
func (d *SqlDb) GetTerraformInventoryAliases(p, i int) (aliases []db.TerraformInventoryAlias, err error) {
	aliases = make([]db.TerraformInventoryAlias, 0)
	_, err = d.selectAll(&aliases, "select * from project__terraform_inventory_alias where project_id=? and inventory_id=? order by alias", p, i)
	return
}
func (d *SqlDb) DeleteTerraformInventoryAlias(p, i int, name string) error {
	if _, err := d.GetTerraformInventoryAlias(p, i, name); err != nil {
		return err
	}
	_, err := d.exec("delete from project__terraform_inventory_alias where project_id=? and inventory_id=? and alias=?", p, i, name)
	return err
}

func validTerraformState(value string) error {
	if len(value) > db.TerraformMaxStateBytes {
		return common_errors.NewValidationError("Terraform state exceeds 32 MiB")
	}
	var state map[string]json.RawMessage
	if json.Unmarshal([]byte(value), &state) != nil || state == nil {
		return common_errors.NewValidationError("Terraform state must be a JSON object")
	}
	return nil
}
func (d *SqlDb) SaveTerraformInventoryState(state db.TerraformInventoryState, lockID string) (db.TerraformInventoryState, error) {
	if err := validTerraformState(state.State); err != nil {
		return db.TerraformInventoryState{}, err
	}
	state.ID = 0
	state.Created = tz.Now()
	err := d.terraformTransaction(state.ProjectID, state.InventoryID, func(tx *gorp.Transaction) error {
		if err := d.checkTerraformLock(tx, state.ProjectID, state.InventoryID, lockID); err != nil {
			return err
		}
		if state.TaskID != nil {
			count, err := tx.SelectInt(d.PrepareQuery("select count(*) from task where id=? and project_id=?"), *state.TaskID, state.ProjectID)
			if err != nil {
				return err
			}
			if count != 1 {
				return db.ErrNotFound
			}
		}
		return tx.Insert(&state)
	})
	return state, err
}
func (d *SqlDb) CreateTerraformInventoryState(state db.TerraformInventoryState) (db.TerraformInventoryState, error) {
	return d.SaveTerraformInventoryState(state, "")
}
func (d *SqlDb) ResetTerraformInventoryState(p, i int, lockID string) error {
	return d.terraformTransaction(p, i, func(tx *gorp.Transaction) error {
		if err := d.checkTerraformLock(tx, p, i, lockID); err != nil {
			return err
		}
		// Empty rows are deletion markers. Resetting the backend keeps all historical
		// snapshots and can never make an earlier snapshot become current again.
		state := db.TerraformInventoryState{ProjectID: p, InventoryID: i, Created: tz.Now()}
		return tx.Insert(&state)
	})
}
func (d *SqlDb) GetLatestTerraformInventoryState(p, i int) (state db.TerraformInventoryState, err error) {
	err = d.selectOne(&state, "select * from project__terraform_inventory_state where project_id=? and inventory_id=? order by id desc limit 1", p, i)
	if err == nil && state.State == "" {
		err = db.ErrNotFound
	}
	return
}
func (d *SqlDb) GetTerraformInventoryState(p, i, id int) (state db.TerraformInventoryState, err error) {
	err = d.selectOne(&state, "select * from project__terraform_inventory_state where project_id=? and inventory_id=? and id=? and state<>''", p, i, id)
	return
}
func (d *SqlDb) GetTerraformInventoryStates(p, i int, params db.RetrieveQueryParams) (states []db.TerraformInventoryState, err error) {
	count := params.Count
	if count > 1000 {
		count = 1000
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}
	states = make([]db.TerraformInventoryState, 0)
	// History listings never carry the potentially sensitive state payload.
	query := "select id, created, task_id, project_id, inventory_id from project__terraform_inventory_state where project_id=? and inventory_id=? and state<>'' order by id desc"
	args := []any{p, i}
	if count > 0 {
		query += " limit ? offset ?"
		args = append(args, count, offset)
	}
	_, err = d.selectAll(&states, query, args...)
	return
}
func (d *SqlDb) DeleteTerraformInventoryState(p, i, id int) error {
	return d.terraformTransaction(p, i, func(tx *gorp.Transaction) error {
		if err := d.checkTerraformLock(tx, p, i, ""); err != nil {
			return err
		}
		count, err := tx.SelectInt(d.PrepareQuery("select count(*) from project__terraform_inventory_state where project_id=? and inventory_id=? and id=? and state<>''"), p, i, id)
		if err != nil {
			return err
		}
		if count != 1 {
			return db.ErrNotFound
		}
		latest, err := tx.SelectInt(d.PrepareQuery("select max(id) from project__terraform_inventory_state where project_id=? and inventory_id=?"), p, i)
		if err != nil {
			return err
		}
		if latest == int64(id) {
			marker := db.TerraformInventoryState{ProjectID: p, InventoryID: i, Created: tz.Now()}
			if err = tx.Insert(&marker); err != nil {
				return err
			}
		}
		_, err = d.execTx(tx, "delete from project__terraform_inventory_state where project_id=? and inventory_id=? and id=?", p, i, id)
		return err
	})
}
func (d *SqlDb) GetTerraformStateCount() (int, error) {
	var count int
	err := d.selectOne(&count, "select count(*) from project__terraform_inventory_state where state<>''")
	return count, err
}
