package stt_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Josepavese/aftertalk/internal/ai/stt"
	"github.com/Josepavese/aftertalk/internal/logging"
)

func init() {
	logging.Init("info", "console") //nolint:errcheck
}

// ── Google ────────────────────────────────────────────────────────────────────

func TestGoogleSTTProvider_Name(t *testing.T) {
	if stt.NewGoogleSTTProvider("creds.json").Name() != "google" {
		t.Error("expected name 'google'")
	}
}

func TestGoogleSTTProvider_IsAvailable(t *testing.T) {
	tmp := t.TempDir()
	credPath := filepath.Join(tmp, "creds.json")
	os.WriteFile(credPath, []byte(`{}`), 0600) //nolint:errcheck

	if !stt.NewGoogleSTTProvider(credPath).IsAvailable() {
		t.Error("should be available when credentials file exists")
	}
	if stt.NewGoogleSTTProvider("/nonexistent/path.json").IsAvailable() {
		t.Error("should not be available when credentials file is missing")
	}
	if stt.NewGoogleSTTProvider("").IsAvailable() {
		t.Error("should not be available with empty path")
	}
}

func TestGoogleSTTProvider_Transcribe(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"access_token": "fake"})
	})
	mux.HandleFunc("/speech:recognize", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": []map[string]interface{}{{
				"alternatives": []map[string]interface{}{{
					"transcript": "ciao come stai", "confidence": 0.97,
				}},
			}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tmp := t.TempDir()
	credPath := filepath.Join(tmp, "sa.json")
	sa := map[string]string{
		"type": "service_account", "client_email": "test@proj.iam.gserviceaccount.com",
		"private_key": fakeRSAPrivateKeyPEM, "token_uri": srv.URL + "/token",
	}
	b, _ := json.Marshal(sa)
	os.WriteFile(credPath, b, 0600) //nolint:errcheck

	p := stt.NewGoogleSTTProvider(credPath)
	p.SetSpeechEndpoint(srv.URL + "/speech:recognize")

	result, err := p.Transcribe(context.Background(), &stt.AudioData{
		SessionID: "s1", Role: "therapist", Duration: 3000,
	})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if len(result.Segments) != 1 || result.Segments[0].Text != "ciao come stai" {
		t.Errorf("unexpected result: %+v", result)
	}
}

// ── AWS ───────────────────────────────────────────────────────────────────────

func TestAWSSTTProvider_Name(t *testing.T) {
	if stt.NewAWSSTTProvider("k", "s", "eu-west-1").Name() != "aws" {
		t.Error("expected name 'aws'")
	}
}

func TestAWSSTTProvider_IsAvailable(t *testing.T) {
	cases := []struct{ access, secret, region string; want bool }{
		{"AKID", "secret", "eu-west-1", true},
		{"", "secret", "eu-west-1", false},
		{"AKID", "", "eu-west-1", false},
		{"AKID", "secret", "", false},
	}
	for _, c := range cases {
		if stt.NewAWSSTTProvider(c.access, c.secret, c.region).IsAvailable() != c.want {
			t.Errorf("IsAvailable(%q,%q,%q): want %v", c.access, c.secret, c.region, c.want)
		}
	}
}

func TestAWSSTTProvider_EmptyCredentials(t *testing.T) {
	_, err := stt.NewAWSSTTProvider("", "", "").Transcribe(
		context.Background(), &stt.AudioData{SessionID: "s", Duration: 30})
	if err == nil {
		t.Error("expected error with empty credentials")
	}
}

func TestAWSSTTProvider_Transcribe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"results": map[string]interface{}{
				"transcripts": []map[string]string{{"transcript": "test transcription"}},
				"items": []map[string]interface{}{{
					"type": "pronunciation", "start_time": "0.0", "end_time": "2.5",
					"alternatives": []map[string]string{{"content": "test", "confidence": "0.9"}},
				}},
			},
		})
	}))
	defer srv.Close()

	p := stt.NewAWSSTTProvider("AKID", "secret", "eu-west-1")
	p.SetEndpoint(srv.URL)

	result, err := p.Transcribe(context.Background(), &stt.AudioData{
		SessionID: "s1", Role: "moderator", Duration: 3000,
	})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if len(result.Segments) != 1 || result.Segments[0].Text != "test transcription" {
		t.Errorf("unexpected result: %+v", result)
	}
}

// ── Azure ─────────────────────────────────────────────────────────────────────

func TestAzureSTTProvider_Name(t *testing.T) {
	if stt.NewAzureSTTProvider("key", "eastus").Name() != "azure" {
		t.Error("expected name 'azure'")
	}
}

func TestAzureSTTProvider_IsAvailable(t *testing.T) {
	cases := []struct{ key, region string; want bool }{
		{"key123", "eastus", true},
		{"", "eastus", false},
		{"key123", "", false},
	}
	for _, c := range cases {
		if stt.NewAzureSTTProvider(c.key, c.region).IsAvailable() != c.want {
			t.Errorf("IsAvailable(%q,%q): want %v", c.key, c.region, c.want)
		}
	}
}

func TestAzureSTTProvider_EmptyCredentials(t *testing.T) {
	if stt.NewAzureSTTProvider("", "").IsAvailable() {
		t.Error("should not be available with empty credentials")
	}
}

func TestAzureSTTProvider_Transcribe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"RecognitionStatus": "Success",
			"DisplayText":       "sessione terapeutica",
			"Duration":          30_000_000, "Offset": 0,
			"NBest": []map[string]interface{}{
				{"Confidence": 0.95, "Display": "sessione terapeutica", "Words": []interface{}{}},
			},
		})
	}))
	defer srv.Close()

	p := stt.NewAzureSTTProvider("key123", "eastus")
	p.SetEndpoint(srv.URL)

	result, err := p.Transcribe(context.Background(), &stt.AudioData{
		SessionID: "s1", Role: "therapist", Duration: 3000,
	})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if len(result.Segments) != 1 || result.Segments[0].Text != "sessione terapeutica" {
		t.Errorf("unexpected result: %+v", result)
	}
}

// ── Factory ───────────────────────────────────────────────────────────────────

func TestSTTNewProviderFactory(t *testing.T) {
	cases := []struct{ provider string; wantErr bool }{
		{"stub", false}, {"", false}, {"unsupported", true},
	}
	for _, c := range cases {
		p, err := stt.NewProvider(&stt.STTConfig{Provider: c.provider})
		if c.wantErr {
			if err == nil {
				t.Errorf("provider=%q: expected error", c.provider)
			}
		} else if err != nil || p == nil {
			t.Errorf("provider=%q: unexpected error or nil provider: %v", c.provider, err)
		}
	}
}

// fakeRSAPrivateKeyPEM is a test-only RSA key — NOT used for real authentication.
const fakeRSAPrivateKeyPEM = `-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQCc5OC7RLJP+1Mw
R5pwv2gWwv6W/OHX453T5FKflpbCqrauzyJ7Yk9yMXXU8i/PzM6kBEFDMtg5I8me
l5XfC14fd0oCVLvOB06ApvCyijAaazyqCcGa9KQr9dh37rQ490OJiaPm7N+ojrkU
1MzW/e9f449vW4xQt6rCHTUOmC/YNrf5fMyUNKqydCNVOeX4SvXkcCQwW2jspfJA
z+u8noX78BUbreLKFPYoiWXFg/rPXV1bHKuRXdBlr9MqoUcO5e/Jw/mfnh8ajrZt
of4U3h3nMurU41iHb3XHzGfZhyhKzdWvJM6gQtqI9NH5uSPoUz2674lbdruLWwC7
ddoDqAKdAgMBAAECggEABNB1IOnzusaQf+vCjnEhJYmoPEPYPkKqxiS8cE8zoxeP
8X9DpJuYqn1gCz+/PdYgBSJoSkKWJfK2Lhqiq6xyn+6OI9IrzR+mRgZZXnElFrpx
qxoPicy1+O9bTBrUBud3eBH0KJLeLhLrFPuOqY4zOTMHZLhfbt6j677vsNn0peLD
p0+yl5APfmwnxd9hzSkpdUXf+OlfvqnN9Ldki2+7k7ow0y1lcFpvBhURR24tkdCk
t2cZGLCNHutepswqmC27yjsn/kyWSZswZfJNnlU9KcOt03rztkVHYOvyQ4wH4lqd
TUlRx4/PHMxI1ZEZOYNiTiVReujs3D5ZdrTij7uuoQKBgQDL62usj46oHGZjy9et
YUnVN/TsjuSFOc62KcxnarrIbE1EP6PpmY50Dyk6LSH0k3ZtTmeoL122sCowCiW9
uV7xRXzbmy1mj/YainS6orTjL2q6/ip1Yizh8Tb7qIPfHOjcRZJgfI5s7bTDMU3Z
4pDufnmzN9VngyVuaF98ItW5xQKBgQDE9tqGlrSkczfcDFsBp47cYrt49nTled4o
SsfHrukV2T6wGLubBQFN98HDZozspcnYfkg8UBiQ7dp5OVRHOLc+riqS0hU+sNpL
i0AowmDL1tz9JzQ3Y8P1c6FdfTpaD6kv6MrxZpbvoS7KskcJBY3ilPtLWnHf6cIF
ROmmfNwq+QKBgA9L1YPYMOdDWhraS49h4Nvxmpm0DkhAEdVwRTjstJ4cIZ+g9nar
YhgqmvkWMZnbBeMlInlnNCxkAoYf/LzCjvCiOb9vYHR1EAzlnePyGIeCIwtrzVuI
xb0dDvbJqTqvPHhpb5V1QmnBWvHZXPGfISgCrLZY1dUx7Tje82qoYkfRAoGABT9W
XxOQyHjRWilyGz8tjS2MNRLL1nlCs+waGnXMe+qHwwVFqkGd4Ufif6QxyPQ5xmzG
2+R+Yw4TLfubBTK7nw3g0HyMWFk5151kHjHfhk65IH105KzhwZ5NBEKb1V5pcX9Q
ONI03zl6F6hcQB9HwmuZrk5AjmiZ5K4LU4YsD3ECgYBP2tetAhEyATixQqGoKKO9
GkCBG3gC0iXWVnT/4zAOhJOMtvuVQXlDhenEaD7e2iE7LSQ6IlBiiu5wDJtqL2HA
pkn/lcwvtDFJyRrzD7Dy4qDzI9JJbhFAXm6H2diiC/1rTWYYo6Nw5MfsGOeVWAn/
+uwhtfZzAuQnOUfs9QAEzQ==
-----END PRIVATE KEY-----`
