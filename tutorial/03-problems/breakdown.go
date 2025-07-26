package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// 热点数据统计
type HotDataStats struct {
	accessCount int64
	lastAccess  time.Time
	mu          sync.RWMutex
}

func (h *HotDataStats) Access() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	atomic.AddInt64(&h.accessCount, 1)
	h.lastAccess = time.Now()
}

func (h *HotDataStats) GetCount() int64 {
	return atomic.LoadInt64(&h.accessCount)
}

// 模拟数据库（带延迟）
type SlowDatabase struct {
	data        map[string]string
	queryCount  int64
	rebuildTime time.Duration // 数据重建耗时
	mu          sync.RWMutex
}

func NewSlowDatabase() *SlowDatabase {
	db := &SlowDatabase{
		data:        make(map[string]string),
		rebuildTime: 200 * time.Millisecond, // 模拟复杂查询
	}
	
	// 初始化热点数据
	db.data["hot_user:1"] = "超级明星用户数据"
	db.data["hot_product:1"] = "爆款商品数据"
	
	return db
}

func (db *SlowDatabase) Query(key string) (string, bool) {
	atomic.AddInt64(&db.queryCount, 1)
	
	// 模拟复杂的数据重建过程
	fmt.Printf("📀 数据库重建数据: %s (耗时: %v)\n", key, db.rebuildTime)
	time.Sleep(db.rebuildTime)
	
	db.mu.RLock()
	defer db.mu.RUnlock()
	
	value, exists := db.data[key]
	return value, exists
}

func (db *SlowDatabase) GetQueryCount() int64 {
	return atomic.LoadInt64(&db.queryCount)
}

func (db *SlowDatabase) ResetQueryCount() {
	atomic.StoreInt64(&db.queryCount, 0)
}

// 普通缓存（容易击穿）
type VulnerableCache struct {
	data map[string]CacheItem
	mu   sync.RWMutex
}

func NewVulnerableCache() *VulnerableCache {
	return &VulnerableCache{
		data: make(map[string]CacheItem),
	}
}

func (c *VulnerableCache) Get(key string) (string, bool) {
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

func (c *VulnerableCache) Set(key, value string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.data[key] = CacheItem{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (c *VulnerableCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

// 带互斥锁的缓存（防击穿）
type MutexCache struct {
	data   map[string]CacheItem
	locks  map[string]*sync.Mutex
	mu     sync.RWMutex
	lockMu sync.Mutex
}

func NewMutexCache() *MutexCache {
	return &MutexCache{
		data:  make(map[string]CacheItem),
		locks: make(map[string]*sync.Mutex),
	}
}

func (c *MutexCache) Get(key string) (string, bool) {
	c.mu.RLock()
	item, exists := c.data[key]
	c.mu.RUnlock()
	
	if !exists {
		return "", false
	}
	
	if time.Now().After(item.ExpiresAt) {
		return "", false
	}
	
	return item.Value, true
}

func (c *MutexCache) Set(key, value string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.data[key] = CacheItem{
		Value:     value,
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (c *MutexCache) GetOrSet(key string, loader func() (string, error), ttl time.Duration) (string, error) {
	// 先尝试获取
	if value, exists := c.Get(key); exists {
		return value, nil
	}
	
	// 获取键级别的锁
	c.lockMu.Lock()
	keyLock, exists := c.locks[key]
	if !exists {
		keyLock = &sync.Mutex{}
		c.locks[key] = keyLock
	}
	c.lockMu.Unlock()
	
	// 使用键级别的锁
	keyLock.Lock()
	defer keyLock.Unlock()
	
	// 双重检查
	if value, exists := c.Get(key); exists {
		fmt.Printf("🔒 双重检查命中: %s\n", key)
		return value, nil
	}
	
	// 加载数据
	fmt.Printf("🔒 获得锁，开始重建: %s\n", key)
	value, err := loader()
	if err != nil {
		return "", err
	}
	
	c.Set(key, value, ttl)
	return value, nil
}

// 永不过期缓存（逻辑过期）
type NeverExpireCache struct {
	data map[string]LogicalCacheItem
	mu   sync.RWMutex
}

type LogicalCacheItem struct {
	Value      string
	LogicalExp time.Time // 逻辑过期时间
	Updating   bool      // 是否正在更新
}

func NewNeverExpireCache() *NeverExpireCache {
	return &NeverExpireCache{
		data: make(map[string]LogicalCacheItem),
	}
}

func (c *NeverExpireCache) Get(key string) (string, bool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, exists := c.data[key]
	if !exists {
		return "", false, false
	}
	
	// 检查逻辑过期
	isExpired := time.Now().After(item.LogicalExp)
	return item.Value, true, isExpired
}

func (c *NeverExpireCache) Set(key, value string, logicalTTL time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.data[key] = LogicalCacheItem{
		Value:      value,
		LogicalExp: time.Now().Add(logicalTTL),
		Updating:   false,
	}
}

func (c *NeverExpireCache) SetUpdating(key string, updating bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if item, exists := c.data[key]; exists {
		item.Updating = updating
		c.data[key] = item
	}
}

func (c *NeverExpireCache) IsUpdating(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if item, exists := c.data[key]; exists {
		return item.Updating
	}
	return false
}

// 演示缓存击穿问题
func DemoBreakdownProblem() {
	fmt.Println("=== 缓存击穿问题演示 ===")
	
	db := NewSlowDatabase()
	cache := NewVulnerableCache()
	stats := &HotDataStats{}
	
	// 预热热点数据
	fmt.Println("1. 预热热点数据:")
	hotKey := "hot_user:1"
	if value, exists := db.Query(hotKey); exists {
		cache.Set(hotKey, value, 3*time.Second) // 短TTL，容易过期
		fmt.Printf("预热完成: %s\n", hotKey)
	}
	
	// 正常访问期间
	fmt.Println("\n2. 正常访问期间:")
	for i := 0; i < 5; i++ {
		if _, exists := cache.Get(hotKey); exists {
			stats.Access()
			fmt.Printf("⚡ 缓存命中: %s (访问次数: %d)\n", hotKey, stats.GetCount())
		}
		time.Sleep(500 * time.Millisecond)
	}
	
	// 等待缓存过期
	fmt.Println("\n3. 等待缓存过期...")
	time.Sleep(1 * time.Second) // 确保缓存过期
	
	// 模拟大量并发访问热点数据
	fmt.Println("\n4. 缓存击穿发生 - 大量并发访问:")
	db.ResetQueryCount()
	
	var wg sync.WaitGroup
	concurrency := 20 // 20个并发请求
	start := time.Now()
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// 检查缓存
			if _, exists := cache.Get(hotKey); exists {
				fmt.Printf("⚡ 线程%d: 缓存命中\n", id)
				return
			}
			
			// 缓存未命中，查询数据库
			fmt.Printf("❌ 线程%d: 缓存未命中，查询数据库\n", id)
			if value, exists := db.Query(hotKey); exists {
				cache.Set(hotKey, value, 60*time.Second)
				fmt.Printf("✅ 线程%d: 重建缓存完成\n", id)
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	fmt.Printf("\n击穿结果:\n")
	fmt.Printf("- 并发请求数: %d\n", concurrency)
	fmt.Printf("- 数据库查询次数: %d\n", db.GetQueryCount())
	fmt.Printf("- 总耗时: %v\n", duration)
	fmt.Printf("- 平均响应时间: %v\n", duration/time.Duration(concurrency))
	
	fmt.Println("\n💥 问题分析:")
	fmt.Println("   热点数据过期时，大量请求同时重建缓存")
	fmt.Println("   数据库压力激增，响应时间变长")
	fmt.Println("   重复计算，资源浪费")
}

// 演示互斥锁解决方案
func DemoMutexSolution() {
	fmt.Println("\n=== 互斥锁解决方案演示 ===")
	
	db := NewSlowDatabase()
	cache := NewMutexCache()
	
	// 预热数据
	hotKey := "hot_user:1"
	loader := func() (string, error) {
		if value, exists := db.Query(hotKey); exists {
			return value, nil
		}
		return "", fmt.Errorf("数据不存在")
	}
	
	fmt.Println("预热热点数据:")
	cache.Set(hotKey, "超级明星用户数据", 2*time.Second)
	
	// 等待过期
	fmt.Println("等待缓存过期...")
	time.Sleep(3 * time.Second)
	
	// 模拟并发访问
	fmt.Println("\n使用互斥锁防止击穿:")
	db.ResetQueryCount()
	
	var wg sync.WaitGroup
	concurrency := 20
	start := time.Now()
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			value, err := cache.GetOrSet(hotKey, loader, 60*time.Second)
			if err != nil {
				fmt.Printf("❌ 线程%d: 获取失败\n", id)
			} else {
				fmt.Printf("✅ 线程%d: 获取成功 - %s\n", id, value)
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	fmt.Printf("\n互斥锁效果:\n")
	fmt.Printf("- 并发请求数: %d\n", concurrency)
	fmt.Printf("- 数据库查询次数: %d\n", db.GetQueryCount())
	fmt.Printf("- 总耗时: %v\n", duration)
	fmt.Printf("- 平均响应时间: %v\n", duration/time.Duration(concurrency))
	
	fmt.Println("\n✅ 互斥锁优势:")
	fmt.Println("   只有一个线程重建缓存")
	fmt.Println("   避免重复计算")
	fmt.Println("   减少数据库压力")
}

// 演示永不过期解决方案
func DemoNeverExpireSolution() {
	fmt.Println("\n=== 永不过期解决方案演示 ===")
	
	db := NewSlowDatabase()
	cache := NewNeverExpireCache()
	
	// 预热数据
	hotKey := "hot_user:1"
	fmt.Println("预热热点数据:")
	if value, exists := db.Query(hotKey); exists {
		cache.Set(hotKey, value, 3*time.Second) // 逻辑TTL
		fmt.Printf("预热完成: %s\n", hotKey)
	}
	
	// 异步更新函数
	asyncUpdate := func(key string) {
		if cache.IsUpdating(key) {
			fmt.Printf("🔄 %s 正在更新中，跳过\n", key)
			return
		}
		
		cache.SetUpdating(key, true)
		fmt.Printf("🔄 开始异步更新: %s\n", key)
		
		go func() {
			defer cache.SetUpdating(key, false)
			
			if value, exists := db.Query(key); exists {
				cache.Set(key, value, 60*time.Second)
				fmt.Printf("✅ 异步更新完成: %s\n", key)
			}
		}()
	}
	
	// 模拟访问
	fmt.Println("\n模拟持续访问:")
	
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		
		_, exists, isExpired := cache.Get(hotKey)
		if exists {
			fmt.Printf("⚡ 缓存命中: %s (逻辑过期: %v)\n", hotKey, isExpired)
			
			// 如果逻辑过期，触发异步更新
			if isExpired {
				fmt.Println("🕐 检测到逻辑过期，触发异步更新")
				asyncUpdate(hotKey)
			}
		} else {
			fmt.Printf("❌ 缓存未命中: %s\n", hotKey)
		}
	}
	
	fmt.Println("\n✅ 永不过期优势:")
	fmt.Println("   用户始终能获取到数据（即使是过期的）")
	fmt.Println("   异步更新，不影响用户体验")
	fmt.Println("   避免缓存击穿")
	
	fmt.Println("\n⚠️ 注意事项:")
	fmt.Println("   需要容忍短期的数据不一致")
	fmt.Println("   需要合理的异步更新策略")
}

// 演示热点数据预警
func DemoHotDataMonitoring() {
	fmt.Println("\n=== 热点数据监控演示 ===")
	
	// 模拟访问统计
	hotDataMap := make(map[string]*HotDataStats)
	hotDataMap["user:1"] = &HotDataStats{}
	hotDataMap["user:2"] = &HotDataStats{}
	hotDataMap["product:1"] = &HotDataStats{}
	
	// 模拟访问
	fmt.Println("模拟用户访问:")
	
	// user:1 成为热点
	for i := 0; i < 100; i++ {
		hotDataMap["user:1"].Access()
	}
	
	// user:2 正常访问
	for i := 0; i < 10; i++ {
		hotDataMap["user:2"].Access()
	}
	
	// product:1 中等访问
	for i := 0; i < 50; i++ {
		hotDataMap["product:1"].Access()
	}
	
	// 热点检测
	fmt.Println("\n热点数据检测:")
	hotThreshold := int64(80)
	
	for key, stats := range hotDataMap {
		count := stats.GetCount()
		if count > hotThreshold {
			fmt.Printf("🔥 检测到热点数据: %s (访问次数: %d)\n", key, count)
			fmt.Printf("   建议: 设置永不过期或延长TTL\n")
		} else {
			fmt.Printf("📊 正常数据: %s (访问次数: %d)\n", key, count)
		}
	}
	
	fmt.Println("\n💡 热点数据策略:")
	fmt.Println("   1. 实时监控访问频率")
	fmt.Println("   2. 动态调整缓存策略")
	fmt.Println("   3. 预警和自动处理")
	fmt.Println("   4. 多级缓存保护")
}
