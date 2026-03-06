package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flowup/aftertalk/internal/core/transcription"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockTranscriptionService struct {
	mock.Mock
}

func (m *MockTranscriptionService) GetTranscriptions(ctx context.Context, sessionID string) ([]*transcription.Transcription, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*transcription.Transcription), args.Error(1)
}

func (m *MockTranscriptionService) GetTranscriptionByID(ctx context.Context, id string) (*transcription.Transcription, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*transcription.Transcription), args.Error(1)
}

func TestTranscriptionHandler_GetTranscriptions(t *testing.T) {
	tests := []struct {
		name           string
		sessionID      string
		mockSetup      func(*MockTranscriptionService)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:      "Success - valid session with transcriptions",
			sessionID: "session-123",
			mockSetup: func(m *MockTranscriptionService) {
				m.On("GetTranscriptions", mock.Anything, "session-123").
					Return([]*transcription.Transcription{
						{
							ID:           "trans-1",
							SessionID:    "session-123",
							SegmentIndex: 0,
							Role:         "moderator",
							StartMs:      1000,
							EndMs:        2000,
							Text:         "Hello everyone",
							Confidence:   0.95,
							Provider:     "whisper",
						},
						{
							ID:           "trans-2",
							SessionID:    "session-123",
							SegmentIndex: 1,
							Role:         "participant",
							StartMs:      2000,
							EndMs:        3000,
							Text:         "How are you?",
							Confidence:   0.92,
							Provider:     "whisper",
						},
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var transcriptions []*transcription.Transcription
				assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &transcriptions))
				assert.Equal(t, float64(http.StatusOK), rec.Code)
				assert.Len(t, transcriptions, 2)
				assert.Equal(t, "Hello everyone", transcriptions[0].Text)
				assert.Equal(t, "How are you?", transcriptions[1].Text)
			},
		},
		{
			name:           "Failure - session not found",
			sessionID:      "non-existent",
			mockSetup:      func(m *MockTranscriptionService) {},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "failed to get transcriptions")
			},
		},
		{
			name:           "Failure - empty session ID",
			sessionID:      "",
			mockSetup:      func(m *MockTranscriptionService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "Session ID required", rec.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockTranscriptionService)
			tt.mockSetup(mockService)
			handler := NewTranscriptionHandler(mockService)

			req := httptest.NewRequest("GET", "/transcription?session_id="+tt.sessionID, nil)
			rec := httptest.NewRecorder()

			handler.GetTranscriptions(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestTranscriptionHandler_GetTranscriptionByID(t *testing.T) {
	tests := []struct {
		name            string
		transcriptionID string
		mockSetup       func(*MockTranscriptionService)
		expectedStatus  int
		checkResponse   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:            "Success - valid transcription",
			transcriptionID: "trans-123",
			mockSetup: func(m *MockTranscriptionService) {
				m.On("GetTranscriptionByID", mock.Anything, "trans-123").
					Return(&transcription.Transcription{
						ID:           "trans-123",
						SessionID:    "session-456",
						SegmentIndex: 0,
						Role:         "moderator",
						StartMs:      1000,
						EndMs:        2000,
						Text:         "Meeting started",
						Confidence:   0.98,
						Provider:     "whisper",
						Status:       transcription.StatusReady,
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var transcription *transcription.Transcription
				assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &transcription))
				assert.Equal(t, float64(http.StatusOK), rec.Code)
				assert.Equal(t, "Meeting started", transcription.Text)
			},
		},
		{
			name:            "Failure - transcription not found",
			transcriptionID: "non-existent",
			mockSetup:       func(m *MockTranscriptionService) {},
			expectedStatus:  http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "failed to get transcription")
			},
		},
		{
			name:            "Failure - empty transcription ID",
			transcriptionID: "",
			mockSetup:       func(m *MockTranscriptionService) {},
			expectedStatus:  http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "Transcription ID required", rec.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockTranscriptionService)
			tt.mockSetup(mockService)
			handler := NewTranscriptionHandler(mockService)

			req := httptest.NewRequest("GET", "/transcription/"+tt.transcriptionID, nil)
			rec := httptest.NewRecorder()

			handler.GetTranscriptionByID(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			mockService.AssertExpectations(t)
		})
	}
}
