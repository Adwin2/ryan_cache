#!/bin/bash

# 分布式缓存集群启动脚本

set -e  # 遇到错误立即退出

echo "🚀 启动高性能分布式缓存集群"
echo "================================"

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "❌ Go 未安装，请先安装 Go"
    exit 1
fi

# 进入项目根目录
cd "$(dirname "$0")/.."

# 检查必要文件
echo "📋 检查项目文件..."
required_files=(
    "core/lrucache.go"
    "core/distributed_cache.go"
    "distributed/node_server.go"
    "distributed/cluster_manager.go"
    "distributed/api_handlers.go"
    "distributed/client.go"
    "config/node1.yaml"
    "config/node2.yaml"
    "config/node3.yaml"
    "cmd/node/main.go"
)

for file in "${required_files[@]}"; do
    if [ ! -f "$file" ]; then
        echo "❌ 缺少必要文件: $file"
        exit 1
    fi
done

echo "✅ 项目文件检查完成"

# 安装依赖
echo "📦 安装依赖..."
go mod tidy
go get github.com/gin-gonic/gin
go get gopkg.in/yaml.v3

# 创建必要目录
mkdir -p bin logs

# 构建节点程序
echo "🔨 构建节点程序..."
echo "   构建通用节点启动器..."
go build -o bin/cache-node cmd/node/main.go

echo "   构建客户端测试工具..."
go build -o bin/cache-client cmd/client/main.go

# 检查端口占用
echo "🔍 检查端口占用..."
ports=(8001 8002 8003)
for port in "${ports[@]}"; do
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo "⚠️  端口 $port 已被占用，正在尝试释放..."
        lsof -ti:$port | xargs kill -9 2>/dev/null || true
        sleep 1
    fi
done

# 启动节点
echo "🟢 启动集群节点..."

# 启动节点1
echo "   启动节点1 (端口 8001)..."
nohup ./bin/cache-node -config=config/node1.yaml > logs/node1.log 2>&1 &
NODE1_PID=$!
echo "     节点1 PID: $NODE1_PID"

# 等待节点1启动
sleep 3

# 启动节点2
echo "   启动节点2 (端口 8002)..."
nohup ./bin/cache-node -config=config/node2.yaml > logs/node2.log 2>&1 &
NODE2_PID=$!
echo "     节点2 PID: $NODE2_PID"

# 等待节点2启动
sleep 3

# 启动节点3
echo "   启动节点3 (端口 8003)..."
nohup ./bin/cache-node -config=config/node3.yaml > logs/node3.log 2>&1 &
NODE3_PID=$!
echo "     节点3 PID: $NODE3_PID"

# 等待所有节点启动
sleep 5

# 保存PID到文件
echo "$NODE1_PID" > logs/node1.pid
echo "$NODE2_PID" > logs/node2.pid
echo "$NODE3_PID" > logs/node3.pid

# 验证节点启动状态
echo "🔍 验证节点启动状态..."
nodes=("localhost:8001" "localhost:8002" "localhost:8003")
for i in "${!nodes[@]}"; do
    node="${nodes[$i]}"
    node_num=$((i + 1))
    
    if curl -s "http://$node/api/v1/health" > /dev/null 2>&1; then
        echo "   ✅ 节点$node_num ($node): 运行正常"
    else
        echo "   ❌ 节点$node_num ($node): 启动失败"
        echo "      检查日志: tail -f logs/node$node_num.log"
    fi
done

echo ""
echo "✅ 集群启动完成！"
echo ""
echo "📋 集群信息:"
echo "  - 节点1: http://localhost:8001"
echo "  - 节点2: http://localhost:8002"
echo "  - 节点3: http://localhost:8003"
echo ""
echo "📊 API 端点:"
echo "  客户端API:"
echo "    - GET    /api/v1/cache/:key     - 获取缓存"
echo "    - PUT    /api/v1/cache/:key     - 设置缓存"
echo "    - DELETE /api/v1/cache/:key     - 删除缓存"
echo "    - GET    /api/v1/stats          - 获取统计"
echo "    - GET    /api/v1/health         - 健康检查"
echo ""
echo "  管理API:"
echo "    - GET    /admin/cluster         - 获取集群信息"
echo "    - GET    /admin/nodes           - 获取节点列表"
echo "    - GET    /admin/metrics         - 获取详细指标"
echo ""
echo "🧪 测试命令:"
echo "  # 运行客户端测试"
echo "  ./bin/cache-client"
echo ""
echo "  # 手动API测试"
echo "  curl -X PUT http://localhost:8001/api/v1/cache/test \\"
echo "       -H 'Content-Type: application/json' \\"
echo "       -d '{\"value\":\"hello world\"}'"
echo ""
echo "  curl http://localhost:8001/api/v1/cache/test"
echo ""
echo "  curl http://localhost:8001/api/v1/stats"
echo ""
echo "  curl http://localhost:8001/admin/cluster"
echo ""
echo "🛑 停止集群:"
echo "  ./scripts/stop_cluster.sh"
echo ""
echo "📝 查看日志:"
echo "  tail -f logs/node1.log"
echo "  tail -f logs/node2.log"
echo "  tail -f logs/node3.log"
echo ""
echo "🎯 项目特色:"
echo "  ✅ 基于现有的高性能LRU缓存"
echo "  ✅ 一致性哈希算法实现"
echo "  ✅ 自动数据迁移功能"
echo "  ✅ 并发安全保证"
echo "  ✅ REST API接口"
echo "  ✅ 集群管理和监控"
echo "  ✅ 故障检测和恢复"
