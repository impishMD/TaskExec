package api

import (
	"github.com/impishMD/taskexec/api/runners"
	"github.com/impishMD/taskexec/services/server"
)

// GlobalRunnerController exposes administrator management of all runners.
type GlobalRunnerController = runners.Controller

func NewGlobalRunnerController(service server.RunnerService) *GlobalRunnerController {
	return runners.NewController(service, false)
}
