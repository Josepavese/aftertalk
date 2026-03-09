package config

import "time"

type Config struct {
	Database    DatabaseConfig
	HTTP        HTTPConfig
	WebSocket   WebSocketConfig
	Logging     LoggingConfig
	JWT         JWTConfig
	API         APIConfig
	STT         STTConfig
	LLM         LLMConfig
	Webhook     WebhookConfig
	Processing  ProcessingConfig
	Session     SessionConfig
	Retention   RetentionConfig
	Performance PerformanceConfig
}

type DatabaseConfig struct {
	Path string `koanf:"path"`
}

type HTTPConfig struct {
	Port int    `koanf:"port"`
	Host string `koanf:"host"`
}

type WebSocketConfig struct {
	Port int    `koanf:"port"`
	Host string `koanf:"host"`
}

type LoggingConfig struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}

type JWTConfig struct {
	Secret     string        `koanf:"secret"`
	Issuer     string        `koanf:"issuer"`
	Expiration time.Duration `koanf:"expiration"`
}

type APIConfig struct {
	Key string `koanf:"key"`
}

type STTConfig struct {
	Provider     string `koanf:"provider"`
	Google       GoogleSTTConfig
	AWS          AWSSTTConfig
	Azure        AzureSTTConfig
	WhisperLocal WhisperLocalSTTConfig
}

type GoogleSTTConfig struct {
	CredentialsPath string `koanf:"credentials_path"`
}

type AWSSTTConfig struct {
	AccessKeyID     string `koanf:"access_key_id"`
	SecretAccessKey string `koanf:"secret_access_key"`
	Region          string `koanf:"region"`
}

type AzureSTTConfig struct {
	Key    string `koanf:"key"`
	Region string `koanf:"region"`
}

type WhisperLocalSTTConfig struct {
	URL            string `koanf:"url"`
	Model          string `koanf:"model"`
	Language       string `koanf:"language"`
	ResponseFormat string `koanf:"response_format"`
}

type LLMConfig struct {
	Provider  string `koanf:"provider"`
	OpenAI    OpenAIConfig
	Anthropic AnthropicConfig
	Azure     AzureLLMConfig
	Ollama    OllamaLLMConfig
}

type OllamaLLMConfig struct {
	BaseURL string `koanf:"base_url"`
	Model   string `koanf:"model"`
}

type OpenAIConfig struct {
	APIKey string `koanf:"api_key"`
	Model  string `koanf:"model"`
}

type AnthropicConfig struct {
	APIKey string `koanf:"api_key"`
	Model  string `koanf:"model"`
}

type AzureLLMConfig struct {
	APIKey     string `koanf:"api_key"`
	Endpoint   string `koanf:"endpoint"`
	Deployment string `koanf:"deployment"`
}

type WebhookConfig struct {
	URL     string        `koanf:"url"`
	Timeout time.Duration `koanf:"timeout"`
}

type ProcessingConfig struct {
	MaxConcurrentTranscriptions     int           `koanf:"max_concurrent_transcriptions"`
	MaxConcurrentMinutesGenerations int           `koanf:"max_concurrent_minutes_generations"`
	TranscriptionTimeout            time.Duration `koanf:"transcription_timeout"`
	MinutesGenerationTimeout        time.Duration `koanf:"minutes_generation_timeout"`
}

type SessionConfig struct {
	MaxDuration               time.Duration `koanf:"max_duration"`
	MaxParticipantsPerSession int           `koanf:"max_participants_per_session"`
}

type RetentionConfig struct {
	TranscriptionDays int `koanf:"transcription_days"`
	MinutesDays       int `koanf:"minutes_days"`
	WebhookEventsDays int `koanf:"webhook_events_days"`
}

type PerformanceConfig struct {
	EnableProfiling bool `koanf:"enable_profiling"`
	ProfilingPort   int  `koanf:"profiling_port"`
}

func Default() *Config {
	return &Config{
		Database: DatabaseConfig{
			Path: "./aftertalk.db",
		},
		HTTP: HTTPConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
		WebSocket: WebSocketConfig{
			Port: 8081,
			Host: "0.0.0.0",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		JWT: JWTConfig{
			Secret:     "change-this-in-production",
			Issuer:     "aftertalk",
			Expiration: 2 * time.Hour,
		},
		API: APIConfig{
			Key: "your-api-key-change-this-in-production",
		},
		STT: STTConfig{
			Provider: "google",
			Google: GoogleSTTConfig{
				CredentialsPath: "creds.json",
			},
			AWS: AWSSTTConfig{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "secret-access-key",
				Region:          "us-east-1",
			},
			Azure: AzureSTTConfig{
				Key:    "key-123",
				Region: "eastus",
			},
		},
		LLM: LLMConfig{
			Provider: "openai",
			OpenAI: OpenAIConfig{
				APIKey: "sk-test",
				Model:  "gpt-4",
			},
			Anthropic: AnthropicConfig{
				APIKey: "sk-test",
				Model:  "claude-2",
			},
			Azure: AzureLLMConfig{
				APIKey:     "sk-test",
				Endpoint:   "https://example.com/openai",
				Deployment: "gpt-4",
			},
		},
		Webhook: WebhookConfig{
			URL:     "https://example.com/webhook",
			Timeout: 30 * time.Second,
		},
		Processing: ProcessingConfig{
			MaxConcurrentTranscriptions:     10,
			MaxConcurrentMinutesGenerations: 5,
			TranscriptionTimeout:            10 * time.Minute,
			MinutesGenerationTimeout:        5 * time.Minute,
		},
		Session: SessionConfig{
			MaxDuration:               2 * time.Hour,
			MaxParticipantsPerSession: 10,
		},
		Retention: RetentionConfig{
			TranscriptionDays: 90,
			MinutesDays:       90,
			WebhookEventsDays: 30,
		},
		Performance: PerformanceConfig{
			EnableProfiling: false,
			ProfilingPort:   6060,
		},
	}
}
