package tests

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"tdd-learning/distributed"
	"testing"
	"time"
)

// TestClientHealthCheck 测试客户端健康检查功能
func TestClientHealthCheck(t *testing.T) {
	// 创建模拟服务器
	healthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer healthyServer.Close()

	unhealthyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/health" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Error"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer unhealthyServer.Close()

	// 配置客户端
	config := distributed.ClientConfig{
		Nodes: []string{
			healthyServer.URL[7:], // 移除 "http://" 前缀
			unhealthyServer.URL[7:],
		},
		Timeout:               5 * time.Second,
		RetryCount:            3,
		HealthCheckEnabled:    true,
		HealthCheckInterval:   500 * time.Millisecond, // 快速检查用于测试
		FailureThreshold:      1, // 降低阈值，更快检测到故障
		RecoveryCheckInterval: 1 * time.Second,
	}

	client := distributed.NewDistributedClient(config)
	defer client.Close()

	// 等待健康检查运行
	time.Sleep(2 * time.Second)

	// 检查节点状态
	nodeStatus := client.GetNodeStatus()
	if nodeStatus == nil {
		t.Fatal("节点状态为空")
	}

	// 验证健康节点
	healthyFound := false
	unhealthyFound := false
	
	for node, status := range nodeStatus {
		t.Logf("节点 %s: 健康=%v, 失败次数=%d", node, status.IsHealthy, status.FailureCount)
		
		if node == healthyServer.URL[7:] && status.IsHealthy {
			healthyFound = true
		}
		if node == unhealthyServer.URL[7:] && !status.IsHealthy {
			unhealthyFound = true
		}
	}

	if !healthyFound {
		t.Error("健康节点应该被标记为健康")
	}
	if !unhealthyFound {
		t.Error("不健康节点应该被标记为不健康")
	}
}

// TestClientFailover 测试客户端故障转移
func TestClientFailover(t *testing.T) {
	var server1Requests, server2Requests int
	var mu sync.Mutex

	// 创建两个服务器
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		server1Requests++
		mu.Unlock()
		
		if r.URL.Path == "/api/v1/health" {
			w.WriteHeader(http.StatusOK)
		} else if r.URL.Path == "/api/v1/cache/test" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"key":"test","value":"value1","found":true}`))
		}
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		server2Requests++
		mu.Unlock()
		
		if r.URL.Path == "/api/v1/health" {
			w.WriteHeader(http.StatusOK)
		} else if r.URL.Path == "/api/v1/cache/test" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"key":"test","value":"value2","found":true}`))
		}
	}))
	defer server2.Close()

	// 配置客户端
	config := distributed.ClientConfig{
		Nodes: []string{
			server1.URL[7:],
			server2.URL[7:],
		},
		Timeout:               5 * time.Second,
		RetryCount:            3,
		HealthCheckEnabled:    true,
		HealthCheckInterval:   1 * time.Second,
		FailureThreshold:      1,
		RecoveryCheckInterval: 2 * time.Second,
	}

	client := distributed.NewDistributedClient(config)
	defer client.Close()

	// 等待健康检查
	time.Sleep(2 * time.Second)

	// 发送多个请求，验证负载均衡
	for i := 0; i < 10; i++ {
		_, _, err := client.Get("test")
		if err != nil {
			t.Errorf("请求失败: %v", err)
		}
	}

	mu.Lock()
	total := server1Requests + server2Requests
	mu.Unlock()

	t.Logf("服务器1请求数: %d, 服务器2请求数: %d", server1Requests, server2Requests)

	// 验证两个服务器都收到了请求（负载均衡）
	if server1Requests == 0 || server2Requests == 0 {
		t.Error("负载均衡失败，某个服务器没有收到请求")
	}

	if total < 10 {
		t.Errorf("总请求数不正确，期望至少10，实际%d", total)
	}
}

// TestClientNodeRecovery 测试节点恢复功能
func TestClientNodeRecovery(t *testing.T) {
	var serverEnabled bool = false
	var mu sync.Mutex

	// 创建一个可以动态启用/禁用的服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		enabled := serverEnabled
		mu.Unlock()

		if !enabled {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if r.URL.Path == "/api/v1/health" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"key":"test","value":"value","found":true}`))
		}
	}))
	defer server.Close()

	// 配置客户端
	config := distributed.ClientConfig{
		Nodes: []string{
			server.URL[7:],
		},
		Timeout:               2 * time.Second,
		RetryCount:            3,
		HealthCheckEnabled:    true,
		HealthCheckInterval:   500 * time.Millisecond,
		FailureThreshold:      1, // 降低阈值
		RecoveryCheckInterval: 500 * time.Millisecond,
	}

	client := distributed.NewDistributedClient(config)
	defer client.Close()

	// 等待健康检查运行几次，检测到服务器不健康
	time.Sleep(2 * time.Second)

	// 验证节点被标记为不健康
	nodeStatus := client.GetNodeStatus()
	if nodeStatus[server.URL[7:]].IsHealthy {
		t.Errorf("节点应该被检测为不健康状态，当前状态: 健康=%v, 失败次数=%d",
			nodeStatus[server.URL[7:]].IsHealthy,
			nodeStatus[server.URL[7:]].FailureCount)
	}

	// 启用服务器
	mu.Lock()
	serverEnabled = true
	mu.Unlock()

	// 等待节点恢复
	time.Sleep(3 * time.Second)

	// 验证节点恢复为健康状态
	nodeStatus = client.GetNodeStatus()
	if !nodeStatus[server.URL[7:]].IsHealthy {
		t.Error("节点应该恢复为健康状态")
	}

	t.Log("节点恢复测试通过")
}

// TestClientConfiguration 测试客户端配置
func TestClientConfiguration(t *testing.T) {
	// 测试默认配置
	config := distributed.ClientConfig{
		Nodes: []string{"localhost:8001", "localhost:8002"},
	}

	client := distributed.NewDistributedClient(config)
	defer client.Close()

	// 验证默认值
	if client.GetTimeout() != 5*time.Second {
		t.Error("默认超时时间应该是5秒")
	}

	// 测试自定义配置
	customConfig := distributed.ClientConfig{
		Nodes:                 []string{"localhost:8001"},
		Timeout:               10 * time.Second,
		RetryCount:            5,
		HealthCheckEnabled:    false,
		HealthCheckInterval:   60 * time.Second,
		FailureThreshold:      5,
		RecoveryCheckInterval: 120 * time.Second,
	}

	customClient := distributed.NewDistributedClient(customConfig)
	defer customClient.Close()

	if customClient.GetTimeout() != 10*time.Second {
		t.Error("自定义超时时间设置失败")
	}
}

// BenchmarkClientLoadBalancing 负载均衡性能测试
func BenchmarkClientLoadBalancing(b *testing.B) {
	// 创建多个模拟服务器
	servers := make([]*httptest.Server, 3)
	nodes := make([]string, 3)

	for i := 0; i < 3; i++ {
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v1/health" {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"key":"test","value":"value","found":true}`))
			}
		}))
		nodes[i] = servers[i].URL[7:]
	}

	defer func() {
		for _, server := range servers {
			server.Close()
		}
	}()

	config := distributed.ClientConfig{
		Nodes:              nodes,
		Timeout:            5 * time.Second,
		RetryCount:         3,
		HealthCheckEnabled: true,
		HealthCheckInterval: 30 * time.Second,
		FailureThreshold:   3,
	}

	client := distributed.NewDistributedClient(config)
	defer client.Close()

	// 等待健康检查
	time.Sleep(1 * time.Second)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, err := client.Get("test")
			if err != nil {
				b.Errorf("请求失败: %v", err)
			}
		}
	})
}
