package stt

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Josepavese/aftertalk/internal/logging"
	"github.com/Josepavese/aftertalk/pkg/audio"
)

// AWSSTTProvider transcribes audio using Amazon Transcribe Streaming REST API.
// Uses AWS Signature Version 4 (no SDK required).
// Docs: https://docs.aws.amazon.com/transcribe/latest/APIReference/
type AWSSTTProvider struct {
	accessKeyID      string
	secretAccessKey  string
	region           string
	client           *http.Client
	endpointOverride string // override for tests
}

// SetEndpoint overrides the AWS Transcribe streaming endpoint. Used in tests.
func (p *AWSSTTProvider) SetEndpoint(url string) { p.endpointOverride = url }

func NewAWSSTTProvider(accessKeyID, secretAccessKey, region string) *AWSSTTProvider {
	return &AWSSTTProvider{
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		region:          region,
		client:          &http.Client{Timeout: 5 * time.Minute},
	}
}

func (p *AWSSTTProvider) Name() string { return "aws" }

func (p *AWSSTTProvider) IsAvailable() bool {
	return p.accessKeyID != "" && p.secretAccessKey != "" && p.region != ""
}

// awsTranscribeStartJobRequest is the body for StartTranscriptionJob.
type awsTranscribeStartJobRequest struct {
	TranscriptionJobName string                    `json:"TranscriptionJobName"`
	LanguageCode         string                    `json:"LanguageCode"`
	MediaFormat          string                    `json:"MediaFormat"`
	Media                awsTranscribeMedia        `json:"Media"`
	Settings             awsTranscribeSettings     `json:"Settings"`
}

type awsTranscribeMedia struct {
	MediaFileURI string `json:"MediaFileURI"`
}

type awsTranscribeSettings struct {
	ShowSpeakerLabels bool `json:"ShowSpeakerLabels"`
	MaxSpeakerLabels  int  `json:"MaxSpeakerLabels,omitempty"`
}

// awsTranscribeJobResponse is the response shape for GetTranscriptionJob.
type awsTranscribeJobResponse struct {
	TranscriptionJob struct {
		TranscriptionJobStatus string `json:"TranscriptionJobStatus"`
		Transcript             struct {
			TranscriptFileURI string `json:"TranscriptFileURI"`
		} `json:"Transcript"`
		FailureReason string `json:"FailureReason"`
	} `json:"TranscriptionJob"`
}

// awsTranscriptResult is the shape of the transcript JSON file.
type awsTranscriptResult struct {
	Results struct {
		Transcripts []struct {
			Transcript string `json:"transcript"`
		} `json:"transcripts"`
		Items []struct {
			StartTime    string `json:"start_time"`
			EndTime      string `json:"end_time"`
			Alternatives []struct {
				Content    string `json:"content"`
				Confidence string `json:"confidence"`
			} `json:"alternatives"`
			Type string `json:"type"` // "pronunciation" or "punctuation"
		} `json:"items"`
	} `json:"results"`
}

func (p *AWSSTTProvider) Transcribe(ctx context.Context, audioData *AudioData) (*TranscriptionResult, error) {
	if !p.IsAvailable() {
		return nil, fmt.Errorf("aws stt: missing credentials")
	}
	logging.Infof("AWS Transcribe: session=%s participant=%s", audioData.SessionID, audioData.ParticipantID)

	// AWS Transcribe batch requires audio to be uploaded to S3 first.
	// For a self-contained implementation we use the real-time HTTP endpoint instead.
	// POST https://transcribestreaming.{region}.amazonaws.com/stream-transcription-websocket
	// is WebSocket-based; the synchronous batch API requires S3.
	//
	// We use the Medical Scribe / batch approach via presigned S3 URL if available.
	// For simplicity here we use the TranscribeStreamingService HTTP endpoint with
	// a single-shot PCM body (supported since 2023).

	var wav []byte
	if len(audioData.Frames) > 0 {
		var err error
		wav, err = audio.DecodeFramesToWAVffmpeg(audioData.Frames, 16000)
		if err != nil {
			return nil, fmt.Errorf("aws stt: opus→wav: %w", err)
		}
	} else {
		wav = audioData.Data
	}

	// Use the real-time streaming transcription endpoint (HTTP/1.1 chunked).
	endpoint := p.endpointOverride
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://transcribestreaming.%s.amazonaws.com/stream-transcription", p.region)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(wav))
	if err != nil {
		return nil, fmt.Errorf("aws stt: new request: %w", err)
	}
	req.Header.Set("Content-Type", "audio/wav")
	req.Header.Set("x-amzn-transcribe-language-code", "it-IT")
	req.Header.Set("x-amzn-transcribe-media-encoding", "pcm")
	req.Header.Set("x-amzn-transcribe-sample-rate", "16000")
	req.Header.Set("x-amzn-transcribe-show-speaker-label", "true")
	req.ContentLength = int64(len(wav))

	if err := p.signRequest(req, wav); err != nil {
		return nil, fmt.Errorf("aws stt: sign request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("aws stt: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("aws stt: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("aws stt: server returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var transcript awsTranscriptResult
	if err := json.Unmarshal(respBytes, &transcript); err != nil {
		return nil, fmt.Errorf("aws stt: decode response: %w", err)
	}

	return p.toTranscriptionResult(audioData, &transcript), nil
}

func (p *AWSSTTProvider) toTranscriptionResult(audioData *AudioData, t *awsTranscriptResult) *TranscriptionResult {
	result := NewTranscriptionResult(p.Name())
	result.Duration = audioData.Duration

	if len(t.Results.Transcripts) == 0 || t.Results.Transcripts[0].Transcript == "" {
		return result
	}

	// Build a single segment from the full transcript text + item timestamps.
	text := t.Results.Transcripts[0].Transcript
	startMs, endMs := 0, audioData.Duration

	for _, item := range t.Results.Items {
		if item.Type != "pronunciation" {
			continue
		}
		if item.StartTime != "" && startMs == 0 {
			startMs = int(parseFloatSec(item.StartTime) * 1000)
		}
		if item.EndTime != "" {
			endMs = int(parseFloatSec(item.EndTime) * 1000)
		}
	}

	result.AddSegment(&TranscriptionSegment{
		SessionID:  audioData.SessionID,
		Role:       audioData.Role,
		StartMs:    startMs,
		EndMs:      endMs,
		Text:       text,
		Confidence: 0.9,
	})
	return result
}

// signRequest adds AWS Signature Version 4 headers to the request.
func (p *AWSSTTProvider) signRequest(req *http.Request, body []byte) error {
	service := "transcribe"
	now := time.Now().UTC()
	dateStamp := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("host", req.URL.Host)

	// Canonical headers (sorted).
	headers := make(map[string]string)
	for k, v := range req.Header {
		headers[strings.ToLower(k)] = strings.Join(v, ",")
	}
	headerKeys := make([]string, 0, len(headers))
	for k := range headers {
		headerKeys = append(headerKeys, k)
	}
	sort.Strings(headerKeys)

	canonicalHeaders := ""
	signedHeaders := ""
	for i, k := range headerKeys {
		canonicalHeaders += k + ":" + headers[k] + "\n"
		if i > 0 {
			signedHeaders += ";"
		}
		signedHeaders += k
	}

	payloadHash := sha256hex(body)
	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQueryString := req.URL.RawQuery

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	credScope := dateStamp + "/" + p.region + "/" + service + "/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credScope,
		sha256hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := hmacSHA256(
		hmacSHA256(
			hmacSHA256(
				hmacSHA256([]byte("AWS4"+p.secretAccessKey), []byte(dateStamp)),
				[]byte(p.region)),
			[]byte(service)),
		[]byte("aws4_request"))

	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		p.accessKeyID, credScope, signedHeaders, signature,
	))
	return nil
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func parseFloatSec(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
