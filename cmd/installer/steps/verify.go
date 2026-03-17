package steps

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
)

func stepVerify() *Step {
	return &Step{
		ID:          "90-verify",
		Description: "Verify aftertalk is running and healthy",
		Run:         runVerify,
	}
}

func runVerify(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	if err := waitAftertalkHealthy(ctx, cfg, log); err != nil {
		return err
	}
	checkDependencies(ctx, cfg, log)
	return nil
}

func waitAftertalkHealthy(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	healthURL := fmt.Sprintf("http://127.0.0.1:%d/v1/health", cfg.HTTPPort)
	client := &http.Client{Timeout: 3 * time.Second}

	var lastErr error
	for attempt := 1; attempt <= 10; attempt++ {
		log.Info(fmt.Sprintf("health check attempt %d/10: %s", attempt, healthURL))

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
		if err != nil {
			return fmt.Errorf("build health request: %w", err)
		}
		if cfg.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
		resp, err := client.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body) //nolint:errcheck
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				log.Info(fmt.Sprintf("aftertalk is healthy (HTTP %d)", resp.StatusCode))
				return nil
			}
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		if attempt < 10 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
	}
	return fmt.Errorf("health check failed after 10 attempts: %w", lastErr)
}

// checkDependencies warns (but does not fail) if STT/LLM services are unreachable.
// Services may still be starting up; the operator is informed to check manually.
func checkDependencies(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) {
	if cfg.STTProvider == "whisper-local" {
		if err := checkEndpoint(ctx, cfg.WhisperURL+"/v1/models", 5*time.Second); err != nil {
			log.Warn(fmt.Sprintf("whisper-local at %s not reachable: %v (check aftertalk-whisper.service)", cfg.WhisperURL, err))
		} else {
			log.Info("whisper-local reachable ✓")
		}
	}
	if cfg.LLMProvider == "ollama" {
		if err := checkEndpoint(ctx, cfg.OllamaURL+"/api/tags", 5*time.Second); err != nil {
			log.Warn(fmt.Sprintf("ollama at %s not reachable: %v (check ollama.service)", cfg.OllamaURL, err))
		} else {
			log.Info("ollama reachable ✓")
		}
	}
}

func checkEndpoint(ctx context.Context, url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
