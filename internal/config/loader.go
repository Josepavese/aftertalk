package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

var (
	errDatabasePathRequired  = errors.New("database path is required")
	errJWTSecretDefault      = errors.New("JWT secret must be changed from default value")
	errAPIKeyDefault         = errors.New("API key must be changed from default value")
	errJWTExpirationPositive = errors.New("JWT expiration must be positive")
	// errWebhookURLRequired intentionally removed: empty webhook URL is allowed (delivery disabled).
	errWebhookTimeoutPositive              = errors.New("webhook timeout must be positive")
	errMaxConcurrentTranscriptionsPositive = errors.New("max concurrent transcriptions must be positive")
	errInvalidHTTPPort                     = errors.New("invalid HTTP port")
	errInvalidWebSocketPort                = errors.New("invalid WebSocket port")
	errInvalidLogLevel                     = errors.New("invalid log level")
	errInvalidLogFormat                    = errors.New("invalid log format")
	errInvalidSTTProvider                  = errors.New("invalid STT provider")
	errInvalidLLMProvider                  = errors.New("invalid LLM provider")
)

func Load(configPath string) (*Config, error) {
	k := koanf.New(".")
	cfg := Default()

	if configPath != "" {
		if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	if err := k.Load(env.Provider("AFTERTALK_", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(s, "AFTERTALK_")), "_", ".")
	}), nil); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	if err := k.Unmarshal("", cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func validate(cfg *Config) error { //nolint:gocyclo // validation function needs to check all fields
	if cfg.Database.Path == "" {
		return errDatabasePathRequired
	}

	if cfg.HTTP.Port <= 0 || cfg.HTTP.Port > 65535 {
		return fmt.Errorf("%w: %d", errInvalidHTTPPort, cfg.HTTP.Port)
	}

	if cfg.WebSocket.Port <= 0 || cfg.WebSocket.Port > 65535 {
		return fmt.Errorf("%w: %d", errInvalidWebSocketPort, cfg.WebSocket.Port)
	}

	if cfg.JWT.Secret == "change-this-in-production" {
		return errJWTSecretDefault
	}

	if cfg.API.Key == "your-api-key-change-this-in-production" {
		return errAPIKeyDefault
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[cfg.Logging.Level] {
		return fmt.Errorf("%w: %s", errInvalidLogLevel, cfg.Logging.Level)
	}

	validLogFormats := map[string]bool{
		"json":    true,
		"console": true,
	}
	if !validLogFormats[cfg.Logging.Format] {
		return fmt.Errorf("%w: %s", errInvalidLogFormat, cfg.Logging.Format)
	}

	validSTTProviders := map[string]bool{
		"google":        true,
		"aws":           true,
		"azure":         true,
		"whisper-local": true,
		"openai":        true,
		"stub":          true,
	}
	if !validSTTProviders[cfg.STT.Provider] {
		return fmt.Errorf("%w: %s (supported: google, aws, azure, whisper-local, openai, stub)", errInvalidSTTProvider, cfg.STT.Provider)
	}

	validLLMProviders := map[string]bool{
		"openai":    true,
		"anthropic": true,
		"azure":     true,
		"ollama":    true,
		"stub":      true,
	}
	if !validLLMProviders[cfg.LLM.Provider] {
		return fmt.Errorf("%w: %s (supported: openai, anthropic, azure, ollama, stub)", errInvalidLLMProvider, cfg.LLM.Provider)
	}

	if cfg.JWT.Expiration <= 0 {
		return errJWTExpirationPositive
	}

	if cfg.Webhook.Timeout <= 0 {
		return errWebhookTimeoutPositive
	}

	if cfg.Processing.MaxConcurrentTranscriptions <= 0 {
		return errMaxConcurrentTranscriptionsPositive
	}

	return nil
}
