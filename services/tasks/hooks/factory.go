package hooks

import (
	"github.com/impishMD/jeh/db"
)

func GetHook(app db.TemplateApp) Hook {
	switch app {
	case db.AppAnsible:
		return &AnsibleHook{}
	default:
		return nil
	}
}
