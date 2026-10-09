package db

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"regexp"
	"strings"

	"github.com/impishMD/taskexec/pkg/common_errors"
)

const MaxProjectIconLength = 60000 // Fits a TEXT column on every supported database.

var projectIconName = regexp.MustCompile(`^mdi-[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Icons are either bundled glyph names or compact PNG data URLs. Remote URLs and
// active image formats are not project icons. Uploads are resized by the UI.
func (p *Project) ValidateIcon() error {
	if p.Icon == nil {
		return nil
	}
	icon := *p.Icon
	if icon == "" {
		p.Icon = nil
		return nil
	}
	if len(icon) <= 80 && projectIconName.MatchString(icon) {
		return nil
	}
	invalid := common_errors.NewValidationError("invalid project icon")
	const prefix = "data:image/png;base64,"
	if len(icon) > MaxProjectIconLength || !strings.HasPrefix(icon, prefix) {
		return invalid
	}
	data, err := base64.StdEncoding.Strict().DecodeString(strings.TrimPrefix(icon, prefix))
	if err != nil {
		return invalid
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 256 || config.Height > 256 {
		return invalid
	}
	if _, err = png.Decode(bytes.NewReader(data)); err != nil {
		return invalid
	}
	return nil
}
