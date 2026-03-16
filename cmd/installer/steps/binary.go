package steps

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
)

func stepBinary() *Step {
	return &Step{
		ID:          "40-binary",
		Description: "Install aftertalk binary",
		Run:         runBinary,
	}
}

func runBinary(ctx context.Context, cfg *instconfig.InstallConfig, log Logger) error {
	binName := "aftertalk"
	if runtime.GOOS == "windows" {
		binName = "aftertalk.exe"
	}
	destPath := filepath.Join(cfg.ServiceRoot, binName)

	// If a local binary exists alongside the installer, copy it.
	if localBin, err := localBinaryPath(binName); err == nil {
		log.Info(fmt.Sprintf("copying local binary %s → %s", localBin, destPath))
		return copyFile(localBin, destPath)
	}

	// Otherwise, attempt download from AFTERTALK_DOWNLOAD_URL env var.
	downloadURL := os.Getenv("AFTERTALK_DOWNLOAD_URL")
	if downloadURL == "" {
		log.Warn("AFTERTALK_DOWNLOAD_URL not set and no local binary found — skipping binary install")
		return nil
	}

	log.Info(fmt.Sprintf("downloading %s → %s", downloadURL, destPath))
	return downloadBinary(ctx, downloadURL, destPath, log)
}

// localBinaryPath returns the path to a binary in the same directory as the
// running installer executable, or an error if not found.
func localBinaryPath(name string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(filepath.Dir(exe), name)
	if _, err := os.Stat(candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //nolint:gosec
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy: %w", err)
	}
	return nil
}

func downloadBinary(ctx context.Context, url, dest string, log Logger) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //nolint:gosec
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer f.Close()

	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	log.Info(fmt.Sprintf("binary installed (%d bytes) → %s", n, dest))
	return nil
}
