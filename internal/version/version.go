// Package version exposes the application version read from version.txt at build time.
// version.txt is the single source of truth for the application version.
// To release a new version: update version.txt and tag the commit.
package version

import (
	_ "embed"
	"strings"
)

//go:embed version.txt
var raw string

// Current is the application version (e.g. "1.0.0").
var Current = strings.TrimSpace(raw) //nolint:gochecknoglobals // embed-initialized version variable
