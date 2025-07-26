package main

import (
	"fmt"
	"sync"
	"time"
)

// WriteThroughCache Write-Through模式缓存
type WriteThroughCache struct {
	cache *Cache
	db    *Database
	mu    sync.RWMutex
}

func NewWriteThroughCache() *WriteThroughCache {
	return &WriteThroughCache{
		cache: NewCache(),
		db:    NewDatabase(),
	}
}

// Get Write-Through读取
// 读取逻辑与Cache-Aside相同
func (wt *WriteThroughCache) Get(key string) (string, error) {
	fmt.Printf("\n🔍 Write-Through 读取: %s\n", key)
	
	// 1. 先查缓存
	if value, exists := wt.cache.Get(key); exists {
		return value, nil
	}
	
	// 2. 缓存未命中，查询数据库
	value, exists := wt.db.Get(key)
	if !exists {
		return "", fmt.Errorf("数据不存在: %s", key)
	}
	
	// 3. 将数据写入缓存
	wt.cache.Set(key, value)
	
	return value, nil
}

// Set Write-Through写入
// 关键：同时写入缓存和数据库，保证一致性
func (wt *WriteThroughCache) Set(key, value string) error {
	fmt.Printf("\n✏️ Write-Through 写入: %s = %s\n", key, value)
	
	wt.mu.Lock()
	defer wt.mu.Unlock()
	
	// 1. 先写入缓存
	wt.cache.Set(key, value)
	
	// 2. 同步写入数据库
	wt.db.Set(key, value)
	
	// 注意：只有两者都成功才算成功
	fmt.Println("✅ Write-Through 写入完成（缓存和数据库都已更新）")
	
	return nil
}

// SetWithRollback 带回滚的Write-Through写入
func (wt *WriteThroughCache) SetWithRollback(key, value string) error {
	fmt.Printf("\n✏️ Write-Through 写入(带回滚): %s = %s\n", key, value)
	
	wt.mu.Lock()
	defer wt.mu.Unlock()
	
	// 保存原始值用于回滚
	originalValue, hasOriginal := wt.cache.Get(key)
	
	// 1. 先写入缓存
	wt.cache.Set(key, value)
	
	// 2. 尝试写入数据库
	// 模拟数据库写入失败的情况
	if key == "fail_key" {
		fmt.Println("❌ 数据库写入失败，开始回滚")
		
		// 回滚缓存
		if hasOriginal {
			wt.cache.Set(key, originalValue)
			fmt.Printf("🔄 缓存回滚到原始值: %s\n", originalValue)
		} else {
			wt.cache.Delete(key)
			fmt.Println("🔄 缓存回滚：删除新增的键")
		}
		
		return fmt.Errorf("数据库写入失败")
	}
	
	wt.db.Set(key, value)
	fmt.Println("✅ Write-Through 写入完成")
	
	return nil
}

// Delete Write-Through删除
func (wt *WriteThroughCache) Delete(key string) error {
	fmt.Printf("\n🗑️ Write-Through 删除: %s\n", key)
	
	wt.mu.Lock()
	defer wt.mu.Unlock()
	
	// 1. 删除缓存
	wt.cache.Delete(key)
	
	// 2. 删除数据库
	wt.db.Delete(key)
	
	fmt.Println("✅ Write-Through 删除完成")
	return nil
}

// DemoWriteThrough 演示Write-Through模式
func DemoWriteThrough() {
	fmt.Println("=== Write-Through 模式演示 ===")
	
	cache := NewWriteThroughCache()
	
	// 演示写入流程
	fmt.Println("\n--- 写入流程演示 ---")
	
	start := time.Now()
	cache.Set("user:1", "张三")
	writeTime := time.Since(start)
	fmt.Printf("Write-Through 写入耗时: %v\n", writeTime)
	
	// 演示读取流程
	fmt.Println("\n--- 读取流程演示 ---")
	
	// 第一次读取：缓存命中（因为写入时已经更新了缓存）
	start = time.Now()
	value1, _ := cache.Get("user:1")
	readTime := time.Since(start)
	fmt.Printf("读取结果: %s, 耗时: %v\n", value1, readTime)
	
	// 演示数据一致性
	fmt.Println("\n--- 数据一致性验证 ---")
	
	// 更新数据
	cache.Set("user:1", "张三(已更新)")
	
	// 直接从缓存读取
	cacheValue, _ := cache.cache.Get("user:1")
	fmt.Printf("缓存中的值: %s\n", cacheValue)
	
	// 直接从数据库读取
	dbValue, _ := cache.db.Get("user:1")
	fmt.Printf("数据库中的值: %s\n", dbValue)
	
	if cacheValue == dbValue {
		fmt.Println("✅ 数据一致性验证通过")
	} else {
		fmt.Println("❌ 数据不一致")
	}
}

// DemoWriteThroughFailure 演示Write-Through失败处理
func DemoWriteThroughFailure() {
	fmt.Println("\n=== Write-Through 失败处理演示 ===")
	
	cache := NewWriteThroughCache()
	
	// 先设置一个正常值
	cache.Set("test_key", "原始值")
	fmt.Println("设置原始值完成")
	
	// 尝试设置一个会失败的值
	fmt.Println("\n尝试更新为会导致失败的值:")
	err := cache.SetWithRollback("fail_key", "这个会失败")
	if err != nil {
		fmt.Printf("预期的失败: %s\n", err.Error())
	}
	
	// 验证失败后的状态
	fmt.Println("\n验证失败后的状态:")
	_, exists := cache.cache.Get("fail_key")
	if !exists {
		fmt.Println("✅ 缓存中没有失败的数据")
	}
	
	_, exists = cache.db.Get("fail_key")
	if !exists {
		fmt.Println("✅ 数据库中没有失败的数据")
	}
	
	fmt.Println("\n💡 Write-Through模式的优势:")
	fmt.Println("   1. 强一致性：缓存和数据库始终保持一致")
	fmt.Println("   2. 简化逻辑：应用程序只需要操作缓存")
	fmt.Println("   3. 故障恢复：失败时可以回滚，保证数据完整性")
	
	fmt.Println("\n💡 Write-Through模式的劣势:")
	fmt.Println("   1. 写入延迟：需要同时写入两个存储")
	fmt.Println("   2. 可用性：缓存或数据库故障都会影响写入")
	fmt.Println("   3. 复杂性：需要处理事务和回滚逻辑")
}

// CompareWritePerformance 比较写入性能
func CompareWritePerformance() {
	fmt.Println("\n=== 写入性能对比 ===")
	
	// Cache-Aside模式
	cacheAside := NewCacheAsideService()
	
	// Write-Through模式
	writeThrough := NewWriteThroughCache()
	
	// 测试Cache-Aside写入性能
	fmt.Println("\n测试Cache-Aside写入性能:")
	start := time.Now()
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("ca_user:%d", i)
		cacheAside.Set(key, fmt.Sprintf("用户%d", i))
	}
	cacheAsideTime := time.Since(start)
	fmt.Printf("Cache-Aside 5次写入耗时: %v\n", cacheAsideTime)
	
	// 测试Write-Through写入性能
	fmt.Println("\n测试Write-Through写入性能:")
	start = time.Now()
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("wt_user:%d", i)
		writeThrough.Set(key, fmt.Sprintf("用户%d", i))
	}
	writeThroughTime := time.Since(start)
	fmt.Printf("Write-Through 5次写入耗时: %v\n", writeThroughTime)
	
	// 性能对比
	ratio := float64(writeThroughTime) / float64(cacheAsideTime)
	fmt.Printf("\n性能对比: Write-Through 比 Cache-Aside 慢 %.1fx\n", ratio)
	
	fmt.Println("\n💡 性能分析:")
	fmt.Println("   Cache-Aside: 先写数据库，再删除缓存（异步）")
	fmt.Println("   Write-Through: 同时写缓存和数据库（同步）")
	fmt.Println("   Write-Through的写入延迟更高，但数据一致性更好")
}
