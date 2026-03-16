package steps

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
)

func stepFirewall() *Step {
	return &Step{
		ID:          "50-firewall",
		Description: "Open firewall port for aftertalk",
		Run:         runFirewall,
	}
}

func runFirewall(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	if cfg.SkipFirewall {
		log.Info("firewall configuration skipped (--skip-firewall)")
		return nil
	}

	port := fmt.Sprintf("%d", cfg.HTTPPort)

	switch runtime.GOOS {
	case "linux":
		return ufwAllow(ctx, port, log)
	case "darwin":
		log.Info("macOS: skipping firewall (pf not configured automatically)")
		return nil
	case "windows":
		return netshAllow(ctx, port, log)
	default:
		log.Warn(fmt.Sprintf("firewall not configured on %s", runtime.GOOS))
		return nil
	}
}

func ufwAllow(ctx context.Context, port string, log Logger) error {
	// Check if ufw is available.
	if _, err := exec.LookPath("ufw"); err != nil {
		log.Warn("ufw not found — skipping firewall rule")
		return nil
	}
	cmd := exec.CommandContext(ctx, "ufw", "allow", port+"/tcp")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ufw allow %s: %w\n%s", port, err, out)
	}
	log.Info(fmt.Sprintf("ufw: allowed port %s/tcp", port))
	return nil
}

func netshAllow(ctx context.Context, port string, log Logger) error {
	name := "Aftertalk"
	cmd := exec.CommandContext(ctx, "netsh", "advfirewall", "firewall", "add", "rule", //nolint:gosec
		"name="+name,
		"protocol=TCP",
		"dir=in",
		"action=allow",
		"localport="+port,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("netsh add rule: %w\n%s", err, out)
	}
	log.Info(fmt.Sprintf("Windows Firewall: allowed port %s/tcp (%s)", port, name))
	return nil
}
