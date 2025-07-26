package main

import (
	"fmt"
	"sort"

	"tdd-learning/core"
)

// HashRingAnalyzer 哈希环分析器
type HashRingAnalyzer struct {
	testData map[string]string
}

func main() {
	fmt.Println("🔍 一致性哈希环虚拟节点分布分析")
	fmt.Println("=====================================")

	analyzer := &HashRingAnalyzer{
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
	}

	// 分析添加节点前后的变化
	analyzer.analyzeNodeAddition()
}

// analyzeNodeAddition 分析添加节点的过程
func (a *HashRingAnalyzer) analyzeNodeAddition() {
	fmt.Println("\n📊 步骤1: 创建初始2节点集群")
	
	// 创建初始2节点集群
	initialNodes := []string{"node1", "node2"}
	initialDC := core.NewDistributedCacheWithVirtualNodes(initialNodes, 150)
	
	// 分析初始数据分布
	fmt.Println("\n🔍 初始哈希环分析:")
	a.analyzeHashRing(initialDC, "初始状态 (node1, node2)")
	
	// 设置测试数据并分析分布
	initialDistribution := a.analyzeDataDistribution(initialDC, "初始数据分布")
	
	fmt.Println("\n📊 步骤2: 添加新节点node4")
	
	// 添加新节点
	err := initialDC.AddNode("node4")
	if err != nil {
		fmt.Printf("❌ 添加节点失败: %v\n", err)
		return
	}
	
	// 分析添加节点后的哈希环
	fmt.Println("\n🔍 添加节点后哈希环分析:")
	a.analyzeHashRing(initialDC, "添加node4后")
	
	// 分析新的数据分布
	finalDistribution := a.analyzeDataDistribution(initialDC, "迁移后数据分布")
	
	// 分析数据迁移详情
	fmt.Println("\n📈 步骤3: 数据迁移详细分析")
	a.analyzeMigrationDetails(initialDistribution, finalDistribution)
	
	// 解释虚拟节点原理
	fmt.Println("\n💡 步骤4: 虚拟节点原理解释")
	a.explainVirtualNodePrinciple()
}

// analyzeHashRing 分析哈希环结构
func (a *HashRingAnalyzer) analyzeHashRing(dc *core.DistributedCache, title string) {
	fmt.Printf("  📍 %s:\n", title)
	
	// 统计每个节点的虚拟节点数量
	virtualNodeCount := make(map[string]int)
	for _, node := range dc.HashRing {
		virtualNodeCount[node]++
	}
	
	fmt.Printf("    总虚拟节点数: %d\n", len(dc.SortedHashes))
	for node, count := range virtualNodeCount {
		fmt.Printf("    %s: %d 个虚拟节点\n", node, count)
	}
	
	// 显示哈希环上的分布情况（简化版）
	fmt.Println("    哈希环分布示例 (前10个虚拟节点):")
	for i := 0; i < 10 && i < len(dc.SortedHashes); i++ {
		hash := dc.SortedHashes[i]
		node := dc.HashRing[hash]
		fmt.Printf("      哈希值 %10d -> %s\n", hash, node)
	}
}

// analyzeDataDistribution 分析数据分布
func (a *HashRingAnalyzer) analyzeDataDistribution(dc *core.DistributedCache, title string) map[string][]string {
	fmt.Printf("\n  📊 %s:\n", title)
	
	distribution := make(map[string][]string)
	
	for key := range a.testData {
		targetNode := dc.GetNodeForKey(key)
		distribution[targetNode] = append(distribution[targetNode], key)
	}
	
	// 显示分布统计
	for node, keys := range distribution {
		fmt.Printf("    %s: %d 个数据项\n", node, len(keys))
		for _, key := range keys {
			fmt.Printf("      - %s\n", key)
		}
	}
	
	return distribution
}

// analyzeMigrationDetails 分析迁移详情
func (a *HashRingAnalyzer) analyzeMigrationDetails(before, after map[string][]string) {
	fmt.Println("  🔄 数据迁移详细分析:")
	
	// 分析每个节点的变化
	allNodes := make(map[string]bool)
	for node := range before {
		allNodes[node] = true
	}
	for node := range after {
		allNodes[node] = true
	}
	
	for node := range allNodes {
		beforeCount := len(before[node])
		afterCount := len(after[node])
		change := afterCount - beforeCount
		
		if change > 0 {
			fmt.Printf("    %s: %d → %d (+%d) 📈\n", node, beforeCount, afterCount, change)
		} else if change < 0 {
			fmt.Printf("    %s: %d → %d (%d) 📉\n", node, beforeCount, afterCount, change)
		} else {
			fmt.Printf("    %s: %d → %d (无变化) ➡️\n", node, beforeCount, afterCount)
		}
	}
	
	// 分析具体迁移的数据
	fmt.Println("\n  📋 具体迁移的数据:")
	for key := range a.testData {
		beforeNode := ""
		afterNode := ""
		
		// 找到数据在迁移前后的位置
		for node, keys := range before {
			for _, k := range keys {
				if k == key {
					beforeNode = node
					break
				}
			}
		}
		
		for node, keys := range after {
			for _, k := range keys {
				if k == key {
					afterNode = node
					break
				}
			}
		}
		
		if beforeNode != afterNode {
			fmt.Printf("    %s: %s → %s 🔄\n", key, beforeNode, afterNode)
		}
	}
}

// explainVirtualNodePrinciple 解释虚拟节点原理
func (a *HashRingAnalyzer) explainVirtualNodePrinciple() {
	fmt.Println("  🎯 为什么三个节点的数据都会发生变化？")
	fmt.Println()
	
	fmt.Println("  📚 一致性哈希 + 虚拟节点原理:")
	fmt.Println("    1. 每个物理节点在哈希环上有150个虚拟节点")
	fmt.Println("    2. 虚拟节点通过哈希函数随机分布在环上")
	fmt.Println("    3. 数据根据key的哈希值路由到顺时针最近的虚拟节点")
	fmt.Println()
	
	fmt.Println("  🔄 添加新节点时的影响:")
	fmt.Println("    1. 新节点的150个虚拟节点插入到哈希环的各个位置")
	fmt.Println("    2. 每个新虚拟节点都会'接管'一段数据范围")
	fmt.Println("    3. 这些数据范围原本属于不同的现有节点")
	fmt.Println("    4. 因此新节点会从多个现有节点'抢走'数据")
	fmt.Println()
	
	fmt.Println("  📊 具体示例:")
	fmt.Println("    假设哈希环范围是 0-1000:")
	fmt.Println("    - node1的虚拟节点可能在: 50, 200, 350, 600, 800...")
	fmt.Println("    - node2的虚拟节点可能在: 100, 250, 400, 700, 900...")
	fmt.Println("    - 添加node4后，其虚拟节点插入: 75, 175, 300, 650, 850...")
	fmt.Println("    - 原本路由到node1(200)的数据现在路由到node4(175)")
	fmt.Println("    - 原本路由到node2(250)的数据现在路由到node4(300)")
	fmt.Println()
	
	fmt.Println("  ✅ 这种设计的优势:")
	fmt.Println("    1. 数据分布更均匀")
	fmt.Println("    2. 避免数据热点")
	fmt.Println("    3. 最小化数据迁移量")
	fmt.Println("    4. 负载均衡效果更好")
	
	// 创建一个简化的示例来演示
	a.demonstrateSimpleExample()
}

// demonstrateSimpleExample 演示简化示例
func (a *HashRingAnalyzer) demonstrateSimpleExample() {
	fmt.Println("\n  🎭 简化演示 (每个节点只有3个虚拟节点):")
	
	// 创建简化的分布式缓存
	simpleDC := core.NewDistributedCacheWithVirtualNodes([]string{"node1", "node2"}, 3)
	
	// 显示虚拟节点分布
	fmt.Println("    初始虚拟节点分布:")
	sortedHashes := make([]uint32, len(simpleDC.SortedHashes))
	copy(sortedHashes, simpleDC.SortedHashes)
	sort.Slice(sortedHashes, func(i, j int) bool {
		return sortedHashes[i] < sortedHashes[j]
	})
	
	for i, hash := range sortedHashes {
		node := simpleDC.HashRing[hash]
		fmt.Printf("      %d. 哈希值 %10d -> %s\n", i+1, hash, node)
	}
	
	// 添加新节点
	simpleDC.AddNode("node4")
	
	fmt.Println("\n    添加node4后的虚拟节点分布:")
	newSortedHashes := make([]uint32, len(simpleDC.SortedHashes))
	copy(newSortedHashes, simpleDC.SortedHashes)
	sort.Slice(newSortedHashes, func(i, j int) bool {
		return newSortedHashes[i] < newSortedHashes[j]
	})
	
	for i, hash := range newSortedHashes {
		node := simpleDC.HashRing[hash]
		isNew := true
		for _, oldHash := range sortedHashes {
			if oldHash == hash {
				isNew = false
				break
			}
		}
		
		if isNew {
			fmt.Printf("      %d. 哈希值 %10d -> %s ⭐ (新增)\n", i+1, hash, node)
		} else {
			fmt.Printf("      %d. 哈希值 %10d -> %s\n", i+1, hash, node)
		}
	}
	
	fmt.Println("\n  💡 从这个简化示例可以看出:")
	fmt.Println("    - 新节点的虚拟节点插入到了不同位置")
	fmt.Println("    - 每个新虚拟节点都会影响其前一个虚拟节点的数据范围")
	fmt.Println("    - 因此多个原有节点的数据都会被重新分配")
}