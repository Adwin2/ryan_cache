# 分布式缓存系统 API 文档

## 📋 API 概览

本系统提供三类API接口：
- **客户端API**: 对外提供缓存服务
- **内部API**: 节点间通信
- **管理API**: 集群管理和监控

## 🌐 客户端API

### 1. 设置缓存

**请求**
```http
PUT /api/v1/cache/{key}
Content-Type: application/json

{
  "value": "缓存值"
}
```

**响应**
```json
{
  "key": "user:1001",
  "value": "张三",
  "found": true,
  "node_id": "node1",
  "message": "success"
}
```

**示例**
```bash
curl -X PUT http://localhost:8001/api/v1/cache/user:1001 \
     -H 'Content-Type: application/json' \
     -d '{"value":"张三"}'
```

### 2. 获取缓存

**请求**
```http
GET /api/v1/cache/{key}
```

**响应**
```json
{
  "key": "user:1001",
  "value": "张三",
  "found": true,
  "node_id": "node2"
}
```

**示例**
```bash
curl http://localhost:8001/api/v1/cache/user:1001
```

### 3. 删除缓存

**请求**
```http
DELETE /api/v1/cache/{key}
```

**响应**
```json
{
  "message": "deleted",
  "key": "user:1001",
  "node_id": "node1"
}
```

**示例**
```bash
curl -X DELETE http://localhost:8001/api/v1/cache/user:1001
```

### 4. 获取统计信息

**请求**
```http
GET /api/v1/stats
```

**响应**
```json
{
  "node_id": "node1",
  "cache_stats": {
    "total_Hits": 1250,
    "total_Misses": 89,
    "total_Size": 456
  },
  "cluster_stats": {
    "total_nodes": 3,
    "healthy_nodes": 3,
    "unhealthy_nodes": 0,
    "nodes": [
      {
        "node_id": "node1",
        "address": "localhost:8001",
        "status": "healthy",
        "last_seen": "2025-07-25T22:30:00Z",
        "response_time": 2
      }
    ],
    "last_update": "2025-07-25T22:30:00Z"
  },
  "timestamp": "2025-07-25T22:30:00Z"
}
```

**示例**
```bash
curl http://localhost:8001/api/v1/stats
```

### 5. 健康检查

**请求**
```http
GET /api/v1/health
```

**响应**
```json
{
  "status": "healthy",
  "node_id": "node1",
  "timestamp": "2025-07-25T22:30:00Z",
  "uptime": 1721943000
}
```

**示例**
```bash
curl http://localhost:8001/api/v1/health
```

## 🔧 内部API

### 1. 内部缓存操作

**获取本地缓存**
```http
GET /internal/cache/{key}
```

**设置本地缓存**
```http
PUT /internal/cache/{key}
Content-Type: application/json

{
  "value": "缓存值"
}
```

**删除本地缓存**
```http
DELETE /internal/cache/{key}
```

### 2. 集群管理

**节点加入通知**
```http
POST /internal/cluster/join
Content-Type: application/json

{
  "node_id": "node4",
  "address": "localhost:8004"
}
```

**节点离开通知**
```http
POST /internal/cluster/leave
Content-Type: application/json

{
  "node_id": "node4"
}
```

**集群健康检查**
```http
GET /internal/cluster/health
```

## 🛠️ 管理API

### 1. 获取集群信息

**请求**
```http
GET /admin/cluster
```

**响应**
```json
{
  "cluster_status": {
    "total_nodes": 3,
    "healthy_nodes": 3,
    "unhealthy_nodes": 0,
    "nodes": [
      {
        "node_id": "node1",
        "address": "localhost:8001",
        "status": "healthy",
        "last_seen": "2025-07-25T22:30:00Z",
        "response_time": 2
      }
    ],
    "last_update": "2025-07-25T22:30:00Z"
  },
  "current_node": "node1",
  "timestamp": "2025-07-25T22:30:00Z"
}
```

**示例**
```bash
curl http://localhost:8001/admin/cluster
```

### 2. 获取节点列表

**请求**
```http
GET /admin/nodes
```

**响应**
```json
{
  "nodes": {
    "node1": {
      "node_id": "node1",
      "address": "localhost:8001",
      "status": "healthy",
      "last_seen": "2025-07-25T22:30:00Z",
      "response_time": 2
    },
    "node2": {
      "node_id": "node2",
      "address": "localhost:8002",
      "status": "healthy",
      "last_seen": "2025-07-25T22:30:00Z",
      "response_time": 3
    }
  },
  "count": 2,
  "timestamp": "2025-07-25T22:30:00Z"
}
```

**示例**
```bash
curl http://localhost:8001/admin/nodes
```

### 3. 获取详细指标

**请求**
```http
GET /admin/metrics
```

**响应**
```json
{
  "node_id": "node1",
  "cache_stats": {
    "total_Hits": 1250,
    "total_Misses": 89,
    "total_Size": 456
  },
  "migration_stats": {
    "MigratedKeys": 125,
    "Duration": "150ms",
    "LastMigration": "2025-07-25T22:25:00Z"
  },
  "cluster_stats": {
    "total_nodes": 3,
    "healthy_nodes": 3,
    "unhealthy_nodes": 0
  },
  "timestamp": "2025-07-25T22:30:00Z"
}
```

**示例**
```bash
curl http://localhost:8001/admin/metrics
```

### 4. 集群重平衡

**请求**
```http
POST /admin/cluster/rebalance
```

**响应**
```json
{
  "message": "rebalance completed",
  "node_id": "node1",
  "timestamp": "2025-07-25T22:30:00Z"
}
```

**示例**
```bash
curl -X POST http://localhost:8001/admin/cluster/rebalance
```

## 📝 错误响应

所有API在出错时返回统一的错误格式：

```json
{
  "error": "error_type",
  "message": "详细错误信息",
  "timestamp": "2025-07-25T22:30:00Z"
}
```

### 常见错误类型

| 错误类型 | HTTP状态码 | 说明 |
|---------|-----------|------|
| `invalid_request` | 400 | 请求格式错误 |
| `cache_error` | 500 | 缓存操作失败 |
| `node_not_found` | 500 | 目标节点不存在 |
| `forward_failed` | 500 | 请求转发失败 |
| `decode_failed` | 500 | 响应解析失败 |
| `add_node_error` | 500 | 添加节点失败 |
| `remove_node_error` | 500 | 移除节点失败 |

## 🔄 请求转发机制

当客户端向任意节点发送请求时，系统会：

1. **计算目标节点**: 使用一致性哈希算法确定数据应该存储在哪个节点
2. **本地处理**: 如果目标节点是当前节点，直接处理
3. **请求转发**: 如果目标节点是其他节点，转发请求到目标节点
4. **响应返回**: 将目标节点的响应返回给客户端

### 转发流程示例

```
客户端 -> 节点1 -> 节点2 (实际存储) -> 节点1 -> 客户端
```

## 📊 性能优化建议

### 1. 批量操作
对于大量数据操作，建议使用客户端SDK的批量接口：

```go
// 批量设置
data := map[string]string{
    "key1": "value1",
    "key2": "value2",
}
client.BatchSet(data)

// 批量获取
keys := []string{"key1", "key2"}
result, _ := client.BatchGet(keys)
```

### 2. 连接复用
客户端应该复用HTTP连接，避免频繁建立连接：

```go
client := &http.Client{
    Timeout: 5 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
    },
}
```

### 3. 负载均衡
客户端可以轮询访问不同节点，实现负载均衡：

```go
nodes := []string{"localhost:8001", "localhost:8002", "localhost:8003"}
node := nodes[requestCount % len(nodes)]
```

## 🔐 安全考虑

### 1. 访问控制
- 内部API应该只允许集群内部访问
- 管理API应该有认证机制
- 客户端API可以添加API Key验证

### 2. 数据加密
- 敏感数据可以在应用层加密
- 节点间通信可以使用TLS
- 配置文件中的敏感信息应该加密存储

### 3. 网络安全
- 使用防火墙限制端口访问
- 配置网络隔离
- 监控异常访问模式

## 📈 监控集成

### 1. Prometheus指标
系统可以暴露Prometheus格式的指标：

```
# HELP cache_hits_total Total number of cache hits
# TYPE cache_hits_total counter
cache_hits_total{node="node1"} 1250

# HELP cache_misses_total Total number of cache misses
# TYPE cache_misses_total counter
cache_misses_total{node="node1"} 89
```

### 2. 日志格式
建议使用结构化日志格式：

```json
{
  "timestamp": "2025-07-25T22:30:00Z",
  "level": "INFO",
  "node_id": "node1",
  "operation": "GET",
  "key": "user:1001",
  "duration": "2ms",
  "status": "hit"
}
```

### 3. 健康检查
定期调用健康检查接口，监控集群状态：

```bash
# 检查所有节点健康状态
for node in node1:8001 node2:8002 node3:8003; do
  curl -f http://$node/api/v1/health || echo "$node is down"
done
```
