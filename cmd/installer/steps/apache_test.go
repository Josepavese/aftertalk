package steps

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
)

// dualVhost is a realistic Apache config with both port-80 redirect and port-443 SSL block.
const dualVhost = `<VirtualHost *:80>
    ServerName app.example.it
    Redirect permanent / https://app.example.it/
</VirtualHost>

<VirtualHost *:443>
    ServerName app.example.it
    SSLEngine on
    SSLCertificateFile /etc/letsencrypt/live/app.example.it/fullchain.pem
    ProxyPreserveHost On
    ProxyPass /proxy http://localhost:8080/proxy
    ProxyPassReverse /proxy http://localhost:8080/proxy
</VirtualHost>
`

func apacheCfg(t *testing.T, vhostContent, vhostPath string) *instconfig.InstallConfig {
	t.Helper()
	require.NoError(t, os.WriteFile(vhostPath, []byte(vhostContent), 0o644))
	cfg := instconfig.Default()
	cfg.ApacheVhostConf = vhostPath
	cfg.HTTPPort = 9080
	cfg.SkipApache = false
	return cfg
}

func TestApache_InjectsIntoSSLBlock(t *testing.T) {
	dir := t.TempDir()
	vhostPath := filepath.Join(dir, "ssl.conf")
	cfg := apacheCfg(t, dualVhost, vhostPath)

	log := &testLogger{}
	// runApache calls apache2ctl and systemctl which won't be available in CI.
	// We test the file-rewrite logic by calling the inner function directly via
	// a minimal integration: write the file, call runApache with a stub, check output.
	// Since we can't run apache2ctl in tests, we extract the file-write logic result directly.
	err := injectApacheProxy(cfg, log)
	require.NoError(t, err)

	result, err := os.ReadFile(vhostPath)
	require.NoError(t, err)
	content := string(result)

	// Proxy block must appear INSIDE the port-443 block, not the port-80 block.
	// Strategy: split on </VirtualHost> and check the LAST segment before it contains the injection.
	parts := strings.Split(content, "</VirtualHost>")
	// parts[0] = port-80 block content, parts[1] = port-443 block content, parts[2] = "" (after last tag)
	require.GreaterOrEqual(t, len(parts), 3, "must have at least two </VirtualHost> tags")

	port80Block := parts[0]
	sslBlock := parts[1]

	assert.NotContains(t, port80Block, "Aftertalk", "proxy must NOT be in the port-80 block")
	assert.Contains(t, sslBlock, "Aftertalk", "proxy MUST be in the SSL (port-443) block")
	assert.Contains(t, sslBlock, "9080", "proxy must use configured HTTPPort")
	assert.Contains(t, sslBlock, "/aftertalk")
}

func TestApache_Idempotent(t *testing.T) {
	dir := t.TempDir()
	vhostPath := filepath.Join(dir, "ssl.conf")
	cfg := apacheCfg(t, dualVhost, vhostPath)

	log := &testLogger{}
	require.NoError(t, injectApacheProxy(cfg, log))
	require.NoError(t, injectApacheProxy(cfg, log)) // second call must be no-op

	result, _ := os.ReadFile(vhostPath)
	// Count the opening marker only — appears once per injected block.
	count := strings.Count(string(result), installerMarker)
	assert.Equal(t, 1, count, "proxy block must appear exactly once after two calls")
	assert.Contains(t, log.infos[len(log.infos)-1], "skipping", "second call must log 'skipping'")
}

func TestApache_NoVirtualHostTag_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	vhostPath := filepath.Join(dir, "bad.conf")
	require.NoError(t, os.WriteFile(vhostPath, []byte("# empty file\n"), 0o644))

	cfg := instconfig.Default()
	cfg.ApacheVhostConf = vhostPath
	cfg.HTTPPort = 9080

	log := &testLogger{}
	err := injectApacheProxy(cfg, log)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "</VirtualHost>")
}

func TestApache_SkipApache(t *testing.T) {
	cfg := instconfig.Default()
	cfg.SkipApache = true
	cfg.ApacheVhostConf = "/nonexistent/path"

	log := &testLogger{}
	err := runApache(context.Background(), cfg, log)
	assert.NoError(t, err)
	assert.Contains(t, log.infos[0], "skipped")
}
