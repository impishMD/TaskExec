package hooks

import (
	"github.com/impishMD/jeh/db"
)

type AnsibleHook struct {
}

func (h *AnsibleHook) End(store db.Store, projectID int, taskID int) {
}
