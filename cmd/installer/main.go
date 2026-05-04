// Command aftertalk-installer installs and configures the aftertalk server.
//
// Usage:
//
//	aftertalk-installer                    # interactive mode
//	aftertalk-installer --env install.env  # non-interactive, env file
//	aftertalk-installer --agent            # HTTP SSE agent on :9977
//	aftertalk-installer --dry-run          # print steps without running
//	aftertalk-installer --step 40-binary   # run only one step
//	aftertalk-installer --from 30-config   # resume from a given step
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Josepavese/aftertalk/cmd/installer/agent"
	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
	"github.com/Josepavese/aftertalk/cmd/installer/steps"
	"github.com/Josepavese/aftertalk/internal/version"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		envFile     = flag.String("env", "", "path to KEY=VALUE env file (non-interactive)")
		agentMode   = flag.Bool("agent", false, "run as HTTP SSE agent on port 9977")
		agentPort   = flag.Int("port", 9977, "agent HTTP port (with --agent)")
		dryRun      = flag.Bool("dry-run", false, "print steps without executing")
		stepOnly    = flag.String("step", "", "run only this step ID")
		stepFrom    = flag.String("from", "", "run from this step ID onwards")
		versionFlag = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *versionFlag {
		fmt.Println(version.Line("aftertalk-installer"))
		return nil
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// ── Load configuration ────────────────────────────────────────────────

	var cfg *instconfig.InstallConfig
	var err error

	switch {
	case *envFile != "":
		cfg, err = loadFromEnvFile(*envFile)
	case *agentMode:
		// In agent mode config is provided by the deploy orchestrator at /run time.
		cfg = instconfig.Default()
	default:
		cfg, err = instconfig.Interactive()
	}
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	applyVerificationEnv(cfg)

	cfg.DryRun = *dryRun

	// ── Agent mode ────────────────────────────────────────────────────────

	if *agentMode {
		a := agent.New(cfg, *agentPort)
		return a.ListenAndServe(ctx)
	}

	// ── Step runner ───────────────────────────────────────────────────────

	log := &steps.StdLogger{}
	runner := steps.NewRunner(cfg, log)
	runner.Only = *stepOnly
	runner.From = *stepFrom

	fmt.Printf("\nStarting aftertalk installation (dry-run=%v)\n\n", *dryRun)

	results := runner.Run(ctx)

	// ── Summary ───────────────────────────────────────────────────────────

	fmt.Println()
	fmt.Println("─────────────────────────────────────────────")
	failed := 0
	for _, r := range results {
		switch {
		case r.Err != nil:
			fmt.Printf(" ✗ [%s] FAILED: %v\n", r.StepID, r.Err)
			failed++
		case r.Skipped:
			fmt.Printf(" ─ [%s] skipped\n", r.StepID)
		default:
			fmt.Printf(" ✓ [%s] done (%s)\n", r.StepID, r.Duration)
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d step(s) failed", failed)
	}
	fmt.Println("\n  Installation complete.")
	return nil
}

func loadFromEnvFile(path string) (*instconfig.InstallConfig, error) {
	m, err := instconfig.ReadEnvFile(path)
	if err != nil {
		return nil, fmt.Errorf("read env file %s: %w", path, err)
	}
	return instconfig.FromEnvMap(m), nil
}

func applyVerificationEnv(cfg *instconfig.InstallConfig) {
	if v := os.Getenv("AFTERTALK_EXPECTED_TAG"); v != "" {
		cfg.ExpectedTag = v
	}
	if v := os.Getenv("AFTERTALK_EXPECTED_COMMIT"); v != "" {
		cfg.ExpectedCommit = v
	}
}
