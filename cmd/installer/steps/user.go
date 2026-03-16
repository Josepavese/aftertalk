package steps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
)

func stepUser() *Step {
	return &Step{
		ID:          "20-service-user",
		Description: "Create service user and directory structure",
		Platforms:   []string{"linux", "darwin"},
		Run:         runUser,
	}
}

func runUser(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	dirs := []string{
		cfg.ServiceRoot,
		filepath.Join(cfg.ServiceRoot, ".state", "install"),
		"/var/log/aftertalk",
		"/etc/aftertalk",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
		log.Info("created " + d)
	}

	if runtime.GOOS == "linux" {
		return ensureLinuxUser(ctx, cfg.ServiceUser, cfg.ServiceRoot, log)
	}
	// macOS: skip user creation (run as current user in dev scenarios)
	return nil
}

func ensureLinuxUser(ctx context.Context, username, serviceRoot string, log Logger) error {
	// Check if user already exists
	if out, err := exec.CommandContext(ctx, "id", username).CombinedOutput(); err == nil {
		log.Info(fmt.Sprintf("user %q already exists: %s", username, string(out)))
		return nil
	}

	cmd := exec.CommandContext(ctx, "useradd", //nolint:gosec
		"--system",
		"--no-create-home",
		"--home-dir", serviceRoot,
		"--shell", "/usr/sbin/nologin",
		username,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("useradd %s: %w\n%s", username, err, out)
	}
	log.Info(fmt.Sprintf("created system user %q", username))

	// chown service root
	cmd = exec.CommandContext(ctx, "chown", "-R", username+":"+username, serviceRoot) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chown %s: %w\n%s", serviceRoot, err, out)
	}
	return nil
}
