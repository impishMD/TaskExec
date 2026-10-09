package sql

import (
	"errors"
	"github.com/impishMD/taskexec/db"
)

// SaveVaultStorage commits the connection and encrypted credentials
// together: a failed method change must not leave half of the previous connection.
func (d *SqlDb) SaveVaultStorage(storage db.SecretStorage, key db.AccessKey) (db.SecretStorage, error) {
	if key.Secret == nil || key.SourceStorageType != nil || key.ProjectID == nil || *key.ProjectID != storage.ProjectID || key.Owner != db.AccessKeySecretStorage {
		return storage, errors.New("invalid Vault credentials")
	}
	tx, err := d.Sql().Begin()
	if err != nil {
		return storage, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = d.execTx(tx, "update project set name=name where id=?", storage.ProjectID); err != nil {
		return storage, err
	}
	if storage.ID == 0 {
		storage.ID, err = d.GetConnection().InsertTx(tx, "id", "insert into project__secret_storage (name,type,project_id,params,readonly) values (?,?,?,?,?)", storage.Name, storage.Type, storage.ProjectID, storage.Params, storage.ReadOnly)
	} else {
		count, e := tx.SelectInt(d.PrepareQuery("select count(*) from project__secret_storage where project_id=? and id=?"), storage.ProjectID, storage.ID)
		if e != nil {
			return storage, e
		}
		if count != 1 {
			return storage, db.ErrNotFound
		}
		_, err = d.execTx(tx, "update project__secret_storage set name=?,type=?,params=?,readonly=? where project_id=? and id=?", storage.Name, storage.Type, storage.Params, storage.ReadOnly, storage.ProjectID, storage.ID)
	}
	if err != nil {
		return storage, err
	}
	if key.ID == 0 {
		key.StorageID = &storage.ID
		if err = tx.Insert(&key); err != nil {
			return storage, err
		}
	} else {
		result, e := d.execTx(tx, "update access_key set name=?,secret=?,source_storage_type=NULL,source_storage_key=NULL,source_storage_id=NULL where id=? and project_id=? and storage_id=? and owner=?", key.Name, key.Secret, key.ID, storage.ProjectID, storage.ID, db.AccessKeySecretStorage)
		if err = validateMutationResult(result, e); err != nil {
			return storage, err
		}
	}

	return storage, tx.Commit()
}
