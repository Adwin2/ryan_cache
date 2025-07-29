# 分布式缓存系统部署文档

## 🚀 快速开始

### 环境要求
- Go 1.19+
- Linux/macOS/Windows
- 可用端口: 8001, 8002, 8003

### 一键启动
```bash
# 克隆项目
git clone <repository-url>
cd tdd-learning

# 启动集群
./scripts/start_cluster.sh

# 测试集群
./scripts/test_cluster.sh

# 停止集群
./scripts/stop_cluster.sh
```

## 📋 详细部署步骤

### 1. 环境准备

**安装Go环境**
```bash
# Ubuntu/Debian
sudo apt update
sudo apt install golang-go

# CentOS/RHEL
sudo yum install golang

# macOS
brew install go

# 验证安装
go version
```

**检查端口可用性**
```bash
# 检查端口占用
netstat -tlnp | grep -E ':(8001|8002|8003)'

# 如果端口被占用，释放端口
sudo lsof -ti:8001 | xargs kill -9
sudo lsof -ti:8002 | xargs kill -9
sudo lsof -ti:8003 | xargs kill -9
```

### 2. 项目构建

**下载依赖**
```bash
cd tdd-learning
go mod tidy
go get github.com/gin-gonic/gin
go get gopkg.in/yaml.v3
```

**构建程序**
```bash
# 构建节点程序
go build -o bin/cache-node cmd/node/main.go

# 构建客户端
go build -o bin/cache-client cmd/client/main.go

# 验证构建
ls -la bin/
```

### 3. 配置文件

**节点配置示例 (config/node1.yaml)**
```yaml
node_id: "node1"
address: ":8001"
cluster_nodes:
  node1: "localhost:8001"
  node2: "localhost:8002"
  node3: "localhost:8003"
cache_size: 1000
virtual_nodes: 150
```

**配置参数说明**
- `node_id`: 节点唯一标识符
- `address`: 节点监听地址
- `cluster_nodes`: 集群所有节点列表
- `cache_size`: 本地缓存容量
- `virtual_nodes`: 虚拟节点数量

### 4. 启动集群

**手动启动**
```bash
# 启动节点1
./bin/cache-node -config=config/node1.yaml > logs/node1.log 2>&1 &

# 启动节点2
./bin/cache-node -config=config/node2.yaml > logs/node2.log 2>&1 &

# 启动节点3
./bin/cache-node -config=config/node3.yaml > logs/node3.log 2>&1 &
```

**脚本启动**
```bash
# 使用启动脚本
./scripts/start_cluster.sh

# 检查启动状态
ps aux | grep cache-node
```

### 5. 验证部署

**健康检查**
```bash
# 检查所有节点
curl http://localhost:8001/api/v1/health
curl http://localhost:8002/api/v1/health
curl http://localhost:8003/api/v1/health
```

**功能测试**
```bash
# 设置缓存
curl -X PUT http://localhost:8001/api/v1/cache/test \
     -H 'Content-Type: application/json' \
     -d '{"value":"hello world"}'

# 获取缓存
curl http://localhost:8001/api/v1/cache/test

# 获取集群信息
curl http://localhost:8001/admin/cluster
```

**运行测试套件**
```bash
# 运行完整测试
./scripts/test_cluster.sh

# 运行客户端测试
./bin/cache-client
```

## 🐳 Docker部署

### 1. 创建Dockerfile

```dockerfile
FROM golang:1.19-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod tidy
RUN go build -o cache-node cmd/node/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/cache-node .
COPY --from=builder /app/config ./config
EXPOSE 8001 8002 8003
CMD ["./cache-node"]
```

### 2. 构建镜像

```bash
# 构建镜像
docker build -t distributed-cache:latest .

# 验证镜像
docker images | grep distributed-cache
```

### 3. Docker Compose部署

**docker-compose.yml**
```yaml
version: '3.8'

services:
  cache-node1:
    image: distributed-cache:latest
    command: ["./cache-node", "-config=config/node1.yaml"]
    ports:
      - "8001:8001"
    volumes:
      - ./logs:/root/logs
    networks:
      - cache-network

  cache-node2:
    image: distributed-cache:latest
    command: ["./cache-node", "-config=config/node2.yaml"]
    ports:
      - "8002:8002"
    volumes:
      - ./logs:/root/logs
    networks:
      - cache-network

  cache-node3:
    image: distributed-cache:latest
    command: ["./cache-node", "-config=config/node3.yaml"]
    ports:
      - "8003:8003"
    volumes:
      - ./logs:/root/logs
    networks:
      - cache-network

networks:
  cache-network:
    driver: bridge
```

**启动容器**
```bash
# 启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f cache-node1
```

## ☸️ Kubernetes部署

### 1. 创建ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cache-config
data:
  node1.yaml: |
    node_id: "node1"
    address: ":8001"
    cluster_nodes:
      node1: "cache-node1:8001"
      node2: "cache-node2:8002"
      node3: "cache-node3:8003"
    cache_size: 1000
    virtual_nodes: 150
```

### 2. 创建Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: cache-node1
spec:
  replicas: 1
  selector:
    matchLabels:
      app: cache-node1
  template:
    metadata:
      labels:
        app: cache-node1
    spec:
      containers:
      - name: cache-node
        image: distributed-cache:latest
        command: ["./cache-node", "-config=/config/node1.yaml"]
        ports:
        - containerPort: 8001
        volumeMounts:
        - name: config
          mountPath: /config
      volumes:
      - name: config
        configMap:
          name: cache-config
```

### 3. 创建Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: cache-node1
spec:
  selector:
    app: cache-node1
  ports:
  - port: 8001
    targetPort: 8001
  type: ClusterIP
```

### 4. 部署到集群

```bash
# 应用配置
kubectl apply -f k8s/

# 查看部署状态
kubectl get pods
kubectl get services

# 端口转发测试
kubectl port-forward svc/cache-node1 8001:8001
```

## 🔧 生产环境配置

### 1. 系统优化

**内核参数调优**
```bash
# /etc/sysctl.conf
net.core.somaxconn = 65535
net.core.netdev_max_backlog = 5000
net.ipv4.tcp_max_syn_backlog = 65535
net.ipv4.tcp_fin_timeout = 30
net.ipv4.tcp_keepalive_time = 1200
net.ipv4.tcp_max_tw_buckets = 5000

# 应用配置
sysctl -p
```

**文件描述符限制**
```bash
# /etc/security/limits.conf
* soft nofile 65535
* hard nofile 65535

# 验证
ulimit -n
```

### 2. 监控配置

**Prometheus配置**
```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'distributed-cache'
    static_configs:
      - targets: ['localhost:8001', 'localhost:8002', 'localhost:8003']
    metrics_path: '/admin/metrics'
    scrape_interval: 15s
```

**Grafana仪表板**
- 缓存命中率
- QPS统计
- 响应时间
- 集群健康状态

### 3. 日志配置

**日志轮转 (logrotate)**
```bash
# /etc/logrotate.d/distributed-cache
/var/log/distributed-cache/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 644 cache cache
    postrotate
        systemctl reload distributed-cache
    endscript
}
```

### 4. 服务管理

**Systemd服务文件**
```ini
# /etc/systemd/system/cache-node1.service
[Unit]
Description=Distributed Cache Node 1
After=network.target

[Service]
Type=simple
User=cache
Group=cache
WorkingDirectory=/opt/distributed-cache
ExecStart=/opt/distributed-cache/bin/cache-node -config=/opt/distributed-cache/config/node1.yaml
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

**服务管理命令**
```bash
# 启用服务
sudo systemctl enable cache-node1
sudo systemctl enable cache-node2
sudo systemctl enable cache-node3

# 启动服务
sudo systemctl start cache-node1
sudo systemctl start cache-node2
sudo systemctl start cache-node3

# 查看状态
sudo systemctl status cache-node1
```

## 🔍 故障排查

### 1. 常见问题

**端口占用**
```bash
# 检查端口占用
netstat -tlnp | grep :8001

# 释放端口
sudo lsof -ti:8001 | xargs kill -9
```

**配置文件错误**
```bash
# 验证YAML格式
python -c "import yaml; yaml.safe_load(open('config/node1.yaml'))"

# 检查配置内容
cat config/node1.yaml
```

**网络连接问题**
```bash
# 测试节点间连通性
curl -v http://localhost:8002/internal/cluster/health

# 检查防火墙
sudo iptables -L
sudo ufw status
```

### 2. 日志分析

**查看启动日志**
```bash
# 实时查看日志
tail -f logs/node1.log

# 搜索错误信息
grep -i error logs/node1.log
grep -i "failed\|panic" logs/node1.log
```

**常见错误模式**
- `bind: address already in use` - 端口占用
- `connection refused` - 目标节点不可达
- `yaml: unmarshal errors` - 配置文件格式错误
- `no such file or directory` - 文件路径错误

### 3. 性能调优

**内存使用优化**
```bash
# 监控内存使用
top -p $(pgrep cache-node)
ps aux | grep cache-node

# 调整缓存大小
# 修改配置文件中的 cache_size 参数
```

**网络性能优化**
```bash
# 调整TCP参数
echo 'net.ipv4.tcp_congestion_control = bbr' >> /etc/sysctl.conf
sysctl -p

# 监控网络连接
ss -tuln | grep :800
```

## 📊 监控和告警

### 1. 关键指标

**系统指标**
- CPU使用率
- 内存使用率
- 磁盘I/O
- 网络带宽

**应用指标**
- 缓存命中率
- QPS
- 响应时间
- 错误率

**集群指标**
- 节点健康状态
- 数据分布均匀度
- 迁移频率

### 2. 告警规则

**Prometheus告警规则**
```yaml
groups:
- name: distributed-cache
  rules:
  - alert: CacheNodeDown
    expr: up{job="distributed-cache"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "Cache node is down"
      
  - alert: HighErrorRate
    expr: rate(cache_errors_total[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High error rate detected"
```

### 3. 健康检查脚本

```bash
#!/bin/bash
# health_check.sh

NODES=("localhost:8001" "localhost:8002" "localhost:8003")
FAILED_NODES=()

for node in "${NODES[@]}"; do
    if ! curl -f -s "http://$node/api/v1/health" > /dev/null; then
        FAILED_NODES+=("$node")
    fi
done

if [ ${#FAILED_NODES[@]} -gt 0 ]; then
    echo "CRITICAL: Failed nodes: ${FAILED_NODES[*]}"
    exit 2
elif [ ${#FAILED_NODES[@]} -eq 1 ]; then
    echo "WARNING: One node failed: ${FAILED_NODES[0]}"
    exit 1
else
    echo "OK: All nodes healthy"
    exit 0
fi
```

## 🔄 升级和维护

### 1. 滚动升级

```bash
# 逐个升级节点
./scripts/stop_node.sh node1
# 部署新版本
./scripts/start_node.sh node1

# 等待节点稳定后继续下一个
./scripts/stop_node.sh node2
./scripts/start_node.sh node2
```

### 2. 数据备份

```bash
# 导出集群数据
curl http://localhost:8001/admin/export > backup.json

# 恢复数据
curl -X POST http://localhost:8001/admin/import \
     -H 'Content-Type: application/json' \
     -d @backup.json
```

### 3. 配置变更

```bash
# 修改配置文件
vim config/node1.yaml

# 重启服务应用配置
sudo systemctl restart cache-node1
```
