// Package containerworker runs the existing task executor inside an ephemeral
// container. The versioned stdin/stdout protocol never puts credentials in Docker
// environment variables, command arguments or labels. Keys are installed by the
// local executor only inside the disposable container.
package containerworker

import (
	"time"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/task_logger"
)

const ProtocolVersion = 1

type Request struct {
	Version     int             `json:"version"`
	Task        db.Task         `json:"task"`
	Template    db.Template     `json:"template"`
	Inventory   db.Inventory    `json:"inventory"`
	Repository  db.Repository   `json:"repository"`
	Environment db.Environment  `json:"environment"`
	HostConfigs []db.HostConfig `json:"host_configs"`
	// Hydrated relations are deliberately excluded from the public API models.
	// Carry them explicitly on this private, single-task channel.
	RepositoryKey          db.AccessKey      `json:"repository_key"`
	InventoryKey           db.AccessKey      `json:"inventory_key"`
	BecomeKey              db.AccessKey      `json:"become_key"`
	InventoryRepository    *db.Repository    `json:"inventory_repository"`
	InventoryRepositoryKey db.AccessKey      `json:"inventory_repository_key"`
	VaultKeys              []*db.AccessKey   `json:"vault_keys"`
	HostKeys               []db.AccessKey    `json:"host_keys"`
	JWT                    string            `json:"jwt"`
	Username               string            `json:"username"`
	IncomingVersion        *string           `json:"incoming_version"`
	Alias                  string            `json:"alias"`
	WebRoot                string            `json:"web_root"`
	EnvVars                map[string]string `json:"env_vars"`
}

type Control struct {
	Status task_logger.TaskStatus `json:"status"`
}

type Event struct {
	Type          string                 `json:"type"`
	Time          time.Time              `json:"time,omitempty"`
	Message       string                 `json:"message,omitempty"`
	Status        task_logger.TaskStatus `json:"status,omitempty"`
	CommitHash    string                 `json:"commit_hash,omitempty"`
	CommitMessage string                 `json:"commit_message,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

func (r *Request) Hydrate() {
	r.Repository.SSHKey = r.RepositoryKey
	r.Inventory.SSHKey = r.InventoryKey
	r.Inventory.BecomeKey = r.BecomeKey
	r.Inventory.Repository = r.InventoryRepository
	if r.Inventory.Repository != nil {
		r.Inventory.Repository.SSHKey = r.InventoryRepositoryKey
	}
	for i := range r.Template.Vaults {
		if i < len(r.VaultKeys) {
			r.Template.Vaults[i].Vault = r.VaultKeys[i]
		}
	}
	for i := range r.HostConfigs {
		if i < len(r.HostKeys) {
			r.HostConfigs[i].SSHKey = r.HostKeys[i]
		}
	}
}
