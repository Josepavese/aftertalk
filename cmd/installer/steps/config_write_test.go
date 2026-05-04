package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
)

func renderYAML(t *testing.T, cfg *instconfig.InstallConfig) string {
	t.Helper()
	dir := t.TempDir()
	cfg.ServiceRoot = dir
	// runConfigWrite writes to ServiceRoot/aftertalk.yaml; it also tries /etc/aftertalk/aftertalk.env
	// which may fail in CI — that's a Warn not an error, so it's fine.
	log := &testLogger{}
	err := runConfigWrite(t.Context(), cfg, log)
	require.NoError(t, err)
	out, err := os.ReadFile(filepath.Join(dir, "aftertalk.yaml"))
	require.NoError(t, err)
	return string(out)
}

// ── STT providers ─────────────────────────────────────────────────────────────

func TestConfigWrite_STT_WhisperLocal(t *testing.T) {
	cfg := instconfig.Default()
	cfg.STTProvider = "whisper-local"
	cfg.WhisperURL = "http://localhost:9001"
	cfg.STTConfig = map[string]string{}

	yaml := renderYAML(t, cfg)
	assert.Contains(t, yaml, `provider: "whisper-local"`)
	assert.Contains(t, yaml, `whisper_local:`)
	assert.Contains(t, yaml, `"http://localhost:9001"`)
	assert.NotContains(t, yaml, "google:")
	assert.NotContains(t, yaml, "aws:")
}

func TestConfigWrite_STT_Google(t *testing.T) {
	cfg := instconfig.Default()
	cfg.STTProvider = "google"
	cfg.STTConfig = map[string]string{
		"GOOGLE_APPLICATION_CREDENTIALS": "/opt/aftertalk/gcp.json",
	}

	yaml := renderYAML(t, cfg)
	assert.Contains(t, yaml, `provider: "google"`)
	assert.Contains(t, yaml, `google:`)
	assert.Contains(t, yaml, `credentials_path: "/opt/aftertalk/gcp.json"`)
	assert.NotContains(t, yaml, "whisper_local:")
	assert.NotContains(t, yaml, "aws:")
}

func TestConfigWrite_STT_AWS(t *testing.T) {
	cfg := instconfig.Default()
	cfg.STTProvider = "aws"
	cfg.STTConfig = map[string]string{
		"AWS_ACCESS_KEY_ID":     "AKIAIOSFODNN7",
		"AWS_SECRET_ACCESS_KEY": "wJalrXUtnFEMI",
		"AWS_REGION":            "eu-west-1",
	}

	yaml := renderYAML(t, cfg)
	assert.Contains(t, yaml, `provider: "aws"`)
	assert.Contains(t, yaml, `aws:`)
	assert.Contains(t, yaml, `access_key_id:`)
	assert.Contains(t, yaml, `secret_access_key:`)
	assert.Contains(t, yaml, `region:            "eu-west-1"`)
	assert.NotContains(t, yaml, "whisper_local:")
}

func TestConfigWrite_STT_Azure(t *testing.T) {
	cfg := instconfig.Default()
	cfg.STTProvider = "azure"
	cfg.STTConfig = map[string]string{
		"AZURE_SPEECH_KEY":    "abc123",
		"AZURE_SPEECH_REGION": "westeurope",
	}

	yaml := renderYAML(t, cfg)
	assert.Contains(t, yaml, `provider: "azure"`)
	assert.Contains(t, yaml, `azure:`)
	assert.Contains(t, yaml, `key:    "abc123"`)
	assert.Contains(t, yaml, `region: "westeurope"`)
}

// ── LLM providers ─────────────────────────────────────────────────────────────

func TestConfigWrite_LLM_Ollama(t *testing.T) {
	cfg := instconfig.Default()
	cfg.LLMProvider = "ollama"
	cfg.OllamaURL = "http://localhost:11434"
	cfg.OllamaModel = "qwen2.5:1.5b"
	cfg.LLMConfig = map[string]string{}

	yaml := renderYAML(t, cfg)
	assert.Contains(t, yaml, `provider: "ollama"`)
	assert.Contains(t, yaml, `ollama:`)
	assert.Contains(t, yaml, `base_url: "http://localhost:11434"`)
	assert.Contains(t, yaml, `model:    "qwen2.5:1.5b"`)
	assert.NotContains(t, yaml, "openai:")
	assert.NotContains(t, yaml, "anthropic:")
}

func TestConfigWrite_LLM_OpenAI(t *testing.T) {
	cfg := instconfig.Default()
	cfg.LLMProvider = "openai"
	cfg.LLMConfig = map[string]string{
		"LLM_API_KEY": "sk-test",
		"LLM_MODEL":   "gpt-4o",
	}

	yaml := renderYAML(t, cfg)
	assert.Contains(t, yaml, `provider: "openai"`)
	assert.Contains(t, yaml, `openai:`)
	assert.Contains(t, yaml, `api_key: "sk-test"`)
	assert.Contains(t, yaml, `model:   "gpt-4o"`)
	assert.NotContains(t, yaml, "anthropic:")
	assert.NotContains(t, yaml, "ollama:")
}

func TestConfigWrite_LLM_Anthropic(t *testing.T) {
	cfg := instconfig.Default()
	cfg.LLMProvider = "anthropic"
	cfg.LLMConfig = map[string]string{
		"LLM_API_KEY": "sk-ant-test",
		"LLM_MODEL":   "claude-sonnet-4-6",
	}

	yaml := renderYAML(t, cfg)
	assert.Contains(t, yaml, `provider: "anthropic"`)
	assert.Contains(t, yaml, `anthropic:`)
	assert.Contains(t, yaml, `api_key: "sk-ant-test"`)
	assert.Contains(t, yaml, `model:   "claude-sonnet-4-6"`)
	// Must NOT write openai: section for anthropic provider
	assert.NotContains(t, yaml, "openai:")
}

func TestConfigWrite_LLM_AzureOpenAI(t *testing.T) {
	cfg := instconfig.Default()
	cfg.LLMProvider = "azure"
	cfg.LLMConfig = map[string]string{
		"LLM_API_KEY":           "azure-key",
		"LLM_MODEL":             "gpt-4",
		"AZURE_OPENAI_ENDPOINT": "https://myco.openai.azure.com/",
	}

	yaml := renderYAML(t, cfg)
	assert.Contains(t, yaml, `provider: "azure"`)
	// Must have a dedicated azure: LLM section
	// (different from azure: STT section — here we check it's in the llm: block)
	assert.Contains(t, yaml, `endpoint: "https://myco.openai.azure.com/"`)
	assert.Contains(t, yaml, `model:    "gpt-4"`)
	assert.NotContains(t, yaml, "openai:")
	assert.NotContains(t, yaml, "anthropic:")
}

// ── Nil / empty map safety ─────────────────────────────────────────────────────

func TestConfigWrite_NilMaps_DoNotPanic(t *testing.T) {
	cfg := instconfig.Default()
	cfg.STTProvider = "google"
	cfg.STTConfig = nil // nil map — index should return ""
	cfg.LLMProvider = "openai"
	cfg.LLMConfig = nil

	assert.NotPanics(t, func() {
		renderYAML(t, cfg)
	})
}

// ── No cross-contamination ─────────────────────────────────────────────────────

func TestConfigWrite_OnlySelectedProviderWritten(t *testing.T) {
	cfg := instconfig.Default()
	cfg.LLMProvider = "anthropic"
	cfg.LLMConfig = map[string]string{"LLM_API_KEY": "k", "LLM_MODEL": "m"}

	yaml := renderYAML(t, cfg)

	// Exactly one LLM provider section must appear
	count := strings.Count(yaml, "api_key:")
	assert.Equal(t, 1, count, "only the selected provider section should appear")
}
