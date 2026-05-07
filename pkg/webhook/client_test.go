package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		timeout time.Duration
	}{
		{
			name:    "valid configuration",
			url:     "https://example.com/webhook",
			timeout: 30 * time.Second,
		},
		{
			name:    "default timeout",
			url:     "https://example.com/webhook",
			timeout: 0,
		},
		{
			name:    "custom timeout",
			url:     "https://example.com/webhook",
			timeout: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.url, tt.timeout)
			if client == nil {
				t.Fatal("NewClient returned nil")
			}
			if client.url != tt.url {
				t.Errorf("Expected URL %s, got %s", tt.url, client.url)
			}
			if client.timeout != tt.timeout {
				t.Errorf("Expected timeout %v, got %v", tt.timeout, client.timeout)
			}
			if client.httpClient == nil {
				t.Fatal("httpClient is nil")
			}
		})
	}
}

func TestClient_Send_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/webhook" {
			t.Errorf("Expected /webhook, got %s", r.URL.Path)
		}
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", contentType)
		}

		var payload MinutesPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Failed to decode payload: %v", err)
		}
		if payload.SessionID != "test-session" {
			t.Errorf("Expected session_id test-session, got %s", payload.SessionID)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/webhook", 5*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	err := client.Send(context.Background(), payload)
	if err != nil {
		t.Errorf("Send() failed: %v", err)
	}
}

func TestClient_Send_InvalidURL(t *testing.T) {
	client := NewClient("", 5*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	err := client.Send(context.Background(), payload)
	if err != nil {
		t.Errorf("Expected no error for empty URL, got: %v", err)
	}
}

func TestClient_Send_MarshalError(t *testing.T) {
	client := NewClient("https://example.com/webhook", 5*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   make(chan int), // Invalid type for JSON
		Timestamp: time.Now(),
	}

	err := client.Send(context.Background(), payload)
	if err == nil {
		t.Error("Expected error for invalid payload type, got nil")
	}
	if !strings.Contains(err.Error(), "failed to marshal payload") {
		t.Errorf("Expected error to mention 'failed to marshal payload', got: %v", err)
	}
}

func TestClient_Send_RequestCreationError(t *testing.T) {
	client := NewClient("http://invalid-host-12345.com:9999/webhook", 5*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	err := client.Send(context.Background(), payload)
	if err == nil {
		t.Error("Expected error for request creation, got nil")
	}
	if !strings.Contains(err.Error(), "failed to send webhook") {
		t.Errorf("Expected error to mention 'failed to send webhook', got: %v", err)
	}
}

func TestClient_Send_HTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, 1*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Send(ctx, payload)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestClient_Send_ConnectionRefused(t *testing.T) {
	client := NewClient("http://localhost:1/webhook", 1*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	err := client.Send(context.Background(), payload)
	if err == nil {
		t.Error("Expected connection refused error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to send webhook") {
		t.Errorf("Expected error to mention 'failed to send webhook', got: %v", err)
	}
}

func TestClient_Send_Non2xxStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad request"))
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	err := client.Send(context.Background(), payload)
	if err == nil {
		t.Error("Expected error for non-2xx status code, got nil")
	}
	if !strings.Contains(err.Error(), "webhook returned bad status") {
		t.Errorf("Expected error to mention 'webhook returned status', got: %v", err)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("Expected error to mention status code 400, got: %v", err)
	}
}

func TestClient_Send_500StatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	err := client.Send(context.Background(), payload)
	if err == nil {
		t.Error("Expected error for 500 status code, got nil")
	}
	if !strings.Contains(err.Error(), "webhook returned bad status") {
		t.Errorf("Expected error to mention 'webhook returned status', got: %v", err)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Expected error to mention status code 500, got: %v", err)
	}
}

func TestClient_Send_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, 30*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := client.Send(ctx, payload)
	if err == nil {
		t.Error("Expected context canceled error, got nil")
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected context canceled error, got: %v", err)
	}
}

func TestClient_Send_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, 1*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Send(ctx, payload)
	if err == nil {
		t.Error("Expected context deadline exceeded error, got nil")
	}
	if !strings.Contains(err.Error(), "deadline exceeded") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected context deadline exceeded or timeout error, got: %v", err)
	}
}

func TestClient_Send_PayloadWithComplexData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload MinutesPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("Failed to decode payload: %v", err)
		}
		if payload.SessionID != "complex-session" {
			t.Errorf("Expected session_id complex-session, got %s", payload.SessionID)
		}
		minutesMap, ok := payload.Minutes.(map[string]interface{})
		if !ok || len(minutesMap) == 0 {
			t.Error("Expected minutes to be non-empty")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)
	payload := &MinutesPayload{
		SessionID: "complex-session",
		Minutes: map[string]interface{}{
			"summary":      "Test summary",
			"actionItems":  []string{"item1", "item2"},
			"participants": []string{"user1", "user2"},
		},
		Timestamp: time.Now(),
	}

	err := client.Send(context.Background(), payload)
	if err != nil {
		t.Errorf("Send() failed: %v", err)
	}
}

func TestClient_Send_ResponseBodyHandling(t *testing.T) {
	responseBody := `{"error": "Something went wrong"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(responseBody))
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	err := client.Send(context.Background(), payload)
	if err == nil {
		t.Error("Expected error for 409 status code, got nil")
	}
	expectedMsg := "webhook returned bad status 409: " + responseBody
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error message to contain %s, got: %v", expectedMsg, err)
	}
}

func TestClient_Send_MultipleRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)
	payload1 := &MinutesPayload{
		SessionID: "session-1",
		Minutes:   map[string]string{"summary": "First summary"},
		Timestamp: time.Now(),
	}
	payload2 := &MinutesPayload{
		SessionID: "session-2",
		Minutes:   map[string]string{"summary": "Second summary"},
		Timestamp: time.Now(),
	}
	payload3 := &MinutesPayload{
		SessionID: "session-3",
		Minutes:   map[string]string{"summary": "Third summary"},
		Timestamp: time.Now(),
	}

	for _, payload := range []*MinutesPayload{payload1, payload2, payload3} {
		if err := client.Send(context.Background(), payload); err != nil {
			t.Errorf("Send() failed: %v", err)
		}
	}
}

func TestMinutesPayload_JSONSerialization(t *testing.T) {
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes: map[string]string{
			"summary": "Test minutes",
		},
		Timestamp: time.Now(),
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	var decoded MinutesPayload
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if decoded.SessionID != payload.SessionID {
		t.Errorf("Expected SessionID %s, got %s", payload.SessionID, decoded.SessionID)
	}
}

func TestClient_Send_401Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	err := client.Send(context.Background(), payload)
	if err == nil {
		t.Error("Expected error for 401 status code, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected error to mention status code 401, got: %v", err)
	}
}

func TestClient_Send_404NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not found"))
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	err := client.Send(context.Background(), payload)
	if err == nil {
		t.Error("Expected error for 404 status code, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected error to mention status code 404, got: %v", err)
	}
}

func TestClient_Send_503ServiceUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Service unavailable"))
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	err := client.Send(context.Background(), payload)
	if err == nil {
		t.Error("Expected error for 503 status code, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("Expected error to mention status code 503, got: %v", err)
	}
}

func TestClient_Send_ZeroTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(10 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, 0)
	payload := &MinutesPayload{
		SessionID: "test-session",
		Minutes:   map[string]string{"summary": "Test summary"},
		Timestamp: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Send(ctx, payload)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Errorf("Expected timeout/deadline error, got: %v", err)
	}
}

// --- SessionContext and payload enrichment tests ---

func TestSessionContext_EmptyOmittedFromJSON(t *testing.T) {
	payload := &MinutesPayload{
		SessionID: "sess-1",
		Minutes:   map[string]string{},
		Timestamp: time.Now(),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["session_metadata"]; ok {
		t.Error("session_metadata should be omitted when empty")
	}
	if _, ok := m["participants"]; ok {
		t.Error("participants should be omitted when empty")
	}
}

func TestMinutesPayload_SessionContextRoundtrip(t *testing.T) {
	payload := &MinutesPayload{
		SessionID:       "sess-1",
		Minutes:         map[string]string{"summary": "ok"},
		Timestamp:       time.Now(),
		SessionMetadata: `{"appointment_id":"appt_123","doctor_id":"doc_456"}`,
		Participants: []ParticipantSummary{
			{UserID: "doc_456", Role: "terapeuta"},
			{UserID: "pat_789", Role: "paziente"},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded MinutesPayload
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.SessionMetadata != payload.SessionMetadata {
		t.Errorf("SessionMetadata: got %q, want %q", decoded.SessionMetadata, payload.SessionMetadata)
	}
	if len(decoded.Participants) != 2 {
		t.Fatalf("Participants: got %d, want 2", len(decoded.Participants))
	}
	if decoded.Participants[0].UserID != "doc_456" || decoded.Participants[0].Role != "terapeuta" {
		t.Errorf("Participants[0]: got %+v", decoded.Participants[0])
	}
}

func TestNotificationPayload_SessionContextRoundtrip(t *testing.T) {
	payload := &NotificationPayload{
		SessionID:       "sess-1",
		RetrieveURL:     "https://aftertalk.example/v1/minutes/pull/tok_abc",
		ExpiresAt:       time.Now().Add(time.Hour),
		Timestamp:       time.Now(),
		SessionMetadata: `{"appointment_id":"appt_999"}`,
		Participants: []ParticipantSummary{
			{UserID: "usr_1", Role: "terapeuta"},
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded NotificationPayload
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.SessionMetadata != payload.SessionMetadata {
		t.Errorf("SessionMetadata: got %q, want %q", decoded.SessionMetadata, payload.SessionMetadata)
	}
	if len(decoded.Participants) != 1 || decoded.Participants[0].Role != "terapeuta" {
		t.Errorf("Participants: got %+v", decoded.Participants)
	}
}

func TestClient_SendError_SignedWithSecret(t *testing.T) {
	var signature string
	var received ErrorPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signature = r.Header.Get("X-Aftertalk-Signature")
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClientWithSecret(server.URL, "01234567890123456789012345678901", 5*time.Second)
	payload := &ErrorPayload{
		SessionID:    "sess-error",
		MinutesID:    "min-error",
		Status:       "error",
		ErrorCode:    "provider_timeout",
		ErrorMessage: "deadline exceeded",
		Timestamp:    time.Now(),
	}
	if err := client.SendError(context.Background(), payload); err != nil {
		t.Fatalf("SendError: %v", err)
	}
	if !strings.HasPrefix(signature, "hmac-sha256=") {
		t.Fatalf("expected HMAC signature, got %q", signature)
	}
	if received.SessionID != "sess-error" || received.ErrorCode != "provider_timeout" {
		t.Fatalf("unexpected error payload: %+v", received)
	}
}

func TestMinutesPayload_DeliveredWithSessionContext(t *testing.T) {
	var received MinutesPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, 5*time.Second)
	payload := &MinutesPayload{
		SessionID:       "sess-deliver",
		Minutes:         map[string]string{"section": "content"},
		Timestamp:       time.Now(),
		SessionMetadata: `{"appointment_id":"appt_001","doctor_id":"doc_001"}`,
		Participants: []ParticipantSummary{
			{UserID: "doc_001", Role: "terapeuta"},
			{UserID: "pat_002", Role: "paziente"},
		},
	}
	if err := client.Send(context.Background(), payload); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if received.SessionMetadata != payload.SessionMetadata {
		t.Errorf("server received SessionMetadata %q, want %q", received.SessionMetadata, payload.SessionMetadata)
	}
	if len(received.Participants) != 2 {
		t.Errorf("server received %d participants, want 2", len(received.Participants))
	}
}

func TestSessionContext_ZeroValue(t *testing.T) {
	var ctx SessionContext
	if ctx.Metadata != "" {
		t.Error("zero-value Metadata should be empty string")
	}
	if ctx.Participants != nil {
		t.Error("zero-value Participants should be nil")
	}
}
