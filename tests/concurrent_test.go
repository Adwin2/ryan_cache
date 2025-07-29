package tests

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"tdd-learning/distributed"
)

// TestConcurrentOperations 测试并发操作的安全性
func TestConcurrentOperations(t *testing.T) {
	// 创建测试节点配置
	config := distributed.NodeConfig{
		NodeID: "test-node",
		Address: "localhost:9001",
		ClusterNodes: map[string]string{
			"test-node": "localhost:9001",
			"node2":     "localhost:9002",
			"node3":     "localhost:9003",
		},
		CacheSize:    1000,
		VirtualNodes: 150,
	}

	// 创建分布式节点
	node := distributed.NewDistributedNode(config)

	// 并发测试参数
	numGoroutines := 100
	numOperations := 50
	
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	// 测试1: 并发Set操作
	t.Run("ConcurrentSet", func(t *testing.T) {
		wg.Add(numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()
				
				for j := 0; j < numOperations; j++ {
					key := fmt.Sprintf("key_%d_%d", goroutineID, j)
					value := fmt.Sprintf("value_%d_%d", goroutineID, j)
					
					if err := node.Set(key, value); err != nil {
						errors <- fmt.Errorf("Set失败 [%d,%d]: %v", goroutineID, j, err)
					}
				}
			}(i)
		}
		
		wg.Wait()
		close(errors)
		
		// 检查错误
		var errorCount int
		for err := range errors {
			t.Logf("错误: %v", err)
			errorCount++
		}
		
		if errorCount > 0 {
			t.Errorf("并发Set操作出现 %d 个错误", errorCount)
		}
		
		t.Logf("✅ 并发Set测试完成: %d个goroutine，每个%d次操作", numGoroutines, numOperations)
	})

	// 重置错误通道
	errors = make(chan error, numGoroutines*numOperations)

	// 测试2: 并发Get操作
	t.Run("ConcurrentGet", func(t *testing.T) {
		wg.Add(numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()
				
				for j := 0; j < numOperations; j++ {
					key := fmt.Sprintf("key_%d_%d", goroutineID, j)
					
					_, _, err := node.Get(key)
					if err != nil {
						errors <- fmt.Errorf("Get失败 [%d,%d]: %v", goroutineID, j, err)
					}
				}
			}(i)
		}
		
		wg.Wait()
		close(errors)
		
		// 检查错误
		var errorCount int
		for err := range errors {
			t.Logf("错误: %v", err)
			errorCount++
		}
		
		if errorCount > 0 {
			t.Errorf("并发Get操作出现 %d 个错误", errorCount)
		}
		
		t.Logf("✅ 并发Get测试完成: %d个goroutine，每个%d次操作", numGoroutines, numOperations)
	})

	// 重置错误通道
	errors = make(chan error, numGoroutines*numOperations)

	// 测试3: 并发集群配置更新
	t.Run("ConcurrentClusterUpdate", func(t *testing.T) {
		wg.Add(numGoroutines)
		
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()
				
				for j := 0; j < 10; j++ { // 减少操作次数，因为这是重操作
					// 模拟添加和移除节点
					nodeID := fmt.Sprintf("temp_node_%d_%d", goroutineID, j)
					address := fmt.Sprintf("localhost:%d", 10000+goroutineID*100+j)
					
					// 添加节点
					node.AddClusterNode(nodeID, address)
					
					// 短暂等待
					time.Sleep(time.Millisecond)
					
					// 移除节点
					node.RemoveClusterNode(nodeID)
				}
			}(i)
		}
		
		wg.Wait()
		
		t.Logf("✅ 并发集群配置更新测试完成")
	})

	// 测试4: 混合并发操作
	t.Run("MixedConcurrentOperations", func(t *testing.T) {
		wg.Add(numGoroutines * 3) // Set, Get, Delete各一组
		
		// 并发Set
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					key := fmt.Sprintf("mixed_key_%d_%d", goroutineID, j)
					value := fmt.Sprintf("mixed_value_%d_%d", goroutineID, j)
					node.Set(key, value)
				}
			}(i)
		}
		
		// 并发Get
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					key := fmt.Sprintf("mixed_key_%d_%d", goroutineID, j)
					node.Get(key)
				}
			}(i)
		}
		
		// 并发Delete
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					key := fmt.Sprintf("mixed_key_%d_%d", goroutineID, j)
					node.Delete(key)
				}
			}(i)
		}
		
		wg.Wait()
		
		t.Logf("✅ 混合并发操作测试完成")
	})
}

// TestRaceConditionDetection 使用Go race detector检测竞态条件
func TestRaceConditionDetection(t *testing.T) {
	config := distributed.NodeConfig{
		NodeID: "race-test-node",
		Address: "localhost:9004",
		ClusterNodes: map[string]string{
			"race-test-node": "localhost:9004",
		},
		CacheSize:    100,
		VirtualNodes: 50,
	}

	node := distributed.NewDistributedNode(config)

	// 快速并发操作，容易触发竞态条件
	var wg sync.WaitGroup
	numGoroutines := 50
	
	wg.Add(numGoroutines * 2)
	
	// 并发读写集群配置
	for i := 0; i < numGoroutines; i++ {
		// 写操作
		go func(id int) {
			defer wg.Done()
			nodeID := fmt.Sprintf("node_%d", id)
			address := fmt.Sprintf("localhost:%d", 8000+id)
			node.AddClusterNode(nodeID, address)
		}(i)
		
		// 读操作
		go func(id int) {
			defer wg.Done()
			_ = node.GetClusterNodes()
		}(i)
	}
	
	wg.Wait()
	
	t.Logf("✅ 竞态条件检测测试完成（使用 go test -race 运行以检测竞态条件）")
}

// BenchmarkConcurrentOperations 并发操作性能基准测试
func BenchmarkConcurrentOperations(b *testing.B) {
	config := distributed.NodeConfig{
		NodeID: "bench-node",
		Address: "localhost:9005",
		ClusterNodes: map[string]string{
			"bench-node": "localhost:9005",
		},
		CacheSize:    10000,
		VirtualNodes: 150,
	}

	node := distributed.NewDistributedNode(config)

	b.ResetTimer()
	
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench_key_%d", i)
			value := fmt.Sprintf("bench_value_%d", i)
			
			// 执行Set和Get操作
			node.Set(key, value)
			node.Get(key)
			
			i++
		}
	})
}
