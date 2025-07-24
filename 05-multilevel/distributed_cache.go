package main

import (
	"fmt"
	"sync"
	"time"
)

// DistributedCacheItem 分布式缓存项
type DistributedCacheItem struct {
	Value     interface{}
	ExpiresAt time.Time
	CreatedAt time.Time
}

// DistributedCache 分布式缓存模拟 (模拟Redis)
type DistributedCache struct {
	data      map[string]*DistributedCacheItem
	mu        sync.RWMutex
	metrics   *DistributedCacheMetrics
	latency   time.Duration // 模拟网络延迟
}

// DistributedCacheMetrics 分布式缓存指标
type DistributedCacheMetrics struct {
	Hits         int64
	Misses       int64
	Sets         int64
	Deletes      int64
	NetworkCalls int64
	TotalLatency time.Duration
	mu           sync.RWMutex
}

// NewDistributedCache 创建分布式缓存
func NewDistributedCache(networkLatency time.Duration) *DistributedCache {
	dc := &DistributedCache{
		data:    make(map[string]*DistributedCacheItem),
		metrics: &DistributedCacheMetrics{},
		latency: networkLatency,
	}
	
	// 启动清理goroutine
	go dc.cleanupExpired()
	
	return dc
}

// Get 获取缓存值
func (dc *DistributedCache) Get(key string) (interface{}, bool) {
	// 模拟网络延迟
	dc.simulateNetworkLatency()
	
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	
	dc.recordNetworkCall()
	
	item, exists := dc.data[key]
	if !exists {
		dc.recordMiss()
		return nil, false
	}
	
	// 检查是否过期
	if time.Now().After(item.ExpiresAt) {
		dc.recordMiss()
		return nil, false
	}
	
	dc.recordHit()
	return item.Value, true
}

// Set 设置缓存值
func (dc *DistributedCache) Set(key string, value interface{}, ttl time.Duration) {
	// 模拟网络延迟
	dc.simulateNetworkLatency()
	
	dc.mu.Lock()
	defer dc.mu.Unlock()
	
	dc.recordNetworkCall()
	dc.recordSet()
	
	dc.data[key] = &DistributedCacheItem{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
		CreatedAt: time.Now(),
	}
}

// Delete 删除缓存项
func (dc *DistributedCache) Delete(key string) {
	// 模拟网络延迟
	dc.simulateNetworkLatency()
	
	dc.mu.Lock()
	defer dc.mu.Unlock()
	
	dc.recordNetworkCall()
	dc.recordDelete()
	
	delete(dc.data, key)
}

// MGet 批量获取
func (dc *DistributedCache) MGet(keys []string) map[string]interface{} {
	// 模拟网络延迟 (批量操作只有一次网络开销)
	dc.simulateNetworkLatency()
	
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	
	dc.recordNetworkCall()
	
	result := make(map[string]interface{})
	now := time.Now()
	
	for _, key := range keys {
		if item, exists := dc.data[key]; exists && now.Before(item.ExpiresAt) {
			result[key] = item.Value
			dc.recordHit()
		} else {
			dc.recordMiss()
		}
	}
	
	return result
}

// MSet 批量设置
func (dc *DistributedCache) MSet(items map[string]interface{}, ttl time.Duration) {
	// 模拟网络延迟
	dc.simulateNetworkLatency()
	
	dc.mu.Lock()
	defer dc.mu.Unlock()
	
	dc.recordNetworkCall()
	
	now := time.Now()
	expiresAt := now.Add(ttl)
	
	for key, value := range items {
		dc.data[key] = &DistributedCacheItem{
			Value:     value,
			ExpiresAt: expiresAt,
			CreatedAt: now,
		}
		dc.recordSet()
	}
}

// Exists 检查键是否存在
func (dc *DistributedCache) Exists(key string) bool {
	dc.simulateNetworkLatency()
	
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	
	dc.recordNetworkCall()
	
	item, exists := dc.data[key]
	if !exists {
		return false
	}
	
	return time.Now().Before(item.ExpiresAt)
}

// TTL 获取剩余生存时间
func (dc *DistributedCache) TTL(key string) time.Duration {
	dc.simulateNetworkLatency()
	
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	
	dc.recordNetworkCall()
	
	item, exists := dc.data[key]
	if !exists {
		return -2 // 键不存在
	}
	
	remaining := time.Until(item.ExpiresAt)
	if remaining < 0 {
		return -1 // 已过期
	}
	
	return remaining
}

// Size 获取缓存大小
func (dc *DistributedCache) Size() int {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	
	return len(dc.data)
}

// Clear 清空缓存
func (dc *DistributedCache) Clear() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	
	dc.data = make(map[string]*DistributedCacheItem)
}

// GetMetrics 获取指标
func (dc *DistributedCache) GetMetrics() DistributedCacheMetrics {
	dc.metrics.mu.RLock()
	defer dc.metrics.mu.RUnlock()
	
	return *dc.metrics
}

// GetHitRate 获取命中率
func (dc *DistributedCache) GetHitRate() float64 {
	dc.metrics.mu.RLock()
	defer dc.metrics.mu.RUnlock()
	
	total := dc.metrics.Hits + dc.metrics.Misses
	if total == 0 {
		return 0
	}
	
	return float64(dc.metrics.Hits) / float64(total)
}

// GetAverageLatency 获取平均延迟
func (dc *DistributedCache) GetAverageLatency() time.Duration {
	dc.metrics.mu.RLock()
	defer dc.metrics.mu.RUnlock()
	
	if dc.metrics.NetworkCalls == 0 {
		return 0
	}
	
	return dc.metrics.TotalLatency / time.Duration(dc.metrics.NetworkCalls)
}

// simulateNetworkLatency 模拟网络延迟
func (dc *DistributedCache) simulateNetworkLatency() {
	time.Sleep(dc.latency)
	
	dc.metrics.mu.Lock()
	dc.metrics.TotalLatency += dc.latency
	dc.metrics.mu.Unlock()
}

// recordHit 记录命中
func (dc *DistributedCache) recordHit() {
	dc.metrics.mu.Lock()
	defer dc.metrics.mu.Unlock()
	
	dc.metrics.Hits++
}

// recordMiss 记录未命中
func (dc *DistributedCache) recordMiss() {
	dc.metrics.mu.Lock()
	defer dc.metrics.mu.Unlock()
	
	dc.metrics.Misses++
}

// recordSet 记录设置操作
func (dc *DistributedCache) recordSet() {
	dc.metrics.mu.Lock()
	defer dc.metrics.mu.Unlock()
	
	dc.metrics.Sets++
}

// recordDelete 记录删除操作
func (dc *DistributedCache) recordDelete() {
	dc.metrics.mu.Lock()
	defer dc.metrics.mu.Unlock()
	
	dc.metrics.Deletes++
}

// recordNetworkCall 记录网络调用
func (dc *DistributedCache) recordNetworkCall() {
	dc.metrics.mu.Lock()
	defer dc.metrics.mu.Unlock()
	
	dc.metrics.NetworkCalls++
}

// cleanupExpired 清理过期数据
func (dc *DistributedCache) cleanupExpired() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		dc.mu.Lock()
		now := time.Now()
		
		for key, item := range dc.data {
			if now.After(item.ExpiresAt) {
				delete(dc.data, key)
			}
		}
		
		dc.mu.Unlock()
	}
}

// 演示分布式缓存功能
func DemoDistributedCache() {
	fmt.Println("=== 分布式缓存演示 ===")
	
	// 创建分布式缓存 (模拟2ms网络延迟)
	cache := NewDistributedCache(2 * time.Millisecond)
	
	// 单个操作测试
	fmt.Println("\n1. 单个操作测试:")
	
	start := time.Now()
	cache.Set("user:1", "张三", 10*time.Minute)
	setLatency := time.Since(start)
	fmt.Printf("SET操作延迟: %v\n", setLatency)
	
	start = time.Now()
	if value, exists := cache.Get("user:1"); exists {
		getLatency := time.Since(start)
		fmt.Printf("GET操作延迟: %v, 值: %v\n", getLatency, value)
	}
	
	// 批量操作测试
	fmt.Println("\n2. 批量操作测试:")
	
	// 批量设置
	batchData := map[string]interface{}{
		"product:1": "iPhone 15",
		"product:2": "MacBook Pro",
		"product:3": "iPad Air",
		"product:4": "Apple Watch",
		"product:5": "AirPods Pro",
	}
	
	start = time.Now()
	cache.MSet(batchData, 15*time.Minute)
	msetLatency := time.Since(start)
	fmt.Printf("MSET操作延迟 (5个键): %v\n", msetLatency)
	
	// 批量获取
	keys := []string{"product:1", "product:2", "product:3", "product:999"}
	start = time.Now()
	results := cache.MGet(keys)
	mgetLatency := time.Since(start)
	fmt.Printf("MGET操作延迟 (4个键): %v\n", mgetLatency)
	fmt.Printf("批量获取结果: %d/%d 命中\n", len(results), len(keys))
	
	// TTL测试
	fmt.Println("\n3. TTL测试:")
	cache.Set("temp:1", "临时数据", 3*time.Second)
	
	ttl := cache.TTL("temp:1")
	fmt.Printf("temp:1 剩余TTL: %v\n", ttl)
	
	time.Sleep(1 * time.Second)
	ttl = cache.TTL("temp:1")
	fmt.Printf("1秒后 temp:1 剩余TTL: %v\n", ttl)
	
	// 性能对比
	fmt.Println("\n4. 性能对比:")
	
	// 单个操作 vs 批量操作
	fmt.Println("单个操作 vs 批量操作:")
	
	// 5次单个SET操作
	start = time.Now()
	for i := 1; i <= 5; i++ {
		key := fmt.Sprintf("single:%d", i)
		cache.Set(key, fmt.Sprintf("value%d", i), 10*time.Minute)
	}
	singleOpsTime := time.Since(start)
	
	// 1次批量SET操作
	batchData2 := make(map[string]interface{})
	for i := 1; i <= 5; i++ {
		key := fmt.Sprintf("batch:%d", i)
		batchData2[key] = fmt.Sprintf("value%d", i)
	}
	
	start = time.Now()
	cache.MSet(batchData2, 10*time.Minute)
	batchOpTime := time.Since(start)
	
	fmt.Printf("5次单个SET: %v\n", singleOpsTime)
	fmt.Printf("1次批量SET: %v\n", batchOpTime)
	fmt.Printf("批量操作性能提升: %.1fx\n", float64(singleOpsTime)/float64(batchOpTime))
	
	// 显示指标
	fmt.Println("\n5. 缓存指标:")
	metrics := cache.GetMetrics()
	fmt.Printf("网络调用次数: %d\n", metrics.NetworkCalls)
	fmt.Printf("命中次数: %d\n", metrics.Hits)
	fmt.Printf("未命中次数: %d\n", metrics.Misses)
	fmt.Printf("命中率: %.2f%%\n", cache.GetHitRate()*100)
	fmt.Printf("平均延迟: %v\n", cache.GetAverageLatency())
	fmt.Printf("SET操作次数: %d\n", metrics.Sets)
	fmt.Printf("DELETE操作次数: %d\n", metrics.Deletes)
	
	fmt.Println("\n💡 分布式缓存特点:")
	fmt.Println("   1. 网络延迟 (毫秒级)")
	fmt.Println("   2. 大容量存储")
	fmt.Println("   3. 数据持久化")
	fmt.Println("   4. 支持集群部署")
	fmt.Println("   5. 批量操作优化网络开销")
}
