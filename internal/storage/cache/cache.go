package cache

import (
	"sync"
	"time"
)

type Cache struct {
	mu    sync.RWMutex
	items map[string]*item
}

type item struct {
	value     interface{}
	expiresAt time.Time
}

func New() *Cache {
	c := &Cache{
		items: make(map[string]*item),
	}
	go c.cleanup()
	return c
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &item{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.value, true
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

func (c *Cache) Exists(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return false
	}

	return time.Now().Before(item.expiresAt)
}

func (c *Cache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, item := range c.items {
			if now.After(item.expiresAt) {
				delete(c.items, key)
			}
		}
		c.mu.Unlock()
	}
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*item)
}

func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.items)
}
