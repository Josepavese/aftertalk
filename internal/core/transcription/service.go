package transcription

import (
	"context"
	"fmt"

	"github.com/Josepavese/aftertalk/internal/ai/stt"
	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/google/uuid"
)

type Service struct {
	repo        *TranscriptionRepository
	sttProvider stt.STTProvider
	retryConfig *stt.RetryConfig
}

func NewService(repo *TranscriptionRepository, provider stt.STTProvider, retryConfig *stt.RetryConfig) *Service {
	return &Service{
		repo:        repo,
		sttProvider: provider,
		retryConfig: retryConfig,
	}
}

// GetDetectedLanguageForSession returns the language code detected by the STT
// provider for the most recent transcription segment of the given session.
// Returns "" if unknown.
func (s *Service) GetDetectedLanguageForSession(ctx context.Context, sessionID string) string {
	lang, err := s.repo.GetDetectedLanguage(ctx, sessionID)
	if err != nil {
		return ""
	}
	return lang
}

func (s *Service) TranscribeAudio(ctx context.Context, audioData *stt.AudioData) error {
	logging.Infof("Transcribing audio for session %s, participant %s", audioData.SessionID, audioData.ParticipantID)

	result, err := stt.TranscribeWithRetry(ctx, s.sttProvider, audioData, s.retryConfig)
	if err != nil {
		logging.Errorf("Transcription failed: %v", err)
		return fmt.Errorf("transcription failed: %w", err)
	}

	logging.Infof("Transcription completed: %d segments", len(result.Segments))

	// Offset segment_index by existing count so reconnections accumulate, not overwrite.
	existingCount, err := s.repo.CountBySession(ctx, audioData.SessionID)
	if err != nil {
		return fmt.Errorf("failed to count existing segments: %w", err)
	}

	for i, segment := range result.Segments {
		// Convert chunk-relative timestamps to session-absolute timestamps.
		// segment.StartMs/EndMs are relative to the beginning of the audio chunk;
		// audioData.OffsetMs is the time elapsed from session start to the chunk start.
		absStartMs := audioData.OffsetMs + segment.StartMs
		absEndMs := audioData.OffsetMs + segment.EndMs

		transcription := NewTranscription(
			uuid.New().String(),
			audioData.SessionID,
			existingCount+i,
			segment.Role,
			absStartMs,
			absEndMs,
			segment.Text,
		)
		transcription.SetConfidence(segment.Confidence)
		transcription.SetProvider(result.Provider)
		transcription.Language = result.DetectedLanguage
		transcription.MarkReady()

		if err := s.repo.Create(ctx, transcription); err != nil {
			logging.Errorf("Failed to save transcription segment %d: %v", i, err)
			return fmt.Errorf("failed to save transcription: %w", err)
		}
	}

	return nil
}

func (s *Service) GetTranscriptions(ctx context.Context, sessionID string) ([]*Transcription, error) {
	transcriptions, err := s.repo.GetBySessionOrdered(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transcriptions: %w", err)
	}

	return transcriptions, nil
}

func (s *Service) GetTranscriptionByID(ctx context.Context, id string) (*Transcription, error) {
	transcription, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get transcription: %w", err)
	}

	return transcription, nil
}

func (s *Service) GetTranscriptionsAsText(ctx context.Context, sessionID string) (string, error) {
	transcriptions, err := s.repo.GetBySessionOrdered(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get transcriptions: %w", err)
	}

	text := ""
	for _, t := range transcriptions {
		roleLabel := t.Role
		if roleLabel == "" {
			roleLabel = "speaker"
		}
		// Include ms explicitly so the LLM can copy them verbatim into citations/timestamps.
		text += fmt.Sprintf("[%dms %s]: %s\n", t.StartMs, roleLabel, t.Text)
	}

	return text, nil
}
