package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"tdd-learning/distributed"

	"gopkg.in/yaml.v3"
)

// MigrationTester 数据迁移测试器
type MigrationTester struct {
	initialNodes []NodeInfo
	newNode      NodeInfo
	testData     map[string]string
	httpClient   *http.Client
}

// NodeInfo 节点信息
type NodeInfo struct {
	NodeID  string `json:"node_id"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	PID     int    `json:"pid,omitempty"`
}

// TestResult 测试结果
type TestResult struct {
	Success        bool                   `json:"success"`
	Message        string                 `json:"message"`
	DataIntegrity  float64                `json:"data_integrity"`
	MigrationStats map[string]interface{} `json:"migration_stats"`
	Errors         []string               `json:"errors"`
}

func main() {
	fmt.Println("🧪 分布式缓存数据迁移测试")
	fmt.Println("===========================")

	tester := NewMigrationTester()
	
	// 运行完整的数据迁移测试
	result := tester.RunFullMigrationTest()
	
	// 输出测试结果
	tester.PrintTestResult(result)
	
	if !result.Success {
		os.Exit(1)
	}
}

// NewMigrationTester 创建数据迁移测试器
func NewMigrationTester() *MigrationTester {
	return &MigrationTester{
		initialNodes: []NodeInfo{
			{NodeID: "node1", Address: "localhost:8001", Port: 8001},
			{NodeID: "node2", Address: "localhost:8002", Port: 8002},
		},
		newNode: NodeInfo{
			NodeID: "node4", Address: "localhost:8004", Port: 8004,
		},
		testData: map[string]string{
			"user:1001":    "张三",
			"user:1002":    "李四", 
			"user:1003":    "王五",
			"user:1004":    "赵六",
			"user:1005":    "钱七",
			"product:2001": "iPhone15",
			"product:2002": "MacBook",
			"product:2003": "iPad",
			"product:2004": "AirPods",
			"product:2005": "AppleWatch",
			"order:3001":   "订单1",
			"order:3002":   "订单2",
			"order:3003":   "订单3",
			"session:abc":  "会话1",
			"session:def":  "会话2",
			"cache:key1":   "缓存值1",
			"cache:key2":   "缓存值2",
			"cache:key3":   "缓存值3",
		},
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// RunFullMigrationTest 运行完整的数据迁移测试
func (mt *MigrationTester) RunFullMigrationTest() *TestResult {
	result := &TestResult{
		Success: true,
		Errors:  make([]string, 0),
	}

	fmt.Println("\n🚀 步骤1: 启动初始集群（2个节点）")
	if err := mt.startInitialCluster(); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("启动初始集群失败: %v", err))
		return result
	}

	fmt.Println("\n📝 步骤2: 设置测试数据")
	if err := mt.setupTestData(); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("设置测试数据失败: %v", err))
		return result
	}

	fmt.Println("\n📊 步骤3: 记录初始数据分布")
	initialDistribution, err := mt.captureDataDistribution()
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("记录初始数据分布失败: %v", err))
		return result
	}
	mt.printDataDistribution("初始数据分布", initialDistribution)

	fmt.Println("\n🆕 步骤4: 创建并启动新节点")
	if err := mt.createAndStartNewNode(); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("启动新节点失败: %v", err))
		return result
	}

	fmt.Println("\n🔗 步骤5: 将新节点添加到集群")
	if err := mt.addNodeToCluster(); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("添加节点到集群失败: %v", err))
		return result
	}

	fmt.Println("\n⏳ 步骤6: 等待数据迁移完成")
	time.Sleep(5 * time.Second)

	fmt.Println("\n📊 步骤7: 记录迁移后数据分布")
	finalDistribution, err := mt.captureDataDistribution()
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("记录最终数据分布失败: %v", err))
		return result
	}
	mt.printDataDistribution("迁移后数据分布", finalDistribution)

	fmt.Println("\n🔍 步骤8: 验证数据完整性")
	integrity, err := mt.verifyDataIntegrity()
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("验证数据完整性失败: %v", err))
		return result
	}
	result.DataIntegrity = integrity

	fmt.Println("\n📈 步骤9: 收集迁移统计信息")
	migrationStats, err := mt.collectMigrationStats()
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Sprintf("收集迁移统计失败: %v", err))
		return result
	}
	result.MigrationStats = migrationStats

	fmt.Println("\n🧹 步骤10: 清理测试环境")
	mt.cleanup()

	if result.DataIntegrity >= 100.0 {
		result.Message = "数据迁移测试完全成功！"
	} else if result.DataIntegrity >= 95.0 {
		result.Message = "数据迁移测试基本成功，但有少量数据丢失"
	} else {
		result.Success = false
		result.Message = "数据迁移测试失败，数据完整性不足"
	}

	return result
}

// startInitialCluster 启动初始集群
func (mt *MigrationTester) startInitialCluster() error {
	// 停止可能存在的进程
	mt.stopExistingProcesses()

	// 构建节点程序
	if err := mt.buildNodeProgram(); err != nil {
		return fmt.Errorf("构建节点程序失败: %v", err)
	}

	// 启动节点1和节点2
	for _, node := range mt.initialNodes {
		if err := mt.startNode(node); err != nil {
			return fmt.Errorf("启动节点 %s 失败: %v", node.NodeID, err)
		}
	}

	// 等待节点就绪
	return mt.waitForNodesReady(mt.initialNodes)
}

// buildNodeProgram 构建节点程序
func (mt *MigrationTester) buildNodeProgram() error {
	fmt.Println("  🔨 构建节点程序...")
	cmd := exec.Command("go", "build", "-o", "bin/cache-node", "cmd/node/main.go")
	cmd.Dir = "."
	return cmd.Run()
}

// startNode 启动单个节点
func (mt *MigrationTester) startNode(node NodeInfo) error {
	fmt.Printf("  🟢 启动节点 %s (端口 %d)...\n", node.NodeID, node.Port)

	// 对于初始节点使用测试配置文件
	var configFile string
	if node.NodeID == "node1" || node.NodeID == "node2" {
		configFile = fmt.Sprintf("config/%s_test.yaml", node.NodeID)
	} else {
		configFile = fmt.Sprintf("config/%s.yaml", node.NodeID)
	}
	logFile := fmt.Sprintf("logs/%s.log", node.NodeID)
	
	// 确保日志目录存在
	os.MkdirAll("logs", 0755)
	
	cmd := exec.Command("./bin/cache-node", "-config="+configFile)
	cmd.Dir = "."
	
	// 重定向输出到日志文件
	logFileHandle, err := os.Create(logFile)
	if err != nil {
		return err
	}
	cmd.Stdout = logFileHandle
	cmd.Stderr = logFileHandle
	
	if err := cmd.Start(); err != nil {
		return err
	}
	
	// 保存PID
	pidFile := fmt.Sprintf("logs/%s.pid", node.NodeID)
	return os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
}

// waitForNodesReady 等待节点就绪
func (mt *MigrationTester) waitForNodesReady(nodes []NodeInfo) error {
	fmt.Println("  ⏳ 等待节点就绪...")
	
	maxAttempts := 30
	for attempt := 0; attempt < maxAttempts; attempt++ {
		allReady := true
		for _, node := range nodes {
			if !mt.isNodeHealthy(node.Port) {
				allReady = false
				break
			}
		}
		
		if allReady {
			fmt.Println("  ✅ 所有节点就绪")
			return nil
		}
		
		time.Sleep(1 * time.Second)
	}
	
	return fmt.Errorf("节点启动超时")
}

// isNodeHealthy 检查节点是否健康
func (mt *MigrationTester) isNodeHealthy(port int) bool {
	url := fmt.Sprintf("http://localhost:%d/api/v1/health", port)
	resp, err := mt.httpClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// setupTestData 设置测试数据
func (mt *MigrationTester) setupTestData() error {
	fmt.Printf("  📝 设置 %d 个测试数据项...\n", len(mt.testData))

	for key, value := range mt.testData {
		if err := mt.setCache(key, value); err != nil {
			return fmt.Errorf("设置数据 %s 失败: %v", key, err)
		}
		fmt.Printf("    ✅ %s = %s\n", key, value)
	}

	fmt.Println("  ✅ 测试数据设置完成")
	return nil
}

// setCache 设置缓存数据
func (mt *MigrationTester) setCache(key, value string) error {
	url := fmt.Sprintf("http://localhost:%d/api/v1/cache/%s", mt.initialNodes[0].Port, key)

	requestBody := map[string]string{"value": value}
	jsonData, _ := json.Marshal(requestBody)

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := mt.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}

	return nil
}

// captureDataDistribution 捕获数据分布
func (mt *MigrationTester) captureDataDistribution() (map[string]interface{}, error) {
	distribution := make(map[string]interface{})

	// 获取所有活跃节点的统计信息
	allNodes := append(mt.initialNodes, mt.newNode)

	for _, node := range allNodes {
		if !mt.isNodeHealthy(node.Port) {
			continue // 跳过不健康的节点
		}

		url := fmt.Sprintf("http://localhost:%d/api/v1/stats", node.Port)
		resp, err := mt.httpClient.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		var stats map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			continue
		}

		distribution[node.NodeID] = stats
	}

	return distribution, nil
}

// printDataDistribution 打印数据分布
func (mt *MigrationTester) printDataDistribution(title string, distribution map[string]interface{}) {
	fmt.Printf("  📊 %s:\n", title)

	for nodeID, stats := range distribution {
		if statsMap, ok := stats.(map[string]interface{}); ok {
			if cacheStats, exists := statsMap["cache_stats"]; exists {
				if cacheStatsMap, ok := cacheStats.(map[string]interface{}); ok {
					size := cacheStatsMap["total_Size"]
					fmt.Printf("    %s: %v 个数据项\n", nodeID, size)
				}
			}
		}
	}
}

// createAndStartNewNode 创建并启动新节点
func (mt *MigrationTester) createAndStartNewNode() error {
	// 创建新节点的配置文件
	if err := mt.createNodeConfig(); err != nil {
		return fmt.Errorf("创建节点配置失败: %v", err)
	}

	// 启动新节点
	if err := mt.startNode(mt.newNode); err != nil {
		return fmt.Errorf("启动新节点失败: %v", err)
	}

	// 等待新节点就绪
	return mt.waitForNodesReady([]NodeInfo{mt.newNode})
}

// createNodeConfig 创建节点配置文件
func (mt *MigrationTester) createNodeConfig() error {
	config := distributed.NodeConfig{
		NodeID:  mt.newNode.NodeID,
		Address: fmt.Sprintf(":%d", mt.newNode.Port),
		ClusterNodes: map[string]string{
			"node1": "localhost:8001",
			"node2": "localhost:8002",
			"node4": "localhost:8004",
		},
		CacheSize:    1000,
		VirtualNodes: 150,
	}

	configData, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	configFile := fmt.Sprintf("config/%s.yaml", mt.newNode.NodeID)
	configDir := filepath.Dir(configFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	return os.WriteFile(configFile, configData, 0644)
}

// addNodeToCluster 将新节点添加到集群
func (mt *MigrationTester) addNodeToCluster() error {
	fmt.Printf("  🔗 将节点 %s 添加到集群...\n", mt.newNode.NodeID)

	// 向现有节点发送添加节点请求
	url := fmt.Sprintf("http://localhost:%d/internal/cluster/sync-add", mt.initialNodes[0].Port)

	requestBody := map[string]string{
		"node_id": mt.newNode.NodeID,
		"address": mt.newNode.Address,
	}
	jsonData, _ := json.Marshal(requestBody)

	resp, err := mt.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	fmt.Println("  ✅ 节点添加成功")
	return nil
}

// verifyDataIntegrity 验证数据完整性
func (mt *MigrationTester) verifyDataIntegrity() (float64, error) {
	fmt.Println("  🔍 验证所有测试数据是否完整...")

	successCount := 0
	totalCount := len(mt.testData)

	for key, expectedValue := range mt.testData {
		// 从任意节点获取数据（测试路由）
		found, actualValue, err := mt.getCache(key)
		if err != nil {
			fmt.Printf("    ❌ %s: 获取失败 - %v\n", key, err)
			continue
		}

		if !found {
			fmt.Printf("    ❌ %s: 数据未找到\n", key)
			continue
		}

		if actualValue != expectedValue {
			fmt.Printf("    ❌ %s: 值不匹配 (期望=%s, 实际=%s)\n", key, expectedValue, actualValue)
			continue
		}

		fmt.Printf("    ✅ %s = %s\n", key, actualValue)
		successCount++
	}

	integrity := float64(successCount) / float64(totalCount) * 100.0
	fmt.Printf("  📊 数据完整性: %d/%d (%.1f%%)\n", successCount, totalCount, integrity)

	return integrity, nil
}

// getCache 获取缓存数据
func (mt *MigrationTester) getCache(key string) (bool, string, error) {
	// 随机选择一个健康的节点
	allNodes := append(mt.initialNodes, mt.newNode)

	for _, node := range allNodes {
		if !mt.isNodeHealthy(node.Port) {
			continue
		}

		url := fmt.Sprintf("http://localhost:%d/api/v1/cache/%s", node.Port, key)
		resp, err := mt.httpClient.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			continue
		}

		var response map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			continue
		}

		found, _ := response["found"].(bool)
		value, _ := response["value"].(string)

		return found, value, nil
	}

	return false, "", fmt.Errorf("无法从任何节点获取数据")
}

// collectMigrationStats 收集迁移统计信息
func (mt *MigrationTester) collectMigrationStats() (map[string]interface{}, error) {
	fmt.Println("  📈 收集各节点迁移统计信息...")

	stats := make(map[string]interface{})
	totalMigrated := 0

	allNodes := append(mt.initialNodes, mt.newNode)

	for _, node := range allNodes {
		if !mt.isNodeHealthy(node.Port) {
			continue
		}

		url := fmt.Sprintf("http://localhost:%d/admin/metrics", node.Port)
		resp, err := mt.httpClient.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		var metrics map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
			continue
		}

		stats[node.NodeID] = metrics

		// 提取迁移统计
		if migrationStats, exists := metrics["migration_stats"]; exists {
			if migrationMap, ok := migrationStats.(map[string]interface{}); ok {
				if migratedKeys, exists := migrationMap["migrated_keys"]; exists {
					if count, ok := migratedKeys.(float64); ok {
						totalMigrated += int(count)
						fmt.Printf("    %s: 迁移了 %.0f 个key\n", node.NodeID, count)
					}
				}
			}
		}
	}

	stats["total_migrated"] = totalMigrated
	fmt.Printf("  📊 总迁移数据: %d 个key\n", totalMigrated)

	return stats, nil
}

// stopExistingProcesses 停止现有进程
func (mt *MigrationTester) stopExistingProcesses() {
	fmt.Println("  🛑 停止现有进程...")

	// 尝试从PID文件停止进程
	pidFiles := []string{"logs/node1.pid", "logs/node2.pid", "logs/node4.pid"}

	for _, pidFile := range pidFiles {
		if data, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(string(data)); err == nil {
				if process, err := os.FindProcess(pid); err == nil {
					process.Kill()
				}
			}
			os.Remove(pidFile)
		}
	}

	// 等待进程完全停止
	time.Sleep(2 * time.Second)
}

// cleanup 清理测试环境
func (mt *MigrationTester) cleanup() {
	fmt.Println("  🧹 清理测试环境...")

	// 停止所有进程
	mt.stopExistingProcesses()

	// 清理配置文件
	os.Remove("config/node4.yaml")

	// 清理日志文件
	os.Remove("logs/node4.log")
	os.Remove("logs/node4.pid")

	fmt.Println("  ✅ 清理完成")
}

// PrintTestResult 打印测试结果
func (mt *MigrationTester) PrintTestResult(result *TestResult) {
	separator := "=================================================="
	fmt.Println("\n" + separator)
	fmt.Println("🧪 数据迁移测试结果")
	fmt.Println(separator)

	if result.Success {
		fmt.Println("✅ 测试状态: 成功")
	} else {
		fmt.Println("❌ 测试状态: 失败")
	}

	fmt.Printf("📝 测试消息: %s\n", result.Message)
	fmt.Printf("📊 数据完整性: %.1f%%\n", result.DataIntegrity)

	if len(result.Errors) > 0 {
		fmt.Println("\n❌ 错误信息:")
		for i, err := range result.Errors {
			fmt.Printf("  %d. %s\n", i+1, err)
		}
	}

	if result.MigrationStats != nil {
		fmt.Println("\n📈 迁移统计:")
		if totalMigrated, exists := result.MigrationStats["total_migrated"]; exists {
			fmt.Printf("  总迁移数据: %v 个key\n", totalMigrated)
		}
	}

	fmt.Println("\n" + separator)
}
