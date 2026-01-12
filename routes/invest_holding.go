// 你的新页面

package routes

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/axiaoxin-com/goutils"
	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/datacenter"
	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/investool/version"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// InvestHoldingHandler 持仓分析工具
func InvestHoldingHandler(c *gin.Context) {
	data := gin.H{
		"Env":       viper.GetString("env"),
		"HostURL":   viper.GetString("server.host_url"),
		"Version":   version.Version,
		"PageTitle": "InvesTool | 持仓分析工具",
		"Error":     "",
		// 在这里添加你需要的其他数据
	}
	c.HTML(http.StatusOK, "invest_holding.html", data)
}

// StockAnalyzerHandler 股票分析计算器
func StockAnalyzerHandler(c *gin.Context) {
	data := gin.H{
		"Env":       viper.GetString("env"),
		"HostURL":   viper.GetString("server.host_url"),
		"Version":   version.Version,
		"PageTitle": "InvesTool | 股票分析计算器",
		"Error":     "",
	}
	c.HTML(http.StatusOK, "stock_analyzer.html", data)
}

// QueryStockDataHandler 查询股票数据API
// 只返回基础财务信息，不计算评分，生成LLM prompt供用户复制
func QueryStockDataHandler(c *gin.Context) {
	data := gin.H{
		"HostURL":   viper.GetString("server.host_url"),
		"Env":       viper.GetString("env"),
		"Version":   version.Version,
		"PageTitle": "InvesTool | 股票数据查询",
		"Error":     "",
		"StockData": nil,
	}

	keyword := c.Query("keyword")
	if keyword == "" {
		data["Error"] = "请输入股票名称或代码"
		c.JSON(http.StatusOK, data)
		return
	}

	// 使用现有的搜索功能
	searcher := core.NewSearcher(c)
	stocks, err := searcher.SearchStocks(c, []string{keyword})
	if err != nil {
		data["Error"] = "查询失败: " + err.Error()
		c.JSON(http.StatusOK, data)
		return
	}

	if len(stocks) == 0 {
		data["Error"] = "未找到相关股票数据"
		c.JSON(http.StatusOK, data)
		return
	}

	// 获取第一个匹配的股票数据
	var stockData gin.H
	for _, stock := range stocks {
		stockData = gin.H{
			"stock":         stock,
			"name":          stock.BaseInfo.SecurityNameAbbr,
			"code":          stock.BaseInfo.Secucode,
			"pe":            stock.BaseInfo.PE,
			"growth":        stock.BaseInfo.NetprofitYoyRatio,
			"industry":      stock.BaseInfo.Industry,
			"market_cap":    stock.BaseInfo.TotalMarketCap,
			"current_price": stock.GetPrice(),
			"llm_prompt":    generateLLMPrompt(stock),
		}
		break
	}

	data["StockData"] = stockData
	c.JSON(http.StatusOK, data)
}

// generateLLMPrompt 生成LLM查询prompt
func generateLLMPrompt(stock models.Stock) string {
	prompt := `请搜索最新的信息分析以下股票信息，并返回JSON格式的分析结果：

股票名称：` + stock.BaseInfo.SecurityNameAbbr + `
股票代码：` + stock.BaseInfo.Secucode + `
所属行业：` + stock.BaseInfo.Industry + `
当前价格：` + fmt.Sprintf("%.2f", stock.BaseInfo.NewPrice) + `
市盈率(PE)：` + fmt.Sprintf("%.2f", stock.BaseInfo.PE) + `
净利润增长率：` + fmt.Sprintf("%.2f%%", stock.BaseInfo.NetprofitYoyRatio) + `
总市值：` + fmt.Sprintf("%.2f亿元", stock.BaseInfo.TotalMarketCap/100000000) + `

请从以下维度进行分析，并严格按照JSON格式返回结果,在json数据的下面并给出分析和数据来源：

1. 行业龙头评分 (industry_leader_score): 0-100分，评估该公司在所属行业中的地位和竞争力
2. 新概念评分 (new_concept_score): 0-100分，评估该公司是否有新的产品，新兴概念、技术或商业模式，新的领导层
3. 行业前景评分 (industry_prospect_score): 0-100分，评估该公司所属行业的成长性、政策支持、市场需求等
4. 护城河评分 (moat_score): 0-100分，评估该公司的竞争优势，包括品牌价值、专利技术、市场壁垒、客户粘性等
5. 管理层评分 (management_score): 0-100分，评估管理层的诚信度、分红政策、股份回购、历史表现等
6. 回购评分 (repurchase_score): 0-100分，评估该公司的股份回购历史、回购金额、回购时机等
7. 机构增持评分 (institution_score): 0-100分，评估机构持仓变化、机构调研频次、机构类型等
8. 技术趋势评分 (technical_trend_score): 0-100分，评估股价技术面，包括成交量、技术指标、突破情况等

请严格按照以下JSON格式返回：
{
  "industry_leader_score": 85,
  "new_concept_score": 70,
  "industry_prospect_score": 80,
  "moat_score": 75,
  "management_score": 80,
  "repurchase_score": 60,
  "institution_score": 70,
  "technical_trend_score": 65
}`

	return prompt
}

// CalculateStockScoreHandler 根据用户提交的LLM结果和财务信息计算股票评分
func CalculateStockScoreHandler(c *gin.Context) {
	data := gin.H{
		"HostURL":   viper.GetString("server.host_url"),
		"Env":       viper.GetString("env"),
		"Version":   version.Version,
		"PageTitle": "InvesTool | 股票评分计算",
		"Error":     "",
		"Results":   nil,
	}

	var req struct {
		StockName   string `json:"stock_name" binding:"required"`
		LLMResponse string `json:"llm_response" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		data["Error"] = "参数错误: " + err.Error()
		c.JSON(http.StatusOK, data)
		return
	}

	// 解析LLM返回的JSON
	var llmAnalysis struct {
		IndustryLeaderScore   float64 `json:"industry_leader_score"`
		NewConceptScore       float64 `json:"new_concept_score"`
		IndustryProspectScore float64 `json:"industry_prospect_score"`
		MoatScore             float64 `json:"moat_score"`
		ManagementScore       float64 `json:"management_score"`
		RepurchaseScore       float64 `json:"repurchase_score"`
		InstitutionScore      float64 `json:"institution_score"`
		TechnicalTrendScore   float64 `json:"technical_trend_score"`
		Analysis              string  `json:"analysis"`
		DataSource            string  `json:"data_source"`
	}

	if err := json.Unmarshal([]byte(req.LLMResponse), &llmAnalysis); err != nil {
		data["Error"] = "LLM返回结果格式错误，请确保返回的是有效的JSON格式: " + err.Error()
		c.JSON(http.StatusOK, data)
		return
	}

	// 查询股票基础信息
	searcher := core.NewSearcher(c)
	stocks, err := searcher.SearchStocks(c, []string{req.StockName})
	if err != nil || len(stocks) == 0 {
		data["Error"] = "查询股票基础信息失败"
		c.JSON(http.StatusOK, data)
		return
	}

	// 获取第一个股票数据
	var stock models.Stock
	for _, s := range stocks {
		stock = s
		break
	}

	// 计算三大评分体系，传入LLM分析结果
	llmAnalysisModel := models.LLMAnalysis{
		IndustryLeaderScore:   llmAnalysis.IndustryLeaderScore,
		NewConceptScore:       llmAnalysis.NewConceptScore,
		IndustryProspectScore: llmAnalysis.IndustryProspectScore,
		MoatScore:             llmAnalysis.MoatScore,
		ManagementScore:       llmAnalysis.ManagementScore,
		RepurchaseScore:       llmAnalysis.RepurchaseScore,
		InstitutionScore:      llmAnalysis.InstitutionScore,
		TechnicalTrendScore:   llmAnalysis.TechnicalTrendScore,
		Analysis:              llmAnalysis.Analysis,
		DataSource:            llmAnalysis.DataSource,
	}
	buffettScore := stock.CalculateBuffettScoreWithLLM(llmAnalysisModel)
	lynchScore := stock.CalculateLynchScoreWithLLM(llmAnalysisModel)
	oneilScore := stock.CalculateONeilScoreWithLLM(llmAnalysisModel)

	// 构建返回结果
	results := gin.H{
		"stock_info": gin.H{
			"name":          stock.BaseInfo.SecurityNameAbbr,
			"code":          stock.BaseInfo.Secucode,
			"pe":            stock.BaseInfo.PE,
			"growth":        stock.BaseInfo.NetprofitYoyRatio,
			"industry":      stock.BaseInfo.Industry,
			"market_cap":    stock.BaseInfo.TotalMarketCap,
			"current_price": stock.BaseInfo.NewPrice,
		},
		"buffett_score": gin.H{
			"total_score":         buffettScore.TotalScore,
			"roe_score":           buffettScore.ROEScore,
			"cash_flow_score":     buffettScore.CashFlowScore,
			"profit_growth_score": buffettScore.ProfitGrowthScore,
			"debt_ratio_score":    buffettScore.DebtRatioScore,
			"moat_score":          buffettScore.MoatScore,
			"management_score":    buffettScore.ManagementScore,
			"valuation_score":     buffettScore.ValuationScore,
			"rd_score":            buffettScore.RDScore,
			"dividend_score":      buffettScore.DividendScore,
			"repurchase_score":    buffettScore.RepurchaseScore,
			"score_description":   buffettScore.ScoreDescription,
		},
		"lynch_score": gin.H{
			"total_score":          lynchScore.TotalScore,
			"peg_score":            lynchScore.PEGScore,
			"eps_growth_score":     lynchScore.EPSGrowthScore,
			"revenue_growth_score": lynchScore.RevenueGrowthScore,
			"profit_growth_score":  lynchScore.ProfitGrowthScore,
			"roe_score":            lynchScore.ROEScore,
			"free_cash_flow_score": lynchScore.FreeCashFlowScore,
			"industry_score":       lynchScore.IndustryScore,
			"market_cap_score":     lynchScore.MarketCapScore,
			"score_description":    lynchScore.ScoreDescription,
		},
		"oneil_score": gin.H{
			"total_score":           oneilScore.TotalScore,
			"current_quarter_score": oneilScore.CurrentQuarterScore,
			"annual_growth_score":   oneilScore.AnnualGrowthScore,
			"new_concept_score":     oneilScore.NewConceptScore,
			"small_float_score":     oneilScore.SmallFloatScore,
			"leader_score":          oneilScore.LeaderScore,
			"institution_score":     oneilScore.InstitutionScore,
			"market_trend_score":    oneilScore.MarketTrendScore,
			"score_description":     oneilScore.ScoreDescription,
		},
		"llm_analysis": gin.H{
			"industry_leader_score":   llmAnalysis.IndustryLeaderScore,
			"new_concept_score":       llmAnalysis.NewConceptScore,
			"industry_prospect_score": llmAnalysis.IndustryProspectScore,
			"moat_score":              llmAnalysis.MoatScore,
			"management_score":        llmAnalysis.ManagementScore,
			"repurchase_score":        llmAnalysis.RepurchaseScore,
			"institution_score":       llmAnalysis.InstitutionScore,
			"technical_trend_score":   llmAnalysis.TechnicalTrendScore,
			"analysis":                llmAnalysis.Analysis,
			"data_source":             llmAnalysis.DataSource,
		},
	}

	data["Results"] = results
	c.JSON(http.StatusOK, data)
}

// getScoreLevel 根据分数返回等级
func getScoreLevel(score float64) string {
	if score >= 80 {
		return "优秀"
	} else if score >= 60 {
		return "良好"
	} else if score >= 40 {
		return "一般"
	} else {
		return "较差"
	}
}

// PositionDeviationHandler 计算仓位偏离度API
func PositionDeviationHandler(c *gin.Context) {
	data := gin.H{
		"HostURL":   viper.GetString("server.host_url"),
		"Env":       viper.GetString("env"),
		"Version":   version.Version,
		"PageTitle": "InvesTool | 仓位偏离度分析",
		"Error":     "",
		"Results":   nil,
	}

	var req struct {
		Holdings []struct {
			StockName string `json:"stock_name" binding:"required"`
			Shares    int    `json:"shares" binding:"required"`
			Expect    int    `json:"expect"`
			Tech      int    `json:"tech"`
		} `json:"holdings" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		data["Error"] = "参数错误: " + err.Error()
		c.JSON(http.StatusOK, data)
		return
	}

	if len(req.Holdings) == 0 {
		data["Error"] = "持仓列表不能为空"
		c.JSON(http.StatusOK, data)
		return
	}

	var results []gin.H
	totalCurrentPosition := 0.0
	totalTargetPosition := 0.0

	// 使用现有的搜索功能获取股票数据
	searcher := core.NewSearcher(c)

	for _, holding := range req.Holdings {
		// 查询股票数据
		stocksMap, err := searcher.SearchStocks(c, []string{holding.StockName})
		if err != nil || len(stocksMap) == 0 {
			// 如果查询失败，使用默认值
			result := gin.H{
				"stock_name":        holding.StockName,
				"shares":            holding.Shares,
				"current_price":     0,
				"current_amount":    0,
				"target_amount":     0,
				"amount_diff":       0,
				"deviation_percent": 0,
				"deviation_level":   "unknown",
				"error":             "查询股票数据失败",
			}
			results = append(results, result)
			continue
		}

		// 获取第一个股票数据
		var stock models.Stock
		for _, s := range stocksMap {
			stock = s
			break
		}
		currentPrice := 0.0
		if price, ok := stock.BaseInfo.NewPrice.(float64); ok {
			currentPrice = price
		}
		currentAmount := (float64(holding.Shares) * currentPrice) / 10000 // 转换为万元

		// 计算目标仓位（使用前端传递的市场预期值和技术面评分）
		expect := holding.Expect
		tech := holding.Tech
		if expect == 0 {
			expect = 3 // 默认中性
		}
		if tech == 0 {
			tech = 2 // 默认中性
		}
		targetAmount := calculateTargetPosition(stock, expect, tech)

		amountDiff := targetAmount - currentAmount
		deviationPercent := 0.0
		if targetAmount > 0 {
			deviationPercent = math.Abs((amountDiff / targetAmount) * 100)
		}

		deviationLevel := "low"
		if deviationPercent > 30 {
			deviationLevel = "high"
		} else if deviationPercent > 15 {
			deviationLevel = "medium"
		}

		result := gin.H{
			"stock_name":        holding.StockName,
			"shares":            holding.Shares,
			"current_price":     currentPrice,
			"current_amount":    currentAmount,
			"target_amount":     targetAmount,
			"amount_diff":       amountDiff,
			"deviation_percent": deviationPercent,
			"deviation_level":   deviationLevel,
			"pe":                stock.BaseInfo.PE,
			"growth":            stock.BaseInfo.NetprofitYoyRatio,
			"buffett_score":     stock.BuffettScore.TotalScore,
		}

		results = append(results, result)
		totalCurrentPosition += currentAmount
		totalTargetPosition += targetAmount
	}

	// 计算总体偏离度
	totalDiff := totalTargetPosition - totalCurrentPosition
	totalDeviationPercent := 0.0
	if totalTargetPosition > 0 {
		totalDeviationPercent = math.Abs((totalDiff / totalTargetPosition) * 100)
	}

	response := gin.H{
		"holdings": results,
		"summary": gin.H{
			"total_current_position":  totalCurrentPosition,
			"total_target_position":   totalTargetPosition,
			"total_diff":              totalDiff,
			"total_deviation_percent": totalDeviationPercent,
			"stock_count":             len(req.Holdings),
		},
	}

	data["Results"] = response
	c.JSON(http.StatusOK, data)
}

// calculateTargetPosition 计算目标仓位的辅助函数
func calculateTargetPosition(stock models.Stock, expect, tech int) float64 {
	pe := stock.BaseInfo.PE
	growth := stock.BaseInfo.NetprofitYoyRatio

	if growth == 0 {
		return 0
	}

	// 计算PEG
	peg := pe / growth

	// 计算各项得分
	pegScore := 0.0
	if peg <= 0.5 {
		pegScore = 1.0
	} else if peg <= 0.9 {
		pegScore = (0.9 - peg) / 0.4
	} else {
		pegScore = 0.0
	}

	expectScore := float64(expect-1) / 4.0
	techScore := float64(tech-1) / 2.0 // 1->0, 2->0.5, 3->1

	// 使用巴菲特评分
	buffettScore := stock.BuffettScore.TotalScore
	if buffettScore == 0 {
		buffettScore = 50.0 // 默认中等水平
	}
	buffettScoreNormalized := buffettScore / 100.0

	// 计算综合得分
	totalScore := 0.4*pegScore + 0.2*expectScore + 0.2*techScore + 0.2*buffettScoreNormalized

	// 计算建议金额
	amount := 3 + 17*totalScore
	finalAmount := amount
	if finalAmount < 3 {
		finalAmount = 3
	} else if finalAmount > 20 {
		finalAmount = 20
	}

	// PEG过高检查
	if peg > 1 {
		finalAmount = 0
	}

	return finalAmount
}

// ZenStockHoldingHandler 股票仓位看板
func ZenStockHoldingHandler(c *gin.Context) {
	data := gin.H{
		"Env":       viper.GetString("env"),
		"HostURL":   viper.GetString("server.host_url"),
		"Version":   version.Version,
		"PageTitle": "仓位管理",
		"Error":     "",
	}
	c.HTML(http.StatusOK, "zen_stock_holding.html", data)
}

// QueryStockInfoHandler 查询股票基础信息API（用于自动补全）
// 使用新浪接口快速查询，然后从东方财富获取价格
func QueryStockInfoHandler(c *gin.Context) {
	keyword := c.Query("keyword")
	marketParam := c.Query("market") // "a" = A股, "hk" = 港股, "" = 全部

	if keyword == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "请输入股票名称或代码",
		})
		return
	}

	// 使用新浪接口快速搜索股票
	sinaResults, err := datacenter.Sina.KeywordSearch(c, keyword)
	if err != nil || len(sinaResults) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "未找到相关股票数据",
		})
		return
	}

	// 根据市场参数筛选结果
	var targetMarkets []int
	if marketParam == "a" {
		targetMarkets = []int{11} // 只查A股
	} else if marketParam == "hk" {
		targetMarkets = []int{31} // 只查港股
	} else {
		targetMarkets = []int{11, 31} // 查全部
	}

	// 获取第一个匹配的结果（Market=11 A股, Market=31 港股）
	var result *struct {
		Name     string
		Code     string
		Secucode string
		Market   int
	}
	for _, r := range sinaResults {
		// 检查是否在目标市场中
		isTargetMarket := false
		for _, tm := range targetMarkets {
			if r.Market == tm {
				isTargetMarket = true
				break
			}
		}

		if isTargetMarket {
			result = &struct {
				Name     string
				Code     string
				Secucode string
				Market   int
			}{
				Name:     r.Name,
				Code:     r.SecurityCode,
				Secucode: r.Secucode,
				Market:   r.Market,
			}
			break
		}
	}

	if result == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "未找到A股或港股数据",
		})
		return
	}

	// 快速获取当前价格
	price := 0.0
	if result.Market == 11 {
		// A股：使用东方财富选股接口
		filter := eastmoney.Filter{
			SpecialSecurityCodeList: []string{result.Code},
		}
		stocks, err := datacenter.EastMoney.QuerySelectedStocksWithFilter(c, filter)
		if err == nil && len(stocks) > 0 {
			if p, ok := stocks[0].NewPrice.(float64); ok {
				price = p
			}
		}
	} else if result.Market == 31 {
		// 港股：使用东方财富行情接口（传入纯数字代码）
		price = getHKStockPrice(c, result.Code)
	}

	// 返回基础信息
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"name":   result.Name,
			"code":   result.Secucode,
			"price":  price,
			"market": result.Market,       // 11=A股, 31=港股
			"isHK":   result.Market == 31, // 是否为港股
		},
	})
}

// BatchQueryStockPricesHandler 批量查询股票价格API
func BatchQueryStockPricesHandler(c *gin.Context) {
	var req struct {
		Stocks []struct {
			Code string `json:"code" binding:"required"` // Secucode格式，如 "300308.SZ" 或 "00700.HK"
			IsHK bool   `json:"isHK"`                    // 是否为港股
		} `json:"stocks" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "参数错误: " + err.Error(),
		})
		return
	}

	if len(req.Stocks) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "股票列表不能为空",
		})
		return
	}

	// 分离A股和港股
	var aStockCodes []string
	var hkStockMap = make(map[string]string) // 纯数字代码 -> 完整代码的映射
	var codeMap = make(map[string]string)    // 大写代码 -> 原始代码的映射（用于匹配）

	fmt.Printf("🔍 [批量查询] 收到 %d 只股票\n", len(req.Stocks))

	for _, stock := range req.Stocks {
		// 统一转换为大写用于匹配
		upperCode := strings.ToUpper(stock.Code)
		codeMap[upperCode] = stock.Code

		if stock.IsHK {
			// 港股代码，提取纯数字部分
			code := stock.Code
			pureCode := code
			if len(code) > 5 {
				pureCode = code[:5] // 提取前5位，如 "00700"
			}
			hkStockMap[pureCode] = upperCode // 使用大写代码
			fmt.Printf("📊 [港股] 完整代码: %s, 大写: %s, 纯数字: %s\n", stock.Code, upperCode, pureCode)
		} else {
			// A股代码，提取纯数字部分
			code := upperCode
			if len(code) > 6 {
				code = code[:6] // 提取前6位，如 "300308"
			}
			aStockCodes = append(aStockCodes, code)
			fmt.Printf("📊 [A股] 完整代码: %s, 大写: %s, 纯数字: %s\n", stock.Code, upperCode, code)
		}
	}

	prices := make(map[string]float64)

	// 批量查询A股价格
	if len(aStockCodes) > 0 {
		fmt.Printf("🔍 [A股批量查询] 查询 %d 只股票: %v\n", len(aStockCodes), aStockCodes)
		filter := eastmoney.Filter{
			SpecialSecurityCodeList: aStockCodes,
		}
		stocks, err := datacenter.EastMoney.QuerySelectedStocksWithFilter(c, filter)
		if err != nil {
			fmt.Printf("❌ [A股查询失败] %v\n", err)
		} else {
			fmt.Printf("✅ [A股查询成功] 返回 %d 只股票\n", len(stocks))
			for _, stock := range stocks {
				fmt.Printf("📊 [股票信息] Secucode: %s, SecurityCode: %s, Name: %s, NewPrice: %v (type: %T)\n",
					stock.Secucode, stock.SecurityCode, stock.SecurityNameAbbr, stock.NewPrice, stock.NewPrice)

				// 处理 NewPrice，可能是 float64 或 string
				var price float64
				switch v := stock.NewPrice.(type) {
				case float64:
					price = v
				case string:
					// 如果是字符串 "-" 或其他，跳过
					if v != "-" {
						fmt.Printf("⚠️ [价格是字符串] %s: %s\n", stock.Secucode, v)
					}
					continue
				default:
					fmt.Printf("⚠️ [未知价格类型] %s: %v (type: %T)\n", stock.Secucode, stock.NewPrice, stock.NewPrice)
					continue
				}

				if price > 0 {
					prices[stock.Secucode] = price
					fmt.Printf("💰 [A股价格] %s: %.2f\n", stock.Secucode, price)
				}
			}
		}
	}

	// 批量查询港股价格（港股需要逐个查询，因为东方财富接口限制）
	if len(hkStockMap) > 0 {
		fmt.Printf("🔍 [港股查询] 查询 %d 只港股\n", len(hkStockMap))
		for pureCode, upperCode := range hkStockMap {
			price := getHKStockPrice(c, pureCode)
			prices[upperCode] = price
			fmt.Printf("💰 [港股价格] %s (%s): %.2f\n", upperCode, pureCode, price)
		}
	}

	// 构建返回结果，保持原始顺序
	results := make([]gin.H, len(req.Stocks))
	for i, stock := range req.Stocks {
		// 使用大写代码匹配价格
		upperCode := strings.ToUpper(stock.Code)
		price, exists := prices[upperCode]
		if !exists {
			price = 0.0
			fmt.Printf("⚠️ [未找到价格] 原始: %s, 大写: %s\n", stock.Code, upperCode)
		} else {
			fmt.Printf("✅ [找到价格] 原始: %s, 大写: %s, 价格: %.2f\n", stock.Code, upperCode, price)
		}
		results[i] = gin.H{
			"code":  stock.Code, // 返回原始代码（保持前端格式）
			"price": price,
			"isHK":  stock.IsHK,
		}
	}

	fmt.Printf("✅ [批量查询完成] 返回 %d 条结果\n", len(results))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    results,
	})
}

// getHKStockPrice 获取港股价格（使用东方财富行情接口）
// 参数 code 是纯数字代码，如 "00700"
func getHKStockPrice(c *gin.Context, code string) float64 {
	// 东方财富港股行情接口
	// 参考：https://push2.eastmoney.com/api/qt/ulist/get
	// secids 格式：116.00700（116是港股市场代码）
	// 注意：fields 中的逗号需要转义为 %2C
	apiURL := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/ulist/get?fields=f2%%2Cf12%%2Cf13%%2Cf14&secids=116.%s&pn=1&np=1&pz=20",
		code,
	)

	// 打印调试信息
	fmt.Printf("🔍 [港股价格查询] Code: %s\n", code)
	fmt.Printf("🌐 [API URL] %s\n", apiURL)

	type HKPriceResp struct {
		RC   int `json:"rc"`
		Data struct {
			Diff []struct {
				F2  float64 `json:"f2"`  // 最新价（单位：分，需要除以1000）
				F12 string  `json:"f12"` // 股票代码
				F13 int     `json:"f13"` // 市场代码
				F14 string  `json:"f14"` // 股票名称
			} `json:"diff"`
		} `json:"data"`
	}

	resp := HKPriceResp{}
	err := goutils.HTTPGET(c, datacenter.EastMoney.HTTPClient, apiURL, nil, &resp)

	// 打印返回结果
	fmt.Printf("📥 [API Response] RC: %d, Error: %v\n", resp.RC, err)
	if len(resp.Data.Diff) > 0 {
		fmt.Printf("📊 [Price Data] F2: %.0f, F12: %s, F13: %d, F14: %s\n",
			resp.Data.Diff[0].F2,
			resp.Data.Diff[0].F12,
			resp.Data.Diff[0].F13,
			resp.Data.Diff[0].F14)
		price := resp.Data.Diff[0].F2 / 1000.0
		fmt.Printf("💰 [Final Price] %.2f 港元\n", price)
		return price
	}
	fmt.Printf("❌ [Error] No data returned or diff is empty\n")

	return 0.0
}
