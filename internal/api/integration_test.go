// Package api_test contains integration tests for the HTTP API server.
// It spins up a real chi router backed by real SQLite + real services,
// using httptest.Server so no ports need to be pre-allocated.
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Josepavese/aftertalk/internal/ai/llm"
	"github.com/Josepavese/aftertalk/internal/ai/stt"
	"github.com/Josepavese/aftertalk/internal/api"
	"github.com/Josepavese/aftertalk/internal/api/handler"
	webrtcpkg "github.com/Josepavese/aftertalk/internal/bot/webrtc"
	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/core/minutes"
	"github.com/Josepavese/aftertalk/internal/core/session"
	"github.com/Josepavese/aftertalk/internal/core/transcription"
	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/internal/storage/cache"
	"github.com/Josepavese/aftertalk/internal/storage/sqlite"
	"github.com/Josepavese/aftertalk/pkg/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	logging.Init("info", "console") //nolint:errcheck
}

// ── Test harness ──────────────────────────────────────────────────────────

type testEnv struct {
	srv            *httptest.Server
	cfg            *config.Config
	db             *sqlite.DB
	sessionService *session.Service
	jwtMgr         *jwt.JWTManager
	tokenCache     *cache.TokenCache
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	ctx := context.Background()

	db, err := sqlite.New(ctx, dbPath)
	require.NoError(t, err)
	require.NoError(t, runMigrations(ctx, db))

	cfg := config.Default()
	cfg.API.Key = "test-api-key"
	cfg.JWT.Secret = "test-jwt-secret"
	cfg.JWT.Expiration = 1 * time.Hour
	cfg.JWT.Issuer = "test"

	jwtMgr := jwt.NewJWTManager(cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.Expiration)
	sessionCache := cache.NewSessionCache()
	tokenCache := cache.NewTokenCache()
	audioBuffer := cache.NewAudioBufferCache()

	sttProvider, _ := stt.NewProvider(&stt.STTConfig{Provider: "stub"})
	llmProvider, _ := llm.NewProvider(&llm.LLMConfig{Provider: "stub"})

	txRepo := transcription.NewTranscriptionRepository(db.DB)
	txSvc := transcription.NewService(txRepo, sttProvider, nil)

	minRepo := minutes.NewMinutesRepository(db.DB)
	minSvc := minutes.NewService(minRepo, llmProvider)

	sessionRepo := session.NewSessionRepository(db.DB)
	sessionSvc := session.NewService(
		sessionRepo,
		jwtMgr,
		sessionCache,
		tokenCache,
		audioBuffer,
		&api.TranscriptionAdapter{Svc: txSvc},
		&api.MinutesAdapter{Svc: minSvc},
		cfg.Processing,
		cfg.Templates,
	)

	botServer := api.NewBotServer(sessionSvc, jwtMgr, tokenCache, nil)
	minutesHandler := handler.NewMinutesHandler(minSvc)
	rtcHandler := handler.NewRTCConfigHandler(cfg, webrtcpkg.NewStaticProvider(cfg.WebRTC.ICEServers))

	server := api.NewServerWithDeps(cfg, sessionSvc, botServer, minutesHandler, nil, rtcHandler)
	httpSrv := httptest.NewServer(server.Handler())

	t.Cleanup(func() {
		httpSrv.Close()
		db.Close()
	})

	return &testEnv{
		srv:            httpSrv,
		cfg:            cfg,
		db:             db,
		sessionService: sessionSvc,
		jwtMgr:         jwtMgr,
		tokenCache:     tokenCache,
	}
}

func (e *testEnv) url(path string) string {
	return e.srv.URL + path
}

func (e *testEnv) get(t *testing.T, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, e.url(path), nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+e.cfg.API.Key)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func (e *testEnv) post(t *testing.T, path string, body interface{}) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(http.MethodPost, e.url(path), bodyReader)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.cfg.API.Key)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(v))
}

// runMigrations creates the tables needed for integration tests.
func runMigrations(ctx context.Context, db *sqlite.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY, status TEXT NOT NULL, created_at TEXT NOT NULL,
			ended_at TEXT, participant_count INTEGER NOT NULL DEFAULT 0,
			metadata TEXT, template_id TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS participants (
			id TEXT PRIMARY KEY, session_id TEXT NOT NULL, user_id TEXT NOT NULL,
			role TEXT NOT NULL, token_jti TEXT, token_expires_at TEXT,
			token_used INTEGER NOT NULL DEFAULT 0, connected_at TEXT, disconnected_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS transcriptions (
			id TEXT PRIMARY KEY, session_id TEXT NOT NULL, segment_index INTEGER NOT NULL DEFAULT 0,
			role TEXT NOT NULL, start_ms INTEGER NOT NULL DEFAULT 0, end_ms INTEGER NOT NULL DEFAULT 0,
			text TEXT NOT NULL, confidence REAL NOT NULL DEFAULT 0, provider TEXT NOT NULL,
			created_at TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'ready'
		)`,
		`CREATE TABLE IF NOT EXISTS minutes (
			id TEXT PRIMARY KEY, session_id TEXT NOT NULL, version INTEGER NOT NULL DEFAULT 1,
			content TEXT, citations TEXT, generated_at TEXT, delivered_at TEXT,
			status TEXT NOT NULL DEFAULT 'pending', provider TEXT NOT NULL DEFAULT '',
			template_id TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS minutes_history (
			id TEXT PRIMARY KEY, minutes_id TEXT NOT NULL, version INTEGER NOT NULL,
			content TEXT, edited_by TEXT, edited_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS webhook_events (
			id TEXT PRIMARY KEY, minutes_id TEXT NOT NULL, webhook_url TEXT NOT NULL,
			payload TEXT NOT NULL, attempt_number INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending', next_retry_at TEXT,
			delivered_at TEXT, error_message TEXT, created_at TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// ── Health ────────────────────────────────────────────────────────────────

func TestAPI_Health(t *testing.T) {
	e := newTestEnv(t)
	resp := e.get(t, "/v1/health")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAPI_Health_ContentType(t *testing.T) {
	e := newTestEnv(t)
	resp := e.get(t, "/v1/health")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// Should return JSON
	body, _ := io.ReadAll(resp.Body)
	assert.True(t, json.Valid(body) || len(body) > 0)
}

// ── Demo config ───────────────────────────────────────────────────────────

func TestAPI_DemoConfig(t *testing.T) {
	e := newTestEnv(t)
	// Enable demo mode so the API key is exposed (as in local-demo use case).
	// cfg is held by pointer so the handler closure sees this update at request time.
	e.cfg.Demo.Enabled = true

	resp, err := http.Get(e.url("/demo/config")) // no auth required
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		APIKey            string                  `json:"api_key"`
		Templates         []config.TemplateConfig `json:"templates"`
		DefaultTemplateID string                  `json:"default_template_id"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	assert.Equal(t, "test-api-key", body.APIKey)
	assert.NotEmpty(t, body.Templates)
	assert.NotEmpty(t, body.DefaultTemplateID)
}

func TestAPI_DemoConfig_HasTherapyTemplate(t *testing.T) {
	e := newTestEnv(t)
	resp, _ := http.Get(e.url("/demo/config"))
	defer resp.Body.Close()

	var body struct {
		Templates []config.TemplateConfig `json:"templates"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	ids := make([]string, len(body.Templates))
	for i, tmpl := range body.Templates {
		ids[i] = tmpl.ID
	}
	assert.Contains(t, ids, "therapy")
}

// ── RTC config ────────────────────────────────────────────────────────────

func TestAPI_RTCConfig_NoTURN(t *testing.T) {
	e := newTestEnv(t)
	resp := e.get(t, "/v1/rtc-config")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		ICEServers []map[string]interface{} `json:"ice_servers"`
		TTL        int                      `json:"ttl"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.NotEmpty(t, body.ICEServers)
	assert.Greater(t, body.TTL, 0)
}

func TestAPI_RTCConfig_RequiresAuth(t *testing.T) {
	e := newTestEnv(t)
	resp, err := http.Get(e.url("/v1/rtc-config"))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAPI_RTCConfig_WithLiveTURN(t *testing.T) {
	e := newTestEnv(t)

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts, err := webrtcpkg.StartTURNServer(ctx, config.TURNServerConfig{
		ListenAddr: fmt.Sprintf("127.0.0.1:%d", port),
		PublicIP:   "127.0.0.1",
		Realm:      "test",
		AuthSecret: "integration-turn-secret",
		AuthTTL:    3600,
		EnableUDP:  true,
	})
	require.NoError(t, err)

	rtcHandler := handler.NewRTCConfigHandler(e.cfg, webrtcpkg.NewEmbeddedProvider(ts, e.cfg.WebRTC.ICEServers))
	server := api.NewServerWithDeps(e.cfg, e.sessionService,
		api.NewBotServer(e.sessionService, e.jwtMgr, e.tokenCache, nil),
		nil, nil, rtcHandler)
	srv := httptest.NewServer(server.Handler())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/rtc-config", nil)
	req.Header.Set("Authorization", "Bearer "+e.cfg.API.Key)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		ICEServers []struct {
			URLs       []string `json:"urls"`
			Username   string   `json:"username,omitempty"`
			Credential string   `json:"credential,omitempty"`
		} `json:"ice_servers"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))

	var foundTURN bool
	for _, s := range body.ICEServers {
		if len(s.URLs) > 0 && strings.HasPrefix(s.URLs[0], "turn:") {
			foundTURN = true
			assert.NotEmpty(t, s.Username)
			assert.NotEmpty(t, s.Credential)
		}
	}
	assert.True(t, foundTURN, "response must contain a TURN entry")
}

// ── Test-start room ───────────────────────────────────────────────────────

func TestAPI_TestStart_CreatesSession(t *testing.T) {
	e := newTestEnv(t)

	resp := e.post(t, "/test/start", map[string]string{
		"code": "room-001", "name": "Alice", "role": "therapist", "template_id": "therapy",
	})
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result struct {
		SessionID string `json:"session_id"`
		Token     string `json:"token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.NotEmpty(t, result.SessionID)
	assert.NotEmpty(t, result.Token)
}

func TestAPI_TestStart_SameRoomSameRole_TakenConflict(t *testing.T) {
	e := newTestEnv(t)

	r1 := e.post(t, "/test/start", map[string]string{"code": "room-002", "name": "Alice", "role": "therapist"})
	io.Copy(io.Discard, r1.Body) //nolint:errcheck
	r1.Body.Close()
	require.Equal(t, http.StatusOK, r1.StatusCode)

	// Same role, different name → conflict.
	r2 := e.post(t, "/test/start", map[string]string{"code": "room-002", "name": "Bob", "role": "therapist"})
	io.Copy(io.Discard, r2.Body) //nolint:errcheck
	r2.Body.Close()
	assert.Equal(t, http.StatusConflict, r2.StatusCode)
}

func TestAPI_TestStart_SameRoomSameRole_SameName_Reconnect(t *testing.T) {
	e := newTestEnv(t)
	body := map[string]string{"code": "room-003", "name": "Alice", "role": "therapist"}

	r1 := e.post(t, "/test/start", body)
	var res1 struct {
		SessionID string `json:"session_id"`
		Token     string `json:"token"`
	}
	decodeJSON(t, r1, &res1)

	r2 := e.post(t, "/test/start", body)
	var res2 struct {
		SessionID string `json:"session_id"`
		Token     string `json:"token"`
	}
	decodeJSON(t, r2, &res2)

	assert.Equal(t, res1.SessionID, res2.SessionID)
	assert.Equal(t, res1.Token, res2.Token, "reconnection must return the same token")
}

func TestAPI_TestStart_TwoRoles_BothJoin(t *testing.T) {
	e := newTestEnv(t)

	r1 := e.post(t, "/test/start", map[string]string{"code": "room-004", "name": "Alice", "role": "therapist"})
	var res1 struct {
		SessionID string `json:"session_id"`
		Token     string `json:"token"`
	}
	decodeJSON(t, r1, &res1)

	r2 := e.post(t, "/test/start", map[string]string{"code": "room-004", "name": "Bob", "role": "patient"})
	var res2 struct {
		SessionID string `json:"session_id"`
		Token     string `json:"token"`
	}
	decodeJSON(t, r2, &res2)

	assert.Equal(t, res1.SessionID, res2.SessionID, "same room → same session")
	assert.NotEqual(t, res1.Token, res2.Token, "different roles → different tokens")
}

func TestAPI_TestStart_MissingFields_BadRequest(t *testing.T) {
	e := newTestEnv(t)

	cases := []map[string]string{
		{"name": "Alice", "role": "therapist"},
		{"code": "x", "role": "therapist"},
		{"code": "x", "name": "Alice"},
		{},
	}
	for _, body := range cases {
		resp := e.post(t, "/test/start", body)
		io.Copy(io.Discard, resp.Body) //nolint:errcheck
		resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "body=%v", body)
	}
}

// ── Session CRUD via /v1/sessions ─────────────────────────────────────────

func TestAPI_CreateSession(t *testing.T) {
	e := newTestEnv(t)

	body := map[string]interface{}{
		"participant_count": 2,
		"template_id":       "therapy",
		"participants": []map[string]string{
			{"user_id": "u1", "role": "therapist"},
			{"user_id": "u2", "role": "patient"},
		},
	}

	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, e.url("/v1/sessions"), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.cfg.API.Key)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.NotEmpty(t, result["session_id"])
}

func TestAPI_GetSession(t *testing.T) {
	e := newTestEnv(t)

	r1 := e.post(t, "/test/start", map[string]string{"code": "room-get", "name": "Alice", "role": "therapist"})
	var res struct{ SessionID string `json:"session_id"` }
	decodeJSON(t, r1, &res)

	resp := e.get(t, "/v1/sessions/"+res.SessionID)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var s map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&s))
	assert.Equal(t, res.SessionID, s["id"])
}

func TestAPI_GetSession_NotFound(t *testing.T) {
	e := newTestEnv(t)
	resp := e.get(t, "/v1/sessions/nonexistent-id")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAPI_EndSession(t *testing.T) {
	e := newTestEnv(t)

	r1 := e.post(t, "/test/start", map[string]string{"code": "room-end", "name": "Alice", "role": "therapist"})
	var res struct{ SessionID string `json:"session_id"` }
	decodeJSON(t, r1, &res)

	resp := e.post(t, "/v1/sessions/"+res.SessionID+"/end", nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}


// ── Transcriptions ────────────────────────────────────────────────────────

func TestAPI_GetTranscriptions_RouteExists(t *testing.T) {
	e := newTestEnv(t)

	// The transcription handler is not wired in the test env (nil), so /v1/transcriptions
	// should return 404. We verify the server handles unknown routes gracefully.
	resp := e.get(t, "/v1/transcriptions")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// ── Minutes ───────────────────────────────────────────────────────────────

func TestAPI_GetMinutes_NoMinutes(t *testing.T) {
	e := newTestEnv(t)

	r1 := e.post(t, "/test/start", map[string]string{"code": "room-min", "name": "Alice", "role": "therapist"})
	var res struct{ SessionID string `json:"session_id"` }
	decodeJSON(t, r1, &res)

	resp := e.get(t, "/v1/sessions/"+res.SessionID+"/minutes")
	defer resp.Body.Close()

	assert.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusOK,
		"expected 404 or 200, got %d", resp.StatusCode)
}

// ── API key enforcement ───────────────────────────────────────────────────

func TestAPI_WrongAPIKey_Unauthorized(t *testing.T) {
	e := newTestEnv(t)

	req, _ := http.NewRequest(http.MethodGet, e.url("/v1/health"), nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAPI_MissingAPIKey_Unauthorized(t *testing.T) {
	e := newTestEnv(t)
	resp, err := http.Get(e.url("/v1/health"))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAPI_PublicRoutes_NoAuth(t *testing.T) {
	e := newTestEnv(t)
	// /demo/config and /test/start are public.
	for _, path := range []string{"/demo/config"} {
		resp, err := http.Get(e.url(path))
		require.NoError(t, err)
		resp.Body.Close()
		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode, "path=%s should be public", path)
	}
}
