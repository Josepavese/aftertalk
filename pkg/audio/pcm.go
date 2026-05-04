package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

const (
	SampleRate = 48000
	Channels   = 1
)

type PCMConverter struct {
	sampleRate int
	channels   int
}

func NewPCMConverter(sampleRate, channels int) *PCMConverter {
	return &PCMConverter{
		sampleRate: sampleRate,
		channels:   channels,
	}
}

func (c *PCMConverter) ConvertToFloat32(pcmData []int16) []float32 {
	floats := make([]float32, len(pcmData))
	for i, sample := range pcmData {
		floats[i] = float32(sample) / 32768.0
	}
	return floats
}

func (c *PCMConverter) ConvertToInt16(floatData []float32) []int16 {
	ints := make([]int16, len(floatData))
	for i, sample := range floatData {
		scaled := float64(sample) * 32767.0
		scaled = math.Max(-32767, math.Min(32767, scaled))
		ints[i] = int16(scaled)
	}
	return ints
}

func ReadPCM(r io.Reader) ([]int16, error) {
	var samples []int16
	buf := make([]byte, 2)

	for {
		_, err := io.ReadFull(r, buf)
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				break
			}
			return nil, fmt.Errorf("failed to read PCM data: %w", err)
		}

		sample := int16(binary.LittleEndian.Uint16(buf)) //nolint:gosec // intentional uint16->int16 reinterpret for PCM decoding
		samples = append(samples, sample)
	}

	return samples, nil
}

func WritePCM(w io.Writer, samples []int16) error {
	buf := new(bytes.Buffer)

	for _, sample := range samples {
		if err := binary.Write(buf, binary.LittleEndian, sample); err != nil {
			return fmt.Errorf("failed to write PCM data: %w", err)
		}
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func ChunkPCM(samples []int16, chunkSizeMs, sampleRate int) [][]int16 {
	samplesPerChunk := (sampleRate * chunkSizeMs) / 1000
	if samplesPerChunk <= 0 {
		return nil
	}

	var chunks [][]int16
	for i := 0; i < len(samples); i += samplesPerChunk {
		end := i + samplesPerChunk
		if end > len(samples) {
			end = len(samples)
		}
		chunks = append(chunks, samples[i:end])
	}

	return chunks
}

func MergePCM(chunks [][]int16) []int16 {
	var totalLen int
	for _, chunk := range chunks {
		totalLen += len(chunk)
	}

	merged := make([]int16, 0, totalLen)
	for _, chunk := range chunks {
		merged = append(merged, chunk...)
	}

	return merged
}
