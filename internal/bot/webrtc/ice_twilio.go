package webrtc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

var errTwilioServerError = errors.New("twilio ice: server error")

// TwilioProvider fetches ICE credentials from the Twilio Network Traversal Service.
//
// API: POST https://api.twilio.com/2010-04-01/Accounts/{SID}/Tokens.json
// Auth: HTTP Basic (AccountSID : AuthToken)
// Docs: https://www.twilio.com/docs/stun-turn
type TwilioProvider struct {
	accountSID   string
	authToken    string
	client       *http.Client
	endpointBase string // override for tests; default: https://api.twilio.com
}

func NewTwilioProvider(accountSID, authToken string) *TwilioProvider {
	return &TwilioProvider{
		accountSID:   accountSID,
		authToken:    authToken,
		client:       &http.Client{Timeout: defaultHTTPTimeout},
		endpointBase: "https://api.twilio.com",
	}
}

// SetEndpoint overrides the API base URL (no trailing slash). Used in tests only.
func (p *TwilioProvider) SetEndpoint(base string) { p.endpointBase = base }

func (p *TwilioProvider) Name() string { return "twilio" }

type twilioTokenResponse struct {
	Username   string            `json:"username"`
	Password   string            `json:"password"`
	TTL        string            `json:"ttl"`
	ICEServers []twilioICEServer `json:"ice_servers"`
}

// twilioICEServer uses "url" (singular) as Twilio returns it.
type twilioICEServer struct {
	URL        string `json:"url"`
	Username   string `json:"username,omitempty"`
	Credential string `json:"credential,omitempty"`
}

func (p *TwilioProvider) GetICEServers(ctx context.Context, ttl int) ([]ICEServer, error) {
	if ttl <= 0 {
		ttl = 86400
	}

	endpoint := fmt.Sprintf("%s/2010-04-01/Accounts/%s/Tokens.json", p.endpointBase, p.accountSID)

	formBody := url.Values{}
	formBody.Set("Ttl", strconv.Itoa(ttl))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		bytes.NewBufferString(formBody.Encode()))
	if err != nil {
		return nil, fmt.Errorf("twilio ice: build request: %w", err)
	}
	req.SetBasicAuth(p.accountSID, p.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twilio ice: http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("twilio ice: read body: %w", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d: %s", errTwilioServerError, resp.StatusCode, string(raw))
	}

	var token twilioTokenResponse
	if err := json.Unmarshal(raw, &token); err != nil {
		return nil, fmt.Errorf("twilio ice: decode response: %w", err)
	}

	servers := make([]ICEServer, 0, len(token.ICEServers))
	for _, s := range token.ICEServers {
		if s.URL == "" {
			continue
		}
		servers = append(servers, ICEServer{
			URLs:       []string{s.URL},
			Username:   s.Username,
			Credential: s.Credential,
		})
	}
	return servers, nil
}
