package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"tdd-learning/distributed"
	"testing"
	"time"
)

// TestClientServerIntegration 测试客户端与服务端的集成
func TestClientServerIntegration(t *testing.T) {
	// 跳过集成测试，除非明确要求
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 启动测试集群
	cluster := startTestCluster(t)
	defer cluster.Stop()

	// 等待集群启动
	time.Sleep(5 * time.Second)

	// 配置客户端
	config := distributed.ClientConfig{
		Nodes: []string{
			"localhost:8001",
			"localhost:8002", 
			"localhost:8003",
		},
		Timeout:               5 * time.Second,
		RetryCount:            3,
		HealthCheckEnabled:    true,
		HealthCheckInterval:   2 * time.Second,
		FailureThreshold:      2,
		RecoveryCheckInterval: 3 * time.Second,
	}

	client := distributed.NewDistributedClient(config)
	defer client.Close()

	// 等待健康检查
	time.Sleep(3 * time.Second)

	// 验证所有节点都是健康的
	nodeStatus := client.GetNodeStatus()
	healthyCount := 0
	for node, status := range nodeStatus {
		t.Logf("节点 %s: 健康=%v, 失败次数=%d", node, status.IsHealthy, status.FailureCount)
		if status.IsHealthy {
			healthyCount++
		}
	}

	if healthyCount != 3 {
		t.Errorf("期望3个健康节点，实际%d个", healthyCount)
	}

	// 测试缓存操作
	err := client.Set("test-key", "test-value")
	if err != nil {
		t.Fatalf("设置缓存失败: %v", err)
	}

	value, found, err := client.Get("test-key")
	if err != nil {
		t.Fatalf("获取缓存失败: %v", err)
	}

	if !found {
		t.Error("缓存键应该存在")
	}

	if value != "test-value" {
		t.Errorf("期望值 'test-value'，实际 '%s'", value)
	}

	t.Log("客户端服务端集成测试通过")
}

// TestClientNodeFailureRecovery 测试节点故障和恢复
func TestClientNodeFailureRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 启动测试集群
	cluster := startTestCluster(t)
	defer cluster.Stop()

	time.Sleep(5 * time.Second)

	// 配置客户端
	config := distributed.ClientConfig{
		Nodes: []string{
			"localhost:8001",
			"localhost:8002",
			"localhost:8003",
		},
		Timeout:               3 * time.Second,
		RetryCount:            3,
		HealthCheckEnabled:    true,
		HealthCheckInterval:   1 * time.Second,
		FailureThreshold:      2,
		RecoveryCheckInterval: 2 * time.Second,
	}

	client := distributed.NewDistributedClient(config)
	defer client.Close()

	// 等待健康检查
	time.Sleep(3 * time.Second)

	// 停止一个节点
	t.Log("停止节点2...")
	cluster.StopNode(2)

	// 等待客户端检测到节点故障
	time.Sleep(5 * time.Second)

	// 验证节点2被标记为不健康
	nodeStatus := client.GetNodeStatus()
	if nodeStatus["localhost:8002"].IsHealthy {
		t.Error("节点2应该被标记为不健康")
	}

	// 验证客户端仍然可以正常工作
	err := client.Set("test-key-2", "test-value-2")
	if err != nil {
		t.Fatalf("节点故障后设置缓存失败: %v", err)
	}

	value, found, err := client.Get("test-key-2")
	if err != nil {
		t.Fatalf("节点故障后获取缓存失败: %v", err)
	}

	if !found || value != "test-value-2" {
		t.Error("节点故障后缓存操作失败")
	}

	// 重启节点2
	t.Log("重启节点2...")
	cluster.StartNode(2)

	// 等待节点恢复
	time.Sleep(8 * time.Second)

	// 验证节点2恢复健康
	nodeStatus = client.GetNodeStatus()
	if !nodeStatus["localhost:8002"].IsHealthy {
		t.Error("节点2应该恢复为健康状态")
	}

	t.Log("节点故障恢复测试通过")
}

// TestCluster 测试集群管理
type TestCluster struct {
	nodes []*exec.Cmd
	pids  []int
}

// startTestCluster 启动测试集群
func startTestCluster(t *testing.T) *TestCluster {
	cluster := &TestCluster{
		nodes: make([]*exec.Cmd, 3),
		pids:  make([]int, 3),
	}

	// 构建节点程序
	buildCmd := exec.Command("go", "build", "-o", "bin/test-cache-node", "cmd/node/main.go")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("构建节点程序失败: %v", err)
	}

	// 启动3个节点
	configs := []string{"config/node1.yaml", "config/node2.yaml", "config/node3.yaml"}
	
	for i, config := range configs {
		cmd := exec.Command("./bin/test-cache-node", "-config="+config)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Start(); err != nil {
			t.Fatalf("启动节点%d失败: %v", i+1, err)
		}
		
		cluster.nodes[i] = cmd
		cluster.pids[i] = cmd.Process.Pid
		t.Logf("启动节点%d，PID: %d", i+1, cmd.Process.Pid)
	}

	return cluster
}

// StopNode 停止指定节点
func (tc *TestCluster) StopNode(nodeIndex int) {
	if nodeIndex < 1 || nodeIndex > 3 {
		return
	}
	
	idx := nodeIndex - 1
	if tc.nodes[idx] != nil && tc.nodes[idx].Process != nil {
		tc.nodes[idx].Process.Signal(syscall.SIGTERM)
		tc.nodes[idx].Wait()
		tc.nodes[idx] = nil
	}
}

// StartNode 启动指定节点
func (tc *TestCluster) StartNode(nodeIndex int) {
	if nodeIndex < 1 || nodeIndex > 3 {
		return
	}
	
	idx := nodeIndex - 1
	configs := []string{"config/node1.yaml", "config/node2.yaml", "config/node3.yaml"}
	
	cmd := exec.Command("./bin/test-cache-node", "-config="+configs[idx])
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Start(); err != nil {
		return
	}
	
	tc.nodes[idx] = cmd
	tc.pids[idx] = cmd.Process.Pid
}

// Stop 停止所有节点
func (tc *TestCluster) Stop() {
	for _, cmd := range tc.nodes {
		if cmd != nil && cmd.Process != nil {
			cmd.Process.Signal(syscall.SIGTERM)
			cmd.Wait()
		}
	}
	
	// 清理构建的二进制文件
	os.Remove("bin/test-cache-node")
}

// TestClientLoadBalancingWithRealServers 使用真实服务器测试负载均衡
func TestClientLoadBalancingWithRealServers(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	cluster := startTestCluster(t)
	defer cluster.Stop()

	time.Sleep(5 * time.Second)

	config := distributed.ClientConfig{
		Nodes: []string{
			"localhost:8001",
			"localhost:8002",
			"localhost:8003",
		},
		Timeout:               5 * time.Second,
		RetryCount:            3,
		HealthCheckEnabled:    true,
		HealthCheckInterval:   2 * time.Second,
		FailureThreshold:      2,
	}

	client := distributed.NewDistributedClient(config)
	defer client.Close()

	time.Sleep(3 * time.Second)

	// 发送多个请求，验证负载均衡
	requestCount := 30
	for i := 0; i < requestCount; i++ {
		key := fmt.Sprintf("load-test-key-%d", i)
		value := fmt.Sprintf("load-test-value-%d", i)
		
		err := client.Set(key, value)
		if err != nil {
			t.Errorf("设置缓存失败 %s: %v", key, err)
			continue
		}
		
		retrievedValue, found, err := client.Get(key)
		if err != nil {
			t.Errorf("获取缓存失败 %s: %v", key, err)
			continue
		}
		
		if !found || retrievedValue != value {
			t.Errorf("缓存值不匹配 %s: 期望=%s, 实际=%s, 找到=%v", key, value, retrievedValue, found)
		}
	}

	// 检查所有节点的统计信息
	for i, node := range config.Nodes {
		stats, err := getNodeStats(node)
		if err != nil {
			t.Errorf("获取节点%d统计失败: %v", i+1, err)
			continue
		}
		t.Logf("节点%d (%s) 统计: %+v", i+1, node, stats)
	}

	t.Log("真实服务器负载均衡测试通过")
}

// getNodeStats 获取节点统计信息
func getNodeStats(node string) (map[string]interface{}, error) {
	resp, err := http.Get(fmt.Sprintf("http://%s/api/v1/stats", node))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}

	return stats, nil
}
