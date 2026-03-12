package config

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// RoleConfig defines a participant role within a template.
type RoleConfig struct {
	Key   string `koanf:"key"`   // machine key, e.g. "therapist"
	Label string `koanf:"label"` // human label, e.g. "Terapeuta"
}

// SectionConfig defines one section of the meeting minutes.
type SectionConfig struct {
	Key         string `koanf:"key"`         // JSON key in LLM output
	Label       string `koanf:"label"`       // human-readable label
	Description string `koanf:"description"` // instruction for the LLM
	// Type controls the expected JSON shape:
	//   "string_list"   → []string
	//   "content_items" → [{"text":"...","timestamp":0}]
	//   "progress"      → {"progress":[...],"issues":[...]}
	Type string `koanf:"type"`
}

// TemplateConfig defines roles and minutes structure for a session context.
type TemplateConfig struct {
	ID          string          `koanf:"id"`
	Name        string          `koanf:"name"`
	Description string          `koanf:"description"`
	Roles       []RoleConfig    `koanf:"roles"`
	Sections    []SectionConfig `koanf:"sections"`
}

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
	WebRTC      WebRTCConfig     `koanf:"webrtc"`
	Templates   []TemplateConfig `koanf:"templates"`
	Demo        DemoConfig       `koanf:"demo"`
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
	Key       string          `koanf:"key"`
	CORS      CORSConfig      `koanf:"cors"`
	RateLimit RateLimitConfig `koanf:"rate_limit"`
}

// CORSConfig controls the Cross-Origin Resource Sharing policy.
// Set AllowedOrigins to specific domains in production.
type CORSConfig struct {
	// AllowedOrigins is the list of allowed origins. Use ["*"] for development.
	AllowedOrigins   []string `koanf:"allowed_origins"`
	AllowedMethods   []string `koanf:"allowed_methods"`
	AllowedHeaders   []string `koanf:"allowed_headers"`
	AllowCredentials bool     `koanf:"allow_credentials"`
}

// RateLimitConfig caps requests per IP per minute across the API.
type RateLimitConfig struct {
	Enabled           bool `koanf:"enabled"`
	RequestsPerMinute int  `koanf:"requests_per_minute"`
}

// DemoConfig controls the embedded demo/test UI and its public endpoints.
type DemoConfig struct {
	// Enabled exposes /demo/config with the API key included — for local demos only.
	// Never enable this in production.
	Enabled bool `koanf:"enabled"`
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
	Endpoint       string `koanf:"endpoint"`
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
	LLMMaxRetries                   int           `koanf:"llm_max_retries"`
	LLMInitialBackoff               time.Duration `koanf:"llm_initial_backoff"`
	LLMMaxBackoff                   time.Duration `koanf:"llm_max_backoff"`
	TranscriptionQueueSize          int           `koanf:"transcription_queue_size"`
	ChunkSizeMs                     int           `koanf:"chunk_size_ms"`
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

// ICEServerConfig defines a single ICE server (STUN or TURN).
type ICEServerConfig struct {
	URLs       []string `koanf:"urls"`
	Username   string   `koanf:"username"`   // required for TURN
	Credential string   `koanf:"credential"` // required for TURN
}

// TwilioICEConfig holds credentials for the Twilio Network Traversal Service.
type TwilioICEConfig struct {
	AccountSID string `koanf:"account_sid"`
	AuthToken  string `koanf:"auth_token"`
}

// XirsysICEConfig holds credentials for the Xirsys TURN network.
type XirsysICEConfig struct {
	Ident   string `koanf:"ident"`
	Secret  string `koanf:"secret"`
	Channel string `koanf:"channel"`
}

// MeteredICEConfig holds credentials for the Metered.ca TURN service.
type MeteredICEConfig struct {
	AppName string `koanf:"app_name"`
	APIKey  string `koanf:"api_key"`
}

// WebRTCConfig holds WebRTC-related settings.
type WebRTCConfig struct {
	// ICEProviderName selects the ICE credential source.
	// Valid values: "static" (default), "embedded", "twilio", "xirsys", "metered".
	ICEProviderName string            `koanf:"ice_provider"`
	ICEServers      []ICEServerConfig `koanf:"ice_servers"` // used by "static" provider
	TURN            TURNServerConfig  `koanf:"turn"`        // used by "embedded" provider
	Twilio          TwilioICEConfig   `koanf:"twilio"`
	Xirsys          XirsysICEConfig   `koanf:"xirsys"`
	Metered         MeteredICEConfig  `koanf:"metered"`
}

// TURNServerConfig configures the optional embedded TURN server (pion/turn).
// When Enabled is true, aftertalk runs a TURN server on ListenAddr so that
// clients behind symmetric NAT or strict firewalls can relay media through it.
type TURNServerConfig struct {
	Enabled    bool   `koanf:"enabled"`
	ListenAddr string `koanf:"listen_addr"` // e.g. "0.0.0.0:3478"
	PublicIP   string `koanf:"public_ip"`   // public IP to advertise; "" = auto-detect
	Realm      string `koanf:"realm"`
	// AuthSecret is the HMAC-SHA1 shared secret used to generate time-limited
	// TURN credentials (RFC 5766 REST API). If empty, a random secret is generated.
	AuthSecret string `koanf:"auth_secret"`
	AuthTTL    int    `koanf:"auth_ttl"` // credential lifetime in seconds (default 86400)
	EnableUDP  bool   `koanf:"enable_udp"`
	EnableTCP  bool   `koanf:"enable_tcp"`
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
			CORS: CORSConfig{
				AllowedOrigins:   []string{"*"},
				AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders:   []string{"Authorization", "Content-Type", "X-API-Key", "X-Request-ID", "X-User-ID"},
				AllowCredentials: false,
			},
			RateLimit: RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 100,
			},
		},
		Demo: DemoConfig{
			Enabled: false,
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
			LLMMaxRetries:                   3,
			LLMInitialBackoff:               1 * time.Second,
			LLMMaxBackoff:                   10 * time.Second,
			TranscriptionQueueSize:          100,
			ChunkSizeMs:                     15000,
		},
		WebRTC: WebRTCConfig{
			ICEServers: []ICEServerConfig{
				{URLs: []string{"stun:stun.l.google.com:19302"}},
			},
			TURN: TURNServerConfig{
				Enabled:    false,
				ListenAddr: "0.0.0.0:3478",
				Realm:      "aftertalk",
				AuthTTL:    86400,
				EnableUDP:  true,
				EnableTCP:  true,
			},
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
		Templates: DefaultTemplates(),
	}
}

// DefaultTemplates returns the built-in session templates.
// Operators can extend this list via config.yaml under the "templates" key.
func DefaultTemplates() []TemplateConfig {
	return []TemplateConfig{
		{
			ID:          "therapy",
			Name:        "Seduta Terapeutica",
			Description: "Template per sessioni di psicoterapia individuale",
			Roles: []RoleConfig{
				{Key: "therapist", Label: "Terapeuta"},
				{Key: "patient", Label: "Paziente"},
			},
			Sections: []SectionConfig{
				{
					Key:         "themes",
					Label:       "Temi",
					Description: "Main topics and themes that emerged during the session",
					Type:        "string_list",
				},
				{
					Key:         "contents_reported",
					Label:       "Contenuti riportati",
					Description: "What the patient reported, described, or discussed",
					Type:        "content_items",
				},
				{
					Key:         "professional_interventions",
					Label:       "Interventi professionali",
					Description: "What the therapist said, asked, or did during the session",
					Type:        "content_items",
				},
				{
					Key:         "progress_issues",
					Label:       "Progressi e problemi",
					Description: "Progress observed and issues or difficulties identified",
					Type:        "progress",
				},
				{
					Key:         "next_steps",
					Label:       "Prossimi passi",
					Description: "Action items, homework, or goals agreed upon for the next session",
					Type:        "string_list",
				},
			},
		},
		{
			ID:          "consulting",
			Name:        "Consulenza Professionale",
			Description: "Template generico per consulenze (commercialista, legale, ecc.)",
			Roles: []RoleConfig{
				{Key: "consultant", Label: "Consulente"},
				{Key: "client", Label: "Cliente"},
			},
			Sections: []SectionConfig{
				{
					Key:         "themes",
					Label:       "Argomenti trattati",
					Description: "Main topics discussed during the consulting session",
					Type:        "string_list",
				},
				{
					Key:         "client_needs",
					Label:       "Esigenze del cliente",
					Description: "What the client reported, requested, or asked about",
					Type:        "content_items",
				},
				{
					Key:         "professional_advice",
					Label:       "Consigli professionali",
					Description: "Advice, analysis, or information provided by the consultant",
					Type:        "content_items",
				},
				{
					Key:         "progress_issues",
					Label:       "Stato e problemi",
					Description: "Current status of the client's situation and open issues",
					Type:        "progress",
				},
				{
					Key:         "next_steps",
					Label:       "Prossimi passi",
					Description: "Actions to take, deadlines, documents to prepare",
					Type:        "string_list",
				},
			},
		},
	}
}

// DumpYAML marshals the default Config to annotated YAML suitable for use as
// a starter config file. Returns an error only if marshalling fails (never in
// practice for a static struct).
func DumpYAML() (string, error) {
	cfg := Default()
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}
	return string(out), nil
}
