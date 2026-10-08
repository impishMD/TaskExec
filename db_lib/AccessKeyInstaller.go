package db_lib

import (
	"github.com/impishMD/jeh/db"
	"github.com/impishMD/jeh/pkg/ssh"
	"github.com/impishMD/jeh/pkg/task_logger"
)

type AccessKeyInstaller interface {
	Install(key db.AccessKey, usage db.AccessKeyRole, logger task_logger.Logger) (installation ssh.AccessKeyInstallation, err error)
}
