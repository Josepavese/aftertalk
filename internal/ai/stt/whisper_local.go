package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/pkg/audio"
)

var errWhisperServerError = errors.New("whisper-local: server error")

// WhisperLocalProvider calls a locally-running faster-whisper-server (or any
// OpenAI-compatible /v1/audio/transcriptions endpoint) that returns word/segment
// level timestamps. This avoids any cloud dependency and works entirely on CPU.
//
// Compatible servers:
//   - faster-whisper-server: docker run -p 9000:9000 fedirz/faster-whisper-server
//   - whisper.cpp: ./server --port 9000
type WhisperLocalProvider struct {
	client *http.Client
	cfg    WhisperLocalConfig
}

func NewWhisperLocalProvider(cfg WhisperLocalConfig) *WhisperLocalProvider {
	return &WhisperLocalProvider{
		cfg: cfg,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (p *WhisperLocalProvider) Name() string { return "whisper-local" }

func (p *WhisperLocalProvider) IsAvailable() bool {
	return p.cfg.URL != ""
}

// whisperVerboseResponse is the OpenAI "verbose_json" transcription response,
// returned by faster-whisper-server and whisper.cpp when response_format=verbose_json.
type whisperVerboseResponse struct {
	Text     string           `json:"text"`
	Language string           `json:"language"`
	Words    []whisperWord    `json:"words"`
	Segments []whisperSegment `json:"segments"`
	Duration float64          `json:"duration"`
}

type whisperWord struct {
	Word        string  `json:"word"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Probability float64 `json:"probability"`
}

type whisperSegment struct {
	Text         string        `json:"text"`
	Words        []whisperWord `json:"words"`
	ID           int           `json:"id"`
	Start        float64       `json:"start"`
	End          float64       `json:"end"`
	AvgLogprob   float64       `json:"avg_logprob"`
	NoSpeechProb float64       `json:"no_speech_prob"`
}

func (p *WhisperLocalProvider) Transcribe(ctx context.Context, audioData *AudioData) (*TranscriptionResult, error) {
	logging.Infof("WhisperLocal: transcribing session=%s participant=%s pcm_bytes=%d frames=%d",
		audioData.SessionID, audioData.ParticipantID, len(audioData.Data), len(audioData.Frames))

	body, contentType, err := p.buildMultipartBody(ctx, audioData)
	if err != nil {
		return nil, fmt.Errorf("whisper-local: build request: %w", err)
	}

	// whisper.cpp server uses /inference; OpenAI-compatible servers use /v1/audio/transcriptions.
	endpoint := "/v1/audio/transcriptions"
	if p.cfg.Endpoint != "" {
		endpoint = p.cfg.Endpoint
	}
	url := p.cfg.URL + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, fmt.Errorf("whisper-local: new request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whisper-local: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("%w: %d (could not read body: %w)", errWhisperServerError, resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("%w: %d: %s", errWhisperServerError, resp.StatusCode, string(b))
	}

	var wr whisperVerboseResponse
	if err := json.NewDecoder(resp.Body).Decode(&wr); err != nil {
		return nil, fmt.Errorf("whisper-local: decode response: %w", err)
	}

	return p.toTranscriptionResult(audioData, &wr), nil
}

func (p *WhisperLocalProvider) buildMultipartBody(ctx context.Context, audioData *AudioData) (io.Reader, string, error) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)

	// Decode Opus RTP frames → 16 kHz PCM WAV (whisper.cpp requires WAV).
	// Fall back to raw PCM bytes when frames are not available.
	var audioBytes []byte
	filename := "audio.wav"

	if len(audioData.Frames) > 0 {
		wav, err := audio.DecodeFramesToWAVffmpeg(ctx, audioData.Frames, 16000)
		if err != nil {
			return nil, "", fmt.Errorf("opus→wav: %w", err)
		}
		audioBytes = wav
		// Debug: dump WAV to disk so we can verify audio content
		dbgPath := fmt.Sprintf("/tmp/aftertalk_debug_%s.wav", audioData.SessionID)
		if writeErr := os.WriteFile(dbgPath, wav, 0600); writeErr != nil {
			logging.Warnf("WhisperLocal: could not write debug WAV: %v", writeErr)
		}
		logging.Infof("WhisperLocal: wav_bytes=%d frames_decoded=%d debug_wav=%s", len(wav), len(audioData.Frames), dbgPath)
	} else {
		audioBytes = audioData.Data
	}

	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, "", err
	}
	if _, err := fw.Write(audioBytes); err != nil {
		return nil, "", err
	}

	model := p.cfg.Model
	if model == "" {
		model = "Systran/faster-whisper-large-v3"
	}
	if err := w.WriteField("model", model); err != nil {
		return nil, "", fmt.Errorf("whisper-local: write model field: %w", err)
	}

	responseFormat := p.cfg.ResponseFormat
	if responseFormat == "" {
		responseFormat = "verbose_json"
	}
	if err := w.WriteField("response_format", responseFormat); err != nil {
		return nil, "", fmt.Errorf("whisper-local: write response_format field: %w", err)
	}

	// timestamp_granularities[]: request both word and segment timestamps
	if err := w.WriteField("timestamp_granularities[]", "word"); err != nil {
		return nil, "", fmt.Errorf("whisper-local: write timestamp_granularities field: %w", err)
	}
	if err := w.WriteField("timestamp_granularities[]", "segment"); err != nil {
		return nil, "", fmt.Errorf("whisper-local: write timestamp_granularities field: %w", err)
	}

	if p.cfg.Language != "" {
		if err := w.WriteField("language", p.cfg.Language); err != nil {
			return nil, "", fmt.Errorf("whisper-local: write language field: %w", err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, "", err
	}

	return buf, w.FormDataContentType(), nil
}

// toTranscriptionResult converts the whisper response to our internal format.
// Prefers word-level timestamps if available, falls back to segment-level.
func (p *WhisperLocalProvider) toTranscriptionResult(audioData *AudioData, wr *whisperVerboseResponse) *TranscriptionResult {
	result := NewTranscriptionResult(p.Name())
	result.Duration = int(wr.Duration * 1000)

	if len(wr.Segments) > 0 {
		for _, seg := range wr.Segments {
			// Skip low-quality segments (likely silence or noise)
			if seg.NoSpeechProb > 0.8 {
				continue
			}

			confidence := logprobToConfidence(seg.AvgLogprob)
			result.AddSegment(&TranscriptionSegment{
				SessionID:  audioData.SessionID,
				Role:       audioData.Role,
				StartMs:    int(seg.Start * 1000),
				EndMs:      int(seg.End * 1000),
				Text:       seg.Text,
				Confidence: confidence,
			})
		}
		return result
	}

	// Fallback: no segments — treat whole audio as one segment
	if wr.Text != "" {
		result.AddSegment(&TranscriptionSegment{
			SessionID:  audioData.SessionID,
			Role:       audioData.Role,
			StartMs:    0,
			EndMs:      int(wr.Duration * 1000),
			Text:       wr.Text,
			Confidence: 0.8,
		})
	}

	return result
}

// logprobToConfidence converts log-probability (typically -1..0) to confidence (0..1).
func logprobToConfidence(logprob float64) float64 {
	if logprob >= 0 {
		return 1.0
	}
	if logprob < -2.0 {
		return 0.1
	}
	// Linear mapping: -2.0 → 0.1, 0.0 → 1.0
	return 1.0 + (logprob/2.0)*0.9
}
