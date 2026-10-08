package hooks

import "github.com/impishMD/jeh/db"

type Hook interface {
	End(store db.Store, projectID int, taskID int)
}
