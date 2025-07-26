package main

import "fmt"

func main() {
	fmt.Println("Test program starting...")
	
	// 创建哈希环
	ring := NewHashRing()
	fmt.Println("Hash ring created")
	
	// 添加一个节点
	node := &Node{ID: "test", Address: "localhost:6379", Weight: 100}
	ring.AddNode(node)
	fmt.Println("Node added")
	
	// 测试获取节点
	result := ring.GetNode("testkey")
	if result != nil {
		fmt.Printf("Got node: %s\n", result.ID)
	} else {
		fmt.Println("No node found")
	}
	
	fmt.Println("Test completed")
}
