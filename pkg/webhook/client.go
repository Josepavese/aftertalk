// Package webhook provides HTTP delivery of Aftertalk events to external systems.
//
// Two payload types are supported:
//
//   - MinutesPayload: "push" mode — full minutes JSON in the POST body together with
//     session metadata and participant summary.
//   - NotificationPayload: "notify_pull" mode — only a signed retrieval URL is sent;
//     the recipient calls GET /v1/minutes/pull/{token} to fetch the actual minutes.
//     Session metadata and participants are included here so the recipient has full
//     context without needing a second API call.
//
// Both payloads carry a SessionContext so that recipients can associate the delivery
// with their own data model (e.g. appointment_id, doctor_id) without maintaining a
// separate session-id → context mapping table on their side.
//
// The Client signs outgoing notification webhooks with HMAC-SHA256 over the request
// body using the configured Secret.  Recipients should verify the
// X-Aftertalk-Signature header before proceeding to pull:
//
//	mac := hmac.New(sha256.New, []byte(webhookSecret))
//	mac.Write(requestBody)
//	expected := "hmac-sha256=" + hex.EncodeToString(mac.Sum(nil))
//	if !hmac.Equal([]byte(expected), []byte(r.Header.Get("X-Aftertalk-Signature"))) {
//	    http.Error(w, "invalid signature", 401)
//	}
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Josepavese/aftertalk/internal/logging"
)

var errWebhookBadStatus = errors.New("webhook returned bad status")

// ParticipantSummary is a compact record of a session participant included in
// webhook payloads. It lets recipients identify who joined without a separate API call.
type ParticipantSummary struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// SessionContext carries the opaque metadata and participant list associated with
// a session. It is set at session-creation time by the integrating backend and
// propagated unchanged through the call chain to every webhook delivery.
//
// Metadata is a raw JSON string — Aftertalk never inspects or modifies it. A
// typical value from a telemedicine backend might look like:
//
//	{"appointment_id":"appt_123","doctor_id":"doc_456","patient_id":"pat_789"}
//
// Participants contains one entry per participant in the order they were created.
// Both fields are omitted from JSON when empty to keep legacy payloads unchanged.
type SessionContext struct {
	Metadata     string               `json:"metadata,omitempty"`
	Participants []ParticipantSummary `json:"participants,omitempty"`
}

// Client delivers webhook payloads to an external HTTP endpoint.
type Client struct {
	httpClient *http.Client
	url        string
	secret     string
	timeout    time.Duration
}

func (c *Client) URL() string { return c.url }

func NewClient(url string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		url:        url,
		timeout:    timeout,
	}
}

// NewClientWithSecret creates a Client with HMAC signing enabled.
// secret must be at least 32 bytes in production.
func NewClientWithSecret(url, secret string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		url:        url,
		timeout:    timeout,
		secret:     secret,
	}
}

// MinutesPayload is the "push" webhook body.
// The full minutes JSON is sent directly to the recipient together with the
// session context (metadata + participants) set at session-creation time.
// Use NotificationPayload instead when sensitive data must not travel in the
// webhook body (notify_pull mode).
type MinutesPayload struct {
	Timestamp       time.Time            `json:"timestamp"`
	Minutes         interface{}          `json:"minutes"`
	SessionID       string               `json:"session_id"`
	SessionMetadata string               `json:"session_metadata,omitempty"`
	Participants    []ParticipantSummary `json:"participants,omitempty"`
}

// NotificationPayload is the "notify_pull" webhook body.
// It carries only a signed, single-use retrieval URL — no medical data.
// The session context (metadata + participants) is included here so the
// recipient has full routing context without needing to pull first.
// The URL expires at ExpiresAt and becomes invalid after the first successful retrieval.
type NotificationPayload struct {
	ExpiresAt       time.Time            `json:"expires_at"`
	Timestamp       time.Time            `json:"timestamp"`
	SessionID       string               `json:"session_id"`
	RetrieveURL     string               `json:"retrieve_url"`
	SessionMetadata string               `json:"session_metadata,omitempty"`
	Participants    []ParticipantSummary `json:"participants,omitempty"`
}

// ErrorPayload is a terminal failure notification. It intentionally contains
// no transcript/minutes content; recipients use it to stop "in progress"
// polling and surface an operator-safe failure state.
type ErrorPayload struct {
	Timestamp       time.Time            `json:"timestamp"`
	SessionID       string               `json:"session_id"`
	MinutesID       string               `json:"minutes_id,omitempty"`
	Status          string               `json:"status"`
	ErrorCode       string               `json:"error_code"`
	ErrorMessage    string               `json:"error_message,omitempty"`
	SessionMetadata string               `json:"session_metadata,omitempty"`
	Participants    []ParticipantSummary `json:"participants,omitempty"`
}

// Send delivers a MinutesPayload (push mode). No HMAC signing.
func (c *Client) Send(ctx context.Context, payload *MinutesPayload) error {
	if c.url == "" {
		logging.Warnf("webhook URL not configured, skipping notification")
		return nil
	}

	logging.Infof("sending webhook to %s for session %s", c.url, payload.SessionID)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.do(req, payload.SessionID)
}

// SendNotification delivers a NotificationPayload (notify_pull mode).
// If a secret is configured, adds an X-Aftertalk-Signature HMAC-SHA256 header
// over the raw JSON body so the recipient can verify authenticity.
func (c *Client) SendNotification(ctx context.Context, payload *NotificationPayload) error {
	if c.url == "" {
		logging.Warnf("webhook URL not configured, skipping notification")
		return nil
	}

	logging.Infof("sending notification webhook to %s for session %s", c.url, payload.SessionID)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal notification payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// HMAC-SHA256 signature over the raw JSON body.
	// Header format: "hmac-sha256=<hex-digest>"
	if c.secret != "" {
		mac := hmac.New(sha256.New, []byte(c.secret))
		mac.Write(jsonPayload)
		req.Header.Set("X-Aftertalk-Signature", "hmac-sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	return c.do(req, payload.SessionID)
}

func (c *Client) SendError(ctx context.Context, payload *ErrorPayload) error {
	if c.url == "" {
		logging.Warnf("webhook URL not configured, skipping error notification")
		return nil
	}

	logging.Infof("sending error webhook to %s for session %s", c.url, payload.SessionID)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal error payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		mac := hmac.New(sha256.New, []byte(c.secret))
		mac.Write(jsonPayload)
		req.Header.Set("X-Aftertalk-Signature", "hmac-sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	return c.do(req, payload.SessionID)
}

// do executes the HTTP request and checks for a non-4xx/5xx response.
func (c *Client) do(req *http.Request, sessionID string) error {
	resp, err := c.httpClient.Do(req) //nolint:gosec // URL comes from server configuration, not user input
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read webhook response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w %d: %s", errWebhookBadStatus, resp.StatusCode, string(body))
	}

	logging.Infof("webhook sent successfully for session %s", sessionID)
	return nil
}
