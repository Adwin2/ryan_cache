package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

// InterviewQuestion 面试题结构
type InterviewQuestion struct {
	ID          int
	Question    string
	Difficulty  string
	Category    string
	Keywords    []string
	Answer      string
	Tips        []string
}

// InterviewSimulator 面试模拟器
type InterviewSimulator struct {
	questions []InterviewQuestion
	score     int
	total     int
}

// NewInterviewSimulator 创建面试模拟器
func NewInterviewSimulator() *InterviewSimulator {
	return &InterviewSimulator{
		questions: initQuestions(),
		score:     0,
		total:     0,
	}
}

// initQuestions 初始化题库
func initQuestions() []InterviewQuestion {
	return []InterviewQuestion{
		{
			ID:         1,
			Question:   "什么是缓存？为什么要使用缓存？",
			Difficulty: "⭐",
			Category:   "基础概念",
			Keywords:   []string{"缓存", "性能", "延迟", "局部性"},
			Answer: `缓存是一种存储技术，将频繁访问的数据存储在访问速度更快的存储介质中。
使用缓存的原因：
1. 提升性能：内存访问比磁盘快1000倍以上
2. 减少延迟：避免重复的复杂计算或网络请求
3. 降低负载：减轻数据库和后端服务的压力
4. 提高并发：缓存可以处理更多并发请求
5. 节约成本：减少昂贵资源的使用`,
			Tips: []string{
				"提及局部性原理（时间局部性和空间局部性）",
				"举例说明缓存在不同层级的应用",
				"可以对比CPU缓存、浏览器缓存、CDN等",
			},
		},
		{
			ID:         2,
			Question:   "请解释缓存雪崩、穿透、击穿的区别和解决方案",
			Difficulty: "⭐⭐⭐⭐⭐",
			Category:   "缓存问题",
			Keywords:   []string{"雪崩", "穿透", "击穿", "布隆过滤器", "互斥锁"},
			Answer: `缓存雪崩：大量缓存同时失效，解决方案包括随机TTL、多级缓存、熔断降级
缓存穿透：查询不存在的数据，解决方案包括布隆过滤器、缓存空值、参数校验
缓存击穿：热点数据失效导致并发重建，解决方案包括互斥锁、永不过期、异步更新`,
			Tips: []string{
				"用具体场景举例说明每种问题",
				"详细解释布隆过滤器的原理",
				"说明不同解决方案的适用场景",
			},
		},
		{
			ID:         3,
			Question:   "如何设计一个LRU缓存？",
			Difficulty: "⭐⭐⭐",
			Category:   "设计实现",
			Keywords:   []string{"LRU", "双向链表", "哈希表", "时间复杂度"},
			Answer: `LRU缓存设计：
1. 使用双向链表 + 哈希表
2. 哈希表提供O(1)查找
3. 双向链表维护访问顺序
4. GET操作：如果存在，移动到链表头部
5. PUT操作：如果满了，删除链表尾部，新节点插入头部`,
			Tips: []string{
				"分析时间复杂度为O(1)",
				"考虑线程安全问题",
				"对比其他淘汰策略（LFU、FIFO等）",
			},
		},
		{
			ID:         4,
			Question:   "Redis和Memcached的区别是什么？",
			Difficulty: "⭐⭐⭐",
			Category:   "技术选型",
			Keywords:   []string{"Redis", "Memcached", "数据类型", "持久化", "分布式"},
			Answer: `主要区别：
1. 数据类型：Redis支持多种数据类型，Memcached只支持String
2. 持久化：Redis支持RDB+AOF，Memcached不支持
3. 分布式：Redis有原生集群，Memcached依赖客户端分片
4. 性能：Memcached单纯缓存性能更高
5. 功能：Redis功能更丰富（发布订阅、Lua脚本等）`,
			Tips: []string{
				"结合具体业务场景分析选择",
				"提及性能测试数据",
				"考虑运维成本和团队技能",
			},
		},
		{
			ID:         5,
			Question:   "如何保证缓存和数据库的数据一致性？",
			Difficulty: "⭐⭐⭐⭐",
			Category:   "数据一致性",
			Keywords:   []string{"一致性", "延迟双删", "消息队列", "版本号"},
			Answer: `主要方案：
1. Cache-Aside + 延迟双删
2. 消息队列异步更新
3. 数据库变更监听（Binlog）
4. 版本号机制
选择方案需要在一致性和性能之间权衡`,
			Tips: []string{
				"分析不同方案的优缺点",
				"结合CAP理论解释权衡",
				"提及监控和运维策略",
			},
		},
		{
			ID:         6,
			Question:   "如何设计一个分布式缓存系统？",
			Difficulty: "⭐⭐⭐⭐⭐",
			Category:   "架构设计",
			Keywords:   []string{"分布式", "一致性哈希", "副本", "故障处理"},
			Answer: `核心组件：
1. 客户端：路由、负载均衡
2. 缓存节点：数据存储、副本
3. 配置中心：节点管理、路由表
4. 监控系统：指标收集、告警
关键设计：数据分片、副本策略、故障处理、一致性保证`,
			Tips: []string{
				"画出架构图",
				"分析CAP理论的权衡",
				"考虑扩容和缩容策略",
			},
		},
		{
			ID:         7,
			Question:   "什么是布隆过滤器？有什么局限性？",
			Difficulty: "⭐⭐⭐⭐",
			Category:   "算法原理",
			Keywords:   []string{"布隆过滤器", "位数组", "哈希函数", "误判率"},
			Answer: `布隆过滤器原理：
1. 使用位数组和多个哈希函数
2. 添加元素时，将对应位置设为1
3. 查询时，检查所有对应位是否为1
局限性：可能误判存在、不能删除元素、容量固定`,
			Tips: []string{
				"计算误判率公式",
				"提及Counting Bloom Filter等变种",
				"说明在缓存穿透中的应用",
			},
		},
		{
			ID:         8,
			Question:   "如何优化缓存性能？",
			Difficulty: "⭐⭐⭐⭐",
			Category:   "性能优化",
			Keywords:   []string{"批量操作", "连接池", "压缩", "本地缓存"},
			Answer: `优化策略：
1. 读取优化：批量操作、Pipeline、连接池、本地缓存
2. 写入优化：异步写入、批量写入、写入合并
3. 内存优化：数据压缩、过期清理、内存监控
4. 网络优化：连接复用、数据压缩`,
			Tips: []string{
				"提供具体的性能数据",
				"说明不同优化策略的适用场景",
				"考虑优化的副作用",
			},
		},
	}
}

// StartInterview 开始面试
func (is *InterviewSimulator) StartInterview() {
	fmt.Println("🎯 欢迎来到缓存技术面试模拟器！")
	fmt.Println("==========================================")
	fmt.Println("📋 面试规则：")
	fmt.Println("   1. 我会随机提问缓存相关问题")
	fmt.Println("   2. 请尽可能详细地回答")
	fmt.Println("   3. 输入 'hint' 获取提示")
	fmt.Println("   4. 输入 'answer' 查看标准答案")
	fmt.Println("   5. 输入 'next' 进入下一题")
	fmt.Println("   6. 输入 'quit' 结束面试")
	fmt.Println("==========================================")
	
	reader := bufio.NewReader(os.Stdin)
	
	for {
		// 随机选择一道题
		question := is.getRandomQuestion()
		is.total++
		
		fmt.Printf("\n📝 第%d题 [%s] [%s]\n", is.total, question.Difficulty, question.Category)
		fmt.Printf("❓ %s\n", question.Question)
		fmt.Print("\n💭 请输入您的答案: ")
		
		for {
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			
			switch strings.ToLower(input) {
			case "hint":
				is.showHints(question)
				fmt.Print("\n💭 请继续回答: ")
			case "answer":
				is.showAnswer(question)
				fmt.Print("\n💭 请继续回答或输入'next': ")
			case "next":
				is.evaluateAnswer(question, "")
				goto nextQuestion
			case "quit":
				is.showFinalScore()
				return
			default:
				if len(input) > 10 { // 认为是实际回答
					is.evaluateAnswer(question, input)
					goto nextQuestion
				} else {
					fmt.Print("💭 请输入您的答案 (或 hint/answer/next/quit): ")
				}
			}
		}
		
		nextQuestion:
		
		if is.total >= 5 { // 限制题目数量
			fmt.Println("\n🎉 面试结束！")
			is.showFinalScore()
			break
		}
	}
}

// getRandomQuestion 获取随机题目
func (is *InterviewSimulator) getRandomQuestion() InterviewQuestion {
	return is.questions[rand.Intn(len(is.questions))]
}

// showHints 显示提示
func (is *InterviewSimulator) showHints(question InterviewQuestion) {
	fmt.Println("\n💡 提示：")
	for i, tip := range question.Tips {
		fmt.Printf("   %d. %s\n", i+1, tip)
	}
	fmt.Printf("\n🔑 关键词: %s\n", strings.Join(question.Keywords, ", "))
}

// showAnswer 显示标准答案
func (is *InterviewSimulator) showAnswer(question InterviewQuestion) {
	fmt.Println("\n📖 标准答案：")
	fmt.Println(question.Answer)
}

// evaluateAnswer 评估答案
func (is *InterviewSimulator) evaluateAnswer(question InterviewQuestion, userAnswer string) {
	if userAnswer == "" {
		fmt.Println("\n⏭️ 跳过此题")
		return
	}
	
	// 简单的关键词匹配评分
	score := 0
	userAnswerLower := strings.ToLower(userAnswer)
	
	for _, keyword := range question.Keywords {
		if strings.Contains(userAnswerLower, strings.ToLower(keyword)) {
			score++
		}
	}
	
	percentage := float64(score) / float64(len(question.Keywords)) * 100
	
	if percentage >= 60 {
		fmt.Printf("\n✅ 回答不错！覆盖了%.0f%%的关键点\n", percentage)
		is.score++
	} else {
		fmt.Printf("\n⚠️ 回答需要改进，只覆盖了%.0f%%的关键点\n", percentage)
	}
	
	// 显示标准答案供参考
	fmt.Println("\n📖 参考答案：")
	fmt.Println(question.Answer)
}

// showFinalScore 显示最终得分
func (is *InterviewSimulator) showFinalScore() {
	fmt.Println("\n🎯 面试结果")
	fmt.Println("==========================================")
	fmt.Printf("📊 总题数: %d\n", is.total)
	fmt.Printf("✅ 答对: %d\n", is.score)
	fmt.Printf("❌ 答错: %d\n", is.total-is.score)
	
	percentage := float64(is.score) / float64(is.total) * 100
	fmt.Printf("📈 得分率: %.1f%%\n", percentage)
	
	// 评级
	var rating string
	var advice string
	
	switch {
	case percentage >= 80:
		rating = "🏆 优秀"
		advice = "恭喜！您对缓存技术掌握得很好，可以自信地参加面试了！"
	case percentage >= 60:
		rating = "👍 良好"
		advice = "不错！建议再复习一下薄弱环节，特别关注缓存问题的解决方案。"
	case percentage >= 40:
		rating = "📚 需要提高"
		advice = "需要加强学习，建议重点复习缓存基础概念和常见问题。"
	default:
		rating = "💪 继续努力"
		advice = "基础还需要加强，建议系统学习缓存相关知识后再来挑战。"
	}
	
	fmt.Printf("🎖️ 评级: %s\n", rating)
	fmt.Printf("💡 建议: %s\n", advice)
	
	fmt.Println("\n📚 推荐学习资源：")
	fmt.Println("   1. 重新学习前面章节的内容")
	fmt.Println("   2. 阅读Redis官方文档")
	fmt.Println("   3. 实践项目中的缓存设计")
	fmt.Println("   4. 关注大厂技术博客")
}

// DemoInterviewQuestions 演示面试题库
func DemoInterviewQuestions() {
	fmt.Println("=== 面试题库演示 ===")
	
	simulator := NewInterviewSimulator()
	
	fmt.Println("\n📚 题库统计：")
	
	// 按难度分类统计
	difficultyCount := make(map[string]int)
	categoryCount := make(map[string]int)
	
	for _, q := range simulator.questions {
		difficultyCount[q.Difficulty]++
		categoryCount[q.Category]++
	}
	
	fmt.Println("\n📊 按难度分布：")
	for difficulty, count := range difficultyCount {
		fmt.Printf("   %s: %d题\n", difficulty, count)
	}
	
	fmt.Println("\n📊 按类别分布：")
	for category, count := range categoryCount {
		fmt.Printf("   %s: %d题\n", category, count)
	}
	
	fmt.Printf("\n📝 总题数: %d\n", len(simulator.questions))
	
	// 展示几道示例题目
	fmt.Println("\n🔍 示例题目：")
	for i, q := range simulator.questions[:3] {
		fmt.Printf("\n%d. [%s] %s\n", i+1, q.Difficulty, q.Question)
		fmt.Printf("   类别: %s\n", q.Category)
		fmt.Printf("   关键词: %s\n", strings.Join(q.Keywords, ", "))
	}
}

func main() {
	fmt.Println("🎮 第六章：面试题集 - 模拟面试程序")
	fmt.Println("==========================================")
	
	// 设置随机种子
	rand.Seed(time.Now().UnixNano())
	
	// 演示题库
	DemoInterviewQuestions()
	
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Print("\n🚀 是否开始模拟面试？(y/n): ")
	
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	
	if input == "y" || input == "yes" {
		simulator := NewInterviewSimulator()
		simulator.StartInterview()
	} else {
		fmt.Println("\n📖 您可以先复习相关资料，准备好后再来挑战！")
		fmt.Println("\n💡 学习建议：")
		fmt.Println("   1. 熟练掌握缓存基础概念")
		fmt.Println("   2. 理解缓存问题的解决方案")
		fmt.Println("   3. 练习系统设计题")
		fmt.Println("   4. 结合实际项目经验")
	}
	
	fmt.Println("\n🎯 面试成功秘诀：")
	fmt.Println("   1. 结构化回答问题")
	fmt.Println("   2. 用具体例子说明")
	fmt.Println("   3. 主动扩展相关知识点")
	fmt.Println("   4. 诚实承认不足")
	
	fmt.Println("\n🚀 祝您面试成功！")
}
