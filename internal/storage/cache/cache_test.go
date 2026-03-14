package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCache_New(t *testing.T) {
	c := New()
	assert.NotNil(t, c)
	assert.NotNil(t, c.items)
}

func TestCache_SetAndGet(t *testing.T) {
	c := New()
	c.Set("key1", "value1", 1*time.Hour)

	val, exists := c.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", val)
}

func TestCache_GetNonExistent(t *testing.T) {
	c := New()
	val, exists := c.Get("nonexistent")
	assert.False(t, exists)
	assert.Nil(t, val)
}

func TestCache_SetAndGetWithTTL(t *testing.T) {
	c := New()
	c.Set("key1", "value1", 100*time.Millisecond)

	val, exists := c.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", val)

	time.Sleep(150 * time.Millisecond)

	val, exists = c.Get("key1")
	assert.False(t, exists)
	assert.Nil(t, val)
}

func TestCache_SetAndGetExpiredImmediately(t *testing.T) {
	c := New()
	c.Set("key1", "value1", 0)

	val, exists := c.Get("key1")
	assert.False(t, exists)
	assert.Nil(t, val)
}

func TestCache_SetAndGetWithNegativeTTL(t *testing.T) {
	c := New()
	c.Set("key1", "value1", -1*time.Hour)

	val, exists := c.Get("key1")
	assert.False(t, exists)
	assert.Nil(t, val)
}

func TestCache_Delete(t *testing.T) {
	c := New()
	c.Set("key1", "value1", 1*time.Hour)
	c.Delete("key1")

	_, exists := c.Get("key1")
	assert.False(t, exists)
}

func TestCache_DeleteNonExistent(t *testing.T) {
	c := New()
	c.Set("key1", "value1", 1*time.Hour)
	c.Delete("key2")

	_, exists := c.Get("key1")
	assert.True(t, exists)
}

func TestCache_Exists(t *testing.T) {
	c := New()

	t.Run("KeyExistsAndNotExpired", func(t *testing.T) {
		c.Set("key1", "value1", 1*time.Hour)
		assert.True(t, c.Exists("key1"))
	})

	t.Run("KeyDoesNotExist", func(t *testing.T) {
		assert.False(t, c.Exists("key2"))
	})

	t.Run("KeyExpired", func(t *testing.T) {
		c.Set("key1", "value1", 100*time.Millisecond)
		time.Sleep(150 * time.Millisecond)
		assert.False(t, c.Exists("key1"))
	})

	t.Run("KeyExpiredImmediately", func(t *testing.T) {
		c.Set("key1", "value1", 0)
		assert.False(t, c.Exists("key1"))
	})
}

func TestCache_Clear(t *testing.T) {
	c := New()
	c.Set("key1", "value1", 1*time.Hour)
	c.Set("key2", "value2", 1*time.Hour)
	c.Clear()

	assert.Equal(t, 0, c.Size())
	_, exists := c.Get("key1")
	assert.False(t, exists)
	_, exists = c.Get("key2")
	assert.False(t, exists)
}

func TestCache_Size(t *testing.T) {
	c := New()
	assert.Equal(t, 0, c.Size())

	c.Set("key1", "value1", 1*time.Hour)
	assert.Equal(t, 1, c.Size())

	c.Set("key2", "value2", 1*time.Hour)
	assert.Equal(t, 2, c.Size())

	c.Delete("key1")
	assert.Equal(t, 1, c.Size())
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := New()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.Set("key", n, 1*time.Hour)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Get("key")
		}()
	}

	wg.Wait()

	val, exists := c.Get("key")
	assert.True(t, exists)
	assert.NotNil(t, val)
}

func TestCache_Values(t *testing.T) {
	c := New()

	val1 := "value1"
	val2 := 123
	val3 := []int{1, 2, 3}
	val4 := struct{ Name string }{"test"}

	c.Set("key1", val1, 1*time.Hour)
	c.Set("key2", val2, 1*time.Hour)
	c.Set("key3", val3, 1*time.Hour)
	c.Set("key4", val4, 1*time.Hour)

	testCases := []struct {
		target interface{}
		key    string
	}{
		{key: "key1", target: val1},
		{key: "key2", target: val2},
		{key: "key3", target: val3},
		{key: "key4", target: val4},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			val, exists := c.Get(tc.key)
			assert.True(t, exists)
			assert.Equal(t, tc.target, val)
		})
	}
}

func TestCache_UpdateExistingKey(t *testing.T) {
	c := New()
	c.Set("key1", "value1", 1*time.Hour)

	val, exists := c.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value1", val)

	c.Set("key1", "value2", 1*time.Hour)

	val, exists = c.Get("key1")
	assert.True(t, exists)
	assert.Equal(t, "value2", val)
}

func TestCache_SetOverwrite(t *testing.T) {
	c := New()
	c.Set("key1", "value1", 1*time.Hour)

	val1, exists1 := c.Get("key1")
	assert.True(t, exists1)
	assert.Equal(t, "value1", val1)

	c.Set("key1", "value2", 1*time.Hour)

	val2, exists2 := c.Get("key1")
	assert.True(t, exists2)
	assert.Equal(t, "value2", val2)
	assert.NotEqual(t, val1, val2)
}

func TestCache_ClearWithExpiry(t *testing.T) {
	c := New()
	c.Set("key1", "value1", 100*time.Millisecond)
	c.Set("key2", "value2", 1*time.Hour)

	time.Sleep(150 * time.Millisecond)

	c.Clear()

	assert.Equal(t, 0, c.Size())
	_, exists := c.Get("key1")
	assert.False(t, exists)
	_, exists = c.Get("key2")
	assert.False(t, exists)
}

func TestCache_NilValue(t *testing.T) {
	c := New()
	c.Set("key", nil, 1*time.Hour)

	val, exists := c.Get("key")
	assert.True(t, exists)
	assert.Nil(t, val)
}

func TestCache_EmptyKey(t *testing.T) {
	c := New()
	c.Set("", "value", 1*time.Hour)

	_, exists := c.Get("")
	assert.True(t, exists)
}

func TestCache_MultipleGetsSameKey(t *testing.T) {
	c := New()
	c.Set("key", "value", 1*time.Hour)

	for i := 0; i < 10; i++ {
		val, exists := c.Get("key")
		assert.True(t, exists)
		assert.Equal(t, "value", val)
	}
}

func TestCache_DeleteAndAdd(t *testing.T) {
	c := New()
	c.Set("key1", "value1", 1*time.Hour)
	c.Set("key2", "value2", 1*time.Hour)

	c.Delete("key1")

	_, exists := c.Get("key1")
	assert.False(t, exists)

	val, exists := c.Get("key2")
	assert.True(t, exists)
	assert.Equal(t, "value2", val)
}

func TestCache_CleanupInterval(t *testing.T) {
	c := New()
	c.Set("key1", "value1", 100*time.Millisecond)
	c.Set("key2", "value2", 100*time.Millisecond)

	val1, exists1 := c.Get("key1")
	assert.True(t, exists1)
	assert.Equal(t, "value1", val1)

	val2, exists2 := c.Get("key2")
	assert.True(t, exists2)
	assert.Equal(t, "value2", val2)

	time.Sleep(150 * time.Millisecond)

	val1, exists1 = c.Get("key1")
	assert.False(t, exists1)
	assert.Nil(t, val1)

	val2, exists2 = c.Get("key2")
	assert.False(t, exists2)
	assert.Nil(t, val2)
}

func TestCache_ClearDuringCleanup(t *testing.T) {
	c := New()
	c.Set("key1", "value1", 100*time.Millisecond)
	c.Set("key2", "value2", 1*time.Hour)

	time.Sleep(150 * time.Millisecond)

	c.Clear()

	assert.Equal(t, 0, c.Size())
}

func TestCache_HugeNumberOfKeys(t *testing.T) {
	c := New()

	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key%d", i)
		c.Set(key, i, 1*time.Hour)
	}

	assert.Equal(t, 10000, c.Size())

	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key%d", i)
		val, exists := c.Get(key)
		assert.True(t, exists)
		assert.Equal(t, i, val)
	}
}

func TestCache_SetWithZeroTTL(t *testing.T) {
	c := New()
	c.Set("key", "value", 0)

	val, exists := c.Get("key")
	assert.False(t, exists)
	assert.Nil(t, val)
}

func TestCache_SetWithVeryLargeTTL(t *testing.T) {
	c := New()
	c.Set("key", "value", 365*24*time.Hour)

	val, exists := c.Get("key")
	assert.True(t, exists)
	assert.Equal(t, "value", val)
}

func TestCache_SetWithVerySmallTTL(t *testing.T) {
	c := New()
	c.Set("key", "value", 1*time.Nanosecond)

	time.Sleep(2 * time.Nanosecond)

	val, exists := c.Get("key")
	assert.False(t, exists)
	assert.Nil(t, val)
}

func TestCache_ExistingKeyExpiredAfterAddition(t *testing.T) {
	c := New()
	c.Set("key", "value1", 100*time.Millisecond)

	val, exists := c.Get("key")
	assert.True(t, exists)
	assert.Equal(t, "value1", val)

	c.Set("key", "value2", 100*time.Millisecond)

	time.Sleep(150 * time.Millisecond)

	val, exists = c.Get("key")
	assert.False(t, exists)
	assert.Nil(t, val)
}

func TestCache_DeleteWithNonExistentKey(t *testing.T) {
	c := New()
	c.Set("key1", "value1", 1*time.Hour)

	c.Delete("key1")
	c.Delete("key2")
	c.Delete("key1")

	val, exists := c.Get("key1")
	assert.False(t, exists)
	assert.Nil(t, val)
}
