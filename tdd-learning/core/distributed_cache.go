// distributed_cache.go - 分布式缓存实现
// 这是面试中最重要的分布式解决方案展示

package core

import (
	"crypto/sha1"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"
)

// 这些包将在实现时使用
// "crypto/sha1"
// "fmt"
// "sort"
// "strconv"

// DistributedCache 分布式缓存结构
type DistributedCache struct {
	// 一致性哈希环相关
	HashRing     map[uint32]string    // 哈希环：哈希值 -> 真实节点
	Nodes        []string             // 真实节点列表
	VirtualNodes int                  // 每个节点的虚拟节点数量
	SortedHashes []uint32             // 排序后的哈希值列表，用于快速查找
	LocalCaches  map[string]*LRUCache // 每个节点的本地缓存

	// 基础数据迁移相关
	BasicMigrationStats BasicMigrationStats     // 基础迁移统计信息

	Mu sync.RWMutex
}

// NewDistributedCache 创建分布式缓存
// 参数：nodes - 节点地址列表，如 ["node1:8001", "node2:8002"]
func NewDistributedCache(nodes []string) *DistributedCache {
	// 1. 初始化结构体字段
	dc := &DistributedCache{
		Nodes: nodes,
		VirtualNodes: 150,  // 经验值，预设
		LocalCaches: make(map[string]*LRUCache),
	}
	dc.buildHashRing()
	// 2. 为每个节点创建本地缓存实例
	for _, node := range nodes {
		dc.LocalCaches[node] = NewLRUCache(1000)
	}
	// 调用 buildHashRing() 构建哈希环
	// 默认虚拟节点数量可以设为 150（经验值）
	
	return dc // 请替换为实际实现
}

// NewDistributedCacheWithVirtualNodes 创建带指定虚拟节点数的分布式缓存
func NewDistributedCacheWithVirtualNodes(nodes []string, virtualNodeCount int) *DistributedCache {
	// TODO: 实现带虚拟节点数的构造函数
	// 提示：类似 NewDistributedCache，但允许自定义虚拟节点数量
	dc := &DistributedCache{
		Nodes: nodes,
		VirtualNodes: virtualNodeCount,
		LocalCaches: make(map[string]*LRUCache),
	}
	dc.buildHashRing()
	// 2. 为每个节点创建本地缓存实例
	for _, node := range nodes {
		dc.LocalCaches[node] = NewLRUCache(1000)
	}
	
	return dc
}

// GetNodeForKey 根据键获取对应的节点
// 这是一致性哈希的核心算法
func (dc *DistributedCache) GetNodeForKey(key string) string {
	// 加读锁保护哈希环的并发访问
	dc.Mu.RLock()
	defer dc.Mu.RUnlock()

	return dc.getNodeForKeyUnsafe(key)
}

// getNodeForKeyUnsafe 不加锁的内部版本，用于已持有锁的上下文
func (dc *DistributedCache) getNodeForKeyUnsafe(key string) string {
	// 1. 计算键的哈希值
	hash := dc.hashKey(key)
	// 2. 在排序的哈希环中找到第一个大于等于该哈希值的虚拟节点
	// 3. 使用二分查找提高效率
	idx := sort.Search(len(dc.SortedHashes), func(i int) bool {
		return dc.SortedHashes[i] >= hash
	})
	// 4. 如果没找到，则选择环上的第一个节点（环形特性）
	if idx == len(dc.SortedHashes) {
		idx = 0
	}
	// 5. 返回虚拟节点对应的真实节点地址
	nodeHash := dc.SortedHashes[idx]
	node := dc.HashRing[nodeHash]

	return node
}

// AddNode 添加新节点到集群
func (dc *DistributedCache) AddNode(node string) error {
	// 1. 加锁保证操作原子性
	dc.Mu.Lock()
	defer dc.Mu.Unlock()

	// 2. 记录迁移开始时间
	startTime := time.Now()

	// 3. 记录添加节点前的哈希环状态（用于确定需要迁移的数据范围）
	oldSortedHashes := make([]uint32, len(dc.SortedHashes))
	copy(oldSortedHashes, dc.SortedHashes)

	// 4. 添加新节点到集群
	dc.Nodes = append(dc.Nodes, node)
	dc.LocalCaches[node] = NewLRUCache(1000)

	// 5. 将新节点添加到哈希环（增量操作）
	dc.addNodeToHashRing(node)

	// 6. 执行一致性哈希数据迁移
	migratedCount := dc.migrateDataForAddedNode(node, oldSortedHashes)

	// 7. 更新统计信息
	dc.updateBasicMigrationStats(migratedCount, time.Since(startTime))

	return nil
}

// RemoveNode 从集群中移除节点
func (dc *DistributedCache) RemoveNode(node string) error {
	// 1. 加锁保证操作原子性
	dc.Mu.Lock()
	defer dc.Mu.Unlock()

	// 2. 记录迁移开始时间
	startTime := time.Now()

	// 3. 获取被移除节点的所有数据
	nodeCache, exists := dc.LocalCaches[node]
	if !exists {
		return fmt.Errorf("节点不存在: %s", node)
	}
	nodeData := dc.getAllDataFromCache(nodeCache)

	// 4. 从集群中移除节点
	dc.removeNodeFromNodes(node)
	delete(dc.LocalCaches, node)

	// 5. 从哈希环中移除该节点（增量操作）
	dc.removeNodeFromHashRing(node)

	// 6. 将被移除节点的数据按一致性哈希重新分布
	migratedCount := dc.redistributeDataBasic(nodeData)

	// 7. 更新统计信息
	dc.updateBasicMigrationStats(migratedCount, time.Since(startTime))

	return nil
}

// buildHashRing 构建哈希环 - 初始化时使用
func (dc *DistributedCache) buildHashRing() {
	// 初始化时才清空哈希环
	dc.HashRing = make(map[uint32]string)
	dc.SortedHashes = make([]uint32, 0)

	// 为所有节点创建虚拟节点
	for _, node := range dc.Nodes {
		dc.addNodeToHashRing(node)
	}
}

// addNodeToHashRing 向哈希环中添加单个节点 - 扩容时使用
func (dc *DistributedCache) addNodeToHashRing(node string) {
	// 为新节点创建虚拟节点并添加到哈希环
	for i := 0; i < dc.VirtualNodes; i++ {
		virtualNode := node + "#" + strconv.Itoa(i)
		hash := dc.hashKey(virtualNode)
		dc.HashRing[hash] = node
		dc.SortedHashes = append(dc.SortedHashes, hash)
	}

	// 重新排序哈希值列表
	slices.Sort(dc.SortedHashes)
}

// removeNodeFromHashRing 从哈希环中移除单个节点 - 缩容时使用
func (dc *DistributedCache) removeNodeFromHashRing(node string) {
	// 找到并移除该节点的所有虚拟节点
	nodesToRemove := make([]uint32, 0)

	for hash, n := range dc.HashRing {
		if n == node {
			nodesToRemove = append(nodesToRemove, hash)
		}
	}

	// 从哈希环中删除
	for _, hash := range nodesToRemove {
		delete(dc.HashRing, hash)
	}

	// 从排序列表中移除
	newSortedHashes := make([]uint32, 0, len(dc.SortedHashes)-len(nodesToRemove))
	for _, hash := range dc.SortedHashes {
		if dc.HashRing[hash] != "" { // 如果哈希值还在环中
			newSortedHashes = append(newSortedHashes, hash)
		}
	}
	dc.SortedHashes = newSortedHashes
}

// hashKey 计算键的哈希值
func (dc *DistributedCache) hashKey(key string) uint32 {
	// TODO: 实现哈希函数
	// 提示：
	// 1. 使用 SHA1 算法：sha1.Sum([]byte(key))
	hashBytes := sha1.Sum([]byte(key))
	// 2. 取前4个字节转换为 uint32
	// 确保哈希分布均匀
	hash := uint32(hashBytes[0])<<24 + 
		   uint32(hashBytes[1])<<16 + 
		   uint32(hashBytes[2])<<8 + 
		   uint32(hashBytes[3])
	return hash
}

// Set 在分布式缓存中设置键值对
func (dc *DistributedCache) Set(key, value string) error {
	// 1. 根据键找到对应的节点（GetNodeForKey内部已加锁）
	node := dc.GetNodeForKey(key)

	// 2. 加读锁保护LocalCaches的并发访问
	dc.Mu.RLock()
	cache, exists := dc.LocalCaches[node]
	dc.Mu.RUnlock()

	if !exists {
		return fmt.Errorf("节点 %s 不存在", node)
	}
	// 3. 在本地缓存中设置键值对（LRUCache内部有锁保护）
	cache.Set(key, value)
	// 4. 返回 nil 表示成功
	return nil
}

// Get 从分布式缓存中获取值
func (dc *DistributedCache) Get(key string) (string, bool, error) {
	// 1. 根据键找到对应的节点（GetNodeForKey内部已加锁）
	node := dc.GetNodeForKey(key)

	// 2. 加读锁保护LocalCaches的并发访问
	dc.Mu.RLock()
	cache, exists := dc.LocalCaches[node]
	dc.Mu.RUnlock()

	if !exists {
		return "", false, fmt.Errorf("节点 %s 不存在", node)
	}
	// 3. 从本地缓存中获取值（LRUCache内部有锁保护）
	value, found := cache.Get(key)
	// 4. 返回 (值, 是否存在, 错误)
	return value, found, nil
}

// Delete 从分布式缓存中删除键
func (dc *DistributedCache) Delete(key string) error {
	// 1. 根据键找到对应的节点（GetNodeForKey内部已加锁）
	node := dc.GetNodeForKey(key)

	// 2. 加读锁保护LocalCaches的并发访问
	dc.Mu.RLock()
	cache, exists := dc.LocalCaches[node]
	dc.Mu.RUnlock()

	if !exists {
		return fmt.Errorf("节点 %s 不存在", node)
	}
	// 3. 从本地缓存中删除键（LRUCache内部有锁保护）
	cache.Delete(key)
	// 4. 返回 nil 表示成功
	return nil
}

// GetStats 获取集群统计信息
func (dc *DistributedCache) GetStats() map[string]interface{} {
	// 加读锁保护LocalCaches的并发访问
	dc.Mu.RLock()
	defer dc.Mu.RUnlock()

	// 汇总统计信息：总命中数、总未命中数、总大小等
	totalHits, totalMisses, totalSize := 0, 0, 0
	// 1. 遍历所有节点的本地缓存
	for _, cache := range dc.LocalCaches {
		// 2. 汇总统计信息（LRUCache内部有锁保护）
		totalHits += int(cache.GetStats().Hits)
		totalMisses += int(cache.GetStats().Misses)
		totalSize += cache.Size()
	}

	// 返回包含集群级别统计的 map
	return map[string]any{
		"total_Hits":   totalHits,
		"total_Misses": totalMisses,
		"total_Size":   totalSize,
	}
}

// BasicMigrationStats 基础迁移统计
type BasicMigrationStats struct {
	MigratedKeys  int           // 迁移的键数量
	Duration      time.Duration // 迁移耗时
	LastMigration time.Time     // 最后一次迁移时间
}

// GetMigrationStats 获取数据迁移统计
func (dc *DistributedCache) GetMigrationStats() BasicMigrationStats {
	dc.Mu.RLock()
	defer dc.Mu.RUnlock()

	return dc.BasicMigrationStats
}

// ===== 基础数据迁移辅助方法 =====

// migrateDataForAddedNode 为新添加的节点执行数据迁移
// 根据一致性哈希原理：新节点只需要接管部分数据，这些数据原本存储在其他节点
func (dc *DistributedCache) migrateDataForAddedNode(newNode string, oldSortedHashes []uint32) int {
	migratedCount := 0

	// 获取新节点在哈希环上的所有虚拟节点位置
	newNodeHashes := make([]uint32, 0)
	for hash, node := range dc.HashRing {
		if node == newNode {
			newNodeHashes = append(newNodeHashes, hash)
		}
	}

	// 对于每个新的虚拟节点，找出需要从其他节点迁移过来的数据
	for _, newHash := range newNodeHashes {
		// 在旧的哈希环中，这个位置的数据应该存储在哪个节点？
		oldResponsibleNode := dc.findResponsibleNodeInOldRing(newHash, oldSortedHashes)
		if oldResponsibleNode == "" || oldResponsibleNode == newNode {
			continue // 没有需要迁移的数据
		}

		// 从旧的负责节点中找出需要迁移的数据
		oldCache := dc.LocalCaches[oldResponsibleNode]
		if oldCache == nil {
			continue
		}

		// 获取该节点的所有数据，检查哪些应该迁移到新节点
		allData := oldCache.GetAllData()
		for key, value := range allData {
			// 重新计算这个key现在应该存储在哪个节点（使用不加锁版本，因为已持有写锁）
			currentResponsibleNode := dc.getNodeForKeyUnsafe(key)
			if currentResponsibleNode == newNode {
				// 这个key现在应该存储在新节点，需要迁移
				dc.LocalCaches[newNode].Set(key, value)
				oldCache.Delete(key)
				migratedCount++
			}
		}
	}

	return migratedCount
}

// findResponsibleNodeInOldRing 在旧的哈希环中找到负责指定哈希值的节点
func (dc *DistributedCache) findResponsibleNodeInOldRing(hash uint32, oldSortedHashes []uint32) string {
	if len(oldSortedHashes) == 0 {
		return ""
	}

	// 在旧的排序哈希列表中找到第一个大于等于hash的位置
	idx := sort.Search(len(oldSortedHashes), func(i int) bool {
		return oldSortedHashes[i] >= hash
	})

	// 如果没找到，则使用环的第一个节点（环形特性）
	if idx == len(oldSortedHashes) {
		idx = 0
	}

	// 通过哈希值找到对应的节点（需要在旧的映射中查找）
	// 这里简化处理：遍历当前HashRing找到对应节点
	targetHash := oldSortedHashes[idx]
	for ringHash, node := range dc.HashRing {
		if ringHash == targetHash {
			return node
		}
	}

	return ""
}

// getAllDataFromCache 获取指定缓存的所有数据
func (dc *DistributedCache) getAllDataFromCache(cache *LRUCache) map[string]string {
	return cache.GetAllData()
}

// redistributeDataBasic 重新分布数据到集群
func (dc *DistributedCache) redistributeDataBasic(data map[string]string) int {
	redistributedCount := 0
	for key, value := range data {
		// 根据当前哈希环计算目标节点（使用不加锁版本，因为已持有写锁）
		targetNode := dc.getNodeForKeyUnsafe(key)
		if targetCache := dc.LocalCaches[targetNode]; targetCache != nil {
			targetCache.Set(key, value)
			redistributedCount++
		}
	}
	return redistributedCount
}

// removeNodeFromNodes 从节点列表中移除指定节点
func (dc *DistributedCache) removeNodeFromNodes(node string) {
	for i, n := range dc.Nodes {
		if n == node {
			dc.Nodes = append(dc.Nodes[:i], dc.Nodes[i+1:]...)
			break
		}
	}
}

// updateBasicMigrationStats 更新基础迁移统计信息
func (dc *DistributedCache) updateBasicMigrationStats(migratedCount int, duration time.Duration) {
	dc.BasicMigrationStats.MigratedKeys += migratedCount
	dc.BasicMigrationStats.Duration = duration
	dc.BasicMigrationStats.LastMigration = time.Now()
}
