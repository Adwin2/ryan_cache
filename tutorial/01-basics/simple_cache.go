package main

import (
	"fmt"
	"sync"
	"time"
)

// SimpleCache 最简单的缓存实现
// 使用map存储数据，用读写锁保证线程安全
type SimpleCache struct {
	data map[string]interface{}
	mu   sync.RWMutex // 读写锁：多个读操作可以并发，写操作独占
}

// NewSimpleCache 创建新的简单缓存
func NewSimpleCache() *SimpleCache {
	return &SimpleCache{
		data: make(map[string]interface{}),
	}
}

// Get 获取缓存值
// 返回值和是否存在的标志
func (c *SimpleCache) Get(key string) (interface{}, bool) {
	c.mu.RLock() // 读锁：允许多个goroutine同时读取
	defer c.mu.RUnlock()
	
	value, exists := c.data[key]
	return value, exists
}

// Set 设置缓存值
func (c *SimpleCache) Set(key string, value interface{}) {
	c.mu.Lock() // 写锁：独占访问
	defer c.mu.Unlock()
	
	c.data[key] = value
}

// Delete 删除缓存值
func (c *SimpleCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.data, key)
}

// Size 返回缓存大小
func (c *SimpleCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return len(c.data)
}

// Clear 清空缓存
func (c *SimpleCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.data = make(map[string]interface{})
}

// ===== 带TTL的缓存实现 =====

// CacheItem 缓存项，包含值和过期时间
type CacheItem struct {
	Value     interface{}
	ExpiresAt time.Time
}

// IsExpired 检查是否过期
func (item *CacheItem) IsExpired() bool {
	return time.Now().After(item.ExpiresAt)
}

// CacheWithTTL 支持TTL的缓存
type CacheWithTTL struct {
	data    map[string]*CacheItem
	mu      sync.RWMutex
	stopCh  chan struct{}
	started bool
}
// 支持TTL(Time To Live)的缓存是什么意思: 带过期时间的缓存

// NewCacheWithTTL 创建支持TTL的缓存
func NewCacheWithTTL() *CacheWithTTL {
	cache := &CacheWithTTL{
		data:   make(map[string]*CacheItem),
		stopCh: make(chan struct{}),
	}
	
	// 启动清理goroutine
	go cache.cleanup()
	cache.started = true
	
	return cache
}

// Get 获取缓存值（检查过期）
func (c *CacheWithTTL) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	item, exists := c.data[key]
	c.mu.RUnlock()
	
	if !exists {
		return nil, false
	}
	
	// 检查是否过期
	if item.IsExpired() {
		c.Delete(key) // 删除过期项
		return nil, false
	}
	
	return item.Value, true
}

// Set 设置缓存值（带TTL）
func (c *CacheWithTTL) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	expiresAt := time.Now().Add(ttl)
	c.data[key] = &CacheItem{
		Value:     value,
		ExpiresAt: expiresAt,
	}
}

// Delete 删除缓存值
func (c *CacheWithTTL) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.data, key)
}

// Size 返回缓存大小
func (c *CacheWithTTL) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return len(c.data)
}

// Close 关闭缓存（停止清理goroutine）
func (c *CacheWithTTL) Close() {
	if c.started {
		close(c.stopCh)
		c.started = false
	}
}

// cleanup 定期清理过期项
func (c *CacheWithTTL) cleanup() {
	ticker := time.NewTicker(1 * time.Minute) // 每分钟清理一次
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.stopCh:
			return
		}
	}
}

// removeExpired 移除过期项
func (c *CacheWithTTL) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	for key, item := range c.data {
		if now.After(item.ExpiresAt) {
			delete(c.data, key)
			fmt.Printf("清理过期缓存: %s\n", key)
		}
	}
}

// ===== 缓存统计信息 =====

// CacheStats 缓存统计
type CacheStats struct {
	Hits   int64 // 命中次数
	Misses int64 // 未命中次数
}

// HitRate 计算命中率
func (s *CacheStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total)
}

// StatsCache 带统计功能的缓存
type StatsCache struct {
	*SimpleCache
	stats CacheStats
	mu    sync.RWMutex
}

// NewStatsCache 创建带统计的缓存
func NewStatsCache() *StatsCache {
	return &StatsCache{
		SimpleCache: NewSimpleCache(),
	}
}

// Get 获取值并更新统计
func (c *StatsCache) Get(key string) (interface{}, bool) {
	value, exists := c.SimpleCache.Get(key)
	
	c.mu.Lock()
	if exists {
		c.stats.Hits++
	} else {
		c.stats.Misses++
	}
	c.mu.Unlock()
	
	return value, exists
}

// GetStats 获取统计信息
func (c *StatsCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.stats
}

// ResetStats 重置统计信息
func (c *StatsCache) ResetStats() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.stats = CacheStats{}
}
