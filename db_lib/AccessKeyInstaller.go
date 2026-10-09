package db_lib

import (
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/ssh"
	"github.com/impishMD/taskexec/pkg/task_logger"
)

type AccessKeyInstaller interface {
	Install(key db.AccessKey, usage db.AccessKeyRole, logger task_logger.Logger) (installation ssh.AccessKeyInstallation, err error)
}
