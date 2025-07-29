package monitoring

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

//监控
// RingVisualizer 哈希环可视化器
// 提供终端可视化界面显示哈希环状态
type RingVisualizer struct {
	width  int
	height int
	radius int
	centerX int
	centerY int
}

// NewRingVisualizer 创建可视化器
func NewRingVisualizer(width, height int) *RingVisualizer {
	radius := min(width, height) / 3
	return &RingVisualizer{
		width:   width,
		height:  height,
		radius:  radius,
		centerX: width / 2,
		centerY: height / 2,
	}
}

// VisualizationConfig 可视化配置
type VisualizationConfig struct {
	ShowVirtualNodes bool
	ShowDataKeys     bool
	ShowMigrations   bool
	CompactMode      bool
	ColorEnabled     bool
}

// RenderRing 渲染哈希环
func (rv *RingVisualizer) RenderRing(snapshot *HashRingSnapshot, config VisualizationConfig) string {
	if snapshot == nil {
		return "❌ 无快照数据"
	}

	var output strings.Builder
	
	// 标题
	output.WriteString(rv.renderTitle(snapshot))
	output.WriteString("\n")
	
	// 环形图
	output.WriteString(rv.renderCircularRing(snapshot, config))
	output.WriteString("\n")
	
	// 详细信息
	output.WriteString(rv.renderDetails(snapshot, config))
	
	return output.String()
}

// renderTitle 渲染标题
func (rv *RingVisualizer) renderTitle(snapshot *HashRingSnapshot) string {
	return fmt.Sprintf("🔄 哈希环状态快照 - %s", snapshot.Timestamp.Format("2006-01-02 15:04:05"))
}

// renderCircularRing 渲染圆形哈希环
func (rv *RingVisualizer) renderCircularRing(snapshot *HashRingSnapshot, config VisualizationConfig) string {
	// 创建画布
	canvas := make([][]rune, rv.height)
	for i := range canvas {
		canvas[i] = make([]rune, rv.width)
		for j := range canvas[i] {
			canvas[i][j] = ' '
		}
	}

	// 绘制环形边界
	rv.drawCircle(canvas, rv.centerX, rv.centerY, rv.radius, '○')
	
	// 绘制节点
	rv.drawNodes(canvas, snapshot.Nodes)
	
	// 绘制虚拟节点（如果启用）
	if config.ShowVirtualNodes {
		rv.drawVirtualNodes(canvas, snapshot.VirtualNodes)
	}
	
	// 绘制数据点（如果启用）
	if config.ShowDataKeys {
		rv.drawDataPoints(canvas, snapshot.DataDistribution)
	}

	// 转换画布为字符串
	var result strings.Builder
	for _, row := range canvas {
		result.WriteString(string(row))
		result.WriteString("\n")
	}
	
	return result.String()
}

// drawCircle 绘制圆形
func (rv *RingVisualizer) drawCircle(canvas [][]rune, centerX, centerY, radius int, char rune) {
	for angle := 0; angle < 360; angle += 2 {
		radian := float64(angle) * math.Pi / 180
		x := centerX + int(float64(radius)*math.Cos(radian))
		y := centerY + int(float64(radius)*math.Sin(radian))
		
		if x >= 0 && x < rv.width && y >= 0 && y < rv.height {
			canvas[y][x] = char
		}
	}
}

// drawNodes 绘制物理节点
func (rv *RingVisualizer) drawNodes(canvas [][]rune, nodes []NodeInfo) {
	nodeSymbols := []rune{'①', '②', '③', '④', '⑤', '⑥', '⑦', '⑧', '⑨', '⑩'}
	
	for i, node := range nodes {
		symbol := nodeSymbols[i%len(nodeSymbols)]
		rv.drawNodeAtPosition(canvas, node.Position, symbol)
	}
}

// drawVirtualNodes 绘制虚拟节点
func (rv *RingVisualizer) drawVirtualNodes(canvas [][]rune, virtualNodes []VirtualNodeInfo) {
	for _, vnode := range virtualNodes {
		rv.drawNodeAtPosition(canvas, vnode.Position, '·')
	}
}

// drawDataPoints 绘制数据点
func (rv *RingVisualizer) drawDataPoints(canvas [][]rune, dataDistribution map[string]DataLocationInfo) {
	for _, data := range dataDistribution {
		rv.drawNodeAtPosition(canvas, data.Position, '•')
	}
}

// drawNodeAtPosition 在指定位置绘制节点
func (rv *RingVisualizer) drawNodeAtPosition(canvas [][]rune, position float64, symbol rune) {
	// 将角度转换为弧度
	radian := position * math.Pi / 180
	
	// 计算在环上的坐标
	x := rv.centerX + int(float64(rv.radius)*math.Cos(radian))
	y := rv.centerY + int(float64(rv.radius)*math.Sin(radian))
	
	if x >= 0 && x < rv.width && y >= 0 && y < rv.height {
		canvas[y][x] = symbol
	}
}

// renderDetails 渲染详细信息
func (rv *RingVisualizer) renderDetails(snapshot *HashRingSnapshot, config VisualizationConfig) string {
	var output strings.Builder
	
	// 节点信息
	output.WriteString("📊 节点信息:\n")
	for i, node := range snapshot.Nodes {
		symbol := []rune{'①', '②', '③', '④', '⑤', '⑥', '⑦', '⑧', '⑨', '⑩'}[i%10]
		output.WriteString(fmt.Sprintf("  %c %s: 位置=%.1f°, 数据=%d个, 虚拟节点=%d个, 状态=%s\n",
			symbol, node.NodeID, node.Position, node.DataCount, node.VirtualCount, node.Status))
	}
	
	// 负载均衡信息
	output.WriteString("\n⚖️  负载均衡:\n")
	lb := snapshot.LoadBalance
	output.WriteString(fmt.Sprintf("  最大负载: %d, 最小负载: %d, 平均负载: %.1f\n", 
		lb.MaxLoad, lb.MinLoad, lb.AvgLoad))
	output.WriteString(fmt.Sprintf("  方差: %.2f, 均衡分数: %.1f/100\n", 
		lb.Variance, lb.BalanceScore))
	
	// 数据分布（如果启用）
	if config.ShowDataKeys && len(snapshot.DataDistribution) > 0 {
		output.WriteString("\n📍 数据分布:\n")
		
		// 按位置排序
		var dataList []DataLocationInfo
		for _, data := range snapshot.DataDistribution {
			dataList = append(dataList, data)
		}
		sort.Slice(dataList, func(i, j int) bool {
			return dataList[i].Position < dataList[j].Position
		})
		
		for _, data := range dataList {
			migrationInfo := ""
			if data.MigrationID != "" {
				migrationInfo = fmt.Sprintf(" [迁移中: %s]", data.MigrationID)
			}
			output.WriteString(fmt.Sprintf("  • %s: 位置=%.1f°, 节点=%s%s\n",
				data.Key, data.Position, data.OwnerNode, migrationInfo))
		}
	}
	
	// 环统计
	output.WriteString(fmt.Sprintf("\n🔢 环统计: 总大小=%d, 节点数=%d, 数据项=%d\n",
		snapshot.RingSize, len(snapshot.Nodes), len(snapshot.DataDistribution)))
	
	return output.String()
}

// RenderComparison 渲染对比视图
func (rv *RingVisualizer) RenderComparison(before, after *HashRingSnapshot) string {
	if before == nil || after == nil {
		return "❌ 缺少对比数据"
	}

	var output strings.Builder
	
	output.WriteString("🔄 哈希环变化对比\n")
	output.WriteString("==================\n\n")
	
	// 节点变化
	output.WriteString("📊 节点变化:\n")
	beforeNodes := make(map[string]NodeInfo)
	afterNodes := make(map[string]NodeInfo)
	
	for _, node := range before.Nodes {
		beforeNodes[node.NodeID] = node
	}
	for _, node := range after.Nodes {
		afterNodes[node.NodeID] = node
	}
	
	// 新增节点
	for nodeID, node := range afterNodes {
		if _, exists := beforeNodes[nodeID]; !exists {
			output.WriteString(fmt.Sprintf("  ➕ 新增: %s (位置=%.1f°)\n", nodeID, node.Position))
		}
	}
	
	// 移除节点
	for nodeID, node := range beforeNodes {
		if _, exists := afterNodes[nodeID]; !exists {
			output.WriteString(fmt.Sprintf("  ➖ 移除: %s (位置=%.1f°)\n", nodeID, node.Position))
		}
	}
	
	// 数据迁移分析
	output.WriteString("\n📦 数据迁移分析:\n")
	migrationCount := 0
	
	for key, afterData := range after.DataDistribution {
		if beforeData, exists := before.DataDistribution[key]; exists {
			if beforeData.OwnerNode != afterData.OwnerNode {
				output.WriteString(fmt.Sprintf("  🔄 %s: %s → %s\n", 
					key, beforeData.OwnerNode, afterData.OwnerNode))
				migrationCount++
			}
		} else {
			output.WriteString(fmt.Sprintf("  ➕ %s: 新数据 → %s\n", 
				key, afterData.OwnerNode))
		}
	}
	
	// 负载均衡变化
	output.WriteString("\n⚖️  负载均衡变化:\n")
	output.WriteString(fmt.Sprintf("  均衡分数: %.1f → %.1f (变化: %+.1f)\n",
		before.LoadBalance.BalanceScore, after.LoadBalance.BalanceScore,
		after.LoadBalance.BalanceScore-before.LoadBalance.BalanceScore))
	
	output.WriteString(fmt.Sprintf("\n📈 变化总结: 迁移了 %d 个数据项\n", migrationCount))
	
	return output.String()
}

// RenderMigrationProgress 渲染迁移进度
func (rv *RingVisualizer) RenderMigrationProgress(tracker *MigrationTracker) string {
	var output strings.Builder
	
	output.WriteString("🚀 数据迁移进度监控\n")
	output.WriteString("===================\n\n")
	
	// 活跃会话
	activeSessions := tracker.GetActiveSessions()
	if len(activeSessions) > 0 {
		output.WriteString("📋 活跃迁移会话:\n")
		for sessionID, session := range activeSessions {
			progress := 0.0
			if session.TotalRecords > 0 {
				progress = float64(session.CompletedRecords) / float64(session.TotalRecords) * 100
			}
			
			output.WriteString(fmt.Sprintf("  🔄 %s (%s)\n", sessionID, session.Type))
			output.WriteString(fmt.Sprintf("     进度: %d/%d (%.1f%%), 失败: %d\n",
				session.CompletedRecords, session.TotalRecords, progress, session.FailedRecords))
			output.WriteString(fmt.Sprintf("     开始时间: %s\n", 
				session.StartTime.Format("15:04:05")))
		}
	} else {
		output.WriteString("📋 当前无活跃迁移会话\n")
	}
	
	// 统计信息
	stats := tracker.GetStatistics()
	output.WriteString("\n📊 迁移统计:\n")
	output.WriteString(fmt.Sprintf("  总迁移数: %d\n", stats.TotalMigrations))
	output.WriteString(fmt.Sprintf("  成功: %d, 失败: %d\n", stats.CompletedMigrations, stats.FailedMigrations))
	output.WriteString(fmt.Sprintf("  成功率: %.1f%%\n", stats.SuccessRate))
	output.WriteString(fmt.Sprintf("  平均耗时: %v\n", stats.AverageDuration))
	
	// 失败记录
	failedMigrations := tracker.GetFailedMigrations()
	if len(failedMigrations) > 0 {
		output.WriteString("\n❌ 失败记录:\n")
		for _, record := range failedMigrations {
			output.WriteString(fmt.Sprintf("  • %s: %s → %s (原因: %s)\n",
				record.Key, record.SourceNode, record.TargetNode, record.Error))
		}
	}
	
	return output.String()
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
//监控
