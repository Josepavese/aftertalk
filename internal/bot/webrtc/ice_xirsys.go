package webrtc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var (
	errXirsysServerError     = errors.New("xirsys ice: server error")
	errXirsysAPIError        = errors.New("xirsys ice: api error")
	errXirsysEmptyICEServers = errors.New("xirsys ice: empty ice_servers in response")
)

// XirsysProvider fetches ICE credentials from the Xirsys TURN network.
//
// API: PUT https://global.xirsys.net/_turn/{channel}
// Auth: HTTP Basic (ident : secretKey)
// Docs: https://docs.xirsys.com/?pg=api-turn
type XirsysProvider struct {
	ident     string
	secretKey string
	channel   string
	client    *http.Client
	baseURL   string // override for tests; default: https://global.xirsys.net
}

func NewXirsysProvider(ident, secretKey, channel string) *XirsysProvider {
	return &XirsysProvider{
		ident:     ident,
		secretKey: secretKey,
		channel:   channel,
		client:    &http.Client{Timeout: defaultHTTPTimeout},
		baseURL:   "https://global.xirsys.net",
	}
}

// SetBaseURL overrides the Xirsys base URL. Used in tests only.
func (p *XirsysProvider) SetBaseURL(u string) { p.baseURL = u }

func (p *XirsysProvider) Name() string { return "xirsys" }

// xirsysResponse is the outer wrapper returned by Xirsys.
type xirsysResponse struct {
	V *xirsysValue `json:"v"`
	S string       `json:"s"`
	E string       `json:"e,omitempty"` // error message when s != "ok"
}

type xirsysValue struct {
	ICEServers xirsysICEServers `json:"iceServers"`
}

type xirsysICEServers struct {
	Username   string   `json:"username"`
	Credential string   `json:"credential"`
	URLs       []string `json:"urls"`
}

func (p *XirsysProvider) GetICEServers(ctx context.Context, _ int) ([]ICEServer, error) {
	endpoint := fmt.Sprintf("%s/_turn/%s", p.baseURL, p.channel)

	reqBody, err := json.Marshal(map[string]string{"format": "urls"})
	if err != nil {
		return nil, fmt.Errorf("xirsys ice: marshal request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("xirsys ice: build request: %w", err)
	}
	req.SetBasicAuth(p.ident, p.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("xirsys ice: http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("xirsys ice: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d: %s", errXirsysServerError, resp.StatusCode, string(raw))
	}

	var xr xirsysResponse
	if err := json.Unmarshal(raw, &xr); err != nil {
		return nil, fmt.Errorf("xirsys ice: decode response: %w", err)
	}
	if xr.S != "ok" {
		return nil, fmt.Errorf("%w: %s", errXirsysAPIError, xr.E)
	}
	if xr.V == nil || len(xr.V.ICEServers.URLs) == 0 {
		return nil, errXirsysEmptyICEServers
	}

	ice := xr.V.ICEServers
	// Xirsys returns all URLs (STUN + TURN) in one array with shared credentials.
	// Split into one entry per URL to match the spec's RTCIceServer shape.
	servers := make([]ICEServer, 0, len(ice.URLs))
	for _, u := range ice.URLs {
		s := ICEServer{URLs: []string{u}}
		// Only TURN/TURNS entries need credentials.
		if len(u) > 4 && (u[:4] == "turn" || u[:5] == "turns") {
			s.Username = ice.Username
			s.Credential = ice.Credential
		}
		servers = append(servers, s)
	}
	return servers, nil
}
