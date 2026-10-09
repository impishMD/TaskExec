package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/impishMD/taskexec/db"
)

// ReadVariableKey is used by both live previews and task preparation. Only
// shared variable keys can be exposed here, never credentials or storage tokens.
func ReadVariableKey(keys db.AccessKeyManager, encryption AccessKeyEncryptionService, projectID, keyID int) (any, error) {
	key, err := keys.GetAccessKey(projectID, keyID)
	if err != nil {
		return nil, err
	}
	if key.Owner != db.AccessKeyShared || (key.Type != db.AccessKeyString && key.Type != db.AccessKeyObject) {
		return nil, errors.New("key type cannot be used by variable binding")
	}
	if err = encryption.DeserializeSecret(&key); err != nil {
		return nil, err
	}
	if key.Type == db.AccessKeyObject {
		return key.Object, nil
	}
	return key.String, nil
}

func (s *accessKeyEncryptionServiceImpl) fillKeyBindings(env *db.Environment, values map[int]any) error {
	pending := []db.EnvironmentSecret{}
	seen := map[string]bool{}
	for _, secret := range env.Secrets {
		seen[string(secret.Type)+":"+secret.Name] = true
	}
	for _, target := range []db.EnvironmentSecretType{db.EnvironmentSecretEnv, db.EnvironmentSecretVar} {
		raw := env.JSON
		if target == db.EnvironmentSecretEnv {
			raw = ""
			if env.ENV != nil {
				raw = *env.ENV
			}
		}
		var plain map[string]any
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &plain); err != nil {
				return err
			}
		}
		for name := range plain {
			seen[string(target)+":"+name] = true
		}
	}
	for _, b := range env.KeyBindings {
		value, found := values[b.KeyID]
		if !found {
			var err error
			value, err = ReadVariableKey(s.accessKeyRepo, s, env.ProjectID, b.KeyID)
			if err != nil {
				return fmt.Errorf("variable %s: %w", b.Name, err)
			}
			values[b.KeyID] = value
		}
		if b.Field != nil {
			object, ok := value.(map[string]any)
			if !ok {
				return errors.New("key type cannot be used by variable binding")
			}
			value, found = object[*b.Field]
			if !found {
				return fmt.Errorf("Vault mapped field is missing: %s", *b.Field)
			}
		}
		expanded, err := expandKeyBinding(b, value)
		if err != nil {
			return err
		}
		for _, secret := range expanded {
			id := string(secret.Type) + ":" + secret.Name
			if seen[id] {
				return fmt.Errorf("key binding conflicts with a variable: %s", secret.Name)
			}
			seen[id] = true
			pending = append(pending, secret)
		}
	}
	env.Secrets = append(env.Secrets, pending...)
	return nil
}

var environmentVariableName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func expandKeyBinding(b db.EnvironmentKeyBinding, value any) ([]db.EnvironmentSecret, error) {
	values := map[string]any{b.Name: value}
	if object, ok := value.(map[string]any); ok && b.Type == db.EnvironmentSecretEnv && b.Field == nil {
		for field, child := range object {
			name := b.Name + "_" + field
			if !environmentVariableName.MatchString(name) || len(name) > 255 || name == "TASKEXEC_JWT" || name == "taskexec_vars" {
				return nil, fmt.Errorf("invalid environment field name: %s", field)
			}
			values[name] = child
		}
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	secrets := make([]db.EnvironmentSecret, 0, len(values))
	for _, name := range names {
		encoded, err := json.Marshal(values[name])
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, db.EnvironmentSecret{Name: name, Type: b.Type, JSONValue: encoded})
	}
	return secrets, nil
}
