package config

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// RoleConfig defines a participant role within a template.
type RoleConfig struct {
	Key   string `koanf:"key"   json:"key"`   // machine key, e.g. "therapist"
	Label string `koanf:"label" json:"label"` // human label, e.g. "Terapeuta"
}

// SectionConfig defines one section of the meeting minutes.
type SectionConfig struct {
	Key         string `koanf:"key"         json:"key"`
	Label       string `koanf:"label"       json:"label"`
	Description string `koanf:"description" json:"description"`
	// Type controls the expected JSON shape:
	//   "string_list"   → []string
	//   "content_items" → [{"text":"...","timestamp":0}]
	//   "progress"      → {"progress":[...],"issues":[...]}
	Type string `koanf:"type" json:"type"`
}

// TemplateConfig defines roles and minutes structure for a session context.
type TemplateConfig struct {
	ID          string          `koanf:"id"          json:"id"`
	Name        string          `koanf:"name"        json:"name"`
	Description string          `koanf:"description" json:"description"`
	Roles       []RoleConfig    `koanf:"roles"       json:"roles"`
	Sections    []SectionConfig `koanf:"sections"    json:"sections"`
}

type Config struct {
	WebRTC      WebRTCConfig `koanf:"webrtc"`
	STT         STTConfig
	LLM         LLMConfig
	Webhook     WebhookConfig
	Logging     LoggingConfig
	HTTP        HTTPConfig
	TLS         TLSConfig
	WebSocket   WebSocketConfig
	Database    DatabaseConfig
	JWT         JWTConfig
	Templates   []TemplateConfig `koanf:"templates"`
	API         APIConfig
	Processing  ProcessingConfig
	Retention   RetentionConfig
	Session     SessionConfig
	Performance PerformanceConfig
}

type DatabaseConfig struct {
	Path string `koanf:"path"`
}

type HTTPConfig struct {
	Host string `koanf:"host"`
	Port int    `koanf:"port"`
}

// TLSConfig enables HTTPS/WSS directly on the Aftertalk server.
// When CertFile and KeyFile are both set and the files exist, the server
// calls ListenAndServeTLS instead of ListenAndServe.
// Leave empty to run as plain HTTP (e.g. when TLS is terminated by a
// reverse proxy such as Apache or nginx in front of Aftertalk).
type TLSConfig struct {
	CertFile string `koanf:"cert_file"`
	KeyFile  string `koanf:"key_file"`
}

type WebSocketConfig struct {
	Host string `koanf:"host"`
	Port int    `koanf:"port"`
}

type LoggingConfig struct {
	Level     string                 `koanf:"level"`
	Format    string                 `koanf:"format"`
	Output    LoggingOutputConfig    `koanf:"output"`
	Rotation  LoggingRotationConfig  `koanf:"rotation"`
	Retention LoggingRetentionConfig `koanf:"retention"`
	Redaction LoggingRedactionConfig `koanf:"redaction"`
}

type LoggingOutputConfig struct {
	Stdout bool              `koanf:"stdout"`
	File   LoggingFileConfig `koanf:"file"`
}

type LoggingFileConfig struct {
	Enabled   bool   `koanf:"enabled"`
	Path      string `koanf:"path"`
	Mandatory bool   `koanf:"mandatory"`
}

type LoggingRotationConfig struct {
	MaxSizeMB  int  `koanf:"max_size_mb"`
	MaxAgeDays int  `koanf:"max_age_days"`
	MaxBackups int  `koanf:"max_backups"`
	Compress   bool `koanf:"compress"`
}

type LoggingRetentionConfig struct {
	DeleteAfterDays       int `koanf:"delete_after_days"`
	EmergencyCutoffSizeMB int `koanf:"emergency_cutoff_size_mb"`
}

type LoggingRedactionConfig struct {
	Enabled bool     `koanf:"enabled"`
	Fields  []string `koanf:"fields"`
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

// STTProfileConfig selects a provider (and optionally overrides the model, URL,
// and API key) for a named quality/cost tier. Fields left empty inherit from
// the parent STTConfig section.
type STTProfileConfig struct {
	Provider string `koanf:"provider"` // whisper-local | google | aws | azure | stub
	Model    string `koanf:"model"`    // optional model override
	URL      string `koanf:"url"`      // optional URL override (e.g. https://openrouter.ai/api for cloud whisper)
	APIKey   string `koanf:"api_key"`  // optional bearer token (required for cloud endpoints)
}

type STTConfig struct {
	Provider       string                      `koanf:"provider"`        // legacy default provider
	DefaultProfile string                      `koanf:"default_profile"` // profile used when session omits stt_profile
	Profiles       map[string]STTProfileConfig `koanf:"profiles"`        // named profiles (e.g. "local", "cloud")
	Google         GoogleSTTConfig
	AWS            AWSSTTConfig
	Azure          AzureSTTConfig
	WhisperLocal   WhisperLocalSTTConfig `koanf:"whisper_local"`
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
	APIKey         string `koanf:"api_key"` // bearer token for cloud-compatible endpoints
}

// LLMProfileConfig selects a provider and optionally overrides the model for a
// named tier. Credentials are inherited from the parent LLMConfig section.
type LLMProfileConfig struct {
	Provider          string            `koanf:"provider"` // openai | anthropic | azure | ollama | stub
	Model             string            `koanf:"model"`    // optional model override
	APIKey            string            `koanf:"api_key"`
	BaseURL           string            `koanf:"base_url"`
	Endpoint          string            `koanf:"endpoint"`
	Deployment        string            `koanf:"deployment"`
	RequestTimeout    time.Duration     `koanf:"request_timeout"`
	MaxTokens         int               `koanf:"max_tokens"`
	GenerationTimeout time.Duration     `koanf:"generation_timeout"`
	Retry             RetryPolicyConfig `koanf:"retry"`
	Reasoning         ReasoningConfig   `koanf:"reasoning"`
	Budget            LLMBudgetConfig   `koanf:"budget"`
	Think             *bool             `koanf:"think"`
}

type LLMConfig struct {
	Provider       string                      `koanf:"provider"`        // legacy default provider
	DefaultProfile string                      `koanf:"default_profile"` // profile used when session omits llm_profile
	Profiles       map[string]LLMProfileConfig `koanf:"profiles"`        // named profiles (e.g. "local", "cloud")
	Budget         LLMBudgetConfig             `koanf:"budget"`
	OpenAI         OpenAIConfig
	Anthropic      AnthropicConfig
	Azure          AzureLLMConfig
	Ollama         OllamaLLMConfig `koanf:"ollama"`
}

type OllamaLLMConfig struct {
	BaseURL string `koanf:"base_url"`
	Model   string `koanf:"model"`
	Think   *bool  `koanf:"think"`
}

type OpenAIConfig struct {
	APIKey         string          `koanf:"api_key"`
	Model          string          `koanf:"model"`
	BaseURL        string          `koanf:"base_url"` // optional override, e.g. https://openrouter.ai/api
	RequestTimeout time.Duration   `koanf:"request_timeout"`
	MaxTokens      int             `koanf:"max_tokens"`
	Reasoning      ReasoningConfig `koanf:"reasoning"`
}

type AnthropicConfig struct {
	APIKey         string        `koanf:"api_key"`
	Model          string        `koanf:"model"`
	RequestTimeout time.Duration `koanf:"request_timeout"`
	MaxTokens      int           `koanf:"max_tokens"`
}

type AzureLLMConfig struct {
	APIKey         string          `koanf:"api_key"`
	Endpoint       string          `koanf:"endpoint"`
	Deployment     string          `koanf:"deployment"`
	RequestTimeout time.Duration   `koanf:"request_timeout"`
	MaxTokens      int             `koanf:"max_tokens"`
	Reasoning      ReasoningConfig `koanf:"reasoning"`
}

type ReasoningConfig struct {
	Enabled *bool  `koanf:"enabled"`
	Effort  string `koanf:"effort"`
	Exclude bool   `koanf:"exclude"`
}

type RetryPolicyConfig struct {
	MaxAttempts    int           `koanf:"max_attempts"`
	InitialBackoff time.Duration `koanf:"initial_backoff"`
	MaxBackoff     time.Duration `koanf:"max_backoff"`
}

type LLMBudgetConfig struct {
	MaxSessionCostCredits float64 `koanf:"max_session_cost_credits"`
	MaxDailyCostCredits   float64 `koanf:"max_daily_cost_credits"`
	AllowLocalFallback    bool    `koanf:"allow_local_fallback"`
}

// WebhookConfig controls how generated minutes are delivered to the caller's system.
//
// Two delivery modes are supported:
//
//	push (legacy default):
//	  The full minutes JSON is POSTed to URL immediately after generation.
//	  Simple but unsuitable for sensitive data — the payload traverses the network
//	  unsolicited and Aftertalk retains the data indefinitely.
//
//	notify_pull (recommended for production / HIPAA / GDPR):
//	  Only a signed, single-use retrieval URL is POSTed to URL.
//	  The recipient must call GET /v1/minutes/pull/{token} to obtain the data.
//	  On a successful pull the minutes and transcriptions are deleted from the DB,
//	  making Aftertalk a pure processing pipeline, not a medical data archive.
//
// Minimal notify_pull config:
//
//	webhook:
//	  mode: notify_pull
//	  url:  https://your-app.example.com/webhook/aftertalk
//	  secret: "<at-least-32-byte-random-string>"
//	  pull_base_url: https://api.aftertalk.io
type WebhookConfig struct {
	DeleteOnPull *bool         `koanf:"delete_on_pull"`
	URL          string        `koanf:"url"`
	Mode         string        `koanf:"mode"`
	Secret       string        `koanf:"secret"`
	PullBaseURL  string        `koanf:"pull_base_url"`
	Timeout      time.Duration `koanf:"timeout"`
	TokenTTL     time.Duration `koanf:"token_ttl"`
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
	MinutesIncremental              bool          `koanf:"minutes_incremental"`
	MinutesBatchMaxSegments         int           `koanf:"minutes_batch_max_segments"`
	MinutesBatchMaxChars            int           `koanf:"minutes_batch_max_chars"`
	MinutesMaxSummaryPhases         int           `koanf:"minutes_max_summary_phases"`
	MinutesMaxCitations             int           `koanf:"minutes_max_citations"`
	MinutesVerifyFinal              bool          `koanf:"minutes_verify_final"`
}

type SessionConfig struct {
	// MaxDuration is the maximum lifetime of an active session. When a session
	// exceeds this duration it is automatically ended by the session reaper
	// background goroutine. Set to 0 to disable auto-timeout (default: 2h).
	MaxDuration time.Duration `koanf:"max_duration"`
	// InactivityTimeout is how long a session can be idle (no audio received)
	// before it is automatically ended. The timer resets on every audio chunk.
	// Restored across restarts via DB-backed last-activity lookup.
	// Set to 0 to disable inactivity-based auto-end (default: 10m).
	InactivityTimeout         time.Duration `koanf:"inactivity_timeout"`
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
	Username   string   `koanf:"username"`
	Credential string   `koanf:"credential"`
	URLs       []string `koanf:"urls"`
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
	Xirsys          XirsysICEConfig   `koanf:"xirsys"`
	Twilio          TwilioICEConfig   `koanf:"twilio"`
	Metered         MeteredICEConfig  `koanf:"metered"`
	ICEProviderName string            `koanf:"ice_provider"`
	ICEServers      []ICEServerConfig `koanf:"ice_servers"`
	TURN            TURNServerConfig  `koanf:"turn"`
	// ICEUDPPortMin/Max pins Pion's ephemeral UDP port range for ICE candidates.
	// Only these ports need to be open in the firewall (ufw allow MIN:MAX/udp).
	// Defaults: 49200–49209.
	ICEUDPPortMin uint16 `koanf:"ice_udp_port_min"`
	ICEUDPPortMax uint16 `koanf:"ice_udp_port_max"`
}

// TURNServerConfig configures the optional embedded TURN server (pion/turn).
// When Enabled is true, aftertalk runs a TURN server on ListenAddr so that
// clients behind symmetric NAT or strict firewalls can relay media through it.
type TURNServerConfig struct {
	ListenAddr string `koanf:"listen_addr"`
	PublicIP   string `koanf:"public_ip"`
	Realm      string `koanf:"realm"`
	AuthSecret string `koanf:"auth_secret"`
	AuthTTL    int    `koanf:"auth_ttl"`
	Enabled    bool   `koanf:"enabled"`
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
			Output: LoggingOutputConfig{
				Stdout: true,
				File: LoggingFileConfig{
					Enabled: false,
					Path:    "./logs/aftertalk.jsonl",
				},
			},
			Rotation: LoggingRotationConfig{
				MaxSizeMB:  100,
				MaxAgeDays: 30,
				MaxBackups: 20,
				Compress:   true,
			},
			Retention: LoggingRetentionConfig{
				DeleteAfterDays:       90,
				EmergencyCutoffSizeMB: 2048,
			},
			Redaction: LoggingRedactionConfig{
				Enabled: true,
				Fields: []string{
					"api_key",
					"token",
					"authorization",
					"secret",
					"password",
					"webhook_payload",
					"transcript_text",
					"minutes",
					"raw_provider_payload",
				},
			},
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
		STT: STTConfig{
			Provider: "google",
			Google: GoogleSTTConfig{
				CredentialsPath: "creds.json",
			},
			AWS: AWSSTTConfig{ //nolint:gosec // example values for documentation only
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
			URL:         "https://example.com/webhook",
			Timeout:     30 * time.Second,
			Mode:        "push", // change to "notify_pull" for production
			Secret:      "change-this-webhook-secret-min-32-bytes",
			TokenTTL:    1 * time.Hour,
			PullBaseURL: "https://api.aftertalk.io",
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
			MinutesIncremental:              true,
			MinutesBatchMaxSegments:         24,
			MinutesBatchMaxChars:            6000,
			MinutesMaxSummaryPhases:         8,
			MinutesMaxCitations:             12,
			MinutesVerifyFinal:              true,
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
			ICEUDPPortMin: 49200,
			ICEUDPPortMax: 49209,
		},
		Session: SessionConfig{
			MaxDuration:               2 * time.Hour,
			InactivityTimeout:         10 * time.Minute,
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
// a starter config file. Returns an error only if marshaling fails (never in
// practice for a static struct).
func DumpYAML() (string, error) {
	cfg := Default()
	out, err := yaml.Marshal(configYAMLValue(reflect.ValueOf(cfg)))
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}
	return string(out), nil
}

func configYAMLValue(v reflect.Value) interface{} {
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		return configYAMLValue(v.Elem())
	}
	if v.Type() == reflect.TypeOf(time.Duration(0)) {
		return v.Interface().(time.Duration).String()
	}

	switch v.Kind() {
	case reflect.Struct:
		out := make(map[string]interface{}, v.NumField())
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := configYAMLFieldName(field)
			if name == "-" {
				continue
			}
			out[name] = configYAMLValue(v.Field(i))
		}
		return out
	case reflect.Slice, reflect.Array:
		out := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			out[i] = configYAMLValue(v.Index(i))
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			return map[string]interface{}{}
		}
		out := make(map[string]interface{}, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			out[fmt.Sprint(iter.Key().Interface())] = configYAMLValue(iter.Value())
		}
		return out
	default:
		if v.CanInterface() {
			return v.Interface()
		}
		return nil
	}
}

func configYAMLFieldName(field reflect.StructField) string {
	if tag := field.Tag.Get("koanf"); tag != "" {
		return strings.Split(tag, ",")[0]
	}
	return strings.ToLower(field.Name)
}
