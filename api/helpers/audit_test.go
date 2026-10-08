package helpers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/impishMD/jeh/services/audit"
	"github.com/impishMD/jeh/services/audit/audittest"
	"github.com/stretchr/testify/assert"
)

func TestAudit_DefaultsToNop(t *testing.T) {
	assert.Equal(t, audit.Nop{}, Audit(httptest.NewRequest(http.MethodGet, "/", nil)))
}

func TestAudit_ReturnsRecorderFromContext(t *testing.T) {
	rec := &audittest.Recorder{}
	r := SetContextValue(httptest.NewRequest(http.MethodGet, "/", nil), "audit", rec)
	assert.Same(t, rec, Audit(r))
}

func TestAudit_WrongTypeFallsBackToNop(t *testing.T) {
	r := SetContextValue(httptest.NewRequest(http.MethodGet, "/", nil), "audit", "not a recorder")
	assert.Equal(t, audit.Nop{}, Audit(r))
}
