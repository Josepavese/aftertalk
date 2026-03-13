package stt_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/flowup/aftertalk/internal/ai/stt"
)

func TestDefaultRetryConfig(t *testing.T) {
	cfg := stt.DefaultRetryConfig()

	if cfg.MaxAttempts != 3 {
		t.Errorf("MaxAttempts mismatch: got %d, want 3", cfg.MaxAttempts)
	}
	if cfg.InitialDelay != 1*time.Second {
		t.Errorf("InitialDelay mismatch: got %v, want 1s", cfg.InitialDelay)
	}
	if cfg.MaxDelay != 30*time.Second {
		t.Errorf("MaxDelay mismatch: got %v, want 30s", cfg.MaxDelay)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("Multiplier mismatch: got %f, want 2.0", cfg.Multiplier)
	}
}

func TestDefaultRetryConfig_Values(t *testing.T) {
	tests := []struct {
		name         string
		maxAttempts  int
		initialDelay time.Duration
		maxDelay     time.Duration
		multiplier   float64
	}{
		{"default config", 3, 1 * time.Second, 30 * time.Second, 2.0},
		{"custom max attempts", 5, 1 * time.Second, 30 * time.Second, 2.0},
		{"custom initial delay", 3, 2 * time.Second, 30 * time.Second, 2.0},
		{"custom max delay", 3, 1 * time.Second, 60 * time.Second, 2.0},
		{"custom multiplier", 3, 1 * time.Second, 30 * time.Second, 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &stt.RetryConfig{
				MaxAttempts:  tt.maxAttempts,
				InitialDelay: tt.initialDelay,
				MaxDelay:     tt.maxDelay,
				Multiplier:   tt.multiplier,
			}

			if cfg.MaxAttempts != tt.maxAttempts {
				t.Errorf("MaxAttempts mismatch")
			}
			if cfg.InitialDelay != tt.initialDelay {
				t.Errorf("InitialDelay mismatch")
			}
			if cfg.MaxDelay != tt.maxDelay {
				t.Errorf("MaxDelay mismatch")
			}
			if cfg.Multiplier != tt.multiplier {
				t.Errorf("Multiplier mismatch")
			}
		})
	}
}

func TestTranscriptionError(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		message  string
		cause    error
		hasCause bool
	}{
		{
			name:     "with cause",
			provider: "google",
			message:  "transcription failed",
			cause:    errors.New("API error"),
			hasCause: true,
		},
		{
			name:     "without cause",
			provider: "aws",
			message:  "network timeout",
			cause:    nil,
			hasCause: false,
		},
		{
			name:     "empty cause",
			provider: "azure",
			message:  "generic error",
			cause:    nil,
			hasCause: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := stt.NewTranscriptionError(tt.provider, tt.message, tt.cause)

			if err == nil {
				t.Fatal("Expected error to be created")
			}

			if err.Error() == "" {
				t.Error("Expected error message")
			}

			if tt.hasCause {
				unwrapped := errors.Unwrap(err)
				if unwrapped != tt.cause {
					t.Errorf("Unwrapped error mismatch: got %v, want %v", unwrapped, tt.cause)
				}
			}
		})
	}
}

func TestTranscriptionError_Error(t *testing.T) {
	tests := []struct {
		name        string
		provider    string
		message     string
		cause       error
		expectedMsg string
	}{
		{
			name:        "with cause",
			provider:    "google",
			message:     "transcription failed",
			cause:       errors.New("API error"),
			expectedMsg: "STT error [google]: transcription failed: API error",
		},
		{
			name:        "without cause",
			provider:    "aws",
			message:     "network timeout",
			cause:       nil,
			expectedMsg: "STT error [aws]: network timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := stt.NewTranscriptionError(tt.provider, tt.message, tt.cause)

			errorMsg := err.Error()
			if errorMsg != tt.expectedMsg {
				t.Errorf("Error message mismatch:\ngot:      %s\nwant:     %s", errorMsg, tt.expectedMsg)
			}
		})
	}
}

func TestTranscriptionError_Unwrap(t *testing.T) {
	cause := errors.New("original error")
	err := stt.NewTranscriptionError("google", "transcription failed", cause)

	unwrapped := errors.Unwrap(err)

	if unwrapped != cause {
		t.Errorf("Unwrap error mismatch: got %v, want %v", unwrapped, cause)
	}
}

func TestTranscribeWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	provider := &mockSTTProvider{
		returnError: nil,
	}
	audioData := &stt.AudioData{
		SessionID: "session1",
		Data:      []byte("test data"),
		Duration:  60,
	}

	cfg := stt.DefaultRetryConfig()
	result, err := stt.TranscribeWithRetry(context.Background(), provider, audioData, cfg)

	if err != nil {
		t.Errorf("Expected success on first attempt, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if provider.callCount != 1 {
		t.Errorf("Expected 1 call, got %d", provider.callCount)
	}
}

func TestTranscribeWithRetry_SuccessOnRetry(t *testing.T) {
	provider := &mockSTTProvider{
		returnError: errors.New("temporary error"),
		callCount:   0,
		maxErrors:   2,
	}
	audioData := &stt.AudioData{
		SessionID: "session1",
		Data:      []byte("test data"),
		Duration:  60,
	}

	cfg := stt.DefaultRetryConfig()
	result, err := stt.TranscribeWithRetry(context.Background(), provider, audioData, cfg)

	if err != nil {
		t.Errorf("Expected success after retries, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if provider.callCount != 3 {
		t.Errorf("Expected 3 calls (1 initial + 2 retries), got %d", provider.callCount)
	}
}

func TestTranscribeWithRetry_FailAfterMaxRetries(t *testing.T) {
	provider := &mockSTTProvider{
		returnError: errors.New("persistent error"),
		callCount:   0,
		maxErrors:   5,
	}
	audioData := &stt.AudioData{
		SessionID: "session1",
		Data:      []byte("test data"),
		Duration:  60,
	}

	cfg := stt.DefaultRetryConfig()
	_, err := stt.TranscribeWithRetry(context.Background(), provider, audioData, cfg)

	if err == nil {
		t.Fatal("Expected error after max retries")
	}

	transcriptionErr, ok := err.(*stt.TranscriptionError)
	if !ok {
		t.Errorf("Expected TranscriptionError, got %T", err)
	}

	if transcriptionErr.Provider != "mock" {
		t.Errorf("Expected provider name 'mock', got %s", transcriptionErr.Provider)
	}
	if provider.callCount != cfg.MaxAttempts {
		t.Errorf("Expected %d calls, got %d", cfg.MaxAttempts, provider.callCount)
	}
}

func TestTranscribeWithRetry_ContextCancellation(t *testing.T) {
	provider := &mockSTTProvider{
		returnError: errors.New("persistent error"),
		callCount:   0,
		maxErrors:   5,
	}
	audioData := &stt.AudioData{
		SessionID: "session1",
		Data:      []byte("test data"),
		Duration:  60,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cfg := stt.DefaultRetryConfig()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := stt.TranscribeWithRetry(ctx, provider, audioData, cfg)

	if err != context.Canceled {
		t.Errorf("Expected context cancellation error, got: %v", err)
	}
	if provider.callCount != 1 {
		t.Errorf("Expected 1 call before context cancellation, got %d", provider.callCount)
	}
}

func TestTranscribeWithRetry_RetryDelays(t *testing.T) {
	provider := &mockSTTProvider{
		returnError: errors.New("temporary error"),
		callCount:   0,
		maxErrors:   2,
	}
	audioData := &stt.AudioData{
		SessionID: "session1",
		Data:      []byte("test data"),
		Duration:  60,
	}

	cfg := &stt.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     1 * time.Second,
		Multiplier:   2.0,
	}

	_, err := stt.TranscribeWithRetry(context.Background(), provider, audioData, cfg)

	if err != nil {
		t.Errorf("Expected success after retries, got error: %v", err)
	}

	if provider.callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", provider.callCount)
	}
}

func TestTranscribeWithRetry_MaxDelayClamping(t *testing.T) {
	provider := &mockSTTProvider{
		returnError: errors.New("temporary error"),
		callCount:   0,
		maxErrors:   9,
	}
	audioData := &stt.AudioData{
		SessionID: "session1",
		Data:      []byte("test data"),
		Duration:  60,
	}

	cfg := &stt.RetryConfig{
		MaxAttempts:  10,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     50 * time.Millisecond,
		Multiplier:   2.0,
	}

	_, err := stt.TranscribeWithRetry(context.Background(), provider, audioData, cfg)

	if err != nil {
		t.Errorf("Expected success after retries, got error: %v", err)
	}

	if provider.callCount != 10 {
		t.Errorf("Expected 10 calls, got %d", provider.callCount)
	}
}

func TestTranscribeWithRetry_AllErrors(t *testing.T) {
	provider := &mockSTTProvider{
		returnError: errors.New("error1"),
		callCount:   0,
		maxErrors:   10,
	}
	audioData := &stt.AudioData{
		SessionID: "session1",
		Data:      []byte("test data"),
		Duration:  60,
	}

	cfg := stt.DefaultRetryConfig()
	_, err := stt.TranscribeWithRetry(context.Background(), provider, audioData, cfg)

	if err == nil {
		t.Fatal("Expected error after all retries")
	}

	transcriptionErr, ok := err.(*stt.TranscriptionError)
	if !ok {
		t.Errorf("Expected TranscriptionError, got %T", err)
	}

	if transcriptionErr.Cause == nil {
		t.Error("Expected error cause")
	}

	if provider.callCount != cfg.MaxAttempts {
		t.Errorf("Expected %d calls, got %d", cfg.MaxAttempts, provider.callCount)
	}
}

func TestTranscribeWithRetry_SuccessWithResult(t *testing.T) {
	provider := &mockSTTProvider{
		returnError: nil,
	}
	audioData := &stt.AudioData{
		SessionID:     "session1",
		ParticipantID: "p1",
		Role:          "user",
		Data:          []byte("test data"),
		SampleRate:    16000,
		Duration:      60,
	}

	cfg := stt.DefaultRetryConfig()
	result, err := stt.TranscribeWithRetry(context.Background(), provider, audioData, cfg)

	if err != nil {
		t.Errorf("Expected success, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Provider != "mock" {
		t.Errorf("Expected provider 'mock', got %s", result.Provider)
	}
	if len(result.Segments) != 1 {
		t.Errorf("Expected 1 segment, got %d", len(result.Segments))
	}
	if result.Duration != 60 {
		t.Errorf("Expected duration 60ms, got %d", result.Duration)
	}
}

func TestTranscribeWithRetry_EmptyConfig(t *testing.T) {
	provider := &mockSTTProvider{
		returnError: nil,
	}
	audioData := &stt.AudioData{
		SessionID: "session1",
		Data:      []byte("test data"),
		Duration:  60,
	}

	cfg := &stt.RetryConfig{MaxAttempts: 1}
	result, err := stt.TranscribeWithRetry(context.Background(), provider, audioData, cfg)

	if err != nil {
		t.Errorf("Expected success with empty config, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestTranscribeWithRetry_MultipleProviders(t *testing.T) {
	provider1 := &mockSTTProvider{
		returnError: errors.New("error from provider 1"),
		callCount:   0,
		maxErrors:   2,
	}
	audioData := &stt.AudioData{
		SessionID: "session1",
		Data:      []byte("test data"),
		Duration:  60,
	}

	cfg := stt.DefaultRetryConfig()
	result, err := stt.TranscribeWithRetry(context.Background(), provider1, audioData, cfg)

	if err != nil {
		t.Errorf("Expected success after provider1 fails and provider2 succeeds, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if provider1.callCount != 3 {
		t.Errorf("Expected 3 calls to provider1, got %d", provider1.callCount)
	}
}

func TestTranscribeWithRetry_ErrorWithSpecificProvider(t *testing.T) {
	provider := &mockSTTProvider{
		returnError: errors.New("specific error"),
		callCount:   0,
		maxErrors:   10,
	}
	audioData := &stt.AudioData{
		SessionID: "session1",
		Data:      []byte("test data"),
		Duration:  60,
	}

	cfg := stt.DefaultRetryConfig()
	_, err := stt.TranscribeWithRetry(context.Background(), provider, audioData, cfg)

	if err == nil {
		t.Fatal("Expected error")
	}

	transcriptionErr, ok := err.(*stt.TranscriptionError)
	if !ok {
		t.Errorf("Expected TranscriptionError, got %T", err)
	}

	if transcriptionErr.Provider != "mock" {
		t.Errorf("Expected provider name 'mock', got %s", transcriptionErr.Provider)
	}
}

func TestTranscribeWithRetry_ContextBeforeFirstRetry(t *testing.T) {
	provider := &mockSTTProvider{
		returnError: errors.New("error1"),
		callCount:   0,
		maxErrors:   10,
	}
	audioData := &stt.AudioData{
		SessionID: "session1",
		Data:      []byte("test data"),
		Duration:  60,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cfg := stt.DefaultRetryConfig()

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := stt.TranscribeWithRetry(ctx, provider, audioData, cfg)

	if err != context.Canceled {
		t.Errorf("Expected context cancellation, got %v", err)
	}
	if provider.callCount != 1 {
		t.Errorf("Expected 1 call before cancellation, got %d", provider.callCount)
	}
}

type mockSTTProvider struct {
	returnError error
	callCount   int
	maxErrors   int
}

func (m *mockSTTProvider) Transcribe(ctx context.Context, audioData *stt.AudioData) (*stt.TranscriptionResult, error) {
	m.callCount++
	if m.callCount <= m.maxErrors && m.returnError != nil {
		return nil, m.returnError
	}
	result := stt.NewTranscriptionResult("mock")
	result.AddSegment(&stt.TranscriptionSegment{
		SessionID:  audioData.SessionID,
		Role:       audioData.Role,
		Text:       "mock transcription",
		Confidence: 1.0,
	})
	result.Duration = audioData.Duration
	return result, nil
}

func (m *mockSTTProvider) Name() string {
	return "mock"
}

func (m *mockSTTProvider) IsAvailable() bool {
	return true
}
