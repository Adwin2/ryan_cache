package monitoring

import (
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"tdd-learning/core"
)

//监控
// HashRingMonitor 哈希环监控器
// 提供实时的哈希环状态监控和数据迁移追踪功能
type HashRingMonitor struct {
	mu                sync.RWMutex
	distributedCache  *core.DistributedCache
	snapshots         []HashRingSnapshot
	migrationTracker  *MigrationTracker
	enabled           bool
	maxSnapshots      int
}

// HashRingSnapshot 哈希环状态快照
type HashRingSnapshot struct {
	Timestamp       time.Time                    `json:"timestamp"`
	Nodes           []NodeInfo                   `json:"nodes"`
	VirtualNodes    []VirtualNodeInfo            `json:"virtual_nodes"`
	DataDistribution map[string]DataLocationInfo `json:"data_distribution"`
	RingSize        uint32                       `json:"ring_size"`
	LoadBalance     LoadBalanceInfo              `json:"load_balance"`
}

// NodeInfo 节点信息
type NodeInfo struct {
	NodeID       string  `json:"node_id"`
	Address      string  `json:"address,omitempty"`
	HashValue    uint32  `json:"hash_value"`
	Position     float64 `json:"position"`     // 在环上的位置 (0-360度)
	DataCount    int     `json:"data_count"`   // 存储的数据数量
	VirtualCount int     `json:"virtual_count"` // 虚拟节点数量
	Status       string  `json:"status"`       // healthy, unhealthy, joining, leaving
}

// VirtualNodeInfo 虚拟节点信息
type VirtualNodeInfo struct {
	VirtualID    string  `json:"virtual_id"`
	PhysicalNode string  `json:"physical_node"`
	HashValue    uint32  `json:"hash_value"`
	Position     float64 `json:"position"`
}

// DataLocationInfo 数据位置信息
type DataLocationInfo struct {
	Key          string  `json:"key"`
	HashValue    uint32  `json:"hash_value"`
	Position     float64 `json:"position"`
	OwnerNode    string  `json:"owner_node"`
	PreviousNode string  `json:"previous_node,omitempty"` // 迁移前的节点
	MigrationID  string  `json:"migration_id,omitempty"`  // 关联的迁移ID
}

// LoadBalanceInfo 负载均衡信息
type LoadBalanceInfo struct {
	MaxLoad      int     `json:"max_load"`
	MinLoad      int     `json:"min_load"`
	AvgLoad      float64 `json:"avg_load"`
	Variance     float64 `json:"variance"`
	BalanceScore float64 `json:"balance_score"` // 0-100，100表示完全均衡
}
//监控

//监控
// NewHashRingMonitor 创建哈希环监控器
func NewHashRingMonitor(dc *core.DistributedCache) *HashRingMonitor {
	return &HashRingMonitor{
		distributedCache: dc,
		snapshots:       make([]HashRingSnapshot, 0),
		migrationTracker: NewMigrationTracker(),
		enabled:         true,
		maxSnapshots:    100, // 保留最近100个快照
	}
}

// Enable 启用监控
func (hrm *HashRingMonitor) Enable() {
	hrm.mu.Lock()
	defer hrm.mu.Unlock()
	hrm.enabled = true
}

// Disable 禁用监控
func (hrm *HashRingMonitor) Disable() {
	hrm.mu.Lock()
	defer hrm.mu.Unlock()
	hrm.enabled = false
}

// IsEnabled 检查是否启用
func (hrm *HashRingMonitor) IsEnabled() bool {
	hrm.mu.RLock()
	defer hrm.mu.RUnlock()
	return hrm.enabled
}
//监控

//监控
// CaptureSnapshot 捕获当前哈希环状态快照
func (hrm *HashRingMonitor) CaptureSnapshot() *HashRingSnapshot {
	if !hrm.IsEnabled() {
		return nil
	}

	hrm.mu.Lock()
	defer hrm.mu.Unlock()

	// 获取分布式缓存的读锁
	hrm.distributedCache.Mu.RLock()
	defer hrm.distributedCache.Mu.RUnlock()

	snapshot := HashRingSnapshot{
		Timestamp:       time.Now(),
		Nodes:           hrm.extractNodeInfo(),
		VirtualNodes:    hrm.extractVirtualNodeInfo(),
		DataDistribution: hrm.extractDataDistribution(),
		RingSize:        hrm.calculateRingSize(),
		LoadBalance:     hrm.calculateLoadBalance(),
	}

	// 添加到快照历史
	hrm.snapshots = append(hrm.snapshots, snapshot)
	
	// 保持快照数量限制
	if len(hrm.snapshots) > hrm.maxSnapshots {
		hrm.snapshots = hrm.snapshots[1:]
	}

	return &snapshot
}

// extractNodeInfo 提取节点信息
func (hrm *HashRingMonitor) extractNodeInfo() []NodeInfo {
	nodeInfoMap := make(map[string]*NodeInfo)
	
	// 初始化节点信息
	for _, node := range hrm.distributedCache.Nodes {
		nodeInfoMap[node] = &NodeInfo{
			NodeID:       node,
			HashValue:    0, // 将使用第一个虚拟节点的hash值作为代表
			DataCount:    0,
			VirtualCount: 0,
			Status:       "healthy", // 简化状态，实际应该从集群管理器获取
		}
	}

	// 统计虚拟节点
	for hash, node := range hrm.distributedCache.HashRing {
		if nodeInfo, exists := nodeInfoMap[node]; exists {
			nodeInfo.VirtualCount++
			if nodeInfo.HashValue == 0 {
				nodeInfo.HashValue = hash // 使用第一个虚拟节点的hash作为代表
			}
		}
	}

	// 统计数据分布
	for node, localCache := range hrm.distributedCache.LocalCaches {
		if nodeInfo, exists := nodeInfoMap[node]; exists {
			nodeInfo.DataCount = localCache.Size()
		}
	}

	// 计算位置（角度）
	for _, nodeInfo := range nodeInfoMap {
		nodeInfo.Position = hrm.hashToPosition(nodeInfo.HashValue)
	}

	// 转换为切片并排序
	result := make([]NodeInfo, 0, len(nodeInfoMap))
	for _, nodeInfo := range nodeInfoMap {
		result = append(result, *nodeInfo)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].HashValue < result[j].HashValue
	})

	return result
}

// extractVirtualNodeInfo 提取虚拟节点信息
func (hrm *HashRingMonitor) extractVirtualNodeInfo() []VirtualNodeInfo {
	var virtualNodes []VirtualNodeInfo

	for hash, physicalNode := range hrm.distributedCache.HashRing {
		virtualNodes = append(virtualNodes, VirtualNodeInfo{
			VirtualID:    fmt.Sprintf("%s#%d", physicalNode, hash),
			PhysicalNode: physicalNode,
			HashValue:    hash,
			Position:     hrm.hashToPosition(hash),
		})
	}

	sort.Slice(virtualNodes, func(i, j int) bool {
		return virtualNodes[i].HashValue < virtualNodes[j].HashValue
	})

	return virtualNodes
}

// extractDataDistribution 提取数据分布信息
func (hrm *HashRingMonitor) extractDataDistribution() map[string]DataLocationInfo {
	// 注意：这里需要实际的数据key列表
	// 由于LRUCache没有提供遍历接口，这里返回空map
	// 在实际使用中，需要从外部传入数据key列表
	return make(map[string]DataLocationInfo)
}

// UpdateDataDistribution 更新数据分布信息（外部调用）
func (hrm *HashRingMonitor) UpdateDataDistribution(dataKeys []string) {
	if !hrm.IsEnabled() {
		return
	}

	hrm.mu.Lock()
	defer hrm.mu.Unlock()

	if len(hrm.snapshots) == 0 {
		return
	}

	// 更新最新快照的数据分布
	latestSnapshot := &hrm.snapshots[len(hrm.snapshots)-1]
	latestSnapshot.DataDistribution = make(map[string]DataLocationInfo)

	for _, key := range dataKeys {
		// 使用公开的GetNodeForKey方法获取节点，然后计算hash
		ownerNode := hrm.distributedCache.GetNodeForKey(key)
		// 简化处理：使用key的长度作为hash值的近似
		hash := uint32(len(key) * 1000)
		
		latestSnapshot.DataDistribution[key] = DataLocationInfo{
			Key:       key,
			HashValue: hash,
			Position:  hrm.hashToPosition(hash),
			OwnerNode: ownerNode,
		}
	}
}

// calculateRingSize 计算环大小
func (hrm *HashRingMonitor) calculateRingSize() uint32 {
	return uint32(len(hrm.distributedCache.HashRing))
}

// calculateLoadBalance 计算负载均衡信息
func (hrm *HashRingMonitor) calculateLoadBalance() LoadBalanceInfo {
	if len(hrm.distributedCache.Nodes) == 0 {
		return LoadBalanceInfo{}
	}

	loads := make([]int, 0, len(hrm.distributedCache.Nodes))
	totalLoad := 0

	for _, cache := range hrm.distributedCache.LocalCaches {
		load := cache.Size()
		loads = append(loads, load)
		totalLoad += load
	}

	if len(loads) == 0 {
		return LoadBalanceInfo{}
	}

	sort.Ints(loads)
	maxLoad := loads[len(loads)-1]
	minLoad := loads[0]
	avgLoad := float64(totalLoad) / float64(len(loads))

	// 计算方差
	variance := 0.0
	for _, load := range loads {
		diff := float64(load) - avgLoad
		variance += diff * diff
	}
	variance /= float64(len(loads))

	// 计算均衡分数 (0-100)
	balanceScore := 100.0
	if maxLoad > 0 {
		balanceScore = (1.0 - float64(maxLoad-minLoad)/float64(maxLoad)) * 100
	}

	return LoadBalanceInfo{
		MaxLoad:      maxLoad,
		MinLoad:      minLoad,
		AvgLoad:      avgLoad,
		Variance:     variance,
		BalanceScore: balanceScore,
	}
}

// hashToPosition 将hash值转换为环上的位置（角度）
func (hrm *HashRingMonitor) hashToPosition(hash uint32) float64 {
	// 将32位hash值映射到0-360度
	return float64(hash) / float64(^uint32(0)) * 360.0
}

// GetLatestSnapshot 获取最新快照
func (hrm *HashRingMonitor) GetLatestSnapshot() *HashRingSnapshot {
	hrm.mu.RLock()
	defer hrm.mu.RUnlock()

	if len(hrm.snapshots) == 0 {
		return nil
	}

	return &hrm.snapshots[len(hrm.snapshots)-1]
}

// GetSnapshotHistory 获取快照历史
func (hrm *HashRingMonitor) GetSnapshotHistory(limit int) []HashRingSnapshot {
	hrm.mu.RLock()
	defer hrm.mu.RUnlock()

	if limit <= 0 || limit > len(hrm.snapshots) {
		limit = len(hrm.snapshots)
	}

	start := len(hrm.snapshots) - limit
	result := make([]HashRingSnapshot, limit)
	copy(result, hrm.snapshots[start:])

	return result
}

// GetMigrationTracker 获取迁移追踪器
func (hrm *HashRingMonitor) GetMigrationTracker() *MigrationTracker {
	return hrm.migrationTracker
}

// ToJSON 将快照转换为JSON
func (snapshot *HashRingSnapshot) ToJSON() ([]byte, error) {
	return json.MarshalIndent(snapshot, "", "  ")
}
//监控
