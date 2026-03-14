// Package webhook provides HTTP delivery of Aftertalk events to external systems.
//
// Two payload types are supported:
//
//   - MinutesPayload: legacy "push" mode — full minutes JSON in the POST body.
//   - NotificationPayload: "notify_pull" mode — only a signed retrieval URL is sent;
//     the recipient calls GET /v1/minutes/pull/{token} to fetch the actual data.
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
	"fmt"
	"io"
	"net/http"

	"github.com/Josepavese/aftertalk/internal/logging"
	"time"
)

// Client delivers webhook payloads to an external HTTP endpoint.
type Client struct {
	httpClient *http.Client
	url        string
	timeout    time.Duration
	// secret is the HMAC-SHA256 key used to sign NotificationPayload webhooks.
	// Empty means no signature header is added (legacy push mode).
	secret string
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

// MinutesPayload is the legacy "push" webhook body.
// The full minutes JSON is sent directly to the recipient.
// Use NotificationPayload instead for sensitive data.
type MinutesPayload struct {
	SessionID string      `json:"session_id"`
	Minutes   interface{} `json:"minutes"`
	Timestamp time.Time   `json:"timestamp"`
}

// NotificationPayload is the "notify_pull" webhook body.
// It carries only a signed, single-use retrieval URL — no medical data.
// The recipient must call RetrieveURL (with an optional TLS-pinned client)
// to fetch the actual minutes. The URL expires at ExpiresAt and becomes
// invalid after the first successful retrieval.
type NotificationPayload struct {
	// SessionID identifies the session whose minutes are ready.
	SessionID string `json:"session_id"`
	// RetrieveURL is the single-use URL the recipient must call to pull the data.
	// Format: {pull_base_url}/v1/minutes/pull/{token}
	RetrieveURL string `json:"retrieve_url"`
	// ExpiresAt is when the retrieval token becomes invalid.
	ExpiresAt time.Time `json:"expires_at"`
	// Timestamp is when this notification was generated.
	Timestamp time.Time `json:"timestamp"`
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

	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewBuffer(jsonPayload))
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

	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewBuffer(jsonPayload))
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

// do executes the HTTP request and checks for a non-4xx/5xx response.
func (c *Client) do(req *http.Request, sessionID string) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	logging.Infof("webhook sent successfully for session %s", sessionID)
	return nil
}
