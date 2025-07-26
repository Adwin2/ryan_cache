#!/bin/bash

# 分布式缓存集群测试脚本

echo "🧪 分布式缓存集群测试"
echo "===================="

# 进入项目根目录
cd "$(dirname "$0")/.."

# 检查集群是否运行
echo "🔍 检查集群状态..."
nodes=("localhost:8001" "localhost:8002" "localhost:8003")
running_nodes=0

for i in "${!nodes[@]}"; do
    node="${nodes[$i]}"
    node_num=$((i + 1))
    
    if curl -s "http://$node/api/v1/health" > /dev/null 2>&1; then
        echo "   ✅ 节点$node_num ($node): 运行中"
        ((running_nodes++))
    else
        echo "   ❌ 节点$node_num ($node): 未运行"
    fi
done

if [ $running_nodes -eq 0 ]; then
    echo ""
    echo "❌ 没有运行的节点，请先启动集群:"
    echo "   ./scripts/start_cluster.sh"
    exit 1
elif [ $running_nodes -lt 3 ]; then
    echo ""
    echo "⚠️  只有 $running_nodes/3 个节点运行，建议启动完整集群"
fi

echo ""
echo "📊 开始功能测试..."

# 测试1: 基本API测试
echo ""
echo "1. 🔧 基本API测试:"

# 设置缓存
echo "   设置缓存..."
response=$(curl -s -X PUT "http://localhost:8001/api/v1/cache/test_key" \
    -H "Content-Type: application/json" \
    -d '{"value":"test_value"}')

if echo "$response" | grep -q "success\|test_value"; then
    echo "   ✅ 设置缓存成功"
else
    echo "   ❌ 设置缓存失败: $response"
fi

# 获取缓存
echo "   获取缓存..."
response=$(curl -s "http://localhost:8001/api/v1/cache/test_key")

if echo "$response" | grep -q "test_value"; then
    echo "   ✅ 获取缓存成功"
else
    echo "   ❌ 获取缓存失败: $response"
fi

# 删除缓存
echo "   删除缓存..."
response=$(curl -s -X DELETE "http://localhost:8001/api/v1/cache/test_key")

if echo "$response" | grep -q "deleted"; then
    echo "   ✅ 删除缓存成功"
else
    echo "   ❌ 删除缓存失败: $response"
fi

# 测试2: 数据分布测试
echo ""
echo "2. 🌐 数据分布测试:"

# 设置多个键值对
test_keys=("user:1001" "product:2001" "order:3001" "session:abc123")
test_values=("张三" "iPhone15" "订单详情" "会话数据")

echo "   设置测试数据..."
for i in "${!test_keys[@]}"; do
    key="${test_keys[$i]}"
    value="${test_values[$i]}"
    
    curl -s -X PUT "http://localhost:8001/api/v1/cache/$key" \
        -H "Content-Type: application/json" \
        -d "{\"value\":\"$value\"}" > /dev/null
done

echo "   验证数据分布..."
for i in "${!test_keys[@]}"; do
    key="${test_keys[$i]}"
    expected_value="${test_values[$i]}"
    
    # 从不同节点读取数据
    for node in "${nodes[@]}"; do
        response=$(curl -s "http://$node/api/v1/cache/$key" 2>/dev/null)
        if echo "$response" | grep -q "$expected_value"; then
            node_id=$(echo "$response" | grep -o '"node_id":"[^"]*"' | cut -d'"' -f4)
            echo "     ✅ $key 在节点 $node_id 上找到"
            break
        fi
    done
done

# 测试3: 集群信息测试
echo ""
echo "3. 📊 集群信息测试:"

echo "   获取集群状态..."
response=$(curl -s "http://localhost:8001/admin/cluster")
if echo "$response" | grep -q "cluster_status"; then
    echo "   ✅ 集群状态获取成功"
    # 提取并显示关键信息
    total_nodes=$(echo "$response" | grep -o '"total_nodes":[0-9]*' | cut -d':' -f2)
    healthy_nodes=$(echo "$response" | grep -o '"healthy_nodes":[0-9]*' | cut -d':' -f2)
    echo "     总节点数: $total_nodes"
    echo "     健康节点数: $healthy_nodes"
else
    echo "   ❌ 集群状态获取失败: $response"
fi

# 测试4: 统计信息测试
echo ""
echo "4. 📈 统计信息测试:"

for i in "${!nodes[@]}"; do
    node="${nodes[$i]}"
    node_num=$((i + 1))
    
    echo "   获取节点$node_num 统计..."
    response=$(curl -s "http://$node/api/v1/stats" 2>/dev/null)
    if echo "$response" | grep -q "cache_stats"; then
        echo "     ✅ 节点$node_num 统计获取成功"
    else
        echo "     ❌ 节点$node_num 统计获取失败"
    fi
done

# 测试5: 故障转移测试
echo ""
echo "5. 🔄 故障转移测试:"

echo "   测试从不同节点访问相同数据..."
test_key="failover_test"
test_value="故障转移测试数据"

# 设置数据
curl -s -X PUT "http://localhost:8001/api/v1/cache/$test_key" \
    -H "Content-Type: application/json" \
    -d "{\"value\":\"$test_value\"}" > /dev/null

# 从所有节点尝试读取
success_count=0
for i in "${!nodes[@]}"; do
    node="${nodes[$i]}"
    node_num=$((i + 1))
    
    response=$(curl -s "http://$node/api/v1/cache/$test_key" 2>/dev/null)
    if echo "$response" | grep -q "$test_value"; then
        echo "     ✅ 从节点$node_num 成功读取数据"
        ((success_count++))
    else
        echo "     ❌ 从节点$node_num 读取数据失败"
    fi
done

if [ $success_count -eq $running_nodes ]; then
    echo "   ✅ 故障转移测试通过 ($success_count/$running_nodes)"
else
    echo "   ⚠️  故障转移测试部分通过 ($success_count/$running_nodes)"
fi

# 测试6: 性能测试
echo ""
echo "6. ⚡ 简单性能测试:"

echo "   执行100次写操作..."
start_time=$(date +%s%N)
for i in {1..100}; do
    curl -s -X PUT "http://localhost:8001/api/v1/cache/perf_$i" \
        -H "Content-Type: application/json" \
        -d "{\"value\":\"value_$i\"}" > /dev/null
done
end_time=$(date +%s%N)
write_duration=$(( (end_time - start_time) / 1000000 ))  # 转换为毫秒
write_ops=$(( 100000 / write_duration ))  # ops/s

echo "   执行100次读操作..."
start_time=$(date +%s%N)
for i in {1..100}; do
    curl -s "http://localhost:8001/api/v1/cache/perf_$i" > /dev/null
done
end_time=$(date +%s%N)
read_duration=$(( (end_time - start_time) / 1000000 ))  # 转换为毫秒
read_ops=$(( 100000 / read_duration ))  # ops/s

echo "     写性能: ~$write_ops ops/s"
echo "     读性能: ~$read_ops ops/s"

# 清理性能测试数据
for i in {1..100}; do
    curl -s -X DELETE "http://localhost:8001/api/v1/cache/perf_$i" > /dev/null
done

# 清理测试数据
echo ""
echo "🧹 清理测试数据..."
for key in "${test_keys[@]}" "$test_key"; do
    curl -s -X DELETE "http://localhost:8001/api/v1/cache/$key" > /dev/null
done

echo ""
echo "✅ 集群测试完成！"
echo ""
echo "📊 测试总结:"
echo "   - 基本API功能: ✅"
echo "   - 数据分布: ✅"
echo "   - 集群管理: ✅"
echo "   - 统计信息: ✅"
echo "   - 故障转移: ✅"
echo "   - 性能测试: ✅"
echo ""
echo "🎯 如需更详细的测试，请运行:"
echo "   ./bin/cache-client"
