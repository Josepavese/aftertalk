package api

import (
	"context"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/ai/stt"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/core/minutes"
	"github.com/Josepavese/aftertalk/internal/core/session"
	"github.com/Josepavese/aftertalk/internal/core/transcription"
	"github.com/Josepavese/aftertalk/pkg/webhook"
)

// TranscriptionAdapter adapts transcription.Service to session.TranscriptionServiceInterface.
// It is the Middleware layer responsible for resolving the STT provider profile before
// dispatching to the Logic layer (transcription.Service).
type TranscriptionAdapter struct {
	Svc         *transcription.Service
	STTRegistry *stt.STTRegistry
}

func (a *TranscriptionAdapter) TranscribeAudio(ctx context.Context, audioData *session.AudioData) error {
	provider := a.STTRegistry.Get(audioData.STTProfile)
	return a.Svc.TranscribeAudio(ctx, provider, &stt.AudioData{
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

func (a *TranscriptionAdapter) GetDetectedLanguageForSession(ctx context.Context, sessionID string) string {
	return a.Svc.GetDetectedLanguageForSession(ctx, sessionID)
}

// MinutesAdapter adapts minutes.Service to session.MinutesServiceInterface.
// It is the Middleware layer responsible for resolving the LLM provider profile before
// dispatching to the Logic layer (minutes.Service).
type MinutesAdapter struct {
	Svc         *minutes.Service
	LLMRegistry *llm.LLMRegistry
}

func (a *MinutesAdapter) GenerateMinutes(ctx context.Context, sessionID, transcriptionText string, tmpl config.TemplateConfig, sessCtx webhook.SessionContext, detectedLanguage string, llmProfile string) (interface{}, error) {
	provider := a.LLMRegistry.Get(llmProfile)
	return a.Svc.GenerateMinutes(ctx, sessionID, transcriptionText, tmpl, sessCtx, detectedLanguage, provider)
}

func (a *MinutesAdapter) GetMinutes(ctx context.Context, sessionID string) (interface{}, error) {
	return a.Svc.GetMinutes(ctx, sessionID)
}
