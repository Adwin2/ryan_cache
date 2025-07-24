package main

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// 模拟数据库
type Database struct {
	data      map[string]string
	queryCount int64 // 查询计数
	mu        sync.RWMutex
}

func NewDatabase() *Database {
	db := &Database{
		data: make(map[string]string),
	}
	
	// 初始化一些数据
	for i := 1; i <= 100; i++ {
		key := fmt.Sprintf("user:%d", i)
		value := fmt.Sprintf("用户%d的数据", i)
		db.data[key] = value
	}
	
	return db
}

func (db *Database) Query(key string) (string, bool) {
	// 增加查询计数
	atomic.AddInt64(&db.queryCount, 1)
	
	// 模拟数据库查询延迟
	time.Sleep(50 * time.Millisecond)
	
	db.mu.RLock()
	defer db.mu.RUnlock()
	
	value, exists := db.data[key]
	fmt.Printf("📀 数据库查询: %s (总查询数: %d)\n", key, atomic.LoadInt64(&db.queryCount))
	return value, exists
}

func (db *Database) GetQueryCount() int64 {
	return atomic.LoadInt64(&db.queryCount)
}

func (db *Database) ResetQueryCount() {
	atomic.StoreInt64(&db.queryCount, 0)
}

// 简单缓存（容易发生雪崩）
type SimpleCache struct {
	data map[string]CacheItem
	mu   sync.RWMutex
}

type CacheItem struct {
	Value     string
	ExpiresAt time.Time
}

func NewSimpleCache() *SimpleCache {
	return &SimpleCache{
		data: make(map[string]CacheItem),
	}
}

func (c *SimpleCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, exists := c.data[key]
	if !exists {
		return "", false
	}
	
	// 检查是否过期
	if time.Now().After(item.ExpiresAt) {
		return "", false
	}
	
	return item.Value, true
}

func (c *SimpleCache) Set(key, value string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.data[key] = CacheItem{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (c *SimpleCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = make(map[string]CacheItem)
}

// 防雪崩缓存（随机TTL）
type AntiAvalancheCache struct {
	data map[string]CacheItem
	mu   sync.RWMutex
}

func NewAntiAvalancheCache() *AntiAvalancheCache {
	return &AntiAvalancheCache{
		data: make(map[string]CacheItem),
	}
}

func (c *AntiAvalancheCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, exists := c.data[key]
	if !exists {
		return "", false
	}
	
	if time.Now().After(item.ExpiresAt) {
		return "", false
	}
	
	return item.Value, true
}

func (c *AntiAvalancheCache) Set(key, value string, baseTTL time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 关键：添加随机时间，避免同时过期
	randomOffset := time.Duration(rand.Intn(int(baseTTL.Seconds()/2))) * time.Second
	actualTTL := baseTTL + randomOffset
	
	c.data[key] = CacheItem{
		Value:     value,
		ExpiresAt: time.Now().Add(actualTTL),
	}
	
	fmt.Printf("⚡ 缓存设置: %s, TTL: %v (基础: %v + 随机: %v)\n", 
		key, actualTTL, baseTTL, randomOffset)
}

// 多级缓存（L1本地 + L2分布式）
type MultiLevelCache struct {
	l1Cache *SimpleCache  // 本地缓存
	l2Cache *SimpleCache  // 模拟分布式缓存
	db      *Database
	mu      sync.RWMutex
}

func NewMultiLevelCache(db *Database) *MultiLevelCache {
	return &MultiLevelCache{
		l1Cache: NewSimpleCache(),
		l2Cache: NewSimpleCache(),
		db:      db,
	}
}

func (mlc *MultiLevelCache) Get(key string) (string, error) {
	// 1. 先查L1缓存
	if value, exists := mlc.l1Cache.Get(key); exists {
		fmt.Printf("⚡ L1缓存命中: %s\n", key)
		return value, nil
	}
	
	// 2. 再查L2缓存
	if value, exists := mlc.l2Cache.Get(key); exists {
		fmt.Printf("🌐 L2缓存命中: %s\n", key)
		// 回写到L1缓存
		mlc.l1Cache.Set(key, value, 30*time.Second)
		return value, nil
	}

	// 3. 最后查数据库
	value, exists := mlc.db.Query(key)
	if !exists {
		return "", fmt.Errorf("数据不存在: %s", key)
	}
	
	// 4. 写入多级缓存
	mlc.l1Cache.Set(key, value, 30*time.Second)
	mlc.l2Cache.Set(key, value, 300*time.Second)
	
	return value, nil
}

// 演示缓存雪崩问题
func DemoAvalancheProblem() {
	fmt.Println("=== 缓存雪崩问题演示 ===")
	
	db := NewDatabase()
	cache := NewSimpleCache()
	
	// 1. 预热缓存 - 所有数据设置相同的TTL（这是问题所在）
	fmt.Println("\n1. 预热缓存（所有数据相同TTL）:")
	baseTTL := 3 * time.Second
	for i := 1; i <= 10; i++ {
		key := fmt.Sprintf("user:%d", i)
		value := fmt.Sprintf("用户%d的数据", i)
		cache.Set(key, value, baseTTL)
	}
	fmt.Printf("预热完成，所有数据将在 %v 后同时过期\n", baseTTL)
	
	// 2. 正常访问期间
	fmt.Println("\n2. 正常访问期间:")
	db.ResetQueryCount()
	
	for i := 1; i <= 5; i++ {
		key := fmt.Sprintf("user:%d", i)
		if value, exists := cache.Get(key); exists {
			fmt.Printf("⚡ 缓存命中: %s = %s\n", key, value)
		}
	}
	fmt.Printf("正常期间数据库查询次数: %d\n", db.GetQueryCount())
	
	// 3. 等待缓存过期
	fmt.Println("\n3. 等待缓存过期...")
	time.Sleep(baseTTL + 500*time.Millisecond)
	
	// 4. 雪崩发生 - 模拟大量并发请求
	fmt.Println("\n4. 雪崩发生 - 大量并发请求:")
	db.ResetQueryCount()
	
	var wg sync.WaitGroup
	start := time.Now()
	
	// 模拟50个并发请求
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			key := fmt.Sprintf("user:%d", (id%10)+1)
			
			// 尝试从缓存获取
			if _, exists := cache.Get(key); exists {
				fmt.Printf("⚡ 缓存命中: %s\n", key)
			} else {
				// 缓存未命中，查询数据库
				if value, exists := db.Query(key); exists {
					cache.Set(key, value, baseTTL)
				}
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	fmt.Printf("\n雪崩结果:\n")
	fmt.Printf("- 并发请求数: 50\n")
	fmt.Printf("- 数据库查询次数: %d\n", db.GetQueryCount())
	fmt.Printf("- 总耗时: %v\n", duration)
	fmt.Printf("- 平均响应时间: %v\n", duration/50)
	
	fmt.Println("\n💥 问题分析:")
	fmt.Println("   所有缓存同时过期，导致大量请求直接打到数据库")
	fmt.Println("   数据库压力激增，响应时间变长")
}

// 演示防雪崩解决方案
func DemoAvalancheSolution() {
	fmt.Println("\n=== 防雪崩解决方案演示 ===")
	
	db := NewDatabase()
	
	// 解决方案1: 随机TTL
	fmt.Println("\n--- 解决方案1: 随机TTL ---")
	antiCache := NewAntiAvalancheCache()
	
	// 预热缓存 - 使用随机TTL
	fmt.Println("预热缓存（随机TTL）:")
	baseTTL := 5 * time.Second
	for i := 1; i <= 10; i++ {
		key := fmt.Sprintf("user:%d", i)
		value := fmt.Sprintf("用户%d的数据", i)
		antiCache.Set(key, value, baseTTL)
	}
	
	// 等待一段时间，观察过期情况
	fmt.Println("\n观察缓存过期情况:")
	for t := 0; t < 8; t++ {
		time.Sleep(1 * time.Second)
		
		hitCount := 0
		for i := 1; i <= 10; i++ {
			key := fmt.Sprintf("user:%d", i)
			if _, exists := antiCache.Get(key); exists {
				hitCount++
			}
		}
		fmt.Printf("第%d秒: 缓存命中数 %d/10\n", t+1, hitCount)
	}
	
	fmt.Println("\n✅ 随机TTL效果:")
	fmt.Println("   缓存逐渐过期，避免了同时失效")
	fmt.Println("   数据库压力分散，系统更稳定")
	
	// 解决方案2: 多级缓存
	fmt.Println("\n--- 解决方案2: 多级缓存 ---")
	mlCache := NewMultiLevelCache(db)
	
	// 预热多级缓存
	fmt.Println("预热多级缓存:")
	for i := 1; i <= 5; i++ {
		key := fmt.Sprintf("user:%d", i)
		mlCache.Get(key) // 这会触发数据库查询并缓存到L1和L2
	}
	
	fmt.Println("\n测试多级缓存访问:")
	db.ResetQueryCount()
	
	// 清空L1缓存，模拟L1故障
	mlCache.l1Cache.Clear()
	fmt.Println("模拟L1缓存故障（清空）")
	
	// 访问数据
	for i := 1; i <= 3; i++ {
		key := fmt.Sprintf("user:%d", i)
		value, _ := mlCache.Get(key)
		fmt.Printf("获取数据: %s = %s\n", key, value)
	}
	
	fmt.Printf("数据库查询次数: %d\n", db.GetQueryCount())
	
	fmt.Println("\n✅ 多级缓存效果:")
	fmt.Println("   L1故障时，L2缓存继续提供服务")
	fmt.Println("   避免了直接访问数据库")
	fmt.Println("   提高了系统的可用性")
}

// 演示熔断降级
func DemoCircuitBreaker() {
	fmt.Println("\n--- 解决方案3: 熔断降级 ---")
	
	// 简单的熔断器实现
	type CircuitBreaker struct {
		failureCount    int
		failureThreshold int
		state          string // "CLOSED", "OPEN", "HALF_OPEN"
		lastFailTime   time.Time
		timeout        time.Duration
		mu             sync.RWMutex
	}
	
	cb := &CircuitBreaker{
		failureThreshold: 3,
		state:           "CLOSED",
		timeout:         5 * time.Second,
	}
	
	queryWithCircuitBreaker := func(key string) (string, error) {
		cb.mu.Lock()
		defer cb.mu.Unlock()
		
		// 检查熔断器状态
		if cb.state == "OPEN" {
			if time.Since(cb.lastFailTime) > cb.timeout {
				cb.state = "HALF_OPEN"
				fmt.Println("🔄 熔断器进入半开状态")
			} else {
				fmt.Println("⚡ 熔断器开启，返回默认值")
				return "默认用户数据", nil
			}
		}
		
		// 模拟数据库查询（可能失败）
		if rand.Float32() < 0.7 { // 70%概率失败，模拟数据库压力大
			cb.failureCount++
			cb.lastFailTime = time.Now()
			
			if cb.failureCount >= cb.failureThreshold {
				cb.state = "OPEN"
				fmt.Printf("💥 数据库查询失败，熔断器开启 (失败次数: %d)\n", cb.failureCount)
			} else {
				fmt.Printf("❌ 数据库查询失败 (失败次数: %d)\n", cb.failureCount)
			}
			return "", fmt.Errorf("数据库查询失败")
		}
		
		// 查询成功
		cb.failureCount = 0
		if cb.state == "HALF_OPEN" {
			cb.state = "CLOSED"
			fmt.Println("✅ 熔断器恢复正常")
		}
		
		return fmt.Sprintf("用户%s的数据", key), nil
	}
	
	// 测试熔断器
	fmt.Println("测试熔断器（模拟数据库压力大）:")
	for i := 1; i <= 10; i++ {
		key := fmt.Sprintf("user:%d", i)
		value, err := queryWithCircuitBreaker(key)
		if err != nil {
			fmt.Printf("查询失败: %s\n", key)
		} else {
			fmt.Printf("查询成功: %s = %s\n", key, value)
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	fmt.Println("\n✅ 熔断降级效果:")
	fmt.Println("   检测到数据库压力大时，自动熔断")
	fmt.Println("   返回默认值，保护数据库")
	fmt.Println("   一段时间后自动尝试恢复")
}
