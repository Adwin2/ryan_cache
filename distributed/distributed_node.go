package distributed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"tdd-learning/core"
)

// DistributedNode 分布式节点
// 代表集群中的单个节点实例，负责本地数据存储和请求转发
type DistributedNode struct {
	// 节点标识
	nodeID      string
	nodeAddress string
	
	// 全局哈希环引用 - 所有节点共享同一个哈希环视图
	hashRing    *core.DistributedCache
	
	// 本地缓存 - 只存储分配给当前节点的数据
	localCache  *core.LRUCache
	
	// 集群节点映射 - nodeID -> address
	clusterNodes map[string]string
	
	// HTTP客户端 - 用于节点间通信
	httpClient  *http.Client
	
	// 并发控制
	mu          sync.RWMutex
}

// NodeConfig 节点配置
type NodeConfig struct {
	NodeID       string            `yaml:"node_id"`
	Address      string            `yaml:"address"`
	ClusterNodes map[string]string `yaml:"cluster_nodes"`
	CacheSize    int               `yaml:"cache_size"`
	VirtualNodes int               `yaml:"virtual_nodes"`
}

// NewDistributedNode 创建分布式节点实例
func NewDistributedNode(config NodeConfig) *DistributedNode {
	// 1. 创建全局哈希环 - 包含所有集群节点
	allNodes := make([]string, 0, len(config.ClusterNodes))
	for nodeID := range config.ClusterNodes {
		allNodes = append(allNodes, nodeID)
	}
	
	// 使用节点ID作为哈希环的标识符，而不是地址
	var hashRing *core.DistributedCache
	if config.VirtualNodes > 0 {
		hashRing = core.NewDistributedCacheWithVirtualNodes(allNodes, config.VirtualNodes)
	} else {
		hashRing = core.NewDistributedCache(allNodes)
	}
	
	// 2. 创建本地缓存
	cacheSize := config.CacheSize
	if cacheSize <= 0 {
		cacheSize = 1000 // 默认大小
	}
	localCache := core.NewLRUCache(cacheSize)
	
	// 3. 创建节点实例
	node := &DistributedNode{
		nodeID:       config.NodeID,
		nodeAddress:  config.ClusterNodes[config.NodeID], // 从配置中获取自己的地址
		hashRing:     hashRing,
		localCache:   localCache,
		clusterNodes: config.ClusterNodes,
		httpClient: createNodeHTTPClient(5 * time.Second),
	}
	
	return node
}

// Set 设置缓存数据
func (dn *DistributedNode) Set(key, value string) error {
	// 1. 通过哈希环确定数据应该存储在哪个节点
	targetNodeID := dn.hashRing.GetNodeForKey(key)

	// 2. 如果是本地节点，直接存储
	if targetNodeID == dn.nodeID {
		dn.localCache.Set(key, value)
		return nil
	}

	// 3. 如果是远程节点，转发请求（需要读取集群配置，加读锁保护）
	dn.mu.RLock()
	targetAddress, exists := dn.clusterNodes[targetNodeID]
	dn.mu.RUnlock()

	if !exists {
		return fmt.Errorf("目标节点不存在: %s", targetNodeID)
	}

	return dn.forwardSetRequestSafe(targetAddress, key, value)
}

// Get 获取缓存数据
func (dn *DistributedNode) Get(key string) (string, bool, error) {
	// 1. 通过哈希环确定数据存储在哪个节点
	targetNodeID := dn.hashRing.GetNodeForKey(key)

	// 2. 如果是本地节点，直接获取
	if targetNodeID == dn.nodeID {
		value, found := dn.localCache.Get(key)
		return value, found, nil
	}

	// 3. 如果是远程节点，转发请求（需要读取集群配置，加读锁保护）
	dn.mu.RLock()
	targetAddress, exists := dn.clusterNodes[targetNodeID]
	dn.mu.RUnlock()

	if !exists {
		return "", false, fmt.Errorf("目标节点不存在: %s", targetNodeID)
	}

	return dn.forwardGetRequestSafe(targetAddress, key)
}

// Delete 删除缓存数据
func (dn *DistributedNode) Delete(key string) error {
	// 1. 通过哈希环确定数据存储在哪个节点
	targetNodeID := dn.hashRing.GetNodeForKey(key)

	// 2. 如果是本地节点，直接删除
	if targetNodeID == dn.nodeID {
		dn.localCache.Delete(key)
		return nil
	}

	// 3. 如果是远程节点，转发请求（需要读取集群配置，加读锁保护）
	dn.mu.RLock()
	targetAddress, exists := dn.clusterNodes[targetNodeID]
	dn.mu.RUnlock()

	if !exists {
		return fmt.Errorf("目标节点不存在: %s", targetNodeID)
	}

	return dn.forwardDeleteRequestSafe(targetAddress, key)
}

// GetLocalStats 获取本地缓存统计信息
func (dn *DistributedNode) GetLocalStats() map[string]interface{} {
	stats := dn.localCache.GetStats()
	return map[string]interface{}{
		"total_Hits":     stats.Hits,
		"total_Misses":   stats.Misses,
		"total_Size":     dn.localCache.Size(),
		"hit_Rate":       stats.HitRate(),
		"total_Requests": stats.TotalRequests,
	}
}

// GetNodeID 获取节点ID
func (dn *DistributedNode) GetNodeID() string {
	return dn.nodeID
}

// GetNodeAddress 获取节点地址
func (dn *DistributedNode) GetNodeAddress() string {
	return dn.nodeAddress
}

// IsLocalKey 判断key是否属于本地节点
func (dn *DistributedNode) IsLocalKey(key string) bool {
	targetNodeID := dn.hashRing.GetNodeForKey(key)
	return targetNodeID == dn.nodeID
}

// ===== 内部方法 =====

// forwardSetRequestSafe 转发SET请求到目标节点（线程安全版本）
func (dn *DistributedNode) forwardSetRequestSafe(targetAddress, key, value string) error {
	
	// 构造请求数据
	reqData := map[string]string{"value": value}
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %v", err)
	}
	
	// 发送内部API请求
	url := fmt.Sprintf("http://%s/internal/cache/%s", targetAddress, key)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := dn.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("转发请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("目标节点返回错误: %d", resp.StatusCode)
	}
	
	return nil
}

// forwardGetRequestSafe 转发GET请求到目标节点（线程安全版本）
func (dn *DistributedNode) forwardGetRequestSafe(targetAddress, key string) (string, bool, error) {
	
	// 发送内部API请求
	url := fmt.Sprintf("http://%s/internal/cache/%s", targetAddress, key)
	resp, err := dn.httpClient.Get(url)
	if err != nil {
		return "", false, fmt.Errorf("转发请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("目标节点返回错误: %d", resp.StatusCode)
	}
	
	// 解析响应
	var response struct {
		Key   string `json:"key"`
		Value string `json:"value"`
		Found bool   `json:"found"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", false, fmt.Errorf("解析响应失败: %v", err)
	}
	
	return response.Value, response.Found, nil
}

// forwardDeleteRequestSafe 转发DELETE请求到目标节点（线程安全版本）
func (dn *DistributedNode) forwardDeleteRequestSafe(targetAddress, key string) error {
	
	// 发送内部API请求
	url := fmt.Sprintf("http://%s/internal/cache/%s", targetAddress, key)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	
	resp, err := dn.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("转发请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("目标节点返回错误: %d", resp.StatusCode)
	}
	
	return nil
}

// SetLocal 直接设置到本地缓存 - 用于内部API
func (dn *DistributedNode) SetLocal(key, value string) error {
	dn.localCache.Set(key, value)
	return nil
}

// GetLocal 直接从本地缓存获取 - 用于内部API
func (dn *DistributedNode) GetLocal(key string) (string, bool) {
	return dn.localCache.Get(key)
}

// DeleteLocal 直接从本地缓存删除 - 用于内部API
func (dn *DistributedNode) DeleteLocal(key string) {
	dn.localCache.Delete(key)
}

// ===== 集群配置管理方法 =====

// UpdateClusterNodes 更新集群节点配置（线程安全）
func (dn *DistributedNode) UpdateClusterNodes(nodes map[string]string) {
	dn.mu.Lock()
	defer dn.mu.Unlock()

	// 创建新的映射副本
	newNodes := make(map[string]string)
	for nodeID, address := range nodes {
		newNodes[nodeID] = address
	}

	dn.clusterNodes = newNodes
}

// AddClusterNode 添加集群节点（线程安全）
func (dn *DistributedNode) AddClusterNode(nodeID, address string) {
	dn.mu.Lock()
	defer dn.mu.Unlock()

	dn.clusterNodes[nodeID] = address
}

// RemoveClusterNode 移除集群节点（线程安全）
func (dn *DistributedNode) RemoveClusterNode(nodeID string) {
	dn.mu.Lock()
	defer dn.mu.Unlock()

	delete(dn.clusterNodes, nodeID)
}

// GetClusterNodes 获取集群节点配置（线程安全）
func (dn *DistributedNode) GetClusterNodes() map[string]string {
	dn.mu.RLock()
	defer dn.mu.RUnlock()

	// 返回副本，避免外部修改
	nodes := make(map[string]string)
	for nodeID, address := range dn.clusterNodes {
		nodes[nodeID] = address
	}

	return nodes
}
