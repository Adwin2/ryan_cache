// LRU + TTL + 统计 + 内存限制

package main

import (
	"sync"
	"time"
)

// 双向链表节点
type LRUNode struct {
	key string
	value string
	prev *LRUNode
	next *LRUNode
}

// LRU缓存结构
type LRUCache struct {
	capacity int  
	size int
	cache map[string]*LRUNode // 哈希表 ： key -> 节点
	head *LRUNode // 虚拟头节点 （最近使用）
	tail *LRUNode // 虚拟尾节点 （最久未使用）
	mu sync.RWMutex

	// TTL 
	ttlMap map[string]time.Time

	// 异步清理
	cleanupInterval time.Duration
	stopCleanup chan struct{}
	cleanupStats CleanupStats
	// 统计
	stats CacheStats

	// 内存限制
	memoryUsage int64 // 当前内存使用量
	memoryLimit int64 // 内存限制（0表示无限制）
}

type CleanupStats struct {
	CleanedKeys int64
	CleanupRuns int64
	LastCleanup time.Time
}

// 统计结构
type CacheStats struct {
	Hits int64
	Misses int64
	TotalRequests int64
}

// 2. API响应结构（面向客户端）
type StatsAPIResponse struct {
    Hits         int64   `json:"hits"`
    Misses       int64   `json:"misses"`
    TotalRequests int64  `json:"total_requests"`
    HitRate      float64 `json:"hit_rate"`
    CacheSize    int     `json:"cache_size"`
    MemoryUsage  int64   `json:"memory_usage"`
    Uptime       string  `json:"uptime,omitempty"`
}

// 3. 转换函数
func (s *CacheServer) buildStatsResponse() *StatsAPIResponse {
    stats := s.cache.GetStats()
    return &StatsAPIResponse{
        Hits:         stats.Hits,
        Misses:       stats.Misses,
        TotalRequests: stats.TotalRequests,
        HitRate:      stats.HitRate(),
        CacheSize:    s.cache.Size(),
        MemoryUsage:  s.cache.GetMemoryUsage(),
    }
}

func NewLRUCache(capacity int) *LRUCache {
	if capacity <= 0 {
		panic("容量必须大于0")
	}

	lru := &LRUCache{
		capacity: capacity,
		cache: make(map[string]*LRUNode),
		head: &LRUNode{},
		tail: &LRUNode{},
		// 内存限制
		memoryLimit: 0,
	}

	// 初始化头尾节点的连接
	lru.head.next = lru.tail
	lru.tail.prev = lru.head

	return lru
}

// 带内存限制的构造函数
func NewLRUCacheWithMemoryLimit(capacity int, memoryLimitBytes int64) *LRUCache {
	lru := NewLRUCache(capacity)
	lru.memoryLimit = memoryLimitBytes
	return lru
}

// 带TTL的构造函数
func NewLRUCacheWithCleanup(capacity int, cleanupInterval time.Duration) *LRUCache {
	c := NewLRUCache(capacity)
	c.cleanupInterval = cleanupInterval
	c.stopCleanup = make(chan struct{})
	c.ttlMap = make(map[string]time.Time)
	go c.startCleanupRoutine()
	return c
}

// 后台清理例程
func (lru *LRUCache) startCleanupRoutine() {
	ticker := time.NewTicker(lru.cleanupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			lru.cleanupExpiredkeys()
		case <-lru.stopCleanup:
			return
		}
	}
}

// 清理过期键
func(lru *LRUCache) cleanupExpiredkeys() {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	now := time.Now()
	cleanedCount := 0

	for key, expireTime := range lru.ttlMap {
		if now.After(expireTime) {
			if node, exists := lru.cache[key]; exists {
				// 更新内存使用量
				lru.memoryUsage -= calculateMemoryUsage(key, node.value)
				lru.removeNode(node)
				delete(lru.cache, key)
				lru.size--
			}
			delete(lru.ttlMap, key)
			cleanedCount++
		}
	}

	// 更新统计
	lru.cleanupStats.CleanedKeys += int64(cleanedCount)
	lru.cleanupStats.CleanupRuns++
	lru.cleanupStats.LastCleanup = now
}

func (lru *LRUCache) Close() {
	close(lru.stopCleanup)
}

func (lru *LRUCache) GetCleanupStats() CleanupStats {
	return lru.cleanupStats
}


// 内存计算辅助函数
func calculateMemoryUsage(key, value string) int64 {
	// 节点结构开销：key + value + 指针 = 24 + 24 + 16 = 64字节
	return int64(len(key) + len(value) + 64)
}

// 将节点添加到头部（最近使用）
func (lru *LRUCache) addToHead(node *LRUNode) {
	node.prev = lru.head
	node.next = lru.head.next
	lru.head.next.prev = node
	lru.head.next = node
}

// 从链表中删除节点
func (lru *LRUCache) removeNode (node *LRUNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

// 将节点移动到头部
func (lru *LRUCache) moveToHead(node *LRUNode) {
	lru.removeNode(node)
	lru.addToHead(node)
}

// 从链表中删除尾部节点 返回被删除的节点
func (lru *LRUCache) removeTail() *LRUNode {
	lastNode := lru.tail.prev
	lru.removeNode(lastNode)
	return lastNode
}

func (lru *LRUCache) SetWithTTL(key, value string, ttl time.Duration) {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	lru.SetInternal(key, value)
	lru.ttlMap[key] = time.Now().Add(ttl)
}

// 添加内存限制检查的Set方法
func (lru *LRUCache) Set(key, value string) {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	lru.SetInternal(key, value)
}

func (lru *LRUCache) SetInternal(key, value string) {
	newMemory := calculateMemoryUsage(key, value)
	if lru.memoryLimit > 0 && newMemory > lru.memoryLimit {
		return
	}

	if node, exists := lru.cache[key]; exists {
		// 更新内存使用量
		lru.memoryUsage = lru.memoryUsage - calculateMemoryUsage(key, node.value) + newMemory
		
		node.value = value
		lru.moveToHead(node)
	} else {
		// 检查内存限制和容量限制
		for (lru.memoryLimit > 0 && lru.memoryUsage + newMemory > lru.memoryLimit && 
			lru.size > 0 ) || lru.size >= lru.capacity {
			if lru.size == 0 {
				break
			}
			// 从链表删除
			lastNode := lru.removeTail()
			
			oldMemory := calculateMemoryUsage(lastNode.key, lastNode.value)
			lru.memoryUsage -= oldMemory
			// 从哈希表删除
			delete(lru.cache, lastNode.key)
			lru.size --
		}
		newNode := &LRUNode{key: key, value: value}
		lru.addToHead(newNode)
		lru.cache[key] = newNode
		lru.memoryUsage += newMemory
		lru.size ++
	}
}

// 添加统计的Get方法
func (lru *LRUCache) Get(key string) (string, bool) {
	// Get会更新访问顺序
	lru.mu.Lock()
	defer lru.mu.Unlock()

	// 总请求数
	lru.stats.TotalRequests ++

	node, exists := lru.cache[key]
	if !exists {
		lru.stats.Misses ++
		return "", false
	}
	// 检查是否过期
	if expireTime, hasTTL := lru.ttlMap[key]; hasTTL {
		if time.Now().After(expireTime) {
			// 过期了，删除并返回未找到
            lru.removeNode(node)
            delete(lru.cache, key)
            delete(lru.ttlMap, key)
            lru.size--
            lru.stats.Misses++
            return "", false
		}
	}
	// 命中
	lru.stats.Hits ++
	lru.moveToHead(node)
	return node.value, true
}

// 传入key 返回是否成功删除
func (lru *LRUCache) Delete(key string) bool {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	if targetNode, exists := lru.cache[key]; exists {
		// 更新内存使用量
		lru.memoryUsage -= calculateMemoryUsage(key, targetNode.value)
		// 从链表中删除
		lru.removeNode(targetNode)
		// 从哈希表中删除
		delete(lru.cache, key)
		// 清理TTL映射
		delete(lru.ttlMap, key)
		lru.size --
		return true
	}
	return false
}

func (lru *LRUCache) Size() int {
	return lru.size
}

func (lru *LRUCache) GetStats() CacheStats {
	lru.mu.RLock()
	defer lru.mu.RUnlock()
	return lru.stats
}

func (lru *LRUCache) GetMemoryUsage() int64 {
	lru.mu.RLock()
	defer lru.mu.RUnlock()
	return lru.memoryUsage
}

// 统计相关
func (s *CacheStats) HitRate() float64 {
	if s.TotalRequests == 0 {
		return 0
	}
	return float64(s.Hits) / float64(s.TotalRequests)
}

// 批量操作
func (lru *LRUCache) SetMulti(data map[string]string) {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	
	for key, value := range data {
		lru.SetInternal(key, value)
	}
}

func (lru *LRUCache) GetMulti(keys []string) map[string]string {
	lru.mu.Lock()
	defer lru.mu.Unlock()
	
	results := make(map[string]string)
	
	for _, key := range keys {
		// 统计
		lru.stats.TotalRequests ++

		if node, exists := lru.cache[key]; exists {
			lru.stats.Hits ++
			lru.moveToHead(node)
			results[key] = node.value
		} else {
			lru.stats.Misses ++
		}
	}
	return results
}

func (lru *LRUCache) DeleteMulti(keys []string) int {
	lru.mu.Lock()
	defer lru.mu.Unlock()

	deletedCount := 0

	for _, key := range keys {
		if node, exists := lru.cache[key]; exists {
			lru.removeNode(node)
			delete(lru.cache, key)

			// 更新内存使用量
			lru.memoryUsage -= calculateMemoryUsage(key, node.value)

			// 清理TTL映射
			delete(lru.ttlMap, key)

			lru.size --
			deletedCount ++
		}
	}
	return deletedCount
}

// GetAllData 获取缓存中的所有数据 - 用于数据迁移
func (lru *LRUCache) GetAllData() map[string]string {
	lru.mu.RLock()
	defer lru.mu.RUnlock()

	result := make(map[string]string)

	// 遍历哈希表获取所有键值对
	for key, node := range lru.cache {
		// 检查是否过期
		if expireTime, hasTTL := lru.ttlMap[key]; hasTTL {
			if time.Now().After(expireTime) {
				// 过期了，跳过
				continue
			}
		}
		result[key] = node.value
	}

	return result
}