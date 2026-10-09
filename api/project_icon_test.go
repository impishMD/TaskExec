package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/impishMD/taskexec/db"
	"github.com/impishMD/taskexec/services/server"
	"github.com/stretchr/testify/require"
)

func TestProjectIconPermissionsPersistenceAndRemoval(t *testing.T) {
	f := newCoreFixture(t)
	f.router = Route(f.store, nil, nil, f.store, nil, server.NewProjectService(f.store, f.store), nil, nil, nil, nil, nil, nil, nil, server.NewRunnerService(f.store), nil, nil)
	path := fmt.Sprintf("/project/%d", f.project.ID)
	body := fmt.Sprintf(`{"id":%d,"name":"primary","icon":"mdi-server"}`, f.project.ID)
	f.request(t, "guest", "PUT", path, body, 403)
	f.request(t, "manager", "PUT", path, body, 403)
	f.request(t, "outsider", "PUT", path, body, 404)
	f.request(t, "owner", "PUT", path, body, 204)
	var p db.Project
	read := func() {
		require.NoError(t, json.Unmarshal(f.request(t, "guest", "GET", path, "", 200).Body.Bytes(), &p))
	}
	read()
	require.Equal(t, "mdi-server", *p.Icon)
	// Existing API callers can update settings without supplying the new field.
	f.request(t, "owner", "PUT", path, fmt.Sprintf(`{"id":%d,"name":"renamed"}`, p.ID), 204)
	read()
	require.Equal(t, "mdi-server", *p.Icon)
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 64, 64))))
	icon := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	p.Icon = &icon
	data, err := json.Marshal(p)
	require.NoError(t, err)
	f.request(t, "owner", "PUT", path, string(data), 204)
	read()
	require.Equal(t, icon, *p.Icon)
	for _, invalid := range []string{"https://example.org/icon.png", "data:image/svg+xml,<svg/>", "data:image/png;base64,broken", strings.Repeat("x", 60001)} {
		p.Icon = &invalid
		data, err = json.Marshal(p)
		require.NoError(t, err)
		f.request(t, "owner", "PUT", path, string(data), 400)
	}
	read()
	require.Equal(t, icon, *p.Icon, "invalid updates must preserve the stored image")
	f.request(t, "owner", "PUT", path, fmt.Sprintf(`{"id":%d,"name":"renamed","icon":null}`, p.ID), 204)
	p.Icon = nil
	read()
	require.Nil(t, p.Icon)
}
