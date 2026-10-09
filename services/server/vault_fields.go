package server

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/impishMD/taskexec/db"
)

type VaultField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ListVaultSecrets lists one directory, without reading any secret values.
func (s *SecretStorageServiceImpl) ListVaultSecrets(ctx context.Context, projectID, storageID int, path string) ([]string, error) {
	p, err := vaultPath(path, true)
	if err != nil {
		return nil, err
	}
	storage, err := s.secretStorageRepo.GetSecretStorage(projectID, storageID)
	if err != nil {
		return nil, err
	}
	c, err := newVaultClient(storage, s.accessKeyRepo, s.encryptionService)
	if err != nil {
		return nil, err
	}
	names, err := c.list(ctx, p)
	if errors.Is(err, errVaultNotFound) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, strings.TrimPrefix(p+"/"+name, "/"))
	}
	sort.Strings(paths)
	return paths, nil
}

func (s *SecretStorageServiceImpl) DescribeVaultSecret(ctx context.Context, projectID, storageID int, path string) ([]VaultField, error) {
	key := db.AccessKey{SourceStorageKey: &path, Type: db.AccessKeyObject}
	p, _, err := vaultReference(&key)
	if err != nil {
		return nil, err
	}
	storage, err := s.secretStorageRepo.GetSecretStorage(projectID, storageID)
	if err != nil {
		return nil, err
	}
	c, err := newVaultClient(storage, s.accessKeyRepo, s.encryptionService)
	if err != nil {
		return nil, err
	}
	data, _, err := c.read(ctx, p)
	if err != nil {
		return nil, err
	}
	fields := make([]VaultField, 0, len(data))
	for name, value := range data {
		kind := "number"
		switch value.(type) {
		case nil:
			kind = "null"
		case string:
			kind = "string"
		case bool:
			kind = "boolean"
		case []any:
			kind = "array"
		case map[string]any:
			kind = "object"
		}
		fields = append(fields, VaultField{Name: name, Type: kind})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	return fields, nil
}
