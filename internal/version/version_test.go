package version

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInfoDefaults(t *testing.T) {
	info := Info()

	assert.Equal(t, Current, info.Version)
	assert.Equal(t, "dev", info.Commit)
	assert.Equal(t, "dev", info.Tag)
	assert.Empty(t, info.BuildTime)
	assert.Equal(t, "local", info.BuildSource)
}

func TestLineIncludesBuildIdentity(t *testing.T) {
	line := Line("aftertalk")

	assert.True(t, strings.HasPrefix(line, "aftertalk "+Current+" dev dev"))
	assert.Contains(t, line, "local")
}
