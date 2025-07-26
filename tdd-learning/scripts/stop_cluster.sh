#!/bin/bash

# 分布式缓存集群停止脚本

echo "🛑 停止高性能分布式缓存集群"
echo "==============================="

# 进入项目根目录
cd "$(dirname "$0")/.."

# 停止所有节点
echo "🔴 停止集群节点..."

# 停止节点1
if [ -f logs/node1.pid ]; then
    NODE1_PID=$(cat logs/node1.pid)
    if kill -0 $NODE1_PID 2>/dev/null; then
        echo "   停止节点1 (PID: $NODE1_PID)..."
        kill $NODE1_PID
        # 等待进程优雅关闭
        sleep 2
        # 如果还在运行，强制杀死
        if kill -0 $NODE1_PID 2>/dev/null; then
            kill -9 $NODE1_PID
        fi
    else
        echo "   节点1 已经停止"
    fi
    rm -f logs/node1.pid
else
    echo "   节点1 PID文件不存在"
fi

# 停止节点2
if [ -f logs/node2.pid ]; then
    NODE2_PID=$(cat logs/node2.pid)
    if kill -0 $NODE2_PID 2>/dev/null; then
        echo "   停止节点2 (PID: $NODE2_PID)..."
        kill $NODE2_PID
        # 等待进程优雅关闭
        sleep 2
        # 如果还在运行，强制杀死
        if kill -0 $NODE2_PID 2>/dev/null; then
            kill -9 $NODE2_PID
        fi
    else
        echo "   节点2 已经停止"
    fi
    rm -f logs/node2.pid
else
    echo "   节点2 PID文件不存在"
fi

# 停止节点3
if [ -f logs/node3.pid ]; then
    NODE3_PID=$(cat logs/node3.pid)
    if kill -0 $NODE3_PID 2>/dev/null; then
        echo "   停止节点3 (PID: $NODE3_PID)..."
        kill $NODE3_PID
        # 等待进程优雅关闭
        sleep 2
        # 如果还在运行，强制杀死
        if kill -0 $NODE3_PID 2>/dev/null; then
            kill -9 $NODE3_PID
        fi
    else
        echo "   节点3 已经停止"
    fi
    rm -f logs/node3.pid
else
    echo "   节点3 PID文件不存在"
fi

# 等待所有进程完全停止
echo "⏳ 等待进程完全停止..."
sleep 3

# 强制清理可能残留的进程
echo "🧹 清理残留进程..."
pkill -f "cache-node" 2>/dev/null || true
pkill -f "node1.yaml" 2>/dev/null || true
pkill -f "node2.yaml" 2>/dev/null || true
pkill -f "node3.yaml" 2>/dev/null || true

# 检查端口是否已释放
echo "🔍 检查端口释放状态..."
ports=(8001 8002 8003)
for port in "${ports[@]}"; do
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo "   ⚠️  端口 $port 仍被占用，强制释放..."
        lsof -ti:$port | xargs kill -9 2>/dev/null || true
    else
        echo "   ✅ 端口 $port 已释放"
    fi
done

# 清理临时文件（可选）
echo "🗑️  清理临时文件..."
# rm -f logs/*.log  # 取消注释以清理日志文件

echo ""
echo "✅ 集群已完全停止"
echo ""
echo "📊 停止后状态:"
echo "   - 所有节点进程已终止"
echo "   - 端口 8001, 8002, 8003 已释放"
echo "   - PID 文件已清理"
echo ""
echo "📝 日志文件保留在 logs/ 目录中"
echo "🔄 重新启动集群: ./scripts/start_cluster.sh"
