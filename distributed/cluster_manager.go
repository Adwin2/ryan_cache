package distributed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// ClusterManager 集群管理器
// 负责节点发现、健康检查、故障恢复等
type ClusterManager struct {
	nodeID       string
	nodes        map[string]*NodeInfo // nodeID -> NodeInfo
	mu           sync.RWMutex
	httpClient   *http.Client
	healthTicker *time.Ticker
	stopChan     chan struct{}
}

// NodeInfo 节点信息
type NodeInfo struct {
	NodeID      string    `json:"node_id"`
	Address     string    `json:"address"`
	Status      string    `json:"status"`      // "healthy", "unhealthy", "unknown"
	LastSeen    time.Time `json:"last_seen"`
	ResponseTime int64    `json:"response_time"` // 响应时间(毫秒)
}

// ClusterStatus 集群状态
type ClusterStatus struct {
	TotalNodes    int         `json:"total_nodes"`
	HealthyNodes  int         `json:"healthy_nodes"`
	UnhealthyNodes int        `json:"unhealthy_nodes"`
	Nodes         []*NodeInfo `json:"nodes"`
	LastUpdate    time.Time   `json:"last_update"`
}

// NewClusterManager 创建集群管理器
func NewClusterManager(nodeID string, clusterNodes map[string]string) *ClusterManager {
	cm := &ClusterManager{
		nodeID: nodeID,
		nodes:  make(map[string]*NodeInfo),
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
		stopChan: make(chan struct{}),
	}
	
	// 初始化节点信息
	for id, addr := range clusterNodes {
		cm.nodes[id] = &NodeInfo{
			NodeID:   id,
			Address:  addr,
			Status:   "unknown",
			LastSeen: time.Now(),
		}
	}
	
	return cm
}

// Start 启动集群管理器
func (cm *ClusterManager) Start() error {
	log.Printf("🌐 启动集群管理器，节点ID: %s", cm.nodeID)
	
	// 启动健康检查
	cm.healthTicker = time.NewTicker(10 * time.Second)
	go cm.healthCheckLoop()
	
	// 向其他节点宣告自己的存在
	go cm.announceToCluster()
	
	return nil
}

// Stop 停止集群管理器
func (cm *ClusterManager) Stop() {
	if cm.healthTicker != nil {
		cm.healthTicker.Stop()
	}
	
	close(cm.stopChan)
	log.Printf("🛑 集群管理器已停止")
}

// announceToCluster 向集群宣告节点加入
func (cm *ClusterManager) announceToCluster() {
	time.Sleep(2 * time.Second) // 等待服务器启动
	
	cm.mu.RLock()
	currentNode := cm.nodes[cm.nodeID]
	cm.mu.RUnlock()
	
	if currentNode == nil {
		return
	}
	
	joinData := map[string]string{
		"node_id": cm.nodeID,
		"address": currentNode.Address,
	}
	
	for nodeID, node := range cm.nodes {
		if nodeID != cm.nodeID {
			cm.notifyNodeJoin(node.Address, joinData)
		}
	}
}

// notifyNodeJoin 通知其他节点有新节点加入
func (cm *ClusterManager) notifyNodeJoin(targetAddr string, joinData map[string]string) {
	jsonData, _ := json.Marshal(joinData)
	url := fmt.Sprintf("http://%s/internal/cluster/join", targetAddr)
	
	resp, err := cm.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("⚠️ 通知节点加入失败 %s: %v", targetAddr, err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusOK {
		log.Printf("✅ 成功通知节点 %s 关于节点加入", targetAddr)
	}
}

// healthCheckLoop 健康检查循环
func (cm *ClusterManager) healthCheckLoop() {
	for {
		select {
		case <-cm.healthTicker.C:
			cm.performHealthCheck()
		case <-cm.stopChan:
			return
		}
	}
}

// performHealthCheck 执行健康检查
func (cm *ClusterManager) performHealthCheck() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	for nodeID, node := range cm.nodes {
		if nodeID == cm.nodeID {
			// 自己总是健康的
			node.Status = "healthy"
			node.LastSeen = time.Now()
			continue
		}
		
		// 检查其他节点
		start := time.Now()
		healthy := cm.checkNodeHealth(node.Address)
		responseTime := time.Since(start).Milliseconds()
		
		if healthy {
			node.Status = "healthy"
			node.LastSeen = time.Now()
			node.ResponseTime = responseTime
		} else {
			node.Status = "unhealthy"
			node.ResponseTime = -1
		}
	}
}

// checkNodeHealth 检查单个节点健康状态
func (cm *ClusterManager) checkNodeHealth(address string) bool {
	url := fmt.Sprintf("http://%s/internal/cluster/health", address)
	
	resp, err := cm.httpClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	return resp.StatusCode == http.StatusOK
}

// AddNode 添加节点到集群
func (cm *ClusterManager) AddNode(nodeID, address string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.nodes[nodeID] = &NodeInfo{
		NodeID:   nodeID,
		Address:  address,
		Status:   "unknown",
		LastSeen: time.Now(),
	}
	
	log.Printf("➕ 添加节点到集群: %s (%s)", nodeID, address)
}

// RemoveNode 从集群中移除节点
func (cm *ClusterManager) RemoveNode(nodeID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	delete(cm.nodes, nodeID)
	log.Printf("➖ 从集群中移除节点: %s", nodeID)
}

// GetNodes 获取所有节点信息
func (cm *ClusterManager) GetNodes() map[string]*NodeInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	// 返回副本
	nodes := make(map[string]*NodeInfo)
	for id, node := range cm.nodes {
		nodeCopy := *node
		nodes[id] = &nodeCopy
	}
	
	return nodes
}

// GetHealthyNodes 获取健康的节点
func (cm *ClusterManager) GetHealthyNodes() map[string]*NodeInfo {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	healthyNodes := make(map[string]*NodeInfo)
	for id, node := range cm.nodes {
		if node.Status == "healthy" {
			nodeCopy := *node
			healthyNodes[id] = &nodeCopy
		}
	}
	
	return healthyNodes
}

// GetClusterStatus 获取集群状态
func (cm *ClusterManager) GetClusterStatus() *ClusterStatus {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	status := &ClusterStatus{
		TotalNodes:   len(cm.nodes),
		LastUpdate:   time.Now(),
		Nodes:        make([]*NodeInfo, 0, len(cm.nodes)),
	}
	
	for _, node := range cm.nodes {
		nodeCopy := *node
		status.Nodes = append(status.Nodes, &nodeCopy)
		
		if node.Status == "healthy" {
			status.HealthyNodes++
		} else {
			status.UnhealthyNodes++
		}
	}
	
	return status
}

// IsHealthy 检查集群是否健康
func (cm *ClusterManager) IsHealthy() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	healthyCount := 0
	for _, node := range cm.nodes {
		if node.Status == "healthy" {
			healthyCount++
		}
	}
	
	// 至少一半节点健康才认为集群健康
	return healthyCount >= len(cm.nodes)/2
}

// Leave 离开集群
func (cm *ClusterManager) Leave() error {
	leaveData := map[string]string{
		"node_id": cm.nodeID,
	}
	
	cm.mu.RLock()
	nodes := make([]*NodeInfo, 0, len(cm.nodes))
	for _, node := range cm.nodes {
		if node.NodeID != cm.nodeID {
			nodes = append(nodes, node)
		}
	}
	cm.mu.RUnlock()
	
	// 通知其他节点自己要离开
	for _, node := range nodes {
		cm.notifyNodeLeave(node.Address, leaveData)
	}
	
	return nil
}

// notifyNodeLeave 通知其他节点有节点离开
func (cm *ClusterManager) notifyNodeLeave(targetAddr string, leaveData map[string]string) {
	jsonData, _ := json.Marshal(leaveData)
	url := fmt.Sprintf("http://%s/internal/cluster/leave", targetAddr)
	
	resp, err := cm.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("⚠️ 通知节点离开失败 %s: %v", targetAddr, err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusOK {
		log.Printf("✅ 成功通知节点 %s 关于节点离开", targetAddr)
	}
}
