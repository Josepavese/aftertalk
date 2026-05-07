package llm

import (
	"context"
	"fmt"
	"sort"

	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
)

// LLMRegistry holds one LLMProvider per named profile and resolves profile
// names to providers at call time. It is built once at startup and is
// safe for concurrent use (all providers are stateless HTTP clients).
type LLMRegistry struct {
	providers      map[string]LLMProvider
	defaultProfile string
}

type ProfileStatus struct {
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// NewLLMRegistry builds a registry from the top-level LLMConfig.
// If no profiles are defined it wraps the legacy single-provider as "default".
func NewLLMRegistry(cfg *config.LLMConfig) (*LLMRegistry, error) {
	r := &LLMRegistry{
		providers:      make(map[string]LLMProvider),
		defaultProfile: cfg.DefaultProfile,
	}

	// Shared credentials / endpoints inherited by all profiles.
	base := &LLMConfig{
		OpenAI: OpenAIConfig{
			APIKey:         cfg.OpenAI.APIKey,
			Model:          cfg.OpenAI.Model,
			BaseURL:        cfg.OpenAI.BaseURL,
			RequestTimeout: cfg.OpenAI.RequestTimeout,
			MaxTokens:      cfg.OpenAI.MaxTokens,
			Reasoning: ReasoningConfig{
				Enabled: cfg.OpenAI.Reasoning.Enabled,
				Effort:  cfg.OpenAI.Reasoning.Effort,
				Exclude: cfg.OpenAI.Reasoning.Exclude,
			},
		},
		Anthropic: AnthropicConfig{
			APIKey:         cfg.Anthropic.APIKey,
			Model:          cfg.Anthropic.Model,
			RequestTimeout: cfg.Anthropic.RequestTimeout,
			MaxTokens:      cfg.Anthropic.MaxTokens,
		},
		Azure: AzureLLMConfig{
			APIKey:         cfg.Azure.APIKey,
			Endpoint:       cfg.Azure.Endpoint,
			Deployment:     cfg.Azure.Deployment,
			RequestTimeout: cfg.Azure.RequestTimeout,
			MaxTokens:      cfg.Azure.MaxTokens,
			Reasoning: ReasoningConfig{
				Enabled: cfg.Azure.Reasoning.Enabled,
				Effort:  cfg.Azure.Reasoning.Effort,
				Exclude: cfg.Azure.Reasoning.Exclude,
			},
		},
		Ollama: OllamaConfig{BaseURL: cfg.Ollama.BaseURL, Model: cfg.Ollama.Model, Think: cfg.Ollama.Think},
	}

	if len(cfg.Profiles) == 0 {
		// Legacy mode: single provider under the "default" key.
		base.Provider = cfg.Provider
		p, err := NewProvider(base)
		if err != nil {
			return nil, err
		}
		r.providers["default"] = p
		if r.defaultProfile == "" {
			r.defaultProfile = "default"
		}
		logging.Infof("LLM registry: single provider=%s (no profiles configured)", p.Name())
		return r, nil
	}

	for name, pcfg := range cfg.Profiles {
		profileBase := *base
		profileBase.Provider = pcfg.Provider
		// Apply optional model override for the selected provider.
		if pcfg.Model != "" {
			switch pcfg.Provider {
			case "openai":
				profileBase.OpenAI.Model = pcfg.Model
			case "anthropic":
				profileBase.Anthropic.Model = pcfg.Model
			case "ollama":
				profileBase.Ollama.Model = pcfg.Model
			}
		}
		if pcfg.RequestTimeout > 0 {
			switch pcfg.Provider {
			case "openai":
				profileBase.OpenAI.RequestTimeout = pcfg.RequestTimeout
			case "anthropic":
				profileBase.Anthropic.RequestTimeout = pcfg.RequestTimeout
			case "azure":
				profileBase.Azure.RequestTimeout = pcfg.RequestTimeout
			}
		}
		if pcfg.MaxTokens > 0 {
			switch pcfg.Provider {
			case "openai":
				profileBase.OpenAI.MaxTokens = pcfg.MaxTokens
			case "anthropic":
				profileBase.Anthropic.MaxTokens = pcfg.MaxTokens
			case "azure":
				profileBase.Azure.MaxTokens = pcfg.MaxTokens
			}
		}
		switch pcfg.Provider {
		case "openai":
			if pcfg.APIKey != "" {
				profileBase.OpenAI.APIKey = pcfg.APIKey
			}
			if pcfg.BaseURL != "" {
				profileBase.OpenAI.BaseURL = pcfg.BaseURL
			}
			profileBase.OpenAI.Reasoning = mergeReasoning(profileBase.OpenAI.Reasoning, convertReasoning(pcfg.Reasoning))
		case "anthropic":
			if pcfg.APIKey != "" {
				profileBase.Anthropic.APIKey = pcfg.APIKey
			}
		case "azure":
			if pcfg.APIKey != "" {
				profileBase.Azure.APIKey = pcfg.APIKey
			}
			if pcfg.Endpoint != "" {
				profileBase.Azure.Endpoint = pcfg.Endpoint
			}
			if pcfg.Deployment != "" {
				profileBase.Azure.Deployment = pcfg.Deployment
			}
			profileBase.Azure.Reasoning = mergeReasoning(profileBase.Azure.Reasoning, convertReasoning(pcfg.Reasoning))
		case "ollama":
			if pcfg.BaseURL != "" {
				profileBase.Ollama.BaseURL = pcfg.BaseURL
			}
			if pcfg.Think != nil {
				profileBase.Ollama.Think = pcfg.Think
			}
		}
		p, err := NewProvider(&profileBase)
		if err != nil {
			return nil, fmt.Errorf("llm profile %q: %w", name, err)
		}
		if runtime := runtimeConfigFromProfile(pcfg); !runtime.IsZero() {
			p = &profileProvider{inner: p, runtime: runtime}
		}
		r.providers[name] = p
		logging.Infof("LLM registry: profile=%s provider=%s", name, p.Name())
	}

	if r.defaultProfile == "" {
		return nil, fmt.Errorf("llm.default_profile must be set when profiles are defined")
	}
	if _, ok := r.providers[r.defaultProfile]; !ok {
		return nil, fmt.Errorf("llm.default_profile=%q not found in defined profiles", r.defaultProfile)
	}

	return r, nil
}

// Get returns the provider for the given profile name.
// If profileName is empty the default profile is used.
// If the profile is unknown the default profile is returned with a warning.
func (r *LLMRegistry) Get(profileName string) LLMProvider {
	if profileName == "" {
		profileName = r.defaultProfile
	}
	if p, ok := r.providers[profileName]; ok {
		return p
	}
	logging.Warnf("LLM registry: unknown profile=%q, falling back to default=%q", profileName, r.defaultProfile)
	return r.providers[r.defaultProfile]
}

// DefaultProfile returns the configured default profile name.
func (r *LLMRegistry) DefaultProfile() string {
	return r.defaultProfile
}

// ProfileNames returns all registered profile names.
func (r *LLMRegistry) ProfileNames() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

func (r *LLMRegistry) Readiness() []ProfileStatus {
	names := r.ProfileNames()
	sort.Strings(names)
	statuses := make([]ProfileStatus, 0, len(names))
	for _, name := range names {
		p := r.providers[name]
		available := p != nil && p.IsAvailable()
		st := ProfileStatus{Name: name, Available: available}
		if p != nil {
			st.Provider = p.Name()
		}
		if !available {
			st.Reason = "provider configuration incomplete or endpoint unavailable"
		}
		statuses = append(statuses, st)
	}
	return statuses
}

// NewLLMRegistryFromProvider wraps a single LLMProvider in a registry under the
// "default" profile. Intended for use in tests that need a *LLMRegistry but
// already have a concrete provider.
func NewLLMRegistryFromProvider(p LLMProvider) *LLMRegistry {
	return &LLMRegistry{
		providers:      map[string]LLMProvider{"default": p},
		defaultProfile: "default",
	}
}

func mergeReasoning(base, override ReasoningConfig) ReasoningConfig {
	if override.Enabled != nil {
		base.Enabled = override.Enabled
	}
	if override.Effort != "" {
		base.Effort = override.Effort
	}
	if override.Exclude {
		base.Exclude = true
	}
	return base
}

func convertReasoning(in config.ReasoningConfig) ReasoningConfig {
	return ReasoningConfig{
		Enabled: in.Enabled,
		Effort:  in.Effort,
		Exclude: in.Exclude,
	}
}

type profileProvider struct {
	inner   LLMProvider
	runtime RuntimeConfig
}

func (p *profileProvider) Generate(ctx context.Context, prompt string) (string, error) {
	return p.inner.Generate(ctx, prompt)
}

func (p *profileProvider) Name() string {
	return p.inner.Name()
}

func (p *profileProvider) IsAvailable() bool {
	return p.inner.IsAvailable()
}

func (p *profileProvider) RuntimeConfig() RuntimeConfig {
	return p.runtime
}

func runtimeConfigFromProfile(p config.LLMProfileConfig) RuntimeConfig {
	return RuntimeConfig{
		GenerationTimeout: p.GenerationTimeout,
		Retry: RetryConfig{
			MaxAttempts:    p.Retry.MaxAttempts,
			InitialBackoff: p.Retry.InitialBackoff,
			MaxBackoff:     p.Retry.MaxBackoff,
		},
	}
}

func (c RuntimeConfig) IsZero() bool {
	return c.GenerationTimeout == 0 &&
		c.Retry.MaxAttempts == 0 &&
		c.Retry.InitialBackoff == 0 &&
		c.Retry.MaxBackoff == 0
}
