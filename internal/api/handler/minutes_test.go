package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flowup/aftertalk/internal/core/minutes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockMinutesService struct {
	mock.Mock
}

func (m *MockMinutesService) GetMinutes(ctx context.Context, sessionID string) (*minutes.Minutes, error) {
	args := m.Called(ctx, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*minutes.Minutes), args.Error(1)
}

func (m *MockMinutesService) GetMinutesByID(ctx context.Context, id string) (*minutes.Minutes, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*minutes.Minutes), args.Error(1)
}

func (m *MockMinutesService) UpdateMinutes(ctx context.Context, id string, updatedMinutes *minutes.Minutes, editedBy string) (*minutes.Minutes, error) {
	args := m.Called(ctx, id, updatedMinutes, editedBy)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*minutes.Minutes), args.Error(1)
}

func (m *MockMinutesService) GetMinutesHistory(ctx context.Context, minutesID string) ([]*minutes.MinutesHistory, error) {
	args := m.Called(ctx, minutesID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*minutes.MinutesHistory), args.Error(1)
}

func TestMinutesHandler_GetMinutes(t *testing.T) {
	tests := []struct {
		name           string
		sessionID      string
		mockSetup      func(*MockMinutesService)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:      "Success - valid session with minutes",
			sessionID: "session-123",
			mockSetup: func(m *MockMinutesService) {
				m.On("GetMinutes", mock.Anything, "session-123").
					Return(&minutes.Minutes{
						ID:        "min-123",
						SessionID: "session-123",
						Version:   1,
						Themes:    []string{"meeting", "strategy"},
						NextSteps: []string{"follow up", "send report"},
						Status:    minutes.MinutesStatusReady,
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var minutes minutes.Minutes
				assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &minutes))
				assert.Equal(t, float64(http.StatusOK), rec.Code)
				assert.Equal(t, "min-123", minutes.ID)
				assert.Equal(t, "gpt-4", minutes.Provider)
			},
		},
		{
			name:           "Failure - session not found",
			sessionID:      "non-existent",
			mockSetup:      func(m *MockMinutesService) {},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "failed to get minutes")
			},
		},
		{
			name:           "Failure - empty session ID",
			sessionID:      "",
			mockSetup:      func(m *MockMinutesService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "Session ID required", rec.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockMinutesService)
			tt.mockSetup(mockService)
			handler := NewMinutesHandler(mockService)

			req := httptest.NewRequest("GET", "/minutes?session_id="+tt.sessionID, nil)
			rec := httptest.NewRecorder()

			handler.GetMinutes(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestMinutesHandler_GetMinutesByID(t *testing.T) {
	tests := []struct {
		name           string
		minutesID      string
		mockSetup      func(*MockMinutesService)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:      "Success - valid minutes",
			minutesID: "min-123",
			mockSetup: func(m *MockMinutesService) {
				m.On("GetMinutesByID", mock.Anything, "min-123").
					Return(&minutes.Minutes{
						ID:        "min-123",
						SessionID: "session-456",
						Version:   2,
						Provider:  "claude-3",
						NextSteps: []string{"call back", "send report"},
						Status:    minutes.MinutesStatusReady,
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var minutes minutes.Minutes
				assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &minutes))
				assert.Equal(t, float64(http.StatusOK), rec.Code)
				assert.Equal(t, "claude-3", minutes.Provider)
			},
		},
		{
			name:           "Failure - minutes not found",
			minutesID:      "non-existent",
			mockSetup:      func(m *MockMinutesService) {},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "failed to get minutes")
			},
		},
		{
			name:           "Failure - empty minutes ID",
			minutesID:      "",
			mockSetup:      func(m *MockMinutesService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "Minutes ID required", rec.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockMinutesService)
			tt.mockSetup(mockService)
			handler := NewMinutesHandler(mockService)

			req := httptest.NewRequest("GET", "/minutes/"+tt.minutesID, nil)
			rec := httptest.NewRecorder()

			handler.GetMinutesByID(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestMinutesHandler_UpdateMinutes(t *testing.T) {
	tests := []struct {
		name           string
		minutesID      string
		request        minutes.Minutes
		mockSetup      func(*MockMinutesService)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:      "Success - valid update",
			minutesID: "min-123",
			request: minutes.Minutes{
				ID:        "min-123",
				SessionID: "session-456",
				Version:   1,
				NextSteps: []string{"update meeting notes", "follow up on decisions"},
			},
			mockSetup: func(m *MockMinutesService) {
				m.On("GetMinutesByID", mock.Anything, "min-123").
					Return(&minutes.Minutes{
						ID:        "min-123",
						SessionID: "session-456",
						Version:   1,
					}, nil)
				m.On("UpdateMinutes", mock.Anything, "min-123", mock.AnythingOfType("*minutes.Minutes"), "test-user").
					Return(&minutes.Minutes{
						ID:        "min-123",
						SessionID: "session-456",
						Version:   2,
						NextSteps: []string{"update meeting notes", "follow up on decisions"},
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var minutes minutes.Minutes
				assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &minutes))
				assert.Equal(t, float64(http.StatusOK), rec.Code)
				assert.Equal(t, int64(2), minutes.Version)
			},
		},
		{
			name:           "Failure - invalid JSON",
			minutesID:      "min-123",
			request:        minutes.Minutes{},
			mockSetup:      func(m *MockMinutesService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "Invalid request body", rec.Body.String())
			},
		},
		{
			name:      "Failure - minutes not found",
			minutesID: "non-existent",
			request: minutes.Minutes{
				ID:        "min-123",
				SessionID: "session-456",
			},
			mockSetup: func(m *MockMinutesService) {
				m.On("GetMinutesByID", mock.Anything, "non-existent").
					Return(nil, errors.New("not found"))

				m.On("UpdateMinutes", mock.Anything, "non-existent", mock.AnythingOfType("*minutes.Minutes"), "test-user").
					Return(nil, errors.New("not found"))
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "failed to get minutes")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockMinutesService)
			tt.mockSetup(mockService)
			handler := NewMinutesHandler(mockService)

			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("PUT", "/minutes/"+tt.minutesID, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-User-ID", "test-user")
			rec := httptest.NewRecorder()

			handler.UpdateMinutes(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			mockService.AssertExpectations(t)
		})
	}
}

func TestMinutesHandler_GetMinutesHistory(t *testing.T) {
	tests := []struct {
		name           string
		minutesID      string
		mockSetup      func(*MockMinutesService)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:      "Success - valid history",
			minutesID: "min-123",
			mockSetup: func(m *MockMinutesService) {
				m.On("GetMinutesHistory", mock.Anything, "min-123").
					Return([]*minutes.MinutesHistory{
						{
							ID:        "hist-1",
							MinutesID: "min-123",
							Version:   1,
							Content:   "first version",
						},
						{
							ID:        "hist-2",
							MinutesID: "min-123",
							Version:   2,
							Content:   "second version",
						},
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var history []*minutes.MinutesHistory
				assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &history))
				assert.Equal(t, float64(http.StatusOK), rec.Code)
				assert.Len(t, history, 2)
				assert.Equal(t, "first version", history[0].Content)
			},
		},
		{
			name:           "Failure - minutes not found",
			minutesID:      "non-existent",
			mockSetup:      func(m *MockMinutesService) {},
			expectedStatus: http.StatusInternalServerError,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "failed to get minutes history")
			},
		},
		{
			name:           "Failure - empty minutes ID",
			minutesID:      "",
			mockSetup:      func(m *MockMinutesService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "Minutes ID required", rec.Body.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockMinutesService)
			tt.mockSetup(mockService)
			handler := NewMinutesHandler(mockService)

			req := httptest.NewRequest("GET", "/minutes/"+tt.minutesID+"/versions", nil)
			rec := httptest.NewRecorder()

			handler.GetMinutesHistory(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, rec)
			}

			mockService.AssertExpectations(t)
		})
	}
}
