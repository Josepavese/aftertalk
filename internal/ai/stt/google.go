package stt

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/pkg/audio"
)

// GoogleSTTProvider transcribes audio using the Google Cloud Speech-to-Text REST API v1.
// Docs: https://cloud.google.com/speech-to-text/docs/reference/rest/v1/speech/recognize
//
// Authentication: service account JSON key at cfg.CredentialsPath.
// The provider exchanges it for an OAuth2 Bearer token via the metadata endpoint,
// or uses the GOOGLE_APPLICATION_CREDENTIALS env var as fallback.
type GoogleSTTProvider struct {
	credentialsPath string
	client          *http.Client
	speechEndpoint  string // override for tests; default: https://speech.googleapis.com
}

// SetSpeechEndpoint overrides the Google Speech API URL. Used in tests.
func (p *GoogleSTTProvider) SetSpeechEndpoint(url string) { p.speechEndpoint = url }

func NewGoogleSTTProvider(credentialsPath string) *GoogleSTTProvider {
	return &GoogleSTTProvider{
		credentialsPath: credentialsPath,
		client:          &http.Client{Timeout: 5 * time.Minute},
	}
}

func (p *GoogleSTTProvider) Name() string { return "google" }

func (p *GoogleSTTProvider) IsAvailable() bool {
	if p.credentialsPath != "" {
		_, err := os.Stat(p.credentialsPath)
		return err == nil
	}
	return os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != ""
}

// googleRecognizeRequest is the body for POST https://speech.googleapis.com/v1/speech:recognize
type googleRecognizeRequest struct {
	Config googleRecognitionConfig `json:"config"`
	Audio  googleRecognitionAudio  `json:"audio"`
}

type googleRecognitionConfig struct {
	Encoding                            string   `json:"encoding"`
	SampleRateHertz                     int      `json:"sampleRateHertz"`
	LanguageCode                        string   `json:"languageCode"`
	EnableWordTimeOffsets               bool     `json:"enableWordTimeOffsets"`
	EnableAutomaticPunctuation          bool     `json:"enableAutomaticPunctuation"`
	Model                               string   `json:"model,omitempty"`
	AudioChannelCount                   int      `json:"audioChannelCount,omitempty"`
	AlternativeLanguageCodes            []string `json:"alternativeLanguageCodes,omitempty"`
}

type googleRecognitionAudio struct {
	Content string `json:"content"` // base64-encoded audio bytes
}

type googleRecognizeResponse struct {
	Results []googleSpeechRecognitionResult `json:"results"`
}

type googleSpeechRecognitionResult struct {
	Alternatives []googleSpeechRecognitionAlternative `json:"alternatives"`
}

type googleSpeechRecognitionAlternative struct {
	Transcript string            `json:"transcript"`
	Confidence float64           `json:"confidence"`
	Words      []googleWordInfo  `json:"words"`
}

type googleWordInfo struct {
	Word        string `json:"word"`
	StartTime   string `json:"startTime"` // e.g. "1.500s"
	EndTime     string `json:"endTime"`   // e.g. "2.100s"
}

func (p *GoogleSTTProvider) Transcribe(ctx context.Context, audioData *AudioData) (*TranscriptionResult, error) {
	logging.Infof("Google STT: transcribing session=%s participant=%s", audioData.SessionID, audioData.ParticipantID)

	// Convert Opus frames to WAV for Google (requires LINEAR16 or WEBM_OPUS).
	var audioBytes []byte
	encoding := "WEBM_OPUS"
	sampleRate := 48000

	if len(audioData.Frames) > 0 {
		wav, err := audio.DecodeFramesToWAVffmpeg(audioData.Frames, 16000)
		if err != nil {
			return nil, fmt.Errorf("google stt: opus→wav: %w", err)
		}
		audioBytes = wav
		encoding = "LINEAR16"
		sampleRate = 16000
	} else {
		audioBytes = audioData.Data
		encoding = "LINEAR16"
		sampleRate = 16000
	}

	langCode := "it-IT"
	if len(audioData.Role) > 0 {
		// Language can be extended via config; using Italian as default for the therapy use case.
	}

	reqBody := googleRecognizeRequest{
		Config: googleRecognitionConfig{
			Encoding:                   encoding,
			SampleRateHertz:            sampleRate,
			LanguageCode:               langCode,
			EnableWordTimeOffsets:      true,
			EnableAutomaticPunctuation: true,
			Model:                      "latest_long",
		},
		Audio: googleRecognitionAudio{
			Content: base64.StdEncoding.EncodeToString(audioBytes),
		},
	}

	token, err := p.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("google stt: auth: %w", err)
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("google stt: marshal request: %w", err)
	}

	endpoint := "https://speech.googleapis.com/v1/speech:recognize"
	if p.speechEndpoint != "" {
		endpoint = p.speechEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("google stt: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google stt: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("google stt: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google stt: server returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var gResp googleRecognizeResponse
	if err := json.Unmarshal(respBytes, &gResp); err != nil {
		return nil, fmt.Errorf("google stt: decode response: %w", err)
	}

	return p.toTranscriptionResult(audioData, &gResp), nil
}

func (p *GoogleSTTProvider) toTranscriptionResult(audioData *AudioData, gResp *googleRecognizeResponse) *TranscriptionResult {
	result := NewTranscriptionResult(p.Name())
	result.Duration = audioData.Duration

	for _, r := range gResp.Results {
		if len(r.Alternatives) == 0 {
			continue
		}
		alt := r.Alternatives[0]
		if alt.Transcript == "" {
			continue
		}

		startMs := 0
		endMs := audioData.Duration
		if len(alt.Words) > 0 {
			startMs = parseDurationS(alt.Words[0].StartTime)
			endMs = parseDurationS(alt.Words[len(alt.Words)-1].EndTime)
		}

		confidence := alt.Confidence
		if confidence == 0 {
			confidence = 0.9
		}

		result.AddSegment(&TranscriptionSegment{
			SessionID:  audioData.SessionID,
			Role:       audioData.Role,
			StartMs:    startMs,
			EndMs:      endMs,
			Text:       alt.Transcript,
			Confidence: confidence,
		})
	}
	return result
}

// parseDurationS parses a Google duration string like "1.500s" into milliseconds.
func parseDurationS(s string) int {
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return int(d.Milliseconds())
}

// googleServiceAccount is the minimal structure of a service account JSON key file.
type googleServiceAccount struct {
	Type        string `json:"type"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// getAccessToken fetches an OAuth2 Bearer token using the service account key file.
// Falls back to GOOGLE_APPLICATION_CREDENTIALS env var if credentialsPath is empty.
func (p *GoogleSTTProvider) getAccessToken(ctx context.Context) (string, error) {
	credPath := p.credentialsPath
	if credPath == "" {
		credPath = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}
	if credPath == "" {
		return "", fmt.Errorf("no Google credentials path configured and GOOGLE_APPLICATION_CREDENTIALS not set")
	}

	keyBytes, err := os.ReadFile(credPath)
	if err != nil {
		return "", fmt.Errorf("read credentials file %q: %w", credPath, err)
	}

	var sa googleServiceAccount
	if err := json.Unmarshal(keyBytes, &sa); err != nil {
		return "", fmt.Errorf("parse credentials file: %w", err)
	}

	// Build and sign a JWT to exchange for an access token.
	// Scope required: https://www.googleapis.com/auth/cloud-platform
	token, err := signServiceAccountJWT(ctx, p.client, &sa,
		"https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}
	return token, nil
}
