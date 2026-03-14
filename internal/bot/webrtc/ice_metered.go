package webrtc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var errMeteredServerError = errors.New("metered ice: server error")

// MeteredProvider fetches ICE credentials from the Metered.ca TURN service.
//
// API: GET https://{appName}.metered.live/api/v1/turn/credentials?apiKey={key}
// Auth: query parameter apiKey (no Authorization header)
// Docs: https://www.metered.ca/docs/turn-rest-api
type MeteredProvider struct {
	appName    string
	apiKey     string
	client     *http.Client
	baseURL    string // override for tests
}

func NewMeteredProvider(appName, apiKey string) *MeteredProvider {
	return &MeteredProvider{
		appName: appName,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: defaultHTTPTimeout},
	}
}

// SetBaseURL overrides the Metered base URL. Used in tests only.
func (p *MeteredProvider) SetBaseURL(u string) { p.baseURL = u }

func (p *MeteredProvider) Name() string { return "metered" }

// meteredICEServer is one entry from the Metered response array.
// Metered uses "urls" as a single string (not an array), contrary to the spec.
type meteredICEServer struct {
	URLs       string `json:"urls"` // single URL string in Metered's response
	Username   string `json:"username,omitempty"`
	Credential string `json:"credential,omitempty"`
}

func (p *MeteredProvider) GetICEServers(ctx context.Context, _ int) ([]ICEServer, error) {
	var endpoint string
	if p.baseURL != "" {
		endpoint = fmt.Sprintf("%s/api/v1/turn/credentials?apiKey=%s", p.baseURL, p.apiKey)
	} else {
		endpoint = fmt.Sprintf("https://%s.metered.live/api/v1/turn/credentials?apiKey=%s",
			p.appName, p.apiKey)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("metered ice: build request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("metered ice: http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("metered ice: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d: %s", errMeteredServerError, resp.StatusCode, string(raw))
	}

	var entries []meteredICEServer
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("metered ice: decode response: %w", err)
	}

	servers := make([]ICEServer, 0, len(entries))
	for _, e := range entries {
		if e.URLs == "" {
			continue
		}
		servers = append(servers, ICEServer{
			URLs:       []string{e.URLs},
			Username:   e.Username,
			Credential: e.Credential,
		})
	}
	return servers, nil
}
