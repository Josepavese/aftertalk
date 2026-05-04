package llm

import (
	"fmt"

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
		},
		Anthropic: AnthropicConfig{
			APIKey:         cfg.Anthropic.APIKey,
			Model:          cfg.Anthropic.Model,
			RequestTimeout: cfg.Anthropic.RequestTimeout,
		},
		Azure: AzureLLMConfig{
			APIKey:         cfg.Azure.APIKey,
			Endpoint:       cfg.Azure.Endpoint,
			Deployment:     cfg.Azure.Deployment,
			RequestTimeout: cfg.Azure.RequestTimeout,
		},
		Ollama: OllamaConfig{BaseURL: cfg.Ollama.BaseURL, Model: cfg.Ollama.Model},
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
		p, err := NewProvider(&profileBase)
		if err != nil {
			return nil, fmt.Errorf("llm profile %q: %w", name, err)
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

// NewLLMRegistryFromProvider wraps a single LLMProvider in a registry under the
// "default" profile. Intended for use in tests that need a *LLMRegistry but
// already have a concrete provider.
func NewLLMRegistryFromProvider(p LLMProvider) *LLMRegistry {
	return &LLMRegistry{
		providers:      map[string]LLMProvider{"default": p},
		defaultProfile: "default",
	}
}
