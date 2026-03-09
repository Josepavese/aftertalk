package stt

import "context"

type AudioData struct {
	SessionID     string
	ParticipantID string
	Role          string
	Data          []byte   // raw PCM bytes (int16 LE) or concatenated Opus payloads
	Frames        [][]byte // individual Opus RTP payloads; preferred by whisper-local
	SampleRate    int
	Duration      int
	// OffsetMs: milliseconds from session start to the beginning of this audio chunk.
	// Must be added to STT segment timestamps to produce session-absolute timestamps.
	OffsetMs int
}

type STTProvider interface {
	Transcribe(ctx context.Context, audioData *AudioData) (*TranscriptionResult, error)
	Name() string
	IsAvailable() bool
}

type TranscriptionResult struct {
	Segments []*TranscriptionSegment
	Provider string
	Duration int
}

type TranscriptionSegment struct {
	ID         string
	SessionID  string
	Role       string
	StartMs    int
	EndMs      int
	Text       string
	Confidence float64
}

func NewTranscriptionResult(provider string) *TranscriptionResult {
	return &TranscriptionResult{
		Segments: make([]*TranscriptionSegment, 0),
		Provider: provider,
	}
}

func (r *TranscriptionResult) AddSegment(segment *TranscriptionSegment) {
	r.Segments = append(r.Segments, segment)
}

type STTConfig struct {
	Provider     string
	Google       GoogleConfig
	AWS          AWSConfig
	Azure        AzureConfig
	WhisperLocal WhisperLocalConfig
}

type GoogleConfig struct {
	CredentialsPath string
}

type AWSConfig struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
}

type AzureConfig struct {
	Key    string
	Region string
}

// WhisperLocalConfig configures a locally-running faster-whisper (or compatible) server.
// Compatible servers: faster-whisper-server (fedirz/faster-whisper-server),
// whisper.cpp server, or any OpenAI-compatible /v1/audio/transcriptions endpoint.
type WhisperLocalConfig struct {
	// URL is the base URL of the local server, e.g. "http://localhost:9000"
	URL string
	// Model is the model name passed to the server, e.g. "large-v3", "Systran/faster-whisper-large-v3"
	Model string
	// Language forces a specific language (e.g. "it" for Italian). Empty = auto-detect.
	Language string
	// ResponseFormat: "verbose_json" for word-level timestamps, "json" for segment-level.
	ResponseFormat string
	// Endpoint overrides the transcription path. Default: /v1/audio/transcriptions.
	// Use /inference for whisper.cpp server.
	Endpoint string
}
