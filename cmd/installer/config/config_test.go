package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefault_WhisperLanguage(t *testing.T) {
	cfg := Default()
	assert.Equal(t, "it", cfg.WhisperLanguage, "default language must be 'it'")
}

func TestFromEnvMap_WhisperLanguage(t *testing.T) {
	t.Run("explicit value", func(t *testing.T) {
		cfg := FromEnvMap(map[string]string{"WHISPER_LANGUAGE": "en"})
		assert.Equal(t, "en", cfg.WhisperLanguage)
	})

	t.Run("falls back to default when missing", func(t *testing.T) {
		cfg := FromEnvMap(map[string]string{})
		assert.Equal(t, "it", cfg.WhisperLanguage, "must fall back to default 'it'")
	})

	t.Run("falls back to default when empty string", func(t *testing.T) {
		cfg := FromEnvMap(map[string]string{"WHISPER_LANGUAGE": ""})
		assert.Equal(t, "it", cfg.WhisperLanguage)
	})
}

func TestWriteReadEnvFile_WhisperLanguage_roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "install.env")

	cfg := Default()
	cfg.WhisperLanguage = "fr"
	cfg.JWTSecret = "secret"
	cfg.APIKey = "key"

	require.NoError(t, WriteEnvFile(path, cfg))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "WHISPER_LANGUAGE=fr", "env file must contain new field")

	m, err := ReadEnvFile(path)
	require.NoError(t, err)

	cfg2 := FromEnvMap(m)
	assert.Equal(t, "fr", cfg2.WhisperLanguage)
}
