package main

import (
	"fmt"
	"strings"
	"time"
)

// 性能测试结果
type PerformanceResult struct {
	Pattern   string
	WriteTime time.Duration
	ReadTime  time.Duration
	Consistency string
	DataSafety string
}

// 运行性能对比测试
func runPerformanceComparison() {
	fmt.Println("=== 三种缓存模式性能对比 ===")
	
	results := make([]PerformanceResult, 0, 3)
	
	// 测试Cache-Aside
	fmt.Println("\n--- 测试 Cache-Aside 模式 ---")
	cacheAside := NewCacheAsideService()
	
	start := time.Now()
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("ca_test:%d", i)
		cacheAside.Set(key, fmt.Sprintf("值%d", i))
	}
	caWriteTime := time.Since(start)
	
	start = time.Now()
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("ca_test:%d", i)
		cacheAside.Get(key)
	}
	caReadTime := time.Since(start)
	
	results = append(results, PerformanceResult{
		Pattern:     "Cache-Aside",
		WriteTime:   caWriteTime,
		ReadTime:    caReadTime,
		Consistency: "弱一致性",
		DataSafety:  "安全",
	})
	
	// 测试Write-Through
	fmt.Println("\n--- 测试 Write-Through 模式 ---")
	writeThrough := NewWriteThroughCache()
	
	start = time.Now()
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("wt_test:%d", i)
		writeThrough.Set(key, fmt.Sprintf("值%d", i))
	}
	wtWriteTime := time.Since(start)
	
	start = time.Now()
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("wt_test:%d", i)
		writeThrough.Get(key)
	}
	wtReadTime := time.Since(start)
	
	results = append(results, PerformanceResult{
		Pattern:     "Write-Through",
		WriteTime:   wtWriteTime,
		ReadTime:    wtReadTime,
		Consistency: "强一致性",
		DataSafety:  "安全",
	})
	
	// 测试Write-Back
	fmt.Println("\n--- 测试 Write-Back 模式 ---")
	writeBack := NewWriteBackCache(10 * time.Second)
	defer writeBack.Close()
	
	start = time.Now()
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("wb_test:%d", i)
		writeBack.Set(key, fmt.Sprintf("值%d", i))
	}
	wbWriteTime := time.Since(start)
	
	start = time.Now()
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("wb_test:%d", i)
		writeBack.Get(key)
	}
	wbReadTime := time.Since(start)
	
	results = append(results, PerformanceResult{
		Pattern:     "Write-Back",
		WriteTime:   wbWriteTime,
		ReadTime:    wbReadTime,
		Consistency: "最终一致性",
		DataSafety:  "有风险",
	})
	
	// 输出对比结果
	fmt.Println("\n=== 性能对比结果 ===")
	fmt.Printf("%-15s %-15s %-15s %-15s %-15s\n", 
		"模式", "写入耗时", "读取耗时", "一致性", "数据安全性")
	fmt.Println(strings.Repeat("-", 75))
	
	for _, result := range results {
		fmt.Printf("%-15s %-15v %-15v %-15s %-15s\n",
			result.Pattern,
			result.WriteTime,
			result.ReadTime,
			result.Consistency,
			result.DataSafety)
	}
	
	// 性能分析
	fmt.Println("\n=== 性能分析 ===")
	
	// 写入性能排序
	fmt.Println("\n📝 写入性能排序 (从快到慢):")
	if wbWriteTime < caWriteTime && wbWriteTime < wtWriteTime {
		fmt.Println("1. Write-Back (最快)")
		if caWriteTime < wtWriteTime {
			fmt.Println("2. Cache-Aside")
			fmt.Println("3. Write-Through (最慢)")
		} else {
			fmt.Println("2. Write-Through")
			fmt.Println("3. Cache-Aside (最慢)")
		}
	}
	
	// 读取性能分析
	fmt.Println("\n📖 读取性能:")
	fmt.Println("所有模式的读取性能相近（都是从缓存读取）")
	
	// 一致性分析
	fmt.Println("\n🔄 一致性分析:")
	fmt.Println("Write-Through > Cache-Aside > Write-Back")
	
	// 适用场景推荐
	fmt.Println("\n💡 适用场景推荐:")
	fmt.Println("• Cache-Aside: 通用场景，平衡性能和一致性")
	fmt.Println("• Write-Through: 强一致性要求的场景")
	fmt.Println("• Write-Back: 高写入性能要求，可容忍数据丢失")
}

// 演示一致性差异
func demonstrateConsistency() {
	fmt.Println("\n=== 一致性差异演示 ===")
	
	// Cache-Aside一致性问题
	fmt.Println("\n--- Cache-Aside 一致性问题 ---")
	ca := NewCacheAsideService()
	ca.Set("user:1", "张三")
	
	// 模拟并发更新导致的不一致
	fmt.Println("模拟并发更新可能导致的数据不一致...")
	
	// Write-Through强一致性
	fmt.Println("\n--- Write-Through 强一致性 ---")
	wt := NewWriteThroughCache()
	wt.Set("user:2", "李四")
	
	// 验证一致性
	cacheVal, _ := wt.cache.Get("user:2")
	dbVal, _ := wt.db.Get("user:2")
	fmt.Printf("缓存值: %s, 数据库值: %s\n", cacheVal, dbVal)
	if cacheVal == dbVal {
		fmt.Println("✅ Write-Through保证强一致性")
	}
	
	// Write-Back最终一致性
	fmt.Println("\n--- Write-Back 最终一致性 ---")
	wb := NewWriteBackCache(2 * time.Second)
	defer wb.Close()
	
	wb.Set("user:3", "王五")
	
	// 立即检查
	cacheVal, _ = wb.cache.Get("user:3")
	dbVal, exists := wb.db.Get("user:3")
	fmt.Printf("缓存值: %s\n", cacheVal)
	if !exists {
		fmt.Println("数据库值: (暂无，等待刷新)")
		fmt.Printf("脏数据数量: %d\n", wb.GetDirtyCount())
	}
	
	// 等待刷新
	fmt.Println("等待后台刷新...")
	time.Sleep(3 * time.Second)
	
	dbVal, _ = wb.db.Get("user:3")
	fmt.Printf("刷新后数据库值: %s\n", dbVal)
	fmt.Println("✅ Write-Back实现最终一致性")
}

// 演示故障场景
func demonstrateFailureScenarios() {
	fmt.Println("\n=== 故障场景演示 ===")
	
	// Cache-Aside缓存故障
	fmt.Println("\n--- Cache-Aside 缓存故障 ---")
	fmt.Println("缓存故障时，应用程序直接访问数据库")
	fmt.Println("✅ 服务可用性不受影响")
	
	// Write-Through缓存故障
	fmt.Println("\n--- Write-Through 缓存故障 ---")
	fmt.Println("缓存故障时，写入操作失败")
	fmt.Println("❌ 影响写入可用性")
	
	// Write-Back数据丢失
	fmt.Println("\n--- Write-Back 数据丢失风险 ---")
	fmt.Println("缓存故障时，未刷新的脏数据丢失")
	fmt.Println("⚠️ 数据安全性风险")
}

func main() {
	fmt.Println("🎮 第二章：缓存模式 - 对比演示程序")
	fmt.Println("==========================================")

	// 运行各种演示
	runPerformanceComparison()
	demonstrateConsistency()
	demonstrateFailureScenarios()
	
	fmt.Println("\n🎉 演示完成！")
	fmt.Println("\n💡 关键要点总结:")
	fmt.Println("   1. Cache-Aside: 最常用，平衡性能和复杂度")
	fmt.Println("   2. Write-Through: 强一致性，但写入性能较差")
	fmt.Println("   3. Write-Back: 最高性能，但有数据丢失风险")
	fmt.Println("   4. 选择模式要根据具体业务需求权衡")
	
	fmt.Println("\n📖 下一步: 学习第三章 - 缓存问题")
	fmt.Println("   将学习缓存雪崩、穿透、击穿等常见问题的解决方案")
}
