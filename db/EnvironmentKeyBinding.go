package db

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// EnvironmentKeyBinding stores references only; previews and resolved values
// never belong to this record. Field nil selects the complete key value.
type EnvironmentKeyBinding struct {
	Name  string                `db:"name" json:"name"`
	Type  EnvironmentSecretType `db:"type" json:"type"`
	KeyID int                   `db:"key_id" json:"key_id"`
	Field *string               `db:"field" json:"field"`
}

var variableBindingName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (env *Environment) ValidateKeyBindings() error {
	seen := make(map[string]bool)
	for _, b := range env.KeyBindings {
		if b.KeyID <= 0 || len(b.Name) > 255 || !variableBindingName.MatchString(b.Name) ||
			(b.Type != EnvironmentSecretVar && b.Type != EnvironmentSecretEnv) ||
			b.Name == "taskexec_vars" || b.Name == "TASKEXEC_JWT" ||
			(b.Field != nil && (*b.Field == "" || len(*b.Field) > 255)) {
			return fmt.Errorf("invalid key variable binding")
		}
		id := string(b.Type) + ":" + b.Name
		if seen[id] {
			return fmt.Errorf("duplicate key variable binding")
		}
		seen[id] = true
		var plain map[string]any
		data := env.JSON
		if b.Type == EnvironmentSecretEnv {
			data = ""
			if env.ENV != nil {
				data = *env.ENV
			}
		}
		if data != "" {
			if err := json.Unmarshal([]byte(data), &plain); err != nil {
				return err
			}
		}
		if _, exists := plain[b.Name]; exists {
			return fmt.Errorf("key binding conflicts with a variable")
		}
		for _, secret := range env.Secrets {
			if secret.Operation != EnvironmentSecretDelete && secret.Name == b.Name && secret.Type == b.Type {
				return fmt.Errorf("key binding conflicts with a variable")
			}
		}
	}
	return nil
}
