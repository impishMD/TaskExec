package util

import "errors"

// Reject unsupported modes instead of silently ignoring requested guarantees.
func (c *ConfigType) ValidateServerCapabilities() error {
	if c.HA != nil && c.HA.Enabled {
		return errors.New("active-active HA is not implemented; disable ha.enabled and run one server")
	}
	if c.Mfa != nil && c.Mfa.Email != nil && c.Mfa.Email.Enabled {
		return errors.New("email OTP is not implemented; disable mfa.email.enabled (TOTP is supported)")
	}
	if c.Audit.IsEnabled() && (c.Audit.Syslog.IsConfigured() || c.Audit.SplunkHEC.IsConfigured()) {
		return errors.New("external audit export is not implemented; remove audit.syslog/audit.splunk_hec destinations to use the database audit log")
	}
	return nil
}
