package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Josepavese/aftertalk/internal/core/minutes"
	"github.com/Josepavese/aftertalk/internal/logging"
)

var (
	errMinutesNotFound        = errors.New("failed to get minutes: not found")
	errMinutesSimpleNotFound  = errors.New("not found")
	errMinutesHistoryNotFound = errors.New("failed to get minutes history: not found")
)

func init() { //nolint:gochecknoinits // test initialization
	logging.Init("info", "console") //nolint:errcheck
}

type MockMinutesService struct {
	mock.Mock
}

func (m *MockMinutesService) GetMinutes(ctx context.Context, sessionID string) (*minutes.Minutes, error) {
	args := m.Called(ctx, sessionID)
	v, _ := args.Get(0).(*minutes.Minutes)
	return v, args.Error(1)
}

func (m *MockMinutesService) GetMinutesByID(ctx context.Context, id string) (*minutes.Minutes, error) {
	args := m.Called(ctx, id)
	v, _ := args.Get(0).(*minutes.Minutes)
	return v, args.Error(1)
}

func (m *MockMinutesService) UpdateMinutes(ctx context.Context, id string, updatedMinutes *minutes.Minutes, editedBy string) (*minutes.Minutes, error) {
	args := m.Called(ctx, id, updatedMinutes, editedBy)
	v, _ := args.Get(0).(*minutes.Minutes)
	return v, args.Error(1)
}

func (m *MockMinutesService) DeleteMinutes(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockMinutesService) GetMinutesHistory(ctx context.Context, minutesID string) ([]*minutes.MinutesHistory, error) {
	args := m.Called(ctx, minutesID)
	v, _ := args.Get(0).([]*minutes.MinutesHistory)
	return v, args.Error(1)
}

func (m *MockMinutesService) ListWebhookEvents(ctx context.Context, sessionID, minutesID string) ([]minutes.WebhookEvent, error) {
	args := m.Called(ctx, sessionID, minutesID)
	v, _ := args.Get(0).([]minutes.WebhookEvent)
	return v, args.Error(1)
}

func (m *MockMinutesService) ReplayWebhookEvent(ctx context.Context, eventID string) error {
	args := m.Called(ctx, eventID)
	return args.Error(0)
}

func (m *MockMinutesService) ConsumeRetrievalToken(ctx context.Context, tokenID string) (*minutes.RetrievalToken, error) {
	args := m.Called(ctx, tokenID)
	v, _ := args.Get(0).(*minutes.RetrievalToken)
	return v, args.Error(1)
}

func (m *MockMinutesService) PurgeMinutes(ctx context.Context, minutesID string) {
	m.Called(ctx, minutesID)
}

func addChiContext(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestMinutesHandler_GetMinutes(t *testing.T) {
	tests := []struct {
		mockSetup      func(*MockMinutesService)
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
		name           string
		sessionID      string
		expectedStatus int
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
						Provider:  "gpt-4",
						Status:    minutes.MinutesStatusReady,
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var m minutes.Minutes
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, "min-123", m.ID)
				assert.Equal(t, "gpt-4", m.Provider)
			},
		},
		{
			name:      "Failure - session not found",
			sessionID: "non-existent",
			mockSetup: func(m *MockMinutesService) {
				m.On("GetMinutes", mock.Anything, "non-existent").
					Return(nil, errMinutesNotFound)
			},
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
				assert.Equal(t, "Session ID required", strings.TrimSpace(rec.Body.String()))
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
		mockSetup      func(*MockMinutesService)
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
		name           string
		minutesID      string
		expectedStatus int
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
						Status:    minutes.MinutesStatusReady,
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var m minutes.Minutes
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, "claude-3", m.Provider)
			},
		},
		{
			name:      "Failure - minutes not found",
			minutesID: "non-existent",
			mockSetup: func(m *MockMinutesService) {
				m.On("GetMinutesByID", mock.Anything, "non-existent").
					Return(nil, errMinutesSimpleNotFound)
			},
			expectedStatus: http.StatusNotFound,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Contains(t, rec.Body.String(), "not found")
			},
		},
		{
			name:           "Failure - empty minutes ID",
			minutesID:      "",
			mockSetup:      func(m *MockMinutesService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "Minutes ID required", strings.TrimSpace(rec.Body.String()))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockMinutesService)
			tt.mockSetup(mockService)
			handler := NewMinutesHandler(mockService)

			req := httptest.NewRequest("GET", "/minutes/"+tt.minutesID, nil)
			if tt.minutesID != "" {
				req = addChiContext(req, "id", tt.minutesID)
			}
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
		mockSetup      func(*MockMinutesService)
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
		request        minutes.Minutes
		minutesID      string
		name           string
		rawBody        []byte
		expectedStatus int
	}{
		{
			name:      "Success - valid update",
			minutesID: "min-123",
			request: minutes.Minutes{
				ID:        "min-123",
				SessionID: "session-456",
				Version:   1,
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
					}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				var m minutes.Minutes
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Equal(t, 2, m.Version)
			},
		},
		{
			name:           "Failure - invalid JSON",
			minutesID:      "min-123",
			rawBody:        []byte("not valid json"),
			mockSetup:      func(m *MockMinutesService) {},
			expectedStatus: http.StatusBadRequest,
			checkResponse: func(t *testing.T, rec *httptest.ResponseRecorder) {
				assert.Equal(t, "Invalid request body", strings.TrimSpace(rec.Body.String()))
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
					Return(nil, errMinutesSimpleNotFound)
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

			var body []byte
			if tt.rawBody != nil {
				body = tt.rawBody
			} else {
				body, _ = json.Marshal(tt.request)
			}
			req := httptest.NewRequestWithContext(context.Background(), "PUT", "/minutes/"+tt.minutesID, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-User-Id", "test-user")
			if tt.minutesID != "" {
				req = addChiContext(req, "id", tt.minutesID)
			}
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
		mockSetup      func(*MockMinutesService)
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
		name           string
		minutesID      string
		expectedStatus int
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
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &history))
				assert.Equal(t, http.StatusOK, rec.Code)
				assert.Len(t, history, 2)
				assert.Equal(t, "first version", history[0].Content)
			},
		},
		{
			name:      "Failure - minutes not found",
			minutesID: "non-existent",
			mockSetup: func(m *MockMinutesService) {
				m.On("GetMinutesHistory", mock.Anything, "non-existent").
					Return(nil, errMinutesHistoryNotFound)
			},
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
				assert.Equal(t, "Minutes ID required", strings.TrimSpace(rec.Body.String()))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockMinutesService)
			tt.mockSetup(mockService)
			handler := NewMinutesHandler(mockService)

			req := httptest.NewRequest("GET", "/minutes/"+tt.minutesID+"/versions", nil)
			if tt.minutesID != "" {
				req = addChiContext(req, "id", tt.minutesID)
			}
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

func TestMinutesHandler_ListWebhookEvents(t *testing.T) {
	mockService := new(MockMinutesService)
	mockService.On("ListWebhookEvents", mock.Anything, "session-1", "minutes-1").
		Return([]minutes.WebhookEvent{
			{
				ID:            "event-1",
				MinutesID:     "minutes-1",
				WebhookURL:    "https://example.test/hook",
				PayloadType:   "error",
				Status:        "failed",
				AttemptNumber: 5,
			},
		}, nil)
	handler := NewMinutesHandler(mockService)

	req := httptest.NewRequest("GET", "/webhooks/events?session_id=session-1&minutes_id=minutes-1", nil)
	rec := httptest.NewRecorder()

	handler.ListWebhookEvents(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var events []minutes.WebhookEvent
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &events))
	require.Len(t, events, 1)
	assert.Equal(t, "event-1", events[0].ID)
	assert.Equal(t, "error", events[0].PayloadType)
	mockService.AssertExpectations(t)
}

func TestMinutesHandler_ReplayWebhookEvent(t *testing.T) {
	mockService := new(MockMinutesService)
	mockService.On("ReplayWebhookEvent", mock.Anything, "event-1").Return(nil)
	handler := NewMinutesHandler(mockService)

	req := httptest.NewRequest("POST", "/webhooks/events/event-1/replay", nil)
	req = addChiContext(req, "id", "event-1")
	rec := httptest.NewRecorder()

	handler.ReplayWebhookEvent(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	mockService.AssertExpectations(t)
}

func TestPullMinutes_PurgeUsesBoundedBackgroundContext(t *testing.T) {
	mockService := new(MockMinutesService)
	handler := NewMinutesHandlerWithConfig(mockService, true)
	mins := minutes.NewMinutes("minutes-1", "session-1", "therapy")
	mins.MarkReady()

	req := httptest.NewRequest(http.MethodGet, "/v1/minutes/pull/token-1", nil)
	reqCtx, cancelReq := context.WithCancel(req.Context())
	req = addChiContext(req.WithContext(reqCtx), "token", "token-1")
	rec := httptest.NewRecorder()
	type purgeCtxState struct {
		err         error
		hasDeadline bool
	}
	purgeCtxCh := make(chan purgeCtxState, 1)

	mockService.On("ConsumeRetrievalToken", mock.Anything, "token-1").
		Return(&minutes.RetrievalToken{ID: "token-1", MinutesID: "minutes-1"}, nil)
	mockService.On("GetMinutesByID", mock.Anything, "minutes-1").
		Run(func(_ mock.Arguments) {
			cancelReq()
		}).
		Return(mins, nil)
	mockService.On("PurgeMinutes", mock.Anything, "minutes-1").
		Run(func(args mock.Arguments) {
			purgeCtx := args.Get(0).(context.Context) //nolint:forcetypeassert
			_, hasDeadline := purgeCtx.Deadline()
			purgeCtxCh <- purgeCtxState{err: purgeCtx.Err(), hasDeadline: hasDeadline}
		})

	handler.PullMinutes(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	select {
	case purgeCtx := <-purgeCtxCh:
		assert.NoError(t, purgeCtx.err)
		assert.True(t, purgeCtx.hasDeadline)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for purge")
	}
	mockService.AssertExpectations(t)
}
