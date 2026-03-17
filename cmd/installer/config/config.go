// Package config defines the InstallConfig struct — the SSOT for all
// aftertalk installation parameters. Every mode (interactive, env file,
// JSON, agent HTTP) produces one InstallConfig that is consumed by the
// step runner.
package config

import "time"

// InstallConfig holds every configurable value that the installer needs.
// It maps 1:1 to the fields written into aftertalk.yaml and install.env.
type InstallConfig struct {
	// Infrastructure
	ServiceRoot string // install root, default /opt/aftertalk
	ServiceUser string // OS user that runs aftertalk, default "aftertalk"
	HTTPPort    int    // Aftertalk HTTP port, default 8080

	// Security
	APIKey    string
	JWTSecret string
	JWTIssuer string
	JWTExpiry string // Go duration string, e.g. "8h"

	// STT provider
	STTProvider  string            // google | aws | azure | whisper-local
	STTConfig    map[string]string // provider-specific env vars
	WhisperModel string            // faster-whisper model size (default: base)
	WhisperURL   string            // URL aftertalk uses to reach whisper server

	// LLM provider
	LLMProvider string
	LLMConfig   map[string]string // LLM_API_KEY, LLM_MODEL, etc.
	OllamaModel string            // Ollama model to pull and use (default: qwen2.5:1.5b)
	OllamaURL   string            // Ollama base URL (default: http://localhost:11434)

	// Webhook
	WebhookURL        string
	WebhookMode       string // push | notify_pull
	WebhookSecret     string
	WebhookPullBase   string
	WebhookTokenTTL   string
	WebhookMaxRetries int

	// Session tuning
	SessionMaxDuration       string // e.g. "1h10m"
	SessionInactivityTimeout string // e.g. "20m"

	// TLS (optional — leave empty to run behind reverse proxy)
	TLSCertFile string
	TLSKeyFile  string

	// Apache reverse proxy (optional)
	ApacheVhostConf string // absolute path to SSL vhost file

	// Installation behaviour flags
	SkipFirewall bool
	SkipApache   bool // true when ApacheVhostConf is empty
	DryRun       bool
}

// Default returns a config with sensible defaults for a fresh install.
func Default() *InstallConfig {
	return &InstallConfig{
		ServiceRoot:              "/opt/aftertalk",
		ServiceUser:              "aftertalk",
		HTTPPort:                 8080,
		JWTIssuer:                "aftertalk",
		JWTExpiry:                "8h",
		STTProvider:              "whisper-local",
		WhisperModel:             "base",
		WhisperURL:               "http://localhost:9001",
		LLMProvider:              "ollama",
		OllamaModel:              "qwen2.5:1.5b",
		OllamaURL:                "http://localhost:11434",
		WebhookMode:              "push",
		WebhookTokenTTL:          "1h",
		WebhookMaxRetries:        3,
		SessionMaxDuration:       (2 * time.Hour).String(),
		SessionInactivityTimeout: (10 * time.Minute).String(),
	}
}
