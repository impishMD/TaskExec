package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/pkg/common_errors"
)

var errVaultNotFound = errors.New("secret not found in Vault")

const vaultMaxResponse = 2 << 20

type vaultConfig struct {
	URL                             *url.URL
	Mount, Namespace                string
	Version                         int
	AuthMethod, AuthMount, AuthRole string
	WithoutSecretID                 bool
}
type vaultClient struct {
	config      vaultConfig
	token       string
	http        *http.Client
	sessionKey  [32]byte
	credentials map[string]string
}

func vaultPath(value string, allowEmpty bool) (string, error) {
	value = strings.Trim(value, "/")
	if value == "" && allowEmpty {
		return "", nil
	}
	if value == "" || strings.ContainsAny(value, "?#%\\\r\n\x00") {
		return "", errors.New("invalid Vault path")
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." {
			return "", errors.New("invalid Vault path")
		}
	}
	return value, nil
}

func parseVaultConfig(storage db.SecretStorage) (config vaultConfig, err error) {
	if storage.Type != db.SecretStorageTypeVault {
		return config, common_errors.NewValidationError("only HashiCorp Vault secret storages are supported")
	}
	stringParam := func(name string) (string, error) {
		v, ok := storage.Params[name]
		if !ok || v == nil {
			return "", nil
		}
		s, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("%s must be a string", name)
		}
		return strings.TrimSpace(s), nil
	}
	address, err := stringParam("url")
	if err != nil {
		return config, err
	}
	u, err := url.Parse(address)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return config, common_errors.NewValidationError("Vault URL must be an HTTP(S) server URL without credentials, query or fragment")
	}
	basePath, err := vaultPath(u.Path, true)
	if err != nil {
		return config, err
	}
	basePath = strings.TrimSuffix(basePath, "/v1")
	if basePath == "v1" {
		basePath = ""
	}
	u.Path = "/" + basePath
	u.RawPath = ""
	config.URL = u
	mount, err := stringParam("mount")
	if err != nil {
		return config, err
	}
	if mount == "" {
		mount = "secret"
	}
	config.Mount, err = vaultPath(mount, false)
	if err != nil {
		return config, err
	}
	ns, err := stringParam("namespace")
	if err != nil {
		return config, err
	}
	config.Namespace, err = vaultPath(ns, true)
	if err != nil {
		return config, err
	}
	config.Version = 2
	if v, ok := storage.Params["kv_version"]; ok && v != nil {
		switch fmt.Sprint(v) {
		case "1":
			config.Version = 1
		case "2":
			config.Version = 2
		default:
			return config, common_errors.NewValidationError("KV version must be 1 or 2")
		}
	}
	config.AuthMethod, err = stringParam("auth_method")
	if err != nil {
		return config, err
	}
	if config.AuthMethod == "" {
		config.AuthMethod = "token"
	}
	switch config.AuthMethod {
	case "token", "approle", "kubernetes", "jwt", "cert":
	default:
		return config, common_errors.NewValidationError("unsupported Vault authentication method")
	}
	config.AuthMount, err = stringParam("auth_mount")
	if err != nil {
		return config, err
	}
	if config.AuthMount == "" {
		config.AuthMount = config.AuthMethod
	}
	config.AuthMount, err = vaultPath(config.AuthMount, false)
	if err != nil {
		return config, err
	}
	config.AuthRole, err = stringParam("auth_role")
	if err != nil {
		return config, err
	}
	if len(config.AuthRole) > 4096 || strings.ContainsAny(config.AuthRole, "\r\n\x00") {
		return config, common_errors.NewValidationError("invalid Vault role")
	}
	if config.AuthRole == "" && (config.AuthMethod == "kubernetes" || config.AuthMethod == "jwt") {
		return config, common_errors.NewValidationError("Vault role is required")
	}
	if v, ok := storage.Params["without_secret_id"]; ok {
		var valid bool
		config.WithoutSecretID, valid = v.(bool)
		if !valid || config.AuthMethod != "approle" {
			return config, common_errors.NewValidationError("invalid AppRole Secret ID option")
		}
	}
	if config.AuthMethod == "cert" && u.Scheme != "https" {
		return config, common_errors.NewValidationError("certificate authentication requires HTTPS")
	}

	// Credentials belong in the encrypted access key, never in public params.
	for k := range storage.Params {
		if k != "url" && k != "mount" && k != "namespace" && k != "kv_version" && k != "auth_method" && k != "auth_mount" && k != "auth_role" && k != "without_secret_id" {
			return config, common_errors.NewValidationError("unsupported Vault parameter: " + k)
		}
	}
	return config, nil
}

func newVaultClient(storage db.SecretStorage, keys db.AccessKeyManager, decryptor interface{ DeserializeSecret(*db.AccessKey) error }) (*vaultClient, error) {
	config, err := parseVaultConfig(storage)
	if err != nil {
		return nil, err
	}
	credentials, err := keys.GetAccessKeys(storage.ProjectID, db.GetAccessKeyOptions{Owner: db.AccessKeySecretStorage, StorageID: &storage.ID}, db.RetrieveQueryParams{})
	if err != nil {
		return nil, err
	}
	if len(credentials) != 1 {
		return nil, errors.New("Vault storage requires exactly one token")
	}
	stored, err := storedVaultCredentials(credentials[0], decryptor)
	if err != nil {
		return nil, err
	}
	values, err := resolveVaultCredentials(stored, decryptor)
	if err != nil {
		return nil, err
	}
	return makeVaultClient(storage, config, values)

}

func (c *vaultClient) request(ctx context.Context, method, area, secretPath string, payload any) (map[string]any, error) {
	p, err := vaultPath(secretPath, method == "LIST")
	if err != nil {
		return nil, err
	}
	u := *c.config.URL
	u.Path = strings.TrimRight(u.Path, "/") + "/v1/" + c.config.Mount + "/"
	if c.config.Version == 2 {
		u.Path += area + "/"
	}
	u.Path += p
	var body io.Reader
	if payload != nil {
		b, e := json.Marshal(payload)
		if e != nil {
			return nil, e
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, errors.New("could not create Vault request")
	}
	token, err := c.authToken(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vault-Token", token)
	if c.config.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", c.config.Namespace)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New("Vault request failed; check connectivity, TLS and server availability")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errVaultNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Vault request failed (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, vaultMaxResponse+1))
	if err != nil || len(b) > vaultMaxResponse {
		return nil, errors.New("invalid or oversized Vault response")
	}
	if len(b) == 0 {
		return nil, nil
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if err = decoder.Decode(&envelope); err != nil {
		return nil, errors.New("invalid Vault JSON response")
	}
	return envelope.Data, nil
}

func (c *vaultClient) read(ctx context.Context, p string) (data map[string]any, version int, err error) {
	data, err = c.request(ctx, "GET", "data", p, nil)
	if err != nil {
		return
	}
	if c.config.Version == 2 {
		metadata, _ := data["metadata"].(map[string]any)
		n, _ := metadata["version"].(json.Number)
		version, err = strconv.Atoi(string(n))
		if err != nil || version < 1 {
			return nil, 0, errors.New("invalid Vault secret version")
		}
		data, _ = data["data"].(map[string]any)
	}
	if data == nil {
		return nil, 0, errors.New("Vault response contains no secret data")
	}
	return
}

func (c *vaultClient) write(ctx context.Context, p string, data map[string]any, version int) error {
	var payload any = data
	if c.config.Version == 2 {
		payload = map[string]any{"data": data, "options": map[string]any{"cas": version}}
	}
	_, err := c.request(ctx, "POST", "data", p, payload)
	return err
}

func (c *vaultClient) list(ctx context.Context, p string) ([]string, error) {
	data, err := c.request(ctx, "LIST", "metadata", p, nil)
	if err != nil {
		return nil, err
	}
	values, ok := data["keys"].([]any)
	if !ok {
		return nil, errors.New("invalid Vault key list")
	}
	keys := make([]string, 0, len(values))
	for _, value := range values {
		key, ok := value.(string)
		if !ok {
			return nil, errors.New("invalid Vault key name")
		}
		clean, err := vaultPath(key, false)
		if err != nil || strings.Contains(clean, "/") {
			return nil, errors.New("invalid Vault key name")
		}
		keys = append(keys, key)
	}
	return keys, nil
}
