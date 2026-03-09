package audio

import (
	"fmt"
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
	return nil, fmt.Errorf("opus decoding requires external library - use github.com/hraban/opus")
}

type OpusEncoder struct {
	sampleRate int
	channels   int
	bitrate    int
}

func NewOpusEncoder(sampleRate, channels, bitrate int) *OpusEncoder {
	return &OpusEncoder{
		sampleRate: sampleRate,
		channels:   channels,
		bitrate:    bitrate,
	}
}

func (e *OpusEncoder) Encode(pcmData []int16) ([]byte, error) {
	return nil, fmt.Errorf("opus encoding requires external library - use github.com/hraban/opus")
}

func ValidateOpusFrame(frame []byte) error {
	if len(frame) == 0 {
		return fmt.Errorf("empty opus frame")
	}

	if len(frame) > 4000 {
		return fmt.Errorf("opus frame too large: %d bytes", len(frame))
	}

	return nil
}

func GetOpusFrameDuration(frameSize int, sampleRate int) int {
	if sampleRate == 0 {
		return 0
	}
	return (frameSize * 1000) / sampleRate
}
