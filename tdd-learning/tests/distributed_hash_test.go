package tests

import (
	"fmt"
	"sort"
	"testing"

	"tdd-learning/core"
	"tdd-learning/distributed"
)

// TestConsistentHashDistribution 测试一致性哈希的数据分布
func TestConsistentHashDistribution(t *testing.T) {
	t.Log("🧪 开始测试一致性哈希数据分布...")

	// 创建3节点集群
	nodes := []string{"node1", "node2", "node3"}
	dc := core.NewDistributedCache(nodes)

	// 测试数据集
	testKeys := []string{
		"user:1001", "user:1002", "user:1003", "user:1004", "user:1005",
		"product:2001", "product:2002", "product:2003", "product:2004", "product:2005",
		"order:3001", "order:3002", "order:3003", "order:3004", "order:3005",
		"session:abc123", "session:def456", "session:ghi789",
		"cache:key1", "cache:key2", "cache:key3", "cache:key4", "cache:key5",
	}

	// 统计每个节点分配到的key数量
	nodeDistribution := make(map[string][]string)
	
	for _, key := range testKeys {
		targetNode := dc.GetNodeForKey(key)
		nodeDistribution[targetNode] = append(nodeDistribution[targetNode], key)
	}

	t.Logf("📊 数据分布结果:")
	totalKeys := len(testKeys)
	for node, keys := range nodeDistribution {
		percentage := float64(len(keys)) / float64(totalKeys) * 100
		t.Logf("  %s: %d个key (%.1f%%) - %v", node, len(keys), percentage, keys)
	}

	// 验证分布相对均匀（每个节点应该有数据）
	if len(nodeDistribution) != 3 {
		t.Errorf("❌ 期望3个节点都有数据分配，实际只有%d个节点", len(nodeDistribution))
	}

	// 验证没有节点分配过多数据（简单的均匀性检查）
	for node, keys := range nodeDistribution {
		if len(keys) == 0 {
			t.Errorf("❌ 节点 %s 没有分配到任何数据", node)
		}
		if len(keys) > totalKeys*2/3 {
			t.Errorf("❌ 节点 %s 分配了过多数据: %d/%d", node, len(keys), totalKeys)
		}
	}

	t.Log("✅ 一致性哈希数据分布测试通过")
}

// TestDataMigrationOnNodeAddition 测试添加节点时的数据迁移
func TestDataMigrationOnNodeAddition(t *testing.T) {
	t.Log("🧪 开始测试添加节点时的数据迁移...")

	// 1. 创建初始2节点集群
	initialNodes := []string{"node1", "node2"}
	dc := core.NewDistributedCache(initialNodes)

	// 2. 添加测试数据
	testData := map[string]string{
		"user:1001":    "张三",
		"user:1002":    "李四",
		"product:2001": "iPhone15",
		"product:2002": "MacBook",
		"order:3001":   "订单1",
		"order:3002":   "订单2",
		"session:abc":  "会话1",
		"session:def":  "会话2",
	}

	for key, value := range testData {
		targetNode := dc.GetNodeForKey(key)
		localCache := dc.LocalCaches[targetNode]
		localCache.Set(key, value)
		t.Logf("📝 初始数据: %s -> %s (存储在 %s)", key, value, targetNode)
	}

	// 3. 记录添加节点前的数据分布
	beforeDistribution := make(map[string][]string)
	for key := range testData {
		targetNode := dc.GetNodeForKey(key)
		beforeDistribution[targetNode] = append(beforeDistribution[targetNode], key)
	}

	t.Logf("📊 添加节点前的分布:")
	for node, keys := range beforeDistribution {
		t.Logf("  %s: %v", node, keys)
	}

	// 4. 添加新节点并触发数据迁移
	t.Log("🔄 添加新节点 node3...")
	err := dc.AddNode("node3")
	if err != nil {
		t.Fatalf("❌ 添加节点失败: %v", err)
	}

	// 5. 检查数据迁移后的分布
	afterDistribution := make(map[string][]string)
	for key := range testData {
		targetNode := dc.GetNodeForKey(key)
		afterDistribution[targetNode] = append(afterDistribution[targetNode], key)
	}

	t.Logf("📊 添加节点后的分布:")
	for node, keys := range afterDistribution {
		t.Logf("  %s: %v", node, keys)
	}

	// 6. 验证数据完整性
	t.Log("🔍 验证数据完整性...")
	for key, expectedValue := range testData {
		targetNode := dc.GetNodeForKey(key)
		localCache := dc.LocalCaches[targetNode]
		
		actualValue, found := localCache.Get(key)
		if !found {
			t.Errorf("❌ 数据丢失: key=%s 应该在节点 %s", key, targetNode)
			continue
		}
		
		if actualValue != expectedValue {
			t.Errorf("❌ 数据错误: key=%s, 期望=%s, 实际=%s", key, expectedValue, actualValue)
		} else {
			t.Logf("✅ 数据正确: %s = %s (在节点 %s)", key, actualValue, targetNode)
		}
	}

	// 7. 验证迁移统计
	migrationStats := dc.GetMigrationStats()
	t.Logf("📈 迁移统计: 迁移了 %d 个key，耗时 %v", 
		migrationStats.MigratedKeys, migrationStats.Duration)

	if migrationStats.MigratedKeys == 0 {
		t.Log("⚠️  注意: 没有key被迁移，这可能是正常的（取决于哈希分布）")
	}

	t.Log("✅ 添加节点数据迁移测试通过")
}

// TestDataMigrationOnNodeRemoval 测试移除节点时的数据迁移
func TestDataMigrationOnNodeRemoval(t *testing.T) {
	t.Log("🧪 开始测试移除节点时的数据迁移...")

	// 1. 创建3节点集群
	nodes := []string{"node1", "node2", "node3"}
	dc := core.NewDistributedCache(nodes)

	// 2. 添加测试数据
	testData := map[string]string{
		"user:1001":    "张三",
		"user:1002":    "李四", 
		"user:1003":    "王五",
		"product:2001": "iPhone15",
		"product:2002": "MacBook",
		"product:2003": "iPad",
		"order:3001":   "订单1",
		"order:3002":   "订单2",
		"order:3003":   "订单3",
	}

	for key, value := range testData {
		targetNode := dc.GetNodeForKey(key)
		localCache := dc.LocalCaches[targetNode]
		localCache.Set(key, value)
		t.Logf("📝 初始数据: %s -> %s (存储在 %s)", key, value, targetNode)
	}

	// 3. 记录移除节点前的数据分布
	beforeDistribution := make(map[string][]string)
	for key := range testData {
		targetNode := dc.GetNodeForKey(key)
		beforeDistribution[targetNode] = append(beforeDistribution[targetNode], key)
	}

	t.Logf("📊 移除节点前的分布:")
	for node, keys := range beforeDistribution {
		t.Logf("  %s: %v", node, keys)
	}

	// 4. 找到有数据的节点进行移除
	var nodeToRemove string
	var keysToMigrate []string
	for node, keys := range beforeDistribution {
		if len(keys) > 0 {
			nodeToRemove = node
			keysToMigrate = keys
			break
		}
	}

	if nodeToRemove == "" {
		t.Skip("⚠️  跳过测试: 没有找到有数据的节点可以移除")
		return
	}

	t.Logf("🔄 移除节点 %s (包含 %d 个key: %v)...", nodeToRemove, len(keysToMigrate), keysToMigrate)

	// 5. 移除节点并触发数据迁移
	err := dc.RemoveNode(nodeToRemove)
	if err != nil {
		t.Fatalf("❌ 移除节点失败: %v", err)
	}

	// 6. 检查数据迁移后的分布
	afterDistribution := make(map[string][]string)
	for key := range testData {
		targetNode := dc.GetNodeForKey(key)
		afterDistribution[targetNode] = append(afterDistribution[targetNode], key)
	}

	t.Logf("📊 移除节点后的分布:")
	for node, keys := range afterDistribution {
		t.Logf("  %s: %v", node, keys)
	}

	// 7. 验证被移除节点的数据已迁移
	if _, exists := afterDistribution[nodeToRemove]; exists {
		t.Errorf("❌ 被移除的节点 %s 仍然在分布中", nodeToRemove)
	}

	// 8. 验证数据完整性
	t.Log("🔍 验证数据完整性...")
	for key, expectedValue := range testData {
		targetNode := dc.GetNodeForKey(key)
		localCache := dc.LocalCaches[targetNode]
		
		actualValue, found := localCache.Get(key)
		if !found {
			t.Errorf("❌ 数据丢失: key=%s 应该在节点 %s", key, targetNode)
			continue
		}
		
		if actualValue != expectedValue {
			t.Errorf("❌ 数据错误: key=%s, 期望=%s, 实际=%s", key, expectedValue, actualValue)
		} else {
			t.Logf("✅ 数据正确: %s = %s (在节点 %s)", key, actualValue, targetNode)
		}
	}

	// 9. 验证迁移统计
	migrationStats := dc.GetMigrationStats()
	t.Logf("📈 迁移统计: 迁移了 %d 个key，耗时 %v", 
		migrationStats.MigratedKeys, migrationStats.Duration)

	if migrationStats.MigratedKeys != len(keysToMigrate) {
		t.Logf("⚠️  迁移数量不匹配: 期望=%d, 实际=%d", len(keysToMigrate), migrationStats.MigratedKeys)
	}

	t.Log("✅ 移除节点数据迁移测试通过")
}

// TestVirtualNodesDistribution 测试虚拟节点对数据分布的影响
func TestVirtualNodesDistribution(t *testing.T) {
	t.Log("🧪 开始测试虚拟节点数据分布...")

	// 测试不同虚拟节点数量的分布效果
	virtualNodeCounts := []int{50, 100, 150, 200}
	nodes := []string{"node1", "node2", "node3"}

	// 生成大量测试key
	testKeys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		testKeys[i] = fmt.Sprintf("key_%d", i)
	}

	for _, virtualNodes := range virtualNodeCounts {
		t.Logf("📊 测试虚拟节点数: %d", virtualNodes)

		dc := core.NewDistributedCacheWithVirtualNodes(nodes, virtualNodes)

		// 统计分布
		distribution := make(map[string]int)
		for _, key := range testKeys {
			targetNode := dc.GetNodeForKey(key)
			distribution[targetNode]++
		}

		// 计算分布均匀性
		var counts []int
		for node, count := range distribution {
			counts = append(counts, count)
			percentage := float64(count) / float64(len(testKeys)) * 100
			t.Logf("  %s: %d keys (%.1f%%)", node, count, percentage)
		}

		// 计算标准差来衡量均匀性
		sort.Ints(counts)
		min, max := counts[0], counts[len(counts)-1]
		variance := float64(max-min) / float64(len(testKeys)) * 100

		t.Logf("  分布差异: %.1f%% (最小=%d, 最大=%d)", variance, min, max)

		// 虚拟节点越多，分布应该越均匀
		if variance > 20.0 {
			t.Logf("⚠️  分布不够均匀，虚拟节点数可能需要增加")
		}
	}

	t.Log("✅ 虚拟节点分布测试完成")
}

// TestHashRingConsistency 测试哈希环的一致性
func TestHashRingConsistency(t *testing.T) {
	t.Log("🧪 开始测试哈希环一致性...")

	nodes := []string{"node1", "node2", "node3"}
	dc := core.NewDistributedCache(nodes)

	testKeys := []string{
		"user:1001", "product:2001", "order:3001", "session:abc123",
		"cache:key1", "data:item1", "temp:file1", "log:entry1",
	}

	// 1. 记录初始路由
	initialRouting := make(map[string]string)
	for _, key := range testKeys {
		targetNode := dc.GetNodeForKey(key)
		initialRouting[key] = targetNode
		t.Logf("📍 初始路由: %s -> %s", key, targetNode)
	}

	// 2. 多次查询相同key，验证路由一致性
	t.Log("🔄 验证路由一致性...")
	for i := 0; i < 10; i++ {
		for _, key := range testKeys {
			targetNode := dc.GetNodeForKey(key)
			if targetNode != initialRouting[key] {
				t.Errorf("❌ 路由不一致: key=%s, 初始=%s, 第%d次=%s",
					key, initialRouting[key], i+1, targetNode)
			}
		}
	}

	// 3. 添加节点后，验证未迁移的key路由保持不变
	t.Log("🔄 添加节点后验证路由一致性...")
	dc.AddNode("node4")

	unchangedCount := 0
	changedCount := 0

	for _, key := range testKeys {
		newTargetNode := dc.GetNodeForKey(key)
		if newTargetNode == initialRouting[key] {
			unchangedCount++
			t.Logf("✅ 路由未变: %s -> %s", key, newTargetNode)
		} else {
			changedCount++
			t.Logf("🔄 路由改变: %s: %s -> %s", key, initialRouting[key], newTargetNode)
		}
	}

	t.Logf("📊 路由变化统计: 未变=%d, 改变=%d, 变化率=%.1f%%",
		unchangedCount, changedCount, float64(changedCount)/float64(len(testKeys))*100)

	// 一致性哈希的优势：只有少部分key的路由会改变
	changeRate := float64(changedCount) / float64(len(testKeys))
	if changeRate > 0.5 {
		t.Errorf("❌ 路由变化率过高: %.1f%%, 一致性哈希效果不佳", changeRate*100)
	}

	t.Log("✅ 哈希环一致性测试通过")
}

// TestDistributedNodeIntegration 测试DistributedNode的集成功能
func TestDistributedNodeIntegration(t *testing.T) {
	t.Log("🧪 开始测试DistributedNode集成功能...")

	// 创建单节点配置（模拟本地节点）
	config := distributed.NodeConfig{
		NodeID: "test-node",
		Address: "localhost:9001",
		ClusterNodes: map[string]string{
			"test-node": "localhost:9001",
		},
		CacheSize:    1000,
		VirtualNodes: 150,
	}

	node := distributed.NewDistributedNode(config)

	// 测试本地数据操作
	testData := map[string]string{
		"local:key1": "value1",
		"local:key2": "value2",
		"local:key3": "value3",
	}

	t.Log("📝 测试本地数据操作...")
	for key, value := range testData {
		// 由于只有一个节点，所有数据都应该存储在本地
		err := node.Set(key, value)
		if err != nil {
			t.Errorf("❌ 设置数据失败: key=%s, error=%v", key, err)
			continue
		}

		retrievedValue, found, err := node.Get(key)
		if err != nil {
			t.Errorf("❌ 获取数据失败: key=%s, error=%v", key, err)
			continue
		}

		if !found {
			t.Errorf("❌ 数据未找到: key=%s", key)
			continue
		}

		if retrievedValue != value {
			t.Errorf("❌ 数据不匹配: key=%s, 期望=%s, 实际=%s", key, value, retrievedValue)
			continue
		}

		t.Logf("✅ 数据正确: %s = %s", key, retrievedValue)
	}

	// 测试统计信息
	stats := node.GetLocalStats()
	t.Logf("📊 本地缓存统计: %+v", stats)

	// 测试集群配置管理
	t.Log("🔧 测试集群配置管理...")

	// 添加虚拟节点
	node.AddClusterNode("virtual-node", "localhost:9002")

	clusterNodes := node.GetClusterNodes()
	if len(clusterNodes) != 2 {
		t.Errorf("❌ 集群节点数量错误: 期望=2, 实际=%d", len(clusterNodes))
	}

	if clusterNodes["virtual-node"] != "localhost:9002" {
		t.Errorf("❌ 虚拟节点地址错误: 期望=localhost:9002, 实际=%s", clusterNodes["virtual-node"])
	}

	// 移除虚拟节点
	node.RemoveClusterNode("virtual-node")

	clusterNodes = node.GetClusterNodes()
	if len(clusterNodes) != 1 {
		t.Errorf("❌ 移除节点后数量错误: 期望=1, 实际=%d", len(clusterNodes))
	}

	t.Log("✅ DistributedNode集成测试通过")
}
