package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Interactive runs a terminal interview and returns a fully populated InstallConfig.
// All questions show the current default in brackets; pressing Enter keeps the default.
func Interactive() (*InstallConfig, error) {
	cfg := Default()
	r := bufio.NewReader(os.Stdin)

	ask := func(label, def string) string {
		if def != "" {
			fmt.Printf("  %s [%s]: ", label, def)
		} else {
			fmt.Printf("  %s: ", label)
		}
		line, _ := r.ReadString('\n')
		v := strings.TrimSpace(line)
		if v == "" {
			return def
		}
		return v
	}
	askSecret := func(label string) string {
		fmt.Printf("  %s (hidden, required): ", label)
		line, _ := r.ReadString('\n')
		return strings.TrimSpace(line)
	}
	askYN := func(label string, def bool) bool {
		d := "n"
		if def {
			d = "y"
		}
		v := ask(label+" (y/n)", d)
		return strings.ToLower(v) == "y"
	}
	section := func(title string) {
		fmt.Printf("\n── %s ─────────────────────────────────────────\n", title)
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════╗")
	fmt.Println("║   Aftertalk Installer — Interactive Setup   ║")
	fmt.Println("╚══════════════════════════════════════════════╝")
	fmt.Println("  Press Enter to accept the default shown in [brackets].")

	section("Infrastructure")
	cfg.ServiceRoot = ask("Install directory", cfg.ServiceRoot)
	cfg.ServiceUser = ask("Service OS user", cfg.ServiceUser)
	portStr := ask("HTTP port", fmt.Sprintf("%d", cfg.HTTPPort))
	fmt.Sscanf(portStr, "%d", &cfg.HTTPPort) //nolint:errcheck

	section("Security")
	cfg.APIKey = askSecret("API key")
	cfg.JWTSecret = askSecret("JWT secret (min 32 chars)")
	cfg.JWTIssuer = ask("JWT issuer", cfg.JWTIssuer)
	cfg.JWTExpiry = ask("JWT expiry", cfg.JWTExpiry)

	section("Speech-to-Text (STT)")
	fmt.Println("  Providers: google | aws | azure | whisper-local")
	cfg.STTProvider = ask("STT provider", cfg.STTProvider)
	cfg.STTConfig = make(map[string]string)
	switch cfg.STTProvider {
	case "google":
		cfg.STTConfig["GOOGLE_APPLICATION_CREDENTIALS"] = ask(
			"Path to GCP service account JSON", "/opt/aftertalk/gcp-stt-key.json")
	case "aws":
		cfg.STTConfig["AWS_ACCESS_KEY_ID"] = ask("AWS Access Key ID", "")
		cfg.STTConfig["AWS_SECRET_ACCESS_KEY"] = askSecret("AWS Secret Access Key")
		cfg.STTConfig["AWS_REGION"] = ask("AWS Region", "us-east-1")
	case "azure":
		cfg.STTConfig["AZURE_SPEECH_KEY"] = askSecret("Azure Speech Key")
		cfg.STTConfig["AZURE_SPEECH_REGION"] = ask("Azure Region", "eastus")
	case "whisper-local":
		cfg.STTConfig["WHISPER_LOCAL_URL"] = ask("Whisper server URL", "http://localhost:9000")
	}

	section("LLM (Minutes Generation)")
	fmt.Println("  Providers: openai | anthropic | azure | ollama")
	cfg.LLMProvider = ask("LLM provider", cfg.LLMProvider)
	cfg.LLMConfig = make(map[string]string)
	switch cfg.LLMProvider {
	case "openai":
		cfg.LLMConfig["LLM_API_KEY"] = askSecret("OpenAI API key")
		cfg.LLMConfig["LLM_MODEL"] = ask("Model", "gpt-4o")
	case "anthropic":
		cfg.LLMConfig["LLM_API_KEY"] = askSecret("Anthropic API key")
		cfg.LLMConfig["LLM_MODEL"] = ask("Model", "claude-sonnet-4-6")
	case "azure":
		cfg.LLMConfig["LLM_API_KEY"] = askSecret("Azure OpenAI API key")
		cfg.LLMConfig["LLM_MODEL"] = ask("Deployment name", "gpt-4")
		cfg.LLMConfig["AZURE_OPENAI_ENDPOINT"] = ask("Azure OpenAI endpoint", "")
	case "ollama":
		cfg.LLMConfig["OLLAMA_BASE_URL"] = ask("Ollama base URL", "http://localhost:11434")
		cfg.LLMConfig["LLM_MODEL"] = ask("Model", "llama3")
	}

	section("Webhook")
	cfg.WebhookURL = ask("Webhook URL (leave empty to disable)", "")
	if cfg.WebhookURL != "" {
		fmt.Println("  Modes: push (full payload) | notify_pull (HIPAA/GDPR — URL only)")
		cfg.WebhookMode = ask("Webhook mode", cfg.WebhookMode)
		if cfg.WebhookMode == "notify_pull" {
			cfg.WebhookSecret = askSecret("Webhook HMAC secret (min 32 chars)")
			cfg.WebhookPullBase = ask("Public Aftertalk base URL (for pull links)", "")
			cfg.WebhookTokenTTL = ask("Pull token TTL", cfg.WebhookTokenTTL)
		}
	}

	section("Session Tuning")
	cfg.SessionMaxDuration = ask("Max session duration", cfg.SessionMaxDuration)
	cfg.SessionInactivityTimeout = ask("Inactivity timeout", cfg.SessionInactivityTimeout)

	section("TLS")
	fmt.Println("  Leave empty to serve plain HTTP (use when behind Apache/nginx).")
	cfg.TLSCertFile = ask("TLS cert file path", "")
	if cfg.TLSCertFile != "" {
		cfg.TLSKeyFile = ask("TLS key file path", "")
	}

	section("Apache Reverse Proxy")
	fmt.Println("  Injects ProxyPass rules for /aftertalk into your existing SSL vhost.")
	if askYN("Configure Apache proxy?", cfg.ApacheVhostConf != "") {
		cfg.ApacheVhostConf = ask("SSL vhost config file", cfg.ApacheVhostConf)
	}

	section("Firewall")
	cfg.SkipFirewall = !askYN("Configure firewall (ufw)?", !cfg.SkipFirewall)

	fmt.Println()
	fmt.Println("─────────────────────────────────────────────")
	fmt.Println("  Configuration complete. Ready to install.")
	fmt.Println()

	return cfg, nil
}
