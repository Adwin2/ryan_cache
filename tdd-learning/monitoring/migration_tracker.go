package monitoring

import (
	"fmt"
	"sync"
	"time"
)

//监控
// MigrationTracker 数据迁移追踪器
// 实现类似APM链路追踪的监控机制，记录每个数据项的完整迁移路径
type MigrationTracker struct {
	mu              sync.RWMutex
	migrations      map[string]*MigrationRecord
	activeMigrations map[string]*MigrationSession
	completedCount  int64
	failedCount     int64
}

// MigrationRecord 迁移记录
type MigrationRecord struct {
	ID              string                 `json:"id"`
	Key             string                 `json:"key"`
	Value           string                 `json:"value,omitempty"`
	SourceNode      string                 `json:"source_node"`
	TargetNode      string                 `json:"target_node"`
	Reason          string                 `json:"reason"`          // 迁移原因：node_added, node_removed, rebalance
	Status          MigrationStatus        `json:"status"`
	StartTime       time.Time              `json:"start_time"`
	EndTime         *time.Time             `json:"end_time,omitempty"`
	Duration        time.Duration          `json:"duration"`
	Error           string                 `json:"error,omitempty"`
	StackTrace      string                 `json:"stack_trace,omitempty"`
	HashDecision    HashDecisionInfo       `json:"hash_decision"`
	ValidationInfo  ValidationInfo         `json:"validation_info"`
	PerformanceInfo PerformanceInfo        `json:"performance_info"`
}

// MigrationSession 迁移会话（批量迁移）
type MigrationSession struct {
	ID              string                    `json:"id"`
	Type            string                    `json:"type"`           // add_node, remove_node, rebalance
	TriggerNode     string                    `json:"trigger_node"`   // 触发迁移的节点
	StartTime       time.Time                 `json:"start_time"`
	EndTime         *time.Time                `json:"end_time,omitempty"`
	TotalRecords    int                       `json:"total_records"`
	CompletedRecords int                      `json:"completed_records"`
	FailedRecords   int                       `json:"failed_records"`
	Records         map[string]*MigrationRecord `json:"records"`
	Status          MigrationSessionStatus   `json:"status"`
}

// MigrationStatus 迁移状态
type MigrationStatus string

const (
	MigrationStatusPending    MigrationStatus = "pending"
	MigrationStatusInProgress MigrationStatus = "in_progress"
	MigrationStatusCompleted  MigrationStatus = "completed"
	MigrationStatusFailed     MigrationStatus = "failed"
	MigrationStatusRolledBack MigrationStatus = "rolled_back"
)

// MigrationSessionStatus 迁移会话状态
type MigrationSessionStatus string

const (
	SessionStatusActive    MigrationSessionStatus = "active"
	SessionStatusCompleted MigrationSessionStatus = "completed"
	SessionStatusFailed    MigrationSessionStatus = "failed"
	SessionStatusAborted   MigrationSessionStatus = "aborted"
)

// HashDecisionInfo 哈希决策信息
type HashDecisionInfo struct {
	KeyHash         uint32 `json:"key_hash"`
	OldResponsible  string `json:"old_responsible"`
	NewResponsible  string `json:"new_responsible"`
	HashRingBefore  string `json:"hash_ring_before"`  // 迁移前的哈希环状态摘要
	HashRingAfter   string `json:"hash_ring_after"`   // 迁移后的哈希环状态摘要
	DecisionReason  string `json:"decision_reason"`   // 决策依据
}

// ValidationInfo 验证信息
type ValidationInfo struct {
	PreMigrationExists  bool      `json:"pre_migration_exists"`
	PostMigrationExists bool      `json:"post_migration_exists"`
	ValueMatches        bool      `json:"value_matches"`
	ChecksumBefore      string    `json:"checksum_before,omitempty"`
	ChecksumAfter       string    `json:"checksum_after,omitempty"`
	ValidationTime      time.Time `json:"validation_time"`
}

// PerformanceInfo 性能信息
type PerformanceInfo struct {
	NetworkLatency    time.Duration `json:"network_latency"`
	SerializationTime time.Duration `json:"serialization_time"`
	StorageWriteTime  time.Duration `json:"storage_write_time"`
	TotalDataSize     int64         `json:"total_data_size"`
	Throughput        float64       `json:"throughput"` // bytes/second
}
//监控

//监控
// NewMigrationTracker 创建迁移追踪器
func NewMigrationTracker() *MigrationTracker {
	return &MigrationTracker{
		migrations:       make(map[string]*MigrationRecord),
		activeMigrations: make(map[string]*MigrationSession),
	}
}

// StartMigrationSession 开始迁移会话
func (mt *MigrationTracker) StartMigrationSession(sessionType, triggerNode string) string {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	sessionID := fmt.Sprintf("session_%d_%s", time.Now().UnixNano(), sessionType)
	
	session := &MigrationSession{
		ID:          sessionID,
		Type:        sessionType,
		TriggerNode: triggerNode,
		StartTime:   time.Now(),
		Records:     make(map[string]*MigrationRecord),
		Status:      SessionStatusActive,
	}

	mt.activeMigrations[sessionID] = session
	return sessionID
}

// EndMigrationSession 结束迁移会话
func (mt *MigrationTracker) EndMigrationSession(sessionID string, status MigrationSessionStatus) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if session, exists := mt.activeMigrations[sessionID]; exists {
		now := time.Now()
		session.EndTime = &now
		session.Status = status
		
		// 统计结果
		for _, record := range session.Records {
			if record.Status == MigrationStatusCompleted {
				session.CompletedRecords++
			} else if record.Status == MigrationStatusFailed {
				session.FailedRecords++
			}
		}
		session.TotalRecords = len(session.Records)

		// 移动到历史记录
		delete(mt.activeMigrations, sessionID)
	}
}

// TrackMigration 追踪单个数据迁移
func (mt *MigrationTracker) TrackMigration(sessionID, key, sourceNode, targetNode, reason string) string {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	migrationID := fmt.Sprintf("mig_%d_%s", time.Now().UnixNano(), key)
	
	record := &MigrationRecord{
		ID:         migrationID,
		Key:        key,
		SourceNode: sourceNode,
		TargetNode: targetNode,
		Reason:     reason,
		Status:     MigrationStatusPending,
		StartTime:  time.Now(),
	}

	mt.migrations[migrationID] = record

	// 添加到会话中
	if session, exists := mt.activeMigrations[sessionID]; exists {
		session.Records[migrationID] = record
	}

	return migrationID
}

// UpdateMigrationStatus 更新迁移状态
func (mt *MigrationTracker) UpdateMigrationStatus(migrationID string, status MigrationStatus, errorMsg string) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if record, exists := mt.migrations[migrationID]; exists {
		record.Status = status
		if status == MigrationStatusCompleted || status == MigrationStatusFailed {
			now := time.Now()
			record.EndTime = &now
			record.Duration = now.Sub(record.StartTime)
		}
		
		if errorMsg != "" {
			record.Error = errorMsg
		}

		// 更新统计
		if status == MigrationStatusCompleted {
			mt.completedCount++
		} else if status == MigrationStatusFailed {
			mt.failedCount++
		}
	}
}

// RecordHashDecision 记录哈希决策信息
func (mt *MigrationTracker) RecordHashDecision(migrationID string, decision HashDecisionInfo) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if record, exists := mt.migrations[migrationID]; exists {
		record.HashDecision = decision
	}
}

// RecordValidation 记录验证信息
func (mt *MigrationTracker) RecordValidation(migrationID string, validation ValidationInfo) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if record, exists := mt.migrations[migrationID]; exists {
		record.ValidationInfo = validation
	}
}

// RecordPerformance 记录性能信息
func (mt *MigrationTracker) RecordPerformance(migrationID string, performance PerformanceInfo) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	if record, exists := mt.migrations[migrationID]; exists {
		record.PerformanceInfo = performance
	}
}

// GetMigrationRecord 获取迁移记录
func (mt *MigrationTracker) GetMigrationRecord(migrationID string) *MigrationRecord {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	if record, exists := mt.migrations[migrationID]; exists {
		// 返回副本
		recordCopy := *record
		return &recordCopy
	}
	return nil
}

// GetActiveSessions 获取活跃的迁移会话
func (mt *MigrationTracker) GetActiveSessions() map[string]*MigrationSession {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	result := make(map[string]*MigrationSession)
	for id, session := range mt.activeMigrations {
		sessionCopy := *session
		result[id] = &sessionCopy
	}
	return result
}

// GetMigrationsByKey 根据key获取迁移记录
func (mt *MigrationTracker) GetMigrationsByKey(key string) []*MigrationRecord {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	var result []*MigrationRecord
	for _, record := range mt.migrations {
		if record.Key == key {
			recordCopy := *record
			result = append(result, &recordCopy)
		}
	}
	return result
}

// GetFailedMigrations 获取失败的迁移记录
func (mt *MigrationTracker) GetFailedMigrations() []*MigrationRecord {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	var result []*MigrationRecord
	for _, record := range mt.migrations {
		if record.Status == MigrationStatusFailed {
			recordCopy := *record
			result = append(result, &recordCopy)
		}
	}
	return result
}

// GetStatistics 获取迁移统计信息
func (mt *MigrationTracker) GetStatistics() MigrationStatistics {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	stats := MigrationStatistics{
		TotalMigrations:     int64(len(mt.migrations)),
		CompletedMigrations: mt.completedCount,
		FailedMigrations:    mt.failedCount,
		ActiveSessions:      int64(len(mt.activeMigrations)),
	}

	if stats.TotalMigrations > 0 {
		stats.SuccessRate = float64(stats.CompletedMigrations) / float64(stats.TotalMigrations) * 100
	}

	// 计算平均迁移时间
	var totalDuration time.Duration
	var completedCount int64
	for _, record := range mt.migrations {
		if record.Status == MigrationStatusCompleted && record.Duration > 0 {
			totalDuration += record.Duration
			completedCount++
		}
	}
	
	if completedCount > 0 {
		stats.AverageDuration = totalDuration / time.Duration(completedCount)
	}

	return stats
}

// MigrationStatistics 迁移统计信息
type MigrationStatistics struct {
	TotalMigrations     int64         `json:"total_migrations"`
	CompletedMigrations int64         `json:"completed_migrations"`
	FailedMigrations    int64         `json:"failed_migrations"`
	ActiveSessions      int64         `json:"active_sessions"`
	SuccessRate         float64       `json:"success_rate"`
	AverageDuration     time.Duration `json:"average_duration"`
}

// GenerateFailureReport 生成故障报告
func (mt *MigrationTracker) GenerateFailureReport() FailureReport {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	report := FailureReport{
		GeneratedAt:    time.Now(),
		FailedRecords:  make([]*MigrationRecord, 0),
		FailureReasons: make(map[string]int),
		Recommendations: make([]string, 0),
	}

	for _, record := range mt.migrations {
		if record.Status == MigrationStatusFailed {
			recordCopy := *record
			report.FailedRecords = append(report.FailedRecords, &recordCopy)
			
			if record.Error != "" {
				report.FailureReasons[record.Error]++
			}
		}
	}

	// 生成修复建议
	report.Recommendations = mt.generateRecommendations(report.FailureReasons)

	return report
}

// FailureReport 故障报告
type FailureReport struct {
	GeneratedAt     time.Time                `json:"generated_at"`
	FailedRecords   []*MigrationRecord       `json:"failed_records"`
	FailureReasons  map[string]int           `json:"failure_reasons"`
	Recommendations []string                 `json:"recommendations"`
}

// generateRecommendations 生成修复建议
func (mt *MigrationTracker) generateRecommendations(failureReasons map[string]int) []string {
	var recommendations []string

	for reason, count := range failureReasons {
		switch {
		case count > 10:
			recommendations = append(recommendations, fmt.Sprintf("高频故障 '%s' 出现 %d 次，建议检查系统配置", reason, count))
		case count > 5:
			recommendations = append(recommendations, fmt.Sprintf("中频故障 '%s' 出现 %d 次，建议监控相关组件", reason, count))
		default:
			recommendations = append(recommendations, fmt.Sprintf("偶发故障 '%s' 出现 %d 次，建议记录观察", reason, count))
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "暂无故障记录，系统运行正常")
	}

	return recommendations
}
//监控
