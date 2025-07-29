package monitoring

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

//监控
// MonitoringAPI 监控API服务
type MonitoringAPI struct {
	monitor    *HashRingMonitor
	visualizer *RingVisualizer
}

// NewMonitoringAPI 创建监控API
func NewMonitoringAPI(monitor *HashRingMonitor) *MonitoringAPI {
	return &MonitoringAPI{
		monitor:    monitor,
		visualizer: NewRingVisualizer(80, 40),
	}
}

// RegisterRoutes 注册监控API路由
func (api *MonitoringAPI) RegisterRoutes(router *gin.Engine) {
	monitorGroup := router.Group("/monitor")
	{
		// 哈希环监控
		monitorGroup.GET("/ring/snapshot", api.GetRingSnapshot)
		monitorGroup.GET("/ring/history", api.GetRingHistory)
		monitorGroup.GET("/ring/visualization", api.GetRingVisualization)
		monitorGroup.GET("/ring/comparison", api.GetRingComparison)
		
		// 数据迁移监控
		monitorGroup.GET("/migration/active", api.GetActiveMigrations)
		monitorGroup.GET("/migration/statistics", api.GetMigrationStatistics)
		monitorGroup.GET("/migration/failures", api.GetMigrationFailures)
		monitorGroup.GET("/migration/record/:id", api.GetMigrationRecord)
		monitorGroup.GET("/migration/progress", api.GetMigrationProgress)
		
		// 故障诊断
		monitorGroup.GET("/diagnosis/report", api.GetDiagnosisReport)
		monitorGroup.POST("/diagnosis/analyze", api.AnalyzeMigrationIssues)
		
		// 监控控制
		monitorGroup.POST("/control/enable", api.EnableMonitoring)
		monitorGroup.POST("/control/disable", api.DisableMonitoring)
		monitorGroup.POST("/control/capture", api.CaptureSnapshot)
		
		// 数据更新
		monitorGroup.POST("/data/update", api.UpdateDataDistribution)
	}
}

// GetRingSnapshot 获取哈希环快照
func (api *MonitoringAPI) GetRingSnapshot(c *gin.Context) {
	snapshot := api.monitor.GetLatestSnapshot()
	if snapshot == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "no_snapshot",
			"message": "没有可用的快照数据",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": snapshot,
		"timestamp": time.Now(),
	})
}

// GetRingHistory 获取哈希环历史
func (api *MonitoringAPI) GetRingHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 10
	}
	
	history := api.monitor.GetSnapshotHistory(limit)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": history,
		"count": len(history),
		"timestamp": time.Now(),
	})
}

// GetRingVisualization 获取哈希环可视化
func (api *MonitoringAPI) GetRingVisualization(c *gin.Context) {
	snapshot := api.monitor.GetLatestSnapshot()
	if snapshot == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "no_snapshot",
			"message": "没有可用的快照数据",
		})
		return
	}
	
	config := VisualizationConfig{
		ShowVirtualNodes: c.DefaultQuery("virtual_nodes", "false") == "true",
		ShowDataKeys:     c.DefaultQuery("data_keys", "true") == "true",
		ShowMigrations:   c.DefaultQuery("migrations", "true") == "true",
		CompactMode:      c.DefaultQuery("compact", "false") == "true",
		ColorEnabled:     c.DefaultQuery("color", "false") == "true",
	}
	
	visualization := api.visualizer.RenderRing(snapshot, config)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"text": visualization,
			"config": config,
			"snapshot_time": snapshot.Timestamp,
		},
		"timestamp": time.Now(),
	})
}

// GetRingComparison 获取哈希环对比
func (api *MonitoringAPI) GetRingComparison(c *gin.Context) {
	history := api.monitor.GetSnapshotHistory(2)
	if len(history) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "insufficient_data",
			"message": "需要至少2个快照进行对比",
		})
		return
	}
	
	before := &history[0]
	after := &history[1]
	
	comparison := api.visualizer.RenderComparison(before, after)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"comparison": comparison,
			"before": before,
			"after": after,
		},
		"timestamp": time.Now(),
	})
}

// GetActiveMigrations 获取活跃迁移
func (api *MonitoringAPI) GetActiveMigrations(c *gin.Context) {
	tracker := api.monitor.GetMigrationTracker()
	activeSessions := tracker.GetActiveSessions()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": activeSessions,
		"count": len(activeSessions),
		"timestamp": time.Now(),
	})
}

// GetMigrationStatistics 获取迁移统计
func (api *MonitoringAPI) GetMigrationStatistics(c *gin.Context) {
	tracker := api.monitor.GetMigrationTracker()
	stats := tracker.GetStatistics()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": stats,
		"timestamp": time.Now(),
	})
}

// GetMigrationFailures 获取迁移失败记录
func (api *MonitoringAPI) GetMigrationFailures(c *gin.Context) {
	tracker := api.monitor.GetMigrationTracker()
	failures := tracker.GetFailedMigrations()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": failures,
		"count": len(failures),
		"timestamp": time.Now(),
	})
}

// GetMigrationRecord 获取特定迁移记录
func (api *MonitoringAPI) GetMigrationRecord(c *gin.Context) {
	migrationID := c.Param("id")
	if migrationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "missing_id",
			"message": "缺少迁移ID参数",
		})
		return
	}
	
	tracker := api.monitor.GetMigrationTracker()
	record := tracker.GetMigrationRecord(migrationID)
	
	if record == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "record_not_found",
			"message": "未找到指定的迁移记录",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": record,
		"timestamp": time.Now(),
	})
}

// GetMigrationProgress 获取迁移进度
func (api *MonitoringAPI) GetMigrationProgress(c *gin.Context) {
	tracker := api.monitor.GetMigrationTracker()
	progress := api.visualizer.RenderMigrationProgress(tracker)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"text": progress,
			"statistics": tracker.GetStatistics(),
			"active_sessions": tracker.GetActiveSessions(),
		},
		"timestamp": time.Now(),
	})
}

// GetDiagnosisReport 获取诊断报告
func (api *MonitoringAPI) GetDiagnosisReport(c *gin.Context) {
	tracker := api.monitor.GetMigrationTracker()
	report := tracker.GenerateFailureReport()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": report,
		"timestamp": time.Now(),
	})
}

// AnalyzeMigrationIssues 分析迁移问题
func (api *MonitoringAPI) AnalyzeMigrationIssues(c *gin.Context) {
	var request struct {
		MigrationIDs []string `json:"migration_ids"`
		TimeRange    struct {
			Start time.Time `json:"start"`
			End   time.Time `json:"end"`
		} `json:"time_range,omitempty"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid_request",
			"message": err.Error(),
		})
		return
	}
	
	// 简化的分析逻辑
	tracker := api.monitor.GetMigrationTracker()
	analysis := make(map[string]interface{})
	
	for _, migrationID := range request.MigrationIDs {
		record := tracker.GetMigrationRecord(migrationID)
		if record != nil {
			analysis[migrationID] = gin.H{
				"status": record.Status,
				"error": record.Error,
				"duration": record.Duration,
				"recommendation": api.generateRecommendation(record),
			}
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": analysis,
		"timestamp": time.Now(),
	})
}

// EnableMonitoring 启用监控
func (api *MonitoringAPI) EnableMonitoring(c *gin.Context) {
	api.monitor.Enable()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "监控已启用",
		"timestamp": time.Now(),
	})
}

// DisableMonitoring 禁用监控
func (api *MonitoringAPI) DisableMonitoring(c *gin.Context) {
	api.monitor.Disable()
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "监控已禁用",
		"timestamp": time.Now(),
	})
}

// CaptureSnapshot 手动捕获快照
func (api *MonitoringAPI) CaptureSnapshot(c *gin.Context) {
	snapshot := api.monitor.CaptureSnapshot()
	if snapshot == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "capture_failed",
			"message": "快照捕获失败",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": snapshot,
		"message": "快照捕获成功",
		"timestamp": time.Now(),
	})
}

// UpdateDataDistribution 更新数据分布
func (api *MonitoringAPI) UpdateDataDistribution(c *gin.Context) {
	var request struct {
		DataKeys []string `json:"data_keys"`
	}
	
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid_request",
			"message": err.Error(),
		})
		return
	}
	
	api.monitor.UpdateDataDistribution(request.DataKeys)
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "数据分布已更新",
		"count": len(request.DataKeys),
		"timestamp": time.Now(),
	})
}

// generateRecommendation 生成修复建议
func (api *MonitoringAPI) generateRecommendation(record *MigrationRecord) string {
	if record.Status == MigrationStatusCompleted {
		return "迁移成功完成"
	}
	
	if record.Error == "" {
		return "检查网络连接和节点状态"
	}
	
	// 基于错误信息生成建议
	switch {
	case contains(record.Error, "connection refused"):
		return "目标节点不可达，检查节点状态和网络连接"
	case contains(record.Error, "timeout"):
		return "请求超时，检查网络延迟和节点负载"
	case contains(record.Error, "not found"):
		return "数据未找到，可能已被删除或迁移"
	default:
		return "检查系统日志获取详细错误信息"
	}
}

// contains 检查字符串包含
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
//监控
