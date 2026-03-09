package audio

import (
	"bytes"
	"testing"
)

func TestWriteOGGOpus_structure(t *testing.T) {
	// Minimal fake Opus frame (1 byte TOC header + payload)
	fakeFrame := []byte{0x78, 0x01, 0x02, 0x03, 0x04}
	frames := [][]byte{fakeFrame, fakeFrame, fakeFrame}

	data, err := WriteOGGOpus(frames, 48000, 1)
	if err != nil {
		t.Fatalf("WriteOGGOpus error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty OGG output")
	}

	// Every OGG page starts with "OggS"
	if !bytes.Contains(data, []byte("OggS")) {
		t.Error("output does not contain OGG capture pattern")
	}

	// Must contain OpusHead and OpusTags
	if !bytes.Contains(data, []byte("OpusHead")) {
		t.Error("output missing OpusHead")
	}
	if !bytes.Contains(data, []byte("OpusTags")) {
		t.Error("output missing OpusTags")
	}
}

func TestWriteOGGOpus_empty(t *testing.T) {
	data, err := WriteOGGOpus(nil, 48000, 1)
	if err != nil {
		t.Fatalf("unexpected error on empty frames: %v", err)
	}
	// Should still produce header pages
	if !bytes.Contains(data, []byte("OpusHead")) {
		t.Error("output missing OpusHead for empty frames")
	}
}

func TestWriteOGGOpus_defaults(t *testing.T) {
	// Zero sampleRate and channels should use defaults
	data, err := WriteOGGOpus([][]byte{{0x78, 0x00}}, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty output")
	}
}
