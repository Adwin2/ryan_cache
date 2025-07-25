package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
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
	if len(results) != 3 {
		t.Errorf("期望返回3个结果，实际为%d", len(results))
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

// 第十四个测试：HTTP API服务
func TestCacheServer_HTTPEndpoints(t *testing.T) {
	// 创建缓存服务器
	server := NewCacheServer(NewLRUCache(10))

	// 测试SET操作
	setData := map[string]interface{}{
		"key":   "test_key",
		"value": "test_value",
	}
	setBody, _ := json.Marshal(setData)

	req := httptest.NewRequest("POST", "/cache", bytes.NewBuffer(setBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("SET操作期望状态码200，实际为%d", w.Code)
	}

	// 测试GET操作
	req = httptest.NewRequest("GET", "/cache/test_key", nil)
	w = httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET操作期望状态码200，实际为%d", w.Code)
	}

	var getResponse map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &getResponse)

	if getResponse["value"] != "test_value" {
		t.Errorf("期望值为'test_value'，实际为'%v'", getResponse["value"])
	}

	// 测试DELETE操作
	req = httptest.NewRequest("DELETE", "/cache/test_key", nil)
	w = httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("DELETE操作期望状态码200，实际为%d", w.Code)
	}

	// 验证删除后GET返回404
	req = httptest.NewRequest("GET", "/cache/test_key", nil)
	w = httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("删除后GET操作期望状态码404，实际为%d", w.Code)
	}
}

// 第十五个测试：统计API
func TestCacheServer_StatsEndpoint(t *testing.T) {
	server := NewCacheServer(NewLRUCache(10))

	// 先进行一些操作产生统计数据
	setData := map[string]interface{}{
		"key":   "stats_key",
		"value": "stats_value",
	}
	setBody, _ := json.Marshal(setData)

	// SET操作
	req := httptest.NewRequest("POST", "/cache", bytes.NewBuffer(setBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	// GET操作（命中）
	req = httptest.NewRequest("GET", "/cache/stats_key", nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	// GET操作（未命中）
	req = httptest.NewRequest("GET", "/cache/nonexistent", nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	// 获取统计信息
	req = httptest.NewRequest("GET", "/stats", nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("统计API期望状态码200，实际为%d", w.Code)
	}

	var stats map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &stats)

	if stats["hits"].(float64) != 1 {
		t.Errorf("期望命中次数为1，实际为%v", stats["hits"])
	}
	if stats["misses"].(float64) != 1 {
		t.Errorf("期望未命中次数为1，实际为%v", stats["misses"])
	}
}

// 第十六个测试：性能基准测试
func BenchmarkLRUCache_Set(b *testing.B) {
	cache := NewLRUCache(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%1000)
		value := fmt.Sprintf("value_%d", i)
		cache.Set(key, value)
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	cache := NewLRUCache(1000)

	// 预填充数据
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		cache.Set(key, value)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%1000)
		cache.Get(key)
	}
}

func BenchmarkLRUCache_SetParallel(b *testing.B) {
	cache := NewLRUCache(10000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key_%d", i%10000)
			value := fmt.Sprintf("value_%d", i)
			cache.Set(key, value)
			i++
		}
	})
}

func BenchmarkLRUCache_GetParallel(b *testing.B) {
	cache := NewLRUCache(10000)

	// 预填充数据
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		cache.Set(key, value)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key_%d", i%10000)
			cache.Get(key)
			i++
		}
	})
}

// 对比测试：标准库map vs LRU缓存
func BenchmarkStdMap_Set(b *testing.B) {
	m := make(map[string]string)
	var mu sync.RWMutex

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%1000)
		value := fmt.Sprintf("value_%d", i)

		mu.Lock()
		m[key] = value
		mu.Unlock()
	}
}

func BenchmarkStdMap_Get(b *testing.B) {
	m := make(map[string]string)
	var mu sync.RWMutex

	// 预填充数据
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		m[key] = value
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%d", i%1000)

		mu.RLock()
		_ = m[key]
		mu.RUnlock()
	}
}

// 第十七个测试：LRU缓存异步过期清理
func TestLRUCache_AsyncExpiration(t *testing.T) {
	// 创建带异步清理的LRU缓存，容量10，清理间隔100ms
	cache := NewLRUCacheWithCleanup(10, 100*time.Millisecond)
	defer cache.Close()

	// 设置一些会过期的键
	startTime := time.Now()
	cache.SetWithTTL("key1", "value1", 200*time.Millisecond)
	cache.SetWithTTL("key2", "value2", 300*time.Millisecond)
	cache.SetWithTTL("key3", "value3", 500*time.Millisecond) // 增加到500ms

	// 立即检查都存在
	if cache.Size() != 3 {
		t.Errorf("期望缓存大小为3，实际为%d", cache.Size())
	}

	// 等待250ms，key1应该被异步清理
	time.Sleep(250 * time.Millisecond)
	t.Logf("250ms后，经过时间: %v", time.Since(startTime))

	if cache.Size() != 2 {
		t.Errorf("250ms后期望缓存大小为2，实际为%d", cache.Size())
	}

	if _, exists := cache.Get("key1"); exists {
		t.Error("key1应该被异步清理")
	}

	// 等待到350ms，key2也应该被清理
	time.Sleep(100 * time.Millisecond)
	t.Logf("350ms后，经过时间: %v", time.Since(startTime))

	if cache.Size() != 1 {
		t.Errorf("350ms后期望缓存大小为1，实际为%d", cache.Size())
	}

	// key3应该还存在
	if _, exists := cache.Get("key3"); !exists {
		t.Error("key3应该还存在")
	}
}

// 第十八个测试：LRU缓存清理统计
func TestLRUCache_CleanupStats(t *testing.T) {
	cache := NewLRUCacheWithCleanup(20, 50*time.Millisecond)
	defer cache.Close()

	// 设置一些会快速过期的键
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key_%d", i)
		cache.SetWithTTL(key, "value", 100*time.Millisecond)
	}

	// 等待清理完成
	time.Sleep(200 * time.Millisecond)

	// 获取清理统计
	cleanupStats := cache.GetCleanupStats()
	if cleanupStats.CleanedKeys < 10 {
		t.Errorf("期望清理至少10个键，实际清理%d个", cleanupStats.CleanedKeys)
	}

	if cleanupStats.CleanupRuns == 0 {
		t.Error("期望至少运行过一次清理")
	}
}

// 第十九个测试：分布式缓存 - 一致性哈希基础功能
func TestDistributedCache_ConsistentHashing(t *testing.T) {
	// 创建分布式缓存集群，3个节点
	cluster := NewDistributedCache([]string{"node1:8001", "node2:8002", "node3:8003"})

	// 测试键的分布是否一致
	key1 := "user:1001"
	key2 := "user:1002"
	key3 := "user:1003"

	// 同一个键应该总是路由到同一个节点
	node1_1 := cluster.GetNodeForKey(key1)
	node1_2 := cluster.GetNodeForKey(key1)
	if node1_1 != node1_2 {
		t.Errorf("同一个键应该路由到同一个节点，但得到了不同的节点: %s vs %s", node1_1, node1_2)
	}

	// 不同的键可能路由到不同的节点（但不是必须的）
	node2 := cluster.GetNodeForKey(key2)
	node3 := cluster.GetNodeForKey(key3)

	t.Logf("键分布: %s->%s, %s->%s, %s->%s", key1, node1_1, key2, node2, key3, node3)
}

// 第二十个测试：分布式缓存 - 节点添加和删除
func TestDistributedCache_NodeAddRemove(t *testing.T) {
	// 初始3个节点
	initialNodes := []string{"node1:8001", "node2:8002", "node3:8003"}
	cluster := NewDistributedCache(initialNodes)

	// 记录一些键的初始分布
	testKeys := []string{"key1", "key2", "key3", "key4", "key5"}
	initialMapping := make(map[string]string)
	for _, key := range testKeys {
		initialMapping[key] = cluster.GetNodeForKey(key)
	}

	// 添加新节点
	cluster.AddNode("node4:8004")

	// 检查键重新分布后，大部分键的映射应该保持不变（一致性哈希的优势）
	unchangedCount := 0
	for _, key := range testKeys {
		newNode := cluster.GetNodeForKey(key)
		if initialMapping[key] == newNode {
			unchangedCount++
		}
	}

	// 一致性哈希应该保证大部分键不需要重新分布
	if unchangedCount < 3 { // 至少60%的键保持不变
		t.Errorf("添加节点后，应该有至少3个键保持在原节点，实际只有%d个", unchangedCount)
	}

	// 移除节点
	cluster.RemoveNode("node2:8002")

	// 被移除节点的键应该重新分布到其他节点
	for _, key := range testKeys {
		node := cluster.GetNodeForKey(key)
		if node == "node2:8002" {
			t.Errorf("键%s仍然路由到已删除的节点node2:8002", key)
		}
	}
}

// 第二十一个测试：分布式缓存 - 虚拟节点提高负载均衡
func TestDistributedCache_VirtualNodes(t *testing.T) {
	// 创建带虚拟节点的分布式缓存
	cluster := NewDistributedCacheWithVirtualNodes([]string{"node1:8001", "node2:8002"}, 150)

	// 测试大量键的分布是否相对均匀
	keyCount := 1000
	nodeDistribution := make(map[string]int)

	for i := 0; i < keyCount; i++ {
		key := fmt.Sprintf("key_%d", i)
		node := cluster.GetNodeForKey(key)
		nodeDistribution[node]++
	}

	// 检查负载是否相对均衡（每个节点应该处理大约50%的键）
	for node, count := range nodeDistribution {
		percentage := float64(count) / float64(keyCount) * 100
		t.Logf("节点%s处理了%.1f%%的键(%d个)", node, percentage, count)

		// 允许30%-70%的范围（虚拟节点应该提供更好的负载均衡）
		if percentage < 30 || percentage > 70 {
			t.Errorf("节点%s的负载不均衡: %.1f%%", node, percentage)
		}
	}
}

// 第二十二个测试：分布式缓存 - 完整的读写操作
func TestDistributedCache_ReadWrite(t *testing.T) {
	// 创建分布式缓存（模拟本地集群）
	cluster := NewDistributedCache([]string{"node1:8001", "node2:8002", "node3:8003"})

	// 设置一些键值对
	testData := map[string]string{
		"user:1001": "张三",
		"user:1002": "李四",
		"user:1003": "王五",
		"product:2001": "iPhone",
		"product:2002": "MacBook",
	}

	// 写入数据
	for key, value := range testData {
		err := cluster.Set(key, value)
		if err != nil {
			t.Errorf("设置键%s失败: %v", key, err)
		}
	}

	// 读取数据
	for key, expectedValue := range testData {
		value, exists, err := cluster.Get(key)
		if err != nil {
			t.Errorf("获取键%s失败: %v", key, err)
		}
		if !exists {
			t.Errorf("键%s应该存在", key)
		}
		if value != expectedValue {
			t.Errorf("键%s的值不正确，期望%s，实际%s", key, expectedValue, value)
		}
	}

	// 删除数据
	err := cluster.Delete("user:1001")
	if err != nil {
		t.Errorf("删除键user:1001失败: %v", err)
	}

	// 验证删除
	_, exists, err := cluster.Get("user:1001")
	if err != nil {
		t.Errorf("检查已删除键失败: %v", err)
	}
	if exists {
		t.Error("键user:1001应该已被删除")
	}
}

// 第二十三个测试：普通哈希 vs 一致性哈希 - 数据迁移对比
func TestHashingComparison_DataMigration(t *testing.T) {
	t.Log("=== 普通哈希 vs 一致性哈希：数据迁移对比 ===")

	// 测试数据：1000个键
	testKeys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		testKeys[i] = fmt.Sprintf("key_%d", i)
	}

	// 初始3个节点
	initialNodes := []string{"node1", "node2", "node3"}

	// === 普通哈希测试 ===
	t.Log("\n--- 普通哈希算法 ---")

	// 记录初始分布（普通哈希：key_hash % node_count）
	normalHashInitial := make(map[string]string)
	for _, key := range testKeys {
		nodeIndex := simpleHash(key) % len(initialNodes)
		normalHashInitial[key] = initialNodes[nodeIndex]
	}

	// 添加第4个节点后的分布
	newNodes := append(initialNodes, "node4")
	normalHashAfter := make(map[string]string)
	for _, key := range testKeys {
		nodeIndex := simpleHash(key) % len(newNodes)
		normalHashAfter[key] = newNodes[nodeIndex]
	}

	// 计算普通哈希的数据迁移量
	normalHashMigrations := 0
	for _, key := range testKeys {
		if normalHashInitial[key] != normalHashAfter[key] {
			normalHashMigrations++
		}
	}
	normalHashMigrationRate := float64(normalHashMigrations) / float64(len(testKeys)) * 100

	t.Logf("普通哈希：需要迁移 %d/%d 个键 (%.1f%%)",
		normalHashMigrations, len(testKeys), normalHashMigrationRate)

	// === 一致性哈希测试 ===
	t.Log("\n--- 一致性哈希算法 ---")

	// 使用我们的分布式缓存
	cluster := NewDistributedCache(initialNodes)

	// 记录初始分布
	consistentHashInitial := make(map[string]string)
	for _, key := range testKeys {
		consistentHashInitial[key] = cluster.GetNodeForKey(key)
	}

	// 添加第4个节点
	cluster.AddNode("node4")

	// 记录添加节点后的分布
	consistentHashAfter := make(map[string]string)
	for _, key := range testKeys {
		consistentHashAfter[key] = cluster.GetNodeForKey(key)
	}

	// 计算一致性哈希的数据迁移量
	consistentHashMigrations := 0
	for _, key := range testKeys {
		if consistentHashInitial[key] != consistentHashAfter[key] {
			consistentHashMigrations++
		}
	}
	consistentHashMigrationRate := float64(consistentHashMigrations) / float64(len(testKeys)) * 100

	t.Logf("一致性哈希：需要迁移 %d/%d 个键 (%.1f%%)",
		consistentHashMigrations, len(testKeys), consistentHashMigrationRate)

	// === 结果对比 ===
	t.Log("\n--- 对比结果 ---")
	improvement := normalHashMigrationRate - consistentHashMigrationRate
	t.Logf("一致性哈希减少了 %.1f%% 的数据迁移", improvement)

	// 理论上，一致性哈希应该只迁移约 1/n 的数据（n为节点数）
	theoreticalMigration := 100.0 / float64(len(newNodes)) // 25%
	t.Logf("理论最优迁移率：%.1f%%", theoreticalMigration)

	// 验证一致性哈希的优势
	if consistentHashMigrationRate >= normalHashMigrationRate {
		t.Errorf("一致性哈希应该比普通哈希有更少的数据迁移")
	}

	// 一致性哈希的迁移率应该接近理论值
	if consistentHashMigrationRate > theoreticalMigration*2 {
		t.Errorf("一致性哈希迁移率过高：%.1f%% > %.1f%%",
			consistentHashMigrationRate, theoreticalMigration*2)
	}
}

// 第二十四个测试：负载均衡对比
func TestHashingComparison_LoadBalance(t *testing.T) {
	t.Log("=== 普通哈希 vs 一致性哈希：负载均衡对比 ===")

	testKeys := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		testKeys[i] = fmt.Sprintf("user_%d", i)
	}

	nodes := []string{"node1", "node2", "node3"}

	// === 普通哈希负载分布 ===
	normalDistribution := make(map[string]int)
	for _, node := range nodes {
		normalDistribution[node] = 0
	}

	for _, key := range testKeys {
		nodeIndex := simpleHash(key) % len(nodes)
		normalDistribution[nodes[nodeIndex]]++
	}

	t.Log("\n--- 普通哈希负载分布 ---")
	for node, count := range normalDistribution {
		percentage := float64(count) / float64(len(testKeys)) * 100
		t.Logf("%s: %d 个键 (%.1f%%)", node, count, percentage)
	}

	// === 一致性哈希负载分布 ===
	cluster := NewDistributedCache(nodes)
	consistentDistribution := make(map[string]int)
	for _, node := range nodes {
		consistentDistribution[node] = 0
	}

	for _, key := range testKeys {
		node := cluster.GetNodeForKey(key)
		consistentDistribution[node]++
	}

	t.Log("\n--- 一致性哈希负载分布 ---")
	for node, count := range consistentDistribution {
		percentage := float64(count) / float64(len(testKeys)) * 100
		t.Logf("%s: %d 个键 (%.1f%%)", node, count, percentage)
	}

	// 计算负载均衡度（标准差）
	normalStdDev := calculateStandardDeviation(normalDistribution, len(testKeys))
	consistentStdDev := calculateStandardDeviation(consistentDistribution, len(testKeys))

	t.Logf("\n负载均衡度（标准差越小越好）：")
	t.Logf("普通哈希：%.2f", normalStdDev)
	t.Logf("一致性哈希：%.2f", consistentStdDev)
}

// 简单哈希函数（模拟普通哈希）
func simpleHash(key string) int {
	hash := 0
	for _, char := range key {
		hash = hash*31 + int(char)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// 计算标准差
func calculateStandardDeviation(distribution map[string]int, total int) float64 {
	mean := float64(total) / float64(len(distribution))
	variance := 0.0

	for _, count := range distribution {
		diff := float64(count) - mean
		variance += diff * diff
	}
	variance /= float64(len(distribution))

	return math.Sqrt(variance)
}





// 第三十个测试：基础数据迁移功能 - 简历优化版
func TestBasicMigration_AddNodeWithDataMigration(t *testing.T) {
	t.Log("=== 基础数据迁移测试 - 添加节点 ===")

	// 创建初始集群
	cluster := NewDistributedCache([]string{"node1:8001", "node2:8002", "node3:8003"})

	// 写入测试数据
	testData := map[string]string{
		"user:1001": "张三",
		"user:1002": "李四",
		"user:1003": "王五",
		"product:2001": "iPhone",
		"product:2002": "MacBook",
		"order:3001": "订单A",
		"order:3002": "订单B",
		"order:3003": "订单C",
	}

	// 写入数据
	for key, value := range testData {
		err := cluster.Set(key, value)
		if err != nil {
			t.Fatalf("写入数据失败: %v", err)
		}
	}

	// 验证数据写入成功
	for key, expectedValue := range testData {
		value, exists, err := cluster.Get(key)
		if err != nil || !exists || value != expectedValue {
			t.Fatalf("数据验证失败: key=%s", key)
		}
	}

	// 记录添加节点前的数据分布
	beforeDistribution := make(map[string]string)
	for key := range testData {
		beforeDistribution[key] = cluster.GetNodeForKey(key)
	}

	t.Logf("添加节点前的数据分布: %+v", beforeDistribution)

	// 添加新节点 - 这应该触发基础数据迁移
	err := cluster.AddNodeWithBasicMigration("node4:8004")
	if err != nil {
		t.Fatalf("添加节点失败: %v", err)
	}

	// 验证所有数据在迁移后仍然可以正确读取
	for key, expectedValue := range testData {
		value, exists, err := cluster.Get(key)
		if err != nil {
			t.Errorf("迁移后读取失败: key=%s, error=%v", key, err)
		}
		if !exists {
			t.Errorf("迁移后数据丢失: key=%s", key)
		}
		if value != expectedValue {
			t.Errorf("迁移后数据错误: key=%s, expected=%s, got=%s",
				key, expectedValue, value)
		}
	}

	// 记录添加节点后的数据分布
	afterDistribution := make(map[string]string)
	for key := range testData {
		afterDistribution[key] = cluster.GetNodeForKey(key)
	}

	t.Logf("添加节点后的数据分布: %+v", afterDistribution)

	// 验证确实有数据被迁移到新节点
	newNodeHasData := false
	for key := range testData {
		if afterDistribution[key] == "node4:8004" {
			newNodeHasData = true
			t.Logf("键 %s 被迁移到新节点", key)
		}
	}

	if !newNodeHasData {
		t.Error("新节点应该承担一部分数据")
	}

	// 获取迁移统计
	stats := cluster.GetBasicMigrationStats()
	t.Logf("迁移统计: 迁移键数=%d, 耗时=%v",
		stats.MigratedKeys, stats.Duration)

	if stats.MigratedKeys == 0 {
		t.Error("应该有键被迁移")
	}
}

// 第三十一个测试：基础数据迁移功能 - 移除节点
func TestBasicMigration_RemoveNodeWithDataMigration(t *testing.T) {
	t.Log("=== 基础数据迁移测试 - 移除节点 ===")

	// 创建4节点集群
	cluster := NewDistributedCache([]string{"node1:8001", "node2:8002", "node3:8003", "node4:8004"})

	// 写入测试数据
	testData := make(map[string]string)
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		testData[key] = value
		cluster.Set(key, value)
	}

	// 找出存储在node2上的数据
	node2Data := make(map[string]string)
	for key, value := range testData {
		if cluster.GetNodeForKey(key) == "node2:8002" {
			node2Data[key] = value
		}
	}

	if len(node2Data) == 0 {
		t.Skip("node2上没有数据，跳过测试")
	}

	t.Logf("node2上有 %d 个键需要迁移: %v", len(node2Data), getKeys(node2Data))

	// 移除node2 - 这应该触发数据迁移
	err := cluster.RemoveNodeWithBasicMigration("node2:8002")
	if err != nil {
		t.Fatalf("移除节点失败: %v", err)
	}

	// 验证原本在node2上的数据被迁移到其他节点
	for key, expectedValue := range node2Data {
		value, exists, err := cluster.Get(key)
		if err != nil {
			t.Errorf("迁移后读取失败: key=%s, error=%v", key, err)
		}
		if !exists {
			t.Errorf("数据迁移后丢失: key=%s", key)
		}
		if value != expectedValue {
			t.Errorf("迁移后数据错误: key=%s, expected=%s, got=%s",
				key, expectedValue, value)
		}

		// 验证数据确实不在node2上了
		currentNode := cluster.GetNodeForKey(key)
		if currentNode == "node2:8002" {
			t.Errorf("数据仍然路由到已删除的节点: key=%s", key)
		}

		t.Logf("键 %s 从 node2:8002 迁移到 %s", key, currentNode)
	}

	// 验证所有其他数据仍然正常
	for key, expectedValue := range testData {
		value, exists, err := cluster.Get(key)
		if err != nil || !exists || value != expectedValue {
			t.Errorf("其他数据受到影响: key=%s", key)
		}
	}

	// 获取迁移统计
	stats := cluster.GetBasicMigrationStats()
	t.Logf("移除节点迁移统计: 迁移键数=%d, 耗时=%v",
		stats.MigratedKeys, stats.Duration)
}

// 第三十二个测试：数据迁移的一致性验证
func TestBasicMigration_DataConsistency(t *testing.T) {
	t.Log("=== 数据迁移一致性验证 ===")

	cluster := NewDistributedCache([]string{"node1:8001", "node2:8002"})

	// 写入大量数据
	dataCount := 100
	for i := 0; i < dataCount; i++ {
		key := fmt.Sprintf("consistency_key_%d", i)
		value := fmt.Sprintf("consistency_value_%d", i)
		cluster.Set(key, value)
	}

	// 添加节点触发迁移
	err := cluster.AddNodeWithBasicMigration("node3:8003")
	if err != nil {
		t.Fatalf("添加节点失败: %v", err)
	}

	// 验证数据完整性 - 所有数据都应该能正确读取
	for i := 0; i < dataCount; i++ {
		key := fmt.Sprintf("consistency_key_%d", i)
		expectedValue := fmt.Sprintf("consistency_value_%d", i)

		value, exists, err := cluster.Get(key)
		if err != nil {
			t.Errorf("读取数据失败: key=%s, error=%v", key, err)
		}
		if !exists {
			t.Errorf("数据丢失: key=%s", key)
		}
		if value != expectedValue {
			t.Errorf("数据不一致: key=%s, expected=%s, got=%s",
				key, expectedValue, value)
		}
	}

	// 验证数据分布 - 新节点应该承担一部分数据
	nodeDistribution := make(map[string]int)
	for i := 0; i < dataCount; i++ {
		key := fmt.Sprintf("consistency_key_%d", i)
		node := cluster.GetNodeForKey(key)
		nodeDistribution[node]++
	}

	t.Log("迁移后数据分布:")
	for node, count := range nodeDistribution {
		percentage := float64(count) / float64(dataCount) * 100
		t.Logf("  %s: %d 个键 (%.1f%%)", node, count, percentage)
	}

	// 验证负载相对均衡（每个节点应该有数据）
	for node, count := range nodeDistribution {
		if count == 0 {
			t.Errorf("节点 %s 没有分配到任何数据", node)
		}
	}
}

// 辅助函数：获取map的所有键
func getKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// 第三十三个测试：验证一致性哈希环的增量操作
func TestConsistentHashing_IncrementalOperations(t *testing.T) {
	t.Log("=== 验证一致性哈希环的增量操作 ===")

	// 创建初始集群
	cluster := NewDistributedCache([]string{"node1:8001", "node2:8002"})

	// 记录初始哈希环状态
	initialHashCount := len(cluster.sortedHashes)
	initialRingSize := len(cluster.hashRing)

	t.Logf("初始状态: 哈希环大小=%d, 排序列表长度=%d", initialRingSize, initialHashCount)

	// 添加一个节点
	cluster.AddNode("node3:8003")

	// 验证哈希环只增加了新节点的虚拟节点
	afterAddHashCount := len(cluster.sortedHashes)
	afterAddRingSize := len(cluster.hashRing)

	expectedIncrease := cluster.virtualNodes // 每个节点150个虚拟节点
	actualIncrease := afterAddRingSize - initialRingSize

	t.Logf("添加节点后: 哈希环大小=%d, 排序列表长度=%d", afterAddRingSize, afterAddHashCount)
	t.Logf("预期增加=%d, 实际增加=%d", expectedIncrease, actualIncrease)

	if actualIncrease != expectedIncrease {
		t.Errorf("哈希环增量不正确: 预期增加%d, 实际增加%d", expectedIncrease, actualIncrease)
	}

	// 验证原有节点的虚拟节点仍然存在
	node1Count := 0
	node2Count := 0
	node3Count := 0

	for _, node := range cluster.hashRing {
		switch node {
		case "node1:8001":
			node1Count++
		case "node2:8002":
			node2Count++
		case "node3:8003":
			node3Count++
		}
	}

	t.Logf("节点分布: node1=%d, node2=%d, node3=%d", node1Count, node2Count, node3Count)

	if node1Count != cluster.virtualNodes || node2Count != cluster.virtualNodes || node3Count != cluster.virtualNodes {
		t.Errorf("虚拟节点分布不正确: node1=%d, node2=%d, node3=%d, 预期每个=%d",
			node1Count, node2Count, node3Count, cluster.virtualNodes)
	}

	// 移除一个节点
	beforeRemoveRingSize := len(cluster.hashRing)
	cluster.RemoveNode("node2:8002")
	afterRemoveRingSize := len(cluster.hashRing)

	expectedDecrease := cluster.virtualNodes
	actualDecrease := beforeRemoveRingSize - afterRemoveRingSize

	t.Logf("移除节点后: 哈希环大小=%d", afterRemoveRingSize)
	t.Logf("预期减少=%d, 实际减少=%d", expectedDecrease, actualDecrease)

	if actualDecrease != expectedDecrease {
		t.Errorf("哈希环减量不正确: 预期减少%d, 实际减少%d", expectedDecrease, actualDecrease)
	}

	// 验证被移除节点的虚拟节点确实被删除
	for _, node := range cluster.hashRing {
		if node == "node2:8002" {
			t.Error("被移除的节点仍然存在于哈希环中")
		}
	}

	t.Log("✅ 一致性哈希环的增量操作验证通过")
}

// 第三十四个测试：对比重建环 vs 增量操作的性能
func TestConsistentHashing_PerformanceComparison(t *testing.T) {
	t.Log("=== 对比重建环 vs 增量操作的性能 ===")

	// 创建大规模集群进行性能测试
	nodes := make([]string, 10)
	for i := 0; i < 10; i++ {
		nodes[i] = fmt.Sprintf("node%d:800%d", i+1, i+1)
	}

	cluster := NewDistributedCache(nodes)

	// 测试增量添加节点的性能
	start := time.Now()
	cluster.AddNode("new_node:8011")
	incrementalDuration := time.Since(start)

	t.Logf("增量添加节点耗时: %v", incrementalDuration)

	// 模拟重建整个环的耗时（理论上会更慢）
	start = time.Now()
	cluster.buildHashRing() // 重建整个环
	rebuildDuration := time.Since(start)

	t.Logf("重建整个环耗时: %v", rebuildDuration)

	// 在大规模场景下，增量操作应该更快
	if len(nodes) > 5 && incrementalDuration > rebuildDuration*2 {
		t.Logf("注意: 在当前规模下增量操作耗时较长，可能需要优化")
	}

	t.Log("✅ 性能对比测试完成")
}

// 辅助函数：计算浮点数绝对值
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
