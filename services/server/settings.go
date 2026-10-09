package server

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/impishMD/taskexec/db"
)

const RemoteRunnersEnabledOption = "server.remote_runners.enabled"

type ServerSettings struct {
	UseRemoteRunner bool `json:"use_remote_runner"`
}

type settingsReader interface{ GetOption(string) (string, error) }
type settingsWriter interface{ SetOption(string, string) error }

// GetServerSettings reads the database on demand so saves apply to new tasks on
// every server without a restart or a process-local configuration cache.
func GetServerSettings(store settingsReader) (ServerSettings, error) {
	value, err := store.GetOption(RemoteRunnersEnabledOption)
	if errors.Is(err, db.ErrNotFound) {
		return ServerSettings{}, nil
	}
	if err != nil {
		return ServerSettings{}, fmt.Errorf("read server settings: %w", err)
	}
	switch value {
	case "", "false":
		return ServerSettings{}, nil
	case "true":
		return ServerSettings{UseRemoteRunner: true}, nil
	default:
		return ServerSettings{}, errors.New("invalid remote runner setting in the database")
	}
}

func SaveServerSettings(store settingsWriter, settings ServerSettings) error {
	return store.SetOption(RemoteRunnersEnabledOption, strconv.FormatBool(settings.UseRemoteRunner))
}
