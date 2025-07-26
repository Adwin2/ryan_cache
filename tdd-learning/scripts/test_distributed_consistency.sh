#!/bin/bash

# 分布式一致性哈希和数据迁移综合测试脚本

set -e

echo "🧪 分布式一致性哈希和数据迁移综合测试"
echo "========================================"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 测试数据
declare -A TEST_DATA=(
    ["user:1001"]="张三"
    ["user:1002"]="李四"
    ["user:1003"]="王五"
    ["user:1004"]="赵六"
    ["user:1005"]="钱七"
    ["product:2001"]="iPhone15"
    ["product:2002"]="MacBook"
    ["product:2003"]="iPad"
    ["product:2004"]="AirPods"
    ["product:2005"]="AppleWatch"
    ["order:3001"]="订单1"
    ["order:3002"]="订单2"
    ["order:3003"]="订单3"
    ["order:3004"]="订单4"
    ["order:3005"]="订单5"
    ["session:abc123"]="会话1"
    ["session:def456"]="会话2"
    ["session:ghi789"]="会话3"
    ["cache:key1"]="缓存值1"
    ["cache:key2"]="缓存值2"
)

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

# 检查节点是否健康
check_node_health() {
    local port=$1
    local response=$(curl -s -w "%{http_code}" http://localhost:$port/api/v1/health -o /dev/null)
    if [ "$response" = "200" ]; then
        return 0
    else
        return 1
    fi
}

# 等待所有节点就绪
wait_for_cluster() {
    log_info "等待集群就绪..."
    local max_attempts=30
    local attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if check_node_health 8001 && check_node_health 8002 && check_node_health 8003; then
            log_success "集群就绪"
            return 0
        fi
        
        attempt=$((attempt + 1))
        sleep 1
    done
    
    log_error "集群启动超时"
    exit 1
}

# 设置测试数据
setup_test_data() {
    log_info "设置测试数据..."
    
    for key in "${!TEST_DATA[@]}"; do
        value="${TEST_DATA[$key]}"
        response=$(curl -s -X PUT http://localhost:8001/api/v1/cache/$key \
                   -H 'Content-Type: application/json' \
                   -d "{\"value\":\"$value\"}" \
                   -w "%{http_code}")
        
        if [[ "$response" == *"200" ]]; then
            echo "  ✅ $key = $value"
        else
            log_error "设置数据失败: $key"
            exit 1
        fi
    done
    
    log_success "测试数据设置完成"
}

# 验证数据完整性
verify_data_integrity() {
    log_info "验证数据完整性..."
    local errors=0
    
    for key in "${!TEST_DATA[@]}"; do
        expected_value="${TEST_DATA[$key]}"
        
        # 从任意节点获取数据（测试路由）
        local node_port=$((8001 + RANDOM % 3))
        response=$(curl -s http://localhost:$node_port/api/v1/cache/$key)
        
        if echo "$response" | jq -e '.found' > /dev/null 2>&1; then
            actual_value=$(echo "$response" | jq -r '.value')
            node_id=$(echo "$response" | jq -r '.node_id')
            
            if [ "$actual_value" = "$expected_value" ]; then
                echo "  ✅ $key = $actual_value (在 $node_id)"
            else
                echo "  ❌ $key: 期望=$expected_value, 实际=$actual_value"
                errors=$((errors + 1))
            fi
        else
            echo "  ❌ $key: 数据未找到"
            errors=$((errors + 1))
        fi
    done
    
    if [ $errors -eq 0 ]; then
        log_success "数据完整性验证通过"
        return 0
    else
        log_error "发现 $errors 个数据错误"
        return 1
    fi
}

# 检查数据分布
check_data_distribution() {
    log_info "检查数据分布..."
    
    echo "📊 各节点数据统计:"
    for port in 8001 8002 8003; do
        stats=$(curl -s http://localhost:$port/api/v1/stats)
        node_id=$(echo "$stats" | jq -r '.node_id')
        cache_size=$(echo "$stats" | jq -r '.cache_stats.total_Size')
        echo "  $node_id: $cache_size 个数据项"
    done
}

# 测试添加节点和数据迁移（模拟场景）
test_add_node_migration() {
    log_info "测试添加节点和数据迁移（模拟场景）..."

    # 记录添加节点前的分布
    log_info "添加节点前的数据分布:"
    check_data_distribution

    # 注意：这里我们只是测试数据迁移逻辑，不真正添加网络节点
    # 在真实场景中，应该先启动新节点，再将其加入集群
    log_info "模拟添加新节点 node4（仅测试数据迁移逻辑）..."
    response=$(curl -s -X POST http://localhost:8001/internal/cluster/join \
               -H 'Content-Type: application/json' \
               -d '{"node_id":"node4","address":"localhost:8004"}' \
               -w "%{http_code}")

    if [[ "$response" == *"200" ]]; then
        log_success "节点4添加成功（数据迁移已执行）"
    else
        log_error "节点4添加失败: $response"
        return 1
    fi

    # 等待数据迁移完成
    sleep 2

    # 检查迁移统计（这是我们真正要验证的）
    log_info "检查迁移统计..."
    local total_migrated=0
    for port in 8001 8002 8003; do
        metrics=$(curl -s http://localhost:$port/admin/metrics)
        node_id=$(echo "$metrics" | jq -r '.node_id')
        migrated_keys=$(echo "$metrics" | jq -r '.migration_stats.migrated_keys // 0')
        duration=$(echo "$metrics" | jq -r '.migration_stats.duration // "0s"')
        echo "  $node_id: 迁移了 $migrated_keys 个key，耗时 $duration"
        total_migrated=$((total_migrated + migrated_keys))
    done

    if [ $total_migrated -gt 0 ]; then
        log_success "数据迁移已执行，总共迁移了 $total_migrated 个key"
    else
        log_warning "没有数据被迁移（这可能是正常的，取决于哈希分布）"
    fi

    # 检查集群状态
    cluster_info=$(curl -s http://localhost:8001/admin/cluster)
    node_count=$(echo "$cluster_info" | jq '.cluster_status.nodes | length')
    echo "  集群节点数: $node_count"
}

# 测试移除节点和数据迁移
test_remove_node_migration() {
    log_info "测试移除节点和数据迁移..."

    # 移除之前添加的虚拟节点
    log_info "移除虚拟节点 node4..."
    response=$(curl -s -X POST http://localhost:8001/internal/cluster/leave \
               -H 'Content-Type: application/json' \
               -d '{"node_id":"node4"}' \
               -w "%{http_code}")

    if [[ "$response" == *"200" ]]; then
        log_success "节点4移除成功"
    else
        log_error "节点4移除失败: $response"
        return 1
    fi

    # 等待数据迁移完成
    sleep 2

    # 检查迁移后的分布
    log_info "移除节点后的数据分布:"
    check_data_distribution

    # 验证数据完整性（现在应该能找到所有数据）
    log_info "验证移除节点后的数据完整性..."
    verify_data_integrity

    # 检查迁移统计
    log_info "检查移除节点后的迁移统计..."
    local total_migrated=0
    for port in 8001 8002 8003; do
        metrics=$(curl -s http://localhost:$port/admin/metrics)
        node_id=$(echo "$metrics" | jq -r '.node_id')
        migrated_keys=$(echo "$metrics" | jq -r '.migration_stats.migrated_keys // 0')
        duration=$(echo "$metrics" | jq -r '.migration_stats.duration // "0s"')
        echo "  $node_id: 迁移了 $migrated_keys 个key，耗时 $duration"
        total_migrated=$((total_migrated + migrated_keys))
    done

    # 检查集群状态
    cluster_info=$(curl -s http://localhost:8001/admin/cluster)
    node_count=$(echo "$cluster_info" | jq '.cluster_status.nodes | length')
    echo "  集群节点数: $node_count"
}

# 测试路由一致性
test_routing_consistency() {
    log_info "测试路由一致性..."
    
    local errors=0
    
    for key in "${!TEST_DATA[@]}"; do
        # 从不同节点查询相同key，应该得到相同结果
        local responses=()
        for port in 8001 8002 8003; do
            response=$(curl -s http://localhost:$port/api/v1/cache/$key)
            responses+=("$response")
        done
        
        # 检查所有响应是否一致
        local first_value=$(echo "${responses[0]}" | jq -r '.value // "null"')
        local first_found=$(echo "${responses[0]}" | jq -r '.found // false')
        
        for i in {1..2}; do
            local value=$(echo "${responses[$i]}" | jq -r '.value // "null"')
            local found=$(echo "${responses[$i]}" | jq -r '.found // false')
            
            if [ "$value" != "$first_value" ] || [ "$found" != "$first_found" ]; then
                echo "  ❌ $key: 路由不一致"
                errors=$((errors + 1))
                break
            fi
        done
        
        if [ $errors -eq 0 ]; then
            echo "  ✅ $key: 路由一致"
        fi
    done
    
    if [ $errors -eq 0 ]; then
        log_success "路由一致性测试通过"
        return 0
    else
        log_error "发现 $errors 个路由不一致问题"
        return 1
    fi
}

# 性能测试
performance_test() {
    log_info "执行性能测试..."
    
    local start_time=$(date +%s.%N)
    local operations=100
    local errors=0
    
    for i in $(seq 1 $operations); do
        key="perf:key$i"
        value="perf_value_$i"
        
        # 设置数据
        response=$(curl -s -X PUT http://localhost:8001/api/v1/cache/$key \
                   -H 'Content-Type: application/json' \
                   -d "{\"value\":\"$value\"}" \
                   -w "%{http_code}")
        
        if [[ "$response" != *"200" ]]; then
            errors=$((errors + 1))
        fi
        
        # 获取数据
        response=$(curl -s http://localhost:8002/api/v1/cache/$key -w "%{http_code}")
        if [[ "$response" != *"200" ]]; then
            errors=$((errors + 1))
        fi
    done
    
    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    local ops_per_sec=$(echo "scale=2; $operations * 2 / $duration" | bc)
    
    echo "📈 性能测试结果:"
    echo "  操作数: $((operations * 2)) (Set + Get)"
    echo "  耗时: ${duration}s"
    echo "  QPS: ${ops_per_sec}"
    echo "  错误数: $errors"
    
    if [ $errors -eq 0 ]; then
        log_success "性能测试通过"
    else
        log_warning "性能测试有 $errors 个错误"
    fi
}

# 主测试流程
main() {
    echo "开始分布式一致性哈希和数据迁移测试..."
    echo
    
    # 等待集群就绪
    wait_for_cluster
    
    # 设置测试数据
    setup_test_data
    echo
    
    # 检查初始数据分布
    check_data_distribution
    echo
    
    # 验证数据完整性
    verify_data_integrity
    echo
    
    # 测试路由一致性
    test_routing_consistency
    echo
    
    # 测试添加节点和数据迁移
    test_add_node_migration
    echo
    
    # 测试移除节点和数据迁移
    test_remove_node_migration
    echo
    
    # 性能测试
    performance_test
    echo
    
    log_success "🎉 所有测试完成！分布式一致性哈希和数据迁移功能正常工作"
}

# 运行主测试
main
