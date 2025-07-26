package main

import (
	"fmt"
	"sort"
	"sync"
)

// VirtualNode 虚拟节点
type VirtualNode struct {
	Hash         uint32
	PhysicalNode *Node
	VirtualID    string // 虚拟节点ID，如 "node1#1", "node1#2"
}

// VirtualHashRing 带虚拟节点的哈希环
type VirtualHashRing struct {
	virtualNodes   map[uint32]*VirtualNode // 哈希值 -> 虚拟节点
	sortedHashes   []uint32                // 排序的哈希值
	physicalNodes  map[string]*Node        // 物理节点映射
	virtualCount   int                     // 每个物理节点的虚拟节点数
	mu            sync.RWMutex
}

// NewVirtualHashRing 创建带虚拟节点的哈希环
func NewVirtualHashRing(virtualCount int) *VirtualHashRing {
	return &VirtualHashRing{
		virtualNodes:  make(map[uint32]*VirtualNode),
		sortedHashes:  make([]uint32, 0),
		physicalNodes: make(map[string]*Node),
		virtualCount:  virtualCount,
	}
}

// hash 计算哈希值（与基础版本相同）
func (vhr *VirtualHashRing) hash(key string) uint32 {
	// 使用简单的哈希函数（生产环境建议使用MD5或SHA1）
	var hash uint32
	for _, c := range key {
		hash = hash*31 + uint32(c)
	}
	return hash
}

// AddNode 添加物理节点（会创建多个虚拟节点）
func (vhr *VirtualHashRing) AddNode(node *Node) {
	vhr.mu.Lock()
	defer vhr.mu.Unlock()
	
	// 检查物理节点是否已存在
	if _, exists := vhr.physicalNodes[node.ID]; exists {
		fmt.Printf("⚠️ 物理节点 %s 已存在\n", node.ID)
		return
	}
	
	// 添加物理节点
	vhr.physicalNodes[node.ID] = node
	
	// 根据权重计算虚拟节点数量
	virtualNodeCount := vhr.virtualCount
	if node.Weight > 0 {
		virtualNodeCount = vhr.virtualCount * node.Weight / 100
	}
	
	fmt.Printf("✅ 添加物理节点: %s (权重: %d, 虚拟节点数: %d)\n", 
		node.ID, node.Weight, virtualNodeCount)
	
	// 创建虚拟节点
	for i := 0; i < virtualNodeCount; i++ {
		virtualID := fmt.Sprintf("%s#%d", node.ID, i)
		virtualHash := vhr.hash(virtualID)
		
		// 处理哈希冲突
		for _, exists := vhr.virtualNodes[virtualHash]; exists; {
			virtualID = fmt.Sprintf("%s#%d#%d", node.ID, i, virtualHash%1000)
			virtualHash = vhr.hash(virtualID)
			_, exists = vhr.virtualNodes[virtualHash]
		}
		
		virtualNode := &VirtualNode{
			Hash:         virtualHash,
			PhysicalNode: node,
			VirtualID:    virtualID,
		}
		
		vhr.virtualNodes[virtualHash] = virtualNode
		vhr.sortedHashes = append(vhr.sortedHashes, virtualHash)
		
		fmt.Printf("  📍 虚拟节点: %s (哈希: %d)\n", virtualID, virtualHash)
	}
	
	// 重新排序
	sort.Slice(vhr.sortedHashes, func(i, j int) bool {
		return vhr.sortedHashes[i] < vhr.sortedHashes[j]
	})
}

// RemoveNode 移除物理节点（会移除所有相关虚拟节点）
func (vhr *VirtualHashRing) RemoveNode(nodeID string) {
	vhr.mu.Lock()
	defer vhr.mu.Unlock()
	
	// 检查物理节点是否存在
	if _, exists := vhr.physicalNodes[nodeID]; !exists {
		fmt.Printf("⚠️ 物理节点 %s 不存在\n", nodeID)
		return
	}
	
	// 移除所有相关的虚拟节点
	removedCount := 0
	newSortedHashes := make([]uint32, 0)
	
	for hash, virtualNode := range vhr.virtualNodes {
		if virtualNode.PhysicalNode.ID == nodeID {
			delete(vhr.virtualNodes, hash)
			removedCount++
		} else {
			newSortedHashes = append(newSortedHashes, hash)
		}
	}
	
	vhr.sortedHashes = newSortedHashes
	sort.Slice(vhr.sortedHashes, func(i, j int) bool {
		return vhr.sortedHashes[i] < vhr.sortedHashes[j]
	})
	
	// 移除物理节点
	delete(vhr.physicalNodes, nodeID)
	
	fmt.Printf("❌ 移除物理节点: %s (移除了 %d 个虚拟节点)\n", nodeID, removedCount)
}

// GetNode 根据key获取对应的物理节点
func (vhr *VirtualHashRing) GetNode(key string) *Node {
	vhr.mu.RLock()
	defer vhr.mu.RUnlock()
	
	if len(vhr.sortedHashes) == 0 {
		return nil
	}
	
	keyHash := vhr.hash(key)
	
	// 二分查找第一个大于等于keyHash的虚拟节点
	idx := sort.Search(len(vhr.sortedHashes), func(i int) bool {
		return vhr.sortedHashes[i] >= keyHash
	})
	
	// 环形处理
	if idx == len(vhr.sortedHashes) {
		idx = 0
	}
	
	virtualHash := vhr.sortedHashes[idx]
	virtualNode := vhr.virtualNodes[virtualHash]
	
	fmt.Printf("🔍 Key: %s (哈希: %d) → 虚拟节点: %s → 物理节点: %s\n", 
		key, keyHash, virtualNode.VirtualID, virtualNode.PhysicalNode.ID)
	
	return virtualNode.PhysicalNode
}

// GetNodes 获取多个副本的物理节点
func (vhr *VirtualHashRing) GetNodes(key string, count int) []*Node {
	vhr.mu.RLock()
	defer vhr.mu.RUnlock()
	
	if len(vhr.sortedHashes) == 0 || count <= 0 {
		return nil
	}
	
	keyHash := vhr.hash(key)
	nodes := make([]*Node, 0, count)
	visited := make(map[string]bool)
	
	// 找到起始位置
	startIdx := sort.Search(len(vhr.sortedHashes), func(i int) bool {
		return vhr.sortedHashes[i] >= keyHash
	})
	
	// 环形遍历，获取count个不同的物理节点
	for i := 0; i < len(vhr.sortedHashes) && len(nodes) < count; i++ {
		idx := (startIdx + i) % len(vhr.sortedHashes)
		virtualHash := vhr.sortedHashes[idx]
		virtualNode := vhr.virtualNodes[virtualHash]
		physicalNode := virtualNode.PhysicalNode
		
		// 避免重复的物理节点
		if !visited[physicalNode.ID] {
			nodes = append(nodes, physicalNode)
			visited[physicalNode.ID] = true
		}
	}
	
	return nodes
}

// PrintVirtualRing 打印虚拟哈希环状态
func (vhr *VirtualHashRing) PrintVirtualRing() {
	vhr.mu.RLock()
	defer vhr.mu.RUnlock()
	
	fmt.Println("\n📊 虚拟哈希环状态:")
	fmt.Printf("物理节点数: %d\n", len(vhr.physicalNodes))
	fmt.Printf("虚拟节点数: %d\n", len(vhr.virtualNodes))
	fmt.Printf("每个物理节点的虚拟节点数: %d\n", vhr.virtualCount)
	
	if len(vhr.physicalNodes) == 0 {
		fmt.Println("哈希环为空")
		return
	}
	
	// 按物理节点分组显示虚拟节点
	fmt.Println("\n物理节点及其虚拟节点:")
	for nodeID, physicalNode := range vhr.physicalNodes {
		fmt.Printf("  📦 %s (权重: %d):\n", nodeID, physicalNode.Weight)
		
		virtualCount := 0
		for _, virtualNode := range vhr.virtualNodes {
			if virtualNode.PhysicalNode.ID == nodeID {
				virtualCount++
			}
		}
		fmt.Printf("    虚拟节点数量: %d\n", virtualCount)
	}
}

// CalculateLoadBalance 计算负载均衡情况
func (vhr *VirtualHashRing) CalculateLoadBalance(keys []string) {
	distribution := make(map[string]int)
	
	for _, key := range keys {
		node := vhr.GetNode(key)
		if node != nil {
			distribution[node.ID]++
		}
	}
	
	fmt.Println("\n📈 负载均衡分析:")
	totalKeys := len(keys)
	expectedPerNode := float64(totalKeys) / float64(len(vhr.physicalNodes))
	
	for nodeID, count := range distribution {
		percentage := float64(count) / float64(totalKeys) * 100
		deviation := (float64(count) - expectedPerNode) / expectedPerNode * 100
		
		fmt.Printf("  %s: %d keys (%.1f%%, 偏差: %+.1f%%)\n", 
			nodeID, count, percentage, deviation)
	}
	
	// 计算标准差
	var variance float64
	for _, count := range distribution {
		diff := float64(count) - expectedPerNode
		variance += diff * diff
	}
	variance /= float64(len(distribution))
	stdDev := variance
	
	fmt.Printf("负载标准差: %.2f (越小越均匀)\n", stdDev)
}

// DemoVirtualNodes 演示虚拟节点功能
func DemoVirtualNodes() {
	fmt.Println("=== 虚拟节点演示 ===")
	
	// 创建虚拟哈希环，每个物理节点150个虚拟节点
	ring := NewVirtualHashRing(150)
	
	// 添加物理节点
	fmt.Println("\n1. 添加物理节点:")
	nodes := []*Node{
		{ID: "node1", Address: "192.168.1.1:6379", Weight: 100},
		{ID: "node2", Address: "192.168.1.2:6379", Weight: 100},
		{ID: "node3", Address: "192.168.1.3:6379", Weight: 100},
	}
	
	for _, node := range nodes {
		ring.AddNode(node)
	}
	
	ring.PrintVirtualRing()
	
	// 生成测试数据
	fmt.Println("\n2. 测试负载均衡:")
	testKeys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		testKeys[i] = fmt.Sprintf("key_%d", i)
	}
	
	ring.CalculateLoadBalance(testKeys)
}

// DemoWeightedNodes 演示加权节点
func DemoWeightedNodes() {
	fmt.Println("\n=== 加权节点演示 ===")
	
	ring := NewVirtualHashRing(100)
	
	// 添加不同权重的节点
	fmt.Println("\n1. 添加不同权重的节点:")
	nodes := []*Node{
		{ID: "high_perf", Address: "192.168.1.1:6379", Weight: 200}, // 高性能节点
		{ID: "medium_perf", Address: "192.168.1.2:6379", Weight: 100}, // 中等性能节点
		{ID: "low_perf", Address: "192.168.1.3:6379", Weight: 50},  // 低性能节点
	}
	
	for _, node := range nodes {
		ring.AddNode(node)
	}
	
	ring.PrintVirtualRing()
	
	// 测试加权分布
	fmt.Println("\n2. 测试加权分布:")
	testKeys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		testKeys[i] = fmt.Sprintf("weighted_key_%d", i)
	}
	
	ring.CalculateLoadBalance(testKeys)
	
	fmt.Println("\n💡 加权节点效果:")
	fmt.Println("   高性能节点承担更多负载")
	fmt.Println("   低性能节点承担较少负载")
	fmt.Println("   实现按性能分配的负载均衡")
}

// DemoDataMigration 演示数据迁移
func DemoDataMigration() {
	fmt.Println("\n=== 数据迁移演示 ===")
	
	ring := NewVirtualHashRing(50) // 较少虚拟节点便于观察
	
	// 初始3个节点
	fmt.Println("\n1. 初始状态 (3个节点):")
	initialNodes := []*Node{
		{ID: "node1", Address: "192.168.1.1:6379", Weight: 100},
		{ID: "node2", Address: "192.168.1.2:6379", Weight: 100},
		{ID: "node3", Address: "192.168.1.3:6379", Weight: 100},
	}
	
	for _, node := range initialNodes {
		ring.AddNode(node)
	}
	
	// 测试数据分布
	testKeys := []string{
		"user:1", "user:2", "user:3", "user:4", "user:5",
		"product:1", "product:2", "product:3", "product:4", "product:5",
	}
	
	fmt.Println("\n初始数据分布:")
	beforeDistribution := make(map[string]string)
	for _, key := range testKeys {
		node := ring.GetNode(key)
		beforeDistribution[key] = node.ID
	}
	
	// 添加新节点
	fmt.Println("\n2. 添加新节点 node4:")
	newNode := &Node{ID: "node4", Address: "192.168.1.4:6379", Weight: 100}
	ring.AddNode(newNode)
	
	// 检查数据迁移
	fmt.Println("\n3. 数据迁移分析:")
	migratedCount := 0
	for _, key := range testKeys {
		node := ring.GetNode(key)
		if beforeDistribution[key] != node.ID {
			fmt.Printf("  📦 %s: %s → %s (迁移)\n", 
				key, beforeDistribution[key], node.ID)
			migratedCount++
		}
	}
	
	migrationRate := float64(migratedCount) / float64(len(testKeys)) * 100
	fmt.Printf("\n迁移统计: %d/%d (%.1f%%)\n", 
		migratedCount, len(testKeys), migrationRate)
	
	fmt.Println("\n💡 虚拟节点优势:")
	fmt.Println("   数据迁移量相对较小")
	fmt.Println("   负载分布更加均匀")
	fmt.Println("   减少热点问题")
}
