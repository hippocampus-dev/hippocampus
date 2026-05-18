package swr

import (
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type entry[V any] struct {
	value       V
	createdAt   time.Time
	staleAfter  time.Duration
	expireAfter time.Duration
}

type FetchResult[V any] struct {
	Value       V
	StaleAfter  time.Duration
	ExpireAfter time.Duration
}

type Cache[V any] struct {
	entries map[string]*entry[V]
	mu      sync.RWMutex
	group   singleflight.Group
}

func New[V any]() *Cache[V] {
	return &Cache[V]{
		entries: make(map[string]*entry[V]),
	}
}

func (c *Cache[V]) Get(key string, f func() (FetchResult[V], error)) (V, error) {
	c.mu.RLock()
	if e, ok := c.entries[key]; ok {
		elapsed := time.Since(e.createdAt)

		if elapsed < e.staleAfter {
			value := e.value
			c.mu.RUnlock()
			return value, nil
		}

		if elapsed < e.expireAfter {
			value := e.value
			c.mu.RUnlock()
			go func() {
				_, _ = c.refresh(key, f)
			}()
			return value, nil
		}
	}
	c.mu.RUnlock()

	return c.refresh(key, f)
}

func (c *Cache[V]) refresh(key string, f func() (FetchResult[V], error)) (V, error) {
	result, err, _ := c.group.Do(key, func() (interface{}, error) {
		r, err := f()
		if err != nil {
			return nil, err
		}

		c.mu.Lock()
		c.entries[key] = &entry[V]{value: r.Value, createdAt: time.Now(), staleAfter: r.StaleAfter, expireAfter: r.ExpireAfter}
		c.mu.Unlock()

		return r.Value, nil
	})
	if err != nil {
		var zero V
		return zero, err
	}

	return result.(V), nil
}
