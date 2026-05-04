package audio

import (
	"bytes"
	"encoding/binary"
)

// oggCRCTable is the lookup table for the Ogg-specific CRC32.
// Ogg uses polynomial 0x04c11db7 with NO input/output bit reflection,
// which differs from the standard hash/crc32 IEEE implementation.
var oggCRCTable [256]uint32 //nolint:gochecknoglobals // CRC table initialized once in init()

func init() { //nolint:gochecknoinits // CRC table must be initialized before first use
	for i := range 256 {
		crc := uint32(i) << 24
		for range 8 {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ 0x04c11db7
			} else {
				crc <<= 1
			}
		}
		oggCRCTable[i] = crc
	}
}

// oggCRC computes the Ogg page checksum (checksum field must be zeroed first).
func oggCRC(data []byte) uint32 {
	var crc uint32
	for _, b := range data {
		crc = (crc << 8) ^ oggCRCTable[((crc>>24)^uint32(b))&0xFF]
	}
	return crc
}

// WriteOGGOpus wraps individual Opus RTP frame payloads into a valid OGG Opus stream.
// The output is a self-contained OGG Opus file that ffmpeg/faster-whisper can decode.
//
// Parameters:
//   - frames: individual Opus RTP payloads (each is one Opus packet, typically 20ms)
//   - sampleRate: input sample rate (typically 48000 for WebRTC Opus)
//   - channels: number of audio channels (1 = mono)
func WriteOGGOpus(frames [][]byte, sampleRate uint32, channels uint8) ([]byte, error) {
	if sampleRate == 0 {
		sampleRate = 48000
	}
	if channels == 0 {
		channels = 1
	}

	w := &bytes.Buffer{}
	serial := uint32(0x4AFFE2A1) // arbitrary stream serial number

	// Page 0: OpusHead identification header (BOS)
	opusHead := buildOpusHead(channels, sampleRate)
	writePage(w, opusHead, 0x02, 0, serial, 0)

	// Page 1: OpusTags comment header (minimal)
	opusTags := buildOpusTags()
	writePage(w, opusTags, 0x00, 0, serial, 1)

	// Pages 2+: audio data — one page per frame for simplicity
	// granulePos counts PCM samples; Opus at 48kHz, 20ms/frame = 960 samples/frame
	var granulePos int64
	const samplesPerFrame = 960 // 20ms at 48kHz

	for i, frame := range frames {
		granulePos += samplesPerFrame
		headerType := byte(0x00)
		if i == len(frames)-1 {
			headerType = 0x04 // EOS on last page
		}
		writePage(w, frame, headerType, granulePos, serial, uint32(i+2))
	}

	return w.Bytes(), nil
}

// buildOpusHead builds the 19-byte OpusHead identification packet (RFC 7845 §5.1).
func buildOpusHead(channels uint8, inputSampleRate uint32) []byte {
	h := &bytes.Buffer{}
	h.WriteString("OpusHead")
	h.WriteByte(1)                                            // version
	h.WriteByte(channels)                                     // channel count
	_ = binary.Write(h, binary.LittleEndian, uint16(3840))    //nolint:errcheck // bytes.Buffer never fails
	_ = binary.Write(h, binary.LittleEndian, inputSampleRate) //nolint:errcheck // bytes.Buffer never fails
	_ = binary.Write(h, binary.LittleEndian, int16(0))        //nolint:errcheck // bytes.Buffer never fails
	h.WriteByte(0)                                            // channel mapping family (mono/stereo)
	return h.Bytes()
}

// buildOpusTags builds a minimal OpusTags comment packet (RFC 7845 §5.2).
func buildOpusTags() []byte {
	vendor := "aftertalk"
	t := &bytes.Buffer{}
	t.WriteString("OpusTags")
	_ = binary.Write(t, binary.LittleEndian, uint32(len(vendor))) //nolint:errcheck,gosec // bytes.Buffer never fails; len is always non-negative
	t.WriteString(vendor)
	_ = binary.Write(t, binary.LittleEndian, uint32(0)) //nolint:errcheck // bytes.Buffer never fails
	return t.Bytes()
}

// writePage writes a single OGG page to w (RFC 3533).
// data must fit in a single page (max 255*255 = 65025 bytes).
func writePage(w *bytes.Buffer, data []byte, headerType byte, granulePos int64, serial, seqNo uint32) {
	// Build segment table: each segment is at most 255 bytes.
	var segTable []byte
	remaining := len(data)
	for remaining > 0 {
		seg := remaining
		if seg > 255 {
			seg = 255
		}
		segTable = append(segTable, byte(seg))
		remaining -= seg
		// A segment of exactly 255 means the packet continues in next segment.
		// A segment < 255 terminates the packet. If remaining == 0 and last seg == 255,
		// we must add a zero-length terminating segment.
	}
	// If last segment is 255 bytes, add terminator.
	if len(segTable) > 0 && segTable[len(segTable)-1] == 255 {
		segTable = append(segTable, 0)
	}

	page := &bytes.Buffer{}
	page.WriteString("OggS")                                // capture pattern
	page.WriteByte(0)                                       // version
	page.WriteByte(headerType)                              // header type
	_ = binary.Write(page, binary.LittleEndian, granulePos) //nolint:errcheck // bytes.Buffer never fails
	_ = binary.Write(page, binary.LittleEndian, serial)     //nolint:errcheck // bytes.Buffer never fails
	_ = binary.Write(page, binary.LittleEndian, seqNo)      //nolint:errcheck // bytes.Buffer never fails
	_ = binary.Write(page, binary.LittleEndian, uint32(0))  //nolint:errcheck // bytes.Buffer never fails
	page.WriteByte(byte(len(segTable)))                     //nolint:gosec // Ogg spec: segment count ≤ 255
	page.Write(segTable)                                    // segment table
	page.Write(data)                                        // page data

	// Compute CRC over the complete page (checksum bytes are zero as written above).
	pageBytes := page.Bytes()
	crc := oggCRC(pageBytes)
	// Write CRC at offset 22 (after "OggS" + version + header_type + 8-byte granule + 4-byte serial + 4-byte seqno = 22)
	binary.LittleEndian.PutUint32(pageBytes[22:26], crc)

	w.Write(pageBytes)
}
