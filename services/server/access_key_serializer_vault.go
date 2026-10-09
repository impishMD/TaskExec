package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/random"
)

type VaultAccessKeyDeserializer struct {
	keys      db.AccessKeyManager
	storages  db.SecretStorageRepository
	decryptor interface{ DeserializeSecret(*db.AccessKey) error }
}

func NewVaultAccessKeyDeserializer(keys db.AccessKeyManager, storages db.SecretStorageRepository, decryptor interface{ DeserializeSecret(*db.AccessKey) error }) *VaultAccessKeyDeserializer {
	return &VaultAccessKeyDeserializer{keys: keys, storages: storages, decryptor: decryptor}
}

func vaultReference(key *db.AccessKey) (p, field string, err error) {
	if key.SourceStorageKey == nil || *key.SourceStorageKey == "" {
		return "", "", errors.New("Vault secret path is required")
	}
	// Paths and mappings are independent. The removed path#field syntax is rejected.
	if strings.Contains(*key.SourceStorageKey, "#") {
		return "", "", errors.New("Vault path must not contain a field selector")
	}
	p, err = vaultPath(*key.SourceStorageKey, false)
	if key.Type == db.AccessKeyString {
		field, _ = key.SourceMapping["value"].(string)
	}
	return
}

func validateVaultMapping(key *db.AccessKey) error {
	var allowed, required []string
	switch key.Type {
	case db.AccessKeyObject:
	case db.AccessKeyString:
		allowed, required = []string{"value"}, []string{"value"}
	case db.AccessKeySSH:
		allowed, required = []string{"login", "passphrase", "private_key"}, []string{"private_key"}
	case db.AccessKeyLoginPassword:
		allowed, required = []string{"login", "password"}, []string{"password"}
	default:
		return errors.New("invalid Vault reference type")
	}
	for target, value := range key.SourceMapping {
		valid := false
		for _, candidate := range allowed {
			valid = valid || target == candidate
		}
		field, ok := value.(string)
		if !valid || !ok || field == "" || len(field) > 255 || strings.ContainsAny(field, "\r\n\x00") {
			return errors.New("invalid Vault field mapping")
		}
	}
	for _, target := range required {
		if field, _ := key.SourceMapping[target].(string); field == "" {
			return errors.New("required Vault field mapping is missing")
		}
	}
	return nil
}

// resolveVaultDocument interprets all mappings from the same document version.
func resolveVaultDocument(key *db.AccessKey, data map[string]any) (string, error) {
	if err := validateVaultMapping(key); err != nil {
		return "", err
	}
	if key.Type == db.AccessKeyObject {
		encoded, err := json.Marshal(data)
		return string(encoded), err
	}
	fields := map[string]string{}
	for target, mapped := range key.SourceMapping {
		field := mapped.(string)
		value, exists := data[field]
		if !exists {
			return "", fmt.Errorf("Vault mapped field is missing: %s", field)
		}
		var str string
		var err error
		if key.Type == db.AccessKeyString {
			switch value.(type) {
			case nil, map[string]any, []any:
				return "", fmt.Errorf("Vault mapped field must be scalar: %s", field)
			}
			str, err = vaultString(value)
		} else {
			var ok bool
			str, ok = value.(string)
			if !ok {
				return "", fmt.Errorf("Vault mapped field must be a string: %s", field)
			}
		}
		if err != nil {
			return "", err
		}
		if (target == "password" || target == "private_key") && str == "" {
			return "", fmt.Errorf("Vault mapped field must not be empty: %s", field)
		}
		fields[target] = str
	}
	if key.Type == db.AccessKeyString {
		return fields["value"], nil
	}
	encoded, err := json.Marshal(fields)
	return string(encoded), err
}

func (d *VaultAccessKeyDeserializer) client(key *db.AccessKey, write bool) (*vaultClient, error) {
	if key.ProjectID == nil || key.SourceStorageID == nil {
		return nil, errors.New("Vault project and storage IDs are required")
	}
	storage, err := d.storages.GetSecretStorage(*key.ProjectID, *key.SourceStorageID)
	if err != nil {
		return nil, err
	}
	if write && (storage.ReadOnly || key.Synchronized) {
		return nil, ErrReadOnlyStorage
	}
	return newVaultClient(storage, d.keys, d.decryptor)
}

func vaultString(value any) (string, error) {
	if value == nil {
		return "", errors.New("Vault secret field is null")
	}
	if s, ok := value.(string); ok {
		return s, nil
	}
	b, err := json.Marshal(value)
	return string(b), err
}

func (d *VaultAccessKeyDeserializer) DeserializeSecret(key *db.AccessKey) (string, error) {
	p, _, err := vaultReference(key)
	if err != nil {
		return "", err
	}
	if err = validateVaultMapping(key); err != nil {
		return "", err
	}
	c, err := d.client(key, false)
	if err != nil {
		return "", err
	}
	data, _, err := c.read(context.Background(), p)
	if err != nil {
		return "", err
	}
	return resolveVaultDocument(key, data)
}

func (d *VaultAccessKeyDeserializer) SerializeSecret(key *db.AccessKey) error {
	if err := validateVaultMapping(key); err != nil {
		return err
	}
	c, err := d.client(key, true)
	if err != nil {
		return err
	}
	if key.SourceStorageKey == nil || *key.SourceStorageKey == "" {
		key.SourceStorageKey = new("taskexec/" + random.String(32))
	}
	p, _, err := vaultReference(key)
	if err != nil {
		return err
	}
	values := map[string]any{"value": key.String, "login": key.LoginPassword.Login, "password": key.LoginPassword.Password}
	if key.Type == db.AccessKeySSH {
		values = map[string]any{"login": key.SshKey.Login, "private_key": key.SshKey.PrivateKey, "passphrase": key.SshKey.Passphrase}
	}
	data, version, err := c.read(context.Background(), p)
	if errors.Is(err, errVaultNotFound) {
		data, version = map[string]any{}, 0
	} else if err != nil {
		return err
	}
	if key.Type == db.AccessKeyObject {
		for k, v := range key.Object {
			data[k] = v
		}
	} else {
		for target, field := range key.SourceMapping {
			data[field.(string)] = values[target]
		}
	}
	if err = c.write(context.Background(), p, data, version); err != nil {
		return err
	}
	key.Secret = nil
	return nil
}

func (d *VaultAccessKeyDeserializer) DeleteSecret(key *db.AccessKey) error {
	p, _, err := vaultReference(key)
	if err != nil {
		return err
	}
	if err = validateVaultMapping(key); err != nil {
		return err
	}
	c, err := d.client(key, true)
	if err != nil {
		return err
	}
	if key.Type == db.AccessKeyObject {
		_, err = c.request(context.Background(), "DELETE", "data", p, nil)
	} else {
		var data map[string]any
		var version int
		data, version, err = c.read(context.Background(), p)
		if err == nil {
			for _, field := range key.SourceMapping {
				delete(data, field.(string))
			}
			err = c.write(context.Background(), p, data, version)
		}
	}
	if errors.Is(err, errVaultNotFound) {
		return nil
	}
	return err
}
