package main

import (
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"
)

// 布隆过滤器实现
type BloomFilter struct {
	bitArray []bool
	size     uint
	hashFuncs int
	mu       sync.RWMutex
}

func NewBloomFilter(size uint, hashFuncs int) *BloomFilter {
	return &BloomFilter{
		bitArray:  make([]bool, size),
		size:      size,
		hashFuncs: hashFuncs,
	}
}

// 添加元素到布隆过滤器
func (bf *BloomFilter) Add(item string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	
	for i := 0; i < bf.hashFuncs; i++ {
		hash := bf.hash(item, i)
		bf.bitArray[hash] = true
	}
}

// 检查元素是否可能存在
func (bf *BloomFilter) MightContain(item string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	
	for i := 0; i < bf.hashFuncs; i++ {
		hash := bf.hash(item, i)
		if !bf.bitArray[hash] {
			return false // 绝对不存在
		}
	}
	return true // 可能存在
}

// 哈希函数
func (bf *BloomFilter) hash(item string, seed int) uint {
	h := fnv.New32a()
	h.Write([]byte(fmt.Sprintf("%s%d", item, seed)))
	return uint(h.Sum32()) % bf.size
}

// 带统计的数据库
type DatabaseWithStats struct {
	data           map[string]string
	queryCount     int64
	invalidQueries int64 // 无效查询计数
	mu             sync.RWMutex
}

func NewDatabaseWithStats() *DatabaseWithStats {
	db := &DatabaseWithStats{
		data: make(map[string]string),
	}
	
	// 初始化一些有效数据
	for i := 1; i <= 100; i++ {
		key := fmt.Sprintf("user:%d", i)
		value := fmt.Sprintf("用户%d的数据", i)
		db.data[key] = value
	}
	
	return db
}

func (db *DatabaseWithStats) Query(key string) (string, bool) {
	atomic.AddInt64(&db.queryCount, 1)
	
	// 模拟数据库查询延迟
	time.Sleep(20 * time.Millisecond)
	
	db.mu.RLock()
	defer db.mu.RUnlock()
	
	value, exists := db.data[key]
	if !exists {
		atomic.AddInt64(&db.invalidQueries, 1)
		fmt.Printf("📀 数据库查询(无效): %s (总查询: %d, 无效: %d)\n", 
			key, atomic.LoadInt64(&db.queryCount), atomic.LoadInt64(&db.invalidQueries))
	} else {
		fmt.Printf("📀 数据库查询(有效): %s\n", key)
	}
	
	return value, exists
}

func (db *DatabaseWithStats) GetStats() (int64, int64) {
	return atomic.LoadInt64(&db.queryCount), atomic.LoadInt64(&db.invalidQueries)
}

func (db *DatabaseWithStats) ResetStats() {
	atomic.StoreInt64(&db.queryCount, 0)
	atomic.StoreInt64(&db.invalidQueries, 0)
}

// 缓存空值的缓存实现
type NullCache struct {
	data map[string]NullCacheItem
	mu   sync.RWMutex
}

type NullCacheItem struct {
	Value     string
	IsNull    bool // 标记是否为空值
	ExpiresAt time.Time
}

func NewNullCache() *NullCache {
	return &NullCache{
		data: make(map[string]NullCacheItem),
	}
}

func (c *NullCache) Get(key string) (string, bool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, exists := c.data[key]
	if !exists {
		return "", false, false
	}
	
	// 检查是否过期
	if time.Now().After(item.ExpiresAt) {
		return "", false, false
	}
	
	if item.IsNull {
		fmt.Printf("⚡ 缓存命中(空值): %s\n", key)
		return "", true, true // 存在但是空值
	}
	
	fmt.Printf("⚡ 缓存命中: %s = %s\n", key, item.Value)
	return item.Value, true, false
}

func (c *NullCache) Set(key, value string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = NullCacheItem{
		Value:     value,
		IsNull:    false,
		ExpiresAt: time.Now().Add(ttl),
	}
}

func (c *NullCache) SetNull(key string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = NullCacheItem{
		Value:     "",
		IsNull:    true,
		ExpiresAt: time.Now().Add(ttl),
	}
	fmt.Printf("⚡ 缓存空值: %s (TTL: %v)\n", key, ttl)
}

// 演示缓存穿透问题
func DemoPenetrationProblem() {
	fmt.Println("=== 缓存穿透问题演示 ===")
	
	db := NewDatabaseWithStats()
	cache := NewNullCache()
	
	// 模拟正常查询
	fmt.Println("\n1. 正常查询:")
	normalKeys := []string{"user:1", "user:2", "user:3"}
	
	for _, key := range normalKeys {
		// 先查缓存
		if value, exists, isNull := cache.Get(key); exists {
			if !isNull {
				fmt.Printf("缓存命中: %s = %s\n", key, value)
			}
		} else {
			// 查数据库
			if value, exists := db.Query(key); exists {
				cache.Set(key, value, 60*time.Second)
			}
		}
	}
	
	total, invalid := db.GetStats()
	fmt.Printf("正常查询统计 - 总查询: %d, 无效查询: %d\n", total, invalid)
	
	// 模拟恶意攻击 - 查询大量不存在的数据
	fmt.Println("\n2. 恶意攻击 - 查询不存在的数据:")
	db.ResetStats()
	
	maliciousKeys := []string{
		"user:999999", "user:888888", "user:777777",
		"user:666666", "user:555555", "user:444444",
		"user:333333", "user:222222", "user:111111",
		"user:000000",
	}
	
	start := time.Now()
	
	for _, key := range maliciousKeys {
		// 先查缓存
		if value, exists, isNull := cache.Get(key); exists {
			if !isNull {
				fmt.Printf("缓存命中: %s = %s\n", key, value)
			}
		} else {
			// 查数据库
			if value, exists := db.Query(key); exists {
				cache.Set(key, value, 60*time.Second)
			}
			// 注意：这里没有缓存空值，所以每次都会查数据库
		}
	}
	
	duration := time.Since(start)
	total, invalid = db.GetStats()
	
	fmt.Printf("\n恶意攻击结果:\n")
	fmt.Printf("- 查询数量: %d\n", len(maliciousKeys))
	fmt.Printf("- 数据库总查询: %d\n", total)
	fmt.Printf("- 无效查询: %d\n", invalid)
	fmt.Printf("- 总耗时: %v\n", duration)
	fmt.Printf("- 平均耗时: %v\n", duration/time.Duration(len(maliciousKeys)))
	
	fmt.Println("\n💥 问题分析:")
	fmt.Println("   每次查询不存在的数据都要访问数据库")
	fmt.Println("   数据库压力大，响应时间长")
	fmt.Println("   缓存完全失效")
}

// 演示布隆过滤器解决方案
func DemoBloomFilterSolution() {
	fmt.Println("\n=== 布隆过滤器解决方案演示 ===")
	
	db := NewDatabaseWithStats()
	cache := NewNullCache()
	
	// 创建布隆过滤器
	bf := NewBloomFilter(1000, 3)
	
	// 将所有存在的数据添加到布隆过滤器
	fmt.Println("初始化布隆过滤器:")
	for i := 1; i <= 100; i++ {
		key := fmt.Sprintf("user:%d", i)
		bf.Add(key)
	}
	fmt.Println("布隆过滤器初始化完成，包含100个有效用户")
	
	// 测试布隆过滤器的准确性
	fmt.Println("\n测试布隆过滤器:")
	
	// 测试存在的数据
	existingKeys := []string{"user:1", "user:50", "user:100"}
	fmt.Println("测试存在的数据:")
	for _, key := range existingKeys {
		if bf.MightContain(key) {
			fmt.Printf("✅ %s 可能存在\n", key)
		} else {
			fmt.Printf("❌ %s 绝对不存在\n", key)
		}
	}
	
	// 测试不存在的数据
	nonExistingKeys := []string{"user:999", "user:888", "user:777"}
	fmt.Println("\n测试不存在的数据:")
	for _, key := range nonExistingKeys {
		if bf.MightContain(key) {
			fmt.Printf("⚠️ %s 可能存在（误判）\n", key)
		} else {
			fmt.Printf("✅ %s 绝对不存在\n", key)
		}
	}
	
	// 使用布隆过滤器防止穿透
	fmt.Println("\n使用布隆过滤器防止穿透:")
	db.ResetStats()
	
	queryWithBloomFilter := func(key string) (string, error) {
		// 1. 先检查布隆过滤器
		if !bf.MightContain(key) {
			fmt.Printf("🛡️ 布隆过滤器拦截: %s (绝对不存在)\n", key)
			return "", fmt.Errorf("数据不存在")
		}
		
		// 2. 检查缓存
		if value, exists, isNull := cache.Get(key); exists {
			if isNull {
				return "", fmt.Errorf("数据不存在")
			}
			return value, nil
		}
		
		// 3. 查询数据库
		if value, exists := db.Query(key); exists {
			cache.Set(key, value, 60*time.Second)
			return value, nil
		} else {
			// 缓存空值，防止重复查询
			cache.SetNull(key, 10*time.Second)
			return "", fmt.Errorf("数据不存在")
		}
	}
	
	// 测试混合查询
	testKeys := []string{
		"user:1",      // 存在
		"user:999",    // 不存在，会被布隆过滤器拦截
		"user:2",      // 存在
		"user:888",    // 不存在，会被布隆过滤器拦截
		"user:3",      // 存在
	}
	
	start := time.Now()
	
	for _, key := range testKeys {
		value, err := queryWithBloomFilter(key)
		if err != nil {
			fmt.Printf("查询失败: %s - %s\n", key, err.Error())
		} else {
			fmt.Printf("查询成功: %s = %s\n", key, value)
		}
	}
	
	duration := time.Since(start)
	total, invalid := db.GetStats()
	
	fmt.Printf("\n布隆过滤器效果:\n")
	fmt.Printf("- 查询数量: %d\n", len(testKeys))
	fmt.Printf("- 数据库总查询: %d\n", total)
	fmt.Printf("- 无效查询: %d\n", invalid)
	fmt.Printf("- 总耗时: %v\n", duration)
	
	fmt.Println("\n✅ 布隆过滤器优势:")
	fmt.Println("   有效拦截不存在的数据查询")
	fmt.Println("   大幅减少数据库压力")
	fmt.Println("   内存占用小，查询速度快")
}

// 演示缓存空值解决方案
func DemoNullCacheSolution() {
	fmt.Println("\n=== 缓存空值解决方案演示 ===")
	
	db := NewDatabaseWithStats()
	cache := NewNullCache()
	
	queryWithNullCache := func(key string) (string, error) {
		// 1. 检查缓存（包括空值）
		if value, exists, isNull := cache.Get(key); exists {
			if isNull {
				return "", fmt.Errorf("数据不存在（来自缓存）")
			}
			return value, nil
		}
		
		// 2. 查询数据库
		if value, exists := db.Query(key); exists {
			cache.Set(key, value, 60*time.Second)
			return value, nil
		} else {
			// 关键：缓存空值
			cache.SetNull(key, 30*time.Second) // 空值TTL较短
			return "", fmt.Errorf("数据不存在")
		}
	}
	
	// 第一次查询不存在的数据
	fmt.Println("第一次查询不存在的数据:")
	db.ResetStats()
	
	nonExistentKey := "user:999999"
	_, err := queryWithNullCache(nonExistentKey)
	fmt.Printf("查询结果: %s\n", err.Error())
	
	total1, _ := db.GetStats()
	fmt.Printf("数据库查询次数: %d\n", total1)

	// 第二次查询相同的不存在数据
	fmt.Println("\n第二次查询相同的不存在数据:")
	db.ResetStats()

	_, err = queryWithNullCache(nonExistentKey)
	fmt.Printf("查询结果: %s\n", err.Error())

	total2, _ := db.GetStats()
	fmt.Printf("数据库查询次数: %d\n", total2)
	
	fmt.Println("\n✅ 缓存空值效果:")
	fmt.Printf("   第一次查询: 数据库查询 %d 次\n", total1)
	fmt.Printf("   第二次查询: 数据库查询 %d 次\n", total2)
	fmt.Println("   空值缓存有效防止了重复的无效查询")
	
	// 演示空值TTL过期
	fmt.Println("\n演示空值TTL过期:")
	fmt.Println("等待空值缓存过期...")
	time.Sleep(31 * time.Second) // 等待空值过期
	
	fmt.Println("空值过期后再次查询:")
	db.ResetStats()
	_, err = queryWithNullCache(nonExistentKey)
	total3, _ := db.GetStats()
	fmt.Printf("查询结果: %s\n", err.Error())
	fmt.Printf("数据库查询次数: %d\n", total3)
	
	fmt.Println("\n💡 空值缓存策略:")
	fmt.Println("   1. 空值TTL要比正常值短")
	fmt.Println("   2. 防止数据新增后无法及时发现")
	fmt.Println("   3. 平衡防穿透效果和数据时效性")
}
