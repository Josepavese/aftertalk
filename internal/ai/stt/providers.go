package stt

import (
	"context"
	"fmt"

	"github.com/flowup/aftertalk/internal/logging"
)

type GoogleSTTProvider struct {
	credentialsPath string
}

func NewGoogleSTTProvider(credentialsPath string) *GoogleSTTProvider {
	return &GoogleSTTProvider{
		credentialsPath: credentialsPath,
	}
}

func (p *GoogleSTTProvider) Transcribe(ctx context.Context, audioData *AudioData) (*TranscriptionResult, error) {
	logging.Infof("Google STT: Transcribing audio from session %s (%d bytes)", audioData.SessionID, len(audioData.Data))

	result := NewTranscriptionResult(p.Name())

	segment := &TranscriptionSegment{
		SessionID:  audioData.SessionID,
		Role:       audioData.Role,
		StartMs:    0,
		EndMs:      audioData.Duration,
		Text:       "[Transcription placeholder - Google STT integration required]",
		Confidence: 0.95,
	}

	result.AddSegment(segment)
	result.Duration = audioData.Duration

	return result, nil
}

func (p *GoogleSTTProvider) Name() string {
	return "google"
}

func (p *GoogleSTTProvider) IsAvailable() bool {
	return p.credentialsPath != ""
}

type AWSSTTProvider struct {
	accessKeyID     string
	secretAccessKey string
	region          string
}

func NewAWSSTTProvider(accessKeyID, secretAccessKey, region string) *AWSSTTProvider {
	return &AWSSTTProvider{
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		region:          region,
	}
}

func (p *AWSSTTProvider) Transcribe(ctx context.Context, audioData *AudioData) (*TranscriptionResult, error) {
	if !p.IsAvailable() {
		return nil, fmt.Errorf("AWS STT provider is not available: missing credentials")
	}
	logging.Infof("AWS Transcribe: Transcribing audio from session %s (%d bytes)", audioData.SessionID, len(audioData.Data))

	result := NewTranscriptionResult(p.Name())

	segment := &TranscriptionSegment{
		SessionID:  audioData.SessionID,
		Role:       audioData.Role,
		StartMs:    0,
		EndMs:      audioData.Duration,
		Text:       "[Transcription placeholder - AWS Transcribe integration required]",
		Confidence: 0.90,
	}

	result.AddSegment(segment)
	result.Duration = audioData.Duration

	return result, nil
}

func (p *AWSSTTProvider) Name() string {
	return "aws"
}

func (p *AWSSTTProvider) IsAvailable() bool {
	return p.accessKeyID != "" && p.secretAccessKey != "" && p.region != ""
}

type AzureSTTProvider struct {
	key    string
	region string
}

func NewAzureSTTProvider(key, region string) *AzureSTTProvider {
	return &AzureSTTProvider{
		key:    key,
		region: region,
	}
}

func (p *AzureSTTProvider) Transcribe(ctx context.Context, audioData *AudioData) (*TranscriptionResult, error) {
	logging.Infof("Azure Speech: Transcribing audio from session %s (%d bytes)", audioData.SessionID, len(audioData.Data))

	result := NewTranscriptionResult(p.Name())

	segment := &TranscriptionSegment{
		SessionID:  audioData.SessionID,
		Role:       audioData.Role,
		StartMs:    0,
		EndMs:      audioData.Duration,
		Text:       "[Transcription placeholder - Azure Speech integration required]",
		Confidence: 0.92,
	}

	result.AddSegment(segment)
	result.Duration = audioData.Duration

	return result, nil
}

func (p *AzureSTTProvider) Name() string {
	return "azure"
}

func (p *AzureSTTProvider) IsAvailable() bool {
	return p.key != "" && p.region != ""
}

// StubSTTProvider is the no-op provider used when no real STT is configured.
// It returns an empty transcription result without calling any external API.
// Replace this provider with a real implementation when an STT backend is available.
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
