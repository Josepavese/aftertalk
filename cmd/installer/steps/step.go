// Package steps contains the install step registry and runner.
// Each step implements the Step interface and is registered in Registry().
// Steps are idempotent: they check a marker file and skip if already done.
package steps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
)

// Step is a single install action.
type Step struct {
	// ID is a short identifier used for marker files and CLI --step flags.
	ID string
	// Description is a human-readable summary shown in progress output.
	Description string
	// Platforms lists the OS values (runtime.GOOS) where this step applies.
	// Empty means all platforms.
	Platforms []string
	// Run executes the step. It must be idempotent.
	Run func(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error
}

// Logger is a simple structured log sink passed to each step.
// The agent uses it to stream events to SSE clients.
type Logger interface {
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

// StdLogger writes to stdout/stderr.
type StdLogger struct{}

func (s *StdLogger) Info(msg string)  { fmt.Println(" ✓ " + msg) }
func (s *StdLogger) Warn(msg string)  { fmt.Println(" ⚠ " + msg) }
func (s *StdLogger) Error(msg string) { fmt.Fprintln(os.Stderr, " ✗ "+msg) }

// markerPath returns the path of the .ok marker file for a step.
func markerPath(cfg *instconfig.InstallConfig, stepID string) string {
	return filepath.Join(cfg.ServiceRoot, ".state", "install", stepID+".ok")
}

// isDone returns true if the step's marker file exists.
func isDone(cfg *instconfig.InstallConfig, stepID string) bool {
	_, err := os.Stat(markerPath(cfg, stepID))
	return err == nil
}

// markDone creates the marker file for a step.
func markDone(cfg *instconfig.InstallConfig, stepID string) error {
	p := markerPath(cfg, stepID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.Create(p) //nolint:gosec
	if err != nil {
		return err
	}
	return f.Close()
}

// RunResult is the outcome of a single step execution.
type RunResult struct {
	StepID   string
	Skipped  bool
	Err      error
	Duration time.Duration
}

// Runner executes a list of steps in order.
type Runner struct {
	Steps  []*Step
	cfg    *instconfig.InstallConfig
	log    Logger
	Only   string // if set, run only this step ID
	From   string // if set, run from this step ID onwards
	DryRun bool
}

// NewRunner creates a Runner with all registered steps.
func NewRunner(cfg *instconfig.InstallConfig, log Logger) *Runner {
	return &Runner{
		Steps:  Registry(),
		cfg:    cfg,
		log:    log,
		DryRun: cfg.DryRun,
	}
}

// Run executes all applicable steps and returns per-step results.
func (r *Runner) Run(ctx context.Context) []RunResult {
	var results []RunResult
	fromReached := r.From == ""

	for _, step := range r.Steps {
		// --from: skip until we hit the target step
		if !fromReached {
			if step.ID == r.From {
				fromReached = true
			} else {
				continue
			}
		}
		// --step: skip everything else
		if r.Only != "" && step.ID != r.Only {
			continue
		}
		// Platform check
		if len(step.Platforms) > 0 {
			applies := false
			for _, p := range step.Platforms {
				if p == runtime.GOOS {
					applies = true
					break
				}
			}
			if !applies {
				continue
			}
		}

		start := time.Now()

		if isDone(r.cfg, step.ID) {
			r.log.Info(fmt.Sprintf("[%s] already done — skipping", step.ID))
			results = append(results, RunResult{StepID: step.ID, Skipped: true})
			continue
		}

		r.log.Info(fmt.Sprintf("[%s] %s", step.ID, step.Description))

		if r.DryRun {
			r.log.Info(fmt.Sprintf("[%s] dry-run — skipping execution", step.ID))
			results = append(results, RunResult{StepID: step.ID, Skipped: true})
			continue
		}

		if err := step.Run(ctx, r.cfg, r.log); err != nil {
			r.log.Error(fmt.Sprintf("[%s] FAILED: %v", step.ID, err))
			results = append(results, RunResult{StepID: step.ID, Err: err, Duration: time.Since(start)})
			return results // stop on first failure
		}

		if err := markDone(r.cfg, step.ID); err != nil {
			r.log.Warn(fmt.Sprintf("[%s] could not write marker: %v", step.ID, err))
		}
		r.log.Info(fmt.Sprintf("[%s] done (%s)", step.ID, time.Since(start).Round(time.Millisecond)))
		results = append(results, RunResult{StepID: step.ID, Duration: time.Since(start)})
	}

	return results
}

// Registry returns the ordered list of all install steps.
// Steps are always executed in this order.
func Registry() []*Step {
	return []*Step{
		stepPrereqs(),     // 00 — system packages
		stepOllama(),      // 10 — Ollama LLM daemon + model pull
		stepWhisper(),     // 15 — whisper-local STT server
		stepUser(),        // 20 — service user + directories
		stepConfigWrite(), // 30 — aftertalk.yaml + env file
		stepBinary(),      // 40 — aftertalk server binary
		stepFirewall(),    // 50 — open port
		stepService(),     // 60 — systemd/launchd/windows service
		stepApache(),      // 70 — Apache reverse proxy injection
		stepVerify(),      // 90 — health check
	}
}
