package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/impishMD/taskexec/db"
)

func (s *accessKeyEncryptionServiceImpl) fillKeyExpressions(env *db.Environment, cache map[int]any) error {
	if len(env.KeySources) == 0 && len(env.SecretExpressions) == 0 {
		return nil
	}
	if err := env.ValidateKeySources(); err != nil {
		return err
	}
	if err := env.ValidateSecretExpressions(); err != nil {
		return err
	}
	read := func(prefix string) (any, error) {
		for _, source := range env.KeySources {
			if source.Prefix != prefix {
				continue
			}
			if value, ok := cache[source.KeyID]; ok {
				return value, nil
			}
			value, err := ReadVariableKey(s.accessKeyRepo, s, env.ProjectID, source.KeyID)
			if err != nil {
				return nil, fmt.Errorf("key source %s: %w", prefix, err)
			}
			cache[source.KeyID] = value
			return value, nil
		}
		return nil, fmt.Errorf("invalid key source prefix")
	}
	resolve := func(value any) (any, bool, error) { return resolveKeyValue(value, env.KeySources, read) }
	secrets := append([]db.EnvironmentSecret{}, env.Secrets...)
	seen := map[string]bool{}
	for _, secret := range secrets {
		seen[string(secret.Type)+":"+secret.Name] = true
	}
	appendSecret := func(name string, target db.EnvironmentSecretType, value any) error {
		if name == "TASKEXEC_JWT" || name == "taskexec_vars" {
			return fmt.Errorf("invalid key variable binding")
		}
		id := string(target) + ":" + name
		if seen[id] {
			return fmt.Errorf("key binding conflicts with a variable: %s", name)
		}
		seen[id] = true
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		secrets = append(secrets, db.EnvironmentSecret{Name: name, Type: target, JSONValue: data})
		return nil
	}
	var resultJSON, resultENV string
	for _, target := range []db.EnvironmentSecretType{db.EnvironmentSecretVar, db.EnvironmentSecretEnv} {
		raw := env.JSON
		if target == db.EnvironmentSecretEnv {
			raw = ""
			if env.ENV != nil {
				raw = *env.ENV
			}
		}
		plain := map[string]any{}
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &plain); err != nil {
				return err
			}
		}
		for name, value := range plain {
			resolved, changed, err := resolve(value)
			if err != nil {
				return err
			}
			if !changed {
				continue
			}
			// Resolved values join the secret channel so task masking also covers
			// references entered on the Variables tab; persisted JSON stays a template.
			if err = appendSecret(name, target, resolved); err != nil {
				return err
			}
			delete(plain, name)
		}
		encoded, err := json.Marshal(plain)
		if err != nil {
			return err
		}
		if target == db.EnvironmentSecretVar {
			resultJSON = string(encoded)
		} else {
			resultENV = string(encoded)
		}
	}
	for _, expression := range env.SecretExpressions {
		resolved, _, err := resolve(expression.Expression)
		if err != nil {
			return err
		}
		if err = appendSecret(expression.Name, expression.Type, resolved); err != nil {
			return err
		}
	}
	env.JSON = resultJSON
	if env.ENV != nil {
		env.ENV = &resultENV
	}
	env.Secrets = secrets
	return nil
}

func resolveKeyValue(value any, sources []db.EnvironmentKeySource, read func(string) (any, error)) (any, bool, error) {
	switch v := value.(type) {
	case string:
		refs, err := db.ParseKeyExpressions(v, sources)
		if err != nil || len(refs) == 0 {
			return value, false, err
		}
		var output strings.Builder
		last := 0
		for _, ref := range refs {
			selected, err := read(ref.Prefix)
			if err != nil {
				return nil, false, err
			}
			for _, part := range ref.Path {
				found := false
				switch p := part.(type) {
				case string:
					if object, ok := selected.(map[string]any); ok {
						selected, found = object[p]
					}
				case int:
					if array, ok := selected.([]any); ok && p < len(array) {
						selected, found = array[p], true
					}
				}
				if !found {
					return nil, false, fmt.Errorf("key expression field is missing: %s", ref.Prefix)
				}
			}
			// A whole-value reference preserves objects, arrays, numbers and null.
			if len(refs) == 1 && strings.TrimSpace(v[:ref.Start]) == "" && strings.TrimSpace(v[ref.End:]) == "" {
				return selected, true, nil
			}
			output.WriteString(v[last:ref.Start])
			if text, ok := selected.(string); ok {
				output.WriteString(text)
			} else {
				encoded, err := json.Marshal(selected)
				if err != nil {
					return nil, false, err
				}
				output.Write(encoded)
			}
			last = ref.End
		}
		output.WriteString(v[last:])
		return output.String(), true, nil
	case map[string]any:
		result, changed := map[string]any{}, false
		for name, child := range v {
			resolved, used, err := resolveKeyValue(child, sources, read)
			if err != nil {
				return nil, false, err
			}
			result[name], changed = resolved, changed || used
		}
		return result, changed, nil
	case []any:
		result, changed := make([]any, len(v)), false
		for i, child := range v {
			resolved, used, err := resolveKeyValue(child, sources, read)
			if err != nil {
				return nil, false, err
			}
			result[i], changed = resolved, changed || used
		}
		return result, changed, nil
	default:
		return value, false, nil
	}
}
