package monitoring

import (
	"fmt"
	"log"
	"time"

	"tdd-learning/core"
)

//监控
// MonitoringIntegration 监控集成器
// 提供非侵入式的监控集成，不影响核心业务逻辑性能
type MonitoringIntegration struct {
	monitor         *HashRingMonitor
	distributedCache *core.DistributedCache
	enabled         bool
	dataKeys        []string // 当前跟踪的数据key列表
}

// NewMonitoringIntegration 创建监控集成器
func NewMonitoringIntegration(dc *core.DistributedCache) *MonitoringIntegration {
	monitor := NewHashRingMonitor(dc)
	
	return &MonitoringIntegration{
		monitor:          monitor,
		distributedCache: dc,
		enabled:          true,
		dataKeys:         make([]string, 0),
	}
}

// GetMonitor 获取监控器
func (mi *MonitoringIntegration) GetMonitor() *HashRingMonitor {
	return mi.monitor
}

// Enable 启用监控
func (mi *MonitoringIntegration) Enable() {
	mi.enabled = true
	mi.monitor.Enable()
	log.Println("🔍 监控系统已启用")
}

// Disable 禁用监控
func (mi *MonitoringIntegration) Disable() {
	mi.enabled = false
	mi.monitor.Disable()
	log.Println("🔍 监控系统已禁用")
}

// IsEnabled 检查是否启用
func (mi *MonitoringIntegration) IsEnabled() bool {
	return mi.enabled && mi.monitor.IsEnabled()
}

// TrackDataKeys 跟踪数据key列表
func (mi *MonitoringIntegration) TrackDataKeys(keys []string) {
	if !mi.IsEnabled() {
		return
	}
	
	mi.dataKeys = make([]string, len(keys))
	copy(mi.dataKeys, keys)
	
	// 更新监控器的数据分布
	mi.monitor.UpdateDataDistribution(keys)
	
	log.Printf("🔍 开始跟踪 %d 个数据key", len(keys))
}

// AddDataKey 添加数据key到跟踪列表
func (mi *MonitoringIntegration) AddDataKey(key string) {
	if !mi.IsEnabled() {
		return
	}
	
	// 检查是否已存在
	for _, existingKey := range mi.dataKeys {
		if existingKey == key {
			return
		}
	}
	
	mi.dataKeys = append(mi.dataKeys, key)
	mi.monitor.UpdateDataDistribution(mi.dataKeys)
}

// RemoveDataKey 从跟踪列表移除数据key
func (mi *MonitoringIntegration) RemoveDataKey(key string) {
	if !mi.IsEnabled() {
		return
	}
	
	for i, existingKey := range mi.dataKeys {
		if existingKey == key {
			mi.dataKeys = append(mi.dataKeys[:i], mi.dataKeys[i+1:]...)
			mi.monitor.UpdateDataDistribution(mi.dataKeys)
			break
		}
	}
}

// CaptureSnapshot 捕获快照
func (mi *MonitoringIntegration) CaptureSnapshot() *HashRingSnapshot {
	if !mi.IsEnabled() {
		return nil
	}
	
	return mi.monitor.CaptureSnapshot()
}

// OnNodeAdded 节点添加事件处理
func (mi *MonitoringIntegration) OnNodeAdded(nodeID string) string {
	if !mi.IsEnabled() {
		return ""
	}
	
	log.Printf("🔍 监控到节点添加事件: %s", nodeID)
	
	// 开始迁移会话
	tracker := mi.monitor.GetMigrationTracker()
	sessionID := tracker.StartMigrationSession("add_node", nodeID)
	
	// 捕获添加前的快照
	mi.CaptureSnapshot()
	
	return sessionID
}

// OnNodeRemoved 节点移除事件处理
func (mi *MonitoringIntegration) OnNodeRemoved(nodeID string) string {
	if !mi.IsEnabled() {
		return ""
	}
	
	log.Printf("🔍 监控到节点移除事件: %s", nodeID)
	
	// 开始迁移会话
	tracker := mi.monitor.GetMigrationTracker()
	sessionID := tracker.StartMigrationSession("remove_node", nodeID)
	
	// 捕获移除前的快照
	mi.CaptureSnapshot()
	
	return sessionID
}

// OnMigrationCompleted 迁移完成事件处理
func (mi *MonitoringIntegration) OnMigrationCompleted(sessionID string) {
	if !mi.IsEnabled() {
		return
	}
	
	log.Printf("🔍 迁移会话完成: %s", sessionID)
	
	// 结束迁移会话
	tracker := mi.monitor.GetMigrationTracker()
	tracker.EndMigrationSession(sessionID, SessionStatusCompleted)
	
	// 捕获迁移后的快照
	mi.CaptureSnapshot()
}

// TrackDataMigration 跟踪单个数据迁移
func (mi *MonitoringIntegration) TrackDataMigration(sessionID, key, sourceNode, targetNode, reason string) string {
	if !mi.IsEnabled() {
		return ""
	}
	
	tracker := mi.monitor.GetMigrationTracker()
	migrationID := tracker.TrackMigration(sessionID, key, sourceNode, targetNode, reason)
	
	// 记录哈希决策信息
	// 简化处理：使用key的长度作为hash值的近似
	keyHash := uint32(len(key) * 1000)
	decision := HashDecisionInfo{
		KeyHash:        keyHash,
		OldResponsible: sourceNode,
		NewResponsible: targetNode,
		DecisionReason: reason,
	}
	tracker.RecordHashDecision(migrationID, decision)
	
	log.Printf("🔍 跟踪数据迁移: %s (%s -> %s)", key, sourceNode, targetNode)
	
	return migrationID
}

// UpdateMigrationStatus 更新迁移状态
func (mi *MonitoringIntegration) UpdateMigrationStatus(migrationID string, status MigrationStatus, errorMsg string) {
	if !mi.IsEnabled() {
		return
	}
	
	tracker := mi.monitor.GetMigrationTracker()
	tracker.UpdateMigrationStatus(migrationID, status, errorMsg)
	
	if status == MigrationStatusFailed && errorMsg != "" {
		log.Printf("🔍 迁移失败: %s - %s", migrationID, errorMsg)
	}
}

// ValidateDataMigration 验证数据迁移
func (mi *MonitoringIntegration) ValidateDataMigration(migrationID, key, expectedValue string) bool {
	if !mi.IsEnabled() {
		return true // 监控禁用时默认验证通过
	}
	
	// 获取当前数据位置
	targetNode := mi.distributedCache.GetNodeForKey(key)
	localCache := mi.distributedCache.LocalCaches[targetNode]
	
	actualValue, found := localCache.Get(key)
	
	validation := ValidationInfo{
		PreMigrationExists:  true, // 简化处理
		PostMigrationExists: found,
		ValueMatches:        found && actualValue == expectedValue,
		ValidationTime:      time.Now(),
	}
	
	tracker := mi.monitor.GetMigrationTracker()
	tracker.RecordValidation(migrationID, validation)
	
	if !validation.ValueMatches {
		log.Printf("🔍 数据验证失败: %s (期望: %s, 实际: %s, 找到: %v)", 
			key, expectedValue, actualValue, found)
	}
	
	return validation.ValueMatches
}

// GetVisualization 获取可视化
func (mi *MonitoringIntegration) GetVisualization(config VisualizationConfig) string {
	if !mi.IsEnabled() {
		return "监控已禁用"
	}
	
	snapshot := mi.monitor.GetLatestSnapshot()
	if snapshot == nil {
		return "无快照数据"
	}
	
	visualizer := NewRingVisualizer(80, 40)
	return visualizer.RenderRing(snapshot, config)
}

// GetMigrationProgress 获取迁移进度
func (mi *MonitoringIntegration) GetMigrationProgress() string {
	if !mi.IsEnabled() {
		return "监控已禁用"
	}
	
	tracker := mi.monitor.GetMigrationTracker()
	visualizer := NewRingVisualizer(80, 40)
	return visualizer.RenderMigrationProgress(tracker)
}

// GetComparisonView 获取对比视图
func (mi *MonitoringIntegration) GetComparisonView() string {
	if !mi.IsEnabled() {
		return "监控已禁用"
	}
	
	history := mi.monitor.GetSnapshotHistory(2)
	if len(history) < 2 {
		return "需要至少2个快照进行对比"
	}
	
	visualizer := NewRingVisualizer(80, 40)
	return visualizer.RenderComparison(&history[0], &history[1])
}

// GetFailureReport 获取故障报告
func (mi *MonitoringIntegration) GetFailureReport() FailureReport {
	tracker := mi.monitor.GetMigrationTracker()
	return tracker.GenerateFailureReport()
}

// GetStatistics 获取统计信息
func (mi *MonitoringIntegration) GetStatistics() MigrationStatistics {
	tracker := mi.monitor.GetMigrationTracker()
	return tracker.GetStatistics()
}

// PrintStatus 打印监控状态
func (mi *MonitoringIntegration) PrintStatus() {
	if !mi.IsEnabled() {
		fmt.Println("🔍 监控状态: 已禁用")
		return
	}
	
	fmt.Println("🔍 监控状态: 已启用")
	fmt.Printf("📊 跟踪数据key数量: %d\n", len(mi.dataKeys))
	
	stats := mi.GetStatistics()
	fmt.Printf("📈 迁移统计: 总数=%d, 成功=%d, 失败=%d, 成功率=%.1f%%\n",
		stats.TotalMigrations, stats.CompletedMigrations, stats.FailedMigrations, stats.SuccessRate)
	
	snapshot := mi.monitor.GetLatestSnapshot()
	if snapshot != nil {
		fmt.Printf("📸 最新快照: %s (节点数=%d)\n", 
			snapshot.Timestamp.Format("15:04:05"), len(snapshot.Nodes))
	}
}
//监控
