package distributed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// DistributedClient 分布式缓存客户端
// 提供对分布式缓存集群的访问接口
type DistributedClient struct {
	nodes        []string
	httpClient   *http.Client
	currentNode  int
	mu           sync.Mutex
	retryCount   int
	timeout      time.Duration
}

// ClientConfig 客户端配置
type ClientConfig struct {
	Nodes       []string      `yaml:"nodes"`
	Timeout     time.Duration `yaml:"timeout"`
	RetryCount  int           `yaml:"retry_count"`
}

// NewDistributedClient 创建分布式缓存客户端
func NewDistributedClient(config ClientConfig) *DistributedClient {
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.RetryCount == 0 {
		config.RetryCount = 3
	}
	
	return &DistributedClient{
		nodes: config.Nodes,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		retryCount: config.RetryCount,
		timeout:    config.Timeout,
	}
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
		node := dc.getNextNode()
		
		err := operation(node)
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// 如果是网络错误，尝试下一个节点
		if isNetworkError(err) {
			continue
		}
		
		// 如果是业务错误，直接返回
		return err
	}
	
	return fmt.Errorf("所有节点都不可用，最后错误: %v", lastErr)
}

// getNextNode 获取下一个节点（轮询）
func (dc *DistributedClient) getNextNode() string {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	
	node := dc.nodes[dc.currentNode]
	dc.currentNode = (dc.currentNode + 1) % len(dc.nodes)
	
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
