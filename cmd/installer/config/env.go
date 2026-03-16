package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ReadEnvFile reads a KEY=VALUE env file and returns a map.
func ReadEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path) //nolint:gosec // path is operator-provided
	if err != nil {
		return nil, fmt.Errorf("open env file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	m := make(map[string]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		m[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return m, sc.Err()
}

// FromEnvMap populates an InstallConfig from a KEY=VALUE map (produced by ReadEnvFile).
func FromEnvMap(m map[string]string) *InstallConfig {
	cfg := Default()

	get := func(k, def string) string {
		if v, ok := m[k]; ok && v != "" {
			return v
		}
		return def
	}
	getInt := func(k string, def int) int {
		if v, ok := m[k]; ok && v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				return n
			}
		}
		return def
	}
	getBool := func(k string) bool { return m[k] == "1" || m[k] == "true" }

	cfg.ServiceRoot = get("SERVICE_ROOT", cfg.ServiceRoot)
	cfg.ServiceUser = get("SERVICE_USER", cfg.ServiceUser)
	cfg.HTTPPort = getInt("HTTP_PORT", cfg.HTTPPort)
	cfg.APIKey = get("API_KEY", "")
	cfg.JWTSecret = get("JWT_SECRET", "")
	cfg.JWTIssuer = get("JWT_ISSUER", cfg.JWTIssuer)
	cfg.JWTExpiry = get("JWT_EXPIRATION", cfg.JWTExpiry)
	cfg.STTProvider = get("STT_PROVIDER", cfg.STTProvider)
	cfg.LLMProvider = get("LLM_PROVIDER", cfg.LLMProvider)
	cfg.WebhookURL = get("WEBHOOK_URL", "")
	cfg.WebhookMode = get("WEBHOOK_MODE", cfg.WebhookMode)
	cfg.WebhookSecret = get("WEBHOOK_SECRET", "")
	cfg.WebhookPullBase = get("WEBHOOK_PULL_BASE_URL", "")
	cfg.WebhookTokenTTL = get("WEBHOOK_TOKEN_TTL", cfg.WebhookTokenTTL)
	cfg.WebhookMaxRetries = getInt("WEBHOOK_MAX_RETRIES", cfg.WebhookMaxRetries)
	cfg.SessionMaxDuration = get("SESSION_MAX_DURATION", cfg.SessionMaxDuration)
	cfg.SessionInactivityTimeout = get("SESSION_INACTIVITY_TIMEOUT", cfg.SessionInactivityTimeout)
	cfg.TLSCertFile = get("TLS_CERT_FILE", "")
	cfg.TLSKeyFile = get("TLS_KEY_FILE", "")
	cfg.ApacheVhostConf = get("APACHE_VHOST_CONF", "")
	cfg.SkipFirewall = getBool("SKIP_FIREWALL")

	// STT provider-specific keys
	cfg.STTConfig = make(map[string]string)
	for _, k := range []string{
		"GOOGLE_APPLICATION_CREDENTIALS",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION",
		"AZURE_SPEECH_KEY", "AZURE_SPEECH_REGION",
	} {
		if v := m[k]; v != "" {
			cfg.STTConfig[k] = v
		}
	}

	// LLM provider-specific keys
	cfg.LLMConfig = make(map[string]string)
	for _, k := range []string{
		"LLM_API_KEY", "LLM_MODEL", "LLM_MAX_TOKENS",
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY",
		"AZURE_OPENAI_API_KEY", "AZURE_OPENAI_ENDPOINT",
	} {
		if v := m[k]; v != "" {
			cfg.LLMConfig[k] = v
		}
	}

	return cfg
}

// WriteEnvFile serialises an InstallConfig into a KEY=VALUE env file.
// Used when the interactive mode saves its output for later use or hand-off.
func WriteEnvFile(path string, cfg *InstallConfig) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec
	if err != nil {
		return fmt.Errorf("create env file: %w", err)
	}
	defer f.Close() //nolint:errcheck

	w := bufio.NewWriter(f)
	writeln := func(k, v string) { fmt.Fprintf(w, "%s=%s\n", k, v) }
	writeInt := func(k string, v int) { fmt.Fprintf(w, "%s=%d\n", k, v) }

	writeln("SERVICE_ROOT", cfg.ServiceRoot)
	writeln("SERVICE_USER", cfg.ServiceUser)
	writeInt("HTTP_PORT", cfg.HTTPPort)
	writeln("API_KEY", cfg.APIKey)
	writeln("JWT_SECRET", cfg.JWTSecret)
	writeln("JWT_ISSUER", cfg.JWTIssuer)
	writeln("JWT_EXPIRATION", cfg.JWTExpiry)
	writeln("STT_PROVIDER", cfg.STTProvider)
	writeln("LLM_PROVIDER", cfg.LLMProvider)
	writeln("WEBHOOK_URL", cfg.WebhookURL)
	writeln("WEBHOOK_MODE", cfg.WebhookMode)
	writeln("WEBHOOK_SECRET", cfg.WebhookSecret)
	writeln("WEBHOOK_PULL_BASE_URL", cfg.WebhookPullBase)
	writeln("WEBHOOK_TOKEN_TTL", cfg.WebhookTokenTTL)
	writeInt("WEBHOOK_MAX_RETRIES", cfg.WebhookMaxRetries)
	writeln("SESSION_MAX_DURATION", cfg.SessionMaxDuration)
	writeln("SESSION_INACTIVITY_TIMEOUT", cfg.SessionInactivityTimeout)
	writeln("TLS_CERT_FILE", cfg.TLSCertFile)
	writeln("TLS_KEY_FILE", cfg.TLSKeyFile)
	writeln("APACHE_VHOST_CONF", cfg.ApacheVhostConf)
	if cfg.SkipFirewall {
		writeln("SKIP_FIREWALL", "1")
	}
	for k, v := range cfg.STTConfig {
		writeln(k, v)
	}
	for k, v := range cfg.LLMConfig {
		writeln(k, v)
	}

	return w.Flush()
}
