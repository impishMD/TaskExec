package util

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestServerRejectsUnimplementedCapabilities(t *testing.T) {
	for _, tc := range []struct {
		config  ConfigType
		message string
	}{
		{ConfigType{HA: &HAConfig{Enabled: true}}, "HA"},
		{ConfigType{Mfa: &MultifactorAuthConfig{Email: &EmailAuthConfig{Enabled: true}}}, "email OTP"},
		{ConfigType{Audit: &AuditConfig{Enabled: true, Syslog: &AuditSyslogConfig{Address: "localhost:6514"}}}, "audit export"},
		{ConfigType{Audit: &AuditConfig{Enabled: true, SplunkHEC: &AuditSplunkHECConfig{URL: "https://example.test"}}}, "audit export"},
	} {
		require.ErrorContains(t, tc.config.ValidateServerCapabilities(), tc.message)
	}
	require.NoError(t, (&ConfigType{Audit: &AuditConfig{Enabled: true, InstanceID: "local"}}).ValidateServerCapabilities())
}
