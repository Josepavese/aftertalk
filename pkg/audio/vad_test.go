package audio

import (
	"testing"
)

// Frame factory helpers
func silentFrame() []byte     { return make([]byte, 3) }   // 3 bytes  → absolute silence
func speechFrame() []byte     { return make([]byte, 80) }  // 80 bytes → normal speech
func noisyFrame(n int) []byte { return make([]byte, n) }   // arbitrary size for noise simulation

// --- IsOpusSilentFrame ---

func TestIsOpusSilentFrame(t *testing.T) {
	if !IsOpusSilentFrame(silentFrame()) {
		t.Error("small frame should be silent")
	}
	if IsOpusSilentFrame(speechFrame()) {
		t.Error("large frame should not be silent")
	}
	// Boundary: exactly SilenceThresholdBytes-1 → silent, SilenceThresholdBytes → not
	if !IsOpusSilentFrame(make([]byte, SilenceThresholdBytes-1)) {
		t.Error("frame at threshold-1 should be silent")
	}
	if IsOpusSilentFrame(make([]byte, SilenceThresholdBytes)) {
		t.Error("frame at threshold should not be silent")
	}
}

// --- ShouldFlushOnSilence: quiet environment ---

func TestShouldFlushOnSilence_tooEarly(t *testing.T) {
	frames := repeatFrames(speechFrame, 100)
	if ShouldFlushOnSilence(frames, 5000) {
		t.Error("should not flush before MinChunkMs")
	}
}

func TestShouldFlushOnSilence_hardCutoff(t *testing.T) {
	frames := repeatFrames(speechFrame, 1000) // all speech, no silence
	if !ShouldFlushOnSilence(frames, MaxChunkMs) {
		t.Error("should flush at MaxChunkMs even without silence")
	}
}

func TestShouldFlushOnSilence_silenceAfterMin(t *testing.T) {
	// 500 speech frames + SilenceWindowFrames silent frames past MinChunkMs
	frames := append(repeatFrames(speechFrame, 500), repeatFrames(silentFrame, SilenceWindowFrames)...)
	if !ShouldFlushOnSilence(frames, MinChunkMs+100) {
		t.Error("should flush after MinChunkMs + trailing silence")
	}
}

func TestShouldFlushOnSilence_partialSilence(t *testing.T) {
	// One frame short of the required silence window → must NOT flush
	frames := append(repeatFrames(speechFrame, 500), repeatFrames(silentFrame, SilenceWindowFrames-1)...)
	if ShouldFlushOnSilence(frames, MinChunkMs+100) {
		t.Error("should not flush with insufficient trailing silence")
	}
}

func TestShouldFlushOnSilence_speechResumes(t *testing.T) {
	// Full silence window broken by a speech frame at the very end
	frames := append(repeatFrames(speechFrame, 500), repeatFrames(silentFrame, SilenceWindowFrames)...)
	frames = append(frames, speechFrame()) // speech resumes
	if ShouldFlushOnSilence(frames, MinChunkMs+100) {
		t.Error("should not flush when speech resumes after silence")
	}
}

// --- Noisy environment: adaptive threshold ---

func TestShouldFlushOnSilence_noisyRoom_noFlushDuringSpeech(t *testing.T) {
	// Scenario: noisy café. Background noise = 40 bytes/frame.
	// Speaker talking = 90 bytes/frame. No silence window → should NOT flush.
	bgNoise := 40
	speech := 90
	var frames [][]byte
	for i := 0; i < 400; i++ {
		frames = append(frames, noisyFrame(bgNoise+speech/2)) // mixed
	}
	if ShouldFlushOnSilence(frames, MinChunkMs+100) {
		t.Error("should not flush during continuous noisy speech")
	}
}

func TestShouldFlushOnSilence_noisyRoom_flushOnPause(t *testing.T) {
	// Scenario: noisy room. Speech = 100 bytes/frame. During pause = 30 bytes/frame.
	// Adaptive threshold ≈ 100 × 0.45 = 45. Pause frames (30) < 45 → silence.
	speechSize := 100
	pauseSize := 30 // ~30% of speech → below NoiseRatio threshold

	var frames [][]byte
	// 500 speech frames to establish baseline
	for i := 0; i < 500; i++ {
		frames = append(frames, noisyFrame(speechSize))
	}
	// SilenceWindowFrames pause frames
	for i := 0; i < SilenceWindowFrames; i++ {
		frames = append(frames, noisyFrame(pauseSize))
	}

	if !ShouldFlushOnSilence(frames, MinChunkMs+100) {
		t.Error("should flush after pause in noisy environment (adaptive threshold)")
	}
}

func TestShouldFlushOnSilence_noisyRoom_noisyPauseNotFlushed(t *testing.T) {
	// Scenario: background noise = 40 bytes. Speaker pauses but noise continues at 38 bytes.
	// 38/40 = 95% of baseline → NOT a significant drop → should NOT flush.
	speechSize := 40
	noisePauseSize := 38 // barely below baseline

	var frames [][]byte
	for i := 0; i < 500; i++ {
		frames = append(frames, noisyFrame(speechSize))
	}
	for i := 0; i < SilenceWindowFrames; i++ {
		frames = append(frames, noisyFrame(noisePauseSize))
	}

	// threshold ≈ 40 × 0.45 = 18. noisePauseSize=38 > 18 → not silent.
	if ShouldFlushOnSilence(frames, MinChunkMs+100) {
		t.Error("should not flush when noise-only pause is close to speech level")
	}
}

func TestAdaptiveThreshold_quietRoom(t *testing.T) {
	// In a quiet room, rolling avg is small → adaptive threshold = SilenceThresholdBytes
	frames := repeatFrames(func() []byte { return make([]byte, 5) }, 100)
	threshold := adaptiveThreshold(frames)
	if threshold != SilenceThresholdBytes {
		t.Errorf("quiet room: expected threshold=%d, got %d", SilenceThresholdBytes, threshold)
	}
}

func TestAdaptiveThreshold_noisyRoom(t *testing.T) {
	// Noisy room: rolling avg = 80 bytes → adaptive threshold = 80×0.45 = 36
	frames := repeatFrames(func() []byte { return make([]byte, 80) }, 100)
	threshold := adaptiveThreshold(frames)
	expected := int(80 * NoiseRatio) // 36
	if threshold != expected {
		t.Errorf("noisy room: expected threshold=%d, got %d", expected, threshold)
	}
}

// repeatFrames builds a slice of n frames using factory fn.
func repeatFrames(fn func() []byte, n int) [][]byte {
	out := make([][]byte, n)
	for i := range out {
		out[i] = fn()
	}
	return out
}
