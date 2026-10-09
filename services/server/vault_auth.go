package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/impishMD/taskexec/db"
)

type vaultSession struct {
	mu                        sync.Mutex
	token                     string
	expires, renewAt, retryAt time.Time
	renewable                 bool
}

var vaultSessions = struct {
	sync.Mutex
	entries map[[32]byte]*vaultSession
}{entries: make(map[[32]byte]*vaultSession)}

func makeVaultClient(storage db.SecretStorage, config vaultConfig, values map[string]string) (*vaultClient, error) {
	for _, field := range vaultCredentialFields(config.AuthMethod) {
		if field == "secret_id" && config.WithoutSecretID {
			continue
		}
		if values[field] == "" {
			return nil, fmt.Errorf("Vault credential is required: %s", field)
		}
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if ca := values["ca_cert"]; ca != "" {
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM([]byte(ca)) {
			return nil, errors.New("invalid Vault CA certificate")
		}
		tlsConfig.RootCAs = roots
	}
	if config.AuthMethod == "cert" {
		certificate, err := tls.X509KeyPair([]byte(values["client_cert"]), []byte(values["client_key"]))
		if err != nil {
			return nil, errors.New("invalid Vault client certificate or private key")
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	// Clients are short-lived (one secret operation or synchronization); avoid
	// leaving idle transports/goroutines around on every credential read.
	transport.DisableKeepAlives = true
	encoded, _ := json.Marshal(struct {
		Project, Storage int
		Config           vaultConfig
		Credentials      map[string]string
	}{storage.ProjectID, storage.ID, config, values})
	return &vaultClient{config: config, token: values["token"], credentials: values, sessionKey: sha256.Sum256(encoded), http: &http.Client{
		Timeout: 15 * time.Second, Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

type vaultAuthResponse struct {
	Auth *struct {
		Token     string `json:"client_token"`
		TTL       int64  `json:"lease_duration"`
		Renewable bool   `json:"renewable"`
	} `json:"auth"`
	Data map[string]any `json:"data"`
}

type vaultHTTPError int

func (e vaultHTTPError) Error() string {
	return fmt.Sprintf("Vault authentication failed (HTTP %d)", e)
}

func (c *vaultClient) authRequest(ctx context.Context, path, token string, body any) (vaultAuthResponse, error) {
	var result vaultAuthResponse
	encoded, err := json.Marshal(body)
	if err != nil {
		return result, err
	}
	u := *c.config.URL
	u.Path = strings.TrimRight(u.Path, "/") + "/v1/" + path
	method := http.MethodPost
	if body == nil {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(encoded))
	if err != nil {
		return result, errors.New("could not create Vault authentication request")
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Vault-Token", token)
	}
	if c.config.Namespace != "" {
		req.Header.Set("X-Vault-Namespace", c.config.Namespace)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return result, errors.New("Vault connection failed; check connectivity and TLS certificates")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return result, vaultHTTPError(response.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, vaultMaxResponse+1))
	if err != nil || len(content) > vaultMaxResponse || json.Unmarshal(content, &result) != nil {
		return result, errors.New("invalid Vault authentication response")
	}
	return result, nil
}

func (c *vaultClient) authToken(ctx context.Context) (string, error) {
	if c.config.AuthMethod == "token" {
		return c.token, nil
	}
	vaultSessions.Lock()
	session := vaultSessions.entries[c.sessionKey]
	if session == nil {
		// Bound memory, including credentials configured for connection tests.
		if len(vaultSessions.entries) >= 256 {
			for key, candidate := range vaultSessions.entries {
				if candidate.mu.TryLock() {
					expired := time.Now().After(candidate.expires)
					candidate.mu.Unlock()
					if expired {
						delete(vaultSessions.entries, key)
					}
				}
			}
			if len(vaultSessions.entries) >= 256 {
				vaultSessions.Unlock()
				return "", errors.New("Vault authentication cache is full; retry after existing sessions expire")
			}
		}
		session = &vaultSession{}
		vaultSessions.entries[c.sessionKey] = session
	}
	vaultSessions.Unlock()
	session.mu.Lock()
	defer session.mu.Unlock()
	now := time.Now()
	if session.token != "" && now.Before(session.expires) {
		if !session.renewable || now.Before(session.renewAt) || now.Before(session.retryAt) {
			return session.token, nil
		}
		response, err := c.authRequest(ctx, "auth/token/renew-self", session.token, map[string]any{})
		if err == nil {
			if err = session.accept(response); err == nil {
				return session.token, nil
			}
		}
		var status vaultHTTPError
		if !errors.As(err, &status) || (status != 400 && status != 403) {
			session.retryAt = time.Now().Add(5 * time.Second)
			if time.Now().Before(session.expires) {
				return session.token, nil
			}
			return "", err
		}
		// The token may have reached max TTL; use it until expiry, then log in again.
		session.renewable = false
		return session.token, nil
	}
	if now.Before(session.retryAt) {
		return "", errors.New("Vault authentication temporarily unavailable; retry shortly")
	}
	payload := map[string]any{}
	switch c.config.AuthMethod {
	case "approle":
		payload["role_id"] = c.credentials["role_id"]
		if secret := c.credentials["secret_id"]; secret != "" {
			payload["secret_id"] = secret
		}
	case "kubernetes", "jwt":
		payload["role"], payload["jwt"] = c.config.AuthRole, c.credentials["jwt"]
	case "cert":
		if c.config.AuthRole != "" {
			payload["name"] = c.config.AuthRole
		}
	}
	response, err := c.authRequest(ctx, "auth/"+c.config.AuthMount+"/login", "", payload)
	if err == nil {
		err = session.accept(response)
	}
	if err != nil {
		session.retryAt = time.Now().Add(time.Second)
		return "", err
	}
	return session.token, nil
}

func (s *vaultSession) accept(response vaultAuthResponse) error {
	a := response.Auth
	if a == nil || a.Token == "" || strings.ContainsAny(a.Token, "\r\n\x00") || a.TTL <= 0 || a.TTL > 315360000 {
		return errors.New("Vault login returned an invalid token or lifetime")
	}
	ttl := time.Duration(a.TTL) * time.Second
	s.token, s.renewable = a.Token, a.Renewable
	s.expires, s.renewAt, s.retryAt = time.Now().Add(ttl), time.Now().Add(ttl*2/3), time.Time{}
	return nil
}

// TestVaultAuthentication never persists the draft or writes secret data.
func (s *SecretStorageServiceImpl) TestVaultAuthentication(ctx context.Context, storage db.SecretStorage) error {
	prepared, key, err := s.prepareVaultStorage(storage)
	if err != nil {
		return err
	}
	credentials, err := storedVaultCredentials(key, s.encryptionService)
	if err != nil {
		return err
	}
	values, err := resolveVaultCredentials(credentials, s.encryptionService)
	if err != nil {
		return err
	}
	config, err := parseVaultConfig(prepared)
	if err != nil {
		return err
	}
	client, err := makeVaultClient(prepared, config, values)
	if err != nil {
		return err
	}
	token, err := client.authToken(ctx)
	if err != nil {
		return err
	}
	if config.AuthMethod == "token" {
		_, err = client.authRequest(ctx, "auth/token/lookup-self", token, nil)
	}
	return err
}
