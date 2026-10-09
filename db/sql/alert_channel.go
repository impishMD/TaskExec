package sql

import (
	"errors"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/util"
)

func (d *SqlDb) GetAlertChannel(projectID int, channel string) (res db.AlertChannel, err error) {
	res = db.AlertChannel{ProjectID: projectID, Channel: channel, Settings: "{}"}
	if projectID == 0 {
		err = d.selectOne(&res, "select 0 as project_id, channel, enabled, settings, secret from alert_channel where channel=?", channel)
	} else {
		err = d.selectOne(&res, "select * from project__alert_channel where project_id=? and channel=?", projectID, channel)
	}
	if errors.Is(err, db.ErrNotFound) {
		err = nil
	}
	return
}

func (d *SqlDb) SetAlertChannel(c db.AlertChannel) error {
	query := "insert into alert_channel (channel, enabled, settings, secret) values (?, ?, ?, ?)"
	args := []any{c.Channel, c.Enabled, c.Settings, c.Secret}
	conflict := "channel"
	if c.ProjectID != 0 {
		query = "insert into project__alert_channel (project_id, channel, enabled, settings, secret) values (?, ?, ?, ?, ?)"
		args = append([]any{c.ProjectID}, args...)
		conflict = "project_id, channel"
	}
	if d.GetDialect() == util.DbDriverMySQL {
		query += " on duplicate key update enabled=values(enabled), settings=values(settings), secret=values(secret)"
	} else {
		query += " on conflict (" + conflict + ") do update set enabled=excluded.enabled, settings=excluded.settings, secret=excluded.secret"
	}
	_, err := d.exec(query, args...)
	return err
}

func (d *SqlDb) GetAlertChannelsWithSecrets() (res []db.AlertChannel, err error) {
	_, err = d.selectAll(&res, "select 0 as project_id, channel, enabled, settings, secret from alert_channel where secret<>'' union all select project_id, channel, enabled, settings, secret from project__alert_channel where secret<>''")
	return
}
