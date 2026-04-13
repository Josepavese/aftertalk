package stt

import (
	"errors"
	"fmt"
)

var (
	errWhisperURLRequired     = errors.New("whisper-local: STT_WHISPER_URL is required")
	errOpenAICredRequired     = errors.New("openai STT: url and api_key are required")
	errSTTProviderRequired    = errors.New("stt.provider is required — supported: google, aws, azure, whisper-local, openai, stub")
	errUnsupportedSTTProvider = errors.New("unsupported STT provider")
)

// NewProvider selects and returns the STT provider based on cfg.
func NewProvider(cfg *STTConfig) (STTProvider, error) {
	switch cfg.Provider {
	case "google":
		return NewGoogleSTTProvider(cfg.Google.CredentialsPath), nil
	case "aws":
		return NewAWSSTTProvider(cfg.AWS.AccessKeyID, cfg.AWS.SecretAccessKey, cfg.AWS.Region), nil
	case "azure":
		return NewAzureSTTProvider(cfg.Azure.Key, cfg.Azure.Region), nil
	case "whisper-local":
		p := NewWhisperLocalProvider(cfg.WhisperLocal)
		if !p.IsAvailable() {
			return nil, errWhisperURLRequired
		}
		return p, nil
	case "openai":
		p := NewOpenAISTTProvider(cfg.WhisperLocal)
		if !p.IsAvailable() {
			return nil, errOpenAICredRequired
		}
		return p, nil
	case "stub":
		return NewStubProvider(), nil
	case "":
		return nil, errSTTProviderRequired
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedSTTProvider, cfg.Provider)
	}
}
