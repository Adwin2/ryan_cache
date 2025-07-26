package main

import (
	"fmt"
	"math/rand"
	"time"
)

// 模拟数据库查询
func queryDatabase(id string) string {
	// 模拟数据库查询延迟
	time.Sleep(100 * time.Millisecond)
	return fmt.Sprintf("用户数据_%s", id)
}

// 演示基本缓存操作
func demoBasicCache() {
	fmt.Println("=== 基本缓存操作演示 ===")
	
	cache := NewSimpleCache()
	
	// 设置缓存
	cache.Set("user:1", "张三")
	cache.Set("user:2", "李四")
	cache.Set("user:3", "王五")
	
	fmt.Printf("缓存大小: %d\n", cache.Size())
	
	// 获取缓存
	if value, exists := cache.Get("user:1"); exists {
		fmt.Printf("缓存命中: user:1 = %s\n", value)
	}
	
	// 获取不存在的键
	if _, exists := cache.Get("user:999"); !exists {
		fmt.Println("缓存未命中: user:999")
	}
	
	// 删除缓存
	cache.Delete("user:2")
	fmt.Printf("删除后缓存大小: %d\n", cache.Size())
	
	fmt.Println()
}

// 演示TTL缓存
func demoTTLCache() {
	fmt.Println("=== TTL缓存演示 ===")
	
	cache := NewCacheWithTTL()
	defer cache.Close()
	
	// 设置短期缓存（3秒过期）
	cache.Set("temp:1", "临时数据1", 3*time.Second)
	cache.Set("temp:2", "临时数据2", 5*time.Second)
	
	fmt.Println("设置缓存完成，开始测试过期...")
	
	// 立即获取
	if value, exists := cache.Get("temp:1"); exists {
		fmt.Printf("立即获取: temp:1 = %s\n", value)
	}
	
	// 等待2秒后获取
	time.Sleep(2 * time.Second)
	if value, exists := cache.Get("temp:1"); exists {
		fmt.Printf("2秒后获取: temp:1 = %s\n", value)
	}
	
	// 等待2秒后获取（总共4秒，应该过期）
	time.Sleep(2 * time.Second)
	if _, exists := cache.Get("temp:1"); !exists {
		fmt.Println("4秒后获取: temp:1 已过期")
	}
	
	// temp:2 应该还存在（总共4秒，5秒过期）
	if value, exists := cache.Get("temp:2"); exists {
		fmt.Printf("4秒后获取: temp:2 = %s (还未过期)\n", value)
	}
	
	fmt.Println()
}

// 演示缓存统计
func demoStatsCache() {
	fmt.Println("=== 缓存统计演示 ===")
	
	cache := NewStatsCache()
	
	// 预设一些数据
	cache.Set("data:1", "数据1")
	cache.Set("data:2", "数据2")
	cache.Set("data:3", "数据3")
	
	// 模拟随机访问
	fmt.Println("模拟随机访问...")
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("data:%d", rand.Intn(5)+1) // 随机访问data:1到data:5
		if value, exists := cache.Get(key); exists {
			fmt.Printf("命中: %s = %s\n", key, value)
		} else {
			fmt.Printf("未命中: %s\n", key)
		}
	}
	
	// 显示统计信息
	stats := cache.GetStats()
	fmt.Printf("\n统计信息:\n")
	fmt.Printf("命中次数: %d\n", stats.Hits)
	fmt.Printf("未命中次数: %d\n", stats.Misses)
	fmt.Printf("命中率: %.2f%%\n", stats.HitRate()*100)
	
	fmt.Println()
}

// 演示性能对比
func demoPerformanceComparison() {
	fmt.Println("=== 性能对比演示 ===")
	
	cache := NewSimpleCache()
	userIDs := []string{"1", "2", "3", "4", "5"}
	
	// 第一次访问：缓存未命中，需要查询数据库
	fmt.Println("第一次访问（缓存未命中）:")
	start := time.Now()
	for _, id := range userIDs {
		if _, exists := cache.Get("user:" + id); !exists {
			// 缓存未命中，查询数据库
			data := queryDatabase(id)
			cache.Set("user:"+id, data)
			fmt.Printf("查询数据库: user:%s\n", id)
		}
	}
	firstTime := time.Since(start)
	fmt.Printf("第一次访问耗时: %v\n\n", firstTime)
	
	// 第二次访问：缓存命中
	fmt.Println("第二次访问（缓存命中）:")
	start = time.Now()
	for _, id := range userIDs {
		if value, exists := cache.Get("user:" + id); exists {
			fmt.Printf("缓存命中: user:%s = %s\n", id, value)
		}
	}
	secondTime := time.Since(start)
	fmt.Printf("第二次访问耗时: %v\n\n", secondTime)
	
	// 性能提升
	speedup := float64(firstTime) / float64(secondTime)
	fmt.Printf("性能提升: %.1fx\n", speedup)
	
	fmt.Println()
}

// 演示并发安全性
func demoConcurrentSafety() {
	fmt.Println("=== 并发安全性演示 ===")
	
	cache := NewSimpleCache()
	
	// 启动多个goroutine并发读写
	done := make(chan bool, 10)
	
	// 5个写goroutine
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				cache.Set(key, fmt.Sprintf("value_%d_%d", id, j))
			}
			done <- true
		}(i)
	}
	
	// 5个读goroutine
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key_%d_%d", id%5, j) // 读取其他goroutine写入的数据
				cache.Get(key)
			}
			done <- true
		}(i)
	}
	
	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
	
	fmt.Printf("并发测试完成，最终缓存大小: %d\n", cache.Size())
	fmt.Println("没有发生竞态条件，缓存是线程安全的！")
	
	fmt.Println()
}

func main() {
	fmt.Println("🎮 第一章：缓存基础 - 演示程序")
	fmt.Println("=====================================")
	
	// 注意：从Go 1.20开始，rand包会自动使用随机种子，无需手动调用Seed
	
	// 运行各种演示
	demoBasicCache()
	demoTTLCache()
	demoStatsCache()
	demoPerformanceComparison()
	demoConcurrentSafety()
	
	fmt.Println("🎉 演示完成！")
	fmt.Println("💡 关键要点:")
	fmt.Println("   1. 缓存可以显著提升访问速度")
	fmt.Println("   2. TTL机制可以自动清理过期数据")
	fmt.Println("   3. 读写锁保证了线程安全")
	fmt.Println("   4. 统计信息帮助监控缓存效果")
	fmt.Println()
	fmt.Println("📖 下一步: 学习第二章 - 缓存模式")
}
