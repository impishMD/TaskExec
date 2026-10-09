package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/util"
	"github.com/stretchr/testify/require"
)

func vaultTestCertificate(t *testing.T) (string, string, tls.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "TaskExec test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	pair, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	require.NoError(t, err)
	return certPEM, keyPEM, pair
}

func TestVaultAuthenticationMethods(t *testing.T) {
	for _, method := range []string{"approle", "kubernetes", "jwt", "cert"} {
		for _, provider := range []db.SecretStorageType{db.SecretStorageTypeVault} {
			for _, version := range []int{1, 2} {
				t.Run(fmt.Sprintf("%s/%s/kv%d", method, provider, version), func(t *testing.T) {
					f := newVaultFixture(t, version, provider)
					f.remote.put("app", map[string]any{"value": "authenticated-secret"})
					var logins, renewals atomic.Int32
					handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if strings.HasPrefix(r.URL.Path, "/v1/auth/") {
							if r.Header.Get("X-Vault-Namespace") != "engineering" {
								w.WriteHeader(400)
								return
							}
							if r.URL.Path == "/v1/auth/token/renew-self" {
								require.Equal(t, "fixture-token", r.Header.Get("X-Vault-Token"))
								renewals.Add(1)
							} else {
								require.Equal(t, "/v1/auth/custom/path/login", r.URL.Path)
								require.Empty(t, r.Header.Get("X-Vault-Token"))
								var body map[string]string
								require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
								switch method {
								case "approle":
									require.Equal(t, map[string]string{"role_id": "role-id", "secret_id": "secret-id"}, body)
								case "jwt", "kubernetes":
									require.Equal(t, map[string]string{"role": "test-role", "jwt": "test.jwt.value"}, body)
								case "cert":
									require.Equal(t, map[string]string{"name": "test-role"}, body)
									require.NotEmpty(t, r.TLS.VerifiedChains)
								}
								logins.Add(1)
							}
							_ = json.NewEncoder(w).Encode(map[string]any{"auth": map[string]any{"client_token": "fixture-token", "lease_duration": 60, "renewable": true}})
							return
						}
						f.remote.serve(w, r)
					})
					remote := httptest.NewUnstartedServer(handler)
					values := map[string]string{"role_id": "role-id", "secret_id": "secret-id", "jwt": "test.jwt.value"}
					if method == "cert" {
						cert, key, pair := vaultTestCertificate(t)
						values["ca_cert"], values["client_cert"], values["client_key"] = cert, cert, key
						roots := x509.NewCertPool()
						roots.AppendCertsFromPEM([]byte(cert))
						remote.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert, MinVersion: tls.VersionTLS12}
						remote.StartTLS()
					} else {
						remote.Start()
					}
					defer remote.Close()
					f.storage.Params["url"], f.storage.Params["auth_method"], f.storage.Params["auth_mount"] = remote.URL, method, "custom/path"
					if method != "approle" {
						f.storage.Params["auth_role"] = "test-role"
					}
					f.storage.Credentials = map[string]db.VaultCredential{}
					for _, field := range vaultCredentialFields(method) {
						f.storage.Credentials[field] = db.VaultCredential{Source: "database", Value: values[field]}
					}
					if method == "cert" {
						f.storage.Credentials["ca_cert"] = db.VaultCredential{Source: "database", Value: values["ca_cert"]}
					}
					require.NoError(t, f.service.Update(f.storage))
					saved, err := f.service.GetSecretStorage(f.project.ID, f.storage.ID)
					require.NoError(t, err)
					for _, credential := range saved.Credentials {
						require.Empty(t, credential.Value)
						require.True(t, credential.Configured)
					}
					require.Equal(t, db.SecretStorageTypeVault, saved.Type)
					// Editing a masked form preserves every secret.
					saved.Name = "Updated"
					require.NoError(t, f.service.Update(saved))
					key := f.reference("app", "app", db.AccessKeyString)
					require.NoError(t, f.encryption.DeserializeSecret(&key))
					require.Equal(t, "authenticated-secret", key.String)
					require.NoError(t, f.service.TestVaultAuthentication(context.Background(), saved))
					require.EqualValues(t, 1, logins.Load(), "connection test reuses the session instead of consuming Secret ID uses")
					client, err := newVaultClient(saved, f.store, f.encryption)
					require.NoError(t, err)
					vaultSessions.Lock()
					session := vaultSessions.entries[client.sessionKey]
					vaultSessions.Unlock()
					session.mu.Lock()
					session.renewAt = time.Now().Add(-time.Second)
					session.mu.Unlock()
					require.NoError(t, f.encryption.DeserializeSecret(&key))
					require.EqualValues(t, 1, renewals.Load())
					session.mu.Lock()
					session.expires = time.Now().Add(-time.Second)
					session.mu.Unlock()
					var wg sync.WaitGroup
					errs := make(chan error, 12)
					for i := 0; i < 12; i++ {
						wg.Add(1)
						go func() { defer wg.Done(); _, e := client.authToken(context.Background()); errs <- e }()
					}
					wg.Wait()
					close(errs)
					for err := range errs {
						require.NoError(t, err)
					}
					require.EqualValues(t, 2, logins.Load(), "concurrent requests share one login")
					// Existing KV writes work with every authentication method.
					saved.ReadOnly = false
					require.NoError(t, f.service.Update(saved))
					key.String = "written"
					key.OverrideSecret = true
					_, err = f.keys.Create(key)
					require.NoError(t, err)
					key.String = ""
					require.NoError(t, f.encryption.DeserializeSecret(&key))
					require.Equal(t, "written", key.String)
				})
			}
		}
	}
}

func TestVaultCredentialEditsSourcesAndAtomicity(t *testing.T) {
	f := newVaultFixture(t, 2, db.SecretStorageTypeVault)
	f.storage.Credentials = map[string]db.VaultCredential{"token": {Source: "database", Value: "saved-token"}}
	require.NoError(t, f.service.Update(f.storage))
	saved, err := f.service.GetSecretStorage(f.project.ID, f.storage.ID)
	require.NoError(t, err)
	saved.Params["auth_method"] = "approle"
	saved.Credentials = map[string]db.VaultCredential{"role_id": {Source: "database", Value: "role"}, "secret_id": {Source: "database", Configured: true}}
	require.ErrorContains(t, f.service.Update(saved), "secret_id")
	unchanged, err := f.store.GetSecretStorage(f.project.ID, f.storage.ID)
	require.NoError(t, err)
	require.Nil(t, unchanged.Params["auth_method"])
	path := filepath.Join(util.Config.GetSecretsPath(), "jwt")
	require.NoError(t, os.WriteFile(path, []byte("first.jwt\n"), 0600))
	saved.Params["auth_method"], saved.Params["auth_role"] = "jwt", "test"
	saved.Credentials = map[string]db.VaultCredential{"jwt": {Source: "file", Value: path}}
	require.NoError(t, f.service.Update(saved))
	client, err := newVaultClient(saved, f.store, f.encryption)
	require.NoError(t, err)
	require.Equal(t, "first.jwt", client.credentials["jwt"])
	require.NoError(t, os.WriteFile(path, []byte("rotated.jwt\n"), 0600))
	rotated, err := newVaultClient(saved, f.store, f.encryption)
	require.NoError(t, err)
	require.Equal(t, "rotated.jwt", rotated.credentials["jwt"])
	require.NotEqual(t, client.sessionKey, rotated.sessionKey)
	saved, err = f.service.GetSecretStorage(f.project.ID, f.storage.ID)
	require.NoError(t, err)
	require.Equal(t, path, saved.Credentials["jwt"].Value)
	saved.Credentials["jwt"] = db.VaultCredential{Source: "env", Configured: true}
	require.Error(t, f.service.Update(saved), "changing source requires a new reference")
	t.Setenv("TASKEXEC_TEST_AUTH_JWT", "env.jwt")
	saved.Credentials["jwt"] = db.VaultCredential{Source: "env", Value: "TASKEXEC_TEST_AUTH_JWT"}
	require.NoError(t, f.service.Update(saved))
	client, err = newVaultClient(saved, f.store, f.encryption)
	require.NoError(t, err)
	require.Equal(t, "env.jwt", client.credentials["jwt"])
	// Force a failure when the credential update follows the connection update.
	_, err = f.store.Sql().Exec("CREATE TRIGGER fail_auth_credentials BEFORE UPDATE OF secret ON access_key BEGIN SELECT RAISE(ABORT, 'test rollback'); END")
	require.NoError(t, err)
	saved.Params["auth_method"] = "approle"
	saved.Credentials = map[string]db.VaultCredential{"role_id": {Source: "database", Value: "new-role"}, "secret_id": {Source: "database", Value: "new-secret"}}
	require.ErrorContains(t, f.service.Update(saved), "test rollback")
	unchanged, err = f.service.GetSecretStorage(f.project.ID, f.storage.ID)
	require.NoError(t, err)
	require.Equal(t, "jwt", unchanged.Params["auth_method"])
	require.Equal(t, "TASKEXEC_TEST_AUTH_JWT", unchanged.Credentials["jwt"].Value)
	_, err = f.store.Sql().Exec("DROP TRIGGER fail_auth_credentials")
	require.NoError(t, err)
	require.NoError(t, f.encryption.RekeyAccessKeys(""))
	_, err = newVaultClient(unchanged, f.store, f.encryption)
	require.NoError(t, err)
}

func TestVaultAuthenticationFailuresAndTLS(t *testing.T) {
	var logins atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logins.Add(1)
		http.Error(w, "SECRET_RESPONSE private-key", 403)
	}))
	defer remote.Close()
	config := vaultConfig{AuthMethod: "approle", AuthMount: "approle"}
	storage := db.SecretStorage{Type: db.SecretStorageTypeVault, Params: db.MapStringAnyField{"url": remote.URL, "auth_method": "approle"}}
	config, err := parseVaultConfig(storage)
	require.NoError(t, err)
	c, err := makeVaultClient(storage, config, map[string]string{"role_id": "role", "secret_id": "private-key"})
	require.NoError(t, err)
	_, err = c.authToken(context.Background())
	require.ErrorContains(t, err, "HTTP 403")
	require.NotContains(t, err.Error(), "private-key")
	require.NotContains(t, err.Error(), "SECRET_RESPONSE")
	_, err = c.authToken(context.Background())
	require.Error(t, err)
	require.EqualValues(t, 1, logins.Load())
	storage.Params["auth_method"] = "cert"
	_, err = parseVaultConfig(storage)
	require.ErrorContains(t, err, "HTTPS")
	config.AuthMethod = "cert"
	_, err = makeVaultClient(storage, config, map[string]string{"client_cert": "invalid", "client_key": "private-key"})
	require.ErrorContains(t, err, "invalid Vault client certificate")
	require.NotContains(t, err.Error(), "private-key")
}

func TestVaultCredentialBundleIdentityAndRestoredMetadata(t *testing.T) {
	f := newVaultFixture(t, 2, db.SecretStorageTypeVault)
	original, err := f.store.GetAccessKeys(f.project.ID, db.GetAccessKeyOptions{Owner: db.AccessKeySecretStorage, StorageID: &f.storage.ID}, db.RetrieveQueryParams{})
	require.NoError(t, err)
	f.storage.Credentials = map[string]db.VaultCredential{"token": {Source: "database", Value: "token-one"}}
	require.NoError(t, f.service.Update(f.storage))
	keys, err := f.store.GetAccessKeys(f.project.ID, db.GetAccessKeyOptions{Owner: db.AccessKeySecretStorage, StorageID: &f.storage.ID}, db.RetrieveQueryParams{})
	require.NoError(t, err)
	require.Equal(t, original[0].ID, keys[0].ID, "editing preserves the owned key identity")
	other := f.storage
	other.ID = 0
	other.Name = "another"
	other, err = f.service.Create(other)
	require.NoError(t, err)
	otherKeys, err := f.store.GetAccessKeys(f.project.ID, db.GetAccessKeyOptions{Owner: db.AccessKeySecretStorage, StorageID: &other.ID}, db.RetrieveQueryParams{})
	require.NoError(t, err)
	require.NotEqual(t, keys[0].Name, otherKeys[0].Name, "project exports reference keys by unique name")
	// Project exports omit secret values; the restored form must still be editable.
	key := keys[0]
	key.Secret = nil
	key.String = ""
	key.OverrideSecret = true
	require.NoError(t, f.store.UpdateAccessKey(key))
	metadata, err := f.service.GetSecretStorage(f.project.ID, f.storage.ID)
	require.NoError(t, err)
	require.Empty(t, metadata.Credentials)
	metadata.Credentials = map[string]db.VaultCredential{"token": {Source: "database", Value: "restored-token"}}
	require.NoError(t, f.service.Update(metadata))
	client, err := newVaultClient(metadata, f.store, f.encryption)
	require.NoError(t, err)
	require.Equal(t, "restored-token", client.token)
}
