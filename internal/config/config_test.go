package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validBase returns a Default() config with valid JWT secret and API key,
// suitable for tests that only want to test one specific validation field.
func validBase() *Config {
	cfg := Default()
	cfg.JWT.Secret = "valid-test-secret"
	cfg.API.Key = "valid-test-api-key"
	return cfg
}

func TestConfigStructs(t *testing.T) {
	t.Run("DatabaseConfig", func(t *testing.T) {
		cfg := DatabaseConfig{
			Path: "test.db",
		}
		assert.Equal(t, "test.db", cfg.Path)
	})

	t.Run("HTTPConfig", func(t *testing.T) {
		cfg := HTTPConfig{
			Port: 8080,
			Host: "localhost",
		}
		assert.Equal(t, 8080, cfg.Port)
		assert.Equal(t, "localhost", cfg.Host)
	})

	t.Run("WebSocketConfig", func(t *testing.T) {
		cfg := WebSocketConfig{
			Port: 8081,
			Host: "0.0.0.0",
		}
		assert.Equal(t, 8081, cfg.Port)
		assert.Equal(t, "0.0.0.0", cfg.Host)
	})

	t.Run("LoggingConfig", func(t *testing.T) {
		cfg := LoggingConfig{
			Level:  "info",
			Format: "json",
		}
		assert.Equal(t, "info", cfg.Level)
		assert.Equal(t, "json", cfg.Format)
	})

	t.Run("JWTConfig", func(t *testing.T) {
		cfg := JWTConfig{
			Secret:     "test-secret",
			Issuer:     "aftertalk",
			Expiration: 2 * time.Hour,
		}
		assert.Equal(t, "test-secret", cfg.Secret)
		assert.Equal(t, "aftertalk", cfg.Issuer)
		assert.Equal(t, 2*time.Hour, cfg.Expiration)
	})

	t.Run("APIConfig", func(t *testing.T) {
		cfg := APIConfig{
			Key: "api-key-123",
		}
		assert.Equal(t, "api-key-123", cfg.Key)
	})

	t.Run("STTConfig", func(t *testing.T) {
		cfg := STTConfig{
			Provider: "google",
			Google: GoogleSTTConfig{
				CredentialsPath: "creds.json",
			},
		}
		assert.Equal(t, "google", cfg.Provider)
		assert.Equal(t, "creds.json", cfg.Google.CredentialsPath)
	})

	t.Run("LLMConfig", func(t *testing.T) {
		cfg := LLMConfig{
			Provider: "openai",
			OpenAI: OpenAIConfig{
				APIKey: "sk-test",
				Model:  "gpt-4",
			},
		}
		assert.Equal(t, "openai", cfg.Provider)
		assert.Equal(t, "gpt-4", cfg.OpenAI.Model)
	})

	t.Run("WebhookConfig", func(t *testing.T) {
		cfg := WebhookConfig{
			URL:     "https://example.com/webhook",
			Timeout: 30 * time.Second,
		}
		assert.Equal(t, "https://example.com/webhook", cfg.URL)
		assert.Equal(t, 30*time.Second, cfg.Timeout)
	})

	t.Run("ProcessingConfig", func(t *testing.T) {
		cfg := ProcessingConfig{
			MaxConcurrentTranscriptions:     10,
			MaxConcurrentMinutesGenerations: 5,
			TranscriptionTimeout:            10 * time.Minute,
			MinutesGenerationTimeout:        5 * time.Minute,
		}
		assert.Equal(t, 10, cfg.MaxConcurrentTranscriptions)
		assert.Equal(t, 5, cfg.MaxConcurrentMinutesGenerations)
		assert.Equal(t, 10*time.Minute, cfg.TranscriptionTimeout)
		assert.Equal(t, 5*time.Minute, cfg.MinutesGenerationTimeout)
	})

	t.Run("SessionConfig", func(t *testing.T) {
		cfg := SessionConfig{
			MaxDuration:               2 * time.Hour,
			MaxParticipantsPerSession: 10,
		}
		assert.Equal(t, 2*time.Hour, cfg.MaxDuration)
		assert.Equal(t, 10, cfg.MaxParticipantsPerSession)
	})

	t.Run("RetentionConfig", func(t *testing.T) {
		cfg := RetentionConfig{
			TranscriptionDays: 90,
			MinutesDays:       90,
			WebhookEventsDays: 30,
		}
		assert.Equal(t, 90, cfg.TranscriptionDays)
		assert.Equal(t, 90, cfg.MinutesDays)
		assert.Equal(t, 30, cfg.WebhookEventsDays)
	})

	t.Run("PerformanceConfig", func(t *testing.T) {
		cfg := PerformanceConfig{
			EnableProfiling: true,
			ProfilingPort:   6060,
		}
		assert.True(t, cfg.EnableProfiling)
		assert.Equal(t, 6060, cfg.ProfilingPort)
	})
}

func TestConfig_Default(t *testing.T) {
	cfg := Default()

	assert.Equal(t, "./aftertalk.db", cfg.Database.Path)
	assert.Equal(t, 8080, cfg.HTTP.Port)
	assert.Equal(t, "0.0.0.0", cfg.HTTP.Host)
	assert.Equal(t, 8081, cfg.WebSocket.Port)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, "json", cfg.Logging.Format)
	assert.Equal(t, "aftertalk", cfg.JWT.Issuer)
	assert.Equal(t, "change-this-in-production", cfg.JWT.Secret)
	assert.Equal(t, 2*time.Hour, cfg.JWT.Expiration)
	assert.Equal(t, "your-api-key-change-this-in-production", cfg.API.Key)
	assert.Equal(t, "google", cfg.STT.Provider)
	assert.Equal(t, "openai", cfg.LLM.Provider)
	assert.Equal(t, 10, cfg.Processing.MaxConcurrentTranscriptions)
	assert.Equal(t, 2*time.Hour, cfg.Session.MaxDuration)
	assert.Equal(t, 90, cfg.Retention.TranscriptionDays)
	assert.False(t, cfg.Performance.EnableProfiling)
}

func TestConfig_EmptyDatabasePath(t *testing.T) {
	cfg := Default()
	cfg.Database.Path = ""
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "database path is required")
}

func TestConfig_InvalidHTTPPort(t *testing.T) {
	testCases := []struct {
		name  string
		port  int
		error bool
	}{
		{"Port 0", 0, true},
		{"Port 65536", 65536, true},
		{"Port -1", -1, true},
		{"Valid Port 8080", 8080, false},
		{"Valid Port 443", 443, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validBase()
			cfg.HTTP.Port = tc.port
			err := validate(cfg)
			if tc.error {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_InvalidWebSocketPort(t *testing.T) {
	testCases := []struct {
		name  string
		port  int
		error bool
	}{
		{"Port 0", 0, true},
		{"Port 65536", 65536, true},
		{"Port -1", -1, true},
		{"Valid Port 8081", 8081, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validBase()
			cfg.WebSocket.Port = tc.port
			err := validate(cfg)
			if tc.error {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_DefaultJWTSecret(t *testing.T) {
	cfg := Default()
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "JWT secret must be changed from default value")
}

func TestConfig_DefaultAPIKey(t *testing.T) {
	cfg := Default()
	cfg.JWT.Secret = "valid-test-secret" // bypass JWT check to reach API key check
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key must be changed from default value")
}

func TestConfig_InvalidLogLevel(t *testing.T) {
	testCases := []string{"debug", "info", "warn", "error", "invalid"}
	expected := []string{"debug", "info", "warn", "error", "invalid"}

	for i, level := range testCases {
		t.Run(level, func(t *testing.T) {
			cfg := validBase()
			cfg.Logging.Level = level
			err := validate(cfg)
			assert.Equal(t, expected[i] == "invalid", err != nil)
		})
	}
}

func TestConfig_InvalidLogFormat(t *testing.T) {
	testCases := []string{"json", "console", "invalid"}
	expected := []string{"json", "console", "invalid"}

	for i, format := range testCases {
		t.Run(format, func(t *testing.T) {
			cfg := validBase()
			cfg.Logging.Format = format
			err := validate(cfg)
			assert.Equal(t, expected[i] == "invalid", err != nil)
		})
	}
}

func TestConfig_InvalidSTTProvider(t *testing.T) {
	cfg := validBase()
	cfg.STT.Provider = "invalid-provider"
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid STT provider")
}

func TestConfig_ValidSTTProviders(t *testing.T) {
	providers := []string{"google", "aws", "azure"}
	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			cfg := validBase()
			cfg.STT.Provider = provider
			err := validate(cfg)
			assert.NoError(t, err)
		})
	}
}

func TestConfig_InvalidLLMProvider(t *testing.T) {
	cfg := validBase()
	cfg.LLM.Provider = "invalid-provider"
	err := validate(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid LLM provider")
}

func TestConfig_ValidLLMProviders(t *testing.T) {
	providers := []string{"openai", "anthropic", "azure"}
	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			cfg := validBase()
			cfg.LLM.Provider = provider
			err := validate(cfg)
			assert.NoError(t, err)
		})
	}
}

func TestConfig_NestedConfigs(t *testing.T) {
	cfg := Default()

	assert.Equal(t, "creds.json", cfg.STT.Google.CredentialsPath)
	assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", cfg.STT.AWS.AccessKeyID)
	assert.Equal(t, "secret-access-key", cfg.STT.AWS.SecretAccessKey)
	assert.Equal(t, "us-east-1", cfg.STT.AWS.Region)
	assert.Equal(t, "key-123", cfg.STT.Azure.Key)
	assert.Equal(t, "eastus", cfg.STT.Azure.Region)

	assert.Equal(t, "sk-test", cfg.LLM.OpenAI.APIKey)
	assert.Equal(t, "gpt-4", cfg.LLM.OpenAI.Model)
	assert.Equal(t, "sk-test", cfg.LLM.Anthropic.APIKey)
	assert.Equal(t, "claude-2", cfg.LLM.Anthropic.Model)
	assert.Equal(t, "sk-test", cfg.LLM.Azure.APIKey)
	assert.Equal(t, "https://example.com/openai", cfg.LLM.Azure.Endpoint)
	assert.Equal(t, "gpt-4", cfg.LLM.Azure.Deployment)
}

func TestConfig_EdgeCases(t *testing.T) {
	t.Run("ZeroDuration", func(t *testing.T) {
		cfg := Default()
		cfg.JWT.Expiration = 0
		err := validate(cfg)
		require.Error(t, err)
	})

	t.Run("ZeroTimeout", func(t *testing.T) {
		cfg := Default()
		cfg.Webhook.Timeout = 0
		err := validate(cfg)
		require.Error(t, err)
	})

	t.Run("EmptyWebhookURL", func(t *testing.T) {
		cfg := Default()
		cfg.Webhook.URL = ""
		err := validate(cfg)
		require.Error(t, err)
	})

	t.Run("ZeroMaxConcurrent", func(t *testing.T) {
		cfg := Default()
		cfg.Processing.MaxConcurrentTranscriptions = 0
		err := validate(cfg)
		require.Error(t, err)
	})

	t.Run("NegativeMaxConcurrent", func(t *testing.T) {
		cfg := Default()
		cfg.Processing.MaxConcurrentTranscriptions = -1
		err := validate(cfg)
		require.Error(t, err)
	})

	t.Run("EmptyCredentialsPath", func(t *testing.T) {
		cfg := validBase()
		cfg.STT.Google.CredentialsPath = ""
		err := validate(cfg)
		assert.NoError(t, err)
	})
}

func TestConfig_AllowedConfigurations(t *testing.T) {
	cfg := Default()
	cfg.Database.Path = "/tmp/test.db"
	cfg.HTTP.Port = 443
	cfg.WebSocket.Port = 9443
	cfg.JWT.Secret = "my-super-secret-key"
	cfg.API.Key = "my-api-key"
	cfg.Logging.Level = "debug"
	cfg.Logging.Format = "console"
	cfg.STT.Provider = "aws"
	cfg.LLM.Provider = "anthropic"

	err := validate(cfg)
	assert.NoError(t, err)
}
