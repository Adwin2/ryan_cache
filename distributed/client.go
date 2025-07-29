package distributed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"
)

// NodeStatus 节点状态
type NodeStatus struct {
	Address         string
	IsHealthy       bool
	FailureCount    int
	LastCheckTime   time.Time
	LastFailureTime time.Time
	LastSuccessTime time.Time
}

// NodeManager 节点管理器
type NodeManager struct {
	seedNodes     []string
	activeNodes   []string
	nodeStatus    map[string]*NodeStatus
	mu            sync.RWMutex

	healthChecker *HealthChecker
	stopChan      chan struct{}
	config        ClientConfig
}

// HealthChecker 健康检查器
type HealthChecker struct {
	httpClient *http.Client
	manager    *NodeManager
}

// DistributedClient 分布式缓存客户端
// 提供对分布式缓存集群的访问接口
type DistributedClient struct {
	nodes        []string
	httpClient   *http.Client
	currentNode  int
	mu           sync.Mutex
	retryCount   int
	timeout      time.Duration

	// 新增：节点管理
	nodeManager  *NodeManager
	config       ClientConfig
}

// ClientConfig 客户端配置
type ClientConfig struct {
	Nodes                 []string      `yaml:"nodes"`
	Timeout               time.Duration `yaml:"timeout"`
	RetryCount            int           `yaml:"retry_count"`

	// 新增：健康检查配置
	HealthCheckEnabled    bool          `yaml:"health_check_enabled"`    // 默认true
	HealthCheckInterval   time.Duration `yaml:"health_check_interval"`   // 默认30s
	FailureThreshold      int           `yaml:"failure_threshold"`       // 默认3次失败
	RecoveryCheckInterval time.Duration `yaml:"recovery_check_interval"` // 默认60s
}

// NewDistributedClient 创建分布式缓存客户端
func NewDistributedClient(config ClientConfig) *DistributedClient {
	// 设置默认值
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.RetryCount == 0 {
		config.RetryCount = 3
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30 * time.Second
	}
	if config.FailureThreshold == 0 {
		config.FailureThreshold = 3
	}
	if config.RecoveryCheckInterval == 0 {
		config.RecoveryCheckInterval = 60 * time.Second
	}
	// 默认启用健康检查
	if !config.HealthCheckEnabled {
		config.HealthCheckEnabled = true
	}

	client := &DistributedClient{
		nodes: config.Nodes,
		httpClient: createClientHTTPClient(config.Timeout),
		retryCount: config.RetryCount,
		timeout:    config.Timeout,
		config:     config,
	}

	// 创建节点管理器
	client.nodeManager = NewNodeManager(config)

	return client
}

// ===== HTTP客户端工厂函数 =====

// createClientHTTPClient 创建客户端HTTP客户端 - 高并发，长连接
func createClientHTTPClient(timeout time.Duration) *http.Client {
    return &http.Client{
        Timeout: timeout,
        Transport: &http.Transport{
            // 高并发配置
            MaxIdleConns:        200,              // 最大空闲连接数
            MaxIdleConnsPerHost: 20,               // 每个host的最大空闲连接数
            IdleConnTimeout:     120 * time.Second, // 长空闲连接超时
            DisableKeepAlives:   false,            // 启用keep-alive
            
            // 连接超时设置
            TLSHandshakeTimeout: 3 * time.Second,  // TLS握手超时

            // 响应超时
            ResponseHeaderTimeout: timeout / 2,    // 响应头超时为总超时的一半
            ExpectContinueTimeout: 1 * time.Second,
            
            // 启用HTTP/2
            ForceAttemptHTTP2: true,
        },
    }
}

// createHealthCheckHTTPClient 创建健康检查HTTP客户端 - 快速检查，短连接
func createHealthCheckHTTPClient() *http.Client {
    return &http.Client{
        Timeout: 3 * time.Second,
        Transport: &http.Transport{
            // 轻量级配置
            MaxIdleConns:        50,               // 较少的空闲连接
            MaxIdleConnsPerHost: 5,                // 每个host少量连接
            IdleConnTimeout:     30 * time.Second, // 短空闲超时
            DisableKeepAlives:   false,            // 仍然启用keep-alive
            
            // 快速连接设置
            TLSHandshakeTimeout: 1 * time.Second,  // 快速TLS握手
            
            // 快速响应
            ResponseHeaderTimeout: 2 * time.Second,
            ExpectContinueTimeout: 500 * time.Millisecond,
            
            // 禁用压缩以提高速度
            DisableCompression: true,
        },
    }
}

// createNodeHTTPClient 创建节点间通信HTTP客户端 - 中等并发，中等连接
func createNodeHTTPClient(timeout time.Duration) *http.Client {
    return &http.Client{
        Timeout: timeout,
        Transport: &http.Transport{
            // 中等并发配置
            MaxIdleConns:        100,              // 中等空闲连接数
            MaxIdleConnsPerHost: 10,               // 每个host中等连接数
            IdleConnTimeout:     60 * time.Second, // 中等空闲超时
            DisableKeepAlives:   false,            // 启用keep-alive
            
            // 平衡的连接设置
            TLSHandshakeTimeout: 2 * time.Second,  // 中等TLS握手超时
            
            // 平衡的响应超时
            ResponseHeaderTimeout: timeout / 3,    // 响应头超时
            ExpectContinueTimeout: 1 * time.Second,
            
            // 启用压缩以节省带宽
            DisableCompression: false,
        },
    }
}

// createCoordinatorHTTPClient 创建集群协调HTTP客户端 - 低频率，长超时
func createCoordinatorHTTPClient(timeout time.Duration) *http.Client {
    return &http.Client{
        Timeout: timeout,
        Transport: &http.Transport{
            // 低频高可靠配置
            MaxIdleConns:        30,               // 少量空闲连接
            MaxIdleConnsPerHost: 3,                // 每个host少量连接
            IdleConnTimeout:     300 * time.Second, // 长空闲超时
            DisableKeepAlives:   false,            // 启用keep-alive
            
            // 可靠的连接设置
            TLSHandshakeTimeout: 5 * time.Second,  // 较长TLS握手超时
            
            // 长响应超时
            ResponseHeaderTimeout: timeout / 2,    // 长响应头超时
            ExpectContinueTimeout: 2 * time.Second,
            
            // 启用压缩
            DisableCompression: false,
        },
    }
}

// Close 关闭客户端，停止健康检查
func (dc *DistributedClient) Close() {
	if dc.nodeManager != nil {
		dc.nodeManager.Stop()
	}
}

// GetNodeStatus 获取所有节点的状态信息
func (dc *DistributedClient) GetNodeStatus() map[string]*NodeStatus {
	if dc.nodeManager == nil {
		return nil
	}

	dc.nodeManager.mu.RLock()
	defer dc.nodeManager.mu.RUnlock()

	// 复制状态信息
	status := make(map[string]*NodeStatus)
	for node, nodeStatus := range dc.nodeManager.nodeStatus {
		status[node] = &NodeStatus{
			Address:         nodeStatus.Address,
			IsHealthy:       nodeStatus.IsHealthy,
			FailureCount:    nodeStatus.FailureCount,
			LastCheckTime:   nodeStatus.LastCheckTime,
			LastFailureTime: nodeStatus.LastFailureTime,
			LastSuccessTime: nodeStatus.LastSuccessTime,
		}
	}

	return status
}

// GetTimeout 获取客户端超时时间
func (dc *DistributedClient) GetTimeout() time.Duration {
	return dc.timeout
}

// Set 设置缓存
func (dc *DistributedClient) Set(key, value string) error {
	req := CacheRequest{Value: value}
	
	return dc.executeWithRetry(func(node string) error {
		return dc.setToNode(node, key, req)
	})
}

// Get 获取缓存
func (dc *DistributedClient) Get(key string) (string, bool, error) {
	var result string
	var found bool
	
	err := dc.executeWithRetry(func(node string) error {
		value, exists, err := dc.getFromNode(node, key)
		if err != nil {
			return err
		}
		result = value
		found = exists
		return nil
	})
	
	return result, found, err
}

// Delete 删除缓存
func (dc *DistributedClient) Delete(key string) error {
	return dc.executeWithRetry(func(node string) error {
		return dc.deleteFromNode(node, key)
	})
}

// GetStats 获取统计信息
func (dc *DistributedClient) GetStats() (map[string]interface{}, error) {
	var stats map[string]interface{}
	
	err := dc.executeWithRetry(func(node string) error {
		result, err := dc.getStatsFromNode(node)
		if err != nil {
			return err
		}
		stats = result
		return nil
	})
	
	return stats, err
}

// GetClusterInfo 获取集群信息
func (dc *DistributedClient) GetClusterInfo() (map[string]interface{}, error) {
	var clusterInfo map[string]interface{}
	
	err := dc.executeWithRetry(func(node string) error {
		result, err := dc.getClusterInfoFromNode(node)
		if err != nil {
			return err
		}
		clusterInfo = result
		return nil
	})
	
	return clusterInfo, err
}

// CheckHealth 检查集群健康状态
func (dc *DistributedClient) CheckHealth() (map[string]bool, error) {
	healthStatus := make(map[string]bool)
	
	for _, node := range dc.nodes {
		healthy := dc.checkNodeHealth(node)
		healthStatus[node] = healthy
	}
	
	return healthStatus, nil
}

// ===== 内部方法 =====

// executeWithRetry 执行操作并重试
func (dc *DistributedClient) executeWithRetry(operation func(string) error) error {
	var lastErr error

	for attempt := 0; attempt < dc.retryCount; attempt++ {
		node := dc.getNextHealthyNode()

		err := operation(node)
		if err == nil {
			// 标记节点成功
			if dc.nodeManager != nil {
				dc.nodeManager.MarkSuccess(node)
			}
			return nil
		}

		lastErr = err

		// 标记节点失败
		if dc.nodeManager != nil {
			dc.nodeManager.MarkFailure(node)
		}

		// 如果是网络错误，尝试下一个节点
		if isNetworkError(err) {
			continue
		}

		// 如果是业务错误，直接返回
		return err
	}

	return fmt.Errorf("所有节点都不可用，最后错误: %v", lastErr)
}

// getNextNode 获取下一个节点（轮询）- 保持向后兼容
func (dc *DistributedClient) getNextNode() string {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	node := dc.nodes[dc.currentNode]
	dc.currentNode = (dc.currentNode + 1) % len(dc.nodes)

	return node
}

// getNextHealthyNode 获取下一个健康节点（智能选择）
func (dc *DistributedClient) getNextHealthyNode() string {
	// 如果没有节点管理器，使用传统轮询
	if dc.nodeManager == nil {
		return dc.getNextNode()
	}

	// 获取健康节点列表
	healthyNodes := dc.nodeManager.GetHealthyNodes()
	if len(healthyNodes) == 0 {
		// 降级：使用原始节点列表
		return dc.getNextNode()
	}

	// 在健康节点中轮询
	dc.mu.Lock()
	defer dc.mu.Unlock()

	node := healthyNodes[dc.currentNode % len(healthyNodes)]
	dc.currentNode = (dc.currentNode + 1) % len(healthyNodes)

	return node
}

// setToNode 向指定节点设置缓存
func (dc *DistributedClient) setToNode(node, key string, req CacheRequest) error {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %v", err)
	}
	
	url := fmt.Sprintf("http://%s/api/v1/cache/%s", node, key)
	httpReq, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	
	resp, err := dc.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("设置失败: %s", string(body))
	}
	
	return nil
}

// getFromNode 从指定节点获取缓存
func (dc *DistributedClient) getFromNode(node, key string) (string, bool, error) {
	url := fmt.Sprintf("http://%s/api/v1/cache/%s", node, key)
	
	resp, err := dc.httpClient.Get(url)
	if err != nil {
		return "", false, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", false, fmt.Errorf("获取失败: %s", string(body))
	}
	
	var response CacheResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", false, fmt.Errorf("解析响应失败: %v", err)
	}
	
	return response.Value, response.Found, nil
}

// deleteFromNode 从指定节点删除缓存
func (dc *DistributedClient) deleteFromNode(node, key string) error {
	url := fmt.Sprintf("http://%s/api/v1/cache/%s", node, key)
	
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}
	
	resp, err := dc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("删除失败: %s", string(body))
	}
	
	return nil
}

// getStatsFromNode 从指定节点获取统计信息
func (dc *DistributedClient) getStatsFromNode(node string) (map[string]interface{}, error) {
	url := fmt.Sprintf("http://%s/api/v1/stats", node)
	
	resp, err := dc.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取统计失败: %s", string(body))
	}
	
	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}
	
	return stats, nil
}

// getClusterInfoFromNode 从指定节点获取集群信息
func (dc *DistributedClient) getClusterInfoFromNode(node string) (map[string]interface{}, error) {
	url := fmt.Sprintf("http://%s/admin/cluster", node)
	
	resp, err := dc.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("获取集群信息失败: %s", string(body))
	}
	
	var clusterInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&clusterInfo); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}
	
	return clusterInfo, nil
}

// checkNodeHealth 检查节点健康状态
func (dc *DistributedClient) checkNodeHealth(node string) bool {
	url := fmt.Sprintf("http://%s/api/v1/health", node)
	
	resp, err := dc.httpClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	return resp.StatusCode == http.StatusOK
}

// isNetworkError 判断是否为网络错误
func isNetworkError(err error) bool {
	// 简单的网络错误判断
	// 在实际项目中可以更精确地判断错误类型
	return err != nil
}

// ===== 批量操作 =====

// BatchSet 批量设置缓存
func (dc *DistributedClient) BatchSet(data map[string]string) error {
	for key, value := range data {
		if err := dc.Set(key, value); err != nil {
			return fmt.Errorf("批量设置失败，键 %s: %v", key, err)
		}
	}
	return nil
}

// BatchGet 批量获取缓存
func (dc *DistributedClient) BatchGet(keys []string) (map[string]string, error) {
	result := make(map[string]string)
	
	for _, key := range keys {
		value, found, err := dc.Get(key)
		if err != nil {
			return nil, fmt.Errorf("批量获取失败，键 %s: %v", key, err)
		}
		if found {
			result[key] = value
		}
	}
	
	return result, nil
}

// BatchDelete 批量删除缓存
func (dc *DistributedClient) BatchDelete(keys []string) error {
	for _, key := range keys {
		if err := dc.Delete(key); err != nil {
			return fmt.Errorf("批量删除失败，键 %s: %v", key, err)
		}
	}
	return nil
}

// ===== NodeManager 实现 =====

// NewNodeManager 创建节点管理器
func NewNodeManager(config ClientConfig) *NodeManager {
	manager := &NodeManager{
		seedNodes:  make([]string, len(config.Nodes)),
		nodeStatus: make(map[string]*NodeStatus),
		stopChan:   make(chan struct{}),
		config:     config,
	}

	// 复制种子节点列表
	copy(manager.seedNodes, config.Nodes)

	// 初始化所有节点状态
	for _, node := range config.Nodes {
		manager.nodeStatus[node] = &NodeStatus{
			Address:         node,
			IsHealthy:       true, // 初始假设所有节点健康
			FailureCount:    0,
			LastCheckTime:   time.Now(),
			LastSuccessTime: time.Now(),
		}
	}

	// 初始化活跃节点列表
	manager.activeNodes = make([]string, len(config.Nodes))
	copy(manager.activeNodes, config.Nodes)

	// 创建健康检查器
	if config.HealthCheckEnabled {
		manager.healthChecker = &HealthChecker{
			httpClient: createHealthCheckHTTPClient(),
			manager:    manager,
		}

		// 启动健康检查协程
		go manager.startHealthCheckRoutine()

		// 立即执行一次健康检查
		go manager.performHealthCheck()
	}

	return manager
}

// GetHealthyNodes 获取健康的节点列表
func (nm *NodeManager) GetHealthyNodes() []string {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	var healthyNodes []string
	for _, node := range nm.activeNodes {
		if status, exists := nm.nodeStatus[node]; exists && status.IsHealthy {
			healthyNodes = append(healthyNodes, node)
		}
	}

	// 如果没有健康节点，返回所有节点（降级处理）
	if len(healthyNodes) == 0 {
		return nm.seedNodes
	}

	return healthyNodes
}

// MarkSuccess 标记节点成功
func (nm *NodeManager) MarkSuccess(node string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if status, exists := nm.nodeStatus[node]; exists {
		status.IsHealthy = true
		status.FailureCount = 0
		status.LastSuccessTime = time.Now()

		// 如果节点之前不在活跃列表中，重新添加
		nm.ensureNodeInActiveList(node)
	}
}

// MarkFailure 标记节点失败
func (nm *NodeManager) MarkFailure(node string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if status, exists := nm.nodeStatus[node]; exists {
		status.FailureCount++
		status.LastFailureTime = time.Now()

		// 如果失败次数超过阈值，标记为不健康
		if status.FailureCount >= nm.config.FailureThreshold {
			status.IsHealthy = false
		}
	}
}

// ensureNodeInActiveList 确保节点在活跃列表中
func (nm *NodeManager) ensureNodeInActiveList(node string) {
	if slices.Contains(nm.activeNodes, node) {
			return // 已经在列表中
	}
	// 不在列表中，添加
	nm.activeNodes = append(nm.activeNodes, node)
}

// startHealthCheckRoutine 启动健康检查协程
func (nm *NodeManager) startHealthCheckRoutine() {
	ticker := time.NewTicker(nm.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			nm.performHealthCheck()
		case <-nm.stopChan:
			return
		}
	}
}

// performHealthCheck 执行健康检查
func (nm *NodeManager) performHealthCheck() {
	nm.mu.RLock()
	nodes := make([]string, len(nm.seedNodes))
	copy(nodes, nm.seedNodes)
	nm.mu.RUnlock()

	for _, node := range nodes {
		go nm.checkSingleNode(node)
	}
}

// checkSingleNode 检查单个节点
func (nm *NodeManager) checkSingleNode(node string) {
	healthy := nm.healthChecker.CheckNode(node)

	nm.mu.Lock()
	defer nm.mu.Unlock()

	if status, exists := nm.nodeStatus[node]; exists {
		status.LastCheckTime = time.Now()

		if healthy {
			if !status.IsHealthy {
				// 节点恢复了
				status.IsHealthy = true
				status.FailureCount = 0
				status.LastSuccessTime = time.Now()
				nm.ensureNodeInActiveList(node)
			}
		} else {
			status.FailureCount++
			status.LastFailureTime = time.Now()
			if status.FailureCount >= nm.config.FailureThreshold {
				status.IsHealthy = false
			}
		}
	}
}

// Stop 停止节点管理器
func (nm *NodeManager) Stop() {
	close(nm.stopChan)
}

// ===== HealthChecker 实现 =====

// CheckNode 检查单个节点的健康状态
func (hc *HealthChecker) CheckNode(node string) bool {
	url := fmt.Sprintf("http://%s/api/v1/health", node)

	resp, err := hc.httpClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// 200状态码表示健康
	return resp.StatusCode == http.StatusOK
}
