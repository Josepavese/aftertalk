package stt

import (
	"context"
	"fmt"

	"github.com/flowup/aftertalk/internal/logging"
)

// StubSTTProvider is the no-op provider used when no real STT is configured.
// It returns a labelled transcription segment without calling any external API.
type StubSTTProvider struct{}

func NewStubSTTProvider() *StubSTTProvider {
	return &StubSTTProvider{}
}

func (p *StubSTTProvider) Transcribe(_ context.Context, audioData *AudioData) (*TranscriptionResult, error) {
	logging.Warnf("STT stub: session=%s participant=%s role=%s frames=%d bytes=%d duration=%dms offset=%dms",
		audioData.SessionID, audioData.ParticipantID, audioData.Role,
		len(audioData.Frames), len(audioData.Data), audioData.Duration, audioData.OffsetMs)

	result := NewTranscriptionResult(p.Name())
	result.Duration = audioData.Duration
	result.AddSegment(&TranscriptionSegment{
		SessionID:  audioData.SessionID,
		Role:       audioData.Role,
		StartMs:    audioData.OffsetMs,
		EndMs:      audioData.OffsetMs + audioData.Duration,
		Text:       fmt.Sprintf("[stub: %dms di audio da %s]", audioData.Duration, audioData.Role),
		Confidence: 1.0,
	})
	return result, nil
}

func (p *StubSTTProvider) Name() string      { return "stub" }
func (p *StubSTTProvider) IsAvailable() bool { return true }

// NewProvider selects and returns the STT provider based on cfg.
// Falls back to StubSTTProvider when provider name is empty or unrecognised.
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
			return nil, fmt.Errorf("whisper-local: STT_WHISPER_URL is required")
		}
		return p, nil
	case "", "stub":
		return NewStubSTTProvider(), nil
	default:
		return nil, fmt.Errorf("unsupported STT provider: %s", cfg.Provider)
	}
}
