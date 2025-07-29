package distributed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// APIHandlers API处理器
// 处理HTTP请求并调用底层的DistributedNode
type APIHandlers struct {
	node        *DistributedNode
	cluster     *ClusterManager
	coordinator *ClusterCoordinator
}

// CacheRequest 缓存请求
type CacheRequest struct {
	Value string `json:"value" binding:"required"`
}

// CacheResponse 缓存响应
type CacheResponse struct {
	Key     string `json:"key"`
	Value   string `json:"value,omitempty"`
	Found   bool   `json:"found"`
	NodeID  string `json:"node_id"`
	Message string `json:"message,omitempty"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// StatsResponse 统计响应
type StatsResponse struct {
	NodeID        string                 `json:"node_id"`
	CacheStats    map[string]interface{} `json:"cache_stats"`
	ClusterStats  *ClusterStatus         `json:"cluster_stats"`
	Timestamp     string                 `json:"timestamp"`
}

// NewAPIHandlers 创建API处理器
func NewAPIHandlers(node *DistributedNode, cluster *ClusterManager) *APIHandlers {
	coordinator := NewClusterCoordinator(node, cluster)
	return &APIHandlers{
		node:        node,
		cluster:     cluster,
		coordinator: coordinator,
	}
}

// ===== 客户端API处理器 =====

// HandleGet 处理GET请求
func (h *APIHandlers) HandleGet(c *gin.Context) {
	key := c.Param("key")

	// 使用DistributedNode的Get方法，它会自动处理路由和转发
	value, found, err := h.node.Get(key)
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "cache_error", err.Error())
		return
	}

	c.JSON(http.StatusOK, CacheResponse{
		Key:    key,
		Value:  value,
		Found:  found,
		NodeID: h.node.GetNodeID(),
	})
}

// HandleSet 处理PUT请求
func (h *APIHandlers) HandleSet(c *gin.Context) {
	key := c.Param("key")

	var req CacheRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// 使用DistributedNode的Set方法，它会自动处理路由和转发
	if err := h.node.Set(key, req.Value); err != nil {
		h.sendError(c, http.StatusInternalServerError, "cache_error", err.Error())
		return
	}

	c.JSON(http.StatusOK, CacheResponse{
		Key:     key,
		Value:   req.Value,
		Found:   true,
		NodeID:  h.node.GetNodeID(),
		Message: "success",
	})
}

// HandleDelete 处理DELETE请求
func (h *APIHandlers) HandleDelete(c *gin.Context) {
	key := c.Param("key")

	// 使用DistributedNode的Delete方法，它会自动处理路由和转发
	if err := h.node.Delete(key); err != nil {
		h.sendError(c, http.StatusInternalServerError, "cache_error", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "deleted",
		"key":     key,
		"node_id": h.node.GetNodeID(),
	})
}

// HandleGetStats 处理统计请求
func (h *APIHandlers) HandleGetStats(c *gin.Context) {
	cacheStats := h.node.GetLocalStats()
	clusterStats := h.cluster.GetClusterStatus()

	c.JSON(http.StatusOK, StatsResponse{
		NodeID:       h.node.GetNodeID(),
		CacheStats:   cacheStats,
		ClusterStats: clusterStats,
		Timestamp:    time.Now().Format(time.RFC3339),
	})
}

// HandleHealthCheck 处理健康检查
func (h *APIHandlers) HandleHealthCheck(c *gin.Context) {
	healthy := h.cluster.IsHealthy()
	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status":    status,
		"node_id":   h.cluster.nodeID,
		"timestamp": time.Now().Format(time.RFC3339),
		"uptime":    time.Now().Unix(),
	})
}

// ===== 内部API处理器 =====

// HandleInternalGet 处理内部GET请求
func (h *APIHandlers) HandleInternalGet(c *gin.Context) {
	// 🔧 修复：内部请求直接访问本地缓存，不进行转发
	key := c.Param("key")

	// 直接从本地缓存获取数据
	value, found := h.node.GetLocal(key)

	c.JSON(http.StatusOK, CacheResponse{
		Key:    key,
		Value:  value,
		Found:  found,
		NodeID: h.node.GetNodeID(),
	})
}

// HandleInternalSet 处理内部SET请求
func (h *APIHandlers) HandleInternalSet(c *gin.Context) {
	key := c.Param("key")

	var req CacheRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.sendError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	// 直接设置到本地缓存
	if err := h.node.SetLocal(key, req.Value); err != nil {
		h.sendError(c, http.StatusInternalServerError, "cache_error", err.Error())
		return
	}

	c.JSON(http.StatusOK, CacheResponse{
		Key:     key,
		Value:   req.Value,
		Found:   true,
		NodeID:  h.node.GetNodeID(),
		Message: "success",
	})
}

// HandleInternalDelete 处理内部DELETE请求
func (h *APIHandlers) HandleInternalDelete(c *gin.Context) {
	key := c.Param("key")

	// 直接从本地缓存删除
	h.node.DeleteLocal(key)

	c.JSON(http.StatusOK, gin.H{
		"message": "deleted",
		"key":     key,
		"node_id": h.node.GetNodeID(),
	})
}

// HandleNodeJoin 处理节点加入通知
func (h *APIHandlers) HandleNodeJoin(c *gin.Context) {
	var joinData map[string]string
	if err := c.ShouldBindJSON(&joinData); err != nil {
		h.sendError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	nodeID := joinData["node_id"]
	address := joinData["address"]

	// 使用集群协调器添加节点（包含数据迁移和广播）
	if err := h.coordinator.AddNodeToCluster(nodeID, address); err != nil {
		h.sendError(c, http.StatusInternalServerError, "add_node_error", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "node joined successfully",
		"node_id": nodeID,
	})
}

// HandleNodeLeave 处理节点离开通知
func (h *APIHandlers) HandleNodeLeave(c *gin.Context) {
	var leaveData map[string]string
	if err := c.ShouldBindJSON(&leaveData); err != nil {
		h.sendError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	nodeID := leaveData["node_id"]

	// 使用集群协调器移除节点（包含数据迁移和广播）
	if err := h.coordinator.RemoveNodeFromCluster(nodeID); err != nil {
		h.sendError(c, http.StatusInternalServerError, "remove_node_error", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "node left successfully",
		"node_id": nodeID,
	})
}

// HandleClusterHealth 处理集群健康检查
func (h *APIHandlers) HandleClusterHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"node_id":   h.cluster.nodeID,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// HandleSyncAddNode 处理同步添加节点请求（接收广播）
func (h *APIHandlers) HandleSyncAddNode(c *gin.Context) {
	var request NodeChangeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if err := h.coordinator.SyncAddNode(request.NodeID, request.Address); err != nil {
		h.sendError(c, http.StatusInternalServerError, "sync_add_error", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "node synced successfully",
		"node_id": request.NodeID,
	})
}

// HandleSyncRemoveNode 处理同步移除节点请求（接收广播）
func (h *APIHandlers) HandleSyncRemoveNode(c *gin.Context) {
	var request NodeChangeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.sendError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	if err := h.coordinator.SyncRemoveNode(request.NodeID); err != nil {
		h.sendError(c, http.StatusInternalServerError, "sync_remove_error", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "node removed successfully",
		"node_id": request.NodeID,
	})
}

// ===== 管理API处理器 =====

// HandleGetCluster 获取集群信息
func (h *APIHandlers) HandleGetCluster(c *gin.Context) {
	clusterStatus := h.cluster.GetClusterStatus()
	
	c.JSON(http.StatusOK, gin.H{
		"cluster_status": clusterStatus,
		"current_node":   h.cluster.nodeID,
		"timestamp":      time.Now().Format(time.RFC3339),
	})
}

// HandleGetNodes 获取节点列表
func (h *APIHandlers) HandleGetNodes(c *gin.Context) {
	nodes := h.cluster.GetNodes()
	
	c.JSON(http.StatusOK, gin.H{
		"nodes":     nodes,
		"count":     len(nodes),
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// HandleRebalance 处理集群重平衡
func (h *APIHandlers) HandleRebalance(c *gin.Context) {
	if err := h.coordinator.TriggerRebalance(); err != nil {
		h.sendError(c, http.StatusInternalServerError, "rebalance_error", err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "rebalance completed",
		"node_id":   h.cluster.nodeID,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// HandleGetMetrics 获取详细指标
func (h *APIHandlers) HandleGetMetrics(c *gin.Context) {
	cacheStats := h.node.GetLocalStats()
	migrationStats := h.coordinator.GetMigrationStats()
	clusterStats := h.cluster.GetClusterStatus()

	c.JSON(http.StatusOK, gin.H{
		"node_id":         h.node.GetNodeID(),
		"cache_stats":     cacheStats,
		"migration_stats": migrationStats,
		"cluster_stats":   clusterStats,
		"timestamp":       time.Now().Format(time.RFC3339),
	})
}

// ===== 辅助方法 =====

// sendError 发送错误响应
func (h *APIHandlers) sendError(c *gin.Context, statusCode int, errorType, message string) {
	c.JSON(statusCode, ErrorResponse{
		Error:     errorType,
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// ===== 请求转发方法 =====

// forwardGetRequest 转发GET请求到目标节点
func (h *APIHandlers) forwardGetRequest(targetNodeID, key string) (*CacheResponse, error) {
	// 1. 根据节点ID获取节点地址
	nodeAddress, err := h.getNodeAddress(targetNodeID)
	if err != nil {
		return nil, err
	}

	// 2. 发送内部API请求
	url := fmt.Sprintf("http://%s/internal/cache/%s", nodeAddress, key)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("转发GET请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 3. 解析响应
	var response CacheResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("解析转发响应失败: %v", err)
	}

	return &response, nil
}

// forwardSetRequest 转发SET请求到目标节点
func (h *APIHandlers) forwardSetRequest(targetNodeID, key, value string) (*CacheResponse, error) {
	// 1. 根据节点ID获取节点地址
	nodeAddress, err := h.getNodeAddress(targetNodeID)
	if err != nil {
		return nil, err
	}

	// 2. 构造请求数据
	reqData := CacheRequest{Value: value}
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("序列化请求数据失败: %v", err)
	}

	// 3. 发送内部API请求
	url := fmt.Sprintf("http://%s/internal/cache/%s", nodeAddress, key)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建转发请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("转发SET请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 4. 解析响应
	var response CacheResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("解析转发响应失败: %v", err)
	}

	return &response, nil
}

// getNodeAddress 根据节点ID获取节点地址
func (h *APIHandlers) getNodeAddress(nodeID string) (string, error) {
	// 从集群管理器获取节点信息
	nodes := h.cluster.GetNodes()
	if nodeInfo, exists := nodes[nodeID]; exists {
		return nodeInfo.Address, nil
	}
	return "", fmt.Errorf("节点 %s 不存在", nodeID)
}
