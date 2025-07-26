package main

import (
	"fmt"
	"sync"
	"time"
)

// LocalCacheItem 本地缓存项
type LocalCacheItem struct {
	Value     interface{}
	ExpiresAt time.Time
	AccessCount int64
	LastAccess  time.Time
}

// LocalCache 本地缓存实现
type LocalCache struct {
	data     map[string]*LocalCacheItem
	maxSize  int
	mu       sync.RWMutex
	metrics  *LocalCacheMetrics
}

// LocalCacheMetrics 本地缓存指标
type LocalCacheMetrics struct {
	Hits        int64
	Misses      int64
	Evictions   int64
	TotalAccess int64
	mu          sync.RWMutex
}

// NewLocalCache 创建本地缓存
func NewLocalCache(maxSize int) *LocalCache {
	lc := &LocalCache{
		data:    make(map[string]*LocalCacheItem),
		maxSize: maxSize,
		metrics: &LocalCacheMetrics{},
	}
	
	// 启动清理goroutine
	go lc.cleanupExpired()
	
	return lc
}

// Get 获取缓存值
func (lc *LocalCache) Get(key string) (interface{}, bool) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	
	lc.metrics.mu.Lock()
	lc.metrics.TotalAccess++
	lc.metrics.mu.Unlock()
	
	item, exists := lc.data[key]
	if !exists {
		lc.recordMiss()
		return nil, false
	}
	
	// 检查是否过期
	if time.Now().After(item.ExpiresAt) {
		lc.recordMiss()
		return nil, false
	}
	
	// 更新访问信息
	item.AccessCount++
	item.LastAccess = time.Now()
	
	lc.recordHit()
	return item.Value, true
}

// Set 设置缓存值
func (lc *LocalCache) Set(key string, value interface{}, ttl time.Duration) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	// 检查容量限制
	if len(lc.data) >= lc.maxSize {
		lc.evictLRU()
	}
	
	lc.data[key] = &LocalCacheItem{
		Value:       value,
		ExpiresAt:   time.Now().Add(ttl),
		AccessCount: 0,
		LastAccess:  time.Now(),
	}
}

// Delete 删除缓存项
func (lc *LocalCache) Delete(key string) {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	delete(lc.data, key)
}

// Clear 清空缓存
func (lc *LocalCache) Clear() {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	
	lc.data = make(map[string]*LocalCacheItem)
}

// Size 获取缓存大小
func (lc *LocalCache) Size() int {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	
	return len(lc.data)
}

// GetMetrics 获取缓存指标
func (lc *LocalCache) GetMetrics() LocalCacheMetrics {
	lc.metrics.mu.RLock()
	defer lc.metrics.mu.RUnlock()
	
	return *lc.metrics
}

// GetHitRate 获取命中率
func (lc *LocalCache) GetHitRate() float64 {
	lc.metrics.mu.RLock()
	defer lc.metrics.mu.RUnlock()
	
	if lc.metrics.TotalAccess == 0 {
		return 0
	}
	
	return float64(lc.metrics.Hits) / float64(lc.metrics.TotalAccess)
}

// recordHit 记录命中
func (lc *LocalCache) recordHit() {
	lc.metrics.mu.Lock()
	defer lc.metrics.mu.Unlock()
	
	lc.metrics.Hits++
}

// recordMiss 记录未命中
func (lc *LocalCache) recordMiss() {
	lc.metrics.mu.Lock()
	defer lc.metrics.mu.Unlock()
	
	lc.metrics.Misses++
}

// evictLRU 使用LRU策略淘汰数据
func (lc *LocalCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time = time.Now()
	
	// 找到最久未访问的项
	for key, item := range lc.data {
		if item.LastAccess.Before(oldestTime) {
			oldestTime = item.LastAccess
			oldestKey = key
		}
	}
	
	if oldestKey != "" {
		delete(lc.data, oldestKey)
		lc.metrics.mu.Lock()
		lc.metrics.Evictions++
		lc.metrics.mu.Unlock()
	}
}

// cleanupExpired 清理过期数据
func (lc *LocalCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		lc.mu.Lock()
		now := time.Now()
		
		for key, item := range lc.data {
			if now.After(item.ExpiresAt) {
				delete(lc.data, key)
			}
		}
		
		lc.mu.Unlock()
	}
}

// GetTopKeys 获取访问最频繁的键
func (lc *LocalCache) GetTopKeys(limit int) []KeyStats {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	
	type keyAccess struct {
		key   string
		count int64
	}
	
	var keys []keyAccess
	for key, item := range lc.data {
		keys = append(keys, keyAccess{key: key, count: item.AccessCount})
	}
	
	// 简单排序（生产环境建议使用更高效的排序）
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i].count < keys[j].count {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	
	result := make([]KeyStats, 0, limit)
	for i := 0; i < limit && i < len(keys); i++ {
		result = append(result, KeyStats{
			Key:         keys[i].key,
			AccessCount: keys[i].count,
		})
	}
	
	return result
}

// KeyStats 键统计信息
type KeyStats struct {
	Key         string
	AccessCount int64
}

// 演示本地缓存功能
func DemoLocalCache() {
	fmt.Println("=== 本地缓存演示 ===")
	
	// 创建本地缓存
	cache := NewLocalCache(100)
	
	// 设置一些数据
	fmt.Println("\n1. 设置缓存数据:")
	cache.Set("user:1", "张三", 5*time.Minute)
	cache.Set("user:2", "李四", 5*time.Minute)
	cache.Set("user:3", "王五", 5*time.Minute)
	cache.Set("product:1", "iPhone 15", 10*time.Minute)
	cache.Set("product:2", "MacBook Pro", 10*time.Minute)
	
	fmt.Printf("缓存大小: %d\n", cache.Size())
	
	// 测试读取
	fmt.Println("\n2. 测试缓存读取:")
	testKeys := []string{"user:1", "user:2", "user:999", "product:1"}
	
	for _, key := range testKeys {
		if value, exists := cache.Get(key); exists {
			fmt.Printf("✅ 缓存命中: %s = %v\n", key, value)
		} else {
			fmt.Printf("❌ 缓存未命中: %s\n", key)
		}
	}
	
	// 显示指标
	fmt.Println("\n3. 缓存指标:")
	metrics := cache.GetMetrics()
	fmt.Printf("总访问次数: %d\n", metrics.TotalAccess)
	fmt.Printf("命中次数: %d\n", metrics.Hits)
	fmt.Printf("未命中次数: %d\n", metrics.Misses)
	fmt.Printf("命中率: %.2f%%\n", cache.GetHitRate()*100)
	
	// 测试热点数据
	fmt.Println("\n4. 模拟热点访问:")
	for i := 0; i < 10; i++ {
		cache.Get("user:1") // 频繁访问user:1
	}
	for i := 0; i < 5; i++ {
		cache.Get("product:1") // 中等访问product:1
	}
	
	// 显示热点数据
	fmt.Println("\n5. 热点数据统计:")
	topKeys := cache.GetTopKeys(3)
	for i, stat := range topKeys {
		fmt.Printf("%d. %s: 访问%d次\n", i+1, stat.Key, stat.AccessCount)
	}
	
	// 测试容量限制
	fmt.Println("\n6. 测试容量限制:")
	smallCache := NewLocalCache(3)
	
	// 添加超过容量的数据
	smallCache.Set("a", "value_a", 5*time.Minute)
	smallCache.Set("b", "value_b", 5*time.Minute)
	smallCache.Set("c", "value_c", 5*time.Minute)
	fmt.Printf("添加3个项后，缓存大小: %d\n", smallCache.Size())
	
	smallCache.Set("d", "value_d", 5*time.Minute)
	fmt.Printf("添加第4个项后，缓存大小: %d (触发LRU淘汰)\n", smallCache.Size())
	
	// 检查哪个被淘汰了
	keys := []string{"a", "b", "c", "d"}
	for _, key := range keys {
		if _, exists := smallCache.Get(key); exists {
			fmt.Printf("✅ %s 仍在缓存中\n", key)
		} else {
			fmt.Printf("❌ %s 已被淘汰\n", key)
		}
	}
	
	fmt.Println("\n💡 本地缓存特点:")
	fmt.Println("   1. 极低延迟 (纳秒级)")
	fmt.Println("   2. 无网络开销")
	fmt.Println("   3. 容量受内存限制")
	fmt.Println("   4. 进程重启数据丢失")
	fmt.Println("   5. 适合热点数据缓存")
}
