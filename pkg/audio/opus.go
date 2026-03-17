package audio

import (
	"encoding/binary"
	"errors"
	"fmt"

	kazopus  "github.com/kazzmir/opus-go/opus"
	concentus "github.com/lostromb/concentus/go/opus"
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
// 16-bit little-endian WAV file at the requested sampleRate (typically 16 kHz).
//
// Uses concentus (pure-Go, full CELT+SILK support) to decode Chrome WebRTC
// Opus frames. Decodes at 48 kHz then downsamples to sampleRate.
func DecodeFramesToWAV(frames [][]byte, sampleRate int) ([]byte, error) {
	dec, err := concentus.NewOpusDecoder(48000, 1)
	if err != nil {
		return nil, fmt.Errorf("opus: create decoder: %w", err)
	}

	// Max Opus frame = 120 ms at 48 kHz = 5760 samples.
	pcmBuf := make([]int16, 5760)
	// Collect 48 kHz samples, then downsample.
	allPCM48k := make([]int16, 0, len(frames)*960)

	for _, frame := range frames {
		if len(frame) == 0 {
			continue
		}
		n, decErr := dec.Decode(frame, 0, len(frame), pcmBuf, 0, 5760, false)
		if decErr != nil || n == 0 {
			continue
		}
		allPCM48k = append(allPCM48k, pcmBuf[:n]...)
	}

	// Downsample 48 kHz → sampleRate by taking every Nth sample.
	ratio := 48000 / sampleRate
	allPCM := make([]int16, 0, len(allPCM48k)/ratio)
	for i := 0; i < len(allPCM48k); i += ratio {
		allPCM = append(allPCM, allPCM48k[i])
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
