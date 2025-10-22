package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/axiaoxin-com/investool/datacenter/coze"
	"github.com/spf13/viper"
)

func main() {
	// 加载配置文件
	viper.SetConfigFile("config.toml")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}

	// 获取配置
	apiKey := viper.GetString("coze.api_key")
	botID := viper.GetString("coze.bot_id")
	enabled := viper.GetBool("coze.enabled")

	fmt.Printf("=== Coze API 测试 ===\n")
	fmt.Printf("启用状态: %v\n", enabled)
	fmt.Printf("API Key: %s\n", maskAPIKey(apiKey))
	fmt.Printf("Bot ID: %s\n", botID)

	if !enabled || apiKey == "" || botID == "" {
		log.Fatal("Coze API 未配置或未启用")
	}

	// 创建客户端
	client := coze.NewCozeClient(apiKey, botID)
	ctx := context.Background()

	// 测试1: 简单查询
	fmt.Printf("\n=== 测试1: 简单查询 ===\n")
	testSimpleQuery(client, ctx)

	// 测试2: 行业分析
	fmt.Printf("\n=== 测试2: 行业分析 ===\n")
	testIndustryAnalysis(client, ctx)
}

func testSimpleQuery(client *coze.CozeClient, ctx context.Context) {
	query := "你好，请简单介绍一下你自己"
	
	fmt.Printf("发送查询: %s\n", query)
	
	start := time.Now()
	resp, err := client.CallCozeAPI(ctx, query, nil)
	duration := time.Since(start)

	fmt.Printf("请求耗时: %v\n", duration)
	
	if err != nil {
		fmt.Printf("❌ 查询失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 查询成功!\n")
	fmt.Printf("响应码: %d\n", resp.Code)
	fmt.Printf("响应消息: %s\n", resp.Message)
	fmt.Printf("消息数量: %d\n", len(resp.Data.Messages))

	for i, message := range resp.Data.Messages {
		fmt.Printf("\n--- 消息 %d ---\n", i)
		fmt.Printf("ID: %s\n", message.ID)
		fmt.Printf("类型: %s\n", message.Type)
		fmt.Printf("角色: %s\n", message.Role)
		fmt.Printf("内容长度: %d\n", len(message.Content))
		fmt.Printf("内容: %s\n", message.Content)
		
		if message.Function != nil {
			fmt.Printf("函数名称: %s\n", message.Function.Name)
			fmt.Printf("函数参数: %v\n", message.Function.Arguments)
		}
	}
}

func testIndustryAnalysis(client *coze.CozeClient, ctx context.Context) {
	req := coze.IndustryAnalysisRequest{
		StockName:    "贵州茅台",
		Industry:     "食品饮料",
		MarketCap:    2000.0,
		MainBusiness: "白酒生产销售",
		Concept:      "白酒龙头",
	}

	fmt.Printf("发送行业分析请求: %+v\n", req)

	start := time.Now()
	analysis, err := client.GetIndustryAnalysis(ctx, req)
	duration := time.Since(start)

	fmt.Printf("请求耗时: %v\n", duration)

	if err != nil {
		fmt.Printf("❌ 行业分析失败: %v\n", err)
		return
	}

	fmt.Printf("✅ 行业分析成功!\n")
	fmt.Printf("行业前景评分: %.1f\n", analysis.IndustryProspectScore)
	fmt.Printf("行业龙头评分: %.1f\n", analysis.IndustryLeaderScore)
	fmt.Printf("新概念评分: %.1f\n", analysis.NewConceptScore)
	fmt.Printf("分析说明: %s\n", analysis.Analysis)
	fmt.Printf("数据来源: %s\n", analysis.DataSource)
}

// maskAPIKey 遮蔽 API 密钥用于安全显示
func maskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return "***"
	}
	return apiKey[:4] + "***" + apiKey[len(apiKey)-4:]
}
