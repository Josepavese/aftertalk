package api

import (
	"context"

	"github.com/Josepavese/aftertalk/internal/ai/stt"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/core/minutes"
	"github.com/Josepavese/aftertalk/internal/core/session"
	"github.com/Josepavese/aftertalk/internal/core/transcription"
)

// TranscriptionAdapter adapts transcription.Service to session.TranscriptionServiceInterface.
type TranscriptionAdapter struct {
	Svc *transcription.Service
}

func (a *TranscriptionAdapter) TranscribeAudio(ctx context.Context, audioData *session.AudioData) error {
	return a.Svc.TranscribeAudio(ctx, &stt.AudioData{
		SessionID:     audioData.SessionID,
		ParticipantID: audioData.ParticipantID,
		Role:          audioData.Role,
		Data:          audioData.Data,
		Frames:        audioData.Frames,
		SampleRate:    audioData.SampleRate,
		Duration:      audioData.Duration,
		OffsetMs:      audioData.OffsetMs,
	})
}

func (a *TranscriptionAdapter) GetTranscriptionsAsText(ctx context.Context, sessionID string) (string, error) {
	return a.Svc.GetTranscriptionsAsText(ctx, sessionID)
}

// MinutesAdapter adapts minutes.Service to session.MinutesServiceInterface.
type MinutesAdapter struct {
	Svc *minutes.Service
}

func (a *MinutesAdapter) GenerateMinutes(ctx context.Context, sessionID, transcriptionText string, tmpl config.TemplateConfig) (interface{}, error) {
	return a.Svc.GenerateMinutes(ctx, sessionID, transcriptionText, tmpl)
}

func (a *MinutesAdapter) GetMinutes(ctx context.Context, sessionID string) (interface{}, error) {
	return a.Svc.GetMinutes(ctx, sessionID)
}
