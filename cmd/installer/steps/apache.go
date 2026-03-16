package steps

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
)

func stepApache() *Step {
	return &Step{
		ID:          "70-apache",
		Description: "Inject Aftertalk ProxyPass into Apache SSL vhost",
		Platforms:   []string{"linux"},
		Run:         runApache,
	}
}

const apacheProxyBlock = `
    # >>> Aftertalk reverse proxy — managed by aftertalk-installer
    ProxyPreserveHost On
    ProxyPass        /aftertalk http://127.0.0.1:%d/
    ProxyPassReverse /aftertalk http://127.0.0.1:%d/
    # <<< Aftertalk reverse proxy
`

const apacheWSBlock = `
    # >>> Aftertalk WebSocket proxy — managed by aftertalk-installer
    RewriteEngine On
    RewriteCond %%{HTTP:Upgrade} websocket [NC]
    RewriteCond %%{HTTP:Connection} upgrade [NC]
    RewriteRule ^/aftertalk/(ws|signaling)(.*) ws://127.0.0.1:%d/$1$2 [P,L]
    # <<< Aftertalk WebSocket proxy
`

const installerMarker = "# >>> Aftertalk reverse proxy — managed by aftertalk-installer"

func runApache(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	if cfg.SkipApache || cfg.ApacheVhostConf == "" {
		log.Info("Apache proxy configuration skipped")
		return nil
	}

	// Check if block is already present.
	content, err := os.ReadFile(cfg.ApacheVhostConf) //nolint:gosec
	if err != nil {
		return fmt.Errorf("read %s: %w", cfg.ApacheVhostConf, err)
	}
	if strings.Contains(string(content), installerMarker) {
		log.Info("Apache vhost already contains aftertalk proxy block — skipping")
		return nil
	}

	// Find the closing </VirtualHost> tag and inject before it.
	proxyBlock := fmt.Sprintf(apacheProxyBlock, cfg.HTTPPort, cfg.HTTPPort)
	wsBlock := fmt.Sprintf(apacheWSBlock, cfg.HTTPPort)
	injection := proxyBlock + wsBlock

	var out bytes.Buffer
	scanner := bufio.NewScanner(bytes.NewReader(content))
	injected := false
	for scanner.Scan() {
		line := scanner.Text()
		if !injected && strings.TrimSpace(line) == "</VirtualHost>" {
			out.WriteString(injection)
			injected = true
		}
		out.WriteString(line + "\n")
	}
	if !injected {
		return fmt.Errorf("could not find </VirtualHost> in %s", cfg.ApacheVhostConf)
	}

	if err := os.WriteFile(cfg.ApacheVhostConf, out.Bytes(), 0o644); err != nil { //nolint:gosec
		return fmt.Errorf("write %s: %w", cfg.ApacheVhostConf, err)
	}
	log.Info(fmt.Sprintf("injected proxy block into %s", cfg.ApacheVhostConf))

	// Enable required modules.
	for _, mod := range []string{"proxy", "proxy_http", "proxy_wstunnel", "rewrite"} {
		cmd := exec.CommandContext(ctx, "a2enmod", mod)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Warn(fmt.Sprintf("a2enmod %s: %s", mod, out))
		} else {
			log.Info("a2enmod: " + mod)
		}
	}

	// Test and reload Apache.
	if out, err := exec.CommandContext(ctx, "apache2ctl", "configtest").CombinedOutput(); err != nil {
		return fmt.Errorf("apache2ctl configtest: %w\n%s", err, out)
	}
	if out, err := exec.CommandContext(ctx, "systemctl", "reload", "apache2").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl reload apache2: %w\n%s", err, out)
	}
	log.Info("apache2 reloaded")
	return nil
}
