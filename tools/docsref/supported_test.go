package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReferenceDocumentsSupportedOptions(t *testing.T) {
	ov, err := readOverlay("groups.json")
	require.NoError(t, err)
	page := render([]option{
		{Key: "ha.enabled", Type: "boolean"},
		{Key: "runner.executor.k8s.image", Type: "string"},
		{Key: "mfa.email.enabled", Type: "boolean"},
		{Key: "audit.syslog.address", Type: "string"},
		{Key: "audit.splunk_hec.url", Type: "string"},
		{Key: "runner.executor.type", Type: "string", Values: []string{"local", "docker", "k8s"}},
		{Key: "runner.executor.docker.image", Type: "string"},
		{Key: "audit.enabled", Type: "boolean"},
		{Key: "syslog.enabled", Type: "boolean"},
		{Key: "mfa.totp.enabled", Type: "boolean"},
	}, ov)
	for _, key := range []string{"ha.enabled", "runner.executor.k8s.image", "mfa.email.enabled", "audit.syslog.address", "audit.splunk_hec.url", "`k8s`"} {
		require.NotContains(t, page, key)
	}
	for _, key := range []string{"runner.executor.type", "runner.executor.docker.image", "audit.enabled", "syslog.enabled", "mfa.totp.enabled"} {
		require.Contains(t, page, "`"+key+"`")
	}
}
