package stt

import (
	"fmt"

	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
)

// STTRegistry holds one STTProvider per named profile and resolves profile
// names to providers at call time. It is built once at startup and is
// safe for concurrent use.
type STTRegistry struct {
	providers      map[string]STTProvider
	defaultProfile string
}

// NewSTTRegistry builds a registry from the top-level STTConfig.
// If no profiles are defined it wraps the legacy single-provider as "default".
func NewSTTRegistry(cfg *config.STTConfig) (*STTRegistry, error) {
	r := &STTRegistry{
		providers:      make(map[string]STTProvider),
		defaultProfile: cfg.DefaultProfile,
	}

	// Shared base config (credentials / URLs) inherited by all profiles.
	base := &STTConfig{
		Google:       GoogleConfig{CredentialsPath: cfg.Google.CredentialsPath},
		AWS:          AWSConfig{AccessKeyID: cfg.AWS.AccessKeyID, SecretAccessKey: cfg.AWS.SecretAccessKey, Region: cfg.AWS.Region},
		Azure:        AzureConfig{Key: cfg.Azure.Key, Region: cfg.Azure.Region},
		WhisperLocal: WhisperLocalConfig{URL: cfg.WhisperLocal.URL, Model: cfg.WhisperLocal.Model, Language: cfg.WhisperLocal.Language, ResponseFormat: cfg.WhisperLocal.ResponseFormat, Endpoint: cfg.WhisperLocal.Endpoint},
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
		logging.Infof("STT registry: single provider=%s (no profiles configured)", p.Name())
		return r, nil
	}

	for name, pcfg := range cfg.Profiles {
		profileBase := *base
		profileBase.Provider = pcfg.Provider
		// Apply optional model override for whisper-local.
		if pcfg.Model != "" && pcfg.Provider == "whisper-local" {
			profileBase.WhisperLocal.Model = pcfg.Model
		}
		p, err := NewProvider(&profileBase)
		if err != nil {
			return nil, fmt.Errorf("stt profile %q: %w", name, err)
		}
		r.providers[name] = p
		logging.Infof("STT registry: profile=%s provider=%s", name, p.Name())
	}

	if r.defaultProfile == "" {
		return nil, fmt.Errorf("stt.default_profile must be set when profiles are defined")
	}
	if _, ok := r.providers[r.defaultProfile]; !ok {
		return nil, fmt.Errorf("stt.default_profile=%q not found in defined profiles", r.defaultProfile)
	}

	return r, nil
}

// Get returns the provider for the given profile name.
// If profileName is empty the default profile is used.
// If the profile is unknown the default profile is returned with a warning.
func (r *STTRegistry) Get(profileName string) STTProvider {
	if profileName == "" {
		profileName = r.defaultProfile
	}
	if p, ok := r.providers[profileName]; ok {
		return p
	}
	logging.Warnf("STT registry: unknown profile=%q, falling back to default=%q", profileName, r.defaultProfile)
	return r.providers[r.defaultProfile]
}

// DefaultProfile returns the configured default profile name.
func (r *STTRegistry) DefaultProfile() string {
	return r.defaultProfile
}

// ProfileNames returns all registered profile names.
func (r *STTRegistry) ProfileNames() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// NewSTTRegistryFromProvider wraps a single STTProvider in a registry under the
// "default" profile. Intended for use in tests that need a *STTRegistry but
// already have a concrete provider.
func NewSTTRegistryFromProvider(p STTProvider) *STTRegistry {
	return &STTRegistry{
		providers:      map[string]STTProvider{"default": p},
		defaultProfile: "default",
	}
}
