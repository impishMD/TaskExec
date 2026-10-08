package factory

import (
	"github.com/impishMD/jeh/db"
	"github.com/impishMD/jeh/pro/db/sql"
)

func NewTerraformStore(store db.Store) db.TerraformStore {
	return &sql.TerraformStoreImpl{}
}

func NewAnsibleTaskRepository(store db.Store) db.AnsibleTaskRepository {
	return &sql.AnsibleTaskStoreImpl{}
}

func NewWorkflowStore(store db.Store) db.WorkflowManager {
	return &sql.WorkflowStoreImpl{}
}
