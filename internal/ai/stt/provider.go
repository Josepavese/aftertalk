package stt

import "context"

type AudioData struct {
	SessionID     string
	ParticipantID string
	Role          string
	Data          []byte
	SampleRate    int
	Duration      int
}

type STTProvider interface {
	Transcribe(ctx context.Context, audioData *AudioData) (*TranscriptionResult, error)
	Name() string
	IsAvailable() bool
}

type TranscriptionResult struct {
	Segments []*TranscriptionSegment
	Provider string
	Duration int
}

type TranscriptionSegment struct {
	ID         string
	SessionID  string
	Role       string
	StartMs    int
	EndMs      int
	Text       string
	Confidence float64
}

func NewTranscriptionResult(provider string) *TranscriptionResult {
	return &TranscriptionResult{
		Segments: make([]*TranscriptionSegment, 0),
		Provider: provider,
	}
}

func (r *TranscriptionResult) AddSegment(segment *TranscriptionSegment) {
	r.Segments = append(r.Segments, segment)
}

type STTConfig struct {
	Provider string
	Google   GoogleConfig
	AWS      AWSConfig
	Azure    AzureConfig
}

type GoogleConfig struct {
	CredentialsPath string
}

type AWSConfig struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
}

type AzureConfig struct {
	Key    string
	Region string
}
