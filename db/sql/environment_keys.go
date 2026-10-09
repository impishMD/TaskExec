package sql

import (
	"database/sql"
	"fmt"

	"github.com/go-gorp/gorp/v3"
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/common_errors"
)

func (d *SqlDb) fillEnvironmentBindings(env *db.Environment) error {
	env.KeyBindings = []db.EnvironmentKeyBinding{}
	env.KeySources = []db.EnvironmentKeySource{}
	env.SecretExpressions = []db.EnvironmentSecretExpression{}
	var bindings []db.EnvironmentKeyBinding
	_, err := d.selectAll(&bindings, "select name, type, key_id, field from project__environment_key where environment_id=? order by type, name", env.ID)
	if err != nil {
		return err
	}
	for _, b := range bindings {
		if b.Type == db.EnvironmentKeySourceType {
			env.KeySources = append(env.KeySources, db.EnvironmentKeySource{Prefix: b.Name, KeyID: b.KeyID})
		} else {
			env.KeyBindings = append(env.KeyBindings, b)
		}
	}
	_, err = d.selectAll(&env.SecretExpressions, "select name, type, expression from project__environment_expression where environment_id=? order by type, name", env.ID)
	return err
}

func (d *SqlDb) saveEnvironmentBindings(tx *gorp.Transaction, env db.Environment) error {
	if err := env.ValidateKeySources(); err != nil {
		return common_errors.NewValidationError(err.Error())
	}
	if err := env.ValidateSecretExpressions(); err != nil {
		return common_errors.NewValidationError(err.Error())
	}
	if err := env.ValidateKeyBindings(); err != nil {
		return common_errors.NewValidationError(err.Error())
	}
	var previous []db.EnvironmentKeyBinding
	if _, err := tx.Select(&previous, d.PrepareQuery("select name,type,key_id,field from project__environment_key where environment_id=? and type=?"), env.ID, db.EnvironmentKeySourceType); err != nil {
		return err
	}
	oldSources := []db.EnvironmentKeySource{}
	for _, b := range previous {
		oldSources = append(oldSources, db.EnvironmentKeySource{Prefix: b.Name, KeyID: b.KeyID})
	}
	if err := env.ValidateKeyExpressions(oldSources); err != nil {
		return common_errors.NewValidationError(err.Error())
	}

	// Lock shared credentials before attaching them. Key edits/deletes use the
	// same rows, preventing a concurrent change of their interpretation type.
	bindings := append([]db.EnvironmentKeyBinding{}, env.KeyBindings...)
	for _, source := range env.KeySources {
		bindings = append(bindings, db.EnvironmentKeyBinding{Name: source.Prefix, KeyID: source.KeyID, Type: db.EnvironmentKeySourceType})
	}
	for _, binding := range bindings {
		_, err := d.execTx(tx, "update access_key set name=name where id=? and project_id=? and owner=?", binding.KeyID, env.ProjectID, db.AccessKeyShared)
		if err != nil {
			return err
		}
		var key db.AccessKey
		if err = tx.SelectOne(&key, d.PrepareQuery("select * from access_key where id=? and project_id=? and owner=?"), binding.KeyID, env.ProjectID, db.AccessKeyShared); err != nil {
			if err == sql.ErrNoRows {
				return db.ErrNotFound
			}
			return err
		}
		if (key.Type != db.AccessKeyString && key.Type != db.AccessKeyObject) || (binding.Field != nil && key.Type != db.AccessKeyObject) {
			return common_errors.NewValidationError("key type cannot be used by variable binding")
		}
		if binding.Type == db.EnvironmentKeySourceType {
			continue
		}
		owner := binding.Type.GetAccessKeyOwner()
		count, err := tx.SelectInt(d.PrepareQuery("select count(*) from access_key where environment_id=? and owner=? and name=?"), env.ID, owner, binding.Name)
		if err != nil {
			return err
		}
		if count > 0 {
			return common_errors.NewValidationError("key binding conflicts with a variable")
		}
	}
	if _, err := d.execTx(tx, "delete from project__environment_key where environment_id=?", env.ID); err != nil {
		return err
	}
	for _, b := range bindings {
		if _, err := d.execTx(tx, "insert into project__environment_key (environment_id, key_id, name, type, field) values (?, ?, ?, ?, ?)", env.ID, b.KeyID, b.Name, b.Type, b.Field); err != nil {
			return err
		}
	}
	for _, expression := range env.SecretExpressions {
		var existing []db.AccessKey
		if _, err := tx.Select(&existing, d.PrepareQuery("select * from access_key where environment_id=? and owner=? and name=?"), env.ID, expression.Type.GetAccessKeyOwner(), expression.Name); err != nil {
			return err
		}
		for _, key := range existing {
			deleted := false
			for _, operation := range env.Secrets {
				if operation.ID == key.ID && operation.Operation == db.EnvironmentSecretDelete {
					deleted = true
				}
			}
			if !deleted {
				return common_errors.NewValidationError("key binding conflicts with a variable")
			}
		}
	}
	if _, err := d.execTx(tx, "delete from project__environment_expression where environment_id=?", env.ID); err != nil {
		return err
	}
	for _, expression := range env.SecretExpressions {
		if _, err := d.execTx(tx, "insert into project__environment_expression (environment_id,name,type,expression) values (?,?,?,?)", env.ID, expression.Name, expression.Type, expression.Expression); err != nil {
			return err
		}
	}
	return nil
}

func (d *SqlDb) verifyVariableBindingsAcceptKey(tx *gorp.Transaction, key db.AccessKey) error {
	var bindings []db.EnvironmentKeyBinding
	_, err := tx.Select(&bindings, d.PrepareQuery("select b.name, b.type, b.key_id, b.field from project__environment_key b join project__environment e on e.id=b.environment_id where e.project_id=? and b.key_id=?"), *key.ProjectID, key.ID)
	if err != nil {
		return err
	}
	for _, b := range bindings {
		if (key.Type != db.AccessKeyObject && key.Type != db.AccessKeyString) || (b.Field != nil && key.Type != db.AccessKeyObject) {
			return common_errors.NewValidationError("key type cannot be used by variable binding")
		}
	}
	return nil
}

func (d *SqlDb) validateSecretBindingName(key db.AccessKey) error {
	if key.EnvironmentID == nil {
		return nil
	}
	var target db.EnvironmentSecretType
	switch key.Owner {
	case db.AccessKeyVariable:
		target = db.EnvironmentSecretVar
	case db.AccessKeyEnvironment:
		target = db.EnvironmentSecretEnv
	default:
		return nil
	}
	count, err := d.Sql().SelectInt(d.PrepareQuery("select count(*) from project__environment_key where environment_id=? and type=? and name=?"), *key.EnvironmentID, target, key.Name)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("key binding conflicts with a variable")
	}
	count, err = d.Sql().SelectInt(d.PrepareQuery("select count(*) from project__environment_expression where environment_id=? and type=? and name=?"), *key.EnvironmentID, target, key.Name)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("key binding conflicts with a variable")
	}
	return nil
}
