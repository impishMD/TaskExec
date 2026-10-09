package server

import (
	"strings"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/common_errors"
)

func validateSecretStorage(storage db.SecretStorage) error {
	if strings.TrimSpace(storage.Name) == "" {
		return common_errors.NewValidationError("storage name is required")
	}
	if storage.ProjectID <= 0 {
		return common_errors.NewValidationError("project ID is required")
	}
	if _, err := parseVaultConfig(storage); err != nil {
		return common_errors.NewUserError(err)
	}
	if storage.SourceStorageType != nil && *storage.SourceStorageType != db.AccessKeySourceStorageEnv && *storage.SourceStorageType != db.AccessKeySourceStorageFile {
		return common_errors.NewValidationError("unsupported source storage type")
	}
	return nil
}
