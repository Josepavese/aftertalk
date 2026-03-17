package audio

import (
	"encoding/binary"
	"math"
	"testing"

	kazopus "github.com/kazzmir/opus-go/opus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// generateOpusFrames creates real Opus-encoded frames from a synthetic sine wave.
// sampleRate must be 48000 (Opus RTP requirement), frameSize = 960 (20ms).
func generateOpusFrames(t *testing.T, numFrames int) [][]byte {
	t.Helper()
	const (
		sampleRate = 48000
		channels   = 1
		frameSize  = 960 // 20ms at 48kHz
		appVOIP    = 2048
	)

	enc, err := kazopus.NewEncoder(sampleRate, channels, appVOIP)
	require.NoError(t, err, "create opus encoder")
	defer enc.Close()

	// 440 Hz sine wave
	pcm := make([]int16, frameSize)
	frames := make([][]byte, 0, numFrames)
	packet := make([]byte, 4000)

	for f := 0; f < numFrames; f++ {
		offset := f * frameSize
		for i := range pcm {
			angle := 2.0 * math.Pi * 440.0 * float64(offset+i) / sampleRate
			pcm[i] = int16(math.Sin(angle) * 16000)
		}
		n, err := enc.Encode(pcm, frameSize, packet)
		require.NoError(t, err, "encode frame %d", f)
		frame := make([]byte, n)
		copy(frame, packet[:n])
		frames = append(frames, frame)
	}
	return frames
}

func TestDecodeFramesToWAV_RealOpusFrames(t *testing.T) {
	frames := generateOpusFrames(t, 5) // 5 frames × 20ms = 100ms

	wav, err := DecodeFramesToWAV(frames, 16000)
	require.NoError(t, err)
	require.Greater(t, len(wav), 44, "WAV must be larger than header")

	// Verify RIFF header
	assert.Equal(t, []byte("RIFF"), wav[0:4], "RIFF magic")
	assert.Equal(t, []byte("WAVE"), wav[8:12], "WAVE magic")
	assert.Equal(t, []byte("fmt "), wav[12:16], "fmt chunk")
	assert.Equal(t, []byte("data"), wav[36:40], "data chunk")

	// Verify format chunk: PCM=1, mono=1, 16kHz, 16-bit
	assert.Equal(t, uint16(1), binary.LittleEndian.Uint16(wav[20:22]), "audio format PCM")
	assert.Equal(t, uint16(1), binary.LittleEndian.Uint16(wav[22:24]), "channels")
	assert.Equal(t, uint32(16000), binary.LittleEndian.Uint32(wav[24:28]), "sample rate")
	assert.Equal(t, uint16(16), binary.LittleEndian.Uint16(wav[34:36]), "bits per sample")

	// data chunk must contain non-zero audio
	dataLen := binary.LittleEndian.Uint32(wav[40:44])
	assert.Greater(t, dataLen, uint32(0), "data chunk must not be empty")
	assert.Equal(t, uint32(len(wav)-44), dataLen, "data length must match file size")

	// At least some non-zero samples (sine wave should produce signal)
	hasSignal := false
	for i := 44; i+1 < len(wav); i += 2 {
		if binary.LittleEndian.Uint16(wav[i:i+2]) != 0 {
			hasSignal = true
			break
		}
	}
	assert.True(t, hasSignal, "WAV must contain non-zero audio samples")
}

func TestDecodeFramesToWAV_48kHz_passthrough(t *testing.T) {
	frames := generateOpusFrames(t, 3)

	wav, err := DecodeFramesToWAV(frames, 48000)
	require.NoError(t, err)
	require.Greater(t, len(wav), 44)

	// sample rate in header should be 48000
	assert.Equal(t, uint32(48000), binary.LittleEndian.Uint32(wav[24:28]))

	// data should be 3× more samples than 16kHz version
	wav16, _ := DecodeFramesToWAV(frames, 16000)
	dataLen48 := binary.LittleEndian.Uint32(wav[40:44])
	dataLen16 := binary.LittleEndian.Uint32(wav16[40:44])
	assert.InDelta(t, float64(dataLen48)/float64(dataLen16), 3.0, 0.1,
		"48kHz should have ~3x the data of 16kHz after downsampling")
}

func TestDecodeFramesToWAV_EmptyFrames(t *testing.T) {
	// Empty frame list should produce a valid WAV with no audio data
	wav, err := DecodeFramesToWAV([][]byte{}, 16000)
	require.NoError(t, err)
	assert.Equal(t, []byte("RIFF"), wav[0:4])
	dataLen := binary.LittleEndian.Uint32(wav[40:44])
	assert.Equal(t, uint32(0), dataLen, "empty input → 0 data bytes")
}

func TestDecodeFramesToWAV_SkipsBadFrames(t *testing.T) {
	frames := generateOpusFrames(t, 3)
	// Inject garbage frames between real ones
	mixed := [][]byte{
		frames[0],
		{0xFF, 0xFF, 0xFF}, // garbage — decoder will skip
		frames[1],
		{},        // empty — skipped by len==0 guard
		frames[2],
	}
	wav, err := DecodeFramesToWAV(mixed, 16000)
	require.NoError(t, err)
	dataLen := binary.LittleEndian.Uint32(wav[40:44])
	assert.Greater(t, dataLen, uint32(0), "good frames should produce audio despite garbage")
}

func TestDecodeFramesToWAV_BoxFilterEdge(t *testing.T) {
	// Feed only 2 real frames (each 960 samples = 1920 total, divisible by 3 → 640 out).
	// This exercises the box filter path but stays well above the len<3 edge case.
	frames := generateOpusFrames(t, 2)
	wav, err := DecodeFramesToWAV(frames, 16000)
	require.NoError(t, err)
	dataLen := binary.LittleEndian.Uint32(wav[40:44])
	// 960*2 = 1920 input samples / 3 = 640 output samples × 2 bytes = 1280
	assert.Equal(t, uint32(1280), dataLen)
}
