package main

import (
	"fmt"
	"sync"
	"time"
)

// MultilevelCache 多级缓存实现
type MultilevelCache struct {
	l1Cache *LocalCache        // L1: 本地缓存
	l2Cache *DistributedCache  // L2: 分布式缓存
	config  *MultilevelConfig
	metrics *MultilevelMetrics
	mu      sync.RWMutex
}

// MultilevelConfig 多级缓存配置
type MultilevelConfig struct {
	L1TTL        time.Duration // L1缓存TTL
	L2TTL        time.Duration // L2缓存TTL
	L1MaxSize    int           // L1最大容量
	EnableL1     bool          // 是否启用L1
	EnableL2     bool          // 是否启用L2
	WriteThrough bool          // 是否写穿透
}

// MultilevelMetrics 多级缓存指标
type MultilevelMetrics struct {
	L1Hits       int64
	L2Hits       int64
	DatabaseHits int64
	TotalRequests int64
	L1HitRate    float64
	L2HitRate    float64
	OverallHitRate float64
	mu           sync.RWMutex
}

// Database 模拟数据库
type Database struct {
	data    map[string]string
	latency time.Duration
	mu      sync.RWMutex
}

func NewDatabase() *Database {
	db := &Database{
		data:    make(map[string]string),
		latency: 50 * time.Millisecond, // 模拟数据库延迟
	}
	
	// 初始化一些数据
	db.data["user:1"] = "张三"
	db.data["user:2"] = "李四"
	db.data["user:3"] = "王五"
	db.data["product:1"] = "iPhone 15"
	db.data["product:2"] = "MacBook Pro"
	db.data["order:1"] = "订单001"
	db.data["order:2"] = "订单002"
	
	return db
}

func (db *Database) Get(key string) (string, bool) {
	// 模拟数据库查询延迟
	time.Sleep(db.latency)
	
	db.mu.RLock()
	defer db.mu.RUnlock()
	
	value, exists := db.data[key]
	return value, exists
}

func (db *Database) Set(key, value string) {
	time.Sleep(db.latency)
	
	db.mu.Lock()
	defer db.mu.Unlock()
	
	db.data[key] = value
}

// NewMultilevelCache 创建多级缓存
func NewMultilevelCache(config *MultilevelConfig) *MultilevelCache {
	if config == nil {
		config = &MultilevelConfig{
			L1TTL:        5 * time.Minute,
			L2TTL:        30 * time.Minute,
			L1MaxSize:    1000,
			EnableL1:     true,
			EnableL2:     true,
			WriteThrough: true,
		}
	}
	
	var l1Cache *LocalCache
	var l2Cache *DistributedCache
	
	if config.EnableL1 {
		l1Cache = NewLocalCache(config.L1MaxSize)
	}
	
	if config.EnableL2 {
		l2Cache = NewDistributedCache(2 * time.Millisecond)
	}
	
	return &MultilevelCache{
		l1Cache: l1Cache,
		l2Cache: l2Cache,
		config:  config,
		metrics: &MultilevelMetrics{},
	}
}

// Get 多级缓存读取
func (mc *MultilevelCache) Get(key string, database *Database) (string, error) {
	mc.recordRequest()
	
	// L1: 本地缓存
	if mc.config.EnableL1 && mc.l1Cache != nil {
		if value, exists := mc.l1Cache.Get(key); exists {
			mc.recordL1Hit()
			return value.(string), nil
		}
	}
	
	// L2: 分布式缓存
	if mc.config.EnableL2 && mc.l2Cache != nil {
		if value, exists := mc.l2Cache.Get(key); exists {
			mc.recordL2Hit()
			
			// 回写到L1
			if mc.config.EnableL1 && mc.l1Cache != nil {
				mc.l1Cache.Set(key, value, mc.config.L1TTL)
			}
			
			return value.(string), nil
		}
	}
	
	// L3: 数据库
	value, exists := database.Get(key)
	if !exists {
		return "", fmt.Errorf("key not found: %s", key)
	}
	
	mc.recordDatabaseHit()
	
	// 写入缓存
	if mc.config.EnableL2 && mc.l2Cache != nil {
		mc.l2Cache.Set(key, value, mc.config.L2TTL)
	}
	
	if mc.config.EnableL1 && mc.l1Cache != nil {
		mc.l1Cache.Set(key, value, mc.config.L1TTL)
	}
	
	return value, nil
}

// Set 多级缓存写入
func (mc *MultilevelCache) Set(key, value string, database *Database) error {
	// 写入数据库
	database.Set(key, value)
	
	if mc.config.WriteThrough {
		// 写穿透模式：同时更新缓存
		if mc.config.EnableL2 && mc.l2Cache != nil {
			mc.l2Cache.Set(key, value, mc.config.L2TTL)
		}
		
		if mc.config.EnableL1 && mc.l1Cache != nil {
			mc.l1Cache.Set(key, value, mc.config.L1TTL)
		}
	} else {
		// 写回模式：删除缓存
		mc.Delete(key)
	}
	
	return nil
}

// Delete 删除缓存
func (mc *MultilevelCache) Delete(key string) {
	if mc.config.EnableL1 && mc.l1Cache != nil {
		mc.l1Cache.Delete(key)
	}
	
	if mc.config.EnableL2 && mc.l2Cache != nil {
		mc.l2Cache.Delete(key)
	}
}

// GetMetrics 获取指标
func (mc *MultilevelCache) GetMetrics() MultilevelMetrics {
	mc.metrics.mu.RLock()
	defer mc.metrics.mu.RUnlock()
	
	// 计算命中率
	if mc.metrics.TotalRequests > 0 {
		mc.metrics.L1HitRate = float64(mc.metrics.L1Hits) / float64(mc.metrics.TotalRequests)
		mc.metrics.L2HitRate = float64(mc.metrics.L2Hits) / float64(mc.metrics.TotalRequests)
		mc.metrics.OverallHitRate = float64(mc.metrics.L1Hits+mc.metrics.L2Hits) / float64(mc.metrics.TotalRequests)
	}
	
	return *mc.metrics
}

// recordRequest 记录请求
func (mc *MultilevelCache) recordRequest() {
	mc.metrics.mu.Lock()
	defer mc.metrics.mu.Unlock()
	
	mc.metrics.TotalRequests++
}

// recordL1Hit 记录L1命中
func (mc *MultilevelCache) recordL1Hit() {
	mc.metrics.mu.Lock()
	defer mc.metrics.mu.Unlock()
	
	mc.metrics.L1Hits++
}

// recordL2Hit 记录L2命中
func (mc *MultilevelCache) recordL2Hit() {
	mc.metrics.mu.Lock()
	defer mc.metrics.mu.Unlock()
	
	mc.metrics.L2Hits++
}

// recordDatabaseHit 记录数据库命中
func (mc *MultilevelCache) recordDatabaseHit() {
	mc.metrics.mu.Lock()
	defer mc.metrics.mu.Unlock()
	
	mc.metrics.DatabaseHits++
}

// 演示多级缓存功能
func DemoMultilevelCache() {
	fmt.Println("=== 多级缓存演示 ===")
	
	// 创建数据库
	database := NewDatabase()
	
	// 创建多级缓存
	config := &MultilevelConfig{
		L1TTL:        2 * time.Minute,
		L2TTL:        10 * time.Minute,
		L1MaxSize:    100,
		EnableL1:     true,
		EnableL2:     true,
		WriteThrough: true,
	}
	
	cache := NewMultilevelCache(config)
	
	// 测试读取流程
	fmt.Println("\n1. 测试多级缓存读取流程:")
	
	testKeys := []string{"user:1", "product:1", "order:1"}
	
	for _, key := range testKeys {
		fmt.Printf("\n--- 测试键: %s ---\n", key)
		
		// 第一次读取：从数据库加载
		start := time.Now()
		value1, err := cache.Get(key, database)
		latency1 := time.Since(start)
		if err != nil {
			fmt.Printf("❌ 读取失败: %v\n", err)
			continue
		}
		fmt.Printf("第1次读取: %s, 延迟: %v (数据库)\n", value1, latency1)
		
		// 第二次读取：L1缓存命中
		start = time.Now()
		value2, _ := cache.Get(key, database)
		latency2 := time.Since(start)
		fmt.Printf("第2次读取: %s, 延迟: %v (L1缓存)\n", value2, latency2)
		
		// 性能提升
		speedup := float64(latency1) / float64(latency2)
		fmt.Printf("性能提升: %.1fx\n", speedup)
	}
	
	// 显示指标
	fmt.Println("\n2. 缓存指标:")
	metrics := cache.GetMetrics()
	fmt.Printf("总请求数: %d\n", metrics.TotalRequests)
	fmt.Printf("L1命中数: %d (命中率: %.2f%%)\n", metrics.L1Hits, metrics.L1HitRate*100)
	fmt.Printf("L2命中数: %d (命中率: %.2f%%)\n", metrics.L2Hits, metrics.L2HitRate*100)
	fmt.Printf("数据库命中数: %d\n", metrics.DatabaseHits)
	fmt.Printf("总体命中率: %.2f%%\n", metrics.OverallHitRate*100)
	
	// 测试写入
	fmt.Println("\n3. 测试写入操作:")
	
	newKey := "user:999"
	newValue := "新用户"
	
	fmt.Printf("写入新数据: %s = %s\n", newKey, newValue)
	cache.Set(newKey, newValue, database)
	
	// 立即读取（应该从L1缓存命中）
	start := time.Now()
	readValue, _ := cache.Get(newKey, database)
	readLatency := time.Since(start)
	fmt.Printf("立即读取: %s, 延迟: %v (L1缓存)\n", readValue, readLatency)
	
	// 测试缓存失效
	fmt.Println("\n4. 测试缓存失效:")
	
	// 删除缓存
	cache.Delete("user:1")
	fmt.Println("删除 user:1 的缓存")
	
	// 再次读取（应该从数据库加载）
	start = time.Now()
	value, _ := cache.Get("user:1", database)
	latency := time.Since(start)
	fmt.Printf("删除后读取: %s, 延迟: %v (数据库)\n", value, latency)
	
	fmt.Println("\n💡 多级缓存优势:")
	fmt.Println("   1. L1提供极低延迟访问")
	fmt.Println("   2. L2提供大容量缓存")
	fmt.Println("   3. 自动数据回写和同步")
	fmt.Println("   4. 高可用性和容错能力")
	fmt.Println("   5. 显著提升整体性能")
}

// 演示不同配置的性能对比
func DemoConfigComparison() {
	fmt.Println("\n=== 配置对比演示 ===")
	
	database := NewDatabase()
	testKeys := []string{"user:1", "user:2", "product:1", "product:2", "order:1"}
	
	// 配置1: 只有L2缓存
	fmt.Println("\n1. 只有L2缓存:")
	config1 := &MultilevelConfig{
		L2TTL:    10 * time.Minute,
		EnableL1: false,
		EnableL2: true,
	}
	cache1 := NewMultilevelCache(config1)
	
	start := time.Now()
	for _, key := range testKeys {
		cache1.Get(key, database) // 第一次加载
		cache1.Get(key, database) // 第二次从L2读取
	}
	time1 := time.Since(start)
	metrics1 := cache1.GetMetrics()
	
	fmt.Printf("总耗时: %v\n", time1)
	fmt.Printf("总体命中率: %.2f%%\n", metrics1.OverallHitRate*100)
	
	// 配置2: L1+L2缓存
	fmt.Println("\n2. L1+L2缓存:")
	config2 := &MultilevelConfig{
		L1TTL:     2 * time.Minute,
		L2TTL:     10 * time.Minute,
		L1MaxSize: 100,
		EnableL1:  true,
		EnableL2:  true,
	}
	cache2 := NewMultilevelCache(config2)
	
	start = time.Now()
	for _, key := range testKeys {
		cache2.Get(key, database) // 第一次加载
		cache2.Get(key, database) // 第二次从L1读取
	}
	time2 := time.Since(start)
	metrics2 := cache2.GetMetrics()
	
	fmt.Printf("总耗时: %v\n", time2)
	fmt.Printf("L1命中率: %.2f%%\n", metrics2.L1HitRate*100)
	fmt.Printf("总体命中率: %.2f%%\n", metrics2.OverallHitRate*100)
	
	// 性能对比
	fmt.Printf("\n性能提升: %.1fx\n", float64(time1)/float64(time2))
	
	fmt.Println("\n💡 配置建议:")
	fmt.Println("   1. 热点数据多：启用L1+L2")
	fmt.Println("   2. 内存受限：只启用L2")
	fmt.Println("   3. 延迟敏感：优先L1缓存")
	fmt.Println("   4. 容量需求大：依赖L2缓存")
}
