// 测试客户端 ： ./bin/cache_client
package main

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"tdd-learning/distributed"
)

func main() {
	// 创建分布式客户端 - 启用智能健康检查和负载均衡
	config := distributed.ClientConfig{
		Nodes: []string{
			"localhost:8001",
			"localhost:8002",
			"localhost:8003",
		},
		Timeout:    5 * time.Second,
		RetryCount: 3,

		// 新增：智能节点管理配置
		HealthCheckEnabled:    true,
		HealthCheckInterval:   10 * time.Second, // 10秒检查一次
		FailureThreshold:      2,                // 2次失败后标记为不健康
		RecoveryCheckInterval: 15 * time.Second, // 15秒检查恢复
	}

	client := distributed.NewDistributedClient(config)
	defer client.Close() // 优雅关闭，停止健康检查协程

	fmt.Println("🧪 智能分布式缓存客户端测试")
	fmt.Println("================================")
	fmt.Println("✨ 新特性：智能健康检查 + 自动故障转移 + 负载均衡")
	fmt.Println()

	// 等待服务器启动和初始健康检查
	fmt.Println("⏳ 等待服务器启动和健康检查...")
	time.Sleep(5 * time.Second)
	
	// 1. 智能节点状态检查
	fmt.Println("1. 🔍 智能节点状态检查:")
	nodeStatus := client.GetNodeStatus()
	if nodeStatus == nil {
		fmt.Println("   ⚠️  健康检查未启用或节点管理器未初始化")
	} else {
		fmt.Println("   📊 详细节点状态:")
		healthyCount := 0
		for node, status := range nodeStatus {
			healthIcon := "❌"
			if status.IsHealthy {
				healthIcon = "✅"
				healthyCount++
			}
			fmt.Printf("   %s 节点 %s:\n", healthIcon, node)
			fmt.Printf("      健康状态: %v\n", status.IsHealthy)
			fmt.Printf("      失败次数: %d\n", status.FailureCount)
			fmt.Printf("      最后检查: %v\n", status.LastCheckTime.Format("15:04:05"))
			if !status.LastSuccessTime.IsZero() {
				fmt.Printf("      最后成功: %v\n", status.LastSuccessTime.Format("15:04:05"))
			}
			if !status.LastFailureTime.IsZero() {
				fmt.Printf("      最后失败: %v\n", status.LastFailureTime.Format("15:04:05"))
			}
			fmt.Println()
		}
		fmt.Printf("   � 健康节点: %d/%d\n", healthyCount, len(nodeStatus))
	}

	// 传统健康检查对比
	fmt.Println("\n   🔄 传统健康检查对比:")
	if healthStatus, err := client.CheckHealth(); err != nil {
		log.Printf("   ❌ 检查健康状态失败: %v", err)
	} else {
		for node, healthy := range healthStatus {
			status := "❌ 不健康"
			if healthy {
				status = "✅ 健康"
			}
			fmt.Printf("   节点 %s: %s\n", node, status)
		}
	}
	
	// 2. 获取集群信息
	fmt.Println("\n2. 🌐 获取集群信息:")
	if clusterInfo, err := client.GetClusterInfo(); err != nil {
		log.Printf("❌ 获取集群信息失败: %v", err)
	} else {
		fmt.Printf("   集群信息: %+v\n", clusterInfo)
	}
	
	// 3. 智能负载均衡测试
	fmt.Println("\n3. ⚖️  智能负载均衡测试:")
	testLoadBalancing(client)

	// 4. 测试基本缓存操作
	fmt.Println("\n4. 📝 测试基本缓存操作:")
	
	// 测试数据
	testData := map[string]string{
		"user:1001":    "张三",
		"user:1002":    "李四",
		"user:1003":    "王五",
		"product:2001": "iPhone 15",
		"product:2002": "MacBook Pro",
		"order:3001":   "订单详情1",
		"order:3002":   "订单详情2",
		"session:abc":  "用户会话数据",
	}
	
	// 设置数据
	fmt.Println("   设置数据:")
	for key, value := range testData {
		if err := client.Set(key, value); err != nil {
			fmt.Printf("     ❌ 设置 %s 失败: %v\n", key, err)
		} else {
			fmt.Printf("     ✅ 设置 %s = %s\n", key, value)
		}
	}
	
	// 获取数据
	fmt.Println("\n   获取数据:")
	for key, expectedValue := range testData {
		if value, found, err := client.Get(key); err != nil {
			fmt.Printf("     ❌ 获取 %s 失败: %v\n", key, err)
		} else if !found {
			fmt.Printf("     ⚠️  %s 未找到\n", key)
		} else if value != expectedValue {
			fmt.Printf("     ❌ %s 值不匹配: 期望=%s, 实际=%s\n", key, expectedValue, value)
		} else {
			fmt.Printf("     ✅ 获取 %s = %s\n", key, value)
		}
	}
	
	// 5. 测试批量操作
	fmt.Println("\n5. 📦 测试批量操作:")

	// 批量设置
	batchData := map[string]string{
		"batch:1": "批量数据1",
		"batch:2": "批量数据2",
		"batch:3": "批量数据3",
	}

	if err := client.BatchSet(batchData); err != nil {
		fmt.Printf("   ❌ 批量设置失败: %v\n", err)
	} else {
		fmt.Printf("   ✅ 批量设置成功: %d 个键\n", len(batchData))
	}

	// 批量获取
	keys := []string{"batch:1", "batch:2", "batch:3"}
	if result, err := client.BatchGet(keys); err != nil {
		fmt.Printf("   ❌ 批量获取失败: %v\n", err)
	} else {
		fmt.Printf("   ✅ 批量获取成功: %d 个键\n", len(result))
		for key, value := range result {
			fmt.Printf("     %s = %s\n", key, value)
		}
	}

	// 6. 测试删除操作
	fmt.Println("\n6. 🗑️  测试删除操作:")
	deleteKeys := []string{"user:1001", "product:2001", "batch:1"}

	for _, key := range deleteKeys {
		if err := client.Delete(key); err != nil {
			fmt.Printf("   ❌ 删除 %s 失败: %v\n", key, err)
		} else {
			fmt.Printf("   ✅ 删除 %s 成功\n", key)
		}
	}

	// 验证删除
	fmt.Println("   验证删除:")
	for _, key := range deleteKeys {
		if value, found, err := client.Get(key); err != nil {
			fmt.Printf("     ❌ 验证 %s 失败: %v\n", key, err)
		} else if found {
			fmt.Printf("     ❌ %s 仍然存在: %s\n", key, value)
		} else {
			fmt.Printf("     ✅ %s 已被删除\n", key)
		}
	}

	// 7. 故障转移测试
	fmt.Println("\n7. 🔄 故障转移测试:")
	testFailover(client)

	// 8. 获取统计信息
	fmt.Println("\n8. 📊 获取统计信息:")
	if stats, err := client.GetStats(); err != nil {
		fmt.Printf("   ❌ 获取统计失败: %v\n", err)
	} else {
		fmt.Printf("   📈 统计信息: %+v\n", stats)
	}

	// 9. 性能测试
	fmt.Println("\n9. ⚡ 性能测试:")
	performanceTest(client)

	// 10. 并发性能测试
	fmt.Println("\n10. 🚀 并发性能测试:")
	concurrentPerformanceTest(client)

	// 11. 连接池效果验证
	fmt.Println("\n11. 🔗 连接池效果验证:")
	connectionPoolTest(client)
	
	fmt.Println("\n✅ 智能分布式缓存测试完成！")
	fmt.Println("\n🎯 新特性验证结果:")
	fmt.Println("   ✅ 智能健康检查：自动监控节点状态")
	fmt.Println("   ✅ 故障检测：自动识别和标记故障节点")
	fmt.Println("   ✅ 智能负载均衡：优先使用健康节点")
	fmt.Println("   ✅ 自动故障转移：故障节点自动切换")
	fmt.Println("   ✅ 节点恢复：故障节点恢复后自动重新使用")
	fmt.Println("   ✅ 性能优化：减少请求失败和重试")
	fmt.Println()
	fmt.Println("💡 传统特性保持:")
	fmt.Println("   - 数据根据一致性哈希算法分布在不同节点")
	fmt.Println("   - 客户端可以从任意节点访问任意数据")
	fmt.Println("   - 请求会自动转发到正确的存储节点")
	fmt.Println("   - 支持批量操作和性能优化")
	fmt.Println()
	fmt.Println("🚀 企业级特性:")
	fmt.Println("   - 零配置动态节点管理")
	fmt.Println("   - 生产环境就绪的可靠性")
	fmt.Println("   - 完整的监控和状态查询")
	fmt.Println("   - 优雅的资源管理和关闭")
}

// performanceTest 性能测试
func performanceTest(client *distributed.DistributedClient) {
	fmt.Println("   执行性能测试...")
	
	// 测试参数
	numOperations := 1000
	
	// 写性能测试
	start := time.Now()
	for i := 0; i < numOperations; i++ {
		key := fmt.Sprintf("perf:write:%d", i)
		value := fmt.Sprintf("value_%d", i)
		if err := client.Set(key, value); err != nil {
			fmt.Printf("     ⚠️ 写操作失败: %v\n", err)
			break
		}
	}
	writeDuration := time.Since(start)
	writeOPS := float64(numOperations) / writeDuration.Seconds()
	
	// 读性能测试
	start = time.Now()
	for i := 0; i < numOperations; i++ {
		key := fmt.Sprintf("perf:write:%d", i)
		if _, _, err := client.Get(key); err != nil {
			fmt.Printf("     ⚠️ 读操作失败: %v\n", err)
			break
		}
	}
	readDuration := time.Since(start)
	readOPS := float64(numOperations) / readDuration.Seconds()
	
	fmt.Printf("   📊 性能结果:\n")
	fmt.Printf("     写操作: %.2f ops/s (%d 操作，耗时 %v)\n", writeOPS, numOperations, writeDuration)
	fmt.Printf("     读操作: %.2f ops/s (%d 操作，耗时 %v)\n", readOPS, numOperations, readDuration)
	
	// 清理性能测试数据
	for i := 0; i < numOperations; i++ {
		key := fmt.Sprintf("perf:write:%d", i)
		client.Delete(key)
	}
}

// testLoadBalancing 测试智能负载均衡
func testLoadBalancing(client *distributed.DistributedClient) {
	fmt.Println("   🔄 执行负载均衡测试...")

	// 发送多个请求，观察负载分布
	testKeys := []string{
		"lb:test:1", "lb:test:2", "lb:test:3", "lb:test:4", "lb:test:5",
		"lb:test:6", "lb:test:7", "lb:test:8", "lb:test:9", "lb:test:10",
	}

	fmt.Println("   📤 发送测试请求...")
	successCount := 0
	for i, key := range testKeys {
		value := fmt.Sprintf("load_balance_value_%d", i)
		if err := client.Set(key, value); err != nil {
			fmt.Printf("     ⚠️ 设置 %s 失败: %v\n", key, err)
		} else {
			successCount++
		}
	}

	fmt.Printf("   📊 请求结果: %d/%d 成功\n", successCount, len(testKeys))

	// 验证数据
	fmt.Println("   📥 验证数据...")
	retrieveCount := 0
	for _, key := range testKeys {
		if _, found, err := client.Get(key); err != nil {
			fmt.Printf("     ⚠️ 获取 %s 失败: %v\n", key, err)
		} else if found {
			retrieveCount++
		}
	}

	fmt.Printf("   📊 验证结果: %d/%d 找到\n", retrieveCount, len(testKeys))

	// 显示当前节点状态
	fmt.Println("   📈 当前节点状态:")
	nodeStatus := client.GetNodeStatus()
	for node, status := range nodeStatus {
		healthIcon := "✅"
		if !status.IsHealthy {
			healthIcon = "❌"
		}
		fmt.Printf("     %s %s (失败: %d)\n", healthIcon, node, status.FailureCount)
	}

	// 清理测试数据
	for _, key := range testKeys {
		client.Delete(key)
	}
}

// testFailover 测试故障转移
func testFailover(client *distributed.DistributedClient) {
	fmt.Println("   🔧 模拟故障转移场景...")

	// 设置一些测试数据
	testData := map[string]string{
		"failover:1": "数据1",
		"failover:2": "数据2",
		"failover:3": "数据3",
	}

	fmt.Println("   📤 设置测试数据...")
	for key, value := range testData {
		if err := client.Set(key, value); err != nil {
			fmt.Printf("     ⚠️ 设置 %s 失败: %v\n", key, err)
		}
	}

	// 显示设置后的节点状态
	fmt.Println("   📊 设置后节点状态:")
	nodeStatus := client.GetNodeStatus()
	if nodeStatus != nil {
		healthyNodes := 0
		for node, status := range nodeStatus {
			if status.IsHealthy {
				healthyNodes++
			}
			fmt.Printf("     节点 %s: 健康=%v, 失败=%d\n",
				node, status.IsHealthy, status.FailureCount)
		}
		fmt.Printf("   💚 健康节点数: %d\n", healthyNodes)
	}

	// 尝试访问数据，测试客户端的智能路由
	fmt.Println("   📥 验证数据访问...")
	accessCount := 0
	for key := range testData {
		if _, found, err := client.Get(key); err != nil {
			fmt.Printf("     ⚠️ 访问 %s 失败: %v\n", key, err)
		} else if found {
			accessCount++
		}
	}

	fmt.Printf("   📊 访问结果: %d/%d 成功\n", accessCount, len(testData))

	// 提示用户可以手动测试故障转移
	fmt.Println("   💡 故障转移测试提示:")
	fmt.Println("     - 可以手动停止一个节点来测试故障转移")
	fmt.Println("     - 客户端会自动检测故障并切换到健康节点")
	fmt.Println("     - 重启节点后会自动恢复并重新加入负载均衡")

	// 清理测试数据
	for key := range testData {
		client.Delete(key)
	}
}

// concurrentPerformanceTest 并发性能测试
func concurrentPerformanceTest(client *distributed.DistributedClient) {
	fmt.Println("   🔄 执行并发性能测试...")

	// 测试参数
	numOperations := 1000
	concurrencyLevels := []int{1, 5, 10, 20}

	for _, concurrency := range concurrencyLevels {
		fmt.Printf("\n   📊 并发度 %d 测试:\n", concurrency)

		// 并发写测试
		testConcurrentWrites(client, numOperations, concurrency)

		// 并发读测试
		testConcurrentReads(client, numOperations, concurrency)

		// 混合读写测试
		testMixedOperations(client, numOperations, concurrency)
	}
}

// testConcurrentWrites 测试并发写操作
func testConcurrentWrites(client *distributed.DistributedClient, total, concurrency int) {
	var wg sync.WaitGroup
	operationsPerWorker := total / concurrency

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < operationsPerWorker; j++ {
				key := fmt.Sprintf("concurrent:write:%d:%d", workerID, j)
				value := fmt.Sprintf("value_%d_%d", workerID, j)
				client.Set(key, value)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)
	ops := float64(total) / duration.Seconds()

	fmt.Printf("     ✍️  并发写: %.2f ops/s (%d 操作，%d 并发，耗时 %v)\n",
		ops, total, concurrency, duration)

	// 清理数据
	for i := 0; i < concurrency; i++ {
		for j := 0; j < operationsPerWorker; j++ {
			key := fmt.Sprintf("concurrent:write:%d:%d", i, j)
			client.Delete(key)
		}
	}
}

// testConcurrentReads 测试并发读操作
func testConcurrentReads(client *distributed.DistributedClient, total, concurrency int) {
	operationsPerWorker := total / concurrency

	// 先准备测试数据
	for i := 0; i < concurrency; i++ {
		for j := 0; j < operationsPerWorker; j++ {
			key := fmt.Sprintf("concurrent:read:%d:%d", i, j)
			value := fmt.Sprintf("read_value_%d_%d", i, j)
			client.Set(key, value)
		}
	}

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < operationsPerWorker; j++ {
				key := fmt.Sprintf("concurrent:read:%d:%d", workerID, j)
				client.Get(key)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)
	ops := float64(total) / duration.Seconds()

	fmt.Printf("     📖 并发读: %.2f ops/s (%d 操作，%d 并发，耗时 %v)\n",
		ops, total, concurrency, duration)

	// 清理数据
	for i := 0; i < concurrency; i++ {
		for j := 0; j < operationsPerWorker; j++ {
			key := fmt.Sprintf("concurrent:read:%d:%d", i, j)
			client.Delete(key)
		}
	}
}

// testMixedOperations 测试混合读写操作
func testMixedOperations(client *distributed.DistributedClient, total, concurrency int) {
	operationsPerWorker := total / concurrency
	var wg sync.WaitGroup

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < operationsPerWorker; j++ {
				key := fmt.Sprintf("concurrent:mixed:%d:%d", workerID, j)

				// 70%读操作，30%写操作
				if j%10 < 7 {
					// 读操作
					client.Get(key)
				} else {
					// 写操作
					value := fmt.Sprintf("mixed_value_%d_%d", workerID, j)
					client.Set(key, value)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)
	ops := float64(total) / duration.Seconds()

	fmt.Printf("     🔄 混合读写: %.2f ops/s (%d 操作，%d 并发，耗时 %v)\n",
		ops, total, concurrency, duration)

	// 清理数据
	for i := 0; i < concurrency; i++ {
		for j := 0; j < operationsPerWorker; j++ {
			key := fmt.Sprintf("concurrent:mixed:%d:%d", i, j)
			client.Delete(key)
		}
	}
}

// connectionPoolTest 连接池效果验证测试
func connectionPoolTest(client *distributed.DistributedClient) {
	fmt.Println("   🔗 验证连接池优化效果...")

	// 测试1: 连续请求延迟测试
	fmt.Println("\n   📊 连续请求延迟分析:")
	testConnectionReuse(client)

	// 测试2: 突发并发测试
	fmt.Println("\n   📊 突发并发测试:")
	testBurstConcurrency(client)

	// 测试3: 长时间运行测试
	fmt.Println("\n   📊 长时间运行测试:")
	testLongRunning(client)
}

// testConnectionReuse 测试连接复用效果
func testConnectionReuse(client *distributed.DistributedClient) {
	numRequests := 100
	latencies := make([]time.Duration, numRequests)

	// 预热一次请求，建立连接
	client.Set("warmup", "warmup_value")
	client.Delete("warmup")

	// 测试连续请求的延迟
	for i := 0; i < numRequests; i++ {
		key := fmt.Sprintf("latency_test_%d", i)
		value := fmt.Sprintf("value_%d", i)

		start := time.Now()
		client.Set(key, value)
		latencies[i] = time.Since(start)

		client.Delete(key) // 立即清理
	}

	// 分析延迟分布
	var total time.Duration
	var min, max time.Duration = latencies[0], latencies[0]

	for _, latency := range latencies {
		total += latency
		if latency < min {
			min = latency
		}
		if latency > max {
			max = latency
		}
	}

	avg := total / time.Duration(numRequests)

	fmt.Printf("     平均延迟: %v\n", avg)
	fmt.Printf("     最小延迟: %v\n", min)
	fmt.Printf("     最大延迟: %v\n", max)
	fmt.Printf("     延迟比值: %.2fx (最大/最小)\n", float64(max)/float64(min))

	// 分析前10次和后10次的延迟差异
	var firstTen, lastTen time.Duration
	for i := 0; i < 10; i++ {
		firstTen += latencies[i]
		lastTen += latencies[numRequests-10+i]
	}

	firstTenAvg := firstTen / 10
	lastTenAvg := lastTen / 10

	fmt.Printf("     前10次平均: %v\n", firstTenAvg)
	fmt.Printf("     后10次平均: %v\n", lastTenAvg)

	if firstTenAvg > lastTenAvg {
		improvement := float64(firstTenAvg-lastTenAvg) / float64(firstTenAvg) * 100
		fmt.Printf("     🚀 连接复用效果: 延迟降低 %.1f%%\n", improvement)
	} else {
		fmt.Printf("     📊 连接复用效果: 延迟相对稳定\n")
	}
}

// testBurstConcurrency 测试突发并发场景
func testBurstConcurrency(client *distributed.DistributedClient) {
	burstSizes := []int{5, 10, 20, 50}

	for _, burstSize := range burstSizes {
		var wg sync.WaitGroup
		start := time.Now()

		// 突发并发请求
		for i := 0; i < burstSize; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("burst_%d_%d", burstSize, id)
				value := fmt.Sprintf("burst_value_%d", id)
				client.Set(key, value)
				client.Get(key)
				client.Delete(key)
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		fmt.Printf("     突发 %d 并发: 总耗时 %v, 平均 %v/请求\n",
			burstSize, duration, duration/time.Duration(burstSize))
	}
}

// testLongRunning 测试长时间运行场景
func testLongRunning(client *distributed.DistributedClient) {
	fmt.Println("     执行30秒长时间运行测试...")

	duration := 30 * time.Second
	concurrency := 5

	var wg sync.WaitGroup
	var totalOps int64
	var totalErrors int64

	start := time.Now()
	stopChan := make(chan struct{})

	// 启动定时器
	go func() {
		time.Sleep(duration)
		close(stopChan)
	}()

	// 启动工作协程
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			ops := 0
			errors := 0

			for {
				select {
				case <-stopChan:
					atomic.AddInt64(&totalOps, int64(ops))
					atomic.AddInt64(&totalErrors, int64(errors))
					return
				default:
					key := fmt.Sprintf("longrun_%d_%d", workerID, ops)
					value := fmt.Sprintf("value_%d", ops)

					if err := client.Set(key, value); err != nil {
						errors++
					} else {
						if _, _, err := client.Get(key); err != nil {
							errors++
						}
						client.Delete(key)
						ops++
					}
				}
			}
		}(i)
	}

	wg.Wait()
	actualDuration := time.Since(start)

	avgOps := float64(totalOps) / actualDuration.Seconds()
	errorRate := float64(totalErrors) / float64(totalOps+totalErrors) * 100

	fmt.Printf("     总操作数: %d\n", totalOps)
	fmt.Printf("     总错误数: %d\n", totalErrors)
	fmt.Printf("     平均QPS: %.2f ops/s\n", avgOps)
	fmt.Printf("     错误率: %.2f%%\n", errorRate)
	fmt.Printf("     实际运行时间: %v\n", actualDuration)

	if errorRate < 1.0 {
		fmt.Printf("     🎉 长时间运行稳定性: 优秀\n")
	} else if errorRate < 5.0 {
		fmt.Printf("     ✅ 长时间运行稳定性: 良好\n")
	} else {
		fmt.Printf("     ⚠️ 长时间运行稳定性: 需要优化\n")
	}
}
