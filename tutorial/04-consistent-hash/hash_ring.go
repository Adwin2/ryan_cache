package main

import (
	"crypto/md5"
	"fmt"
	"sort"
	"sync"
)

// Node 表示一个缓存节点
type Node struct {
	ID       string
	Address  string
	Weight   int // 节点权重
}

func (n *Node) String() string {
	return fmt.Sprintf("Node{ID: %s, Address: %s, Weight: %d}", n.ID, n.Address, n.Weight)
}

// HashRing 一致性哈希环
type HashRing struct {
	nodes     map[uint32]*Node // 哈希值 -> 节点映射
	sortedKeys []uint32         // 排序的哈希值列表
	mu        sync.RWMutex     // 读写锁
}

// NewHashRing 创建新的哈希环
func NewHashRing() *HashRing {
	return &HashRing{
		nodes:     make(map[uint32]*Node),
		sortedKeys: make([]uint32, 0),
	}
}

// hash 计算字符串的哈希值
func (hr *HashRing) hash(key string) uint32 {
	h := md5.New()
	h.Write([]byte(key))
	hashBytes := h.Sum(nil)
	
	// 取前4个字节作为uint32
	return uint32(hashBytes[0])<<24 + 
		   uint32(hashBytes[1])<<16 + 
		   uint32(hashBytes[2])<<8 + 
		   uint32(hashBytes[3])
}

// AddNode 添加节点到哈希环
func (hr *HashRing) AddNode(node *Node) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	
	// 计算节点的哈希值
	nodeHash := hr.hash(node.ID)
	
	// 检查是否已存在
	if _, exists := hr.nodes[nodeHash]; exists {
		fmt.Printf("⚠️ 节点 %s 已存在，跳过添加\n", node.ID)
		return
	}
	
	// 添加节点
	hr.nodes[nodeHash] = node
	hr.sortedKeys = append(hr.sortedKeys, nodeHash)
	
	// 重新排序
	sort.Slice(hr.sortedKeys, func(i, j int) bool {
		return hr.sortedKeys[i] < hr.sortedKeys[j]
	})
	
	fmt.Printf("✅ 添加节点: %s (哈希值: %d)\n", node.ID, nodeHash)
}

// RemoveNode 从哈希环移除节点
func (hr *HashRing) RemoveNode(nodeID string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	
	nodeHash := hr.hash(nodeID)
	
	// 检查节点是否存在
	if _, exists := hr.nodes[nodeHash]; !exists {
		fmt.Printf("⚠️ 节点 %s 不存在，无法移除\n", nodeID)
		return
	}
	
	// 移除节点
	delete(hr.nodes, nodeHash)
	
	// 从排序列表中移除
	for i, key := range hr.sortedKeys {
		if key == nodeHash {
			hr.sortedKeys = append(hr.sortedKeys[:i], hr.sortedKeys[i+1:]...)
			break
		}
	}
	
	fmt.Printf("❌ 移除节点: %s (哈希值: %d)\n", nodeID, nodeHash)
}

// GetNode 根据key获取对应的节点
func (hr *HashRing) GetNode(key string) *Node {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	
	if len(hr.sortedKeys) == 0 {
		return nil
	}
	
	keyHash := hr.hash(key)
	
	// 使用二分查找找到第一个大于等于keyHash的节点
	idx := sort.Search(len(hr.sortedKeys), func(i int) bool {
		return hr.sortedKeys[i] >= keyHash
	})
	
	// 如果没找到，说明key的哈希值比所有节点都大，选择第一个节点（环形）
	if idx == len(hr.sortedKeys) {
		idx = 0
	}
	
	nodeHash := hr.sortedKeys[idx]
	node := hr.nodes[nodeHash]
	
	fmt.Printf("🔍 Key: %s (哈希: %d) → Node: %s (哈希: %d)\n", 
		key, keyHash, node.ID, nodeHash)
	
	return node
}

// GetNodes 获取多个副本节点
func (hr *HashRing) GetNodes(key string, count int) []*Node {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	
	if len(hr.sortedKeys) == 0 || count <= 0 {
		return nil
	}
	
	keyHash := hr.hash(key)
	nodes := make([]*Node, 0, count)
	visited := make(map[string]bool)
	
	// 找到起始位置
	startIdx := sort.Search(len(hr.sortedKeys), func(i int) bool {
		return hr.sortedKeys[i] >= keyHash
	})
	
	// 环形遍历，获取count个不同的节点
	for i := 0; i < len(hr.sortedKeys) && len(nodes) < count; i++ {
		idx := (startIdx + i) % len(hr.sortedKeys)
		nodeHash := hr.sortedKeys[idx]
		node := hr.nodes[nodeHash]
		
		// 避免重复节点
		if !visited[node.ID] {
			nodes = append(nodes, node)
			visited[node.ID] = true
		}
	}
	
	fmt.Printf("🔍 Key: %s 的 %d 个副本节点: ", key, count)
	for i, node := range nodes {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(node.ID)
	}
	fmt.Println()
	
	return nodes
}

// GetAllNodes 获取所有节点
func (hr *HashRing) GetAllNodes() []*Node {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	
	nodes := make([]*Node, 0, len(hr.nodes))
	for _, node := range hr.nodes {
		nodes = append(nodes, node)
	}
	
	return nodes
}

// PrintRing 打印哈希环状态
func (hr *HashRing) PrintRing() {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	
	fmt.Println("\n📊 哈希环状态:")
	fmt.Printf("节点数量: %d\n", len(hr.nodes))
	
	if len(hr.sortedKeys) == 0 {
		fmt.Println("哈希环为空")
		return
	}
	
	fmt.Println("节点分布 (按哈希值排序):")
	for i, nodeHash := range hr.sortedKeys {
		node := hr.nodes[nodeHash]
		fmt.Printf("  %d. %s (哈希: %d)\n", i+1, node.ID, nodeHash)
	}
}

// CalculateDataDistribution 计算数据分布情况
func (hr *HashRing) CalculateDataDistribution(keys []string) map[string]int {
	distribution := make(map[string]int)
	
	for _, key := range keys {
		node := hr.GetNode(key)
		if node != nil {
			distribution[node.ID]++
		}
	}
	
	return distribution
}






