package audio

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPCMConverter(t *testing.T) {
	t.Run("DefaultConfiguration", func(t *testing.T) {
		converter := NewPCMConverter(SampleRate, Channels)
		assert.NotNil(t, converter)
		assert.Equal(t, SampleRate, converter.sampleRate)
		assert.Equal(t, Channels, converter.channels)
	})

	t.Run("CustomSampleRate", func(t *testing.T) {
		converter := NewPCMConverter(44100, 2)
		assert.NotNil(t, converter)
		assert.Equal(t, 44100, converter.sampleRate)
		assert.Equal(t, 2, converter.channels)
	})

	t.Run("CustomChannels", func(t *testing.T) {
		converter := NewPCMConverter(48000, 2)
		assert.NotNil(t, converter)
		assert.Equal(t, 2, converter.channels)
	})
}

func TestPCMConverter_ConvertToFloat32(t *testing.T) {
	converter := NewPCMConverter(SampleRate, Channels)

	t.Run("PositiveSamples", func(t *testing.T) {
		samples := []int16{1000, 5000, 10000}
		floats := converter.ConvertToFloat32(samples)

		assert.Len(t, floats, len(samples))
		assert.InDelta(t, float32(1000)/32768.0, floats[0], 1e-6)
		assert.InDelta(t, float32(5000)/32768.0, floats[1], 1e-6)
		assert.InDelta(t, float32(10000)/32768.0, floats[2], 1e-6)
	})

	t.Run("NegativeSamples", func(t *testing.T) {
		samples := []int16{-1000, -5000, -10000}
		floats := converter.ConvertToFloat32(samples)

		assert.Len(t, floats, len(samples))
		assert.InDelta(t, -float32(1000)/32768.0, floats[0], 1e-6)
		assert.InDelta(t, -float32(5000)/32768.0, floats[1], 1e-6)
		assert.InDelta(t, -float32(10000)/32768.0, floats[2], 1e-6)
	})

	t.Run("ZeroSamples", func(t *testing.T) {
		samples := []int16{0, 0, 0}
		floats := converter.ConvertToFloat32(samples)

		assert.Len(t, floats, len(samples))
		assert.Equal(t, float32(0), floats[0])
		assert.Equal(t, float32(0), floats[1])
		assert.Equal(t, float32(0), floats[2])
	})

	t.Run("FullRangeSamples", func(t *testing.T) {
		samples := []int16{32767, -32768}
		floats := converter.ConvertToFloat32(samples)

		assert.Len(t, floats, len(samples))
		assert.Equal(t, float32(32767)/32768.0, floats[0])
		assert.Equal(t, float32(-32768)/32768.0, floats[1])
	})

	t.Run("EmptySamples", func(t *testing.T) {
		samples := []int16{}
		floats := converter.ConvertToFloat32(samples)

		assert.Empty(t, floats)
	})

	t.Run("SingleSample", func(t *testing.T) {
		samples := []int16{100}
		floats := converter.ConvertToFloat32(samples)

		assert.Len(t, floats, 1)
		assert.Equal(t, float32(100)/32768.0, floats[0])
	})

	t.Run("LargeNumberOfSamples", func(t *testing.T) {
		samples := make([]int16, 10000)
		for i := range samples {
			samples[i] = int16(i % 65536) //nolint:gosec // intentional wraparound for test data
		}

		floats := converter.ConvertToFloat32(samples)

		assert.Len(t, floats, 10000)
	})
}

func TestPCMConverter_ConvertToInt16(t *testing.T) {
	converter := NewPCMConverter(SampleRate, Channels)

	t.Run("PositiveFloats", func(t *testing.T) {
		floats := []float32{0.1, 0.5, 1.0}
		samples := converter.ConvertToInt16(floats)

		assert.Len(t, samples, len(floats))
		assert.InDelta(t, int16(3276), samples[0], 0.1)
		assert.InDelta(t, int16(16383), samples[1], 0.1)
		assert.Equal(t, int16(32767), samples[2])
	})

	t.Run("NegativeFloats", func(t *testing.T) {
		floats := []float32{-0.1, -0.5, -1.0}
		samples := converter.ConvertToInt16(floats)

		assert.Len(t, samples, len(floats))
		assert.InDelta(t, int16(-3276), samples[0], 0.1)
		assert.InDelta(t, int16(-16383), samples[1], 0.1)
		assert.Equal(t, int16(-32767), samples[2])
	})

	t.Run("ZeroFloats", func(t *testing.T) {
		floats := []float32{0.0, 0.0, 0.0}
		samples := converter.ConvertToInt16(floats)

		assert.Len(t, samples, len(floats))
		assert.Equal(t, int16(0), samples[0])
		assert.Equal(t, int16(0), samples[1])
		assert.Equal(t, int16(0), samples[2])
	})

	t.Run("EdgeValues", func(t *testing.T) {
		floats := []float32{32767.0, -32768.0}
		samples := converter.ConvertToInt16(floats)

		assert.Len(t, samples, len(floats))
		assert.Equal(t, int16(32767), samples[0])
		assert.Equal(t, int16(-32767), samples[1])
	})

	t.Run("EmptyFloats", func(t *testing.T) {
		floats := []float32{}
		samples := converter.ConvertToInt16(floats)

		assert.Empty(t, samples)
	})

	t.Run("SingleFloat", func(t *testing.T) {
		floats := []float32{0.5}
		samples := converter.ConvertToInt16(floats)

		assert.Len(t, samples, 1)
		expected := int16(16383)
		assert.InDelta(t, expected, samples[0], 0.1)
	})

	t.Run("ClippingValues", func(t *testing.T) {
		floats := []float32{1.5, -1.5, 2.0}
		samples := converter.ConvertToInt16(floats)

		assert.Len(t, samples, len(floats))
		assert.Equal(t, int16(32767), samples[0])
		assert.Equal(t, int16(-32767), samples[1])
		assert.Equal(t, int16(32767), samples[2])
	})

	t.Run("VerySmallFloats", func(t *testing.T) {
		floats := []float32{0.0001, 0.0002}
		samples := converter.ConvertToInt16(floats)

		assert.Len(t, samples, len(floats))
		expected1 := int16(3)
		expected2 := int16(6)
		assert.InDelta(t, expected1, samples[0], 0.1)
		assert.InDelta(t, expected2, samples[1], 0.1)
	})
}

func TestPCMConverter_ConvertRoundTrip(t *testing.T) {
	converter := NewPCMConverter(SampleRate, Channels)

	t.Run("ConvertFloatToInt16AndBack", func(t *testing.T) {
		floats := []float32{0.0, 0.25, 0.5, 0.75, 1.0}
		samples := converter.ConvertToInt16(floats)
		convertedFloats := converter.ConvertToFloat32(samples)

		assert.Len(t, convertedFloats, len(floats))
		for i := range floats {
			assert.InDelta(t, floats[i], convertedFloats[i], 0.001)
		}
	})

	t.Run("FullRangeRoundTrip", func(t *testing.T) {
		samples := []int16{0, 1000, -1000}
		floats := converter.ConvertToFloat32(samples)
		convertedSamples := converter.ConvertToInt16(floats)

		assert.Len(t, convertedSamples, len(samples))
		for i := range samples {
			assert.InDelta(t, float64(samples[i]), float64(convertedSamples[i]), 1)
		}
	})
}

func TestReadPCM(t *testing.T) {
	t.Run("ValidPCMData", func(t *testing.T) {
		data := []byte{0x00, 0x00, 0x00, 0x80, 0x00, 0x01, 0x00, 0x80, 0x00, 0x03, 0x00, 0x00}

		reader := bytes.NewReader(data)
		samples, err := ReadPCM(reader)

		assert.NoError(t, err)
		assert.Len(t, samples, len(data)/2)
		assert.Equal(t, int16(0), samples[0])
		assert.Equal(t, int16(-32768), samples[1])
		assert.Equal(t, int16(256), samples[2])
		assert.Equal(t, int16(-32768), samples[3])
		assert.Equal(t, int16(768), samples[4])
		assert.Equal(t, int16(0), samples[5])
	})

	t.Run("EOFBeforeReadingFullSamples", func(t *testing.T) {
		data := []byte{0x00, 0x00, 0x01}

		reader := bytes.NewReader(data)
		samples, err := ReadPCM(reader)

		assert.NoError(t, err)
		assert.Len(t, samples, 1)
		assert.Equal(t, int16(0), samples[0])
	})

	t.Run("EmptyReader", func(t *testing.T) {
		reader := bytes.NewReader([]byte{})
		samples, err := ReadPCM(reader)

		assert.NoError(t, err)
		assert.Empty(t, samples)
	})

	t.Run("ReaderWithPartialSample", func(t *testing.T) {
		data := []byte{0x00, 0x00, 0x01}

		reader := bytes.NewReader(data)
		samples, err := ReadPCM(reader)

		assert.NoError(t, err)
		assert.Len(t, samples, 1)
	})

	t.Run("MultipleReads", func(t *testing.T) {
		data := []byte{
			0x00, 0x00, 0x00, 0x80, 0x00, 0x01,
			0x00, 0x00, 0x00, 0x02, 0x00, 0x80,
			0x00, 0x03, 0x00, 0x00, 0x00, 0x04,
			0x00, 0x80, 0x00, 0x05,
		}

		reader := bytes.NewReader(data)
		samples, err := ReadPCM(reader)

		assert.NoError(t, err)
		assert.Len(t, samples, len(data)/2)
		assert.Equal(t, int16(0), samples[0])
		assert.Equal(t, int16(-32768), samples[1])
		assert.Equal(t, int16(256), samples[2])
		assert.Equal(t, int16(0), samples[3])
		assert.Equal(t, int16(512), samples[4])
		assert.Equal(t, int16(-32768), samples[5])
		assert.Equal(t, int16(768), samples[6])
		assert.Equal(t, int16(0), samples[7])
		assert.Equal(t, int16(1024), samples[8])
		assert.Equal(t, int16(-32768), samples[9])
		assert.Equal(t, int16(1280), samples[10])
	})

	t.Run("BigEndianInt16", func(t *testing.T) {
		data := []byte{0x00, 0x00, 0x00, 0x01}

		reader := bytes.NewReader(data)
		samples, err := ReadPCM(reader)

		assert.NoError(t, err)
		assert.Len(t, samples, 2)
		assert.Equal(t, int16(0), samples[0])
		assert.Equal(t, int16(256), samples[1])
	})
}

func TestWritePCM(t *testing.T) {
	t.Run("WriteSingleSample", func(t *testing.T) {
		samples := []int16{1000}

		var buf bytes.Buffer
		err := WritePCM(&buf, samples)

		assert.NoError(t, err)
		assert.Len(t, buf.Bytes(), 2)
		assert.Equal(t, int16(1000), int16(binary.LittleEndian.Uint16(buf.Bytes()[:2])))  //nolint:gosec // intentional uint16->int16 reinterpret in test
	})

	t.Run("WriteMultipleSamples", func(t *testing.T) {
		samples := []int16{1000, -5000, 10000, -32768, 32767}

		var buf bytes.Buffer
		err := WritePCM(&buf, samples)

		assert.NoError(t, err)
		assert.Len(t, buf.Bytes(), len(samples)*2)
	})

	t.Run("WriteEmptySamples", func(t *testing.T) {
		samples := []int16{}

		var buf bytes.Buffer
		err := WritePCM(&buf, samples)

		assert.NoError(t, err)
		assert.Empty(t, buf.Bytes())
	})

	t.Run("WriteBigSamples", func(t *testing.T) {
		samples := make([]int16, 10000)
		for i := range samples {
			samples[i] = int16(i % 65536) //nolint:gosec // intentional wraparound for test data
		}

		var buf bytes.Buffer
		err := WritePCM(&buf, samples)

		assert.NoError(t, err)
		assert.Len(t, buf.Bytes(), len(samples)*2)
	})

	t.Run("WriteAllRange", func(t *testing.T) {
		samples := []int16{32767, 0, -32768}

		var buf bytes.Buffer
		err := WritePCM(&buf, samples)

		assert.NoError(t, err)
		assert.Len(t, buf.Bytes(), 6)

		expected := []byte{0xFF, 0x7F, 0x00, 0x00, 0x00, 0x80}
		assert.Equal(t, expected, buf.Bytes())
	})

	t.Run("LittleEndian", func(t *testing.T) {
		samples := []int16{256, -256}

		var buf bytes.Buffer
		err := WritePCM(&buf, samples)

		assert.NoError(t, err)
		assert.Len(t, buf.Bytes(), 4)

		// First sample: 256 = 0x0100 -> 0x00, 0x01
		assert.Equal(t, byte(0), buf.Bytes()[0])
		assert.Equal(t, byte(1), buf.Bytes()[1])

		// Second sample: -256 = 0xFF00 -> 0x00, 0xFF
		assert.Equal(t, byte(0), buf.Bytes()[2])
		assert.Equal(t, byte(255), buf.Bytes()[3])
	})
}

func TestChunkPCM(t *testing.T) {
	t.Run("StandardChunkSize", func(t *testing.T) {
		samples := make([]int16, 4800)
		for i := range samples {
			samples[i] = int16(i % 65536) //nolint:gosec // intentional wraparound for test data
		}

		chunks := ChunkPCM(samples, 100, SampleRate)

		assert.Len(t, chunks, 1)
		assert.Len(t, chunks[0], 4800)
	})

	t.Run("MultipleChunks", func(t *testing.T) {
		samples := make([]int16, 9600)
		for i := range samples {
			samples[i] = int16(i % 65536) //nolint:gosec // intentional wraparound for test data
		}

		chunks := ChunkPCM(samples, 100, SampleRate)

		assert.Len(t, chunks, 2)
		assert.Len(t, chunks[0], 4800)
		assert.Len(t, chunks[1], 4800)
	})

	t.Run("PartialLastChunk", func(t *testing.T) {
		samples := make([]int16, 10000)
		for i := range samples {
			samples[i] = int16(i % 65536) //nolint:gosec // intentional wraparound for test data
		}

		chunks := ChunkPCM(samples, 100, SampleRate)

		assert.Len(t, chunks, 3)
		assert.Len(t, chunks[0], 4800)
		assert.Len(t, chunks[1], 4800)
		assert.Len(t, chunks[2], 400)
	})

	t.Run("EmptySamples", func(t *testing.T) {
		samples := []int16{}
		chunks := ChunkPCM(samples, 100, SampleRate)

		assert.Empty(t, chunks)
	})

	t.Run("SingleSampleChunk", func(t *testing.T) {
		samples := []int16{1000}
		chunks := ChunkPCM(samples, 100, SampleRate)

		assert.Len(t, chunks, 1)
		assert.Len(t, chunks[0], 1)
		assert.Equal(t, int16(1000), chunks[0][0])
	})

	t.Run("ExactMultipleOfChunkSize", func(t *testing.T) {
		samples := make([]int16, 48000)
		for i := range samples {
			samples[i] = int16(i % 65536) //nolint:gosec // intentional wraparound for test data
		}

		chunks := ChunkPCM(samples, 100, SampleRate)

		assert.Len(t, chunks, 10)
		assert.Len(t, chunks[0], 4800)
		assert.Len(t, chunks[9], 4800)
	})

	t.Run("ChunkSizeZero", func(t *testing.T) {
		samples := make([]int16, 10000)
		for i := range samples {
			samples[i] = int16(i % 65536) //nolint:gosec // intentional wraparound for test data
		}

		chunks := ChunkPCM(samples, 0, SampleRate)

		assert.Empty(t, chunks)
	})

	t.Run("VerySmallChunkSize", func(t *testing.T) {
		samples := make([]int16, 10000)
		for i := range samples {
			samples[i] = int16(i % 65536) //nolint:gosec // intentional wraparound for test data
		}

		chunks := ChunkPCM(samples, 1, SampleRate)

		// 1ms at 48000 Hz = 48 samples/chunk; 10000/48 = 208 full + 1 partial = 209 chunks
		assert.Len(t, chunks, 209)
		assert.Len(t, chunks[0], 48)
		assert.Len(t, chunks[208], 16)
	})

	t.Run("VeryLargeChunkSize", func(t *testing.T) {
		samples := make([]int16, 10000)
		for i := range samples {
			samples[i] = int16(i % 65536) //nolint:gosec // intentional wraparound for test data
		}

		chunks := ChunkPCM(samples, 100000, SampleRate)

		assert.Len(t, chunks, 1)
		assert.Len(t, chunks[0], 10000)
	})

	t.Run("DifferentSampleRate", func(t *testing.T) {
		samples := make([]int16, 8820)
		for i := range samples {
			samples[i] = int16(i % 65536) //nolint:gosec // intentional wraparound for test data
		}

		chunks := ChunkPCM(samples, 100, 44100)

		assert.Len(t, chunks, 2)
		assert.Len(t, chunks[0], 4410)
		assert.Len(t, chunks[1], 4410)
	})
}

func TestMergePCM(t *testing.T) {
	t.Run("MergeSingleChunk", func(t *testing.T) {
		chunks := [][]int16{
			{1, 2, 3},
			{4, 5, 6},
		}

		merged := MergePCM(chunks)

		assert.Len(t, merged, 6)
		assert.Equal(t, []int16{1, 2, 3, 4, 5, 6}, merged)
	})

	t.Run("MergeMultipleChunks", func(t *testing.T) {
		chunks := [][]int16{
			{1, 2},
			{3, 4},
			{5, 6},
		}

		merged := MergePCM(chunks)

		assert.Len(t, merged, 6)
		assert.Equal(t, []int16{1, 2, 3, 4, 5, 6}, merged)
	})

	t.Run("MergeEmptyChunks", func(t *testing.T) {
		chunks := [][]int16{}

		merged := MergePCM(chunks)

		assert.Empty(t, merged)
	})

	t.Run("MergeEmptyChunk", func(t *testing.T) {
		chunks := [][]int16{
			{1, 2},
			{},
			{3, 4},
		}

		merged := MergePCM(chunks)

		assert.Len(t, merged, 4)
		assert.Equal(t, []int16{1, 2, 3, 4}, merged)
	})

	t.Run("MergeSingleChunk", func(t *testing.T) {
		chunks := [][]int16{
			{1, 2, 3, 4, 5},
		}

		merged := MergePCM(chunks)

		assert.Len(t, merged, 5)
		assert.Equal(t, []int16{1, 2, 3, 4, 5}, merged)
	})

	t.Run("MergeZeroLengthChunks", func(t *testing.T) {
		chunks := [][]int16{
			{},
			{},
			{},
		}

		merged := MergePCM(chunks)

		assert.Empty(t, merged)
	})

	t.Run("MergeLargeChunks", func(t *testing.T) {
		chunks := make([][]int16, 100)
		for i := range chunks {
			chunk := make([]int16, 1000)
			for j := range chunk {
				chunk[j] = int16(j % 65536) //nolint:gosec // intentional wraparound for test data
			}
			chunks[i] = chunk
		}

		merged := MergePCM(chunks)

		assert.Len(t, merged, 100000)
	})

	t.Run("MergeAllNegativeValues", func(t *testing.T) {
		chunks := [][]int16{
			{-1, -2, -3},
			{-4, -5, -6},
		}

		merged := MergePCM(chunks)

		assert.Len(t, merged, 6)
		assert.Equal(t, []int16{-1, -2, -3, -4, -5, -6}, merged)
	})

	t.Run("MergeAllZeroValues", func(t *testing.T) {
		chunks := [][]int16{
			{0, 0},
			{0, 0},
			{0, 0},
		}

		merged := MergePCM(chunks)

		assert.Len(t, merged, 6)
		assert.Equal(t, []int16{0, 0, 0, 0, 0, 0}, merged)
	})

	t.Run("MergeMixedValues", func(t *testing.T) {
		chunks := [][]int16{
			{1, 0, -1},
			{2, -2, 0},
			{-3, 3, -3},
		}

		merged := MergePCM(chunks)

		assert.Len(t, merged, 9)
		assert.Equal(t, []int16{1, 0, -1, 2, -2, 0, -3, 3, -3}, merged)
	})

	t.Run("MergeSingleSampleChunks", func(t *testing.T) {
		chunks := [][]int16{
			{1},
			{2},
			{3},
			{4},
			{5},
		}

		merged := MergePCM(chunks)

		assert.Len(t, merged, 5)
		assert.Equal(t, []int16{1, 2, 3, 4, 5}, merged)
	})

	t.Run("MergeLargeDataset", func(t *testing.T) {
		chunks := make([][]int16, 1000)
		for i := range chunks {
			chunk := make([]int16, 100)
			for j := range chunk {
				chunk[j] = int16((i*100 + j) % 65536) //nolint:gosec // intentional wraparound for test data
			}
			chunks[i] = chunk
		}

		merged := MergePCM(chunks)

		assert.Len(t, merged, 100000)
	})

	t.Run("MergePreservesOrder", func(t *testing.T) {
		chunks := [][]int16{
			{1, 2, 3},
			{4, 5, 6},
			{7, 8, 9},
		}

		merged := MergePCM(chunks)

		expected := []int16{1, 2, 3, 4, 5, 6, 7, 8, 9}
		assert.Equal(t, expected, merged)
	})
}

func TestOpusDecoder_NewOpusDecoder(t *testing.T) {
	t.Run("ValidDecoder", func(t *testing.T) {
		decoder := NewOpusDecoder(48000, 1)
		assert.NotNil(t, decoder)
		assert.Equal(t, 48000, decoder.sampleRate)
		assert.Equal(t, 1, decoder.channels)
	})

	t.Run("CustomSampleRate", func(t *testing.T) {
		decoder := NewOpusDecoder(44100, 2)
		assert.NotNil(t, decoder)
		assert.Equal(t, 44100, decoder.sampleRate)
		assert.Equal(t, 2, decoder.channels)
	})

	t.Run("CustomChannels", func(t *testing.T) {
		decoder := NewOpusDecoder(48000, 2)
		assert.NotNil(t, decoder)
		assert.Equal(t, 2, decoder.channels)
	})
}

func TestOpusDecoder_Decode(t *testing.T) {
	t.Run("InvalidOpusData", func(t *testing.T) {
		decoder := NewOpusDecoder(48000, 1)

		opusData := []byte{0x00, 0x01, 0x02}
		_, err := decoder.Decode(opusData)
		// May succeed (returning silence) or fail — just must not panic
		_ = err
	})

	t.Run("EmptyOpusData", func(t *testing.T) {
		decoder := NewOpusDecoder(48000, 1)

		// Empty data may return silence or error — must not panic
		_, _ = decoder.Decode([]byte{})
	})
}

func TestValidateOpusFrame(t *testing.T) {
	t.Run("ValidFrame", func(t *testing.T) {
		frame := []byte{0x01, 0x02, 0x03}
		err := ValidateOpusFrame(frame)

		assert.NoError(t, err)
	})

	t.Run("EmptyFrame", func(t *testing.T) {
		frame := []byte{}
		err := ValidateOpusFrame(frame)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty opus frame")
	})

	t.Run("FrameTooLarge", func(t *testing.T) {
		frame := make([]byte, 5000)
		err := ValidateOpusFrame(frame)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "opus frame too large")
	})

	t.Run("MaximumValidSize", func(t *testing.T) {
		frame := make([]byte, 4000)
		for i := range frame {
			frame[i] = byte(i % 256)
		}

		err := ValidateOpusFrame(frame)
		assert.NoError(t, err)
	})

	t.Run("OneByteTooLarge", func(t *testing.T) {
		frame := make([]byte, 4001)
		for i := range frame {
			frame[i] = byte(i % 256)
		}

		err := ValidateOpusFrame(frame)
		assert.Error(t, err)
	})

	t.Run("ExactlyFourThousandBytes", func(t *testing.T) {
		frame := make([]byte, 4000)
		for i := range frame {
			frame[i] = byte(i % 256)
		}

		err := ValidateOpusFrame(frame)
		assert.NoError(t, err)
	})

	t.Run("ZeroByteFrame", func(t *testing.T) {
		frame := []byte{0x00}
		err := ValidateOpusFrame(frame)

		assert.NoError(t, err)
	})
}

func TestGetOpusFrameDuration(t *testing.T) {
	t.Run("ValidDurationCalculation", func(t *testing.T) {
		frameSize := 960
		sampleRate := 48000

		duration := GetOpusFrameDuration(frameSize, sampleRate)
		expected := (frameSize * 1000) / sampleRate

		assert.Equal(t, expected, duration)
	})

	t.Run("DefaultOpusFrameSize", func(t *testing.T) {
		frameSize := OpusFrameSize
		sampleRate := OpusSampleRate

		duration := GetOpusFrameDuration(frameSize, sampleRate)
		expected := (frameSize * 1000) / sampleRate

		assert.Equal(t, expected, duration)
	})

	t.Run("FrameSizeZero", func(t *testing.T) {
		frameSize := 0
		sampleRate := 48000

		duration := GetOpusFrameDuration(frameSize, sampleRate)
		assert.Equal(t, 0, duration)
	})

	t.Run("SampleRateZero", func(t *testing.T) {
		frameSize := 960
		sampleRate := 0

		duration := GetOpusFrameDuration(frameSize, sampleRate)
		assert.Equal(t, 0, duration)
	})

	t.Run("DifferentSampleRates", func(t *testing.T) {
		frameSize := 960

		sampleRates := []int{8000, 16000, 48000, 44100}

		for _, sampleRate := range sampleRates {
			duration := GetOpusFrameDuration(frameSize, sampleRate)
			expected := (frameSize * 1000) / sampleRate
			assert.Equal(t, expected, duration)
		}
	})

	t.Run("DifferentFrameSizes", func(t *testing.T) {
		sampleRate := 48000

		frameSizes := []int{480, 960, 1920}

		for _, frameSize := range frameSizes {
			duration := GetOpusFrameDuration(frameSize, sampleRate)
			expected := (frameSize * 1000) / sampleRate
			assert.Equal(t, expected, duration)
		}
	})

	t.Run("LargeFrameSize", func(t *testing.T) {
		frameSize := 8192
		sampleRate := 48000

		duration := GetOpusFrameDuration(frameSize, sampleRate)
		expected := (frameSize * 1000) / sampleRate

		assert.Equal(t, expected, duration)
	})

	t.Run("SmallFrameSize", func(t *testing.T) {
		frameSize := 120
		sampleRate := 48000

		duration := GetOpusFrameDuration(frameSize, sampleRate)
		expected := (frameSize * 1000) / sampleRate

		assert.Equal(t, expected, duration)
	})
}

func TestOpusConstants(t *testing.T) {
	t.Run("SampleRate", func(t *testing.T) {
		assert.Equal(t, 48000, OpusSampleRate)
	})

	t.Run("Channels", func(t *testing.T) {
		assert.Equal(t, 1, OpusChannels)
	})

	t.Run("FrameSize", func(t *testing.T) {
		assert.Equal(t, 960, OpusFrameSize)
	})

	t.Run("DurationOfDefaultFrame", func(t *testing.T) {
		duration := GetOpusFrameDuration(OpusFrameSize, OpusSampleRate)
		expected := 20 // 960 samples at 48kHz = 20ms

		assert.Equal(t, expected, duration)
	})
}
