package server

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/common_errors"
	"github.com/impishMD/taskexec/util"
)

type AccessKeyService interface {
	Update(key db.AccessKey) error
	Create(key db.AccessKey) (newKey db.AccessKey, err error)
	GetAll(projectID int, options db.GetAccessKeyOptions, params db.RetrieveQueryParams) ([]db.AccessKey, error)
	Delete(projectID int, keyID int, referenceOnly ...bool) (err error)
}

type AccessKeyServiceImpl struct {
	accessKeyRepo     db.AccessKeyManager
	encryptionService AccessKeyEncryptionService
	secretStorageRepo db.SecretStorageRepository
	hostConfigRepo    db.HostConfigManager
}

func NewAccessKeyService(
	accessKeyRepo db.AccessKeyManager,
	encryptionService AccessKeyEncryptionService,
	secretStorageRepo db.SecretStorageRepository,
	hostConfigRepo db.HostConfigManager,
) AccessKeyService {
	return &AccessKeyServiceImpl{
		accessKeyRepo:     accessKeyRepo,
		encryptionService: encryptionService,
		secretStorageRepo: secretStorageRepo,
		hostConfigRepo:    hostConfigRepo,
	}
}

// hostConfigsUsing returns the credential mappings of the project which point at
// the key. The repository rejects a change breaking one of them too, but only
// once the secret storage has already been written or emptied, which is too
// late to undo.
func (s *AccessKeyServiceImpl) hostConfigsUsing(projectID *int, keyID int) (used []db.HostConfig, err error) {
	if s.hostConfigRepo == nil || projectID == nil {
		return
	}

	hostConfigs, err := s.hostConfigRepo.GetHostConfigs(*projectID, db.RetrieveQueryParams{})
	if err != nil {
		return
	}

	for _, hostConfig := range hostConfigs {
		if hostConfig.SSHKeyID == keyID {
			used = append(used, hostConfig)
		}
	}

	return
}

func (s *AccessKeyServiceImpl) Delete(projectID int, keyID int, referenceOnly ...bool) (err error) {
	key, err := s.accessKeyRepo.GetAccessKey(projectID, keyID)
	if err != nil {
		return
	}

	// Checked here rather than left to the repository: the secret is removed
	// from its storage below, and a deletion refused after that has destroyed
	// the credential while leaving the mapping pointing at it.
	used, err := s.hostConfigsUsing(&projectID, keyID)
	if err != nil {
		return
	}

	if len(used) > 0 {
		err = common_errors.NewValidationError(
			"the credential is used by the mapping for " + used[0].Name)
		return
	}

	refs, err := s.accessKeyRepo.GetAccessKeyRefs(projectID, key.ID)
	if err != nil {
		return err
	}
	if len(refs.Environments) > 0 {
		return db.ErrInvalidOperation
	}
	if key.SourceStorageID != nil {
		var storage db.SecretStorage
		storage, err = s.secretStorageRepo.GetSecretStorage(projectID, *key.SourceStorageID)
		if err != nil {
			return
		}

		if storage.ReadOnly || key.Synchronized || (len(referenceOnly) > 0 && referenceOnly[0]) {
			// Do nothing

			//if key.Synchronized {
			//	err = common_errors.NewUserErrorS("cannot delete synchronized secret from read-only storage")
			//}
		} else {
			refs, e := s.accessKeyRepo.GetAccessKeyRefs(projectID, key.ID)
			if e != nil {
				return e
			}
			if len(refs.Templates)+len(refs.Inventories)+len(refs.Repositories)+len(refs.Integrations)+len(refs.Schedules)+len(refs.AccessKeys)+len(refs.HostConfigs) > 0 {
				return db.ErrInvalidOperation
			}
			err = s.encryptionService.DeleteSecret(&key)
		}

		if err != nil {
			return
		}
	}

	err = s.accessKeyRepo.DeleteAccessKey(projectID, keyID)

	return
}

func (s *AccessKeyServiceImpl) GetAll(projectID int, options db.GetAccessKeyOptions, params db.RetrieveQueryParams) ([]db.AccessKey, error) {
	return s.accessKeyRepo.GetAccessKeys(projectID, options, params)
}

// generateSSHKeyPair returns a new private key in PEM format and the matching
// public key in OpenSSH authorized_keys format.
func generateSSHKeyPair() (privateKey string, publicKey string, err error) {
	var buf bytes.Buffer

	publicKey, err = util.GeneratePrivateKey(&buf)
	if err != nil {
		return
	}

	privateKey = buf.String()
	return
}

// encodePublicKeyPlain builds the JSON document stored in the non-secret
// "plain" column so the UI can show the public key.
func encodePublicKeyPlain(publicKey string) (string, error) {
	doc := struct {
		PublicKey string `json:"public_key"`
	}{PublicKey: publicKey}

	b, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// assignGeneratedSSHKey replaces the key's secret with a freshly generated
// SSH key pair and exposes the public half through the plain field.
func assignGeneratedSSHKey(key *db.AccessKey) error {
	if key.Type != db.AccessKeySSH {
		return common_errors.NewUserErrorS("generate_ssh_key is only allowed for ssh keys")
	}

	privateKey, publicKey, err := generateSSHKeyPair()
	if err != nil {
		return err
	}

	plain, err := encodePublicKeyPlain(publicKey)
	if err != nil {
		return err
	}

	key.SshKey.PrivateKey = privateKey
	key.SshKey.Passphrase = ""
	key.Plain = &plain
	key.IgnorePlain = false

	return nil
}

func (s *AccessKeyServiceImpl) Create(key db.AccessKey) (newKey db.AccessKey, err error) {
	// Plain is derived data, never taken from the caller.
	key.Plain = nil
	if key.ReferenceOnly {
		if err = s.validateVaultReference(key); err != nil {
			return
		}
		key.Secret = nil
		return s.accessKeyRepo.CreateAccessKey(key)
	}

	if key.GenerateSSHKey {
		err = assignGeneratedSSHKey(&key)
		if err != nil {
			return
		}
	}

	// SerializeSecret encrypts/persists the secret for writable backends. For read-only
	// external storage the secret is not stored in TaskExec, so SerializeSecret fails
	// with ErrReadOnlyStorage; we still create the access key row (metadata / reference).
	// A generated key is the exception: nobody else holds the private half, so
	// a storage that cannot persist it must reject the request.
	err = s.encryptionService.SerializeSecret(&key)
	if err != nil && (key.GenerateSSHKey || !errors.Is(err, ErrReadOnlyStorage)) {
		return
	}

	if errors.Is(err, ErrReadOnlyStorage) && key.SourceStorageType != nil && *key.SourceStorageType == db.AccessKeySourceStorageVault {
		if _, _, err = vaultReference(&key); err != nil {
			return
		}
	}
	newKey, err = s.accessKeyRepo.CreateAccessKey(key)
	return
}

func (s *AccessKeyServiceImpl) Update(key db.AccessKey) (err error) {
	// Plain is derived data, never taken from the caller.
	key.Plain = nil
	if key.ReferenceOnly {
		if err = s.validateVaultReference(key); err != nil {
			return err
		}
		old, err := s.accessKeyRepo.GetAccessKey(*key.ProjectID, key.ID)
		if err != nil {
			return err
		}
		if old.Owner == db.AccessKeyEnvironment && key.Type != db.AccessKeyString {
			return common_errors.NewValidationError("environment secrets must use string type")
		}
		// OverrideSecret is the repository's update-column switch. Do not call the
		// serializer: only type/reference metadata changes, including for read-only
		// connections. Host mapping constraints are checked by the repository.
		key.OverrideSecret = true
		key.Secret = nil
		key.IgnorePlain = false
		return s.accessKeyRepo.UpdateAccessKey(key)
	}

	if !key.OverrideSecret {
		err = s.accessKeyRepo.UpdateAccessKey(key)
		return
	}

	if key.GenerateSSHKey && key.IsNativelyReadOnly() {
		// Env/file sources are never written, so a generated private key
		// would have nowhere to live. Read-only vaults are rejected later
		// by SerializeSecret.
		err = common_errors.NewUserError(ErrReadOnlyStorage)
		return
	}

	var oldKey db.AccessKey
	oldKey, err = s.accessKeyRepo.GetAccessKey(*key.ProjectID, key.ID)
	if err != nil {
		return
	}

	if oldKey.Synchronized {
		return common_errors.NewUserError(ErrReadOnlyStorage)
	}
	// Switching away from Vault changes a reference; it does not move or delete
	// the old remote value. Only remote-to-remote writes retain the old guard.
	if oldKey.SourceStorageType != nil && !oldKey.IsNativelyReadOnly() &&
		key.SourceStorageType != nil && *key.SourceStorageType == db.AccessKeySourceStorageVault {
		// validate if it is secure to override secret storage

		var oldSt db.SecretStorage
		oldSt, err = s.secretStorageRepo.GetSecretStorage(*key.ProjectID, *oldKey.SourceStorageID)
		if err != nil {
			return
		}

		if !oldSt.ReadOnly && (key.SourceStorageID == nil || *oldKey.SourceStorageID != *key.SourceStorageID) {
			err = common_errors.NewUserErrorS("cannot override secret storage")
			return
		}
	}
	if key.SourceStorageType == nil {
		key.SourceStorageID, key.SourceStorageKey, key.SourceMapping = nil, nil, nil
	} else if key.IsNativelyReadOnly() {
		key.SourceStorageID = nil
		key.SourceMapping = nil
	}
	if (oldKey.SourceStorageType == nil) != (key.SourceStorageType == nil) ||
		(oldKey.SourceStorageType != nil && key.SourceStorageType != nil && *oldKey.SourceStorageType != *key.SourceStorageType) {
		key.IgnorePlain = false
	}

	// Before the secret is written to its storage: a type the mappings can not
	// use is rejected by the repository, but by then the remote secret has
	// already been overwritten with a payload the old row can not deserialize.
	var used []db.HostConfig
	used, err = s.hostConfigsUsing(key.ProjectID, key.ID)
	if err != nil {
		return
	}

	for _, hostConfig := range used {
		if err = hostConfig.ValidateCredential(key.Type); err != nil {
			return
		}
	}

	if key.GenerateSSHKey {
		err = assignGeneratedSSHKey(&key)
		if err != nil {
			return
		}
	}

	if !key.IsNativelyReadOnly() {
		err = s.encryptionService.SerializeSecret(&key)
		if err != nil {
			return
		}
	}

	err = s.accessKeyRepo.UpdateAccessKey(key)

	return
}

func (s *AccessKeyServiceImpl) validateVaultReference(key db.AccessKey) error {
	if key.SourceStorageType == nil || *key.SourceStorageType != db.AccessKeySourceStorageVault ||
		key.SourceStorageID == nil || key.ProjectID == nil {
		return common_errors.NewValidationError("vault storage id is required")
	}
	if key.GenerateSSHKey || key.OverrideSecret || key.String != "" || key.Object != nil ||
		key.SshKey != (db.SshKey{}) || key.LoginPassword != (db.LoginPassword{}) {
		return common_errors.NewValidationError("reference-only key cannot contain secret values")
	}
	if key.Type != db.AccessKeySSH && key.Type != db.AccessKeyLoginPassword && key.Type != db.AccessKeyString && key.Type != db.AccessKeyObject {
		return common_errors.NewValidationError("invalid Vault reference type")
	}
	if err := key.Validate(false); err != nil {
		return common_errors.NewValidationError(err.Error())
	}
	if _, _, err := vaultReference(&key); err != nil {
		return common_errors.NewValidationError(err.Error())
	}
	if err := validateVaultMapping(&key); err != nil {
		return common_errors.NewValidationError(err.Error())
	}
	storage, err := s.secretStorageRepo.GetSecretStorage(*key.ProjectID, *key.SourceStorageID)
	if err != nil {
		return err
	}
	if storage.Type != db.SecretStorageTypeVault {
		return common_errors.NewValidationError("invalid Vault reference type")
	}
	return nil
}
