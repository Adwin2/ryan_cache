package main

import (
	"fmt"
	"sync"
	"time"
)

// DirtyItem 脏数据项
type DirtyItem struct {
	Key       string
	Value     string
	Timestamp time.Time
	IsDirty   bool
}

// WriteBackCache Write-Back模式缓存
type WriteBackCache struct {
	cache     *Cache
	db        *Database
	dirtyData map[string]*DirtyItem
	mu        sync.RWMutex
	stopCh    chan struct{}
	flushInterval time.Duration
}

func NewWriteBackCache(flushInterval time.Duration) *WriteBackCache {
	wbc := &WriteBackCache{
		cache:         NewCache(),
		db:           NewDatabase(),
		dirtyData:    make(map[string]*DirtyItem),
		stopCh:       make(chan struct{}),
		flushInterval: flushInterval,
	}
	
	// 启动后台刷新goroutine
	go wbc.backgroundFlush()
	
	return wbc
}

// Get Write-Back读取
func (wb *WriteBackCache) Get(key string) (string, error) {
	fmt.Printf("\n🔍 Write-Back 读取: %s\n", key)
	
	// 1. 先查缓存
	if value, exists := wb.cache.Get(key); exists {
		return value, nil
	}
	
	// 2. 缓存未命中，查询数据库
	value, exists := wb.db.Get(key)
	if !exists {
		return "", fmt.Errorf("数据不存在: %s", key)
	}
	
	// 3. 将数据写入缓存
	wb.cache.Set(key, value)
	
	return value, nil
}

// Set Write-Back写入
// 关键：只写缓存，标记为脏数据，延迟写入数据库
func (wb *WriteBackCache) Set(key, value string) error {
	fmt.Printf("\n✏️ Write-Back 写入: %s = %s\n", key, value)
	
	wb.mu.Lock()
	defer wb.mu.Unlock()
	
	// 1. 写入缓存
	wb.cache.Set(key, value)
	
	// 2. 标记为脏数据
	wb.dirtyData[key] = &DirtyItem{
		Key:       key,
		Value:     value,
		Timestamp: time.Now(),
		IsDirty:   true,
	}
	
	fmt.Printf("🏷️ 标记为脏数据，等待后台刷新到数据库\n")
	
	return nil
}

// Delete Write-Back删除
func (wb *WriteBackCache) Delete(key string) error {
	fmt.Printf("\n🗑️ Write-Back 删除: %s\n", key)
	
	wb.mu.Lock()
	defer wb.mu.Unlock()
	
	// 1. 删除缓存
	wb.cache.Delete(key)
	
	// 2. 如果是脏数据，也要删除
	delete(wb.dirtyData, key)
	
	// 3. 立即删除数据库（删除操作通常立即执行）
	wb.db.Delete(key)
	
	return nil
}

// Flush 手动刷新脏数据到数据库
func (wb *WriteBackCache) Flush() error {
	fmt.Printf("\n🔄 手动刷新脏数据到数据库\n")
	
	wb.mu.Lock()
	defer wb.mu.Unlock()
	
	if len(wb.dirtyData) == 0 {
		fmt.Println("没有脏数据需要刷新")
		return nil
	}
	
	// 批量写入数据库
	for key, item := range wb.dirtyData {
		if item.IsDirty {
			wb.db.Set(key, item.Value)
			item.IsDirty = false
			fmt.Printf("✅ 刷新到数据库: %s = %s\n", key, item.Value)
		}
	}
	
	// 清理已刷新的数据
	wb.dirtyData = make(map[string]*DirtyItem)
	
	return nil
}

// GetDirtyCount 获取脏数据数量
func (wb *WriteBackCache) GetDirtyCount() int {
	wb.mu.RLock()
	defer wb.mu.RUnlock()
	
	count := 0
	for _, item := range wb.dirtyData {
		if item.IsDirty {
			count++
		}
	}
	return count
}

// Close 关闭Write-Back缓存
func (wb *WriteBackCache) Close() error {
	fmt.Println("\n🔒 关闭Write-Back缓存，刷新所有脏数据")
	
	// 刷新所有脏数据
	wb.Flush()
	
	// 停止后台刷新
	close(wb.stopCh)
	
	return nil
}

// backgroundFlush 后台定期刷新脏数据
func (wb *WriteBackCache) backgroundFlush() {
	ticker := time.NewTicker(wb.flushInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			wb.autoFlush()
		case <-wb.stopCh:
			return
		}
	}
}

// autoFlush 自动刷新脏数据
func (wb *WriteBackCache) autoFlush() {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	
	if len(wb.dirtyData) == 0 {
		return
	}
	
	fmt.Printf("\n⏰ 后台自动刷新脏数据 (数量: %d)\n", len(wb.dirtyData))
	
	// 批量写入数据库
	for key, item := range wb.dirtyData {
		if item.IsDirty {
			wb.db.Set(key, item.Value)
			item.IsDirty = false
		}
	}
	
	// 清理已刷新的数据
	wb.dirtyData = make(map[string]*DirtyItem)
}

// DemoWriteBack 演示Write-Back模式
func DemoWriteBack() {
	fmt.Println("=== Write-Back 模式演示 ===")
	
	// 创建Write-Back缓存，每3秒刷新一次
	cache := NewWriteBackCache(3 * time.Second)
	defer cache.Close()
	
	// 演示写入流程
	fmt.Println("\n--- 写入流程演示 ---")
	
	start := time.Now()
	cache.Set("user:1", "张三")
	cache.Set("user:2", "李四")
	cache.Set("user:3", "王五")
	writeTime := time.Since(start)
	fmt.Printf("Write-Back 3次写入耗时: %v\n", writeTime)
	fmt.Printf("脏数据数量: %d\n", cache.GetDirtyCount())
	
	// 演示读取流程
	fmt.Println("\n--- 读取流程演示 ---")
	
	// 读取缓存中的数据（非常快）
	start = time.Now()
	value1, _ := cache.Get("user:1")
	readTime := time.Since(start)
	fmt.Printf("读取结果: %s, 耗时: %v\n", value1, readTime)
	
	// 演示数据状态
	fmt.Println("\n--- 数据状态检查 ---")
	
	// 检查缓存
	cacheValue, _ := cache.cache.Get("user:1")
	fmt.Printf("缓存中的值: %s\n", cacheValue)
	
	// 检查数据库（此时可能还没有刷新）
	dbValue, exists := cache.db.Get("user:1")
	if exists {
		fmt.Printf("数据库中的值: %s\n", dbValue)
	} else {
		fmt.Println("数据库中暂无数据（还未刷新）")
	}
	
	// 手动刷新
	fmt.Println("\n--- 手动刷新演示 ---")
	cache.Flush()
	
	// 再次检查数据库
	dbValue, _ = cache.db.Get("user:1")
	fmt.Printf("刷新后数据库中的值: %s\n", dbValue)
}

// DemoWriteBackPerformance 演示Write-Back性能优势
func DemoWriteBackPerformance() {
	fmt.Println("\n=== Write-Back 性能优势演示 ===")
	
	cache := NewWriteBackCache(5 * time.Second)
	defer cache.Close()
	
	// 大量写入测试
	fmt.Println("\n执行大量写入操作:")
	start := time.Now()
	
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("batch_user:%d", i)
		value := fmt.Sprintf("用户%d", i)
		cache.Set(key, value)
	}
	
	batchWriteTime := time.Since(start)
	fmt.Printf("Write-Back 10次写入耗时: %v\n", batchWriteTime)
	fmt.Printf("平均每次写入: %v\n", batchWriteTime/10)
	fmt.Printf("脏数据数量: %d\n", cache.GetDirtyCount())
	
	// 等待后台刷新
	fmt.Println("\n等待后台自动刷新...")
	time.Sleep(6 * time.Second)
	
	fmt.Printf("刷新后脏数据数量: %d\n", cache.GetDirtyCount())
	
	fmt.Println("\n💡 Write-Back模式的优势:")
	fmt.Println("   1. 写入性能最好：只写缓存，延迟写数据库")
	fmt.Println("   2. 批量操作：可以将多次写入合并为一次数据库操作")
	fmt.Println("   3. 减少数据库负载：降低数据库写入频率")
	
	fmt.Println("\n💡 Write-Back模式的风险:")
	fmt.Println("   1. 数据丢失：缓存故障可能导致未刷新的数据丢失")
	fmt.Println("   2. 一致性问题：缓存和数据库可能短期不一致")
	fmt.Println("   3. 复杂性：需要处理脏数据管理和刷新策略")
}

// DemoDataLossRisk 演示数据丢失风险
func DemoDataLossRisk() {
	fmt.Println("\n=== 数据丢失风险演示 ===")
	
	cache := NewWriteBackCache(10 * time.Second) // 较长的刷新间隔
	
	// 写入一些数据
	cache.Set("important:1", "重要数据1")
	cache.Set("important:2", "重要数据2")
	
	fmt.Printf("写入重要数据，脏数据数量: %d\n", cache.GetDirtyCount())
	
	// 模拟缓存故障（不调用Close，直接丢弃）
	fmt.Println("\n⚠️ 模拟缓存故障（数据丢失）")
	cache = nil // 模拟缓存崩溃
	
	// 创建新的缓存实例
	newCache := NewWriteBackCache(3 * time.Second)
	defer newCache.Close()
	
	// 尝试读取数据
	fmt.Println("\n尝试从新缓存实例读取数据:")
	_, err1 := newCache.Get("important:1")
	_, err2 := newCache.Get("important:2")
	
	if err1 != nil && err2 != nil {
		fmt.Println("❌ 数据丢失：重要数据无法找到")
		fmt.Println("   原因：缓存故障时，未刷新的脏数据丢失")
	}
	
	fmt.Println("\n💡 防止数据丢失的策略:")
	fmt.Println("   1. 缩短刷新间隔")
	fmt.Println("   2. 实现缓存持久化")
	fmt.Println("   3. 使用主从复制")
	fmt.Println("   4. 关键数据立即刷新")
	fmt.Println("   5. 应用层做好容错处理")
}
