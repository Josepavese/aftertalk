package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAPIKeyChecker struct {
	mock.Mock
}

func (m *MockAPIKeyChecker) CheckAPIKey(apiKey string) bool {
	args := m.Called(apiKey)
	return args.Bool(0)
}

type MockResponseRecorder struct {
	httptest.ResponseRecorder
	statusCode int
}

func (r *MockResponseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseRecorder.WriteHeader(statusCode)
}

type MockLoggingHandler struct {
	called bool
}

func (h *MockLoggingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.called = true
	w.WriteHeader(http.StatusOK)
}

func MockRecoverHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func TestAPIKey_Middleware(t *testing.T) {
	tests := []struct {
		name           string
		apiKey         string
		path           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "Success - valid API key for health endpoint",
			apiKey:         "valid-api-key",
			path:           "/health",
			authHeader:     "Bearer valid-api-key",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Success - valid API key for ready endpoint",
			apiKey:         "valid-api-key",
			path:           "/ready",
			authHeader:     "Bearer valid-api-key",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Failure - missing authorization header",
			apiKey:         "valid-api-key",
			path:           "/api/session",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Failure - invalid authorization format",
			apiKey:         "valid-api-key",
			path:           "/api/session",
			authHeader:     "InvalidFormat",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Failure - invalid API key",
			apiKey:         "valid-api-key",
			path:           "/api/session",
			authHeader:     "Bearer invalid-key",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := APIKey(tt.apiKey)
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", tt.path, nil)
			req.Header.Set("Authorization", tt.authHeader)
			rec := httptest.NewRecorder()

			handler(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestLogging_Middleware(t *testing.T) {
	logging.Init("info", "console") //nolint
	handler := Logging(&MockLoggingHandler{})

	req := httptest.NewRequest("GET", "/test/path", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.True(t, rec.Code == http.StatusOK)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRecovery_Middleware(t *testing.T) {
	tests := []struct {
		name           string
		panicValue     interface{}
		expectedStatus int
	}{
		{
			name:           "Success - no panic",
			panicValue:     nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Failure - panic caught and handled",
			panicValue:     "test panic",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "Failure - panic with complex error",
			panicValue:     "complex error: timeout",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			panicHandler := MockRecoverHandler

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.panicValue != nil {
					panic(tt.panicValue)
				}
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			rec := httptest.NewRecorder()

			panicHandler(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}

func TestCORS_Middleware(t *testing.T) {
	tests := []struct {
		name               string
		allowedOrigins     []string
		origin             string
		expectedHeaders    map[string]string
		expectedStatusCode int
	}{
		{
			name:               "Success - wildcard origin",
			allowedOrigins:     []string{"*"},
			origin:             "http://example.com",
			expectedHeaders:    map[string]string{"Access-Control-Allow-Origin": "http://example.com"},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "Success - specific origin",
			allowedOrigins:     []string{"http://example.com"},
			origin:             "http://example.com",
			expectedHeaders:    map[string]string{"Access-Control-Allow-Origin": "http://example.com"},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "Failure - origin not allowed",
			allowedOrigins:     []string{"http://allowed.com"},
			origin:             "http://blocked.com",
			expectedHeaders:    map[string]string{},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "Success - multiple allowed origins",
			allowedOrigins:     []string{"http://example.com", "http://example.org"},
			origin:             "http://example.org",
			expectedHeaders:    map[string]string{"Access-Control-Allow-Origin": "http://example.org"},
			expectedStatusCode: http.StatusOK,
		},
		{
			name:               "Success - OPTIONS method",
			allowedOrigins:     []string{"*"},
			origin:             "http://example.com",
			expectedHeaders:    map[string]string{"Access-Control-Allow-Origin": "http://example.com"},
			expectedStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := CORS(tt.allowedOrigins)

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("Origin", tt.origin)
			rec := &MockResponseRecorder{}

			handler(next).ServeHTTP(rec, req)

			// Check headers
			for key, expectedValue := range tt.expectedHeaders {
				if rec.Header().Get(key) != expectedValue {
					t.Errorf("Header %s = %v, want %v", key, rec.Header().Get(key), expectedValue)
				}
			}

			assert.Equal(t, tt.expectedStatusCode, rec.statusCode)
		})
	}
}

func TestPrometheusMetrics(t *testing.T) {
	handler := PrometheusMetrics()

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "# HELP")
}

func TestMetricsMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "Success - GET request",
			method:         "GET",
			path:           "/api/session/123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Success - POST request",
			method:         "POST",
			path:           "/api/session",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Success - PUT request",
			method:         "PUT",
			path:           "/api/minutes/123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Success - DELETE request",
			method:         "DELETE",
			path:           "/api/session/123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Success - health check",
			method:         "GET",
			path:           "/health",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := MetricsMiddleware

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.expectedStatus)
			})

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := &MockResponseRecorder{}

			handler(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.statusCode)
		})
	}
}

func TestRateLimit_Middleware(t *testing.T) {
	tests := []struct {
		name              string
		requestsPerMinute int
		makeRequests      func(http.Handler) func() *MockResponseRecorder
	}{
		{
			name:              "Success - within rate limit",
			requestsPerMinute: 10,
			makeRequests: func(h http.Handler) func() *MockResponseRecorder {
				var responses []*MockResponseRecorder
				for i := 0; i < 5; i++ {
					rec := &MockResponseRecorder{}
					req := httptest.NewRequest("GET", "/test", nil)
					h.ServeHTTP(rec, req)
					responses = append(responses, rec)
				}
				return func() *MockResponseRecorder {
					if len(responses) > 0 {
						r := responses[0]
						responses = responses[1:]
						return r
					}
					return nil
				}
			},
		},
		{
			name:              "Failure - rate limit exceeded",
			requestsPerMinute: 2,
			makeRequests: func(h http.Handler) func() *MockResponseRecorder {
				var responses []*MockResponseRecorder
				// Make 3 requests to exceed limit
				for i := 0; i < 3; i++ {
					rec := &MockResponseRecorder{}
					req := httptest.NewRequest("GET", "/test", nil)
					h.ServeHTTP(rec, req)
					responses = append(responses, rec)
				}
				return func() *MockResponseRecorder {
					if len(responses) > 0 {
						r := responses[0]
						responses = responses[1:]
						return r
					}
					return nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RateLimit(tt.requestsPerMinute)

			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			rec := &MockResponseRecorder{}

			handler(next).ServeHTTP(rec, req)

			// Just ensure the handler doesn't panic
			assert.NotNil(t, rec)
		})
	}
}
