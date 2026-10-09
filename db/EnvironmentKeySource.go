package db

import (
	"fmt"
)

// EnvironmentKeySource makes a shared key available to expressions within one
// variable group. Prefixes are namespaces, not exported variables.
type EnvironmentKeySource struct {
	Prefix string `json:"prefix"`
	KeyID  int    `json:"key_id"`
}

// EnvironmentSecretExpression contains an expression only, never its resolved
// value. Literal secrets continue to use encrypted access keys.
type EnvironmentSecretExpression struct {
	Name       string                `db:"name" json:"name"`
	Type       EnvironmentSecretType `db:"type" json:"type"`
	Expression string                `db:"expression" json:"expression"`
}

const EnvironmentKeySourceType EnvironmentSecretType = "source"

func (env *Environment) ValidateKeySources() error {
	seen := map[string]bool{}
	for _, source := range env.KeySources {
		if source.KeyID <= 0 || len(source.Prefix) > 255 || !variableBindingName.MatchString(source.Prefix) ||
			source.Prefix == "__proto__" || source.Prefix == "constructor" || source.Prefix == "prototype" {
			return fmt.Errorf("invalid key source prefix")
		}
		if seen[source.Prefix] {
			return fmt.Errorf("duplicate key source prefix")
		}
		seen[source.Prefix] = true
	}
	return nil
}
