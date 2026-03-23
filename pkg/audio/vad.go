package audio

// VAD (Voice Activity Detection) for Opus RTP frames.
//
// # How it works
//
// Opus is a perceptual codec: it allocates more bytes to complex audio (speech)
// and very few bytes to silence or comfort noise. We use frame size as a proxy
// for audio energy without decoding the Opus bitstream.
//
// Two-stage detection handles both quiet and noisy environments:
//
//  1. Absolute threshold (quiet room): frame < SilenceThresholdBytes → silence.
//     Works when Chrome sends tiny comfort-noise frames (typically 1–8 bytes).
//
//  2. Relative threshold (noisy room): frame < BaselineLookback-frame-avg × NoiseRatio.
//     Detects pauses relative to recent background noise level. A microphone
//     in a busy café has a high baseline (~40 bytes/frame); during a speaker
//     pause the frames drop to ~noise-only level (~20 bytes). The relative
//     drop triggers silence even though the absolute level is not "quiet".
//
// Typical Opus frame sizes at 20ms/frame, 48kHz:
//
//	Silence / DTX comfort noise  :   1–8  bytes
//	Whispered speech             :  10–30 bytes
//	Normal speech                :  40–120 bytes
//	Speech + background noise    :  60–150 bytes
//	Background noise only        :  15–50 bytes  ← relative drop from speech

const (
	// SilenceThresholdBytes is the absolute lower bound: frames smaller than
	// this are always considered silent, regardless of environment.
	SilenceThresholdBytes = 10

	// SilenceWindowFrames is the number of consecutive silent frames required
	// to declare a speech boundary (~300 ms at 20 ms/frame).
	SilenceWindowFrames = 15

	// BaselineLookbackFrames is the number of recent frames used to compute
	// the rolling average for adaptive noise floor estimation (~2 s).
	BaselineLookbackFrames = 100

	// NoiseRatio: a frame whose size is below (rolling_avg × NoiseRatio) is
	// treated as a relative silence even in a noisy environment.
	// 0.45 means "40% quieter than recent average" = speaker stopped talking.
	NoiseRatio = 0.45

	// MinChunkMs: minimum buffer duration before a VAD-triggered flush.
	MinChunkMs = 10_000

	// MaxChunkMs: hard upper limit — flush unconditionally regardless of VAD.
	MaxChunkMs = 20_000
)

// IsOpusSilentFrame reports whether a single Opus RTP payload is silence
// using only the absolute threshold. Suitable for quiet environments.
func IsOpusSilentFrame(frame []byte) bool {
	return len(frame) < SilenceThresholdBytes
}

// adaptiveThreshold computes the silence threshold for the current noise floor.
func adaptiveThreshold(recentFrames [][]byte) int {
	n := len(recentFrames)
	if n > BaselineLookbackFrames {
		n = BaselineLookbackFrames
	}
	if n == 0 {
		return SilenceThresholdBytes
	}

	tail := recentFrames[len(recentFrames)-n:]
	var total int
	for _, f := range tail {
		total += len(f)
	}
	avg := total / n

	// Relative threshold: NoiseRatio × rolling average.
	relative := int(float64(avg) * NoiseRatio)

	// Always use the larger of absolute and relative thresholds.
	if relative > SilenceThresholdBytes {
		return relative
	}
	return SilenceThresholdBytes
}

// ShouldFlushOnSilence returns true when the buffer should be flushed to
// the transcription pipeline.
//
//   - Returns false if durationMs < MinChunkMs (too early to cut).
//   - Returns true  if durationMs >= MaxChunkMs (hard safety cutoff).
//   - Returns true  if the last SilenceWindowFrames frames are all silent
//     using the adaptive two-stage detector (handles noisy environments).
func ShouldFlushOnSilence(frames [][]byte, durationMs int) bool {
	if durationMs >= MaxChunkMs {
		return true
	}
	if durationMs < MinChunkMs {
		return false
	}
	return hasAdaptiveTrailingSilence(frames, SilenceWindowFrames)
}

// hasAdaptiveTrailingSilence checks whether the last n frames are all silent
// using the adaptive threshold computed from the full frame history.
func hasAdaptiveTrailingSilence(frames [][]byte, n int) bool {
	if len(frames) < n {
		return false
	}

	// Compute baseline from frames before the tail window so that the tail
	// itself doesn't inflate or deflate the noise floor estimate.
	baseline := frames
	if len(frames) > n {
		baseline = frames[:len(frames)-n]
	}
	threshold := adaptiveThreshold(baseline)

	tail := frames[len(frames)-n:]
	for _, f := range tail {
		if len(f) >= threshold {
			return false // frame is above threshold → speech or loud noise
		}
	}
	return true
}
