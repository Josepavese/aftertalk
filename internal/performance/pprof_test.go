package performance

import (
	"context"
	"math/rand"
	"net/http"
	"net/http/pprof"
	"os"
	"sync"
	"testing"
	"time"

	_ "github.com/flowup/aftertalk/internal/core"
	"github.com/flowup/aftertalk/internal/storage/sqlite"
)

var pprofServer *http.Server

func TestMain(m *testing.M) {
	// Setup pprof server
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.HandleFunc("/debug/pprof/goroutine", pprof.Handler("goroutine").ServeHTTP)
	mux.HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
	mux.HandleFunc("/debug/pprof/threadcreate", pprof.Handler("threadcreate").ServeHTTP)

	pprofServer = &http.Server{
		Addr:    ":6060",
		Handler: mux,
	}

	go func() {
		if err := pprofServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	// Setup test database
	db, err := sqlite.New(context.Background(), "/tmp/perf_pprof.db")
	if err != nil {
		panic(err)
	}
	os.Remove("/tmp/perf_pprof.db")
	defer db.Close()

	exitCode := m.Run()

	// Cleanup
	pprofServer.Shutdown(context.Background())

	os.Exit(exitCode)
}

func BenchmarkCPUProfile(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate some work
		_ = generateRandomUUID()
		_ = generateRandomString(36)
	}
}

func BenchmarkMemoryProfile(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		// Allocate memory
		_ = make([]byte, 1024)
		_ = make([]int, 1000)
	}

	b.StopTimer()
}

func BenchmarkGoroutineProfile(b *testing.B) {
	b.ResetTimer()

	var wg sync.WaitGroup

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Do some work
			time.Sleep(10 * time.Millisecond)
		}()
	}

	wg.Wait()
}

func BenchmarkLatencyProfile(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()

		// Simulate some operation
		time.Sleep(time.Millisecond * 10)

		b.ReportMetric(float64(time.Since(start))/1e6, "ms")
	}
}

func generateRandomUUID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 36)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}

func generateRandomString(length int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
