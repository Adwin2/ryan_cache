package main

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// 第一个测试：基本的存储和获取功能
func TestCache_SetAndGet(t *testing.T) {
	// 创建缓存实例
	cache := NewSimpleCache()
	
	// 存储一个键值对
	cache.Set("key1", "value1")
	
	// 获取并验证
	value, exists := cache.Get("key1")
	if !exists {
		t.Error("期望键 'key1' 存在，但实际不存在")
	}
	if value != "value1" {
		t.Errorf("期望值为 'value1'，但实际为 '%s'", value)
	}
}

// 第二个测试：获取不存在的键
func TestCache_GetNonExistentKey(t *testing.T) {
	cache := NewSimpleCache()

	// 获取不存在的键
	_, exists := cache.Get("nonexistent")
	if exists {
		t.Error("期望键 'nonexistent' 不存在，但实际存在")
	}
}

// 第三个测试：删除存在的键
func TestCache_DeleteExistingKey(t *testing.T) {
	cache := NewSimpleCache()

	// 先设置一个键值对
	cache.Set("key1", "value1")

	// 验证键存在
	_, exists := cache.Get("key1")
	if !exists {
		t.Error("设置后键应该存在")
	}

	// 删除键
	deleted := cache.Delete("key1")
	if !deleted {
		t.Error("删除存在的键应该返回true")
	}

	// 验证键已被删除
	_, exists = cache.Get("key1")
	if exists {
		t.Error("删除后键不应该存在")
	}
}

// 第四个测试：删除不存在的键
func TestCache_DeleteNonExistentKey(t *testing.T) {
	cache := NewSimpleCache()

	// 删除不存在的键
	deleted := cache.Delete("nonexistent")
	if deleted {
		t.Error("删除不存在的键应该返回false")
	}
}

// 第五个测试：缓存大小统计
func TestCache_Size(t *testing.T) {
	cache := NewSimpleCache()

	// 初始大小应该为0
	if cache.Size() != 0 {
		t.Errorf("初始缓存大小应该为0，实际为%d", cache.Size())
	}

	// 添加元素后大小增加
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	if cache.Size() != 2 {
		t.Errorf("添加2个元素后大小应该为2，实际为%d", cache.Size())
	}

	// 删除元素后大小减少
	cache.Delete("key1")
	if cache.Size() != 1 {
		t.Errorf("删除1个元素后大小应该为1，实际为%d", cache.Size())
	}

	// 覆盖已存在的键，大小不变
	cache.Set("key2", "new_value2")
	if cache.Size() != 1 {
		t.Errorf("覆盖已存在键后大小应该为1，实际为%d", cache.Size())
	}
}

// 第六个测试：并发安全性
func TestCache_ConcurrentAccess(t *testing.T) {
	cache := NewSimpleCache()
	var wg sync.WaitGroup

	// 并发写入
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key_%d_%d", id, j)
				value := fmt.Sprintf("value_%d_%d", id, j)
				cache.Set(key, value)
			}
		}(i)
	}

	// 并发读取
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("key_%d_%d", id%100, j)
				cache.Get(key)
			}
		}(i)
	}

	// 并发删除
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				key := fmt.Sprintf("key_%d_%d", id%100, j)
				cache.Delete(key)
			}
		}(i)
	}

	wg.Wait()

	// 如果没有竞态条件，程序应该正常结束
	t.Logf("并发测试完成，最终缓存大小: %d", cache.Size())
}

// 第七个测试：TTL过期功能
func TestCache_TTL(t *testing.T) {
	cache := NewSimpleCache()

	// 设置一个1秒后过期的键
	cache.SetWithTTL("temp_key", "temp_value", 1*time.Second)

	// 立即获取应该存在
	if value, exists := cache.Get("temp_key"); !exists || value != "temp_value" {
		t.Error("设置TTL后立即获取应该成功")
	}

	// 等待1.5秒后应该过期
	time.Sleep(1500 * time.Millisecond)
	if _, exists := cache.Get("temp_key"); exists {
		t.Error("TTL过期后键应该不存在")
	}
}

// 第八个测试：TTL不影响普通键
func TestCache_TTLMixedWithNormalKeys(t *testing.T) {
	cache := NewSimpleCache()

	// 设置普通键（无过期时间）
	cache.Set("normal_key", "normal_value")

	// 设置TTL键
	cache.SetWithTTL("ttl_key", "ttl_value", 500*time.Millisecond)

	// 等待TTL过期
	time.Sleep(600 * time.Millisecond)

	// 普通键应该还存在
	if _, exists := cache.Get("normal_key"); !exists {
		t.Error("普通键不应该受TTL影响")
	}

	// TTL键应该过期
	if _, exists := cache.Get("ttl_key"); exists {
		t.Error("TTL键应该已过期")
	}
}

// 第九个测试：LRU基本功能
func TestCache_LRU_Basic(t *testing.T) {
	// 创建容量为3的LRU缓存
	cache := NewLRUCache(3)

	// 添加3个元素
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// 验证所有元素都存在
	if cache.Size() != 3 {
		t.Errorf("期望大小为3，实际为%d", cache.Size())
	}

	// 添加第4个元素，应该淘汰最久未使用的key1
	cache.Set("key4", "value4")

	if cache.Size() != 3 {
		t.Errorf("LRU缓存大小应该保持为3，实际为%d", cache.Size())
	}

	// key1应该被淘汰
	if _, exists := cache.Get("key1"); exists {
		t.Error("key1应该被LRU淘汰")
	}

	// 其他键应该还存在
	if _, exists := cache.Get("key2"); !exists {
		t.Error("key2应该还存在")
	}
	if _, exists := cache.Get("key3"); !exists {
		t.Error("key3应该还存在")
	}
	if _, exists := cache.Get("key4"); !exists {
		t.Error("key4应该存在")
	}
}

// 第十个测试：LRU访问更新
func TestCache_LRU_AccessUpdate(t *testing.T) {
	cache := NewLRUCache(3)

	// 添加3个元素
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// 访问key1，使其变为最近使用
	cache.Get("key1")

	// 添加新元素，应该淘汰key2（现在是最久未使用的）
	cache.Set("key4", "value4")

	// key1应该还存在（因为刚被访问）
	if _, exists := cache.Get("key1"); !exists {
		t.Error("key1应该还存在，因为刚被访问")
	}

	// key2应该被淘汰
	if _, exists := cache.Get("key2"); exists {
		t.Error("key2应该被LRU淘汰")
	}
}

// 第十一个测试：LRU缓存集成统计功能
func TestLRUCache_WithStats(t *testing.T) {
	cache := NewLRUCache(3)

	// 初始统计应该为0
	stats := cache.GetStats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.TotalRequests != 0 {
		t.Error("初始统计应该全为0")
	}

	// 设置一些数据
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	// 命中测试
	cache.Get("key1") // 命中
	cache.Get("key1") // 命中
	cache.Get("key3") // 未命中

	stats = cache.GetStats()
	if stats.Hits != 2 {
		t.Errorf("期望命中次数为2，实际为%d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("期望未命中次数为1，实际为%d", stats.Misses)
	}
	if stats.TotalRequests != 3 {
		t.Errorf("期望总请求数为3，实际为%d", stats.TotalRequests)
	}

	// 验证命中率
	expectedHitRate := float64(2) / float64(3)
	if abs(stats.HitRate()-expectedHitRate) > 0.001 {
		t.Errorf("期望命中率为%.3f，实际为%.3f", expectedHitRate, stats.HitRate())
	}
}

// 第十二个测试：LRU缓存集成内存限制功能
func TestLRUCache_WithMemoryLimit(t *testing.T) {
	// 创建容量为10，内存限制为100字节的LRU缓存
	cache := NewLRUCacheWithMemoryLimit(10, 100)

	// 添加一些数据
	cache.Set("key1", "value1") // 约12字节
	cache.Set("key2", "value2") // 约12字节

	// 获取内存使用情况
	memUsage := cache.GetMemoryUsage()
	if memUsage <= 0 {
		t.Error("内存使用量应该大于0")
	}

	// 添加大量数据直到超过内存限制
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("large_key_%d", i)
		value := fmt.Sprintf("large_value_%d_with_more_data", i)
		cache.Set(key, value)
	}

	// 验证内存使用量不超过限制
	finalMemUsage := cache.GetMemoryUsage()
	if finalMemUsage > 100 {
		t.Errorf("内存使用量%d字节超过了限制100字节", finalMemUsage)
	}

	// 验证一些早期的键被淘汰了
	if _, exists := cache.Get("key1"); exists {
		t.Error("早期的键应该因为内存限制被淘汰")
	}
}

// 第十三个测试：批量操作功能
func TestLRUCache_BatchOperations(t *testing.T) {
	cache := NewLRUCache(10)

	// 测试SetMulti
	data := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
	}

	cache.SetMulti(data)

	// 验证所有键都被设置
	if cache.Size() != 4 {
		t.Errorf("期望缓存大小为4，实际为%d", cache.Size())
	}

	// 测试GetMulti
	keys := []string{"key1", "key2", "key3", "key5"} // key5不存在
	results := cache.GetMulti(keys)

	// 验证结果
	if len(results) != 4 {
		t.Errorf("期望返回4个结果，实际为%d", len(results))
	}

	if results["key1"] != "value1" {
		t.Errorf("key1的值不正确")
	}
	if results["key2"] != "value2" {
		t.Errorf("key2的值不正确")
	}
	if results["key3"] != "value3" {
		t.Errorf("key3的值不正确")
	}
	if _, exists := results["key5"]; exists {
		t.Error("key5不应该存在于结果中")
	}

	// 测试DeleteMulti
	deleteKeys := []string{"key1", "key3", "key5"} // key5不存在
	deletedCount := cache.DeleteMulti(deleteKeys)

	if deletedCount != 2 {
		t.Errorf("期望删除2个键，实际删除%d个", deletedCount)
	}

	// 验证删除结果
	if cache.Size() != 2 {
		t.Errorf("删除后期望缓存大小为2，实际为%d", cache.Size())
	}

	if _, exists := cache.Get("key1"); exists {
		t.Error("key1应该被删除")
	}
	if _, exists := cache.Get("key3"); exists {
		t.Error("key3应该被删除")
	}
	if _, exists := cache.Get("key2"); !exists {
		t.Error("key2应该还存在")
	}
}

// 辅助函数：计算浮点数绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
