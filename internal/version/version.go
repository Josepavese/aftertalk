// Package version exposes semantic version and build identity metadata.
// version.txt remains the single source of truth for the product/API semantic
// version. Commit, tag, build time, and source are deploy identity fields
// injected by CI with -ldflags; local builds keep explicit dev defaults.
package version

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed version.txt
var raw string

// Current is the application version (e.g. "1.0.0").
var Current = strings.TrimSpace(raw) //nolint:gochecknoglobals // embed-initialized version variable

var ( //nolint:gochecknoglobals // populated by release builds through -ldflags.
	// Commit is the Git commit SHA that produced this binary.
	Commit = "dev"
	// Tag is the release tag or channel that produced this binary.
	Tag = "dev"
	// BuildTime is the UTC timestamp for the release build.
	BuildTime = ""
	// BuildSource identifies the build system that produced this binary.
	BuildSource = "local"
)

// BuildInfo is the runtime-visible identity of a concrete binary.
type BuildInfo struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	Tag         string `json:"tag"`
	BuildTime   string `json:"build_time"`
	BuildSource string `json:"build_source"`
}

// Info returns the semantic version plus deploy identity for this binary.
func Info() BuildInfo {
	return BuildInfo{
		Version:     Current,
		Commit:      Commit,
		Tag:         Tag,
		BuildTime:   BuildTime,
		BuildSource: BuildSource,
	}
}

// Line returns a compact CLI representation suitable for deploy logs.
func Line(command string) string {
	info := Info()
	if info.BuildTime == "" {
		return fmt.Sprintf("%s %s %s %s %s", command, info.Version, info.Tag, info.Commit, info.BuildSource)
	}
	return fmt.Sprintf("%s %s %s %s %s %s", command, info.Version, info.Tag, info.Commit, info.BuildTime, info.BuildSource)
}
