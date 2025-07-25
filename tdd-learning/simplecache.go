package main

import (
	"sync"
	"time"
)

type SimpleCache struct {
	data map[string]string
	dataTTL map[string]time.Time
	mu   sync.RWMutex  // 读写锁
}

func NewSimpleCache() *SimpleCache {
	return &SimpleCache{
		data: make(map[string]string),
		dataTTL: make(map[string]time.Time),
	}
}

func (c *SimpleCache) Set(key, value string) {
	c.mu.Lock()         // 写操作需要独占锁
	defer c.mu.Unlock()
	c.data[key] = value
}

func (c *SimpleCache)SetWithTTL(key, value string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
	c.dataTTL[key] = time.Now().Add(ttl)
}

// 将读写锁的操作分开
func (c *SimpleCache) Get(key string) (string, bool) {
	c.mu.RLock()        // 读操作使用读锁，允许并发读
	val, exists := c.data[key]
	ttl, hasTTL := c.dataTTL[key]
	c.mu.RUnlock()

	if hasTTL && time.Now().After(ttl) {
		c.mu.Lock()
		delete(c.data, key)
		delete(c.dataTTL, key)
		c.mu.Unlock()
		return "", false
	}
	return val, exists
}

func (c *SimpleCache) Delete(key string) bool {
	c.mu.Lock()         // 写操作需要独占锁
	defer c.mu.Unlock()
	_, exists := c.data[key]
	if exists {
		delete(c.data, key)
		delete(c.dataTTL, key)
		return true
	}
	return false
}

func (c *SimpleCache) Size() int {
	c.mu.RLock()        // 读操作使用读锁
	defer c.mu.RUnlock()
	return len(c.data)
}