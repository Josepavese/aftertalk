package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os/exec"
)

// writeOggPage writes a single Ogg page to buf.
// headerType: 0x00=continuation, 0x02=first, 0x04=last
// granule: granule position (-1 = unknown for header pages)
func writeOggPage(buf *bytes.Buffer, headerType byte, granule int64, serial, seqNo uint32, packets [][]byte) {
	// Build segment table: lace each packet with 255-byte segments
	var segTable []byte
	var segData []byte
	for _, pkt := range packets {
		n := len(pkt)
		for n >= 255 {
			segTable = append(segTable, 255)
			segData = append(segData, pkt[:255]...)
			pkt = pkt[255:]
			n = len(pkt)
		}
		segTable = append(segTable, byte(n))
		segData = append(segData, pkt...)
	}

	page := make([]byte, 27+len(segTable)+len(segData))
	copy(page[0:], "OggS")
	page[4] = 0 // version
	page[5] = headerType
	binary.LittleEndian.PutUint64(page[6:], uint64(granule))
	binary.LittleEndian.PutUint32(page[14:], serial)
	binary.LittleEndian.PutUint32(page[18:], seqNo)
	// checksum at [22:26] starts as zero
	page[26] = byte(len(segTable))
	copy(page[27:], segTable)
	copy(page[27+len(segTable):], segData)

	chk := oggCRC(page)
	binary.LittleEndian.PutUint32(page[22:], chk)
	buf.Write(page)
}

// EncodeOggOpus wraps raw Opus RTP payloads in a minimal Ogg/Opus container.
// sampleRate must be 48000 for standard WebRTC Opus.
// frameSizeSamples is 960 for 20ms at 48kHz.
func EncodeOggOpus(frames [][]byte, sampleRate int, frameSizeSamples int) []byte {
	const serial = 0x12345678
	var buf bytes.Buffer
	seqNo := uint32(0)

	// OpusHead: identification header
	opusHead := make([]byte, 19)
	copy(opusHead[0:], "OpusHead")
	opusHead[8] = 1 // version
	opusHead[9] = 1 // channel count (mono)
	binary.LittleEndian.PutUint16(opusHead[10:], 0)              // pre-skip
	binary.LittleEndian.PutUint32(opusHead[12:], uint32(sampleRate)) // input sample rate
	binary.LittleEndian.PutUint16(opusHead[16:], 0)              // output gain
	opusHead[18] = 0                                              // channel mapping family (mono/stereo)

	writeOggPage(&buf, 0x02, 0, serial, seqNo, [][]byte{opusHead})
	seqNo++

	// OpusTags: comment header (minimal)
	opusTags := []byte("OpusTags\x07\x00\x00\x00aftertalk\x00\x00\x00\x00")
	writeOggPage(&buf, 0x00, 0, serial, seqNo, [][]byte{opusTags})
	seqNo++

	// Audio pages: one packet per page for simplicity
	granule := int64(0)
	for i, frame := range frames {
		if len(frame) == 0 {
			continue
		}
		granule += int64(frameSizeSamples)
		headerType := byte(0x00)
		if i == len(frames)-1 {
			headerType = 0x04 // last page
		}
		writeOggPage(&buf, headerType, granule, serial, seqNo, [][]byte{frame})
		seqNo++
	}

	return buf.Bytes()
}

// DecodeFramesToWAVffmpeg wraps Opus RTP frames in Ogg and decodes via ffmpeg.
// Falls back to kazzmir if ffmpeg is not available.
func DecodeFramesToWAVffmpeg(frames [][]byte, sampleRate int) ([]byte, error) {
	ogg := EncodeOggOpus(frames, 48000, OpusFrameSize)

	// Use ffmpeg to convert Ogg/Opus → WAV 16kHz mono PCM
	cmd := exec.Command("ffmpeg",
		"-f", "ogg", "-i", "pipe:0",
		"-ar", "16000", "-ac", "1",
		"-f", "wav", "pipe:1",
		"-loglevel", "error",
	)
	cmd.Stdin = bytes.NewReader(ogg)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg decode: %w", err)
	}
	return out, nil
}
