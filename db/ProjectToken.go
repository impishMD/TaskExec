package db

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-gorp/gorp/v3"
	"github.com/impishMD/taskexec/pkg/common_errors"
)

const ProjectTokenPrefix = "texp_"

const (
	TokenReadTemplates = "templates:read"
	TokenRunTasks      = "tasks:run"
	TokenReadTasks     = "tasks:read"
	TokenReadLogs      = "tasks:logs"
	TokenStopTasks     = "tasks:stop"
)

var ProjectTokenScopes = []string{TokenReadTemplates, TokenRunTasks, TokenReadTasks, TokenReadLogs, TokenStopTasks}
var ProjectTokenOverrides = []string{"git_branch", "playbook", "inventory_id", "arguments", "environment", "params"}

// ProjectToken is an independent project credential. Creator fields are audit
// snapshots, deliberately not foreign keys or an authorization principal.
type ProjectToken struct {
	ID              string     `db:"id" json:"id"`
	ProjectID       int        `db:"project_id" json:"project_id"`
	Name            string     `db:"name" json:"name"`
	SecretHash      string     `db:"secret_hash" json:"-"`
	Scopes          []string   `db:"-" json:"scopes"`
	ScopesJSON      string     `db:"scopes" json:"-"`
	AllTemplates    bool       `db:"all_templates" json:"all_templates"`
	TemplateIDs     []int      `db:"-" json:"template_ids"`
	TemplateIDsJSON string     `db:"template_ids" json:"-"`
	Overrides       []string   `db:"-" json:"overrides"`
	OverridesJSON   string     `db:"overrides" json:"-"`
	CreatorID       int        `db:"creator_id" json:"creator_id"`
	CreatorName     string     `db:"creator_name" json:"creator_name"`
	Created         time.Time  `db:"created" json:"created"`
	ExpiresAt       *time.Time `db:"expires_at" json:"expires_at"`
	RevokedAt       *time.Time `db:"revoked_at" json:"revoked_at"`
	LastUsedAt      *time.Time `db:"last_used_at" json:"last_used_at"`
}

func (t ProjectToken) Can(scope string) bool { return slices.Contains(t.Scopes, scope) }
func (t ProjectToken) AllowsTemplate(id int) bool {
	return t.AllTemplates || slices.Contains(t.TemplateIDs, id)
}
func (t ProjectToken) IsActive(now time.Time) bool {
	return t.RevokedAt == nil && (t.ExpiresAt == nil || t.ExpiresAt.After(now))
}

func (t ProjectToken) Validate(now time.Time) error {
	invalid := func(message string) error { return common_errors.NewValidationError(message) }
	if strings.TrimSpace(t.Name) == "" || utf8.RuneCountInString(t.Name) > 100 {
		return invalid("Token name must contain 1–100 characters")
	}
	if len(t.Scopes) == 0 || len(t.Scopes) > len(ProjectTokenScopes) {
		return invalid("Select token permissions")
	}
	for _, scope := range t.Scopes {
		if !slices.Contains(ProjectTokenScopes, scope) {
			return invalid("Unknown token permission")
		}
	}
	if len(t.Overrides) > len(ProjectTokenOverrides) {
		return invalid("Too many token overrides")
	}
	for _, field := range t.Overrides {
		if !slices.Contains(ProjectTokenOverrides, field) {
			return invalid("Unknown task override")
		}
	}
	if len(t.Overrides) > 0 && !t.Can(TokenRunTasks) {
		return invalid("Overrides require task execution permission")
	}
	if (!t.AllTemplates && len(t.TemplateIDs) == 0) || len(t.TemplateIDs) > 1000 {
		return invalid("Select allowed templates")
	}
	if t.AllTemplates && len(t.TemplateIDs) > 0 {
		return invalid("Choose all templates or a template list")
	}
	if t.ExpiresAt != nil && !t.ExpiresAt.After(now) {
		return invalid("Token expiry must be in the future")
	}
	return nil
}

func (t *ProjectToken) IssueSecret() (string, error) {
	var id [16]byte
	var secret [32]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	if _, err := rand.Read(secret[:]); err != nil {
		return "", err
	}
	t.ID = hex.EncodeToString(id[:])
	value := ProjectTokenPrefix + t.ID + "_" + base64.RawURLEncoding.EncodeToString(secret[:])
	sum := sha256.Sum256([]byte(value))
	t.SecretHash = hex.EncodeToString(sum[:])
	return value, nil
}

func ProjectTokenID(value string) string {
	if !strings.HasPrefix(value, ProjectTokenPrefix) || len(value) != len(ProjectTokenPrefix)+32+1+43 {
		return ""
	}
	id := value[len(ProjectTokenPrefix) : len(ProjectTokenPrefix)+32]
	if value[len(ProjectTokenPrefix)+32] != '_' {
		return ""
	}
	if _, err := hex.DecodeString(id); err != nil {
		return ""
	}
	return id
}

func (t ProjectToken) MatchesSecret(value string) bool {
	sum := sha256.Sum256([]byte(value))
	return subtle.ConstantTimeCompare([]byte(t.SecretHash), []byte(hex.EncodeToString(sum[:]))) == 1
}

func (t *ProjectToken) PreInsert(gorp.SqlExecutor) error {
	for _, field := range []struct {
		value  any
		target *string
	}{{t.Scopes, &t.ScopesJSON}, {t.TemplateIDs, &t.TemplateIDsJSON}, {t.Overrides, &t.OverridesJSON}} {
		b, err := json.Marshal(field.value)
		if err != nil {
			return err
		}
		*field.target = string(b)
	}
	return nil
}
func (t *ProjectToken) PostGet(gorp.SqlExecutor) error {
	for _, field := range []struct {
		value  string
		target any
	}{{t.ScopesJSON, &t.Scopes}, {t.TemplateIDsJSON, &t.TemplateIDs}, {t.OverridesJSON, &t.Overrides}} {
		if err := json.Unmarshal([]byte(field.value), field.target); err != nil {
			return err
		}
	}
	return nil
}
