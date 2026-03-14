package stt

import (
	"context"
	"fmt"
	"time"

	"github.com/Josepavese/aftertalk/internal/logging"
)

type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
	}
}

type TranscriptionError struct {
	Cause    error
	Provider string
	Message  string
}

func (e *TranscriptionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("STT error [%s]: %s: %v", e.Provider, e.Message, e.Cause)
	}
	return fmt.Sprintf("STT error [%s]: %s", e.Provider, e.Message)
}

func (e *TranscriptionError) Unwrap() error {
	return e.Cause
}

func NewTranscriptionError(provider, message string, cause error) *TranscriptionError {
	return &TranscriptionError{
		Provider: provider,
		Message:  message,
		Cause:    cause,
	}
}

func TranscribeWithRetry(ctx context.Context, provider STTProvider, audioData *AudioData, cfg *RetryConfig) (*TranscriptionResult, error) {
	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		result, err := provider.Transcribe(ctx, audioData)
		if err == nil {
			return result, nil
		}

		lastErr = err
		logging.Warnf("STT attempt %d/%d failed: %v", attempt, cfg.MaxAttempts, err)

		if attempt < cfg.MaxAttempts {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			delay = time.Duration(float64(delay) * cfg.Multiplier)
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}
		}
	}

	return nil, NewTranscriptionError(provider.Name(), "transcription failed after retries", lastErr)
}
