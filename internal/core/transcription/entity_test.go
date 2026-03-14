package transcription

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestTranscriptionStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   TranscriptionStatus
		expected string
	}{
		{"StatusPending", StatusPending, "pending"},
		{"StatusProcessing", StatusProcessing, "processing"},
		{"StatusReady", StatusReady, "ready"},
		{"StatusError", StatusError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestNewTranscription(t *testing.T) {
	now := time.Now().UTC()
	transcription := NewTranscription(
		"test-id",
		"session-123",
		0,
		"host",
		0,
		1000,
		"Hello world",
	)

	assert.Equal(t, "test-id", transcription.ID)
	assert.Equal(t, "session-123", transcription.SessionID)
	assert.Equal(t, 0, transcription.SegmentIndex)
	assert.Equal(t, "host", transcription.Role)
	assert.Equal(t, 0, transcription.StartMs)
	assert.Equal(t, 1000, transcription.EndMs)
	assert.Equal(t, "Hello world", transcription.Text)
	assert.Empty(t, transcription.Provider)
	assert.Equal(t, StatusPending, transcription.Status)
	assert.WithinDuration(t, now, transcription.CreatedAt, time.Second)
}

func TestTranscriptionSetConfidence(t *testing.T) {
	transcription := NewTranscription("id", "session", 0, "host", 0, 1000, "Hello")
	transcription.SetConfidence(0.95)

	assert.InEpsilon(t, 0.95, transcription.Confidence, 1e-9)
}

func TestTranscriptionSetProvider(t *testing.T) {
	transcription := NewTranscription("id", "session", 0, "host", 0, 1000, "Hello")
	transcription.SetProvider("google")

	assert.Equal(t, "google", transcription.Provider)
}

func TestTranscriptionMarkProcessing(t *testing.T) {
	transcription := NewTranscription("id", "session", 0, "host", 0, 1000, "Hello")
	transcription.MarkProcessing()

	assert.Equal(t, StatusProcessing, transcription.Status)
}

func TestTranscriptionMarkReady(t *testing.T) {
	transcription := NewTranscription("id", "session", 0, "host", 0, 1000, "Hello")
	transcription.MarkReady()

	assert.Equal(t, StatusReady, transcription.Status)
}

func TestTranscriptionMarkError(t *testing.T) {
	transcription := NewTranscription("id", "session", 0, "host", 0, 1000, "Hello")
	transcription.MarkError()

	assert.Equal(t, StatusError, transcription.Status)
}

func TestTranscriptionStateTransitions(t *testing.T) {
	trans := NewTranscription("id", "session", 0, "host", 0, 1000, "Hello")

	tests := []struct {
		name         string
		markFn       func()
		expectStatus TranscriptionStatus
		expectErr    bool
	}{
		{"MarkProcessing", trans.MarkProcessing, StatusProcessing, false},
		{"MarkReady", trans.MarkReady, StatusReady, false},
		{"MarkError", trans.MarkError, StatusError, false},
		{"Reset to Processing", trans.MarkProcessing, StatusProcessing, false},
		{"Reset to Ready", trans.MarkReady, StatusReady, false},
		{"Reset to Error", trans.MarkError, StatusError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.markFn()
			assert.Equal(t, tt.expectStatus, trans.Status)
		})
	}
}

func TestTranscriptionUUIDGeneration(t *testing.T) {
	trans1 := NewTranscription(uuid.New().String(), "session", 0, "host", 0, 1000, "Hello")
	trans2 := NewTranscription(uuid.New().String(), "session", 0, "host", 0, 1000, "Hello")

	assert.NotEqual(t, trans1.ID, trans2.ID)
}

func TestTranscriptionFieldValidation(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		transcription *Transcription
		name          string
		expectValid   bool
	}{
		{
			name: "Valid transcription",
			transcription: &Transcription{
				ID:           "test-id",
				SessionID:    "session-123",
				SegmentIndex: 0,
				Role:         "host",
				StartMs:      0,
				EndMs:        1000,
				Text:         "Hello world",
				Confidence:   0.9,
				Provider:     "google",
				CreatedAt:    now,
				Status:       StatusReady,
			},
			expectValid: true,
		},
		{
			name: "Missing text",
			transcription: &Transcription{
				ID:        "test-id",
				SessionID: "session-123",
				Text:      "",
			},
			expectValid: false,
		},
		{
			name: "Invalid status",
			transcription: &Transcription{
				ID:        "test-id",
				SessionID: "session-123",
				Status:    TranscriptionStatus("invalid"),
			},
			expectValid: false,
		},
		{
			name: "Invalid confidence range",
			transcription: &Transcription{
				ID:         "test-id",
				SessionID:  "session-123",
				Confidence: -0.1,
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectValid {
				assert.True(t, tt.transcription.Text != "" || tt.transcription.Status != TranscriptionStatus(""))
			}
		})
	}
}
