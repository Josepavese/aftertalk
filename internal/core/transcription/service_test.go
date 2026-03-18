package transcription

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Josepavese/aftertalk/internal/ai/stt"
)

var errTestTranscription = errors.New("transcription error")

type MockSTTProvider struct {
	mock.Mock
}

func (m *MockSTTProvider) Transcribe(ctx context.Context, audioData *stt.AudioData) (*stt.TranscriptionResult, error) {
	args := m.Called(ctx, audioData)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*stt.TranscriptionResult), args.Error(1) //nolint:forcetypeassert
}

func (m *MockSTTProvider) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockSTTProvider) IsAvailable() bool {
	args := m.Called()
	return args.Bool(0)
}

func TestTranscribeAudio_Success(t *testing.T) {
	if err := logging.Init("debug", "console"); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logging.Sync()

	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	mockProvider := new(MockSTTProvider)
	retryConfig := stt.DefaultRetryConfig()

	audioData := &stt.AudioData{
		SessionID:     "session-123",
		ParticipantID: "participant-456",
		Role:          "host",
		Data:          []byte{1, 2, 3},
		SampleRate:    48000,
		Duration:      10,
	}

	expectedSegments := []*stt.TranscriptionSegment{
		{
			ID:         "seg-1",
			SessionID:  "session-123",
			Role:       "host",
			StartMs:    0,
			EndMs:      500,
			Text:       "Hello world",
			Confidence: 0.95,
		},
		{
			ID:         "seg-2",
			SessionID:  "session-123",
			Role:       "host",
			StartMs:    500,
			EndMs:      1000,
			Text:       "This is a test",
			Confidence: 0.92,
		},
	}

	expectedResult := stt.NewTranscriptionResult("mock-provider")
	expectedResult.AddSegment(expectedSegments[0])
	expectedResult.AddSegment(expectedSegments[1])

	mockProvider.On("Transcribe", ctx, audioData).Return(expectedResult, nil)

	service := NewService(repo, retryConfig)

	err := service.TranscribeAudio(ctx, mockProvider, audioData)
	require.NoError(t, err)

	assert.True(t, mockProvider.AssertExpectations(t))

	transcriptions, err := repo.GetBySession(ctx, "session-123")
	require.NoError(t, err)
	require.Len(t, transcriptions, 2)

	for i, transcription := range transcriptions {
		assert.Equal(t, expectedSegments[i].Text, transcription.Text)
		assert.Equal(t, expectedSegments[i].StartMs, transcription.StartMs)
		assert.Equal(t, expectedSegments[i].EndMs, transcription.EndMs)
		assert.InEpsilon(t, expectedSegments[i].Confidence, transcription.Confidence, 1e-9)
		assert.Equal(t, "mock-provider", transcription.Provider)
		assert.Equal(t, StatusReady, transcription.Status)
		assert.Equal(t, i, transcription.SegmentIndex)
	}
}

func TestTranscribeAudio_RetryMultipleFailures(t *testing.T) {
	if err := logging.Init("debug", "console"); err != nil {
		t.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logging.Sync()

	ctx := context.Background()
	db := setupTestDB(t)
	repo := NewTranscriptionRepository(db)

	mockProvider := new(MockSTTProvider)
	retryConfig := &stt.RetryConfig{
		MaxAttempts:  4,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     200 * time.Millisecond,
		Multiplier:   1.5,
	}

	audioData := &stt.AudioData{
		SessionID:     "session-multi-fail",
		ParticipantID: "participant-1",
		Role:          "host",
		Data:          []byte{1, 2, 3},
		SampleRate:    48000,
		Duration:      10,
	}

	expectedSegments := []*stt.TranscriptionSegment{
		{
			ID:         "seg-1",
			SessionID:  "session-multi-fail",
			Role:       "host",
			StartMs:    0,
			EndMs:      500,
			Text:       "Success after failures",
			Confidence: 0.95,
		},
	}

	expectedResult := stt.NewTranscriptionResult("mock-provider")
	expectedResult.AddSegment(expectedSegments[0])

	// Fail first 2 attempts, succeed on 3rd
	mockProvider.On("Transcribe", ctx, audioData).Return(nil, errTestTranscription).Times(2)
	mockProvider.On("Transcribe", ctx, audioData).Return(expectedResult, nil)

	service := NewService(repo, retryConfig)

	err := service.TranscribeAudio(ctx, mockProvider, audioData)
	require.NoError(t, err)
}
