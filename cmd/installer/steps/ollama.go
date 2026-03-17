package steps

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
)

func stepOllama() *Step {
	return &Step{
		ID:          "10-ollama",
		Description: "Install Ollama and pull LLM model",
		Run:         runOllama,
	}
}

func runOllama(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	if cfg.LLMProvider != "ollama" {
		log.Info(fmt.Sprintf("LLM provider is %q — skipping Ollama installation", cfg.LLMProvider))
		return nil
	}

	// Install Ollama daemon if not already present.
	if _, err := exec.LookPath("ollama"); err != nil {
		log.Info("Ollama not found — installing...")
		if err := installOllamaBinary(ctx, log); err != nil {
			return fmt.Errorf("install ollama: %w", err)
		}
	} else {
		log.Info("Ollama already installed — skipping download")
	}

	// Ensure service is running.
	if err := startOllamaService(ctx, log); err != nil {
		return fmt.Errorf("start ollama service: %w", err)
	}

	// Pull the model.
	model := cfg.OllamaModel
	log.Info(fmt.Sprintf("pulling Ollama model: %s", model))
	cmd := exec.CommandContext(ctx, "ollama", "pull", model) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ollama pull %s: %w\n%s", model, err, out)
	}
	log.Info(fmt.Sprintf("model ready: %s", model))
	return nil
}

func installOllamaBinary(ctx context.Context, log Logger) error {
	switch runtime.GOOS {
	case "linux":
		log.Info("downloading Ollama installer (Linux)")
		// Official one-liner — pipes to sh.
		cmd := exec.CommandContext(ctx, "sh", "-c", //nolint:gosec
			`curl -fsSL https://ollama.com/install.sh | sh`)
		cmd.Env = append(cmd.Environ(), "OLLAMA_NO_GPU_MSG=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ollama installer: %w\n%s", err, out)
		}
	case "darwin":
		if _, err := exec.LookPath("brew"); err != nil {
			return fmt.Errorf("homebrew not found — install Ollama manually from https://ollama.com")
		}
		log.Info("brew install ollama")
		cmd := exec.CommandContext(ctx, "brew", "install", "ollama")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("brew install ollama: %w\n%s", err, out)
		}
	case "windows":
		return fmt.Errorf("automatic Ollama install not supported on Windows — " +
			"download and run the installer from https://ollama.com then re-run the installer")
	default:
		return fmt.Errorf("unsupported platform for Ollama: %s", runtime.GOOS)
	}
	return nil
}

func startOllamaService(ctx context.Context, log Logger) error {
	switch runtime.GOOS {
	case "linux":
		// Ollama installs a systemd unit; start it if not already running.
		check := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "ollama")
		if check.Run() != nil {
			log.Info("systemctl: [start ollama]")
			cmd := exec.CommandContext(ctx, "systemctl", "start", "ollama")
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("systemctl start ollama: %w\n%s", err, out)
			}
		} else {
			log.Info("Ollama service already running")
		}
	case "darwin":
		check := exec.CommandContext(ctx, "brew", "services", "list")
		if out, _ := check.Output(); len(out) > 0 {
			log.Info("brew services start ollama")
			cmd := exec.CommandContext(ctx, "brew", "services", "start", "ollama")
			if out2, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("brew services start ollama: %w\n%s", err, out2)
			}
		}
	default:
		// Windows: Ollama runs as a user-space tray app; assume it started by the installer.
		log.Info("Ollama service management skipped on this platform")
	}
	return nil
}
