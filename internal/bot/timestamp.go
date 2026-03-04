package bot

import (
	"sync"
	"time"

	"github.com/flowup/aftertalk/internal/logging"
)

type Timestamp struct {
	SessionStart time.Time
	mu           sync.RWMutex
}

func NewTimestamp() *Timestamp {
	return &Timestamp{
		SessionStart: time.Now(),
	}
}

func (t *Timestamp) GetMonotonicTime() int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	elapsed := time.Since(t.SessionStart)
	return elapsed.Milliseconds()
}

func (t *Timestamp) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.SessionStart = time.Now()
	logging.Debug("Timestamp reset")
}

type AudioChunk struct {
	ParticipantID string
	Timestamp     int64
	Data          []byte
	Duration      int
}

type AudioBuffer struct {
	chunks  []AudioChunk
	mu      sync.RWMutex
	maxSize int
}

func NewAudioBuffer(maxSize int) *AudioBuffer {
	return &AudioBuffer{
		chunks:  make([]AudioChunk, 0),
		maxSize: maxSize,
	}
}

func (b *AudioBuffer) Add(chunk AudioChunk) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.chunks) >= b.maxSize {
		b.chunks = b.chunks[1:]
	}

	b.chunks = append(b.chunks, chunk)
	logging.Debugf("Audio buffer: added chunk from %s at %dms (%d bytes)",
		chunk.ParticipantID, chunk.Timestamp, len(chunk.Data))
}

func (b *AudioBuffer) GetAll() []AudioChunk {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]AudioChunk, len(b.chunks))
	copy(result, b.chunks)
	return result
}

func (b *AudioBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.chunks = make([]AudioChunk, 0)
	logging.Debug("Audio buffer cleared")
}

func (b *AudioBuffer) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return len(b.chunks)
}
