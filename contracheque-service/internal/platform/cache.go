package platform

import (
	"sync"
	"time"
)

// TTLCache é um cache em memória simples, equivalente ao @Cacheable do legado.
type TTLCache[V any] struct {
	ttl   time.Duration
	mu    sync.Mutex
	items map[string]ttlItem[V]
}

type ttlItem[V any] struct {
	value   V
	expires time.Time
}

func NewTTLCache[V any](ttl time.Duration) *TTLCache[V] {
	return &TTLCache[V]{ttl: ttl, items: map[string]ttlItem[V]{}}
}

func (c *TTLCache[V]) Get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[key]
	if !ok || time.Now().After(item.expires) {
		delete(c.items, key)
		var zero V
		return zero, false
	}
	return item.value, true
}

func (c *TTLCache[V]) Set(key string, value V) {
	if c.ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = ttlItem[V]{value: value, expires: time.Now().Add(c.ttl)}
}
