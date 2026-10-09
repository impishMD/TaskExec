package projects

import (
	"github.com/impishMD/taskexec/api/runners"
	"github.com/impishMD/taskexec/services/server"
)

func NewProjectRunnerController(service server.RunnerService) *runners.Controller {
	return runners.NewController(service, true)
}
