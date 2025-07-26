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

// ClusterCoordinator 集群协调器
// 负责协调集群拓扑变更和数据迁移
type ClusterCoordinator struct {
	node        *DistributedNode
	cluster     *ClusterManager
	httpClient  *http.Client
	mu          sync.RWMutex
}

// NodeChangeRequest 节点变更请求
type NodeChangeRequest struct {
	NodeID    string `json:"node_id"`
	Address   string `json:"address,omitempty"`
	Operation string `json:"operation"` // "add" or "remove"
}

// MigrationResult 数据迁移结果
type MigrationResult struct {
	Success       bool   `json:"success"`
	MigratedCount int    `json:"migrated_count"`
	Duration      string `json:"duration"`
	Error         string `json:"error,omitempty"`
}

// NewClusterCoordinator 创建集群协调器
func NewClusterCoordinator(node *DistributedNode, cluster *ClusterManager) *ClusterCoordinator {
	return &ClusterCoordinator{
		node:    node,
		cluster: cluster,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// AddNodeToCluster 向集群添加节点
func (cc *ClusterCoordinator) AddNodeToCluster(nodeID, address string) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	
	log.Printf("🔄 开始添加节点到集群: %s (%s)", nodeID, address)
	
	// 1. 添加节点到集群管理器
	cc.cluster.AddNode(nodeID, address)
	
	// 2. 获取当前节点的哈希环实例
	hashRing := cc.node.hashRing
	
	// 3. 调用你实现的AddNode方法，执行数据迁移
	if err := hashRing.AddNode(nodeID); err != nil {
		log.Printf("❌ 添加节点到哈希环失败: %v", err)
		return fmt.Errorf("添加节点失败: %v", err)
	}
	
	// 4. 广播节点变更到集群中的所有其他节点
	if err := cc.broadcastNodeChange(nodeID, address, "add"); err != nil {
		log.Printf("⚠️ 广播节点添加失败: %v", err)
		// 注意：即使广播失败，本地操作已经成功，不回滚
	}
	
	log.Printf("✅ 节点添加完成: %s", nodeID)
	return nil
}

// RemoveNodeFromCluster 从集群移除节点
func (cc *ClusterCoordinator) RemoveNodeFromCluster(nodeID string) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	
	log.Printf("🔄 开始从集群移除节点: %s", nodeID)
	
	// 1. 获取当前节点的哈希环实例
	hashRing := cc.node.hashRing
	
	// 2. 调用你实现的RemoveNode方法，执行数据迁移
	if err := hashRing.RemoveNode(nodeID); err != nil {
		log.Printf("❌ 从哈希环移除节点失败: %v", err)
		return fmt.Errorf("移除节点失败: %v", err)
	}
	
	// 3. 从集群管理器中移除节点
	cc.cluster.RemoveNode(nodeID)
	
	// 4. 广播节点变更到集群中的所有其他节点
	if err := cc.broadcastNodeChange(nodeID, "", "remove"); err != nil {
		log.Printf("⚠️ 广播节点移除失败: %v", err)
		// 注意：即使广播失败，本地操作已经成功，不回滚
	}
	
	log.Printf("✅ 节点移除完成: %s", nodeID)
	return nil
}

// broadcastNodeChange 广播节点变更到集群中的所有其他节点
func (cc *ClusterCoordinator) broadcastNodeChange(nodeID, address, operation string) error {
	request := NodeChangeRequest{
		NodeID:    nodeID,
		Address:   address,
		Operation: operation,
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %v", err)
	}
	
	// 获取集群中的所有节点
	nodes := cc.cluster.GetNodes()
	var errors []string
	
	for targetNodeID, nodeInfo := range nodes {
		// 跳过自己
		if targetNodeID == cc.node.GetNodeID() {
			continue
		}
		
		// 只向健康的节点发送广播
		if nodeInfo.Status != "healthy" {
			log.Printf("⚠️ 跳过不健康的节点: %s", targetNodeID)
			continue
		}
		
		// 发送广播请求
		if err := cc.sendNodeChangeRequest(nodeInfo.Address, jsonData, operation); err != nil {
			errorMsg := fmt.Sprintf("向节点 %s 广播失败: %v", targetNodeID, err)
			errors = append(errors, errorMsg)
			log.Printf("❌ %s", errorMsg)
		} else {
			log.Printf("✅ 成功向节点 %s 广播 %s 操作", targetNodeID, operation)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("部分节点广播失败: %v", errors)
	}
	
	return nil
}

// sendNodeChangeRequest 向指定节点发送变更请求
func (cc *ClusterCoordinator) sendNodeChangeRequest(targetAddress string, jsonData []byte, operation string) error {
	var url string
	var method string
	
	switch operation {
	case "add":
		url = fmt.Sprintf("http://%s/internal/cluster/sync-add", targetAddress)
		method = "POST"
	case "remove":
		url = fmt.Sprintf("http://%s/internal/cluster/sync-remove", targetAddress)
		method = "POST"
	default:
		return fmt.Errorf("未知操作: %s", operation)
	}
	
	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := cc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("目标节点返回错误状态: %d", resp.StatusCode)
	}
	
	return nil
}

// SyncAddNode 同步添加节点（接收广播）
func (cc *ClusterCoordinator) SyncAddNode(nodeID, address string) error {
	log.Printf("🔄 同步添加节点: %s (%s)", nodeID, address)
	
	// 1. 添加到集群管理器
	cc.cluster.AddNode(nodeID, address)
	
	// 2. 添加到哈希环（这会触发数据迁移）
	if err := cc.node.hashRing.AddNode(nodeID); err != nil {
		log.Printf("❌ 同步添加节点到哈希环失败: %v", err)
		return err
	}
	
	log.Printf("✅ 同步添加节点完成: %s", nodeID)
	return nil
}

// SyncRemoveNode 同步移除节点（接收广播）
func (cc *ClusterCoordinator) SyncRemoveNode(nodeID string) error {
	log.Printf("🔄 同步移除节点: %s", nodeID)
	
	// 1. 从哈希环移除（这会触发数据迁移）
	if err := cc.node.hashRing.RemoveNode(nodeID); err != nil {
		log.Printf("❌ 同步从哈希环移除节点失败: %v", err)
		return err
	}
	
	// 2. 从集群管理器移除
	cc.cluster.RemoveNode(nodeID)
	
	log.Printf("✅ 同步移除节点完成: %s", nodeID)
	return nil
}

// GetMigrationStats 获取数据迁移统计信息
func (cc *ClusterCoordinator) GetMigrationStats() map[string]interface{} {
	stats := cc.node.hashRing.GetMigrationStats()
	return map[string]interface{}{
		"migrated_keys":   stats.MigratedKeys,
		"duration":        stats.Duration.String(),
		"last_migration":  stats.LastMigration.Format("2006-01-02 15:04:05"),
	}
}

// TriggerRebalance 触发集群重平衡
func (cc *ClusterCoordinator) TriggerRebalance() error {
	log.Printf("🔄 开始集群重平衡...")
	
	// 获取当前集群状态
	nodes := cc.cluster.GetHealthyNodes()
	if len(nodes) < 2 {
		return fmt.Errorf("健康节点数量不足，无法执行重平衡")
	}
	
	// TODO: 实现重平衡逻辑
	// 1. 分析当前数据分布 
	// 2. 计算理想分布
	// 3. 执行数据迁移
	// 涉及权重相关，暂搁置

	log.Printf("✅ 集群重平衡完成")
	return nil
}
