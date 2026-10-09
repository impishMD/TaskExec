package server

import (
	"context"
	"errors"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/common_errors"
	"github.com/impishMD/taskexec/pkg/random"
)

type SecretStorageService interface {
	GetSecretStorage(projectID int, storageID int) (db.SecretStorage, error)
	Update(storage db.SecretStorage) error
	Delete(projectID int, storageID int) error
	GetSecretStorages(projectID int) ([]db.SecretStorage, error)
	Create(storage db.SecretStorage) (res db.SecretStorage, err error)
	TestVaultAuthentication(context.Context, db.SecretStorage) error
	DescribeVaultSecret(context.Context, int, int, string) ([]VaultField, error)
	ListVaultSecrets(context.Context, int, int, string) ([]string, error)
}

func NewSecretStorageService(
	secretStorageRepo db.SecretStorageRepository,
	accessKeyRepo db.AccessKeyManager,
	accessKeyService AccessKeyService,
	encryptionService AccessKeyEncryptionService,
) SecretStorageService {
	return &SecretStorageServiceImpl{
		secretStorageRepo: secretStorageRepo,
		accessKeyRepo:     accessKeyRepo,
		accessKeyService:  accessKeyService,
		encryptionService: encryptionService,
	}
}

type SecretStorageServiceImpl struct {
	secretStorageRepo db.SecretStorageRepository
	accessKeyRepo     db.AccessKeyManager
	accessKeyService  AccessKeyService
	encryptionService AccessKeyEncryptionService
}

func (s *SecretStorageServiceImpl) Delete(projectID int, storageID int) (err error) {
	_, err = s.secretStorageRepo.GetSecretStorage(projectID, storageID)
	if err != nil {
		return
	}

	// Detaching a storage must never remove credentials that other resources
	// still use, nor delete data in the external provider.
	if s.accessKeyRepo != nil {
		keys, e := s.accessKeyRepo.GetAccessKeys(projectID, db.GetAccessKeyOptions{IgnoreOwner: true, SourceStorageID: &storageID}, db.RetrieveQueryParams{})
		if e != nil {
			return e
		}
		if len(keys) > 0 {
			return common_errors.NewUserErrorS("remove the keys referencing this storage first")
		}
	}
	if environments, ok := s.secretStorageRepo.(db.EnvironmentManager); ok {
		envs, e := environments.GetEnvironments(projectID, db.RetrieveQueryParams{})
		if e != nil {
			return e
		}
		for _, env := range envs {
			if env.SecretStorageID != nil && *env.SecretStorageID == storageID {
				return common_errors.NewUserErrorS("remove the variable groups referencing this storage first")
			}
		}
	}

	// Deleted before the storage row: MySQL 8 ignores the inline access_key.storage_id cascade.
	keys, err := s.accessKeyService.GetAll(projectID, db.GetAccessKeyOptions{
		Owner:     db.AccessKeySecretStorage,
		StorageID: &storageID,
	}, db.RetrieveQueryParams{})
	if err != nil {
		return
	}

	for _, key := range keys {
		if err = s.accessKeyService.Delete(projectID, key.ID); err != nil && !errors.Is(err, db.ErrNotFound) {
			return
		}
	}

	return s.secretStorageRepo.DeleteSecretStorage(projectID, storageID)
}

func (s *SecretStorageServiceImpl) GetSecretStorage(projectID int, storageID int) (res db.SecretStorage, err error) {
	res, err = s.secretStorageRepo.GetSecretStorage(projectID, storageID)
	if err != nil || (res.Type != db.SecretStorageTypeVault) {
		return
	}
	key, e := s.credentialKey(res)
	if e != nil {
		return res, e
	}
	credentials, e := storedVaultCredentials(key, s.encryptionService)
	if e != nil {
		return res, e
	}
	res.Credentials = publicVaultCredentials(credentials)
	if key.SourceStorageType != nil && key.SourceStorageKey != nil {
		res.Secret, res.SourceStorageType = *key.SourceStorageKey, key.SourceStorageType
	}
	return
}

func (s *SecretStorageServiceImpl) Create(storage db.SecretStorage) (res db.SecretStorage, err error) {
	storage.ID = 0
	if storage.Credentials != nil || (storage.Params["auth_method"] != nil && storage.Params["auth_method"] != "token") {
		return s.saveVaultStorage(storage)
	}
	if err = validateSecretStorage(storage); err != nil {
		return
	}
	sourceStorageType := storage.SourceStorageType
	sourceStorageKey := ""

	if storage.Secret == "" {
		err = common_errors.NewUserErrorS("secret must be set")
		return
	}

	if sourceStorageType != nil {
		switch *sourceStorageType {
		case db.AccessKeySourceStorageEnv:
			sourceStorageKey = storage.Secret
		case db.AccessKeySourceStorageFile:
			sourceStorageKey = storage.Secret
		default:
			err = common_errors.NewUserErrorS("unsupported source storage type")
			return
		}
	}

	res, err = s.secretStorageRepo.CreateSecretStorage(storage)

	if err != nil {
		return
	}

	key := db.AccessKey{
		Name:              random.String(10),
		Type:              db.AccessKeyString,
		ProjectID:         &storage.ProjectID,
		Owner:             db.AccessKeySecretStorage,
		StorageID:         &res.ID,
		SourceStorageType: sourceStorageType,
	}

	if sourceStorageKey != "" {
		key.SourceStorageKey = &sourceStorageKey
	} else {
		key.String = storage.Secret
	}

	_, err = s.accessKeyService.Create(key)
	if err != nil {
		err = errors.Join(err, s.secretStorageRepo.DeleteSecretStorage(storage.ProjectID, res.ID))
	}
	res.Secret = ""
	return
}

func (s *SecretStorageServiceImpl) Update(storage db.SecretStorage) (err error) {
	if storage.SourceStorageType != nil && *storage.SourceStorageType != db.AccessKeySourceStorageEnv && *storage.SourceStorageType != db.AccessKeySourceStorageFile {
		return common_errors.NewValidationError("unsupported source storage type")
	}
	if storage.Credentials != nil || (storage.Params["auth_method"] != nil && storage.Params["auth_method"] != "token") {
		_, err = s.saveVaultStorage(storage)
		return
	}
	// Preserve structured credentials when a legacy client edits only metadata.
	if s.accessKeyService != nil {
		key, e := s.credentialKey(storage)
		if e == nil && isVaultAuthKey(key) {
			_, err = s.saveVaultStorage(storage)
			return
		}
	}
	sourceStorageType := storage.SourceStorageType
	sourceStorageKey := ""

	// Checked before the write, so a refused source leaves the storage unchanged.
	if sourceStorageType != nil {
		switch *sourceStorageType {
		case db.AccessKeySourceStorageEnv, db.AccessKeySourceStorageFile:
			sourceStorageKey = storage.Secret
		default:
			err = common_errors.NewUserErrorS("unsupported source storage type")
			return
		}
	}

	if err = validateSecretStorage(storage); err != nil {
		return
	}
	old, err := s.secretStorageRepo.GetSecretStorage(storage.ProjectID, storage.ID)
	if err != nil {
		return
	}
	if old.Type != storage.Type {
		return common_errors.NewValidationError("cannot change a secret storage provider")
	}
	keys, err := s.accessKeyService.GetAll(storage.ProjectID, db.GetAccessKeyOptions{
		Owner:     db.AccessKeySecretStorage,
		StorageID: &storage.ID,
	}, db.RetrieveQueryParams{})

	if err != nil {
		return
	}
	if storage.Secret == "" {
		if len(keys) != 1 {
			return common_errors.NewValidationError("a token or token source is required")
		}
		previousSource := keys[0].SourceStorageType
		if (previousSource == nil) != (sourceStorageType == nil) || (previousSource != nil && sourceStorageType != nil && *previousSource != *sourceStorageType) {
			return common_errors.NewValidationError("a new token or reference is required when changing its source")
		}
	}
	if err = s.secretStorageRepo.UpdateSecretStorage(storage); err != nil {
		return
	}

	if len(keys) == 0 {
		newKey := db.AccessKey{
			Name:              random.String(10),
			Type:              db.AccessKeyString,
			ProjectID:         &storage.ProjectID,
			Owner:             db.AccessKeySecretStorage,
			StorageID:         &storage.ID,
			SourceStorageType: sourceStorageType,
		}

		if sourceStorageKey != "" {
			newKey.SourceStorageKey = &sourceStorageKey
		} else {
			newKey.String = storage.Secret
		}

		_, err = s.accessKeyService.Create(newKey)

	} else {
		vault := keys[0]
		if storage.Secret == "" {
			// A blank field preserves the saved token or reference.
			return
		}

		vault.OverrideSecret = true
		vault.SourceStorageType = sourceStorageType
		if sourceStorageKey != "" {
			vault.SourceStorageKey = &sourceStorageKey
			vault.String = ""
			// Clear previously persisted encrypted secret when switching to env/file source.
			vault.Secret = nil
		} else {
			vault.SourceStorageKey = nil
			vault.String = storage.Secret
		}

		err = s.accessKeyService.Update(vault)
	}

	return
}

func (s *SecretStorageServiceImpl) GetSecretStorages(projectID int) (storages []db.SecretStorage, err error) {
	return s.secretStorageRepo.GetSecretStorages(projectID)
}
