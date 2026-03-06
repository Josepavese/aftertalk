package bot

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTimestamp(t *testing.T) {
	timestamp := NewTimestamp()

	assert.NotNil(t, timestamp)
	assert.NotNil(t, timestamp.SessionStart)
	assert.True(t, timestamp.SessionStart.Before(time.Now()) || timestamp.SessionStart.Equal(time.Now()))
}

func TestNewTimestamp_DifferentInstances(t *testing.T) {
	timestamp1 := NewTimestamp()
	timestamp2 := NewTimestamp()

	assert.NotNil(t, timestamp1)
	assert.NotNil(t, timestamp2)
	assert.Equal(t, timestamp1.SessionStart, timestamp2.SessionStart)
}

func TestTimestamp_GetMonotonicTime_NegativeZero(t *testing.T) {
	timestamp := NewTimestamp()

	time.Sleep(10 * time.Millisecond)

	t := timestamp.GetMonotonicTime()
	assert.True(t >= 0)
}

func TestTimestamp_GetMonotonicTime_MillisecondResolution(t *testing.T) {
	timestamp := NewTimestamp()

	time.Sleep(150 * time.Millisecond)

	t := timestamp.GetMonotonicTime()
	assert.True(t >= 150)
}

func TestTimestamp_GetMonotonicTime_MultipleCalls(t *testing.T) {
	timestamp := NewTimestamp()

	time.Sleep(50 * time.Millisecond)
	t1 := timestamp.GetMonotonicTime()

	time.Sleep(50 * time.Millisecond)
	t2 := timestamp.GetMonotonicTime()

	time.Sleep(50 * time.Millisecond)
	t3 := timestamp.GetMonotonicTime()

	assert.Equal(t, t1, t2)
	assert.Equal(t, t2, t3)
}

func TestTimestamp_GetMonotonicTime_Increments(t *testing.T) {
	timestamp := NewTimestamp()

	time.Sleep(100 * time.Millisecond)
	t1 := timestamp.GetMonotonicTime()

	time.Sleep(100 * time.Millisecond)
	t2 := timestamp.GetMonotonicTime()

	assert.Equal(t, t1, t2)
}

func TestTimestamp_GetMonotonicTime_SameStart(t *testing.T) {
	timestamp1 := NewTimestamp()
	timestamp2 := NewTimestamp()

	time.Sleep(100 * time.Millisecond)

	t1 := timestamp1.GetMonotonicTime()
	t2 := timestamp2.GetMonotonicTime()

	assert.Equal(t, t1, t2)
}

func TestTimestamp_Reset_ZeroTime(t *testing.T) {
	timestamp := NewTimestamp()

	time.Sleep(100 * time.Millisecond)
	t1 := timestamp.GetMonotonicTime()
	assert.True(t1 > 0)

	timestamp.Reset()

	t2 := timestamp.GetMonotonicTime()
	assert.Equal(t, t1, t2)
}

func TestTimestamp_Reset_MultipleTimes(t *testing.T) {
	timestamp := NewTimestamp()

	for i := 0; i < 5; i++ {
		time.Sleep(50 * time.Millisecond)
		t1 := timestamp.GetMonotonicTime()

		time.Sleep(50 * time.Millisecond)
		t2 := timestamp.GetMonotonicTime()

		assert.Equal(t, t1, t2)
		timestamp.Reset()
	}
}

func TestTimestamp_Reset_AfterSleep(t *testing.T) {
	timestamp := NewTimestamp()

	time.Sleep(200 * time.Millisecond)
	t1 := timestamp.GetMonotonicTime()
	assert.True(t1 > 0)

	timestamp.Reset()

	time.Sleep(100 * time.Millisecond)
	t2 := timestamp.GetMonotonicTime()
	assert.True(t2 > 0)

	assert.Equal(t, t1, t2)
}

func TestTimestamp_GetMonotonicTime_AfterReset(t *testing.T) {
	timestamp := NewTimestamp()

	time.Sleep(100 * time.Millisecond)
	t1 := timestamp.GetMonotonicTime()

	timestamp.Reset()

	time.Sleep(100 * time.Millisecond)
	t2 := timestamp.GetMonotonicTime()

	assert.Equal(t, t1, t2)
}

func TestTimestamp_SessionStartTime(t *testing.T) {
	startTime := time.Now()
	timestamp := NewTimestamp()

	time.Sleep(50 * time.Millisecond)

	assert.Equal(t, timestamp.SessionStart, startTime)
}

func TestTimestamp_SessionStartTime_AfterReset(t *testing.T) {
	startTime := time.Now()
	timestamp := NewTimestamp()

	time.Sleep(100 * time.Millisecond)
	t1 := timestamp.GetMonotonicTime()

	timestamp.Reset()

	time.Sleep(50 * time.Millisecond)
	t2 := timestamp.GetMonotonicTime()

	assert.Equal(t, timestamp.SessionStart, startTime)
}

func TestTimestamp_GetMonotonicTime_AgainstSystemTime(t *testing.T) {
	timestamp := NewTimestamp()

	startTime := timestamp.SessionStart
	currentTime := time.Now()

	elapsed := currentTime.Sub(startTime)
	ms := elapsed.Milliseconds()

	t := timestamp.GetMonotonicTime()
	assert.True(t >= ms)
}

func TestTimestamp_GetMonotonicTime_EdgeCase_Zero(t *testing.T) {
	timestamp := NewTimestamp()

	// Reset immediately
	timestamp.Reset()

	t := timestamp.GetMonotonicTime()
	assert.True(t >= 0)
}

func TestTimestamp_GetMonotonicTime_EdgeCase_SmallSleep(t *testing.T) {
	timestamp := NewTimestamp()

	time.Sleep(1 * time.Millisecond)
	t := timestamp.GetMonotonicTime()

	assert.True(t >= 1)
}

func TestTimestamp_GetMonotonicTime_EdgeCase_LargeSleep(t *testing.T) {
	timestamp := NewTimestamp()

	time.Sleep(10 * time.Second)
	t := timestamp.GetMonotonicTime()

	assert.True(t >= 10000)
}

func TestTimestamp_ConcurrentGetMonotonicTime(t *testing.T) {
	timestamp := NewTimestamp()

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			timestamp.GetMonotonicTime()
		}()
	}

	wg.Wait()

	t := timestamp.GetMonotonicTime()
	assert.True(t > 0)
}

func TestTimestamp_GetMonotonicTime_MultipleTimestamps(t *testing.T) {
	timestamps := []*Timestamp{
		NewTimestamp(),
		NewTimestamp(),
		NewTimestamp(),
	}

	time.Sleep(100 * time.Millisecond)

	for _, ts := range timestamps {
		t := ts.GetMonotonicTime()
		assert.True(t > 0)
	}

	// All should have same start time
	for i := 1; i < len(timestamps); i++ {
		assert.Equal(t, timestamps[0].SessionStart, timestamps[i].SessionStart)
	}
}

func TestTimestamp_GetMonotonicTime_IncrementsAreConsistent(t *testing.T) {
	timestamp := NewTimestamp()

	initial := timestamp.GetMonotonicTime()
	time.Sleep(10 * time.Millisecond)
	afterFirstSleep := timestamp.GetMonotonicTime()
	time.Sleep(10 * time.Millisecond)
	afterSecondSleep := timestamp.GetMonotonicTime()

	assert.Equal(t, initial, afterFirstSleep)
	assert.Equal(t, afterFirstSleep, afterSecondSleep)
	assert.True(t, afterSecondSleep > initial)
}

func TestAudioBuffer_NewAudioBuffer(t *testing.T) {
	buffer := NewAudioBuffer(100)

	assert.NotNil(t, buffer)
	assert.NotNil(t, buffer.chunks)
	assert.Equal(t, 0, len(buffer.chunks))
	assert.Equal(t, 100, buffer.maxSize)
}

func TestAudioBuffer_NewAudioBuffer_InvalidSize(t *testing.T) {
	buffer := NewAudioBuffer(10)

	assert.NotNil(t, buffer)
	assert.Equal(t, 10, buffer.maxSize)
}

func TestAudioBuffer_AddChunk(t *testing.T) {
	buffer := NewAudioBuffer(10)

	chunk := AudioChunk{
		ParticipantID: "participant-1",
		Timestamp:     100,
		Data:          []byte("test data"),
		Duration:      200,
	}

	buffer.Add(chunk)

	assert.Equal(t, 1, len(buffer.chunks))
	assert.Equal(t, chunk, buffer.chunks[0])
}

func TestAudioBuffer_AddMultipleChunks(t *testing.T) {
	buffer := NewAudioBuffer(10)

	chunks := []AudioChunk{
		{ParticipantID: "p1", Timestamp: 100, Data: []byte("data1"), Duration: 200},
		{ParticipantID: "p1", Timestamp: 300, Data: []byte("data2"), Duration: 200},
		{ParticipantID: "p2", Timestamp: 100, Data: []byte("data3"), Duration: 200},
	}

	for _, chunk := range chunks {
		buffer.Add(chunk)
	}

	assert.Equal(t, 3, len(buffer.chunks))
}

func TestAudioBuffer_AddChunk_ExceedsMaxSize(t *testing.T) {
	buffer := NewAudioBuffer(2)

	chunks := []AudioChunk{
		{ParticipantID: "p1", Timestamp: 100, Data: []byte("data1"), Duration: 200},
		{ParticipantID: "p1", Timestamp: 300, Data: []byte("data2"), Duration: 200},
		{ParticipantID: "p1", Timestamp: 500, Data: []byte("data3"), Duration: 200},
		{ParticipantID: "p1", Timestamp: 700, Data: []byte("data4"), Duration: 200},
	}

	for _, chunk := range chunks {
		buffer.Add(chunk)
	}

	assert.Equal(t, 2, len(buffer.chunks))
	assert.Equal(t, []byte("data3"), buffer.chunks[0].Data)
	assert.Equal(t, []byte("data4"), buffer.chunks[1].Data)
}

func TestAudioBuffer_AddChunk_ZeroMaxSize(t *testing.T) {
	buffer := NewAudioBuffer(0)

	chunk := AudioChunk{
		ParticipantID: "p1",
		Timestamp:     100,
		Data:          []byte("data"),
		Duration:      200,
	}

	buffer.Add(chunk)

	assert.Equal(t, 0, len(buffer.chunks))
}

func TestAudioBuffer_AddChunk_InitialChunk(t *testing.T) {
	buffer := NewAudioBuffer(10)

	assert.Equal(t, 0, len(buffer.chunks))

	buffer.Add(AudioChunk{
		ParticipantID: "p1",
		Timestamp:     100,
		Data:          []byte("data"),
		Duration:      200,
	})

	assert.Equal(t, 1, len(buffer.chunks))
}

func TestAudioBuffer_AddChunk_NilData(t *testing.T) {
	buffer := NewAudioBuffer(10)

	chunk := AudioChunk{
		ParticipantID: "p1",
		Timestamp:     100,
		Data:          nil,
		Duration:      200,
	}

	buffer.Add(chunk)

	assert.Equal(t, 1, len(buffer.chunks))
	assert.Nil(t, buffer.chunks[0].Data)
}

func TestAudioBuffer_AddChunk_EmptyData(t *testing.T) {
	buffer := NewAudioBuffer(10)

	chunk := AudioChunk{
		ParticipantID: "p1",
		Timestamp:     100,
		Data:          []byte{},
		Duration:      200,
	}

	buffer.Add(chunk)

	assert.Equal(t, 1, len(buffer.chunks))
	assert.Equal(t, []byte{}, buffer.chunks[0].Data)
}

func TestAudioBuffer_GetAll_Empty(t *testing.T) {
	buffer := NewAudioBuffer(10)

	result := buffer.GetAll()

	assert.NotNil(t, result)
	assert.Equal(t, 0, len(result))
}

func TestAudioBuffer_GetAll(t *testing.T) {
	buffer := NewAudioBuffer(10)

	chunks := []AudioChunk{
		{ParticipantID: "p1", Timestamp: 100, Data: []byte("data1"), Duration: 200},
		{ParticipantID: "p1", Timestamp: 300, Data: []byte("data2"), Duration: 200},
		{ParticipantID: "p2", Timestamp: 100, Data: []byte("data3"), Duration: 200},
	}

	for _, chunk := range chunks {
		buffer.Add(chunk)
	}

	result := buffer.GetAll()

	assert.Equal(t, 3, len(result))
	assert.Equal(t, chunks, result)
}

func TestAudioBuffer_GetAll_MultipleCalls(t *testing.T) {
	buffer := NewAudioBuffer(10)

	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 100, Data: []byte("data1"), Duration: 200})
	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 300, Data: []byte("data2"), Duration: 200})

	result1 := buffer.GetAll()
	result2 := buffer.GetAll()

	assert.Equal(t, result1, result2)
}

func TestAudioBuffer_GetAll_AfterModification(t *testing.T) {
	buffer := NewAudioBuffer(10)

	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 100, Data: []byte("data1"), Duration: 200})
	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 300, Data: []byte("data2"), Duration: 200})

	result1 := buffer.GetAll()

	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 500, Data: []byte("data3"), Duration: 200})

	result2 := buffer.GetAll()

	assert.Equal(t, 3, len(result2))
	assert.Equal(t, []byte("data1"), result2[0].Data)
	assert.Equal(t, []byte("data2"), result2[1].Data)
	assert.Equal(t, []byte("data3"), result2[2].Data)
}

func TestAudioBuffer_GetAll_Overwrite(t *testing.T) {
	buffer := NewAudioBuffer(10)

	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 100, Data: []byte("data1"), Duration: 200})
	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 300, Data: []byte("data2"), Duration: 200})

	result1 := buffer.GetAll()
	assert.Equal(t, []byte("data1"), result1[0].Data)

	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 500, Data: []byte("data2"), Duration: 200})

	result2 := buffer.GetAll()
	assert.Equal(t, []byte("data2"), result2[0].Data)
}

func TestAudioBuffer_Clear(t *testing.T) {
	buffer := NewAudioBuffer(10)

	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 100, Data: []byte("data1"), Duration: 200})
	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 300, Data: []byte("data2"), Duration: 200})

	buffer.Clear()

	assert.Equal(t, 0, len(buffer.chunks))
}

func TestAudioBuffer_Clear_MultipleTimes(t *testing.T) {
	buffer := NewAudioBuffer(10)

	for i := 0; i < 5; i++ {
		buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: int64(i * 100), Data: []byte("data"), Duration: 200})
	}

	buffer.Clear()

	assert.Equal(t, 0, len(buffer.chunks))

	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 100, Data: []byte("data"), Duration: 200})

	assert.Equal(t, 1, len(buffer.chunks))
}

func TestAudioBuffer_Clear_EmptyBuffer(t *testing.T) {
	buffer := NewAudioBuffer(10)

	buffer.Clear()

	assert.Equal(t, 0, len(buffer.chunks))
}

func TestAudioBuffer_Clear_ThenAdd(t *testing.T) {
	buffer := NewAudioBuffer(10)

	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 100, Data: []byte("data1"), Duration: 200})
	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 300, Data: []byte("data2"), Duration: 200})

	buffer.Clear()

	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 500, Data: []byte("data3"), Duration: 200})

	result := buffer.GetAll()

	assert.Equal(t, 1, len(result))
	assert.Equal(t, []byte("data3"), result[0].Data)
}

func TestAudioBuffer_Size_Empty(t *testing.T) {
	buffer := NewAudioBuffer(10)

	assert.Equal(t, 0, buffer.Size())
}

func TestAudioBuffer_Size_WithChunks(t *testing.T) {
	buffer := NewAudioBuffer(10)

	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 100, Data: []byte("data1"), Duration: 200})
	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 300, Data: []byte("data2"), Duration: 200})

	assert.Equal(t, 2, buffer.Size())
}

func TestAudioBuffer_Size_AfterClear(t *testing.T) {
	buffer := NewAudioBuffer(10)

	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 100, Data: []byte("data1"), Duration: 200})
	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 300, Data: []byte("data2"), Duration: 200})

	assert.Equal(t, 2, buffer.Size())

	buffer.Clear()

	assert.Equal(t, 0, buffer.Size())
}

func TestAudioBuffer_Size_Overwrite(t *testing.T) {
	buffer := NewAudioBuffer(2)

	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 100, Data: []byte("data1"), Duration: 200})
	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 300, Data: []byte("data2"), Duration: 200})
	buffer.Add(AudioChunk{ParticipantID: "p1", Timestamp: 500, Data: []byte("data3"), Duration: 200})

	assert.Equal(t, 2, buffer.Size())
}

func TestAudioBuffer_ConcurrentAdd(t *testing.T) {
	buffer := NewAudioBuffer(100)

	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			buffer.Add(AudioChunk{
				ParticipantID: "p1",
				Timestamp:     int64(id),
				Data:          []byte("data"),
				Duration:      200,
			})
		}(i)
	}

	wg.Wait()

	assert.Equal(t, 100, buffer.Size())
}

func TestAudioBuffer_ConcurrentGetAll(t *testing.T) {
	buffer := NewAudioBuffer(10)

	for i := 0; i < 50; i++ {
		buffer.Add(AudioChunk{
			ParticipantID: "p1",
			Timestamp:     int64(i),
			Data:          []byte("data"),
			Duration:      200,
		})
	}

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buffer.GetAll()
		}()
	}

	wg.Wait()

	assert.Equal(t, 50, buffer.Size())
}
