#!/bin/bash

# 数据迁移测试脚本

set -e

echo "🧪 分布式缓存数据迁移测试"
echo "=========================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 辅助函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查Go环境
check_go_environment() {
    log_info "检查Go环境..."
    
    if ! command -v go &> /dev/null; then
        log_error "Go 未安装，请先安装 Go"
        exit 1
    fi
    
    log_success "Go环境检查通过"
}

# 进入项目根目录
cd "$(dirname "$0")/.."

# 检查必要文件
check_required_files() {
    log_info "检查必要文件..."
    
    required_files=(
        "core/lrucache.go"
        "core/distributed_cache.go"
        "distributed/node_server.go"
        "distributed/cluster_manager.go"
        "distributed/api_handlers.go"
        "distributed/distributed_node.go"
        "config/node1.yaml"
        "config/node2.yaml"
        "cmd/node/main.go"
        "cmd/migration_test/main.go"
    )
    
    for file in "${required_files[@]}"; do
        if [ ! -f "$file" ]; then
            log_error "缺少必要文件: $file"
            exit 1
        fi
    done
    
    log_success "必要文件检查通过"
}

# 安装依赖
install_dependencies() {
    log_info "安装依赖..."
    
    go mod tidy
    go get github.com/gin-gonic/gin
    go get gopkg.in/yaml.v3
    
    log_success "依赖安装完成"
}

# 创建必要目录
create_directories() {
    log_info "创建必要目录..."
    
    mkdir -p bin logs config
    
    log_success "目录创建完成"
}

# 构建程序
build_programs() {
    log_info "构建程序..."
    
    # 构建节点程序
    log_info "构建节点程序..."
    go build -o bin/cache-node cmd/node/main.go
    
    # 构建数据迁移测试程序
    log_info "构建数据迁移测试程序..."
    go build -o bin/migration-test cmd/migration_test/main.go
    
    log_success "程序构建完成"
}

# 停止现有进程
stop_existing_processes() {
    log_info "停止现有进程..."
    
    # 停止可能运行的节点进程
    ports=(8001 8002 8003 8004)
    for port in "${ports[@]}"; do
        if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
            log_warning "端口 $port 已被占用，正在释放..."
            lsof -ti:$port | xargs kill -9 2>/dev/null || true
        fi
    done
    
    # 清理PID文件
    rm -f logs/*.pid
    
    # 等待进程完全停止
    sleep 2
    
    log_success "现有进程已停止"
}

# 运行数据迁移测试
run_migration_test() {
    log_info "运行数据迁移测试..."
    
    # 运行测试程序
    if ./bin/migration-test; then
        log_success "数据迁移测试通过！"
        return 0
    else
        log_error "数据迁移测试失败！"
        return 1
    fi
}

# 显示测试日志
show_test_logs() {
    log_info "显示测试日志..."
    
    echo ""
    echo "📝 节点日志文件:"
    for log_file in logs/node*.log; do
        if [ -f "$log_file" ]; then
            echo "  - $log_file"
        fi
    done
    
    echo ""
    echo "💡 查看日志命令:"
    echo "  tail -f logs/node1.log"
    echo "  tail -f logs/node2.log"
    echo "  tail -f logs/node4.log"
}

# 清理测试环境
cleanup_test_environment() {
    log_info "清理测试环境..."
    
    # 停止所有进程
    stop_existing_processes
    
    # 清理临时文件
    rm -f config/node4.yaml
    rm -f logs/node4.log
    rm -f logs/node4.pid
    
    log_success "测试环境清理完成"
}

# 主函数
main() {
    echo "开始数据迁移测试..."
    echo ""
    
    # 检查环境
    check_go_environment
    check_required_files
    
    # 准备环境
    install_dependencies
    create_directories
    build_programs
    
    # 停止现有进程
    stop_existing_processes
    
    echo ""
    log_info "开始执行数据迁移测试..."
    echo ""
    
    # 运行测试
    if run_migration_test; then
        echo ""
        log_success "🎉 数据迁移测试完全成功！"
        echo ""
        echo "✅ 测试验证了以下功能:"
        echo "  - 2节点集群启动"
        echo "  - 测试数据设置和分布"
        echo "  - 新节点启动和配置"
        echo "  - 节点添加到哈希环"
        echo "  - 数据自动迁移"
        echo "  - 数据完整性验证"
        echo "  - 迁移统计信息收集"
        echo ""
        show_test_logs
        exit_code=0
    else
        echo ""
        log_error "❌ 数据迁移测试失败！"
        echo ""
        echo "🔍 故障排查建议:"
        echo "  1. 检查端口是否被占用"
        echo "  2. 查看节点日志文件"
        echo "  3. 验证配置文件格式"
        echo "  4. 确认网络连接正常"
        echo ""
        show_test_logs
        exit_code=1
    fi
    
    # 清理环境
    cleanup_test_environment
    
    echo ""
    log_info "数据迁移测试完成"
    
    exit $exit_code
}

# 信号处理
trap cleanup_test_environment EXIT

# 运行主函数
main "$@"
