package transcription

import (
	"context"
	"fmt"

	"github.com/flowup/aftertalk/internal/ai/stt"
	"github.com/flowup/aftertalk/internal/logging"
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

func (s *Service) TranscribeAudio(ctx context.Context, audioData *stt.AudioData) error {
	logging.Infof("Transcribing audio for session %s, participant %s", audioData.SessionID, audioData.ParticipantID)

	result, err := stt.TranscribeWithRetry(ctx, s.sttProvider, audioData, s.retryConfig)
	if err != nil {
		logging.Errorf("Transcription failed: %v", err)
		return fmt.Errorf("transcription failed: %w", err)
	}

	logging.Infof("Transcription completed: %d segments", len(result.Segments))

	for i, segment := range result.Segments {
		// Convert chunk-relative timestamps to session-absolute timestamps.
		// segment.StartMs/EndMs are relative to the beginning of the audio chunk;
		// audioData.OffsetMs is the time elapsed from session start to the chunk start.
		absStartMs := audioData.OffsetMs + segment.StartMs
		absEndMs := audioData.OffsetMs + segment.EndMs

		transcription := NewTranscription(
			uuid.New().String(),
			audioData.SessionID,
			i,
			segment.Role,
			absStartMs,
			absEndMs,
			segment.Text,
		)
		transcription.SetConfidence(segment.Confidence)
		transcription.SetProvider(result.Provider)
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
		timestamp := formatTimestampMs(t.StartMs)
		text += fmt.Sprintf("[%s %s]: %s\n", timestamp, roleLabel, t.Text)
	}

	return text, nil
}

// formatTimestampMs converts milliseconds to MM:SS format.
func formatTimestampMs(ms int) string {
	totalSec := ms / 1000
	minutes := totalSec / 60
	seconds := totalSec % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
