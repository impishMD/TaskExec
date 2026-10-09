package db

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var keyExpressionToken = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)
var keyExpressionIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)

type KeyExpressionReference struct {
	Start, End int
	Prefix     string
	Path       []any // string object keys or integer array indexes
}

// Only registered prefixes belong to TaskExec. Other expressions, including
// Ansible/Jinja variables, are left untouched. This is a path parser, not eval.
func ParseKeyExpressions(value string, sources []EnvironmentKeySource) ([]KeyExpressionReference, error) {
	known := map[string]bool{}
	for _, source := range sources {
		known[source.Prefix] = true
	}
	refs := []KeyExpressionReference{}
	for _, match := range keyExpressionToken.FindAllStringSubmatchIndex(value, -1) {
		text := strings.TrimSpace(value[match[2]:match[3]])
		prefix := keyExpressionIdentifier.FindString(text)
		if !known[prefix] {
			continue
		}
		ref := KeyExpressionReference{Start: match[0], End: match[1], Prefix: prefix}
		rest := strings.TrimSpace(text[len(prefix):])
		for rest != "" {
			switch rest[0] {
			case '.':
				name := keyExpressionIdentifier.FindString(rest[1:])
				if name == "" {
					return nil, fmt.Errorf("invalid key expression")
				}
				ref.Path = append(ref.Path, name)
				rest = strings.TrimSpace(rest[1+len(name):])
			case '[':
				rest = strings.TrimSpace(rest[1:])
				if strings.HasPrefix(rest, `"`) {
					var name string
					decoder := json.NewDecoder(strings.NewReader(rest))
					if err := decoder.Decode(&name); err != nil {
						return nil, fmt.Errorf("invalid key expression")
					}
					rest = strings.TrimSpace(rest[decoder.InputOffset():])
					ref.Path = append(ref.Path, name)
				} else {
					i := strings.IndexByte(rest, ']')
					if i < 0 {
						return nil, fmt.Errorf("invalid key expression")
					}
					n, err := strconv.Atoi(strings.TrimSpace(rest[:i]))
					if err != nil || n < 0 {
						return nil, fmt.Errorf("invalid key expression")
					}
					ref.Path = append(ref.Path, n)
					rest = rest[i:]
				}
				if !strings.HasPrefix(rest, "]") {
					return nil, fmt.Errorf("invalid key expression")
				}
				rest = strings.TrimSpace(rest[1:])
			default:
				return nil, fmt.Errorf("invalid key expression")
			}
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

func (env *Environment) ValidateSecretExpressions() error {
	seen := map[string]bool{}
	for _, expression := range env.SecretExpressions {
		if len(expression.Name) > 255 || !variableBindingName.MatchString(expression.Name) ||
			(expression.Type != EnvironmentSecretEnv && expression.Type != EnvironmentSecretVar) ||
			expression.Name == "TASKEXEC_JWT" || expression.Name == "taskexec_vars" {
			return fmt.Errorf("invalid key variable binding")
		}
		refs, err := ParseKeyExpressions(expression.Expression, env.KeySources)
		if err != nil || len(refs) == 0 {
			return fmt.Errorf("invalid key expression")
		}
		id := string(expression.Type) + ":" + expression.Name
		if seen[id] {
			return fmt.Errorf("key binding conflicts with a variable")
		}
		seen[id] = true
		for _, binding := range env.KeyBindings {
			if binding.Type == expression.Type && binding.Name == expression.Name {
				return fmt.Errorf("key binding conflicts with a variable")
			}
		}
		for _, secret := range env.Secrets {
			if secret.Operation != EnvironmentSecretDelete && secret.Type == expression.Type && secret.Name == expression.Name {
				return fmt.Errorf("key binding conflicts with a variable")
			}
		}
		var plain map[string]any
		raw := env.JSON
		if expression.Type == EnvironmentSecretEnv {
			raw = ""
			if env.ENV != nil {
				raw = *env.ENV
			}
		}
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &plain); err != nil {
				return err
			}
		}
		if _, exists := plain[expression.Name]; exists {
			return fmt.Errorf("key binding conflicts with a variable")
		}
	}
	return nil
}

func (env *Environment) ValidateKeyExpressions(previousSources []EnvironmentKeySource) error {
	removed := []EnvironmentKeySource{}
	for _, old := range previousSources {
		present := false
		for _, source := range env.KeySources {
			if source.Prefix == old.Prefix {
				present = true
			}
		}
		if !present {
			removed = append(removed, old)
		}
	}
	var walk func(any) error
	walk = func(value any) error {
		switch v := value.(type) {
		case string:
			if _, err := ParseKeyExpressions(v, env.KeySources); err != nil {
				return err
			}
			refs, err := ParseKeyExpressions(v, removed)
			if err != nil || len(refs) > 0 {
				return fmt.Errorf("key source prefix is still referenced")
			}
		case map[string]any:
			for _, child := range v {
				if err := walk(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range v {
				if err := walk(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, raw := range []string{env.JSON, func() string {
		if env.ENV != nil {
			return *env.ENV
		}
		return ""
	}()} {
		if raw == "" {
			continue
		}
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return err
		}
		if err := walk(value); err != nil {
			return err
		}
	}
	for _, expression := range env.SecretExpressions {
		if err := walk(expression.Expression); err != nil {
			return err
		}
	}
	return nil
}
