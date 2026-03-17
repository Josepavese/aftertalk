package steps

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
)

//go:embed whisper_server.py
var whisperServerPy []byte

func stepWhisper() *Step {
	return &Step{
		ID:          "15-whisper",
		Description: "Install whisper-local STT server",
		Run:         runWhisper,
	}
}

func runWhisper(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	if cfg.STTProvider != "whisper-local" {
		log.Info(fmt.Sprintf("STT provider is %q — skipping whisper installation", cfg.STTProvider))
		return nil
	}

	whisperDir := filepath.Join(cfg.ServiceRoot, "whisper")
	venvDir := filepath.Join(whisperDir, "venv")
	scriptPath := filepath.Join(whisperDir, "whisper_server.py")

	if err := os.MkdirAll(whisperDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", whisperDir, err)
	}

	// Install Python + pip if needed.
	if err := installPython(ctx, log); err != nil {
		return fmt.Errorf("install python: %w", err)
	}

	// Create venv.
	if _, err := os.Stat(venvDir); os.IsNotExist(err) {
		log.Info(fmt.Sprintf("creating Python venv: %s", venvDir))
		cmd := exec.CommandContext(ctx, "python3", "-m", "venv", venvDir) //nolint:gosec
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("python3 -m venv: %w\n%s", err, out)
		}
	} else {
		log.Info("Python venv already exists — skipping")
	}

	// Install faster-whisper.
	pip := filepath.Join(venvDir, "bin", "pip")
	if runtime.GOOS == "windows" {
		pip = filepath.Join(venvDir, "Scripts", "pip.exe")
	}
	log.Info("pip install faster-whisper")
	cmd := exec.CommandContext(ctx, pip, "install", "-q", "faster-whisper") //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pip install faster-whisper: %w\n%s", err, out)
	}

	// Write whisper_server.py (embedded at build time).
	if err := os.WriteFile(scriptPath, whisperServerPy, 0o644); err != nil { //nolint:gosec
		return fmt.Errorf("write %s: %w", scriptPath, err)
	}
	log.Info(fmt.Sprintf("written: %s", scriptPath))

	// Install system service.
	if err := installWhisperService(ctx, cfg, whisperDir, venvDir, scriptPath, log); err != nil {
		return fmt.Errorf("install whisper service: %w", err)
	}

	log.Info(fmt.Sprintf("whisper-local ready at %s", cfg.WhisperURL))
	return nil
}

func installPython(ctx context.Context, log Logger) error {
	if _, err := exec.LookPath("python3"); err == nil {
		log.Info("python3 already installed")
		return nil
	}
	switch runtime.GOOS {
	case "linux":
		log.Info("apt-get install python3 python3-venv python3-pip")
		// Ensure universe repo is enabled on Ubuntu.
		_ = exec.CommandContext(ctx, "add-apt-repository", "-y", "universe").Run()
		_ = exec.CommandContext(ctx, "apt-get", "update", "-q").Run()
		cmd := exec.CommandContext(ctx, "apt-get", "install", "-y", "-q",
			"python3", "python3-venv", "python3-pip")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("apt-get: %w\n%s", err, out)
		}
	case "darwin":
		if _, err := exec.LookPath("brew"); err != nil {
			return fmt.Errorf("homebrew not found — install Python manually from https://python.org")
		}
		log.Info("brew install python3")
		cmd := exec.CommandContext(ctx, "brew", "install", "python3")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("brew install python3: %w\n%s", err, out)
		}
	case "windows":
		return fmt.Errorf("automatic Python install not supported on Windows — " +
			"download from https://python.org and re-run the installer")
	}
	return nil
}

const whisperServiceUnit = `[Unit]
Description=Aftertalk Whisper STT server
After=network.target

[Service]
Type=simple
User={{.User}}
WorkingDirectory={{.Dir}}
Environment=WHISPER_MODEL={{.Model}}
Environment=WHISPER_LANGUAGE=it
Environment=PORT=9001
ExecStart={{.Python}} {{.Script}}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`

func installWhisperService(ctx context.Context, cfg *instconfig.InstallConfig, whisperDir, venvDir, scriptPath string, log Logger) error {
	switch runtime.GOOS {
	case "linux":
		python := filepath.Join(venvDir, "bin", "python3")
		type tmplData struct{ User, Dir, Model, Python, Script string }
		t := template.Must(template.New("unit").Parse(whisperServiceUnit))
		var buf bytes.Buffer
		if err := t.Execute(&buf, tmplData{
			User:   cfg.ServiceUser,
			Dir:    whisperDir,
			Model:  cfg.WhisperModel,
			Python: python,
			Script: scriptPath,
		}); err != nil {
			return fmt.Errorf("render unit: %w", err)
		}
		unitPath := "/etc/systemd/system/aftertalk-whisper.service"
		if err := os.WriteFile(unitPath, buf.Bytes(), 0o644); err != nil { //nolint:gosec
			return fmt.Errorf("write %s: %w", unitPath, err)
		}
		log.Info(fmt.Sprintf("written: %s", unitPath))
		for _, args := range [][]string{
			{"daemon-reload"},
			{"enable", "aftertalk-whisper"},
			{"restart", "aftertalk-whisper"},
		} {
			log.Info(fmt.Sprintf("systemctl: %v", args))
			cmd := exec.CommandContext(ctx, "systemctl", args...) //nolint:gosec
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("systemctl %v: %w\n%s", args, err, out)
			}
		}

	case "darwin":
		python := filepath.Join(venvDir, "bin", "python3")
		plistPath := os.ExpandEnv("$HOME/Library/LaunchAgents/com.aftertalk.whisper.plist")
		plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.aftertalk.whisper</string>
  <key>ProgramArguments</key><array><string>%s</string><string>%s</string></array>
  <key>EnvironmentVariables</key><dict>
    <key>WHISPER_MODEL</key><string>%s</string>
    <key>WHISPER_LANGUAGE</key><string>it</string>
    <key>PORT</key><string>9001</string>
  </dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
</dict></plist>`, python, scriptPath, cfg.WhisperModel)
		if err := os.WriteFile(plistPath, []byte(plist), 0o644); err != nil { //nolint:gosec
			return fmt.Errorf("write plist: %w", err)
		}
		log.Info(fmt.Sprintf("written: %s", plistPath))
		exec.CommandContext(ctx, "launchctl", "unload", plistPath).Run() //nolint:errcheck
		cmd := exec.CommandContext(ctx, "launchctl", "load", plistPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("launchctl load: %w\n%s", err, out)
		}

	default:
		log.Warn("whisper service registration not implemented on this platform — start whisper_server.py manually")
	}
	return nil
}
