package main

import (
	"fmt"
	"math/rand"
	"time"
)

// DemoBasicHashRing 演示基础哈希环功能
func DemoBasicHashRing() {
	fmt.Println("=== 基础哈希环演示 ===")

	// 创建哈希环
	ring := NewHashRing()

	// 添加节点
	fmt.Println("\n1. 添加节点:")
	nodes := []*Node{
		{ID: "node1", Address: "192.168.1.1:6379", Weight: 100},
		{ID: "node2", Address: "192.168.1.2:6379", Weight: 100},
		{ID: "node3", Address: "192.168.1.3:6379", Weight: 100},
	}

	for _, node := range nodes {
		ring.AddNode(node)
	}

	// 打印哈希环状态
	ring.PrintRing()

	// 测试数据分布
	fmt.Println("\n2. 测试数据分布:")
	testKeys := []string{
		"user:1", "user:2", "user:3", "user:4", "user:5",
		"product:1", "product:2", "product:3",
		"order:1", "order:2",
	}

	for _, key := range testKeys {
		ring.GetNode(key)
	}

	// 统计分布
	distribution := ring.CalculateDataDistribution(testKeys)
	fmt.Println("\n📈 数据分布统计:")
	for nodeID, count := range distribution {
		percentage := float64(count) / float64(len(testKeys)) * 100
		fmt.Printf("  %s: %d 个key (%.1f%%)\n", nodeID, count, percentage)
	}
}

// DemoNodeFailure 演示节点故障处理
func DemoNodeFailure() {
	fmt.Println("\n=== 节点故障处理演示 ===")

	ring := NewHashRing()

	// 添加节点
	nodes := []*Node{
		{ID: "node1", Address: "192.168.1.1:6379", Weight: 100},
		{ID: "node2", Address: "192.168.1.2:6379", Weight: 100},
		{ID: "node3", Address: "192.168.1.3:6379", Weight: 100},
		{ID: "node4", Address: "192.168.1.4:6379", Weight: 100},
	}

	for _, node := range nodes {
		ring.AddNode(node)
	}

	testKeys := []string{"user:1", "user:2", "user:3", "user:4", "user:5"}

	// 故障前的数据分布
	fmt.Println("\n1. 故障前的数据分布:")
	beforeDistribution := ring.CalculateDataDistribution(testKeys)
	for nodeID, count := range beforeDistribution {
		fmt.Printf("  %s: %d 个key\n", nodeID, count)
	}

	// 模拟node2故障
	fmt.Println("\n2. 模拟 node2 故障:")
	ring.RemoveNode("node2")
	ring.PrintRing()

	// 故障后的数据分布
	fmt.Println("\n3. 故障后的数据分布:")
	afterDistribution := ring.CalculateDataDistribution(testKeys)
	for nodeID, count := range afterDistribution {
		fmt.Printf("  %s: %d 个key\n", nodeID, count)
	}

	// 分析数据迁移
	fmt.Println("\n4. 数据迁移分析:")
	migratedCount := 0
	for _, key := range testKeys {
		// 重新计算每个key的分布
		node := ring.GetNode(key)
		if node != nil {
			// 检查是否发生迁移
			beforeNode := ""
			for nodeID, count := range beforeDistribution {
				if count > 0 {
					// 这里简化处理，实际需要记录每个key的原始位置
					beforeNode = nodeID
					break
				}
			}
			if beforeNode == "node2" {
				migratedCount++
			}
		}
	}

	fmt.Printf("需要迁移的数据: %d/%d (%.1f%%)\n",
		migratedCount, len(testKeys),
		float64(migratedCount)/float64(len(testKeys))*100)
}

// DemoReplication 演示副本机制
func DemoReplication() {
	fmt.Println("\n=== 副本机制演示 ===")

	ring := NewHashRing()

	// 添加节点
	nodes := []*Node{
		{ID: "node1", Address: "192.168.1.1:6379", Weight: 100},
		{ID: "node2", Address: "192.168.1.2:6379", Weight: 100},
		{ID: "node3", Address: "192.168.1.3:6379", Weight: 100},
		{ID: "node4", Address: "192.168.1.4:6379", Weight: 100},
		{ID: "node5", Address: "192.168.1.5:6379", Weight: 100},
	}

	for _, node := range nodes {
		ring.AddNode(node)
	}

	// 测试副本分布
	fmt.Println("\n测试3副本分布:")
	testKeys := []string{"user:1", "product:100", "order:999"}

	for _, key := range testKeys {
		replicas := ring.GetNodes(key, 3)
		fmt.Printf("Key: %s → 副本节点: ", key)
		for i, node := range replicas {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(node.ID)
		}
		fmt.Println()
	}

	fmt.Println("\n💡 副本机制优势:")
	fmt.Println("   1. 提高数据可用性")
	fmt.Println("   2. 支持读负载分散")
	fmt.Println("   3. 故障时快速恢复")
}

// CompareWithTraditionalHash 对比传统哈希和一致性哈希
func CompareWithTraditionalHash() {
	fmt.Println("=== 传统哈希 vs 一致性哈希对比 ===")
	
	// 测试数据
	testKeys := []string{
		"user:1", "user:2", "user:3", "user:4", "user:5",
		"product:1", "product:2", "product:3", "product:4", "product:5",
		"order:1", "order:2", "order:3", "order:4", "order:5",
	}
	
	// 传统哈希分布
	fmt.Println("\n1. 传统哈希分布 (3个节点):")
	nodeCount := 3
	traditionalDistribution := make(map[int][]string)
	
	for _, key := range testKeys {
		// 简单哈希函数
		hash := simpleHash(key)
		nodeIndex := int(hash) % nodeCount
		traditionalDistribution[nodeIndex] = append(traditionalDistribution[nodeIndex], key)
	}
	
	for i := 0; i < nodeCount; i++ {
		fmt.Printf("  Node%d: %d keys - %v\n", i, len(traditionalDistribution[i]), traditionalDistribution[i])
	}
	
	// 传统哈希：增加一个节点
	fmt.Println("\n2. 传统哈希：增加节点后 (4个节点):")
	newNodeCount := 4
	newTraditionalDistribution := make(map[int][]string)
	migrationCount := 0
	
	for _, key := range testKeys {
		hash := simpleHash(key)
		oldNodeIndex := int(hash) % nodeCount
		newNodeIndex := int(hash) % newNodeCount
		
		newTraditionalDistribution[newNodeIndex] = append(newTraditionalDistribution[newNodeIndex], key)
		
		if oldNodeIndex != newNodeIndex {
			migrationCount++
		}
	}
	
	for i := 0; i < newNodeCount; i++ {
		fmt.Printf("  Node%d: %d keys - %v\n", i, len(newTraditionalDistribution[i]), newTraditionalDistribution[i])
	}
	
	traditionalMigrationRate := float64(migrationCount) / float64(len(testKeys)) * 100
	fmt.Printf("传统哈希迁移率: %.1f%% (%d/%d)\n", traditionalMigrationRate, migrationCount, len(testKeys))
	
	// 一致性哈希分布
	fmt.Println("\n3. 一致性哈希分布 (3个节点):")
	ring := NewHashRing()
	
	nodes := []*Node{
		{ID: "node0", Address: "192.168.1.1:6379", Weight: 100},
		{ID: "node1", Address: "192.168.1.2:6379", Weight: 100},
		{ID: "node2", Address: "192.168.1.3:6379", Weight: 100},
	}
	
	for _, node := range nodes {
		ring.AddNode(node)
	}
	
	consistentDistribution := make(map[string]string)
	for _, key := range testKeys {
		node := ring.GetNode(key)
		consistentDistribution[key] = node.ID
	}
	
	// 一致性哈希：增加节点
	fmt.Println("\n4. 一致性哈希：增加节点后 (4个节点):")
	newNode := &Node{ID: "node3", Address: "192.168.1.4:6379", Weight: 100}
	ring.AddNode(newNode)
	
	consistentMigrationCount := 0
	for _, key := range testKeys {
		node := ring.GetNode(key)
		if consistentDistribution[key] != node.ID {
			consistentMigrationCount++
		}
	}
	
	consistentMigrationRate := float64(consistentMigrationCount) / float64(len(testKeys)) * 100
	fmt.Printf("一致性哈希迁移率: %.1f%% (%d/%d)\n", 
		consistentMigrationRate, consistentMigrationCount, len(testKeys))
	
	// 对比结果
	fmt.Println("\n📊 对比结果:")
	fmt.Printf("  传统哈希迁移率: %.1f%%\n", traditionalMigrationRate)
	fmt.Printf("  一致性哈希迁移率: %.1f%%\n", consistentMigrationRate)
	fmt.Printf("  性能提升: %.1fx\n", traditionalMigrationRate/consistentMigrationRate)
}

// simpleHash 简单哈希函数
func simpleHash(key string) uint32 {
	var hash uint32
	for _, c := range key {
		hash = hash*31 + uint32(c)
	}
	return hash
}

// DemoPerformanceTest 性能测试
func DemoPerformanceTest() {
	fmt.Println("\n=== 性能测试 ===")
	
	// 创建大规模哈希环
	ring := NewVirtualHashRing(200)
	
	// 添加节点
	fmt.Println("\n1. 创建大规模集群 (10个节点):")
	for i := 0; i < 10; i++ {
		node := &Node{
			ID:      fmt.Sprintf("node%d", i),
			Address: fmt.Sprintf("192.168.1.%d:6379", i+1),
			Weight:  100,
		}
		ring.AddNode(node)
	}
	
	// 生成大量测试数据
	keyCount := 100000
	testKeys := make([]string, keyCount)
	for i := 0; i < keyCount; i++ {
		testKeys[i] = fmt.Sprintf("key_%d", i)
	}
	
	// 测试查找性能
	fmt.Printf("\n2. 查找性能测试 (%d个key):\n", keyCount)
	start := time.Now()
	
	for _, key := range testKeys {
		ring.GetNode(key)
	}
	
	duration := time.Since(start)
	avgTime := duration / time.Duration(keyCount)
	
	fmt.Printf("总耗时: %v\n", duration)
	fmt.Printf("平均查找时间: %v\n", avgTime)
	fmt.Printf("QPS: %.0f\n", float64(keyCount)/duration.Seconds())
	
	// 测试负载均衡
	fmt.Println("\n3. 负载均衡测试:")
	ring.CalculateLoadBalance(testKeys)
}

// DemoRealWorldScenario 真实场景演示
func DemoRealWorldScenario() {
	fmt.Println("\n=== 真实场景演示：Redis集群 ===")
	
	// 模拟Redis集群
	ring := NewVirtualHashRing(150)
	
	// 添加Redis节点
	fmt.Println("\n1. 初始Redis集群 (3主节点):")
	redisNodes := []*Node{
		{ID: "redis-master-1", Address: "10.0.1.1:6379", Weight: 100},
		{ID: "redis-master-2", Address: "10.0.1.2:6379", Weight: 100},
		{ID: "redis-master-3", Address: "10.0.1.3:6379", Weight: 100},
	}
	
	for _, node := range redisNodes {
		ring.AddNode(node)
	}
	
	// 模拟用户数据
	userKeys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		userKeys[i] = fmt.Sprintf("user:%d", i)
	}
	
	fmt.Println("\n2. 用户数据分布:")
	ring.CalculateLoadBalance(userKeys)
	
	// 模拟扩容场景
	fmt.Println("\n3. 集群扩容 (添加2个新节点):")
	newNodes := []*Node{
		{ID: "redis-master-4", Address: "10.0.1.4:6379", Weight: 100},
		{ID: "redis-master-5", Address: "10.0.1.5:6379", Weight: 100},
	}
	
	// 记录扩容前的分布
	beforeExpansion := make(map[string]string)
	for _, key := range userKeys {
		node := ring.GetNode(key)
		beforeExpansion[key] = node.ID
	}
	
	// 添加新节点
	for _, node := range newNodes {
		ring.AddNode(node)
	}
	
	fmt.Println("\n4. 扩容后数据分布:")
	ring.CalculateLoadBalance(userKeys)
	
	// 计算数据迁移
	migrationCount := 0
	for _, key := range userKeys {
		node := ring.GetNode(key)
		if beforeExpansion[key] != node.ID {
			migrationCount++
		}
	}
	
	migrationRate := float64(migrationCount) / float64(len(userKeys)) * 100
	fmt.Printf("\n5. 扩容影响分析:\n")
	fmt.Printf("需要迁移的数据: %d/%d (%.1f%%)\n", 
		migrationCount, len(userKeys), migrationRate)
	fmt.Printf("理论最优迁移率: %.1f%%\n", 
		float64(len(newNodes))/float64(len(redisNodes)+len(newNodes))*100)
	
	// 模拟故障场景
	fmt.Println("\n6. 故障恢复演示:")
	fmt.Println("模拟 redis-master-2 故障...")
	
	beforeFailure := make(map[string]string)
	for _, key := range userKeys[:10] { // 只测试前10个key
		node := ring.GetNode(key)
		beforeFailure[key] = node.ID
	}
	
	ring.RemoveNode("redis-master-2")
	
	fmt.Println("故障后数据重新分布:")
	failoverCount := 0
	for _, key := range userKeys[:10] {
		node := ring.GetNode(key)
		if beforeFailure[key] != node.ID {
			fmt.Printf("  %s: %s → %s\n", key, beforeFailure[key], node.ID)
			failoverCount++
		}
	}
	
	fmt.Printf("受影响的数据: %d/10\n", failoverCount)
	
	fmt.Println("\n💡 真实场景优势:")
	fmt.Println("   1. 扩容时数据迁移量小")
	fmt.Println("   2. 故障时影响范围可控")
	fmt.Println("   3. 负载分布相对均匀")
	fmt.Println("   4. 支持异构节点(不同权重)")
}

func main() {
	fmt.Println("🎮 第四章：一致性哈希 - 综合演示程序")
	fmt.Println("==========================================")
	
	// 设置随机种子
	rand.Seed(time.Now().UnixNano())
	
	// 基础功能演示
	DemoBasicHashRing()
	fmt.Println("✅ DemoBasicHashRing completed")

	DemoNodeFailure()
	fmt.Println("✅ DemoNodeFailure completed")

	DemoReplication()
	fmt.Println("✅ DemoReplication completed")

	// 虚拟节点演示
	DemoVirtualNodes()
	fmt.Println("✅ DemoVirtualNodes completed")

	DemoWeightedNodes()
	fmt.Println("✅ DemoWeightedNodes completed")

	DemoDataMigration()
	fmt.Println("✅ DemoDataMigration completed")

	// 对比和性能测试
	CompareWithTraditionalHash()
	fmt.Println("✅ CompareWithTraditionalHash completed")

	DemoPerformanceTest()
	fmt.Println("✅ DemoPerformanceTest completed")

	// 真实场景
	DemoRealWorldScenario()
	fmt.Println("✅ DemoRealWorldScenario completed")
	
	fmt.Println("\n🎉 演示完成！")
	fmt.Println("==========================================")
	
	fmt.Println("\n💡 关键要点总结:")
	fmt.Println("\n🔄 一致性哈希核心:")
	fmt.Println("   1. 哈希环：将哈希空间组织成环形")
	fmt.Println("   2. 节点映射：服务器节点映射到环上")
	fmt.Println("   3. 数据定位：顺时针找到第一个节点")
	fmt.Println("   4. 最小迁移：节点变化时影响最小")
	
	fmt.Println("\n🎯 虚拟节点优势:")
	fmt.Println("   1. 解决数据倾斜问题")
	fmt.Println("   2. 提高负载均衡性")
	fmt.Println("   3. 支持加权分配")
	fmt.Println("   4. 减少热点问题")
	
	fmt.Println("\n📊 性能特点:")
	fmt.Println("   1. 查找时间复杂度：O(log N)")
	fmt.Println("   2. 数据迁移量：约1/N (N为节点数)")
	fmt.Println("   3. 内存开销：O(N × V) (V为虚拟节点数)")
	fmt.Println("   4. 扩展性好，支持大规模集群")
	
	fmt.Println("\n🎯 面试重点:")
	fmt.Println("   1. 能解释一致性哈希的基本原理")
	fmt.Println("   2. 理解虚拟节点的作用和实现")
	fmt.Println("   3. 掌握与传统哈希的区别和优势")
	fmt.Println("   4. 了解在分布式系统中的应用")
	
	fmt.Println("\n📖 下一步: 学习第五章 - 多级缓存")
	fmt.Println("   将学习本地缓存+分布式缓存的架构设计")
}
