package main

import (
	"fmt"
	"math/rand"
	"time"
)

// 演示一致性问题和解决方案
func DemoConsistencyIssues() {
	fmt.Println("=== 一致性问题演示 ===")
	
	database := NewDatabase()
	
	// 创建多级缓存
	cache := NewMultilevelCache(&MultilevelConfig{
		L1TTL:        1 * time.Minute,
		L2TTL:        5 * time.Minute,
		L1MaxSize:    100,
		EnableL1:     true,
		EnableL2:     true,
		WriteThrough: false, // 使用写回模式演示一致性问题
	})
	
	// 1. 初始数据加载
	fmt.Println("\n1. 初始数据加载:")
	key := "user:1"
	value, _ := cache.Get(key, database)
	fmt.Printf("初始值: %s = %s\n", key, value)
	
	// 2. 直接更新数据库（模拟其他服务更新）
	fmt.Println("\n2. 模拟其他服务直接更新数据库:")
	database.Set(key, "张三(已更新)")
	fmt.Println("数据库已更新为: 张三(已更新)")
	
	// 3. 从缓存读取（会读到旧数据）
	fmt.Println("\n3. 从缓存读取:")
	cachedValue, _ := cache.Get(key, database)
	fmt.Printf("缓存读取值: %s (旧数据)\n", cachedValue)
	
	// 4. 直接从数据库读取
	fmt.Println("\n4. 直接从数据库读取:")
	dbValue, _ := database.Get(key)
	fmt.Printf("数据库实际值: %s (新数据)\n", dbValue)
	
	// 5. 解决方案：主动失效缓存
	fmt.Println("\n5. 解决方案：主动失效缓存")
	cache.Delete(key)
	fmt.Println("已删除缓存")
	
	// 6. 重新读取
	fmt.Println("\n6. 重新读取:")
	freshValue, _ := cache.Get(key, database)
	fmt.Printf("重新读取值: %s (最新数据)\n", freshValue)
	
	fmt.Println("\n💡 一致性问题解决方案:")
	fmt.Println("   1. 写入时主动删除缓存")
	fmt.Println("   2. 设置较短的TTL")
	fmt.Println("   3. 使用消息队列通知")
	fmt.Println("   4. 版本号机制")
}

// 演示热点数据处理
func DemoHotDataHandling() {
	fmt.Println("\n=== 热点数据处理演示 ===")
	
	database := NewDatabase()
	cache := NewMultilevelCache(&MultilevelConfig{
		L1TTL:        30 * time.Second, // 较短的L1 TTL
		L2TTL:        5 * time.Minute,  // 较长的L2 TTL
		L1MaxSize:    50,               // 较小的L1容量
		EnableL1:     true,
		EnableL2:     true,
		WriteThrough: true,
	})
	
	// 模拟热点数据访问
	fmt.Println("\n1. 模拟热点数据访问:")
	hotKeys := []string{"hot:1", "hot:2", "hot:3"}
	normalKeys := []string{"normal:1", "normal:2", "normal:3", "normal:4", "normal:5"}
	
	// 添加数据到数据库
	for _, key := range hotKeys {
		database.Set(key, fmt.Sprintf("热点数据_%s", key))
	}
	for _, key := range normalKeys {
		database.Set(key, fmt.Sprintf("普通数据_%s", key))
	}
	
	// 模拟访问模式：热点数据访问频率高
	fmt.Println("模拟访问模式 (热点数据访问频率高):")
	
	totalRequests := 100
	hotDataRatio := 0.8 // 80%的请求访问热点数据
	
	start := time.Now()
	for i := 0; i < totalRequests; i++ {
		var key string
		if rand.Float64() < hotDataRatio {
			// 访问热点数据
			key = hotKeys[rand.Intn(len(hotKeys))]
		} else {
			// 访问普通数据
			key = normalKeys[rand.Intn(len(normalKeys))]
		}
		cache.Get(key, database)
	}
	totalTime := time.Since(start)
	
	// 显示结果
	metrics := cache.GetMetrics()
	fmt.Printf("\n访问结果:\n")
	fmt.Printf("总请求数: %d\n", totalRequests)
	fmt.Printf("总耗时: %v\n", totalTime)
	fmt.Printf("平均延迟: %v\n", totalTime/time.Duration(totalRequests))
	fmt.Printf("L1命中率: %.2f%%\n", metrics.L1HitRate*100)
	fmt.Printf("L2命中率: %.2f%%\n", metrics.L2HitRate*100)
	fmt.Printf("总体命中率: %.2f%%\n", metrics.OverallHitRate*100)
	
	fmt.Println("\n💡 热点数据优化策略:")
	fmt.Println("   1. 热点数据优先进入L1缓存")
	fmt.Println("   2. 动态调整TTL")
	fmt.Println("   3. 预热机制")
	fmt.Println("   4. 读写分离")
}

// 演示缓存预热
func DemoCacheWarmup() {
	fmt.Println("\n=== 缓存预热演示 ===")
	
	database := NewDatabase()
	cache := NewMultilevelCache(nil)
	
	// 预热前性能测试
	fmt.Println("\n1. 预热前性能测试:")
	testKeys := []string{"user:1", "user:2", "product:1", "product:2", "order:1"}
	
	start := time.Now()
	for _, key := range testKeys {
		cache.Get(key, database)
	}
	coldStartTime := time.Since(start)
	fmt.Printf("冷启动耗时: %v\n", coldStartTime)
	
	// 清空缓存
	for _, key := range testKeys {
		cache.Delete(key)
	}
	
	// 缓存预热
	fmt.Println("\n2. 执行缓存预热:")
	warmupStart := time.Now()
	for _, key := range testKeys {
		if value, exists := database.Get(key); exists {
			// 直接写入各级缓存
			if cache.l1Cache != nil {
				cache.l1Cache.Set(key, value, cache.config.L1TTL)
			}
			if cache.l2Cache != nil {
				cache.l2Cache.Set(key, value, cache.config.L2TTL)
			}
		}
	}
	warmupTime := time.Since(warmupStart)
	fmt.Printf("预热耗时: %v\n", warmupTime)
	
	// 预热后性能测试
	fmt.Println("\n3. 预热后性能测试:")
	start = time.Now()
	for _, key := range testKeys {
		cache.Get(key, database)
	}
	warmStartTime := time.Since(start)
	fmt.Printf("热启动耗时: %v\n", warmStartTime)
	
	// 性能对比
	speedup := float64(coldStartTime) / float64(warmStartTime)
	fmt.Printf("性能提升: %.1fx\n", speedup)
	
	fmt.Println("\n💡 预热策略:")
	fmt.Println("   1. 系统启动时预热核心数据")
	fmt.Println("   2. 基于历史访问模式预热")
	fmt.Println("   3. 分批预热避免系统压力")
	fmt.Println("   4. 监控预热效果")
}

// 演示故障降级
func DemoFailoverAndDegradation() {
	fmt.Println("\n=== 故障降级演示 ===")
	
	database := NewDatabase()
	
	// 正常情况
	fmt.Println("\n1. 正常情况 (L1+L2):")
	normalCache := NewMultilevelCache(&MultilevelConfig{
		L1TTL:     2 * time.Minute,
		L2TTL:     10 * time.Minute,
		L1MaxSize: 100,
		EnableL1:  true,
		EnableL2:  true,
	})
	
	start := time.Now()
	value, _ := normalCache.Get("user:1", database)
	normalTime := time.Since(start)
	fmt.Printf("正常访问: %s, 延迟: %v\n", value, normalTime)
	
	// L1故障情况
	fmt.Println("\n2. L1缓存故障 (只有L2):")
	l2OnlyCache := NewMultilevelCache(&MultilevelConfig{
		L2TTL:    10 * time.Minute,
		EnableL1: false, // L1故障
		EnableL2: true,
	})
	
	start = time.Now()
	value, _ = l2OnlyCache.Get("user:1", database)
	l2OnlyTime := time.Since(start)
	fmt.Printf("L1故障访问: %s, 延迟: %v\n", value, l2OnlyTime)
	
	// L2故障情况
	fmt.Println("\n3. L2缓存故障 (只有L1):")
	l1OnlyCache := NewMultilevelCache(&MultilevelConfig{
		L1TTL:     2 * time.Minute,
		L1MaxSize: 100,
		EnableL1:  true,
		EnableL2:  false, // L2故障
	})
	
	start = time.Now()
	value, _ = l1OnlyCache.Get("user:1", database)
	l1OnlyTime := time.Since(start)
	fmt.Printf("L2故障访问: %s, 延迟: %v\n", value, l1OnlyTime)
	
	// 全部故障情况
	fmt.Println("\n4. 全部缓存故障 (直接数据库):")
	start = time.Now()
	value, _ = database.Get("user:1")
	dbOnlyTime := time.Since(start)
	fmt.Printf("缓存全故障: %s, 延迟: %v\n", value, dbOnlyTime)
	
	// 性能对比
	fmt.Println("\n性能对比:")
	fmt.Printf("正常情况: %v (基准)\n", normalTime)
	fmt.Printf("L1故障: %v (%.1fx慢)\n", l2OnlyTime, float64(l2OnlyTime)/float64(normalTime))
	fmt.Printf("L2故障: %v (%.1fx慢)\n", l1OnlyTime, float64(l1OnlyTime)/float64(normalTime))
	fmt.Printf("全部故障: %v (%.1fx慢)\n", dbOnlyTime, float64(dbOnlyTime)/float64(normalTime))
	
	fmt.Println("\n💡 故障降级策略:")
	fmt.Println("   1. 自动检测缓存故障")
	fmt.Println("   2. 动态调整缓存配置")
	fmt.Println("   3. 熔断机制保护数据库")
	fmt.Println("   4. 监控和告警")
}

func main() {
	fmt.Println("🎮 第五章：多级缓存 - 综合演示程序")
	fmt.Println("==========================================")
	
	// 设置随机种子
	rand.Seed(time.Now().UnixNano())
	
	// 基础功能演示
	DemoLocalCache()
	DemoDistributedCache()
	DemoMultilevelCache()
	DemoConfigComparison()
	
	// 高级特性演示
	DemoConsistencyIssues()
	DemoHotDataHandling()
	DemoCacheWarmup()
	DemoFailoverAndDegradation()
	
	fmt.Println("\n🎉 演示完成！")
	fmt.Println("==========================================")
	
	fmt.Println("\n💡 关键要点总结:")
	
	fmt.Println("\n🏗️ 多级缓存架构:")
	fmt.Println("   L1: 本地缓存 - 极低延迟，小容量")
	fmt.Println("   L2: 分布式缓存 - 低延迟，大容量")
	fmt.Println("   L3: 数据库 - 高延迟，持久化")
	
	fmt.Println("\n⚡ 性能优势:")
	fmt.Println("   1. L1缓存提供纳秒级访问")
	fmt.Println("   2. L2缓存提供毫秒级访问")
	fmt.Println("   3. 自动数据回写和同步")
	fmt.Println("   4. 整体性能提升10-100倍")
	
	fmt.Println("\n🔄 一致性保证:")
	fmt.Println("   1. 写入时主动失效缓存")
	fmt.Println("   2. 设置合理的TTL策略")
	fmt.Println("   3. 使用消息队列通知")
	fmt.Println("   4. 容忍最终一致性")
	
	fmt.Println("\n🛡️ 高可用设计:")
	fmt.Println("   1. 缓存故障自动降级")
	fmt.Println("   2. 多级备份保证可用性")
	fmt.Println("   3. 熔断机制保护数据库")
	fmt.Println("   4. 实时监控和告警")
	
	fmt.Println("\n🎯 面试重点:")
	fmt.Println("   1. 能设计多级缓存架构")
	fmt.Println("   2. 理解各层级的特点和作用")
	fmt.Println("   3. 掌握一致性问题的解决方案")
	fmt.Println("   4. 了解性能优化和故障处理")
	
	fmt.Println("\n📖 下一步: 学习第六章 - 面试题集")
	fmt.Println("   将学习50+常见缓存面试题和标准答案")
}
