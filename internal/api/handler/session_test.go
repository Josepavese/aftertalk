package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockSessionService struct {
	mock.Mock
}

func (m *MockSessionService) CreateSession(ctx context.Context, req *session.CreateSessionRequest) (*session.CreateSessionResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*session.CreateSessionResponse), args.Error(1)
}

func (m *MockSessionService) GetSession(ctx context.Context, sessionID string) (*session.Session, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*session.Session), args.Error(1)
}

func (m *MockSessionService) EndSession(ctx context.Context, sessionID string) error {
	args := m.Called(ctx, sessionID)
	return args.Error(0)
}

func (m *MockSessionService) ValidateParticipant(ctx context.Context, jti string) (*session.Participant, error) {
	args := m.Called(ctx, jti)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*session.Participant), args.Error(1)
}

func (m *MockSessionService) ConnectParticipant(ctx context.Context, participantID string) error {
	args := m.Called(ctx, participantID)
	return args.Error(0)
}

func TestSessionHandler_CreateSession(t *testing.T) {
	tests := []struct {
		name           string
		request        CreateSessionRequest
		mockSetup      func(*MockSessionService)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "Success - valid request",
			request: CreateSessionRequest{
				ParticipantCount: 3,
				Participants: []ParticipantRequest{
					{UserID: "user1", Role: "moderator"},
					{UserID: "user2", Role: "participant"},
					{UserID: "user3", Role: "participant"},
				},
			},
			mockSetup: func(m *MockSessionService) {
				m.On("CreateSession", mock.Anything, mock.AnythingOfType("*session.CreateSessionRequest")).
					Return(&session.CreateSessionResponse{
						SessionID: "test-session-id",
						Participants: []session.ParticipantResponse{
							{
								ID:        "p1",
								UserID:    "user1",
								Role:      "moderator",
								Token:     "valid-token",
								ExpiresAt: "2026-03-04T12:00:00Z",
							},
						},
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response map[string]interface{}
				assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
				assert.Equal(t, float64(http.StatusOK), rec.Code)
				assert.Contains(t, response, "session_id")
				assert.Contains(t, response, "participants")
			},
		},
		{
			name: "Failure - insufficient participants",
			request: CreateSessionRequest{
				ParticipantCount: 1,
				Participants: []ParticipantRequest{
					{UserID: "user1", Role: "participant"},
				},
			},
			mockSetup:      func(m *MockSessionService) {},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "at least 2 participants required", rec.Body.String())
			},
		},
		{
			name: "Failure - invalid JSON",
			request: CreateSessionRequest{
				ParticipantCount: 2,
				Participants:     []ParticipantRequest{{UserID: "user1", Role: "participant"}},
			},
			mockSetup:      func(m *MockSessionService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "Invalid request body", rec.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockSessionService)
			tt.mockSetup(mockService)
			handler := NewSessionHandler(mockService)

			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/session", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.CreateSession(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestSessionHandler_GetSession(t *testing.T) {
	tests := []struct {
		name           string
		sessionID      string
		mockSetup      func(*MockSessionService)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:      "Success - valid session",
			sessionID: "valid-session-id",
			mockSetup: func(m *MockSessionService) {
				m.On("GetSession", mock.Anything, "valid-session-id").
					Return(&session.Session{
						ID:               "valid-session-id",
						Status:           session.StatusActive,
						ParticipantCount: 3,
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var response session.Session
				assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
				assert.Equal(t, "valid-session-id", response.ID)
				assert.Equal(t, float64(http.StatusOK), rec.Code)
			},
		},
		{
			name:           "Failure - session not found",
			sessionID:      "non-existent",
			mockSetup:      func(m *MockSessionService) {},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, rec.Body.String(), "failed to get session: failed to get session")
			},
		},
		{
			name:           "Failure - empty session ID",
			sessionID:      "",
			mockSetup:      func(m *MockSessionService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "Session ID required", rec.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockSessionService)
			tt.mockSetup(mockService)
			handler := NewSessionHandler(mockService)

			req := httptest.NewRequest("GET", "/session/"+tt.sessionID, nil)
			rec := httptest.NewRecorder()

			handler.GetSession(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			mockService.AssertExpectations(t)
		})
	}
}
