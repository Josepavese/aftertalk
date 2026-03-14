package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	webrtcpkg "github.com/Josepavese/aftertalk/internal/bot/webrtc"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// freeTURNPort finds a free UDP port on loopback.
func freeTURNPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()
	return port
}

func baseCfg() *config.Config {
	return &config.Config{
		WebRTC: config.WebRTCConfig{
			ICEServers: []config.ICEServerConfig{
				{URLs: []string{"stun:stun.l.google.com:19302"}},
			},
			TURN: config.TURNServerConfig{AuthTTL: 3600},
		},
	}
}

// ── Static provider ───────────────────────────────────────────────────────

func TestRTCConfigHandler_StaticProvider(t *testing.T) {
	cfg := baseCfg()
	provider := webrtcpkg.NewStaticProvider(cfg.WebRTC.ICEServers)
	h := NewRTCConfigHandler(cfg, provider)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/rtc-config", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var resp struct {
		ICEServers []struct {
			URLs       []string `json:"urls"`
			Username   string   `json:"username,omitempty"`
			Credential string   `json:"credential,omitempty"`
		} `json:"ice_servers"`
		TTL      int    `json:"ttl"`
		Provider string `json:"provider"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, 3600, resp.TTL)
	assert.Equal(t, "static", resp.Provider)
	require.Len(t, resp.ICEServers, 1)
	assert.Equal(t, []string{"stun:stun.l.google.com:19302"}, resp.ICEServers[0].URLs)
	assert.Empty(t, resp.ICEServers[0].Username)
}

func TestRTCConfigHandler_EmptyICEServers(t *testing.T) {
	cfg := &config.Config{
		WebRTC: config.WebRTCConfig{
			TURN: config.TURNServerConfig{AuthTTL: 86400},
		},
	}
	h := NewRTCConfigHandler(cfg, webrtcpkg.NewStaticProvider(nil))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/rtc-config", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	servers := resp["ice_servers"].([]interface{})
	assert.Empty(t, servers)
}

func TestRTCConfigHandler_DefaultTTL(t *testing.T) {
	cfg := &config.Config{
		WebRTC: config.WebRTCConfig{
			TURN: config.TURNServerConfig{AuthTTL: 0},
		},
	}
	h := NewRTCConfigHandler(cfg, webrtcpkg.NewStaticProvider(nil))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/rtc-config", nil))

	var resp struct{ TTL int `json:"ttl"` }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, 86400, resp.TTL)
}

// ── Embedded (pion/turn) provider ─────────────────────────────────────────

func startTestTURN(t *testing.T) (*webrtcpkg.TURNServer, int, context.CancelFunc) {
	t.Helper()
	port := freeTURNPort(t)
	cfg := config.TURNServerConfig{
		ListenAddr: fmt.Sprintf("127.0.0.1:%d", port),
		PublicIP:   "127.0.0.1",
		Realm:      "test",
		AuthSecret: "test-secret-1234",
		AuthTTL:    3600,
		EnableUDP:  true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	ts, err := webrtcpkg.StartTURNServer(ctx, cfg)
	require.NoError(t, err)
	return ts, port, cancel
}

func TestRTCConfigHandler_EmbeddedProvider_HasCredentials(t *testing.T) {
	ts, port, cancel := startTestTURN(t)
	defer cancel()

	cfg := baseCfg()
	provider := webrtcpkg.NewEmbeddedProvider(ts, cfg.WebRTC.ICEServers)
	h := NewRTCConfigHandler(cfg, provider)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/rtc-config", nil))

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		ICEServers []struct {
			URLs       []string `json:"urls"`
			Username   string   `json:"username,omitempty"`
			Credential string   `json:"credential,omitempty"`
		} `json:"ice_servers"`
		Provider string `json:"provider"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "embedded", resp.Provider)
	assert.GreaterOrEqual(t, len(resp.ICEServers), 2)

	// Last entry is the TURN entry
	turnEntry := resp.ICEServers[len(resp.ICEServers)-1]
	assert.NotEmpty(t, turnEntry.Username)
	assert.NotEmpty(t, turnEntry.Credential)
	assert.True(t, strings.HasPrefix(turnEntry.URLs[0], "turn:"))
	assert.Contains(t, turnEntry.URLs[0], fmt.Sprintf("127.0.0.1:%d", port))
}

func TestRTCConfigHandler_EmbeddedProvider_CredentialsFresh(t *testing.T) {
	ts, _, cancel := startTestTURN(t)
	defer cancel()

	provider := webrtcpkg.NewEmbeddedProvider(ts, nil)
	h := NewRTCConfigHandler(baseCfg(), provider)

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/v1/rtc-config", nil))
	time.Sleep(1100 * time.Millisecond)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/v1/rtc-config", nil))

	var r1, r2 struct {
		ICEServers []struct {
			Username string `json:"username,omitempty"`
		} `json:"ice_servers"`
	}
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &r1))
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &r2))

	require.GreaterOrEqual(t, len(r1.ICEServers), 1)
	require.GreaterOrEqual(t, len(r2.ICEServers), 1)

	u1 := r1.ICEServers[len(r1.ICEServers)-1].Username
	u2 := r2.ICEServers[len(r2.ICEServers)-1].Username
	assert.NotEqual(t, u1, u2, "consecutive requests must produce different usernames")
}

// ── Twilio mock provider ───────────────────────────────────────────────────

func TestRTCConfigHandler_TwilioProvider_Mock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "AC123", user)
		assert.Equal(t, "token123", pass)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"username": "twilio-user",
			"password": "twilio-pass",
			"ttl":      "3600",
			"ice_servers": []map[string]interface{}{
				{"url": "stun:stun.twilio.com:3478"},
				{"url": "turn:turn.twilio.com:3478?transport=udp", "username": "twilio-user", "credential": "twilio-pass"},
				{"url": "turn:turn.twilio.com:443?transport=tcp", "username": "twilio-user", "credential": "twilio-pass"},
			},
		})
	}))
	defer srv.Close()

	p := webrtcpkg.NewTwilioProvider("AC123", "token123")
	p.SetEndpoint(srv.URL)

	h := NewRTCConfigHandler(baseCfg(), p)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/rtc-config", nil))

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		ICEServers []struct {
			URLs       []string `json:"urls"`
			Username   string   `json:"username,omitempty"`
			Credential string   `json:"credential,omitempty"`
		} `json:"ice_servers"`
		Provider string `json:"provider"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "twilio", resp.Provider)
	assert.Len(t, resp.ICEServers, 3)

	// First entry is STUN (no credentials).
	assert.Contains(t, resp.ICEServers[0].URLs[0], "stun:")
	assert.Empty(t, resp.ICEServers[0].Username)

	// Second entry is TURN (has credentials).
	assert.Contains(t, resp.ICEServers[1].URLs[0], "turn:")
	assert.Equal(t, "twilio-user", resp.ICEServers[1].Username)
	assert.Equal(t, "twilio-pass", resp.ICEServers[1].Credential)
}

func TestRTCConfigHandler_TwilioProvider_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := webrtcpkg.NewTwilioProvider("bad", "creds")
	p.SetEndpoint(srv.URL)

	h := NewRTCConfigHandler(baseCfg(), p)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/rtc-config", nil))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// ── Xirsys mock provider ───────────────────────────────────────────────────

func TestRTCConfigHandler_XirsysProvider_Mock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)
		assert.Contains(t, r.URL.Path, "mychannel")
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "myident", user)
		assert.Equal(t, "mysecret", pass)

		json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
			"s": "ok",
			"v": map[string]interface{}{
				"iceServers": map[string]interface{}{
					"username":   "xirsys-user",
					"credential": "xirsys-cred",
					"urls": []string{
						"stun:turn1.xirsys.com",
						"turn:turn1.xirsys.com:3478?transport=udp",
						"turn:turn1.xirsys.com:443?transport=tcp",
					},
				},
			},
		})
	}))
	defer srv.Close()

	p := webrtcpkg.NewXirsysProvider("myident", "mysecret", "mychannel")
	p.SetBaseURL(srv.URL)

	h := NewRTCConfigHandler(baseCfg(), p)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/rtc-config", nil))

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		ICEServers []struct {
			URLs       []string `json:"urls"`
			Username   string   `json:"username,omitempty"`
			Credential string   `json:"credential,omitempty"`
		} `json:"ice_servers"`
		Provider string `json:"provider"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "xirsys", resp.Provider)
	assert.Len(t, resp.ICEServers, 3)

	// STUN entry has no credentials.
	assert.Empty(t, resp.ICEServers[0].Username)

	// TURN entries have credentials.
	assert.Equal(t, "xirsys-user", resp.ICEServers[1].Username)
	assert.Equal(t, "xirsys-cred", resp.ICEServers[1].Credential)
}

// ── Metered mock provider ─────────────────────────────────────────────────

func TestRTCConfigHandler_MeteredProvider_Mock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Contains(t, r.URL.RawQuery, "apiKey=mykey")

		json.NewEncoder(w).Encode([]map[string]interface{}{ //nolint:errcheck
			{"urls": "stun:stun.metered.ca:80"},
			{"urls": "turn:relay.metered.ca:80", "username": "metered-user", "credential": "metered-cred"},
			{"urls": "turn:relay.metered.ca:443?transport=tcp", "username": "metered-user", "credential": "metered-cred"},
			{"urls": "turns:relay.metered.ca:443?transport=tcp", "username": "metered-user", "credential": "metered-cred"},
		})
	}))
	defer srv.Close()

	p := webrtcpkg.NewMeteredProvider("myapp", "mykey")
	p.SetBaseURL(srv.URL)

	h := NewRTCConfigHandler(baseCfg(), p)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/rtc-config", nil))

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		ICEServers []struct {
			URLs       []string `json:"urls"`
			Username   string   `json:"username,omitempty"`
			Credential string   `json:"credential,omitempty"`
		} `json:"ice_servers"`
		Provider string `json:"provider"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))

	assert.Equal(t, "metered", resp.Provider)
	assert.Len(t, resp.ICEServers, 4)
	assert.Empty(t, resp.ICEServers[0].Username)
	assert.Equal(t, "metered-user", resp.ICEServers[1].Username)
}

// ── Factory routing (NewICEProvider) ─────────────────────────────────────

func TestICEProviderFactory_Static(t *testing.T) {
	cfg := &config.WebRTCConfig{
		ICEProviderName: "",
		ICEServers:      []config.ICEServerConfig{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	}
	p, err := webrtcpkg.NewICEProvider(cfg, nil)
	require.NoError(t, err)
	assert.Equal(t, "static", p.Name())
}

func TestICEProviderFactory_Embedded_NoServer_Fails(t *testing.T) {
	cfg := &config.WebRTCConfig{ICEProviderName: "embedded"}
	_, err := webrtcpkg.NewICEProvider(cfg, nil)
	assert.Error(t, err)
}

func TestICEProviderFactory_Twilio_MissingCreds_Fails(t *testing.T) {
	cfg := &config.WebRTCConfig{ICEProviderName: "twilio"}
	_, err := webrtcpkg.NewICEProvider(cfg, nil)
	assert.Error(t, err)
}

func TestICEProviderFactory_Xirsys_MissingCreds_Fails(t *testing.T) {
	cfg := &config.WebRTCConfig{ICEProviderName: "xirsys"}
	_, err := webrtcpkg.NewICEProvider(cfg, nil)
	assert.Error(t, err)
}

func TestICEProviderFactory_Metered_MissingCreds_Fails(t *testing.T) {
	cfg := &config.WebRTCConfig{ICEProviderName: "metered"}
	_, err := webrtcpkg.NewICEProvider(cfg, nil)
	assert.Error(t, err)
}

func TestICEProviderFactory_UnknownProvider_Fails(t *testing.T) {
	cfg := &config.WebRTCConfig{ICEProviderName: "nonexistent"}
	_, err := webrtcpkg.NewICEProvider(cfg, nil)
	assert.Error(t, err)
}

func TestICEProviderFactory_Twilio_WithCreds_Succeeds(t *testing.T) {
	cfg := &config.WebRTCConfig{
		ICEProviderName: "twilio",
		Twilio:          config.TwilioICEConfig{AccountSID: "AC123456789", AuthToken: "auth-token"},
	}
	p, err := webrtcpkg.NewICEProvider(cfg, nil)
	require.NoError(t, err)
	assert.Equal(t, "twilio", p.Name())
}

// ── Via HTTP server ───────────────────────────────────────────────────────

func TestRTCConfigHandler_ViaHTTPServer(t *testing.T) {
	provider := webrtcpkg.NewStaticProvider([]config.ICEServerConfig{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
	})
	h := NewRTCConfigHandler(baseCfg(), provider)
	srv := httptest.NewServer(http.HandlerFunc(h.ServeHTTP))
	defer srv.Close()

	resp, err := http.Get(srv.URL) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Contains(t, body, "ice_servers")
	assert.Contains(t, body, "ttl")
	assert.Contains(t, body, "provider")
}
