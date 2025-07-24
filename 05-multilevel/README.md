# 第五章：多级缓存

> 🎯 **学习目标**: 掌握现代高性能系统的多级缓存架构设计
> 
> ⚠️ **面试重点**: 这是大厂面试中的高频架构设计题！

## 📖 理论知识

### 为什么需要多级缓存？

单一缓存层面临的挑战：

1. **性能瓶颈**: 网络延迟影响响应时间
2. **容量限制**: 单层缓存容量有限
3. **可用性风险**: 单点故障影响整个系统
4. **成本问题**: 高性能缓存成本昂贵

### 多级缓存架构

```
应用程序
    ↓
L1: 本地缓存 (JVM内存/进程内)
    ↓ (未命中)
L2: 分布式缓存 (Redis/Memcached)
    ↓ (未命中)  
L3: 数据库 (MySQL/PostgreSQL)
```

## 🏗️ 架构设计

### 1. 缓存层级特点

| 层级 | 类型 | 容量 | 延迟 | 一致性 | 成本 |
|------|------|------|------|--------|------|
| **L1** | 本地缓存 | 小 | 极低(ns) | 弱 | 低 |
| **L2** | 分布式缓存 | 中 | 低(ms) | 强 | 中 |
| **L3** | 数据库 | 大 | 高(ms) | 强 | 高 |

### 2. 缓存策略

#### 读取策略 (Read-Through)
```go
func Get(key string) (value string, err error) {
    // L1: 本地缓存
    if value, exists := l1Cache.Get(key); exists {
        return value, nil
    }
    
    // L2: 分布式缓存
    if value, exists := l2Cache.Get(key); exists {
        l1Cache.Set(key, value) // 回写L1
        return value, nil
    }
    
    // L3: 数据库
    value, err = database.Query(key)
    if err == nil {
        l2Cache.Set(key, value) // 写入L2
        l1Cache.Set(key, value) // 写入L1
    }
    return value, err
}
```

#### 写入策略 (Write-Through)
```go
func Set(key, value string) error {
    // 写入数据库
    if err := database.Update(key, value); err != nil {
        return err
    }
    
    // 更新L2缓存
    l2Cache.Set(key, value)
    
    // 更新L1缓存
    l1Cache.Set(key, value)
    
    return nil
}
```

### 3. 一致性保证

#### 问题：多级缓存数据不一致

```
时刻T1: L1=v1, L2=v1, DB=v1  ✅ 一致
时刻T2: 更新DB=v2
时刻T3: L1=v1, L2=v1, DB=v2  ❌ 不一致
```

#### 解决方案

1. **TTL策略**: 不同层级设置不同过期时间
2. **主动失效**: 写入时主动删除缓存
3. **消息通知**: 使用MQ通知缓存更新
4. **版本控制**: 使用版本号检测数据新旧

## 💻 代码实现

查看具体实现：
- `local_cache.go` - 本地缓存实现
- `distributed_cache.go` - 分布式缓存模拟
- `multilevel_cache.go` - 多级缓存核心实现
- `consistency.go` - 一致性保证机制
- `demo.go` - 完整演示程序

## 🎮 动手实践

```bash
cd 05-multilevel
go run *.go
```

## 📝 面试要点

### 必考问题

**Q: 什么是多级缓存？为什么要使用多级缓存？**

A: 多级缓存是将缓存分为多个层级的架构模式：
- **L1本地缓存**: 极低延迟，容量小
- **L2分布式缓存**: 低延迟，容量中等
- **L3数据库**: 高延迟，容量大
- **优势**: 提高性能、降低延迟、增强可用性

**Q: 多级缓存的一致性问题如何解决？**

A: 
1. **TTL策略**: L1设置较短TTL，L2设置较长TTL
2. **写入时失效**: 更新数据时删除各级缓存
3. **消息通知**: 使用Redis Pub/Sub或MQ通知
4. **最终一致性**: 容忍短期不一致，保证最终一致

**Q: 如何设计一个高性能的多级缓存系统？**

A:
1. **缓存选型**: 本地用Caffeine/Guava，分布式用Redis
2. **容量规划**: 根据热点数据和内存限制设计容量
3. **TTL设计**: L1(1-5分钟)，L2(10-30分钟)
4. **监控告警**: 命中率、延迟、错误率监控
5. **降级策略**: 缓存故障时的备用方案

### 进阶问题

**Q: 多级缓存的性能如何优化？**

A:
1. **异步更新**: 使用异步方式更新缓存
2. **批量操作**: 批量获取和更新数据
3. **预热策略**: 系统启动时预加载热点数据
4. **智能路由**: 根据数据特征选择缓存层级

**Q: 如何处理缓存雪崩、穿透、击穿？**

A: 在多级缓存中的处理策略：
1. **雪崩**: 不同层级设置随机TTL
2. **穿透**: L1层使用布隆过滤器
3. **击穿**: L2层使用分布式锁

## 🏭 生产环境实践

### 真实案例

**案例1: 电商商品详情页**
```
L1: 本地缓存商品基础信息 (1分钟TTL)
L2: Redis缓存完整商品信息 (10分钟TTL)  
L3: MySQL存储商品数据
```

**案例2: 用户会话管理**
```
L1: 本地缓存活跃用户会话 (5分钟TTL)
L2: Redis缓存所有用户会话 (30分钟TTL)
L3: 数据库存储用户信息
```

### 架构演进

#### 初级架构
```
应用 → Redis → MySQL
```

#### 中级架构  
```
应用 → 本地缓存 → Redis → MySQL
```

#### 高级架构
```
应用 → L1(本地) → L2(Redis) → L3(MySQL)
     ↓
   消息队列 (一致性保证)
```

## 📊 性能对比

### 延迟对比

| 操作 | 本地缓存 | Redis | MySQL |
|------|----------|-------|-------|
| 读取 | 0.1ms | 1-5ms | 10-100ms |
| 写入 | 0.1ms | 1-5ms | 10-100ms |

### 吞吐量对比

| 缓存类型 | QPS | 内存使用 | 网络开销 |
|----------|-----|----------|----------|
| 本地缓存 | 100万+ | 高 | 无 |
| Redis | 10万+ | 低 | 有 |
| MySQL | 1万+ | 低 | 有 |

## 🔧 最佳实践

### 1. 缓存设计原则

```go
// 好的设计
type CacheConfig struct {
    L1TTL    time.Duration // 短TTL
    L2TTL    time.Duration // 长TTL  
    L1Size   int          // 小容量
    L2Size   int          // 大容量
}

// 坏的设计 - 所有层级相同配置
type BadCacheConfig struct {
    TTL  time.Duration // 相同TTL
    Size int          // 相同容量
}
```

### 2. 监控指标

```go
type CacheMetrics struct {
    L1HitRate    float64 // L1命中率
    L2HitRate    float64 // L2命中率
    OverallHitRate float64 // 总命中率
    AvgLatency   time.Duration // 平均延迟
    ErrorRate    float64 // 错误率
}
```

### 3. 容量规划

```go
// 根据业务特征规划容量
func PlanCacheCapacity(hotDataSize, totalDataSize int64) CacheConfig {
    return CacheConfig{
        L1Size: int(hotDataSize * 0.1),    // 10%热点数据
        L2Size: int(hotDataSize * 0.8),    // 80%热点数据
        L1TTL:  5 * time.Minute,
        L2TTL:  30 * time.Minute,
    }
}
```

## 🚀 高级特性

### 1. 智能预热

```go
// 根据访问模式预热缓存
func SmartWarmup(accessLog []AccessRecord) {
    hotKeys := analyzeHotKeys(accessLog)
    for _, key := range hotKeys {
        preloadToCache(key)
    }
}
```

### 2. 动态TTL

```go
// 根据访问频率动态调整TTL
func DynamicTTL(key string, accessCount int) time.Duration {
    baseTTL := 5 * time.Minute
    if accessCount > 1000 {
        return baseTTL * 4 // 热点数据延长TTL
    }
    return baseTTL
}
```

### 3. 缓存降级

```go
// 缓存故障时的降级策略
func GetWithFallback(key string) (string, error) {
    // 尝试L1
    if value, exists := l1.Get(key); exists {
        return value, nil
    }
    
    // 尝试L2
    if value, exists := l2.Get(key); exists {
        return value, nil
    }
    
    // 降级到数据库
    return database.Get(key)
}
```

## 🔗 下一章预告

下一章我们将学习**面试题集**：
- 50+常见缓存面试题
- 标准答案和解题思路
- 真实面试场景模拟
- 架构设计题详解

这将是您面试准备的最后冲刺！

---

**继续学习**: [第六章：面试题集](../06-interview/)
