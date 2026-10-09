package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/common_errors"
	"github.com/impishMD/taskexec/pkg/random"
)

const vaultAuthKeyName = "vault-auth-v1"

func isVaultAuthKey(key db.AccessKey) bool {
	return key.Name == vaultAuthKeyName || strings.HasPrefix(key.Name, vaultAuthKeyName+":")
}

type vaultDecryptor interface{ DeserializeSecret(*db.AccessKey) error }

func vaultCredentialFields(method string) []string {
	switch method {
	case "approle":
		return []string{"role_id", "secret_id"}
	case "kubernetes", "jwt":
		return []string{"jwt"}
	case "cert":
		return []string{"client_cert", "client_key"}
	default:
		return []string{"token"}
	}
}

func storedVaultCredentials(key db.AccessKey, decryptor vaultDecryptor) (map[string]db.VaultCredential, error) {
	if key.Type != db.AccessKeyString {
		return nil, errors.New("invalid Vault credential type")
	}
	if !isVaultAuthKey(key) {
		credential := db.VaultCredential{Source: "database"}
		if key.SourceStorageType != nil {
			if key.SourceStorageKey == nil {
				return nil, errors.New("missing Vault credential reference")
			}
			credential.Source, credential.Value = string(*key.SourceStorageType), *key.SourceStorageKey
		} else {
			if err := decryptor.DeserializeSecret(&key); err != nil {
				return nil, errors.New("could not decrypt Vault credentials")
			}
			credential.Value = key.String
		}
		return map[string]db.VaultCredential{"token": credential}, nil
	}
	if key.SourceStorageType != nil {
		return nil, errors.New("invalid Vault credential bundle")
	}
	if err := decryptor.DeserializeSecret(&key); err != nil {
		return nil, errors.New("could not decrypt Vault credentials")
	}
	// Project exports intentionally omit secret values; allow restored connections to be edited.
	if key.String == "" {
		return map[string]db.VaultCredential{}, nil
	}
	var credentials map[string]db.VaultCredential
	if json.Unmarshal([]byte(key.String), &credentials) != nil || credentials == nil {
		return nil, errors.New("invalid Vault credential bundle")
	}
	return credentials, nil
}

func publicVaultCredentials(credentials map[string]db.VaultCredential) map[string]db.VaultCredential {
	result := make(map[string]db.VaultCredential, len(credentials))
	for name, credential := range credentials {
		credential.Configured = credential.Value != ""
		credential.Clear = false
		if credential.Source == "database" {
			credential.Value = ""
		}
		result[name] = credential
	}
	return result
}

var vaultEnvName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func mergeVaultCredentials(config vaultConfig, supplied, previous map[string]db.VaultCredential) (map[string]db.VaultCredential, error) {
	required := vaultCredentialFields(config.AuthMethod)
	allowed := map[string]bool{"ca_cert": true}
	for _, field := range required {
		allowed[field] = true
	}
	result := map[string]db.VaultCredential{}
	for field := range supplied {
		if !allowed[field] {
			return nil, common_errors.NewValidationError("unexpected Vault credential field")
		}
	}
	for field := range allowed {
		credential, provided := supplied[field]
		old := previous[field]
		if !provided {
			credential = old
		}
		if credential.Source == "" {
			credential.Source = "database"
		}
		if credential.Clear {
			credential.Value = ""
		} else if credential.Value == "" && old.Source == credential.Source {
			credential.Value = old.Value
		}
		credential.Configured, credential.Clear = false, false
		if credential.Value == "" {
			if field == "ca_cert" {
				continue
			}
			// Explicitly opt in to AppRoles configured with bind_secret_id=false.
			if field == "secret_id" && config.WithoutSecretID {
				continue
			}
			return nil, common_errors.NewValidationError("Vault credential is required: " + field)
		}
		if len(credential.Value) > 256*1024 {
			return nil, common_errors.NewValidationError("Vault credential is too large")
		}
		switch credential.Source {
		case "database":
			if field != "client_cert" && field != "client_key" && field != "ca_cert" && strings.ContainsAny(credential.Value, "\r\n\x00") {
				return nil, common_errors.NewValidationError("invalid Vault credential: " + field)
			}
		case "env":
			if !vaultEnvName.MatchString(credential.Value) {
				return nil, common_errors.NewValidationError("invalid environment variable name")
			}
		case "file":
			if !filepath.IsAbs(credential.Value) || strings.ContainsAny(credential.Value, "\r\n\x00") {
				return nil, common_errors.NewValidationError("credential file path must be absolute")
			}
		default:
			return nil, common_errors.NewValidationError("unsupported credential source")
		}
		result[field] = credential
	}
	return result, nil
}

func resolveVaultCredentials(credentials map[string]db.VaultCredential, decryptor vaultDecryptor) (map[string]string, error) {
	result := map[string]string{}
	for name, credential := range credentials {
		value := credential.Value
		if credential.Source != "database" && credential.Source != "" {
			if credential.Source != "env" && credential.Source != "file" {
				return nil, errors.New("invalid Vault credential source")
			}
			source := db.AccessKeySourceStorageType(credential.Source)
			key := db.AccessKey{Type: db.AccessKeyString, SourceStorageType: &source, SourceStorageKey: &credential.Value}
			if err := decryptor.DeserializeSecret(&key); err != nil {
				return nil, fmt.Errorf("could not read Vault credential: %s", name)
			}
			value = key.String
		}
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 256*1024 {
			return nil, fmt.Errorf("Vault credential is empty or too large: %s", name)
		}
		if name != "client_cert" && name != "client_key" && name != "ca_cert" && strings.ContainsAny(value, "\r\n\x00") {
			return nil, fmt.Errorf("invalid Vault credential: %s", name)
		}
		result[name] = value
	}
	return result, nil
}

func (s *SecretStorageServiceImpl) credentialKey(storage db.SecretStorage) (db.AccessKey, error) {
	keys, err := s.accessKeyService.GetAll(storage.ProjectID, db.GetAccessKeyOptions{Owner: db.AccessKeySecretStorage, StorageID: &storage.ID}, db.RetrieveQueryParams{})
	if err != nil {
		return db.AccessKey{}, err
	}
	if len(keys) != 1 {
		return db.AccessKey{}, errors.New("Vault storage requires exactly one credential bundle")
	}
	return keys[0], nil
}

// PrepareVaultStorage merges masked edits without ever exposing saved secrets.
func (s *SecretStorageServiceImpl) prepareVaultStorage(storage db.SecretStorage) (db.SecretStorage, db.AccessKey, error) {
	if err := validateSecretStorage(storage); err != nil {
		return storage, db.AccessKey{}, err
	}
	config, _ := parseVaultConfig(storage)
	previous := map[string]db.VaultCredential{}
	var key db.AccessKey
	if storage.ID != 0 {
		old, err := s.secretStorageRepo.GetSecretStorage(storage.ProjectID, storage.ID)
		if err != nil {
			return storage, key, err
		}
		oldConfig, err := parseVaultConfig(old)
		if err != nil {
			return storage, key, err
		}
		key, err = s.credentialKey(old)
		if err != nil {
			return storage, key, err
		}
		saved, e := storedVaultCredentials(key, s.encryptionService)
		if e != nil {
			return storage, key, e
		}
		if oldConfig.AuthMethod == config.AuthMethod {
			previous = saved
		} else if ca, ok := saved["ca_cert"]; ok {
			previous["ca_cert"] = ca
		}

	}
	supplied := storage.Credentials
	// Older API clients can continue sending the single token fields.
	if supplied == nil && config.AuthMethod == "token" {
		source := "database"
		if storage.SourceStorageType != nil {
			source = string(*storage.SourceStorageType)
		}
		supplied = map[string]db.VaultCredential{"token": {Source: source, Value: storage.Secret}}
	}
	credentials, err := mergeVaultCredentials(config, supplied, previous)
	if err != nil {
		return storage, key, err
	}
	encoded, err := json.Marshal(credentials)
	if err != nil {
		return storage, key, err
	}
	name := key.Name
	if !isVaultAuthKey(key) {
		name = vaultAuthKeyName + ":" + random.String(16)
	}
	key = db.AccessKey{ID: key.ID, Name: name, Type: db.AccessKeyString, ProjectID: &storage.ProjectID, Owner: db.AccessKeySecretStorage, StorageID: &storage.ID, String: string(encoded), OverrideSecret: true}
	if err = s.encryptionService.SerializeSecret(&key); err != nil {
		return storage, key, err
	}
	storage.Type = db.SecretStorageTypeVault
	storage.Secret, storage.SourceStorageType, storage.Credentials = "", nil, nil
	return storage, key, nil
}

type vaultStorageWriter interface {
	SaveVaultStorage(db.SecretStorage, db.AccessKey) (db.SecretStorage, error)
}

func (s *SecretStorageServiceImpl) saveVaultStorage(storage db.SecretStorage) (db.SecretStorage, error) {
	prepared, key, err := s.prepareVaultStorage(storage)
	if err != nil {
		return db.SecretStorage{}, err
	}
	writer, ok := s.secretStorageRepo.(vaultStorageWriter)
	if !ok {
		return db.SecretStorage{}, errors.New("storage repository does not support atomic Vault credentials")
	}
	return writer.SaveVaultStorage(prepared, key)
}
