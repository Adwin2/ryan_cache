package distributed

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// NodeServer 分布式节点服务器
// 将 DistributedNode 包装为 HTTP 服务
type NodeServer struct {
	nodeID      string
	address     string
	node        *DistributedNode // 使用新的 DistributedNode
	cluster     *ClusterManager
	handlers    *APIHandlers
	router      *gin.Engine
	server      *http.Server
}



// NewNodeServer 创建新的节点服务器
func NewNodeServer(config NodeConfig) *NodeServer {
	// 创建分布式节点实例
	node := NewDistributedNode(config)

	// 创建集群管理器
	cluster := NewClusterManager(config.NodeID, config.ClusterNodes)

	// 创建API处理器
	handlers := NewAPIHandlers(node, cluster)

	// 创建节点服务器
	server := &NodeServer{
		nodeID:   config.NodeID,
		address:  config.Address,
		node:     node,
		cluster:  cluster,
		handlers: handlers,
	}

	// 设置路由
	server.setupRoutes()

	return server
}

// setupRoutes 设置HTTP路由
func (ns *NodeServer) setupRoutes() {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)
	ns.router = gin.New()
	
	// 添加中间件
	ns.router.Use(gin.Logger())
	ns.router.Use(gin.Recovery())
	ns.router.Use(ns.corsMiddleware())
	
	// 客户端API - 对外提供服务
	clientAPI := ns.router.Group("/api/v1")
	{
		clientAPI.GET("/cache/:key", ns.handlers.HandleGet)
		clientAPI.PUT("/cache/:key", ns.handlers.HandleSet)
		clientAPI.DELETE("/cache/:key", ns.handlers.HandleDelete)
		clientAPI.GET("/stats", ns.handlers.HandleGetStats)
		clientAPI.GET("/health", ns.handlers.HandleHealthCheck)
	}
	
	// 内部API - 节点间通信
	internalAPI := ns.router.Group("/internal")
	{
		internalAPI.GET("/cache/:key", ns.handlers.HandleInternalGet)
		internalAPI.PUT("/cache/:key", ns.handlers.HandleInternalSet)
		internalAPI.DELETE("/cache/:key", ns.handlers.HandleInternalDelete)
		internalAPI.POST("/cluster/join", ns.handlers.HandleNodeJoin)
		internalAPI.POST("/cluster/leave", ns.handlers.HandleNodeLeave)
		internalAPI.POST("/cluster/sync-add", ns.handlers.HandleSyncAddNode)
		internalAPI.POST("/cluster/sync-remove", ns.handlers.HandleSyncRemoveNode)
		internalAPI.GET("/cluster/health", ns.handlers.HandleClusterHealth)
	}
	
	// 管理API - 集群管理
	adminAPI := ns.router.Group("/admin")
	{
		adminAPI.GET("/cluster", ns.handlers.HandleGetCluster)
		adminAPI.GET("/nodes", ns.handlers.HandleGetNodes)
		adminAPI.POST("/cluster/rebalance", ns.handlers.HandleRebalance)
		adminAPI.GET("/metrics", ns.handlers.HandleGetMetrics)
	}
}

// corsMiddleware CORS中间件
func (ns *NodeServer) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

// Start 启动节点服务器
func (ns *NodeServer) Start() error {
	// 创建HTTP服务器
	ns.server = &http.Server{
		Addr:    ns.address,
		Handler: ns.router,
	}
	
	// 启动集群管理器
	if err := ns.cluster.Start(); err != nil {
		return fmt.Errorf("启动集群管理器失败: %v", err)
	}
	
	log.Printf("🚀 启动分布式缓存节点: %s", ns.nodeID)
	log.Printf("📡 监听地址: %s", ns.address)
	log.Printf("🌐 集群节点数: %d", len(ns.cluster.GetNodes()))
	
	// 启动HTTP服务器
	go func() {
		if err := ns.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP服务器启动失败: %v", err)
		}
	}()
	
	// 等待关闭信号
	ns.waitForShutdown()
	
	return nil
}

// waitForShutdown 等待关闭信号
func (ns *NodeServer) waitForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	log.Printf("🛑 正在关闭节点: %s", ns.nodeID)
	
	// 优雅关闭
	ns.Shutdown()
}

// Shutdown 优雅关闭节点服务器
func (ns *NodeServer) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// 从集群中移除节点
	if err := ns.cluster.Leave(); err != nil {
		log.Printf("⚠️ 离开集群失败: %v", err)
	}
	
	// 关闭HTTP服务器
	if err := ns.server.Shutdown(ctx); err != nil {
		log.Printf("⚠️ HTTP服务器关闭失败: %v", err)
	}
	
	// 停止集群管理器
	ns.cluster.Stop()
	
	log.Printf("✅ 节点 %s 已关闭", ns.nodeID)
}

// GetNodeID 获取节点ID
func (ns *NodeServer) GetNodeID() string {
	return ns.nodeID
}

// GetAddress 获取节点地址
func (ns *NodeServer) GetAddress() string {
	return ns.address
}

// GetNode 获取分布式节点实例
func (ns *NodeServer) GetNode() *DistributedNode {
	return ns.node
}

// GetCluster 获取集群管理器
func (ns *NodeServer) GetCluster() *ClusterManager {
	return ns.cluster
}

// IsHealthy 检查节点是否健康
func (ns *NodeServer) IsHealthy() bool {
	return ns.cluster.IsHealthy() && ns.node != nil
}
