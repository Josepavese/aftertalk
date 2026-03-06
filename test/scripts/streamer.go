package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

type AudioStreamer struct {
	wsURL     string
	token     string
	sessionID string
	audioFile string
	role      string
}

func NewAudioStreamer(wsURL, token, sessionID, audioFile, role string) *AudioStreamer {
	return &AudioStreamer{
		wsURL:     wsURL + "?token=" + token,
		token:     token,
		sessionID: sessionID,
		audioFile: audioFile,
		role:      role,
	}
}

func (as *AudioStreamer) Stream() error {
	// Read audio file
	data, err := os.ReadFile(as.audioFile)
	if err != nil {
		return fmt.Errorf("failed to read audio file: %w", err)
	}

	fmt.Printf("🎵 Audio file: %d bytes\n", len(data))

	// Skip WAV header if present (44 bytes)
	if len(data) > 44 && string(data[:4]) == "RIFF" {
		fmt.Println("   Detected WAV format, skipping header")
		data = data[44:]
	}

	// Connect WebSocket
	headers := http.Header{
		"Authorization": []string{"Bearer " + as.token},
	}

	ws, _, err := websocket.DefaultDialer.Dial(as.wsURL, headers)
	if err != nil {
		return fmt.Errorf("failed to connect WebSocket: %w", err)
	}
	defer ws.Close()

	fmt.Printf("✅ Connected to WebSocket: %s\n", as.wsURL)

	// Send metadata first
	metadata := map[string]string{
		"type":       "session_join",
		"session_id": as.sessionID,
		"role":       as.role,
	}
	if err := ws.WriteJSON(metadata); err != nil {
		return fmt.Errorf("failed to send metadata: %w", err)
	}
	fmt.Println("📋 Session metadata sent")

	// Stream audio in chunks (20ms = 640 bytes at 16kHz 16-bit mono)
	const chunkSize = 640
	const chunkDuration = 20 * time.Millisecond

	totalChunks := len(data) / chunkSize
	fmt.Printf("▶️  Starting stream: %d chunks\n", totalChunks)

	startTime := time.Now()
	chunksSent := 0

	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := data[i:end]

		// Send audio chunk
		if err := ws.WriteMessage(websocket.BinaryMessage, chunk); err != nil {
			return fmt.Errorf("failed to send chunk %d: %w", chunksSent, err)
		}

		chunksSent++

		// Progress update every second
		if chunksSent%50 == 0 {
			progress := float64(chunksSent) / float64(totalChunks) * 100
			elapsed := time.Since(startTime)
			fmt.Printf("   📤 Progress: %.1f%% (%d/%d chunks) - %s\n",
				progress, chunksSent, totalChunks, elapsed.Round(time.Second))
		}

		// Simulate real-time streaming
		time.Sleep(chunkDuration)
	}

	// Send end signal
	endMsg := map[string]string{
		"type": "audio_end",
	}
	if err := ws.WriteJSON(endMsg); err != nil {
		log.Printf("Warning: failed to send end signal: %v", err)
	}

	elapsed := time.Since(startTime)
	fmt.Printf("\n✅ Stream completed!\n")
	fmt.Printf("   Chunks sent: %d\n", chunksSent)
	fmt.Printf("   Bytes sent: %d\n", chunksSent*chunkSize)
	fmt.Printf("   Duration: %s\n", elapsed.Round(time.Second))

	return nil
}

func main() {
	runStreamer()
}

func runStreamer() {
	var (
		wsURL     = flag.String("url", "ws://localhost:8080/ws", "WebSocket URL")
		token     = flag.String("token", "", "JWT token")
		sessionID = flag.String("session", "", "Session ID")
		audioFile = flag.String("audio", "", "Audio file path")
		role      = flag.String("role", "participant", "Role (client/professional)")
	)
	flag.Parse()

	if *token == "" || *sessionID == "" || *audioFile == "" {
		fmt.Println("Usage: test_tool stream -url <ws_url> -token <jwt> -session <id> -audio <file>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	streamer := NewAudioStreamer(*wsURL, *token, *sessionID, *audioFile, *role)

	fmt.Println("🎤 Aftertalk Audio Streamer")
	fmt.Println("============================")
	fmt.Printf("   File: %s\n", *audioFile)
	fmt.Printf("   Role: %s\n", *role)
	fmt.Printf("   Session: %s\n", *sessionID)
	fmt.Println("")

	if err := streamer.Stream(); err != nil {
		log.Fatalf("❌ Error: %v", err)
	}
}
