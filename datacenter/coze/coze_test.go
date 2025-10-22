package coze

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// TestCozeAPICall 测试 Coze API 调用
func TestCozeAPICall(t *testing.T) {
	// 从配置文件读取 API 密钥
	apiKey := ""
	botID := "7563990779418066951"
	enabled := true

	fmt.Printf("Coze配置检查:\n")
	fmt.Printf("- 启用状态: %v\n", enabled)
	fmt.Printf("- API Key: %s\n", maskAPIKey(apiKey))
	fmt.Printf("- Bot ID: %s\n", botID)

	// 创建客户端
	client := NewCozeClient(apiKey, botID)
	ctx := context.Background()

	// 测试简单的文本查询
	//fmt.Printf("\n=== 测试简单文本查询 ===\n")
	//testSimpleQuery(t, client, ctx)

	// 测试行业分析
	fmt.Printf("\n=== 测试行业分析 ===\n")
	testIndustryAnalysis(t, client, ctx)
}

// testSimpleQuery 测试简单的文本查询
func testSimpleQuery(t *testing.T, client *CozeClient, ctx context.Context) {
	query := "请简单介绍一下贵州茅台这家公司"

	fmt.Printf("发送查询: %s\n", query)

	resp, err := client.CallCozeAPI(ctx, query, nil)
	if err != nil {
		t.Errorf("简单查询失败: %v", err)
		return
	}

	fmt.Printf("查询成功!\n")
	fmt.Printf("响应消息数量: %d\n", len(resp.Messages))

	for i, message := range resp.Messages {
		fmt.Printf("消息 %d:\n", i)
		fmt.Printf("  类型: %s\n", message.Type)
		fmt.Printf("  角色: %s\n", message.Role)
		fmt.Printf("  内容: %s\n", message.Content)
		if message.Function != nil {
			fmt.Printf("  函数调用: %s\n", message.Function.Name)
		}
		fmt.Printf("\n")
	}
}

// testIndustryAnalysis 测试行业分析
func testIndustryAnalysis(t *testing.T, client *CozeClient, ctx context.Context) {
	req := IndustryAnalysisRequest{
		StockName:    "贵州茅台",
		Industry:     "食品饮料",
		MarketCap:    2000.0,
		MainBusiness: "白酒生产销售",
		Concept:      "白酒龙头",
	}

	fmt.Printf("发送行业分析请求: %+v\n", req)

	analysis, err := client.GetIndustryAnalysis(ctx, req)
	if err != nil {
		t.Errorf("行业分析失败: %v", err)
		return
	}

	fmt.Printf("行业分析成功!\n")
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

// TestCozeAPIDebug 调试测试，显示详细的请求和响应信息
func TestCozeAPIDebug(t *testing.T) {
	apiKey := viper.GetString("coze.api_key")
	botID := viper.GetString("coze.bot_id")
	enabled := viper.GetBool("coze.enabled")

	if !enabled || apiKey == "" || botID == "" {
		t.Skip("Coze API 未配置或未启用")
	}

	client := NewCozeClient(apiKey, botID)
	ctx := context.Background()

	// 测试最简单的查询
	query := "你好"
	fmt.Printf("=== 调试测试 ===\n")
	fmt.Printf("发送查询: %s\n", query)
	fmt.Printf("Bot ID: %s\n", botID)
	fmt.Printf("API Key: %s\n", maskAPIKey(apiKey))

	start := time.Now()
	resp, err := client.CallCozeAPI(ctx, query, nil)
	duration := time.Since(start)

	fmt.Printf("请求耗时: %v\n", duration)

	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return
	}

	fmt.Printf("请求成功!\n")
	fmt.Printf("响应码: %d\n", resp.Code)
	fmt.Printf("响应消息: %s\n", resp.Message)
	fmt.Printf("消息数量: %d\n", len(resp.Messages))

	for i, message := range resp.Messages {
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
