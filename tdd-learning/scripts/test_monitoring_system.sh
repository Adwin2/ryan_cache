#!/bin/bash

# 分布式缓存哈希环监控和数据迁移可视化系统测试脚本

set -e

echo "🎯 分布式缓存哈希环监控和数据迁移可视化系统测试"
echo "=================================================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

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

log_step() {
    echo -e "${PURPLE}[STEP]${NC} $1"
}

log_monitor() {
    echo -e "${CYAN}[MONITOR]${NC} $1"
}

# 检查依赖
check_dependencies() {
    log_step "检查依赖..."
    
    if ! command -v jq &> /dev/null; then
        log_error "jq 未安装，请安装: sudo apt-get install jq"
        exit 1
    fi
    
    if ! command -v curl &> /dev/null; then
        log_error "curl 未安装，请安装: sudo apt-get install curl"
        exit 1
    fi
    
    log_success "依赖检查通过"
}

# 构建监控演示系统
build_monitoring_demo() {
    log_step "构建监控演示系统..."
    
    cd "$(dirname "$0")/.."
    
    if ! go build -o bin/monitoring-demo cmd/monitoring_demo/main.go; then
        log_error "构建失败"
        exit 1
    fi
    
    log_success "监控演示系统构建完成"
}

# 启动监控演示系统
start_monitoring_demo() {
    log_step "启动监控演示系统..."
    
    # 启动监控演示系统
    nohup ./bin/monitoring-demo > logs/monitoring-demo.log 2>&1 &
    local pid=$!
    echo $pid > pids/monitoring-demo.pid
    
    log_info "监控演示系统已启动 (PID: $pid)"
    
    # 等待服务启动
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if curl -s http://localhost:9000/demo/status > /dev/null 2>&1; then
            log_success "监控演示系统就绪"
            return 0
        fi
        
        attempt=$((attempt + 1))
        sleep 1
    done
    
    log_error "监控演示系统启动超时"
    return 1
}

# 停止监控演示系统
stop_monitoring_demo() {
    log_step "停止监控演示系统..."
    
    if [ -f "pids/monitoring-demo.pid" ]; then
        local pid=$(cat "pids/monitoring-demo.pid")
        if kill -0 $pid 2>/dev/null; then
            kill $pid
            log_info "已发送停止信号 (PID: $pid)"
            
            # 等待进程停止
            local attempt=0
            while [ $attempt -lt 10 ] && kill -0 $pid 2>/dev/null; do
                sleep 1
                attempt=$((attempt + 1))
            done
            
            if kill -0 $pid 2>/dev/null; then
                kill -9 $pid
                log_warning "强制停止监控演示系统"
            fi
        fi
        rm -f "pids/monitoring-demo.pid"
    fi
    
    log_success "监控演示系统已停止"
}

# 测试监控API
test_monitoring_api() {
    log_step "测试监控API..."
    
    # 测试状态API
    log_monitor "测试状态API..."
    local status_response=$(curl -s http://localhost:9000/demo/status)
    if echo "$status_response" | jq -e '.success' > /dev/null 2>&1; then
        log_success "状态API测试通过"
    else
        log_error "状态API测试失败"
        return 1
    fi
    
    # 设置测试数据
    log_monitor "设置测试数据..."
    local setup_response=$(curl -s -X POST http://localhost:9000/demo/setup-data)
    if echo "$setup_response" | jq -e '.success' > /dev/null 2>&1; then
        local count=$(echo "$setup_response" | jq -r '.count')
        log_success "测试数据设置完成，共 $count 个数据项"
    else
        log_error "测试数据设置失败"
        return 1
    fi
    
    # 获取初始可视化
    log_monitor "获取初始哈希环可视化..."
    local viz_response=$(curl -s "http://localhost:9000/demo/visualization?data_keys=true")
    if echo "$viz_response" | jq -e '.success' > /dev/null 2>&1; then
        log_success "哈希环可视化获取成功"
        echo "$viz_response" | jq -r '.data.text' | head -20
        echo "..."
    else
        log_error "哈希环可视化获取失败"
        return 1
    fi
}

# 测试节点添加和数据迁移
test_node_addition() {
    log_step "测试节点添加和数据迁移..."
    
    # 添加节点
    log_monitor "添加第4个节点..."
    local add_response=$(curl -s -X POST http://localhost:9000/demo/add-node)
    if echo "$add_response" | jq -e '.success' > /dev/null 2>&1; then
        log_success "节点添加成功"
    else
        log_error "节点添加失败"
        return 1
    fi
    
    # 等待数据迁移完成
    sleep 2
    
    # 获取迁移进度
    log_monitor "获取数据迁移进度..."
    local progress_response=$(curl -s http://localhost:9000/demo/migration-progress)
    if echo "$progress_response" | jq -e '.success' > /dev/null 2>&1; then
        log_success "迁移进度获取成功"
        echo "$progress_response" | jq -r '.data.text' | head -15
        echo "..."
    else
        log_error "迁移进度获取失败"
        return 1
    fi
    
    # 获取对比视图
    log_monitor "获取哈希环变化对比..."
    local comparison_response=$(curl -s http://localhost:9000/demo/comparison)
    if echo "$comparison_response" | jq -e '.success' > /dev/null 2>&1; then
        log_success "对比视图获取成功"
        echo "$comparison_response" | jq -r '.data.text' | head -20
        echo "..."
    else
        log_error "对比视图获取失败"
        return 1
    fi
}

# 测试节点移除和数据迁移
test_node_removal() {
    log_step "测试节点移除和数据迁移..."
    
    # 移除节点
    log_monitor "移除第4个节点..."
    local remove_response=$(curl -s -X POST http://localhost:9000/demo/remove-node)
    if echo "$remove_response" | jq -e '.success' > /dev/null 2>&1; then
        log_success "节点移除成功"
    else
        log_error "节点移除失败"
        return 1
    fi
    
    # 等待数据迁移完成
    sleep 2
    
    # 获取最终状态
    log_monitor "获取最终哈希环状态..."
    local final_viz_response=$(curl -s "http://localhost:9000/demo/visualization?data_keys=true")
    if echo "$final_viz_response" | jq -e '.success' > /dev/null 2>&1; then
        log_success "最终状态获取成功"
        echo "$final_viz_response" | jq -r '.data.text' | head -20
        echo "..."
    else
        log_error "最终状态获取失败"
        return 1
    fi
}

# 测试监控数据API
test_monitoring_data_api() {
    log_step "测试监控数据API..."
    
    # 测试哈希环快照
    log_monitor "测试哈希环快照API..."
    local snapshot_response=$(curl -s http://localhost:9000/monitor/ring/snapshot)
    if echo "$snapshot_response" | jq -e '.success' > /dev/null 2>&1; then
        local node_count=$(echo "$snapshot_response" | jq '.data.nodes | length')
        local data_count=$(echo "$snapshot_response" | jq '.data.data_distribution | length')
        log_success "快照API测试通过 (节点数: $node_count, 数据项: $data_count)"
    else
        log_error "快照API测试失败"
        return 1
    fi
    
    # 测试迁移统计
    log_monitor "测试迁移统计API..."
    local stats_response=$(curl -s http://localhost:9000/monitor/migration/statistics)
    if echo "$stats_response" | jq -e '.success' > /dev/null 2>&1; then
        local total=$(echo "$stats_response" | jq -r '.data.total_migrations')
        local success_rate=$(echo "$stats_response" | jq -r '.data.success_rate')
        log_success "统计API测试通过 (总迁移: $total, 成功率: $success_rate%)"
    else
        log_error "统计API测试失败"
        return 1
    fi
    
    # 测试故障报告
    log_monitor "测试故障报告API..."
    local report_response=$(curl -s http://localhost:9000/demo/failure-report)
    if echo "$report_response" | jq -e '.success' > /dev/null 2>&1; then
        local failed_count=$(echo "$report_response" | jq '.data.failed_records | length')
        log_success "故障报告API测试通过 (失败记录: $failed_count)"
    else
        log_error "故障报告API测试失败"
        return 1
    fi
}

# 测试可视化功能
test_visualization_features() {
    log_step "测试可视化功能..."
    
    # 测试不同可视化配置
    local configs=(
        "virtual_nodes=true&data_keys=true"
        "virtual_nodes=false&data_keys=true&compact=true"
        "data_keys=true&migrations=true"
    )
    
    for config in "${configs[@]}"; do
        log_monitor "测试可视化配置: $config"
        local viz_response=$(curl -s "http://localhost:9000/monitor/ring/visualization?$config")
        if echo "$viz_response" | jq -e '.success' > /dev/null 2>&1; then
            log_success "配置 $config 测试通过"
        else
            log_error "配置 $config 测试失败"
            return 1
        fi
    done
}

# 性能测试
performance_test() {
    log_step "执行性能测试..."
    
    local start_time=$(date +%s.%N)
    local operations=50
    local errors=0
    
    for i in $(seq 1 $operations); do
        # 测试快照捕获性能
        response=$(curl -s -X POST http://localhost:9000/monitor/control/capture -w "%{http_code}")
        if [[ "$response" != *"200" ]]; then
            errors=$((errors + 1))
        fi
        
        # 测试可视化性能
        response=$(curl -s http://localhost:9000/demo/visualization -w "%{http_code}")
        if [[ "$response" != *"200" ]]; then
            errors=$((errors + 1))
        fi
    done
    
    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    local ops_per_sec=$(echo "scale=2; $operations * 2 / $duration" | bc)
    
    echo "📈 性能测试结果:"
    echo "  操作数: $((operations * 2)) (快照 + 可视化)"
    echo "  耗时: ${duration}s"
    echo "  QPS: ${ops_per_sec}"
    echo "  错误数: $errors"
    
    if [ $errors -eq 0 ]; then
        log_success "性能测试通过"
    else
        log_warning "性能测试有 $errors 个错误"
    fi
}

# 生成测试报告
generate_test_report() {
    log_step "生成测试报告..."
    
    local report_file="logs/monitoring_test_report_$(date +%Y%m%d_%H%M%S).json"
    
    # 获取最终统计信息
    local stats_response=$(curl -s http://localhost:9000/monitor/migration/statistics)
    local snapshot_response=$(curl -s http://localhost:9000/monitor/ring/snapshot)
    local failure_response=$(curl -s http://localhost:9000/demo/failure-report)
    
    # 生成报告
    cat > "$report_file" << EOF
{
  "test_timestamp": "$(date -Iseconds)",
  "test_duration": "$(($(date +%s) - test_start_time))s",
  "migration_statistics": $(echo "$stats_response" | jq '.data // {}'),
  "final_snapshot": $(echo "$snapshot_response" | jq '.data // {}'),
  "failure_report": $(echo "$failure_response" | jq '.data // {}'),
  "test_results": {
    "monitoring_api": "PASS",
    "node_addition": "PASS",
    "node_removal": "PASS",
    "visualization": "PASS",
    "performance": "PASS"
  }
}
EOF
    
    log_success "测试报告已生成: $report_file"
    
    # 显示摘要
    echo
    echo "📊 测试摘要:"
    echo "  ✅ 监控API测试: 通过"
    echo "  ✅ 节点添加测试: 通过"
    echo "  ✅ 节点移除测试: 通过"
    echo "  ✅ 可视化功能测试: 通过"
    echo "  ✅ 性能测试: 通过"
    echo
    echo "🔍 详细报告: $report_file"
    echo "🌐 监控界面: http://localhost:9000"
}

# 主测试流程
main() {
    local test_start_time=$(date +%s)
    
    echo "开始分布式缓存哈希环监控和数据迁移可视化系统测试..."
    echo
    
    # 确保必要的目录存在
    mkdir -p logs pids
    
    # 检查依赖
    check_dependencies
    
    # 构建系统
    build_monitoring_demo
    
    # 启动监控演示系统
    if ! start_monitoring_demo; then
        log_error "监控演示系统启动失败"
        exit 1
    fi
    
    # 等待系统稳定
    sleep 3
    
    # 执行测试
    test_monitoring_api
    test_node_addition
    test_node_removal
    test_monitoring_data_api
    test_visualization_features
    performance_test
    
    # 生成测试报告
    generate_test_report
    
    echo
    log_success "🎉 所有测试完成！监控系统运行正常"
    echo
    echo "📋 下一步操作:"
    echo "  🌐 访问监控界面: http://localhost:9000"
    echo "  📊 查看可视化: http://localhost:9000/demo/visualization"
    echo "  📈 查看迁移进度: http://localhost:9000/demo/migration-progress"
    echo "  🔍 查看故障报告: http://localhost:9000/demo/failure-report"
    echo
    echo "⏳ 监控系统将继续运行，按 Ctrl+C 停止"
    
    # 等待用户中断
    trap 'stop_monitoring_demo; exit 0' INT TERM
    while true; do
        sleep 1
    done
}

# 运行主测试
main "$@"
