# Internal Go Interfaces: AI Pipeline

**Feature**: 001-aftertalk-core  
**Date**: 2026-03-04  
**Purpose**: Define Go interfaces for AI processing within the monolithic service

## Overview

In the Go monolithic architecture, the AI Pipeline is not a separate service but an internal package (`internal/ai/`). This document defines the Go interfaces that enable pluggable AI providers and clean separation of concerns.

## Core Interfaces

### 1. STTProvider Interface

```go
package stt

import "context"

// STTProvider defines the interface for Speech-to-Text providers
type STTProvider interface {
    // Name returns the provider name (e.g., "google", "aws", "azure")
    Name() string
    
    // Transcribe converts audio to text
    // audio: raw PCM mono 16kHz audio bytes
    // config: transcription configuration (language, hints, etc.)
    Transcribe(ctx context.Context, audio []byte, config Config) (*Transcription, error)
    
    // HealthCheck verifies the provider is accessible
    HealthCheck(ctx context.Context) error
}

// Config holds transcription configuration
type Config struct {
    Language         string   // e.g., "it-IT", "en-US"
    Hints            []string // Context hints for better accuracy
    EnableSpeakerDiarization bool
    MaxAlternatives  int
}

// Transcription is the result of STT
type Transcription struct {
    Segments    []Segment
    Duration    time.Duration
    Confidence  float64
    Provider    string
    Language    string
}

// Segment represents a transcribed audio segment
type Segment struct {
    Text       string
    StartMs    int64
    EndMs      int64
    Confidence float64
    SpeakerTag string // Optional, if diarization enabled
}
```

### 2. LLMProvider Interface

```go
package llm

import "context"

// LLMProvider defines the interface for Large Language Model providers
type LLMProvider interface {
    // Name returns the provider name (e.g., "openai", "anthropic", "azure")
    Name() string
    
    // Generate produces text completion
    Generate(ctx context.Context, prompt string, config Config) (*Response, error)
    
    // GenerateWithJSON produces structured JSON output
    GenerateWithJSON(ctx context.Context, prompt string, schema interface{}, config Config) ([]byte, error)
    
    // HealthCheck verifies the provider is accessible
    HealthCheck(ctx context.Context) error
}

// Config holds generation configuration
type Config struct {
    Model          string  // e.g., "gpt-4", "claude-3"
    SystemPrompt   string  // System message for context
    Temperature    float64 // Randomness (0.0-2.0)
    MaxTokens      int     // Maximum output tokens
    TopP           float64 // Nucleus sampling
    StopSequences  []string
}

// Response is the LLM generation result
type Response struct {
    Content      string
    TokensUsed   int
    FinishReason string
    Model        string
}
```

### 3. Pipeline Interface

```go
package ai

import "context"

// Pipeline orchestrates transcription and minutes generation
type Pipeline interface {
    // Transcribe processes audio and returns transcription
    Transcribe(ctx context.Context, sessionID string, audioChunks []AudioChunk) (*Transcription, error)
    
    // GenerateMinutes produces structured minutes from transcription
    GenerateMinutes(ctx context.Context, transcription *Transcription, config MinutesConfig) (*Minutes, error)
    
    // ProcessSession is a convenience method that runs the full pipeline
    ProcessSession(ctx context.Context, sessionID string, audioChunks []AudioChunk) (*Minutes, error)
}

// AudioChunk represents a chunk of audio data
type AudioChunk struct {
    Data       []byte
    Role       string    // Participant role
    StartMs    int64     // Start timestamp
    EndMs      int64     // End timestamp
    Sequence   int       // Chunk sequence number
}

// MinutesConfig holds minutes generation configuration
type MinutesConfig struct {
    Language          string
    PromptTemplate    string
    MaxCitations      int
    IncludeTimestamps bool
}

// Minutes is the structured output
type Minutes struct {
    ID                      string
    SessionID               string
    Themes                  []string
    ContentsReported        []Citation
    ProfessionalInterventions []Citation
    ProgressIssues          ProgressIssues
    NextSteps               []string
    Citations               []Citation
    GeneratedAt             time.Time
    Provider                string
}

// Citation represents a timestamped reference
type Citation struct {
    Text       string
    TimestampMs int64
    Role       string
}

// ProgressIssues contains progress and issues sections
type ProgressIssues struct {
    Progress []string
    Issues   []string
}
```

## Provider Implementations

### Google STT

```go
package stt

type GoogleSTT struct {
    client    *http.Client
    apiKey    string
    endpoint  string
}

func NewGoogleSTT(apiKey string) *GoogleSTT {
    return &GoogleSTT{
        client:   &http.Client{Timeout: 60 * time.Second},
        apiKey:   apiKey,
        endpoint: "https://speech.googleapis.com/v1/speech:recognize",
    }
}

func (g *GoogleSTT) Transcribe(ctx context.Context, audio []byte, config Config) (*Transcription, error) {
    req := GoogleRequest{
        Audio: GoogleAudio{
            Content: base64.StdEncoding.EncodeToString(audio),
        },
        Config: GoogleConfig{
            Encoding:          "LINEAR16",
            SampleRateHertz:   16000,
            LanguageCode:      config.Language,
            EnableAutomaticPunctuation: true,
        },
    }
    
    // HTTP request to Google API
    // ...
    
    return &Transcription{
        Segments:   segments,
        Duration:   duration,
        Confidence: avgConfidence,
        Provider:   "google",
        Language:   config.Language,
    }, nil
}
```

### OpenAI LLM

```go
package llm

type OpenAI struct {
    client   *http.Client
    apiKey   string
    model    string
    endpoint string
}

func NewOpenAI(apiKey, model string) *OpenAI {
    return &OpenAI{
        client:   &http.Client{Timeout: 120 * time.Second},
        apiKey:   apiKey,
        model:    model,
        endpoint: "https://api.openai.com/v1/chat/completions",
    }
}

func (o *OpenAI) Generate(ctx context.Context, prompt string, config Config) (*Response, error) {
    req := OpenAIRequest{
        Model: o.model,
        Messages: []Message{
            {Role: "system", Content: config.SystemPrompt},
            {Role: "user", Content: prompt},
        },
        Temperature:   config.Temperature,
        MaxTokens:     config.MaxTokens,
    }
    
    // HTTP request to OpenAI API
    // ...
    
    return &Response{
        Content:    resp.Choices[0].Message.Content,
        TokensUsed: resp.Usage.TotalTokens,
        Model:      resp.Model,
    }, nil
}
```

## Usage in Main Application

```go
package main

import (
    "aftertalk/internal/ai/stt"
    "aftertalk/internal/ai/llm"
    "aftertalk/internal/ai"
)

func main() {
    cfg := config.LoadConfig()
    
    // Create STT provider based on config
    var sttProvider stt.STTProvider
    switch cfg.STTProvider {
    case "google":
        sttProvider = stt.NewGoogleSTT(cfg.GoogleAPIKey)
    case "aws":
        sttProvider = stt.NewAWSSTT(cfg.AWSAccessKey, cfg.AWSSecretKey)
    default:
        log.Fatal("unknown STT provider")
    }
    
    // Create LLM provider based on config
    var llmProvider llm.LLMProvider
    switch cfg.LLMProvider {
    case "openai":
        llmProvider = llm.NewOpenAI(cfg.OpenAIAPIKey, "gpt-4")
    case "anthropic":
        llmProvider = llm.NewAnthropic(cfg.AnthropicAPIKey, "claude-3-opus")
    default:
        log.Fatal("unknown LLM provider")
    }
    
    // Create pipeline
    pipeline := ai.NewPipeline(sttProvider, llmProvider, transcriptionRepo, minutesRepo)
    
    // Use in Bot Recorder
    botRecorder := bot.NewRecorder(pipeline, sessionRepo)
    
    // Use in API handlers
    sessionHandler := api.NewSessionHandler(pipeline, sessionRepo)
}
```

## Error Handling

```go
package ai

import "errors"

var (
    ErrTranscriptionFailed = errors.New("transcription failed")
    ErrMinutesGenerationFailed = errors.New("minutes generation failed")
    ErrProviderUnavailable = errors.New("provider unavailable")
)

// TranscriptionError wraps STT errors with context
type TranscriptionError struct {
    SessionID string
    Provider  string
    Err       error
}

func (e *TranscriptionError) Error() string {
    return fmt.Sprintf("transcription failed for session %s via %s: %v", 
        e.SessionID, e.Provider, e.Err)
}

func (e *TranscriptionError) Unwrap() error {
    return e.Err
}
```

## Testing with Mocks

```go
package ai_test

import (
    "testing"
    "aftertalk/internal/ai/stt"
    "aftertalk/internal/ai/llm"
    "aftertalk/internal/ai"
)

// Mock STT provider
type MockSTT struct {
    TranscribeFunc func(ctx context.Context, audio []byte, config stt.Config) (*stt.Transcription, error)
}

func (m *MockSTT) Name() string { return "mock" }

func (m *MockSTT) Transcribe(ctx context.Context, audio []byte, config stt.Config) (*stt.Transcription, error) {
    if m.TranscribeFunc != nil {
        return m.TranscribeFunc(ctx, audio, config)
    }
    return &stt.Transcription{
        Segments: []stt.Segment{{Text: "mock transcription"}},
    }, nil
}

func TestPipeline(t *testing.T) {
    mockSTT := &MockSTT{
        TranscribeFunc: func(ctx context.Context, audio []byte, config stt.Config) (*stt.Transcription, error) {
            return &stt.Transcription{
                Segments: []stt.Segment{
                    {Text: "Hello", StartMs: 0, EndMs: 1000},
                    {Text: "World", StartMs: 1000, EndMs: 2000},
                },
            }, nil
        },
    }
    
    mockLLM := &MockLLM{}
    
    pipeline := ai.NewPipeline(mockSTT, mockLLM, nil, nil)
    
    minutes, err := pipeline.ProcessSession(context.Background(), "session-123", nil)
    
    require.NoError(t, err)
    assert.NotEmpty(t, minutes.Themes)
}
```

## Configuration

```yaml
# config.yaml
stt:
  provider: google
  google_api_key: ${GOOGLE_API_KEY}
  language: it-IT
  
llm:
  provider: openai
  openai_api_key: ${OPENAI_API_KEY}
  model: gpt-4
  temperature: 0.3
  max_tokens: 2000

pipeline:
  max_retries: 3
  timeout: 300s
  workers: 10
```

## Key Benefits

✅ **Pluggable providers**: Easy to swap STT/LLM providers via configuration  
✅ **Interface-based**: Clean separation, easy to test with mocks  
✅ **In-process**: No network overhead, <1ms latency  
✅ **Type-safe**: Compile-time checks prevent runtime errors  
✅ **Context propagation**: Proper cancellation and timeout handling  
✅ **Error wrapping**: Stack traces preserved for debugging  
