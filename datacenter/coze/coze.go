// Coze API 数据源 - 用于获取缺失的评分数据

package coze

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/axiaoxin-com/logging"
	"go.uber.org/zap"
)

// CozeClient Coze API客户端
type CozeClient struct {
	HTTPClient *http.Client
	APIKey     string
	BotID      string
	BaseURL    string
}

// NewCozeClient 创建Coze客户端
func NewCozeClient(pat, botID string) *CozeClient {
	return &CozeClient{
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		APIKey:  pat, // 这里实际存储的是PAT
		BotID:   botID,
		BaseURL: "https://api.coze.com/open_api/v2/chat", // 国际版API端点
	}
}

// CozeRequest Coze API请求结构
type CozeRequest struct {
	ConversationID string         `json:"conversation_id"`
	BotID          string         `json:"bot_id"`
	User           string         `json:"user"`
	Query          string         `json:"query"`
	Stream         bool           `json:"stream"`
	Functions      []CozeFunction `json:"functions,omitempty"`
}

// CozeFunction Coze函数定义
type CozeFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// CozeResponse Coze API响应结构
type CozeResponse struct {
	Code           int           `json:"code"`
	Message        string        `json:"msg"`
	Messages       []CozeMessage `json:"messages"`
	ConversationId string        `json:"conversation_id"`
}

// CozeMessage Coze消息结构
type CozeMessage struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Role     string            `json:"role"`
	Content  string            `json:"content"`
	Function *CozeFunctionCall `json:"function,omitempty"`
}

// CozeFunctionCall Coze函数调用结构
type CozeFunctionCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// IndustryAnalysisRequest 行业分析请求
type IndustryAnalysisRequest struct {
	StockName    string  `json:"stock_name"`
	Industry     string  `json:"industry"`
	MarketCap    float64 `json:"market_cap"`
	MainBusiness string  `json:"main_business"`
	Concept      string  `json:"concept"`
}

// IndustryAnalysisResponse 行业分析响应
type IndustryAnalysisResponse struct {
	IndustryProspectScore float64 `json:"industry_prospect_score"` // 行业前景评分 0-10
	IndustryLeaderScore   float64 `json:"industry_leader_score"`   // 行业龙头评分 0-10
	NewConceptScore       float64 `json:"new_concept_score"`       // 新品新概念评分 0-10
	Analysis              string  `json:"analysis"`                // 分析说明
	DataSource            string  `json:"data_source"`             // 数据来源
}

// CallCozeAPI 调用Coze API
func (c *CozeClient) CallCozeAPI(ctx context.Context, query string, functions []CozeFunction) (*CozeResponse, error) {
	reqBody := CozeRequest{
		ConversationID: fmt.Sprintf("stock_analysis_%d", time.Now().Unix()),
		BotID:          c.BotID,
		User:           "stock_analyzer",
		Query:          query,
		Stream:         false,
		Functions:      functions,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	logging.Debug(ctx, "Coze API call begin", zap.String("query", query))
	beginTime := time.Now()

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	latency := time.Since(beginTime).Milliseconds()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("response failed: %v", err)
	}
	logging.Debug(ctx, "Coze API call end", zap.Int64("latency(ms)", latency))
	logging.Debug(ctx, "Coze API call body", zap.String("body", string(body)))

	var cozeResp CozeResponse
	if err := json.Unmarshal(body, &cozeResp); err != nil {
		return nil, fmt.Errorf("decode response failed: %v", err)
	}

	if cozeResp.Code != 0 {
		return nil, fmt.Errorf("coze api error: %s", cozeResp.Message)
	}

	return &cozeResp, nil
}

// GetIndustryAnalysis 获取行业分析评分
func (c *CozeClient) GetIndustryAnalysis(ctx context.Context, req IndustryAnalysisRequest) (*IndustryAnalysisResponse, error) {
	// 构建简化的查询prompt，直接要求返回JSON
	query := fmt.Sprintf(`
请分析以下股票的投资价值，重点关注行业前景、行业地位和新概念：

股票信息：
- 股票名称：%s
- 所属行业：%s
- 市值：%.2f亿元
- 主营业务：%s
- 所属概念：%s

请直接返回以下格式的JSON数据（不要包含其他文字）：
{
    "industry_prospect_score": 8.5,
    "industry_leader_score": 7.2,
    "new_concept_score": 6.8
}

评分规则：
1. 行业前景评分（0-10分）：基于行业成长性、政策支持、市场需求等
2. 行业龙头评分（0-10分）：基于公司在行业中的地位、市场份额、竞争优势等
3. 新品新概念评分（0-10分）：基于公司是否涉及新技术、新产品、新赛道等

请确保使用最新的数据和信息进行分析。
`, req.StockName, req.Industry, req.MarketCap, req.MainBusiness, req.Concept)

	// 不传递 functions 参数，让 Bot 直接返回 JSON
	resp, err := c.CallCozeAPI(ctx, query, nil)
	if err != nil {
		return nil, err
	}

	// 解析返回的 JSON 数据
	if len(resp.Messages) > 0 {
		for _, msg := range resp.Messages {
			if msg.Type != "answer" {
				continue
			}
			content := msg.Content
			logging.Debugf(ctx, "Coze response content: %s", content)

			// 尝试从内容中提取 JSON
			var result IndustryAnalysisResponse
			if err := json.Unmarshal([]byte(content), &result); err == nil {
				logging.Infof(ctx, "Successfully parsed Coze JSON response")
				return &result, nil
			}

			// 如果直接解析失败，尝试从文本中提取 JSON
			jsonStart := strings.Index(content, "{")
			jsonEnd := strings.LastIndex(content, "}")
			if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
				jsonStr := content[jsonStart : jsonEnd+1]
				logging.Debugf(ctx, "Extracted JSON string: %s", jsonStr)

				var result IndustryAnalysisResponse
				if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
					logging.Infof(ctx, "Successfully parsed extracted JSON response")
					return &result, nil
				} else {
					logging.Warnf(ctx, "Failed to parse extracted JSON: %v", err)
				}
			}
			logging.Warnf(ctx, "Failed to parse JSON from content: %s", content)
		}

	}

	// 如果没有找到有效的JSON数据，记录调试信息并返回默认值
	logging.Warnf(ctx, "No valid JSON found in Coze response. Messages count: %d", len(resp.Messages))
	for i, message := range resp.Messages {
		logging.Debugf(ctx, "Message %d: Type=%s, Role=%s, Content=%s", i, message.Type, message.Role, message.Content)
	}

	// 返回默认分析结果而不是错误
	return &IndustryAnalysisResponse{
		IndustryProspectScore: 5.0, // 默认中等评分
		IndustryLeaderScore:   5.0,
		NewConceptScore:       5.0,
		Analysis:              "Coze AI 分析暂时不可用，使用默认评分",
		DataSource:            "系统默认值",
	}, nil
}

// HistoricalPERequest 历史PE分析请求
type HistoricalPERequest struct {
	StockName string  `json:"stock_name"`
	SecuCode  string  `json:"secu_code"`
	Industry  string  `json:"industry"`
	MarketCap float64 `json:"market_cap"`
}

// HistoricalPEResponse 历史PE分析响应
type HistoricalPEResponse struct {
	HistoricalPEList []HistoricalPEItem `json:"historical_pe_list"` // 历史PE数据
	Analysis         string             `json:"analysis"`           // 分析说明
	DataSource       string             `json:"data_source"`        // 数据来源
}

// HistoricalPEItem 历史PE单项数据
type HistoricalPEItem struct {
	Date  string  `json:"date"`  // 日期
	Value float64 `json:"value"` // PE值
}

// GetHistoricalPEAnalysis 获取历史PE分析
func (c *CozeClient) GetHistoricalPEAnalysis(ctx context.Context, req HistoricalPERequest) (*HistoricalPEResponse, error) {
	// 构建简化的查询prompt，直接要求返回JSON
	query := fmt.Sprintf(`
请分析以下股票的历史市盈率（PE）数据：

股票信息：
- 股票名称：%s
- 股票代码：%s
- 所属行业：%s
- 市值：%.2f亿元

请直接返回以下格式的JSON数据（不要包含其他文字）：
{
    "historical_pe_list": [
        {"date": "2024-01-01", "value": 15.2},
        {"date": "2023-12-01", "value": 14.8},
        {"date": "2023-11-01", "value": 16.1}
    ],
    "analysis": "详细分析说明...",
    "data_source": "基于2024年最新市场数据"
}

要求：
1. 提供过去3-5年的历史PE数据（按时间倒序）
2. 分析PE的变化趋势
3. 与同行业PE进行对比
4. 评估当前PE的合理性

请确保使用最新的数据和信息进行分析。
`, req.StockName, req.SecuCode, req.Industry, req.MarketCap)

	// 不传递 functions 参数，让 Bot 直接返回 JSON
	resp, err := c.CallCozeAPI(ctx, query, nil)
	if err != nil {
		return nil, err
	}

	// 解析返回的 JSON 数据
	if len(resp.Messages) > 0 {
		content := resp.Messages[0].Content
		logging.Debugf(ctx, "Coze PE response content: %s", content)

		// 尝试从内容中提取 JSON
		var result HistoricalPEResponse
		if err := json.Unmarshal([]byte(content), &result); err == nil {
			logging.Infof(ctx, "Successfully parsed Coze PE JSON response")
			return &result, nil
		}

		// 如果直接解析失败，尝试从文本中提取 JSON
		jsonStart := strings.Index(content, "{")
		jsonEnd := strings.LastIndex(content, "}")
		if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
			jsonStr := content[jsonStart : jsonEnd+1]
			logging.Debugf(ctx, "Extracted PE JSON string: %s", jsonStr)

			var result HistoricalPEResponse
			if err := json.Unmarshal([]byte(jsonStr), &result); err == nil {
				logging.Infof(ctx, "Successfully parsed extracted PE JSON response")
				return &result, nil
			} else {
				logging.Warnf(ctx, "Failed to parse extracted PE JSON: %v", err)
			}
		}

		logging.Warnf(ctx, "Failed to parse PE JSON from content: %s", content)
	}

	// 如果没有找到有效的JSON数据，记录调试信息并返回默认值
	logging.Warnf(ctx, "No valid JSON found in Coze PE response. Messages count: %d", len(resp.Messages))
	for i, message := range resp.Messages {
		logging.Debugf(ctx, "PE Message %d: Type=%s, Role=%s, Content=%s", i, message.Type, message.Role, message.Content)
	}

	// 返回默认历史PE分析结果而不是错误
	return &HistoricalPEResponse{
		HistoricalPEList: []HistoricalPEItem{}, // 空的历史PE列表
		Analysis:         "Coze AI 历史PE分析暂时不可用",
		DataSource:       "系统默认值",
	}, nil
}
