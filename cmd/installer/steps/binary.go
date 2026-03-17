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

// copyFile copies src to dst atomically using a temp file + rename.
// This avoids "text file busy" when overwriting a running executable: the old
// inode stays alive for the running process while the new inode is placed.
func copyFile(src, dst string) error {
	in, err := os.Open(src) //nolint:gosec
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}

	// Write to a temp file in the same directory so rename is atomic.
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".aftertalk-bin-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck // cleanup if rename fails

	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return fmt.Errorf("copy to temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("flush temp: %w", err)
	}

	// Atomic replace — safe even if the binary is currently running.
	if err := os.Rename(tmpName, dst); err != nil {
		return fmt.Errorf("rename %s → %s: %w", tmpName, dst, err)
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
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".aftertalk-bin-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) //nolint:errcheck

	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	n, err := io.Copy(tmp, resp.Body)
	if err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	if err := os.Rename(tmpName, dest); err != nil {
		return fmt.Errorf("rename → %s: %w", dest, err)
	}
	log.Info(fmt.Sprintf("binary installed (%d bytes) → %s", n, dest))
	return nil
}
