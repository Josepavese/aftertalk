package webrtc

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // TURN RFC 5766 mandates HMAC-SHA1
	"encoding/base64"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Josepavese/aftertalk/internal/config"
	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	logging.Init("info", "console") //nolint:errcheck
}

// freePort finds a free UDP port on localhost.
func freePort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	require.True(t, ok, "expected *net.UDPAddr")
	port := udpAddr.Port
	conn.Close()
	return port
}

// ── GenerateCredentials ────────────────────────────────────────────────────

func TestTURNServer_GenerateCredentials_Format(t *testing.T) {
	ts := &TURNServer{secret: "test-secret"}
	username, credential := ts.GenerateCredentials("alice", 3600)

	// username must be "<expiry_unix>:<label>"
	parts := strings.SplitN(username, ":", 2)
	require.Len(t, parts, 2, "username must contain one ':'")
	assert.Equal(t, "alice", parts[1])

	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	require.NoError(t, err)
	assert.Greater(t, expiry, time.Now().Unix(), "expiry must be in the future")
	assert.Less(t, expiry, time.Now().Unix()+3700, "expiry should not exceed ttl by more than a few seconds")

	// credential must be valid base64
	_, err = base64.StdEncoding.DecodeString(credential)
	assert.NoError(t, err, "credential must be valid base64")
}

func TestTURNServer_GenerateCredentials_HMAC(t *testing.T) {
	secret := "my-shared-secret"
	ts := &TURNServer{secret: secret}
	username, credential := ts.GenerateCredentials("bob", 7200)

	// Manually recompute the HMAC and verify it matches.
	mac := hmac.New(sha1.New, []byte(secret)) //nolint:gosec
	mac.Write([]byte(username))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	assert.Equal(t, expected, credential, "credential must be HMAC-SHA1(secret, username)")
}

func TestTURNServer_GenerateCredentials_DefaultTTL(t *testing.T) {
	ts := &TURNServer{secret: "s"}
	username, _ := ts.GenerateCredentials("x", 0) // ttl=0 → default 86400

	parts := strings.SplitN(username, ":", 2)
	expiry, _ := strconv.ParseInt(parts[0], 10, 64)
	delta := expiry - time.Now().Unix()
	assert.Greater(t, delta, int64(86390), "default TTL should be ~86400s")
}

func TestTURNServer_GenerateCredentials_Uniqueness(t *testing.T) {
	ts := &TURNServer{secret: "s"}
	_, c1 := ts.GenerateCredentials("u", 3600)
	time.Sleep(2 * time.Second) // ensure different timestamp
	_, c2 := ts.GenerateCredentials("u", 3600)
	// Different timestamps → different credentials
	assert.NotEqual(t, c1, c2)
}

// ── hmacAuthHandler ────────────────────────────────────────────────────────

func TestHmacAuthHandler_ValidCredential(t *testing.T) {
	secret := "auth-secret"
	ts := &TURNServer{secret: secret}
	username, _ := ts.GenerateCredentials("user1", 3600)

	handler := hmacAuthHandler(secret)
	key, ok := handler(username, "aftertalk", &net.UDPAddr{})
	assert.True(t, ok)
	assert.NotEmpty(t, key)
}

func TestHmacAuthHandler_InvalidUsername(t *testing.T) {
	handler := hmacAuthHandler("secret")
	// Username without ":" separator
	_, ok := handler("badusername", "realm", &net.UDPAddr{})
	assert.False(t, ok)
}

func TestHmacAuthHandler_KeyIsRawHMAC(t *testing.T) {
	secret := "raw-secret"
	ts := &TURNServer{secret: secret}
	username, _ := ts.GenerateCredentials("u", 3600)

	handler := hmacAuthHandler(secret)
	key, ok := handler(username, "realm", &net.UDPAddr{})
	require.True(t, ok)

	// pion/turn expects raw bytes, not base64
	mac := hmac.New(sha1.New, []byte(secret)) //nolint:gosec
	mac.Write([]byte(username))
	assert.Equal(t, mac.Sum(nil), key)
}

// ── detectPublicIP ─────────────────────────────────────────────────────────

func TestDetectPublicIP(t *testing.T) {
	ip, err := detectPublicIP()
	require.NoError(t, err)
	parsed := net.ParseIP(ip)
	require.NotNil(t, parsed, "detectPublicIP must return a valid IP")
	assert.False(t, parsed.IsUnspecified(), "IP should not be 0.0.0.0")
}

// ── randomSecret ──────────────────────────────────────────────────────────

func TestRandomSecret_Length(t *testing.T) {
	s := randomSecret(32)
	// base64url of 32 bytes = ceil(32*4/3) = 43 chars (no padding)
	assert.GreaterOrEqual(t, len(s), 40)
}

func TestRandomSecret_Unique(t *testing.T) {
	a := randomSecret(16)
	b := randomSecret(16)
	assert.NotEqual(t, a, b)
}

// ── StartTURNServer (UDP only, loopback) ───────────────────────────────────

func TestStartTURNServer_UDP(t *testing.T) {
	port := freePort(t)
	cfg := config.TURNServerConfig{
		ListenAddr: fmt.Sprintf("127.0.0.1:%d", port),
		PublicIP:   "127.0.0.1",
		Realm:      "test",
		AuthSecret: "integration-secret",
		AuthTTL:    60,
		EnableUDP:  true,
		EnableTCP:  false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ts, err := StartTURNServer(ctx, cfg)
	require.NoError(t, err, "StartTURNServer must not fail")
	require.NotNil(t, ts)

	assert.Equal(t, fmt.Sprintf("127.0.0.1:%d", port), ts.Addr())

	// Credentials must be consistent with the secret.
	username, credential := ts.GenerateCredentials("tester", 60)
	assert.NotEmpty(t, username)
	assert.NotEmpty(t, credential)

	// Server shuts down cleanly when ctx is cancelled.
	cancel()
	time.Sleep(200 * time.Millisecond) // give goroutine time to close
}

func TestStartTURNServer_TCP(t *testing.T) {
	// Find a free TCP port.
	l, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	require.NoError(t, err)
	tcpAddr, ok := l.Addr().(*net.TCPAddr)
	require.True(t, ok, "expected *net.TCPAddr")
	port := tcpAddr.Port
	l.Close()

	cfg := config.TURNServerConfig{
		ListenAddr: fmt.Sprintf("127.0.0.1:%d", port),
		PublicIP:   "127.0.0.1",
		Realm:      "test",
		AuthSecret: "tcp-secret",
		AuthTTL:    60,
		EnableUDP:  false,
		EnableTCP:  true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ts, err := StartTURNServer(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, ts)

	cancel()
	time.Sleep(200 * time.Millisecond)
}

func TestStartTURNServer_BothDisabled(t *testing.T) {
	cfg := config.TURNServerConfig{
		ListenAddr: "127.0.0.1:3478",
		PublicIP:   "127.0.0.1",
		EnableUDP:  false,
		EnableTCP:  false,
	}
	_, err := StartTURNServer(context.Background(), cfg)
	assert.Error(t, err, "must fail when both UDP and TCP are disabled")
}

func TestStartTURNServer_EphemeralSecret(t *testing.T) {
	port := freePort(t)
	cfg := config.TURNServerConfig{
		ListenAddr: fmt.Sprintf("127.0.0.1:%d", port),
		PublicIP:   "127.0.0.1",
		AuthSecret: "", // empty → ephemeral
		EnableUDP:  true,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ts, err := StartTURNServer(ctx, cfg)
	require.NoError(t, err)
	// Internal secret must have been generated.
	assert.NotEmpty(t, ts.secret)
}
