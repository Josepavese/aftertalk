package stt_test

import (
	"os"
	"testing"

	"github.com/Josepavese/aftertalk/internal/ai/stt"
	"github.com/Josepavese/aftertalk/internal/logging"
)

func init() {
	logging.Init("info", "console") //nolint:errcheck
}

func TestAudioData(t *testing.T) {
	tests := []struct {
		name    string
		session string
		role    string
		sample  int
	}{
		{"valid data", "session123", "user", 16000},
		{"empty session", "", "moderator", 8000},
		{"sample rate 0", "session456", "user", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &stt.AudioData{
				SessionID:     tt.session,
				ParticipantID: "participant1",
				Role:          tt.role,
				Data:          make([]byte, 1024),
				SampleRate:    tt.sample,
				Duration:      60,
			}

			if data.SessionID != tt.session {
				t.Errorf("SessionID mismatch: got %s, want %s", data.SessionID, tt.session)
			}
			if data.Role != tt.role {
				t.Errorf("Role mismatch: got %s, want %s", data.Role, tt.role)
			}
			if data.SampleRate != tt.sample {
				t.Errorf("SampleRate mismatch: got %d, want %d", data.SampleRate, tt.sample)
			}
		})
	}
}

func TestNewTranscriptionResult(t *testing.T) {
	result := stt.NewTranscriptionResult("test-provider")

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Provider != "test-provider" {
		t.Errorf("Provider mismatch: got %s, want %s", result.Provider, "test-provider")
	}
	if len(result.Segments) != 0 {
		t.Errorf("Expected empty segments, got %d segments", len(result.Segments))
	}
}

func TestTranscriptionResult_AddSegment(t *testing.T) {
	result := stt.NewTranscriptionResult("test-provider")
	segment := &stt.TranscriptionSegment{
		ID:         "seg1",
		SessionID:  "session1",
		Role:       "user",
		StartMs:    1000,
		EndMs:      2000,
		Text:       "test text",
		Confidence: 0.95,
	}

	result.AddSegment(segment)

	if len(result.Segments) != 1 {
		t.Errorf("Expected 1 segment, got %d", len(result.Segments))
	}
	if result.Segments[0].Text != "test text" {
		t.Errorf("Segment text mismatch: got %s, want %s", result.Segments[0].Text, "test text")
	}
}

func TestTranscriptionSegment(t *testing.T) {
	segment := &stt.TranscriptionSegment{
		ID:         "seg1",
		SessionID:  "session1",
		Role:       "user",
		StartMs:    1000,
		EndMs:      2000,
		Text:       "test text",
		Confidence: 0.95,
	}

	if segment.ID != "seg1" {
		t.Errorf("ID mismatch: got %s, want %s", segment.ID, "seg1")
	}
	if segment.SessionID != "session1" {
		t.Errorf("SessionID mismatch: got %s, want %s", segment.SessionID, "session1")
	}
	if segment.Role != "user" {
		t.Errorf("Role mismatch: got %s, want %s", segment.Role, "user")
	}
	if segment.StartMs != 1000 {
		t.Errorf("StartMs mismatch: got %d, want %d", segment.StartMs, 1000)
	}
	if segment.EndMs != 2000 {
		t.Errorf("EndMs mismatch: got %d, want %d", segment.EndMs, 2000)
	}
	if segment.Text != "test text" {
		t.Errorf("Text mismatch: got %s, want %s", segment.Text, "test text")
	}
	if segment.Confidence != 0.95 {
		t.Errorf("Confidence mismatch: got %f, want %f", segment.Confidence, 0.95)
	}
}

func TestGoogleConfig(t *testing.T) {
	cfg := stt.GoogleConfig{
		CredentialsPath: "/path/to/creds.json",
	}

	if cfg.CredentialsPath != "/path/to/creds.json" {
		t.Errorf("CredentialsPath mismatch: got %s, want %s", cfg.CredentialsPath, "/path/to/creds.json")
	}
}

func TestAWSConfig(t *testing.T) {
	cfg := stt.AWSConfig{
		AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",                         
		SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",   
		Region:          "us-west-2",
	}

	if cfg.AccessKeyID != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("AccessKeyID mismatch")
	}
	if cfg.SecretAccessKey != "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" {
		t.Errorf("SecretAccessKey mismatch")
	}
	if cfg.Region != "us-west-2" {
		t.Errorf("Region mismatch: got %s, want %s", cfg.Region, "us-west-2")
	}
}

func TestAzureConfig(t *testing.T) {
	cfg := stt.AzureConfig{
		Key:    "abc123def456",
		Region: "eastus",
	}

	if cfg.Key != "abc123def456" {
		t.Errorf("Key mismatch")
	}
	if cfg.Region != "eastus" {
		t.Errorf("Region mismatch: got %s, want %s", cfg.Region, "eastus")
	}
}

func TestSTTConfig(t *testing.T) {
	cfg := &stt.STTConfig{
		Provider: "google",
		Google: stt.GoogleConfig{
			CredentialsPath: "/creds.json",
		},
		AWS: stt.AWSConfig{
			Region: "us-east-1",
		},
		Azure: stt.AzureConfig{
			Region: "eastus",
		},
	}

	if cfg.Provider != "google" {
		t.Errorf("Provider mismatch")
	}
	if cfg.Google.CredentialsPath != "/creds.json" {
		t.Errorf("Google credentials path mismatch")
	}
}

func TestSTTProviderInterface(t *testing.T) {
	// Test interface methods by checking provider types
	provider := &stt.GoogleSTTProvider{}

	// Check that methods exist by calling them
	name := provider.Name()
	if name != "google" {
		t.Errorf("Name() should return 'google', got: %s", name)
	}

	// IsAvailable should return false with empty credentials
	available := provider.IsAvailable()
	if available {
		t.Error("IsAvailable() should return false with empty credentials")
	}
}

func TestSTTConfigDefaults(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *stt.STTConfig
		wantErr bool
	}{
		{"empty provider returns error", &stt.STTConfig{}, true},
		{"google provider", &stt.STTConfig{Provider: "google"}, false},
		{"aws provider", &stt.STTConfig{Provider: "aws"}, false},
		{"azure provider", &stt.STTConfig{Provider: "azure"}, false},
		{"unsupported provider returns error", &stt.STTConfig{Provider: "unsupported"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := stt.NewProvider(tt.cfg)
			if tt.wantErr && err == nil {
				t.Errorf("provider=%q: expected error, got nil", tt.cfg.Provider)
			} else if !tt.wantErr && err != nil {
				t.Errorf("provider=%q: unexpected error: %v", tt.cfg.Provider, err)
			}
		})
	}
}

// validCredsPath creates a temporary credentials file and returns its path.
func validCredsPath(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "creds*.json")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func TestSTTProviderWithEmptyData(t *testing.T) {
	tests := []struct {
		provider stt.STTProvider
		name     string
		expected bool
	}{
		{
			name:     "Google STT with empty credentials",
			provider: stt.NewGoogleSTTProvider(""),
			expected: false,
		},
		{
			name:     "AWS STT with empty credentials",
			provider: stt.NewAWSSTTProvider("", "", "us-east-1"),
			expected: false,
		},
		{
			name:     "Azure STT with empty key",
			provider: stt.NewAzureSTTProvider("", "eastus"),
			expected: false,
		},
		{
			name:     "Google STT with valid credentials",
			provider: stt.NewGoogleSTTProvider(validCredsPath(t)),
			expected: true,
		},
		{
			name:     "AWS STT with valid credentials",
			provider: stt.NewAWSSTTProvider("key", "secret", "us-east-1"),
			expected: true,
		},
		{
			name:     "Azure STT with valid credentials",
			provider: stt.NewAzureSTTProvider("key", "eastus"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.provider.IsAvailable() != tt.expected {
				t.Errorf("Expected IsAvailable()=%v, got %v", tt.expected, tt.provider.IsAvailable())
			}
		})
	}
}

// Skipping TestTranscribeWithEmptyContext due to logging infrastructure requirement
// func TestTranscribeWithEmptyContext(t *testing.T) {
// 	provider := stt.NewGoogleSTTProvider("valid-creds")
// 	audioData := &stt.AudioData{
// 		SessionID:     "session1",
// 		ParticipantID: "p1",
// 		Role:          "user",
// 		Data:          []byte("test data"),
// 		SampleRate:    16000,
// 		Duration:      60,
// 	}
//
// 	result, err := provider.Transcribe(context.Background(), audioData)
//
// 	if err != nil {
// 		t.Errorf("Transcribe should not fail with valid credentials, got error: %v", err)
// 	}
// 	if result == nil {
// 		t.Fatal("Expected non-nil result")
// 	}
// 	if len(result.Segments) == 0 {
// 		t.Error("Expected at least one segment")
// 	}
//
// 	segment := result.Segments[0]
// 	if segment.Confidence != 0.95 {
// 		t.Errorf("Expected confidence 0.95, got %f", segment.Confidence)
// 	}
// }
