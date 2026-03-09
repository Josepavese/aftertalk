package cache

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewProcessingQueue(t *testing.T) {
	q := NewProcessingQueue(5)
	assert.NotNil(t, q)
	assert.NotNil(t, q.jobs)
	assert.NotNil(t, q.quit)
}

func TestNewProcessingQueue_ZeroWorkers(t *testing.T) {
	q := NewProcessingQueue(0)
	assert.NotNil(t, q)
}

func TestNewProcessingQueue_NegativeWorkers(t *testing.T) {
	q := NewProcessingQueue(-1)
	assert.NotNil(t, q)
}

func TestProcessingQueue_Enqueue(t *testing.T) {
	q := NewProcessingQueue(5)

	job := Job{
		Type:      "transcription",
		SessionID: "session-123",
		Payload:   json.RawMessage(`{"test": "data"}`),
	}

	err := q.Enqueue(job)
	assert.NoError(t, err)
	assert.Equal(t, 1, q.Size())
}

func TestProcessingQueue_EnqueueDuplicateJob(t *testing.T) {
	q := NewProcessingQueue(5)

	job := Job{
		Type:      "transcription",
		SessionID: "session-123",
		Payload:   json.RawMessage(`{"test": "data"}`),
	}

	err1 := q.Enqueue(job)
	err2 := q.Enqueue(job)

	assert.NoError(t, err1)
	assert.NoError(t, err2)
	assert.Equal(t, 2, q.Size())
}

func TestProcessingQueue_EnqueueFullQueue(t *testing.T) {
	q := NewProcessingQueue(2)

	for i := 0; i < 4; i++ {
		job := Job{
			Type:      "test",
			SessionID: "session-123",
			Payload:   json.RawMessage(`{"test": "data"}`),
		}
		err := q.Enqueue(job)
		assert.NoError(t, err)
	}

	assert.Equal(t, 4, q.Size())

	job := Job{
		Type:      "test",
		SessionID: "session-123",
		Payload:   json.RawMessage(`{"test": "data"}`),
	}

	err := q.Enqueue(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "queue is full")
}

func TestProcessingQueue_EnqueueMultipleTypes(t *testing.T) {
	q := NewProcessingQueue(5)

	jobTypes := []string{"transcription", "minutes", "webhook", "cleanup"}

	for _, jobType := range jobTypes {
		job := Job{
			Type:      jobType,
			SessionID: "session-123",
			Payload:   json.RawMessage(`{"test": "data"}`),
		}
		err := q.Enqueue(job)
		assert.NoError(t, err)
	}

	assert.Equal(t, 4, q.Size())
}

func TestProcessingQueue_Dequeue(t *testing.T) {
	q := NewProcessingQueue(5)

	job := Job{
		Type:      "transcription",
		SessionID: "session-123",
		Payload:   json.RawMessage(`{"test": "data"}`),
	}

	q.Enqueue(job)

	dequeued, exists := q.Dequeue()
	assert.True(t, exists)
	assert.Equal(t, job.Type, dequeued.Type)
	assert.Equal(t, job.SessionID, dequeued.SessionID)
	assert.Equal(t, job.Payload, dequeued.Payload)
	assert.Equal(t, 0, q.Size())
}

func TestProcessingQueue_DequeueEmptyQueue(t *testing.T) {
	q := NewProcessingQueue(5)
	q.Close()

	job, exists := q.Dequeue()
	assert.False(t, exists)
	assert.Equal(t, Job{}, job)
}

func TestProcessingQueue_DequeueOrder(t *testing.T) {
	q := NewProcessingQueue(10)

	jobs := []Job{
		{Type: "test1", SessionID: "session-1", Payload: json.RawMessage(`{"test": "data"}`)},
		{Type: "test2", SessionID: "session-2", Payload: json.RawMessage(`{"test": "data"}`)},
		{Type: "test3", SessionID: "session-3", Payload: json.RawMessage(`{"test": "data"}`)},
	}

	for _, job := range jobs {
		q.Enqueue(job)
	}

	for i := 0; i < len(jobs); i++ {
		dequeued, exists := q.Dequeue()
		assert.True(t, exists)
		assert.Equal(t, jobs[i].Type, dequeued.Type)
	}

	assert.Equal(t, 0, q.Size())
}

func TestProcessingQueue_DequeueAfterEnqueue(t *testing.T) {
	q := NewProcessingQueue(5)

	job1 := Job{
		Type:      "test1",
		SessionID: "session-1",
		Payload:   json.RawMessage(`{"test": "data1"}`),
	}
	q.Enqueue(job1)

	time.Sleep(10 * time.Millisecond)

	job2 := Job{
		Type:      "test2",
		SessionID: "session-2",
		Payload:   json.RawMessage(`{"test": "data2"}`),
	}
	q.Enqueue(job2)

	dequeued1, exists1 := q.Dequeue()
	assert.True(t, exists1)
	assert.Equal(t, job1.Type, dequeued1.Type)

	dequeued2, exists2 := q.Dequeue()
	assert.True(t, exists2)
	assert.Equal(t, job2.Type, dequeued2.Type)
}

func TestProcessingQueue_DequeueAll(t *testing.T) {
	q := NewProcessingQueue(10)

	for i := 0; i < 10; i++ {
		job := Job{
			Type:      "test",
			SessionID: "session-123",
			Payload:   json.RawMessage(`{"test": "data"}`),
		}
		q.Enqueue(job)
	}

	for i := 0; i < 10; i++ {
		job, exists := q.Dequeue()
		assert.True(t, exists)
		assert.Equal(t, "test", job.Type)
	}

	assert.Equal(t, 0, q.Size())
}

func TestProcessingQueue_DequeueAfterClose(t *testing.T) {
	q := NewProcessingQueue(5)

	q.Enqueue(Job{
		Type:      "test",
		SessionID: "session-123",
		Payload:   json.RawMessage(`{"test": "data"}`),
	})

	q.Close()

	_, exists := q.Dequeue()
	assert.False(t, exists)
}

func TestProcessingQueue_Close(t *testing.T) {
	q := NewProcessingQueue(5)

	job := Job{
		Type:      "test",
		SessionID: "session-123",
		Payload:   json.RawMessage(`{"test": "data"}`),
	}

	q.Enqueue(job)
	q.Close()

	assert.Equal(t, 0, q.Size())
}

func TestProcessingQueue_CloseTwice(t *testing.T) {
	q := NewProcessingQueue(5)

	q.Close()
	q.Close()

	assert.Equal(t, 0, q.Size())
}

func TestProcessingQueue_ConcurrentEnqueue(t *testing.T) {
	q := NewProcessingQueue(50)

	var wg sync.WaitGroup
	numJobs := 100

	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			job := Job{
				Type:      "test",
				SessionID: "session-123",
				Payload:   json.RawMessage(`{"test": "data"}`),
			}
			q.Enqueue(job)
		}(i)
	}

	wg.Wait()
	assert.Equal(t, numJobs, q.Size())
}

func TestProcessingQueue_ConcurrentEnqueueAndDequeue(t *testing.T) {
	q := NewProcessingQueue(25)

	var wg sync.WaitGroup
	numJobs := 50

	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			job := Job{
				Type:      "test",
				SessionID: "session-123",
				Payload:   json.RawMessage(`{"test": "data"}`),
			}
			q.Enqueue(job)
		}(i)
	}

	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q.Dequeue()
		}()
	}

	wg.Wait()
	assert.Equal(t, 0, q.Size())
}

func TestProcessingQueue_EnqueueLargePayload(t *testing.T) {
	q := NewProcessingQueue(5)

	// Create a large payload
	largePayload := make(json.RawMessage, 10000)
	for i := range largePayload {
		largePayload[i] = byte(i % 256)
	}

	job := Job{
		Type:      "test",
		SessionID: "session-123",
		Payload:   largePayload,
	}

	err := q.Enqueue(job)
	assert.NoError(t, err)
	assert.Equal(t, 1, q.Size())

	dequeued, exists := q.Dequeue()
	assert.True(t, exists)
	assert.Equal(t, largePayload, dequeued.Payload)
}

func TestProcessingQueue_MultipleSessions(t *testing.T) {
	q := NewProcessingQueue(10)

	sessions := []string{"session-1", "session-2", "session-3", "session-4", "session-5"}

	for _, sessionID := range sessions {
		job := Job{
			Type:      "transcription",
			SessionID: sessionID,
			Payload:   json.RawMessage(`{"test": "data"}`),
		}
		err := q.Enqueue(job)
		assert.NoError(t, err)
	}

	for _, sessionID := range sessions {
		job, exists := q.Dequeue()
		assert.True(t, exists)
		assert.Equal(t, sessionID, job.SessionID)
	}

	assert.Equal(t, 0, q.Size())
}

func TestProcessingQueue_EmptyJob(t *testing.T) {
	q := NewProcessingQueue(5)

	job := Job{
		Type:      "",
		SessionID: "",
		Payload:   nil,
	}

	err := q.Enqueue(job)
	assert.NoError(t, err)

	dequeued, exists := q.Dequeue()
	assert.True(t, exists)
	assert.Equal(t, "", dequeued.Type)
	assert.Equal(t, "", dequeued.SessionID)
	assert.Equal(t, json.RawMessage(nil), dequeued.Payload)
}

func TestProcessingQueue_NilPayload(t *testing.T) {
	q := NewProcessingQueue(5)

	job := Job{
		Type:      "test",
		SessionID: "session-123",
		Payload:   nil,
	}

	err := q.Enqueue(job)
	assert.NoError(t, err)

	dequeued, exists := q.Dequeue()
	assert.True(t, exists)
	assert.Equal(t, job.Payload, dequeued.Payload)
}

func TestProcessingQueue_ZeroPayload(t *testing.T) {
	q := NewProcessingQueue(5)

	job := Job{
		Type:      "test",
		SessionID: "session-123",
		Payload:   []byte{},
	}

	err := q.Enqueue(job)
	assert.NoError(t, err)

	dequeued, exists := q.Dequeue()
	assert.True(t, exists)
	assert.Equal(t, job.Payload, dequeued.Payload)
}

func TestProcessingQueue_UniquePayload(t *testing.T) {
	q := NewProcessingQueue(5)

	job1 := Job{
		Type:      "test",
		SessionID: "session-123",
		Payload:   []byte(`{"data": "1"}`),
	}

	job2 := Job{
		Type:      "test",
		SessionID: "session-123",
		Payload:   []byte(`{"data": "2"}`),
	}

	err1 := q.Enqueue(job1)
	err2 := q.Enqueue(job2)

	assert.NoError(t, err1)
	assert.NoError(t, err2)

	dequeued1, exists1 := q.Dequeue()
	assert.True(t, exists1)
	assert.Equal(t, job1.Payload, dequeued1.Payload)

	dequeued2, exists2 := q.Dequeue()
	assert.True(t, exists2)
	assert.Equal(t, job2.Payload, dequeued2.Payload)
}

func TestProcessingQueue_HugeQueue(t *testing.T) {
	q := NewProcessingQueue(500)

	for i := 0; i < 1000; i++ {
		job := Job{
			Type:      "test",
			SessionID: "session-123",
			Payload:   json.RawMessage(`{"test": "data"}`),
		}
		err := q.Enqueue(job)
		assert.NoError(t, err)
	}

	assert.Equal(t, 1000, q.Size())

	for i := 0; i < 1000; i++ {
		job, exists := q.Dequeue()
		assert.True(t, exists)
		assert.Equal(t, "test", job.Type)
	}

	assert.Equal(t, 0, q.Size())
}

func TestProcessingQueue_LargePayloads(t *testing.T) {
	q := NewProcessingQueue(50)

	for i := 0; i < 100; i++ {
		largePayload := make(json.RawMessage, 1000)
		for j := range largePayload {
			largePayload[j] = byte(j % 256)
		}

		job := Job{
			Type:      "test",
			SessionID: "session-123",
			Payload:   largePayload,
		}

		err := q.Enqueue(job)
		assert.NoError(t, err)
	}

	assert.Equal(t, 100, q.Size())

	for i := 0; i < 100; i++ {
		job, exists := q.Dequeue()
		assert.True(t, exists)
		assert.Equal(t, 1000, len(job.Payload))
	}

	assert.Equal(t, 0, q.Size())
}

func TestProcessingQueue_RapidEnqueueAndDequeue(t *testing.T) {
	q := NewProcessingQueue(500)

	var wg sync.WaitGroup
	iterations := 1000

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			job := Job{
				Type:      "test",
				SessionID: "session-123",
				Payload:   json.RawMessage(`{"test": "data"}`),
			}
			q.Enqueue(job)
		}(i)
	}

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q.Dequeue()
		}()
	}

	wg.Wait()
	assert.Equal(t, 0, q.Size())
}

func TestProcessingQueue_CloseEmptyQueue(t *testing.T) {
	q := NewProcessingQueue(5)
	q.Close()
	assert.Equal(t, 0, q.Size())
}

func TestProcessingQueue_DequeueAfterEnqueueAndClose(t *testing.T) {
	q := NewProcessingQueue(5)

	q.Enqueue(Job{
		Type:      "test",
		SessionID: "session-123",
		Payload:   json.RawMessage(`{"test": "data"}`),
	})

	q.Close()

	_, exists := q.Dequeue()
	assert.False(t, exists)
}

func TestProcessingQueue_EnqueueAfterClose(t *testing.T) {
	q := NewProcessingQueue(5)

	q.Close()

	err := q.Enqueue(Job{
		Type:      "test",
		SessionID: "session-123",
		Payload:   json.RawMessage(`{"test": "data"}`),
	})

	assert.Error(t, err)
}
