package steps

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDownloadBinaryVerifiesSHA256(t *testing.T) {
	payload := []byte("aftertalk-test-binary")
	sum := sha256.Sum256(payload)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "aftertalk")
	t.Setenv("AFTERTALK_DOWNLOAD_SHA256", fmt.Sprintf("%x", sum))

	err := downloadBinary(context.Background(), server.URL, dest, &testLogger{})
	require.NoError(t, err)

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

func TestDownloadBinaryRejectsSHA256Mismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("tampered"))
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "aftertalk")
	t.Setenv("AFTERTALK_DOWNLOAD_SHA256", "0000000000000000000000000000000000000000000000000000000000000000")

	err := downloadBinary(context.Background(), server.URL, dest, &testLogger{})
	require.Error(t, err)
	require.NoFileExists(t, dest)
}
