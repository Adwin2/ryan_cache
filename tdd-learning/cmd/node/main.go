package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"tdd-learning/distributed"

	"gopkg.in/yaml.v3"
)

// 导入现有的缓存实现
// 注意：这里需要确保能正确导入 DistributedCache 和 LRUCache

func main() {
	// 解析命令行参数
	var configFile string
	flag.StringVar(&configFile, "config", "", "配置文件路径")
	flag.Parse()
	
	if configFile == "" {
		fmt.Println("使用方法: go run main.go -config=config/node1.yaml")
		os.Exit(1)
	}
	
	// 读取配置文件
	config, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("❌ 加载配置文件失败: %v", err)
	}
	
	// 创建节点服务器 - 内部会自动创建DistributedNode
	nodeServer := distributed.NewNodeServer(*config)
	
	// 启动节点服务器
	log.Printf("🚀 启动分布式缓存节点: %s", config.NodeID)
	log.Printf("📡 监听地址: %s", config.Address)
	log.Printf("🌐 集群节点: %v", config.ClusterNodes)
	
	if err := nodeServer.Start(); err != nil {
		log.Fatalf("❌ 启动节点失败: %v", err)
	}
}

// loadConfig 加载配置文件
func loadConfig(configFile string) (*distributed.NodeConfig, error) {
	// 获取绝对路径
	absPath, err := filepath.Abs(configFile)
	if err != nil {
		return nil, fmt.Errorf("获取配置文件绝对路径失败: %v", err)
	}
	
	// 读取配置文件
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}
	
	// 解析YAML配置
	var config distributed.NodeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}
	
	// 验证配置
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %v", err)
	}
	
	return &config, nil
}

// validateConfig 验证配置
func validateConfig(config *distributed.NodeConfig) error {
	if config.NodeID == "" {
		return fmt.Errorf("node_id 不能为空")
	}
	
	if config.Address == "" {
		return fmt.Errorf("address 不能为空")
	}
	
	if len(config.ClusterNodes) == 0 {
		return fmt.Errorf("cluster_nodes 不能为空")
	}
	
	// 检查当前节点是否在集群节点列表中
	found := false
	for nodeID := range config.ClusterNodes {
		if nodeID == config.NodeID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("当前节点 %s 不在集群节点列表中", config.NodeID)
	}
	
	// 设置默认值
	if config.CacheSize == 0 {
		config.CacheSize = 1000
	}
	
	if config.VirtualNodes == 0 {
		config.VirtualNodes = 150
	}
	
	return nil
}