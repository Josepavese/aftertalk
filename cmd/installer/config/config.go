// Package config defines the InstallConfig struct — the SSOT for all
// aftertalk installation parameters. Every mode (interactive, env file,
// JSON, agent HTTP) produces one InstallConfig that is consumed by the
// step runner.
package config

import "time"

// LLMProfileEntry is a named provider profile (provider + optional model override).
type LLMProfileEntry struct {
	Provider          string         `json:"provider"`
	Model             string         `json:"model,omitempty"`
	APIKey            string         `json:"api_key,omitempty"`
	BaseURL           string         `json:"base_url,omitempty"`
	Endpoint          string         `json:"endpoint,omitempty"`
	Deployment        string         `json:"deployment,omitempty"`
	RequestTimeout    string         `json:"request_timeout,omitempty"`
	GenerationTimeout string         `json:"generation_timeout,omitempty"`
	MaxTokens         int            `json:"max_tokens,omitempty"`
	Retry             RetryEntry     `json:"retry,omitempty"`
	Reasoning         ReasoningEntry `json:"reasoning,omitempty"`
	Budget            BudgetEntry    `json:"budget,omitempty"`
	Think             *bool          `json:"think,omitempty"`
}

type RetryEntry struct {
	MaxAttempts    int    `json:"max_attempts,omitempty"`
	InitialBackoff string `json:"initial_backoff,omitempty"`
	MaxBackoff     string `json:"max_backoff,omitempty"`
}

type ReasoningEntry struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Effort  string `json:"effort,omitempty"`
	Exclude bool   `json:"exclude,omitempty"`
}

type BudgetEntry struct {
	MaxSessionCostCredits float64 `json:"max_session_cost_credits,omitempty"`
	MaxDailyCostCredits   float64 `json:"max_daily_cost_credits,omitempty"`
	AllowLocalFallback    bool    `json:"allow_local_fallback,omitempty"`
}

// STTProfileEntry is a named STT provider profile.
type STTProfileEntry struct {
	Provider string `json:"provider"`
	Model    string `json:"model,omitempty"`
	URL      string `json:"url,omitempty"`     // optional endpoint override
	APIKey   string `json:"api_key,omitempty"` // bearer token for cloud endpoints
}

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
	STTProvider     string            // google | aws | azure | whisper-local
	STTConfig       map[string]string // provider-specific env vars
	WhisperModel    string            // faster-whisper model size (default: base)
	WhisperURL      string            // URL aftertalk uses to reach whisper server
	WhisperLanguage string            // STT language code (default: "it")

	STTDefaultProfile string                     // profile used when session omits stt_profile
	STTProfiles       map[string]STTProfileEntry // named profiles

	// LLM provider
	LLMProvider string
	LLMConfig   map[string]string // LLM_API_KEY, LLM_MODEL, etc.
	OllamaModel string            // Ollama model to pull and use (default: qwen2.5:1.5b)
	OllamaURL   string            // Ollama base URL (default: http://localhost:11434)

	LLMDefaultProfile string                     // profile used when session omits llm_profile
	LLMProfiles       map[string]LLMProfileEntry // named profiles, e.g. {"local": ..., "cloud": ...}

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

	// WebRTC / ICE
	ICEUDPPortMin  uint16 // Pion ephemeral UDP port range min (default: 49200)
	ICEUDPPortMax  uint16 // Pion ephemeral UDP port range max (default: 49209)
	TURNEnabled    bool
	TURNListenAddr string
	TURNPublicIP   string
	TURNRealm      string
	TURNAuthSecret string
	TURNAuthTTL    int
	TURNEnableUDP  bool
	TURNEnableTCP  bool

	// TLS (optional — leave empty to run behind reverse proxy)
	TLSCertFile string
	TLSKeyFile  string

	// Apache reverse proxy (optional)
	ApacheVhostConf string // absolute path to SSL vhost file

	// Installation behavior flags
	SkipFirewall        bool
	SkipApache          bool // true when ApacheVhostConf is empty
	DryRun              bool
	ExpectedTag         string // optional runtime build tag verification after restart
	ExpectedCommit      string // optional runtime build commit verification after restart
	RequiredSTTProfiles []string
	RequiredLLMProfiles []string
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
		WhisperLanguage:          "it",
		LLMProvider:              "ollama",
		OllamaModel:              "qwen2.5:1.5b",
		OllamaURL:                "http://localhost:11434",
		ICEUDPPortMin:            49200,
		ICEUDPPortMax:            49209,
		WebhookMode:              "push",
		WebhookTokenTTL:          "1h",
		WebhookMaxRetries:        3,
		SessionMaxDuration:       (2 * time.Hour).String(),
		SessionInactivityTimeout: (10 * time.Minute).String(),
	}
}
