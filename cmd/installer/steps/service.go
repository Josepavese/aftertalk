package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
)

func stepService() *Step {
	return &Step{
		ID:          "60-service",
		Description: "Install and enable aftertalk system service",
		Run:         runService,
	}
}

func runService(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	switch runtime.GOOS {
	case "linux":
		return installSystemd(ctx, cfg, log)
	case "darwin":
		return installLaunchd(ctx, cfg, log)
	case "windows":
		return installWindowsService(ctx, cfg, log)
	default:
		log.Warn(fmt.Sprintf("service installation not supported on %s", runtime.GOOS))
		return nil
	}
}

// ── Linux / systemd ──────────────────────────────────────────────────────────

const systemdUnit = `[Unit]
Description=Aftertalk — AI meeting-minutes service
After=network.target

[Service]
Type=simple
User={{ .ServiceUser }}
Group={{ .ServiceUser }}
WorkingDirectory={{ .ServiceRoot }}
ExecStart={{ .ServiceRoot }}/aftertalk -config {{ .ServiceRoot }}/aftertalk.yaml
Restart=on-failure
RestartSec=5
EnvironmentFile=-/etc/aftertalk/aftertalk.env
StandardOutput=journal
StandardError=journal
SyslogIdentifier=aftertalk

[Install]
WantedBy=multi-user.target
`

func installSystemd(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	unitPath := "/etc/systemd/system/aftertalk.service"

	t, err := template.New("unit").Parse(systemdUnit)
	if err != nil {
		return fmt.Errorf("parse systemd template: %w", err)
	}
	f, err := os.OpenFile(unitPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644) //nolint:gosec
	if err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}
	if err := t.Execute(f, cfg); err != nil {
		_ = f.Close()
		return fmt.Errorf("render unit: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close unit file: %w", err)
	}
	log.Info("written: " + unitPath)

	for _, args := range [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", "aftertalk"},
		{"systemctl", "restart", "aftertalk"},
	} {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%v: %w\n%s", args, err, out)
		}
		log.Info(fmt.Sprintf("systemctl: %v", args[1:]))
	}
	return nil
}

// ── macOS / launchd ──────────────────────────────────────────────────────────

const launchdPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>             <string>com.aftertalk.server</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{ .ServiceRoot }}/aftertalk</string>
    <string>-config</string>
    <string>{{ .ServiceRoot }}/aftertalk.yaml</string>
  </array>
  <key>RunAtLoad</key>         <true/>
  <key>KeepAlive</key>         <true/>
  <key>StandardOutPath</key>   <string>/var/log/aftertalk/aftertalk.log</string>
  <key>StandardErrorPath</key> <string>/var/log/aftertalk/aftertalk.err</string>
</dict>
</plist>
`

func installLaunchd(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	plistDir := "/Library/LaunchDaemons"
	plistPath := filepath.Join(plistDir, "com.aftertalk.server.plist")

	if err := os.MkdirAll(plistDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", plistDir, err)
	}

	t, err := template.New("plist").Parse(launchdPlist)
	if err != nil {
		return fmt.Errorf("parse plist template: %w", err)
	}
	f, err := os.OpenFile(plistPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644) //nolint:gosec
	if err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	if err := t.Execute(f, cfg); err != nil {
		_ = f.Close()
		return fmt.Errorf("render plist: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close plist: %w", err)
	}
	log.Info("written: " + plistPath)

	// Unload first (ignore errors) then load
	_ = exec.CommandContext(ctx, "launchctl", "unload", plistPath).Run()
	cmd := exec.CommandContext(ctx, "launchctl", "load", "-w", plistPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %w\n%s", err, out)
	}
	log.Info("launchd: service loaded")
	return nil
}

// ── Windows / sc.exe ─────────────────────────────────────────────────────────

func installWindowsService(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	binPath := filepath.Join(cfg.ServiceRoot, "aftertalk.exe")
	cfgPath := filepath.Join(cfg.ServiceRoot, "aftertalk.yaml")

	// Check if service already exists; if so, update binPath.
	checkCmd := exec.CommandContext(ctx, "sc", "query", "aftertalk")
	if err := checkCmd.Run(); err == nil {
		log.Info("Windows service 'aftertalk' already exists — updating")
		cmd := exec.CommandContext(ctx, "sc", "config", "aftertalk", //nolint:gosec
			"binPath=", fmt.Sprintf(`"%s" -config "%s"`, binPath, cfgPath))
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("sc config: %w\n%s", err, out)
		}
	} else {
		cmd := exec.CommandContext(ctx, "sc", "create", "aftertalk", //nolint:gosec
			"binPath=", fmt.Sprintf(`"%s" -config "%s"`, binPath, cfgPath),
			"start=", "auto",
			"DisplayName=", "Aftertalk")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("sc create: %w\n%s", err, out)
		}
		log.Info("Windows service 'aftertalk' created")
	}

	cmd := exec.CommandContext(ctx, "sc", "start", "aftertalk")
	if out, err := cmd.CombinedOutput(); err != nil {
		// "already running" is not a real error
		log.Warn(fmt.Sprintf("sc start: %s", out))
	}
	log.Info("Windows service started")
	return nil
}
