package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
	url        string
	timeout    time.Duration
}

func NewClient(url string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		url:        url,
		timeout:    timeout,
	}
}

type MinutesPayload struct {
	SessionID string      `json:"session_id"`
	Minutes   interface{} `json:"minutes"`
	Timestamp time.Time   `json:"timestamp"`
}

func (c *Client) Send(ctx context.Context, payload *MinutesPayload) error {
	if c.url == "" {
		log.Println("[WARN] Webhook URL not configured, skipping notification")
		return nil
	}

	log.Printf("[INFO] Sending webhook to %s for session %s", c.url, payload.SessionID)

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("[INFO] Webhook sent successfully for session %s", payload.SessionID)
	return nil
}
