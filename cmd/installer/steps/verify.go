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

func runVerify(_ context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/health", cfg.HTTPPort)

	var lastErr error
	for attempt := 1; attempt <= 10; attempt++ {
		log.Info(fmt.Sprintf("health check attempt %d/10: %s", attempt, url))

		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(url) //nolint:noctx
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
			time.Sleep(3 * time.Second)
		}
	}
	return fmt.Errorf("health check failed after 10 attempts: %w", lastErr)
}
