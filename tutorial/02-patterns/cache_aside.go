package main

import (
	"fmt"
	"sync"
	"time"
)

// Database 模拟数据库
type Database struct {
	data map[string]string
	mu   sync.RWMutex
}

func NewDatabase() *Database {
	return &Database{
		data: make(map[string]string),
	}
}

func (db *Database) Get(key string) (string, bool) {
	// 模拟数据库查询延迟
	time.Sleep(50 * time.Millisecond)
	
	db.mu.RLock()
	defer db.mu.RUnlock()
	
	value, exists := db.data[key]
	fmt.Printf("📀 数据库查询: %s = %s\n", key, value)
	return value, exists
}

func (db *Database) Set(key, value string) {
	// 模拟数据库写入延迟
	time.Sleep(100 * time.Millisecond)
	
	db.mu.Lock()
	defer db.mu.Unlock()
	
	db.data[key] = value
	fmt.Printf("📀 数据库写入: %s = %s\n", key, value)
}

func (db *Database) Delete(key string) {
	db.mu.Lock()
	defer db.mu.Unlock()
	
	delete(db.data, key)
	fmt.Printf("📀 数据库删除: %s\n", key)
}

// Cache 简单缓存
type Cache struct {
	data map[string]string
	mu   sync.RWMutex
}

func NewCache() *Cache {
	return &Cache{
		data: make(map[string]string),
	}
}

func (c *Cache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	value, exists := c.data[key]
	if exists {
		fmt.Printf("⚡ 缓存命中: %s = %s\n", key, value)
	} else {
		fmt.Printf("❌ 缓存未命中: %s\n", key)
	}
	return value, exists
}

func (c *Cache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.data[key] = value
	fmt.Printf("⚡ 缓存写入: %s = %s\n", key, value)
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.data, key)
	fmt.Printf("⚡ 缓存删除: %s\n", key)
}

// CacheAsideService Cache-Aside模式服务
type CacheAsideService struct {
	cache *Cache
	db    *Database
}

func NewCacheAsideService() *CacheAsideService {
	return &CacheAsideService{
		cache: NewCache(),
		db:    NewDatabase(),
	}
}

// Get Cache-Aside读取模式
// 1. 先查缓存
// 2. 缓存未命中则查数据库
// 3. 将数据库结果写入缓存
func (s *CacheAsideService) Get(key string) (string, error) {
	fmt.Printf("\n🔍 Cache-Aside 读取: %s\n", key)
	
	// 1. 先查缓存
	if value, exists := s.cache.Get(key); exists {
		return value, nil
	}
	
	// 2. 缓存未命中，查询数据库
	value, exists := s.db.Get(key)
	if !exists {
		return "", fmt.Errorf("数据不存在: %s", key)
	}
	
	// 3. 将数据写入缓存
	s.cache.Set(key, value)
	
	return value, nil
}

// Set Cache-Aside写入模式
// 1. 先更新数据库
// 2. 删除缓存（而不是更新缓存）
func (s *CacheAsideService) Set(key, value string) error {
	fmt.Printf("\n✏️ Cache-Aside 写入: %s = %s\n", key, value)
	
	// 1. 先更新数据库
	s.db.Set(key, value)
	
	// 2. 删除缓存（让下次读取时重新加载）
	s.cache.Delete(key)
	
	return nil
}

// Delete Cache-Aside删除模式
func (s *CacheAsideService) Delete(key string) error {
	fmt.Printf("\n🗑️ Cache-Aside 删除: %s\n", key)
	
	// 1. 删除数据库数据
	s.db.Delete(key)
	
	// 2. 删除缓存
	s.cache.Delete(key)
	
	return nil
}

// SetWithUpdate 演示更新缓存的问题
func (s *CacheAsideService) SetWithUpdate(key, value string) error {
	fmt.Printf("\n⚠️ Cache-Aside 写入(更新缓存): %s = %s\n", key, value)
	
	// 1. 更新数据库
	s.db.Set(key, value)
	
	// 2. 更新缓存（这种方式有并发问题）
	s.cache.Set(key, value)
	
	return nil
}

// DemoCacheAside 演示Cache-Aside模式
func DemoCacheAside() {
	fmt.Println("=== Cache-Aside 模式演示 ===")
	
	service := NewCacheAsideService()
	
	// 初始化一些数据到数据库
	service.db.Set("user:1", "张三")
	service.db.Set("user:2", "李四")
	fmt.Println("初始化数据库完成")
	
	// 演示读取流程
	fmt.Println("\n--- 读取流程演示 ---")
	
	// 第一次读取：缓存未命中
	start := time.Now()
	value1, _ := service.Get("user:1")
	time1 := time.Since(start)
	fmt.Printf("第一次读取结果: %s, 耗时: %v\n", value1, time1)
	
	// 第二次读取：缓存命中
	start = time.Now()
	value2, _ := service.Get("user:1")
	time2 := time.Since(start)
	fmt.Printf("第二次读取结果: %s, 耗时: %v\n", value2, time2)
	
	fmt.Printf("性能提升: %.1fx\n", float64(time1)/float64(time2))
	
	// 演示写入流程
	fmt.Println("\n--- 写入流程演示 ---")
	
	// 更新数据
	service.Set("user:1", "张三(已更新)")
	
	// 读取更新后的数据
	value3, _ := service.Get("user:1")
	fmt.Printf("更新后读取: %s\n", value3)
	
	// 演示删除流程
	fmt.Println("\n--- 删除流程演示 ---")
	service.Delete("user:2")
	
	// 尝试读取已删除的数据
	_, err := service.Get("user:2")
	if err != nil {
		fmt.Printf("删除验证: %s\n", err.Error())
	}
}

// DemoConcurrencyProblem 演示并发问题
func DemoConcurrencyProblem() {
	fmt.Println("\n=== 并发问题演示 ===")
	
	service := NewCacheAsideService()
	service.db.Set("counter", "100")
	
	// 模拟并发更新
	var wg sync.WaitGroup
	
	// 启动多个goroutine同时更新
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// 使用更新缓存的方式（有问题）
			newValue := fmt.Sprintf("100_updated_by_%d", id)
			service.SetWithUpdate("counter", newValue)
		}(i)
	}
	
	wg.Wait()
	
	// 检查最终结果
	fmt.Println("\n并发更新后的状态:")
	service.Get("counter")
	
	fmt.Println("\n💡 说明: 在并发环境下，更新缓存可能导致数据不一致")
	fmt.Println("   推荐做法: 删除缓存，让下次读取时重新加载")
}

// DemoDelayedDoubleDelete 演示延迟双删策略
func DemoDelayedDoubleDelete() {
	fmt.Println("\n=== 延迟双删策略演示 ===")
	
	service := NewCacheAsideService()
	service.db.Set("product:1", "商品A")
	
	// 先读取一次，让数据进入缓存
	service.Get("product:1")
	
	fmt.Println("\n执行延迟双删策略:")
	
	// 延迟双删策略
	key := "product:1"
	newValue := "商品A(已更新)"
	
	// 1. 第一次删除缓存
	fmt.Println("1. 第一次删除缓存")
	service.cache.Delete(key)
	
	// 2. 更新数据库
	fmt.Println("2. 更新数据库")
	service.db.Set(key, newValue)
	
	// 3. 延迟一段时间
	fmt.Println("3. 延迟等待...")
	time.Sleep(200 * time.Millisecond)
	
	// 4. 第二次删除缓存
	fmt.Println("4. 第二次删除缓存")
	service.cache.Delete(key)
	
	// 验证结果
	fmt.Println("\n验证最终结果:")
	service.Get(key)
	
	fmt.Println("\n💡 延迟双删可以减少数据不一致的概率")
}
