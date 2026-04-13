package stt

import (
	"context"
	"fmt"
)

// StubProvider returns deterministic placeholder transcriptions for offline
// development and installer smoke tests.
type StubProvider struct{}

func NewStubProvider() *StubProvider {
	return &StubProvider{}
}

func (p *StubProvider) Name() string {
	return "stub"
}

func (p *StubProvider) IsAvailable() bool {
	return true
}

func (p *StubProvider) Transcribe(_ context.Context, audioData *AudioData) (*TranscriptionResult, error) {
	result := NewTranscriptionResult(p.Name())
	text := fmt.Sprintf("[stub] %s provided %dms of audio", fallbackRole(audioData.Role), audioData.Duration)
	result.AddSegment(&TranscriptionSegment{
		SessionID:  audioData.SessionID,
		Role:       fallbackRole(audioData.Role),
		Text:       text,
		StartMs:    0,
		EndMs:      maxInt(audioData.Duration, 1),
		Confidence: 1,
	})
	result.Duration = audioData.Duration
	return result, nil
}

func fallbackRole(role string) string {
	if role == "" {
		return "speaker"
	}
	return role
}

func maxInt(value, floor int) int {
	if value < floor {
		return floor
	}
	return value
}
