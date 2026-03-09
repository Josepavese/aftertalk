package tests

import (
	"context"
	"testing"

	"github.com/flowup/aftertalk/internal/ai/stt"
	"github.com/flowup/aftertalk/internal/logging"
)

func init() {
	logging.Init("info", "console") //nolint:errcheck
}

func TestGoogleSTTProvider_Name(t *testing.T) {
	provider := stt.NewGoogleSTTProvider("creds")
	name := provider.Name()

	if name != "google" {
		t.Errorf("Name mismatch: got %s, want %s", name, "google")
	}
}

func TestGoogleSTTProvider_IsAvailable(t *testing.T) {
	tests := []struct {
		name     string
		creds    string
		expected bool
	}{
		{"with credentials", "/valid/creds.json", true},
		{"without credentials", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := stt.NewGoogleSTTProvider(tt.creds)
			available := provider.IsAvailable()

			if available != tt.expected {
				t.Errorf("Availability mismatch: got %v, want %v", available, tt.expected)
			}
		})
	}
}

func TestGoogleSTTProvider_Transcribe(t *testing.T) {
	provider := stt.NewGoogleSTTProvider("valid-creds")
	audioData := &stt.AudioData{
		SessionID:     "session1",
		ParticipantID: "p1",
		Role:          "user",
		Data:          []byte("test audio data"),
		SampleRate:    16000,
		Duration:      60,
	}

	result, err := provider.Transcribe(context.Background(), audioData)

	if err != nil {
		t.Errorf("Transcribe should not fail, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Provider != "google" {
		t.Errorf("Provider mismatch: got %s, want %s", result.Provider, "google")
	}
	if len(result.Segments) != 1 {
		t.Errorf("Expected 1 segment, got %d", len(result.Segments))
	}
	if result.Segments[0].Role != "user" {
		t.Errorf("Role mismatch: got %s, want %s", result.Segments[0].Role, "user")
	}
	if result.Segments[0].StartMs != 0 {
		t.Errorf("StartMs mismatch: got %d, want 0", result.Segments[0].StartMs)
	}
	if result.Segments[0].EndMs != 60 {
		t.Errorf("EndMs mismatch: got %d, want 60", result.Segments[0].EndMs)
	}
	if result.Segments[0].Confidence != 0.95 {
		t.Errorf("Confidence mismatch: got %f, want 0.95", result.Segments[0].Confidence)
	}
	if result.Duration != 60 {
		t.Errorf("Duration mismatch: got %d, want 60", result.Duration)
	}
}

func TestGoogleSTTProvider_TranscribeMultipleSegments(t *testing.T) {
	provider := stt.NewGoogleSTTProvider("valid-creds")
	audioData := &stt.AudioData{
		SessionID:     "session1",
		ParticipantID: "p1",
		Role:          "user",
		Data:          []byte("test"),
		SampleRate:    16000,
		Duration:      120,
	}

	result, err := provider.Transcribe(context.Background(), audioData)
	if err != nil {
		t.Errorf("Transcribe failed: %v", err)
	}

	result.AddSegment(&stt.TranscriptionSegment{
		ID:         "seg2",
		SessionID:  "session1",
		Role:       "user",
		StartMs:    60,
		EndMs:      120,
		Text:       "second segment",
		Confidence: 0.90,
	})

	if len(result.Segments) != 2 {
		t.Errorf("Expected 2 segments, got %d", len(result.Segments))
	}
	if result.Segments[1].Text != "second segment" {
		t.Errorf("Second segment text mismatch")
	}
}

func TestAWSSTTProvider_Name(t *testing.T) {
	provider := stt.NewAWSSTTProvider("key1", "key2", "us-east-1")
	name := provider.Name()

	if name != "aws" {
		t.Errorf("Name mismatch: got %s, want %s", name, "aws")
	}
}

func TestAWSSTTProvider_IsAvailable(t *testing.T) {
	tests := []struct {
		name      string
		accessKey string
		secretKey string
		region    string
		expected  bool
	}{
		{"with all credentials", "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1", true},
		{"without access key", "", "secret", "us-east-1", false},
		{"without secret key", "access", "", "us-east-1", false},
		{"without region", "access", "secret", "", false},
		{"with all credentials but empty region", "access", "secret", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := stt.NewAWSSTTProvider(tt.accessKey, tt.secretKey, tt.region)
			available := provider.IsAvailable()

			if available != tt.expected {
				t.Errorf("Availability mismatch: got %v, want %v", available, tt.expected)
			}
		})
	}
}

func TestAWSSTTProvider_Transcribe(t *testing.T) {
	provider := stt.NewAWSSTTProvider("AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "us-east-1")
	audioData := &stt.AudioData{
		SessionID:     "session1",
		ParticipantID: "p1",
		Role:          "moderator",
		Data:          []byte("audio data"),
		SampleRate:    8000,
		Duration:      30,
	}

	result, err := provider.Transcribe(context.Background(), audioData)

	if err != nil {
		t.Errorf("Transcribe should not fail, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Provider != "aws" {
		t.Errorf("Provider mismatch: got %s, want %s", result.Provider, "aws")
	}
	if result.Segments[0].Role != "moderator" {
		t.Errorf("Role mismatch: got %s, want %s", result.Segments[0].Role, "moderator")
	}
	if result.Segments[0].Confidence != 0.90 {
		t.Errorf("Confidence mismatch: got %f, want 0.90", result.Segments[0].Confidence)
	}
}

func TestAzureSTTProvider_Name(t *testing.T) {
	provider := stt.NewAzureSTTProvider("key123", "eastus")
	name := provider.Name()

	if name != "azure" {
		t.Errorf("Name mismatch: got %s, want %s", name, "azure")
	}
}

func TestAzureSTTProvider_IsAvailable(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		region   string
		expected bool
	}{
		{"with key and region", "abc123def456", "eastus", true},
		{"without key", "", "eastus", false},
		{"without region", "abc123", "", false},
		{"both empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := stt.NewAzureSTTProvider(tt.key, tt.region)
			available := provider.IsAvailable()

			if available != tt.expected {
				t.Errorf("Availability mismatch: got %v, want %v", available, tt.expected)
			}
		})
	}
}

func TestAzureSTTProvider_Transcribe(t *testing.T) {
	provider := stt.NewAzureSTTProvider("abc123def456", "eastus")
	audioData := &stt.AudioData{
		SessionID:     "session1",
		ParticipantID: "p1",
		Role:          "user",
		Data:          []byte("test data"),
		SampleRate:    16000,
		Duration:      45,
	}

	result, err := provider.Transcribe(context.Background(), audioData)

	if err != nil {
		t.Errorf("Transcribe should not fail, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Provider != "azure" {
		t.Errorf("Provider mismatch: got %s, want %s", result.Provider, "azure")
	}
	if result.Segments[0].Confidence != 0.92 {
		t.Errorf("Confidence mismatch: got %f, want 0.92", result.Segments[0].Confidence)
	}
}

func TestProvider_NewProviderFactory(t *testing.T) {
	tests := []struct {
		name        string
		provider    string
		hasProvider bool
		err         bool
	}{
		{"google provider", "google", true, false},
		{"aws provider", "aws", true, false},
		{"azure provider", "azure", true, false},
		{"unsupported provider", "unsupported", false, true},
		{"empty provider", "", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &stt.STTConfig{
				Provider: tt.provider,
				Google: stt.GoogleConfig{
					CredentialsPath: "creds.json",
				},
			}

			provider, err := stt.NewProvider(cfg)

			if tt.err {
				if err == nil {
					t.Error("Expected error for unsupported provider")
				}
				return
			}

			if err != nil {
				t.Errorf("Expected provider %s, got error: %v", tt.provider, err)
				return
			}

			if tt.hasProvider {
				if provider == nil {
					t.Error("Expected provider to be created")
				}
			} else {
				if provider != nil {
					t.Error("Expected nil provider for unsupported type")
				}
			}
		})
	}
}

func TestGoogleSTTProvider_WithValidCreds(t *testing.T) {
	provider := stt.NewGoogleSTTProvider("/path/to/credentials.json")

	if !provider.IsAvailable() {
		t.Error("Provider should be available with valid credentials")
	}

	ctx := context.Background()
	audioData := &stt.AudioData{
		SessionID:     "test-session",
		ParticipantID: "test-participant",
		Role:          "user",
		Data:          make([]byte, 1024),
		SampleRate:    16000,
		Duration:      100,
	}

	result, err := provider.Transcribe(ctx, audioData)

	if err != nil {
		t.Errorf("Transcribe failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Segments) == 0 {
		t.Error("Expected at least one segment")
	}

	segment := result.Segments[0]
	if segment.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", segment.Confidence)
	}
}

func TestAWSSTTProvider_WithValidCreds(t *testing.T) {
	provider := stt.NewAWSSTTProvider(
		"AKIAIOSFODNN7EXAMPLE",
		"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"us-west-2",
	)

	if !provider.IsAvailable() {
		t.Error("Provider should be available with valid credentials")
	}

	ctx := context.Background()
	audioData := &stt.AudioData{
		SessionID:     "test-session",
		ParticipantID: "test-participant",
		Role:          "moderator",
		Data:          make([]byte, 2048),
		SampleRate:    8000,
		Duration:      50,
	}

	result, err := provider.Transcribe(ctx, audioData)

	if err != nil {
		t.Errorf("Transcribe failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Segments) == 0 {
		t.Error("Expected at least one segment")
	}

	segment := result.Segments[0]
	if segment.Confidence != 0.90 {
		t.Errorf("Expected confidence 0.90, got %f", segment.Confidence)
	}
}

func TestAzureSTTProvider_WithValidCreds(t *testing.T) {
	provider := stt.NewAzureSTTProvider("valid-key", "eastus")

	if !provider.IsAvailable() {
		t.Error("Provider should be available with valid credentials")
	}

	ctx := context.Background()
	audioData := &stt.AudioData{
		SessionID:     "test-session",
		ParticipantID: "test-participant",
		Role:          "user",
		Data:          make([]byte, 512),
		SampleRate:    16000,
		Duration:      25,
	}

	result, err := provider.Transcribe(ctx, audioData)

	if err != nil {
		t.Errorf("Transcribe failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(result.Segments) == 0 {
		t.Error("Expected at least one segment")
	}

	segment := result.Segments[0]
	if segment.Confidence != 0.92 {
		t.Errorf("Expected confidence 0.92, got %f", segment.Confidence)
	}
}

func TestAWSSTTProvider_EmptyCredentials(t *testing.T) {
	provider := stt.NewAWSSTTProvider("", "", "")

	if provider.IsAvailable() {
		t.Error("Provider should not be available with empty credentials")
	}

	ctx := context.Background()
	audioData := &stt.AudioData{
		SessionID: "test-session",
		Data:      []byte("audio"),
		Duration:  30,
	}

	result, err := provider.Transcribe(ctx, audioData)

	if err == nil {
		t.Error("Expected error when provider is not available")
	}
	if result != nil {
		t.Error("Expected nil result when provider is not available")
	}
}

func TestAzureSTTProvider_EmptyCredentials(t *testing.T) {
	provider := stt.NewAzureSTTProvider("", "")

	if provider.IsAvailable() {
		t.Error("Provider should not be available with empty credentials")
	}
}
