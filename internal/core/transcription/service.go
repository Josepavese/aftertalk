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
		transcription := NewTranscription(
			uuid.New().String(),
			audioData.SessionID,
			i,
			segment.Role,
			segment.StartMs,
			segment.EndMs,
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
