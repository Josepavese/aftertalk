package config

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
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

func validate(cfg *Config) error {
	if cfg.Database.Path == "" {
		return fmt.Errorf("database path is required")
	}

	if cfg.HTTP.Port <= 0 || cfg.HTTP.Port > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", cfg.HTTP.Port)
	}

	if cfg.WebSocket.Port <= 0 || cfg.WebSocket.Port > 65535 {
		return fmt.Errorf("invalid WebSocket port: %d", cfg.WebSocket.Port)
	}

	if cfg.JWT.Secret == "change-this-in-production" {
		return fmt.Errorf("JWT secret must be changed from default value")
	}

	if cfg.API.Key == "your-api-key-change-this-in-production" {
		return fmt.Errorf("API key must be changed from default value")
	}

	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[cfg.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", cfg.Logging.Level)
	}

	validLogFormats := map[string]bool{
		"json":    true,
		"console": true,
	}
	if !validLogFormats[cfg.Logging.Format] {
		return fmt.Errorf("invalid log format: %s", cfg.Logging.Format)
	}

	validSTTProviders := map[string]bool{
		"google": true,
		"aws":    true,
		"azure":  true,
	}
	if !validSTTProviders[cfg.STT.Provider] {
		return fmt.Errorf("invalid STT provider: %s", cfg.STT.Provider)
	}

	validLLMProviders := map[string]bool{
		"openai":    true,
		"anthropic": true,
		"azure":     true,
	}
	if !validLLMProviders[cfg.LLM.Provider] {
		return fmt.Errorf("invalid LLM provider: %s", cfg.LLM.Provider)
	}

	return nil
}
