package steps

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
)

// testLogger captures log lines for assertion in tests.
type testLogger struct {
	infos []string
	warns []string
}

func (l *testLogger) Info(msg string)  { l.infos = append(l.infos, msg) }
func (l *testLogger) Warn(msg string)  { l.warns = append(l.warns, msg) }
func (l *testLogger) Error(msg string) { l.warns = append(l.warns, "ERR:"+msg) }

func serverPort(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	return port
}

func TestWaitAftertalkHealthy_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := instconfig.Default()
	cfg.HTTPPort = serverPort(t, srv)

	log := &testLogger{}
	err := waitAftertalkHealthy(context.Background(), cfg, log)
	require.NoError(t, err)
	assert.Contains(t, log.infos[len(log.infos)-1], "healthy")
}

func TestWaitAftertalkHealthy_logsRuntimeBuildIdentity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","version":"1.0.0","commit":"abc123","tag":"edge","build_source":"github-actions"}`))
	}))
	defer srv.Close()

	cfg := instconfig.Default()
	cfg.HTTPPort = serverPort(t, srv)

	log := &testLogger{}
	err := waitAftertalkHealthy(context.Background(), cfg, log)
	require.NoError(t, err)
	assert.Contains(t, log.infos[len(log.infos)-1], "version=1.0.0")
	assert.Contains(t, log.infos[len(log.infos)-1], "tag=edge")
	assert.Contains(t, log.infos[len(log.infos)-1], "commit=abc123")
}

func TestWaitAftertalkHealthy_failsOnRuntimeBuildMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","version":"1.0.0","commit":"abc123","tag":"edge","build_source":"github-actions"}`))
	}))
	defer srv.Close()

	cfg := instconfig.Default()
	cfg.HTTPPort = serverPort(t, srv)
	cfg.ExpectedCommit = "different"

	log := &testLogger{}
	err := waitAftertalkHealthy(context.Background(), cfg, log)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "runtime commit mismatch")
}

func TestWaitAftertalkHealthy_cancelledContext(t *testing.T) {
	// Server returns 503 so retries would run; cancel context to abort quickly.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := instconfig.Default()
	cfg.HTTPPort = serverPort(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before first attempt triggers the select path

	log := &testLogger{}
	err := waitAftertalkHealthy(ctx, cfg, log)
	assert.Error(t, err)
}

func TestCheckEndpoint_reachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := checkEndpoint(context.Background(), srv.URL+"/v1/models", 2_000_000_000)
	assert.NoError(t, err)
}

func TestCheckEndpoint_unreachable(t *testing.T) {
	err := checkEndpoint(context.Background(), "http://127.0.0.1:19999/api/tags", 500_000_000)
	assert.Error(t, err)
}

func TestCheckEndpoint_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := checkEndpoint(context.Background(), srv.URL, 2_000_000_000)
	assert.Error(t, err, "HTTP 5xx must be treated as error")
}

func TestCheckDependencies_whisperWarn(t *testing.T) {
	cfg := instconfig.Default()
	cfg.STTProvider = "whisper-local"
	cfg.WhisperURL = "http://127.0.0.1:19998" // nothing listening
	cfg.LLMProvider = "openai"                // skip ollama check

	log := &testLogger{}
	checkDependencies(context.Background(), cfg, log)
	require.Len(t, log.warns, 1)
	assert.Contains(t, log.warns[0], "whisper-local")
	assert.Contains(t, log.warns[0], "aftertalk-whisper.service")
}

func TestCheckDependencies_ollamaWarn(t *testing.T) {
	cfg := instconfig.Default()
	cfg.STTProvider = "google" // skip whisper check
	cfg.LLMProvider = "ollama"
	cfg.OllamaURL = "http://127.0.0.1:19997" // nothing listening

	log := &testLogger{}
	checkDependencies(context.Background(), cfg, log)
	require.Len(t, log.warns, 1)
	assert.Contains(t, log.warns[0], "ollama")
	assert.Contains(t, log.warns[0], "ollama.service")
}

func TestCheckDependencies_allHealthy(t *testing.T) {
	whisperSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer whisperSrv.Close()
	ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ollamaSrv.Close()

	cfg := instconfig.Default()
	cfg.STTProvider = "whisper-local"
	cfg.WhisperURL = whisperSrv.URL
	cfg.LLMProvider = "ollama"
	cfg.OllamaURL = ollamaSrv.URL

	log := &testLogger{}
	checkDependencies(context.Background(), cfg, log)
	assert.Empty(t, log.warns)
	assert.Len(t, log.infos, 2) // whisper ✓ + ollama ✓
}

func TestCheckDependencies_nonWhisperNonOllama(t *testing.T) {
	cfg := instconfig.Default()
	cfg.STTProvider = "google"
	cfg.LLMProvider = "openai"

	log := &testLogger{}
	checkDependencies(context.Background(), cfg, log)
	assert.Empty(t, log.warns)
	assert.Empty(t, log.infos)
}
