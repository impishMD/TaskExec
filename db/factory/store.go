package factory

import (
	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/db/sql"
	"github.com/impishMD/taskexec/util"
)

func CreateStore() db.Store {
	config, err := util.Config.GetDBConfig()
	if err != nil {
		panic("Can not read configuration")
	}
	switch config.Dialect {
	case util.DbDriverMySQL:
		return sql.CreateDb(config.Dialect)
	case util.DbDriverBolt:
		panic("Bolt is not supported starting from version 2.19")
	case util.DbDriverPostgres:
		return sql.CreateDb(config.Dialect)
	case util.DbDriverSQLite:
		return sql.CreateDb(config.Dialect)
	default:
		panic("Unsupported database dialect: " + config.Dialect)
	}
}
