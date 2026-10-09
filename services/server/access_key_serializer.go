package server

import (
	"github.com/impishMD/taskexec/db"
)

type AccessKeyDeserializer interface {
	DeserializeSecret(key *db.AccessKey) (string, error)
	SerializeSecret(key *db.AccessKey) error
	DeleteSecret(key *db.AccessKey) error
}
