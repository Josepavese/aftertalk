package steps

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
)

func stepPrereqs() *Step {
	return &Step{
		ID:          "00-prereqs",
		Description: "Install system prerequisites",
		Run:         runPrereqs,
	}
}

func runPrereqs(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	switch runtime.GOOS {
	case "linux":
		return linuxPrereqs(ctx, log)
	case "darwin":
		return darwinPrereqs(ctx, log)
	case "windows":
		log.Info("Windows: prerequisite installation is managed by the system. Skipping.")
		return nil
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func linuxPrereqs(ctx context.Context, log Logger) error {
	pkgs := []string{"curl", "ca-certificates", "openssl", "rsync", "logrotate", "jq"}
	log.Info(fmt.Sprintf("apt-get install %v", pkgs))
	args := append([]string{"apt-get", "install", "-y", "--no-install-recommends"}, pkgs...)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("apt-get install: %w\n%s", err, out)
	}
	return nil
}

func darwinPrereqs(ctx context.Context, log Logger) error {
	// Check if brew is available; if not, skip silently.
	if _, err := exec.LookPath("brew"); err != nil {
		log.Warn("Homebrew not found — skipping package installation")
		return nil
	}
	pkgs := []string{"curl", "jq"}
	log.Info(fmt.Sprintf("brew install %v", pkgs))
	args := append([]string{"brew", "install"}, pkgs...)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("brew install: %w\n%s", err, out)
	}
	return nil
}
