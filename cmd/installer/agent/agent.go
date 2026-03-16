// Package agent provides the HTTP SSE install agent.
// It replaces the legacy Python install_agent.py. The deploy system connects
// to this agent on port 9977 to trigger and monitor installation progress.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	instconfig "github.com/Josepavese/aftertalk/cmd/installer/config"
	"github.com/Josepavese/aftertalk/cmd/installer/steps"
)

const defaultPort = 9977

// Agent serves the HTTP install API and streams log events via SSE.
type Agent struct {
	cfg    *instconfig.InstallConfig
	port   int
	mu     sync.Mutex
	events []Event
	done   chan struct{}
}

// Event is a single log line emitted during installation.
type Event struct {
	Level   string    `json:"level"` // info | warn | error
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// StatusResponse is returned by GET /status.
type StatusResponse struct {
	Running bool    `json:"running"`
	Events  []Event `json:"events"`
}

// New creates an Agent with the given config. Port defaults to 9977.
func New(cfg *instconfig.InstallConfig, port int) *Agent {
	if port == 0 {
		port = defaultPort
	}
	return &Agent{
		cfg:  cfg,
		port: port,
		done: make(chan struct{}),
	}
}

// ListenAndServe starts the HTTP server. It blocks until the context is done.
func (a *Agent) ListenAndServe(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/run", a.handleRun)
	mux.HandleFunc("/status", a.handleStatus)
	mux.HandleFunc("/events", a.handleSSE)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", a.port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx) //nolint:errcheck
	}()

	fmt.Printf("aftertalk-installer agent listening on :%d\n", a.port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// handleRun triggers an install run. POST /run accepts an optional JSON body
// with override fields (currently unused — config is fixed at startup).
func (a *Agent) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	log := &sseLogger{agent: a}
	runner := steps.NewRunner(a.cfg, log)

	go func() {
		results := runner.Run(r.Context())
		for _, res := range results {
			if res.Err != nil {
				log.Error(fmt.Sprintf("[%s] FAILED: %v", res.StepID, res.Err))
			}
		}
		close(a.done)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"}) //nolint:errcheck
}

// handleStatus returns current run state and all buffered log events.
func (a *Agent) handleStatus(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	evts := make([]Event, len(a.events))
	copy(evts, a.events)
	a.mu.Unlock()

	running := true
	select {
	case <-a.done:
		running = false
	default:
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StatusResponse{Running: running, Events: evts}) //nolint:errcheck
}

// handleSSE streams log events as Server-Sent Events. The deploy frontend
// connects to this endpoint to display live progress.
func (a *Agent) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sent := 0
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			a.mu.Lock()
			newEvents := a.events[sent:]
			toSend := make([]Event, len(newEvents))
			copy(toSend, newEvents)
			sent += len(toSend)
			a.mu.Unlock()

			for _, evt := range toSend {
				data, _ := json.Marshal(evt)
				fmt.Fprintf(w, "data: %s\n\n", data)
			}
			flusher.Flush()

			// Signal completion via SSE.
			select {
			case <-a.done:
				fmt.Fprintf(w, "event: done\ndata: {}\n\n")
				flusher.Flush()
				return
			default:
			}
		}
	}
}

func (a *Agent) append(level, msg string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, Event{Level: level, Message: msg, Time: time.Now()})
}

// sseLogger implements steps.Logger and buffers events in the agent.
type sseLogger struct{ agent *Agent }

func (l *sseLogger) Info(msg string)  { l.agent.append("info", msg); fmt.Println(" ✓ " + msg) }
func (l *sseLogger) Warn(msg string)  { l.agent.append("warn", msg); fmt.Println(" ⚠ " + msg) }
func (l *sseLogger) Error(msg string) { l.agent.append("error", msg); fmt.Println(" ✗ " + msg) }
