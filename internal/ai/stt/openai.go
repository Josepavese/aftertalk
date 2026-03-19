package stt

import (
	"context"
	"net/http"
	"time"
)

// OpenAISTTProvider calls any OpenAI-compatible /v1/audio/transcriptions endpoint
// (Groq, OpenRouter, OpenAI, etc.). Functionally identical to WhisperLocalProvider
// but named "openai" so it appears correctly in transcription records and logs.
type OpenAISTTProvider struct {
	inner *WhisperLocalProvider
}

// NewOpenAISTTProvider builds a provider for cloud OpenAI-compatible STT endpoints.
func NewOpenAISTTProvider(cfg WhisperLocalConfig) *OpenAISTTProvider {
	return &OpenAISTTProvider{
		inner: &WhisperLocalProvider{
			cfg: cfg,
			client: &http.Client{
				Timeout: 5 * time.Minute,
			},
		},
	}
}

func (p *OpenAISTTProvider) Name() string { return "openai" }

func (p *OpenAISTTProvider) IsAvailable() bool {
	return p.inner.cfg.URL != "" && p.inner.cfg.APIKey != ""
}

func (p *OpenAISTTProvider) Transcribe(ctx context.Context, audioData *AudioData) (*TranscriptionResult, error) {
	result, err := p.inner.Transcribe(ctx, audioData)
	if err != nil {
		return nil, err
	}
	// Override provider name: inner always returns "whisper-local".
	result.Provider = p.Name()
	return result, nil
}
