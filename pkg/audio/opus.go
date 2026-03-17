package audio

import (
	"encoding/binary"
	"errors"
	"fmt"

	kazopus "github.com/kazzmir/opus-go/opus"
)

var (
	errOpusEmptyFrame    = errors.New("empty opus frame")
	errOpusFrameTooLarge = errors.New("opus frame too large")
)

const (
	OpusSampleRate = 48000
	OpusChannels   = 1
	OpusFrameSize  = 960
)

type OpusDecoder struct {
	sampleRate int
	channels   int
}

func NewOpusDecoder(sampleRate, channels int) *OpusDecoder {
	return &OpusDecoder{
		sampleRate: sampleRate,
		channels:   channels,
	}
}

func (d *OpusDecoder) Decode(opusData []byte) ([]int16, error) {
	dec, err := kazopus.NewDecoder(d.sampleRate, d.channels)
	if err != nil {
		return nil, fmt.Errorf("opus: create decoder: %w", err)
	}
	pcm := make([]int16, OpusFrameSize*d.channels)
	n, err := dec.Decode(opusData, pcm, OpusFrameSize, false)
	if err != nil {
		return nil, fmt.Errorf("opus: decode: %w", err)
	}
	return pcm[:n*d.channels], nil
}

// DecodeFramesToWAV decodes a slice of raw Opus RTP payloads and returns a
// 16-bit little-endian WAV file at the given sample rate.
func DecodeFramesToWAV(frames [][]byte, sampleRate int) ([]byte, error) {
	dec, err := kazopus.NewDecoder(48000, 1) // Opus RTP is always 48 kHz mono
	if err != nil {
		return nil, fmt.Errorf("opus: create decoder: %w", err)
	}

	var allPCM []int16
	framePCM := make([]int16, OpusFrameSize)
	for _, frame := range frames {
		if len(frame) == 0 {
			continue
		}
		n, err := dec.Decode(frame, framePCM, OpusFrameSize, false)
		if err != nil {
			continue // skip bad frames
		}
		allPCM = append(allPCM, framePCM[:n]...)
	}

	// Downsample 48000 → 16000 using a box filter (average 3 consecutive samples).
	// This suppresses high-frequency content above 8 kHz before decimation,
	// avoiding aliasing artefacts that degrade STT accuracy.
	if sampleRate == 16000 {
		n := len(allPCM) / 3
		downsampled := make([]int16, n)
		for i := range downsampled {
			avg := (int32(allPCM[i*3]) + int32(allPCM[i*3+1]) + int32(allPCM[i*3+2])) / 3
			downsampled[i] = int16(avg) //nolint:gosec // clamped by division; value fits int16
		}
		allPCM = downsampled
	}

	return encodeWAV(allPCM, sampleRate), nil
}

// encodeWAV builds a minimal PCM WAV file from int16 samples.
func encodeWAV(samples []int16, sampleRate int) []byte {
	dataLen := len(samples) * 2
	buf := make([]byte, 44+dataLen)

	copy(buf[0:4], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(36+dataLen)) //nolint:gosec // dataLen is always non-negative
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16)          // chunk size
	binary.LittleEndian.PutUint16(buf[20:], 1)           // PCM
	binary.LittleEndian.PutUint16(buf[22:], 1)           // mono
	binary.LittleEndian.PutUint32(buf[24:], uint32(sampleRate))        //nolint:gosec // sampleRate is always positive
	binary.LittleEndian.PutUint32(buf[28:], uint32(sampleRate*2))      //nolint:gosec // sampleRate is always positive
	binary.LittleEndian.PutUint16(buf[32:], 2)           // block align
	binary.LittleEndian.PutUint16(buf[34:], 16)          // bits per sample
	copy(buf[36:40], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(dataLen)) //nolint:gosec // dataLen is always non-negative
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[44+i*2:], uint16(s)) //nolint:gosec // intentional int16->uint16 reinterpret for WAV encoding
	}
	return buf
}


func ValidateOpusFrame(frame []byte) error {
	if len(frame) == 0 {
		return errOpusEmptyFrame
	}

	if len(frame) > 4000 {
		return fmt.Errorf("%w: %d bytes", errOpusFrameTooLarge, len(frame))
	}

	return nil
}

func GetOpusFrameDuration(frameSize int, sampleRate int) int {
	if sampleRate == 0 {
		return 0
	}
	return (frameSize * 1000) / sampleRate
}
