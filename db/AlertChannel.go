package db

// AlertChannel stores one channel's settings independently of other providers.
// ProjectID == 0 denotes instance-wide defaults. Secrets never enter API or
// project backup JSON; only the alert service decrypts them.
type AlertChannel struct {
	ProjectID int    `db:"project_id" json:"-"`
	Channel   string `db:"channel" json:"channel"`
	Enabled   bool   `db:"enabled" json:"enabled"`
	Settings  string `db:"settings" json:"-"`
	Secret    string `db:"secret" json:"-"`
}

type AlertChannelRepository interface {
	GetAlertChannel(projectID int, channel string) (AlertChannel, error)
	SetAlertChannel(channel AlertChannel) error
	GetAlertChannelsWithSecrets() ([]AlertChannel, error)
}
