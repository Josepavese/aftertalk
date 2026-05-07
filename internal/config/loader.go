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
	for name, profile := range cfg.LLM.Profiles {
		if !validLLMProviders[profile.Provider] {
			return fmt.Errorf("%w in profile %q: %s (supported: openai, anthropic, azure, ollama, stub)", errInvalidLLMProvider, name, profile.Provider)
		}
		if err := validateLLMProfile(name, profile, cfg.LLM); err != nil {
			return err
		}
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

func validateLLMProfile(name string, profile LLMProfileConfig, llm LLMConfig) error {
	if profile.RequestTimeout < 0 {
		return fmt.Errorf("llm.profiles.%s.request_timeout must be non-negative", name)
	}
	if profile.GenerationTimeout < 0 {
		return fmt.Errorf("llm.profiles.%s.generation_timeout must be non-negative", name)
	}
	if profile.Retry.MaxAttempts < 0 {
		return fmt.Errorf("llm.profiles.%s.retry.max_attempts must be non-negative", name)
	}
	if profile.Retry.InitialBackoff < 0 {
		return fmt.Errorf("llm.profiles.%s.retry.initial_backoff must be non-negative", name)
	}
	if profile.Retry.MaxBackoff < 0 {
		return fmt.Errorf("llm.profiles.%s.retry.max_backoff must be non-negative", name)
	}
	switch profile.Provider {
	case "openai":
		apiKey := firstNonEmpty(profile.APIKey, llm.OpenAI.APIKey)
		model := firstNonEmpty(profile.Model, llm.OpenAI.Model)
		if apiKey == "" {
			return fmt.Errorf("llm.profiles.%s.api_key is required for openai profile", name)
		}
		if model == "" {
			return fmt.Errorf("llm.profiles.%s.model is required for openai profile", name)
		}
	case "anthropic":
		apiKey := firstNonEmpty(profile.APIKey, llm.Anthropic.APIKey)
		model := firstNonEmpty(profile.Model, llm.Anthropic.Model)
		if apiKey == "" {
			return fmt.Errorf("llm.profiles.%s.api_key is required for anthropic profile", name)
		}
		if model == "" {
			return fmt.Errorf("llm.profiles.%s.model is required for anthropic profile", name)
		}
	case "azure":
		apiKey := firstNonEmpty(profile.APIKey, llm.Azure.APIKey)
		endpoint := firstNonEmpty(profile.Endpoint, llm.Azure.Endpoint)
		deployment := firstNonEmpty(profile.Deployment, profile.Model, llm.Azure.Deployment)
		if apiKey == "" {
			return fmt.Errorf("llm.profiles.%s.api_key is required for azure profile", name)
		}
		if endpoint == "" {
			return fmt.Errorf("llm.profiles.%s.endpoint is required for azure profile", name)
		}
		if deployment == "" {
			return fmt.Errorf("llm.profiles.%s.deployment is required for azure profile", name)
		}
	case "ollama":
		model := firstNonEmpty(profile.Model, llm.Ollama.Model)
		if model == "" {
			return fmt.Errorf("llm.profiles.%s.model is required for ollama profile", name)
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
