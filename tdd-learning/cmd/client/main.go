package main

import (
	"fmt"
	"log"
	"time"

	"tdd-learning/distributed"
)

func main() {
	// 创建分布式客户端
	config := distributed.ClientConfig{
		Nodes: []string{
			"localhost:8001",
			"localhost:8002",
			"localhost:8003",
		},
		Timeout:    5 * time.Second,
		RetryCount: 3,
	}
	
	client := distributed.NewDistributedClient(config)
	
	fmt.Println("🧪 分布式缓存客户端测试")
	fmt.Println("========================")
	
	// 等待服务器启动
	fmt.Println("⏳ 等待服务器启动...")
	time.Sleep(3 * time.Second)
	
	// 1. 检查集群健康状态
	fmt.Println("\n1. 📊 检查集群健康状态:")
	if healthStatus, err := client.CheckHealth(); err != nil {
		log.Printf("❌ 检查健康状态失败: %v", err)
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
	
	// 3. 测试基本缓存操作
	fmt.Println("\n3. 📝 测试基本缓存操作:")
	
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
	
	// 4. 测试批量操作
	fmt.Println("\n4. 📦 测试批量操作:")
	
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
	
	// 5. 测试删除操作
	fmt.Println("\n5. 🗑️  测试删除操作:")
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
	
	// 6. 获取统计信息
	fmt.Println("\n6. 📊 获取统计信息:")
	if stats, err := client.GetStats(); err != nil {
		fmt.Printf("   ❌ 获取统计失败: %v\n", err)
	} else {
		fmt.Printf("   📈 统计信息: %+v\n", stats)
	}
	
	// 7. 性能测试
	fmt.Println("\n7. ⚡ 性能测试:")
	performanceTest(client)
	
	fmt.Println("\n✅ 测试完成！")
	fmt.Println("\n💡 观察结果:")
	fmt.Println("   - 数据根据一致性哈希算法分布在不同节点")
	fmt.Println("   - 客户端可以从任意节点访问任意数据")
	fmt.Println("   - 请求会自动转发到正确的存储节点")
	fmt.Println("   - 支持故障转移和负载均衡")
	fmt.Println("   - 提供批量操作和性能优化")
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
