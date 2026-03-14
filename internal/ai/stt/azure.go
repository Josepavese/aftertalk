package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/pkg/audio"
)

var (
	errAzureMissingKeyOrRegion = errors.New("azure stt: missing key or region")
	errAzureServerError        = errors.New("azure stt: server error")
)

// AzureSTTProvider transcribes audio using Azure Cognitive Services Speech REST API.
// Uses the batch transcription / fast transcription REST API (no SDK required).
// Docs: https://learn.microsoft.com/en-us/azure/ai-services/speech-service/rest-speech-to-text
type AzureSTTProvider struct {
	key              string
	region           string
	client           *http.Client
	endpointOverride string // override for tests
}

// SetEndpoint overrides the Azure Speech endpoint URL. Used in tests.
func (p *AzureSTTProvider) SetEndpoint(url string) { p.endpointOverride = url }

func NewAzureSTTProvider(key, region string) *AzureSTTProvider {
	return &AzureSTTProvider{
		key:    key,
		region: region,
		client: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (p *AzureSTTProvider) Name() string { return "azure" }

func (p *AzureSTTProvider) IsAvailable() bool {
	return p.key != "" && p.region != ""
}

// azureSpeechResponse is the response from the Azure Speech-to-Text REST API.
type azureSpeechResponse struct {
	RecognitionStatus string             `json:"RecognitionStatus"`
	DisplayText       string             `json:"DisplayText"`
	NBest             []azureSpeechNBest `json:"NBest"`
	Duration          int64              `json:"Duration"`
	Offset            int64              `json:"Offset"`
}

type azureSpeechNBest struct {
	Display    string          `json:"Display"`
	Words      []azureWordInfo `json:"Words"`
	Confidence float64         `json:"Confidence"`
}

type azureWordInfo struct {
	Word     string `json:"Word"`
	Offset   int64  `json:"Offset"`   // 100-ns units
	Duration int64  `json:"Duration"` // 100-ns units
}

func (p *AzureSTTProvider) Transcribe(ctx context.Context, audioData *AudioData) (*TranscriptionResult, error) {
	if !p.IsAvailable() {
		return nil, errAzureMissingKeyOrRegion
	}
	logging.Infof("Azure Speech: session=%s participant=%s", audioData.SessionID, audioData.ParticipantID)

	var wav []byte
	if len(audioData.Frames) > 0 {
		var err error
		wav, err = audio.DecodeFramesToWAVffmpeg(ctx, audioData.Frames, 16000)
		if err != nil {
			return nil, fmt.Errorf("azure stt: opus→wav: %w", err)
		}
	} else {
		wav = audioData.Data
	}

	// Azure Speech-to-Text REST API (short audio, up to 60s).
	// For longer audio, use the batch transcription API.
	url := p.endpointOverride
	if url == "" {
		url = fmt.Sprintf(
			"https://%s.stt.speech.microsoft.com/speech/recognition/conversation/cognitiveservices/v1?language=it-IT&format=detailed&profanity=raw&wordLevelTimestamps=true",
			p.region,
		)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(wav))
	if err != nil {
		return nil, fmt.Errorf("azure stt: new request: %w", err)
	}
	req.Header.Set("Content-Type", "audio/wav; codecs=audio/pcm; samplerate=16000")
	req.Header.Set("Ocp-Apim-Subscription-Key", p.key)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure stt: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("azure stt: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %d: %s", errAzureServerError, resp.StatusCode, string(respBytes))
	}

	var aResp azureSpeechResponse
	if err := json.Unmarshal(respBytes, &aResp); err != nil {
		return nil, fmt.Errorf("azure stt: decode response: %w", err)
	}

	return p.toTranscriptionResult(audioData, &aResp), nil
}

func (p *AzureSTTProvider) toTranscriptionResult(audioData *AudioData, aResp *azureSpeechResponse) *TranscriptionResult {
	result := NewTranscriptionResult(p.Name())
	result.Duration = audioData.Duration

	if aResp.RecognitionStatus != "Success" {
		logging.Warnf("Azure STT: non-success status %q for session %s", aResp.RecognitionStatus, audioData.SessionID)
		return result
	}

	// Prefer NBest[0] (highest confidence) if available.
	if len(aResp.NBest) > 0 {
		best := aResp.NBest[0]
		if best.Display == "" {
			return result
		}

		startMs := int(aResp.Offset / 10_000) // 100-ns → ms
		endMs := startMs + int(aResp.Duration/10_000)

		// Refine from word-level timestamps if present.
		if len(best.Words) > 0 {
			startMs = int(best.Words[0].Offset / 10_000)
			last := best.Words[len(best.Words)-1]
			endMs = int((last.Offset + last.Duration) / 10_000)
		}

		result.AddSegment(&TranscriptionSegment{
			SessionID:  audioData.SessionID,
			Role:       audioData.Role,
			StartMs:    startMs,
			EndMs:      endMs,
			Text:       best.Display,
			Confidence: best.Confidence,
		})
		return result
	}

	// Fallback to top-level DisplayText.
	if aResp.DisplayText != "" {
		result.AddSegment(&TranscriptionSegment{
			SessionID:  audioData.SessionID,
			Role:       audioData.Role,
			StartMs:    int(aResp.Offset / 10_000),
			EndMs:      int((aResp.Offset + aResp.Duration) / 10_000),
			Text:       aResp.DisplayText,
			Confidence: 0.9,
		})
	}
	return result
}
