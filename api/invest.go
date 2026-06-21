// 你的新页面

package api

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/datacenter"
	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/models"
	"github.com/axiaoxin-com/investool/version"
	"github.com/axiaoxin-com/logging"
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
			"llm_prompt":    core.GenerateLLMPrompt(stock),
		}
		break
	}

	data["StockData"] = stockData
	c.JSON(http.StatusOK, data)
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
		targetAmount := core.CalculateTargetPosition(stock, expect, tech)

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

// ZenStockNoteHandler 股票笔记页面
func ZenStockNoteHandler(c *gin.Context) {
	data := gin.H{
		"Env":       viper.GetString("env"),
		"HostURL":   viper.GetString("server.host_url"),
		"Version":   version.Version,
		"PageTitle": "股票笔记",
		"Error":     "",
	}
	c.HTML(http.StatusOK, "zen_stock_note.html", data)
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
	switch marketParam {
	case "a":
		targetMarkets = []int{11} // 只查A股
	case "hk":
		targetMarkets = []int{31} // 只查港股
	default:
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
	switch result.Market {
	case 11:
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
	case 31:
		// 港股：使用东方财富行情接口（传入纯数字代码）
		price = core.GetHKStockPrice(c, result.Code)
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

	logging.Debugf(c, "BatchQueryStockPrices received %d stocks", len(req.Stocks))

	for _, stock := range req.Stocks {
		// 统一转换为大写用于匹配
		upperCode := strings.ToUpper(stock.Code)
		codeMap[upperCode] = stock.Code

		if stock.IsHK {
			// 港股代码处理：按小数点分割，把后面的部分移到前面
			// 例如：700.00 -> 00700, 367.02 -> 02367
			code := strings.ToUpper(stock.Code)
			// 去掉 .HK 后缀
			code = strings.TrimSuffix(code, ".HK")

			var pureCode string
			if strings.Contains(code, ".") {
				// 如果有小数点，按小数点分割
				parts := strings.Split(code, ".")
				frontPart := parts[0] // 前面的部分，如 "700" 或 "367"
				backPart := ""
				if len(parts) > 1 {
					backPart = parts[1] // 后面的部分，如 "00" 或 "02"
				}

				// 提取数字部分（去掉非数字字符）
				frontDigits := ""
				for _, r := range frontPart {
					if r >= '0' && r <= '9' {
						frontDigits += string(r)
					}
				}
				backDigits := ""
				for _, r := range backPart {
					if r >= '0' && r <= '9' {
						backDigits += string(r)
					}
				}

				// 前面部分补0到3位，后面部分补0到2位
				if len(frontDigits) < 3 {
					frontDigits = strings.Repeat("0", 3-len(frontDigits)) + frontDigits
				} else if len(frontDigits) > 3 {
					frontDigits = frontDigits[:3] // 只取前3位
				}
				if len(backDigits) < 2 {
					backDigits = backDigits + strings.Repeat("0", 2-len(backDigits))
				} else if len(backDigits) > 2 {
					backDigits = backDigits[:2] // 只取前2位
				}

				// 把后面的部分放到前面：后面2位 + 前面3位 = 5位
				pureCode = backDigits + frontDigits
			} else {
				// 如果没有小数点，提取纯数字部分，前面补0到5位
				pureCode = ""
				for _, r := range code {
					if r >= '0' && r <= '9' {
						pureCode += string(r)
					}
				}
				if len(pureCode) < 5 {
					pureCode = strings.Repeat("0", 5-len(pureCode)) + pureCode
				} else if len(pureCode) > 5 {
					pureCode = pureCode[:5]
				}
			}

			hkStockMap[pureCode] = upperCode // 使用大写代码
			logging.Debugf(c, "BatchQueryStockPrices HK code=%s upper=%s pure=%s", stock.Code, upperCode, pureCode)
		} else {
			// A股代码，提取纯数字部分
			code := upperCode
			if len(code) > 6 {
				code = code[:6] // 提取前6位，如 "300308"
			}
			aStockCodes = append(aStockCodes, code)
			logging.Debugf(c, "BatchQueryStockPrices A code=%s upper=%s pure=%s", stock.Code, upperCode, code)
		}
	}

	prices := make(map[string]float64)
	marketCaps := make(map[string]float64) // 存储市值（单位：亿元）
	// fundamentals 保存每只 A 股的基本面指标，键为大写 Secucode
	fundamentals := make(map[string]gin.H)

	// 批量查询A股价格、市值及基本面指标
	if len(aStockCodes) > 0 {
		logging.Debugf(c, "BatchQueryStockPrices querying %d A stocks: %v", len(aStockCodes), aStockCodes)
		filter := eastmoney.Filter{
			SpecialSecurityCodeList: aStockCodes,
		}
		stocks, err := datacenter.EastMoney.QuerySelectedStocksWithFilter(c, filter)
		if err != nil {
			logging.Errorf(c, "BatchQueryStockPrices query A stocks err:%v", err)
		} else {
			logging.Debugf(c, "BatchQueryStockPrices A stocks returned %d", len(stocks))
			for _, stock := range stocks {
				secucodeUpper := strings.ToUpper(stock.Secucode)

				// 处理 NewPrice，可能是 float64 或 string
				var price float64
				switch v := stock.NewPrice.(type) {
				case float64:
					price = v
				case string:
					// 如果是字符串 "-" 或其他，跳过价格但仍可保留基本面
					if v != "-" {
						logging.Debugf(c, "BatchQueryStockPrices %s price is string:%s", stock.Secucode, v)
					}
				default:
					logging.Debugf(c, "BatchQueryStockPrices %s unknown price type:%T", stock.Secucode, stock.NewPrice)
				}

				if price > 0 {
					prices[secucodeUpper] = price
				}

				// 保存市值（TotalMarketCap单位是元，转换为亿元）
				if stock.TotalMarketCap > 0 {
					marketCaps[secucodeUpper] = stock.TotalMarketCap / 100000000 // 转换为亿元
				}

				// 保存基本面指标（来自同一次选股接口调用，无额外请求成本）
				fundamentals[secucodeUpper] = gin.H{
					"industry":              stock.Industry,
					"pe":                    stock.PE,
					"pb":                    stock.PBNewMRQ,
					"roe":                   stock.RoeWeight,
					"dividendYield":         stock.Zxgxl,
					"netprofitYoyRatio":     stock.NetprofitYoyRatio,
					"predictNetprofitRatio": stock.PredictNetprofitRatio,
				}
			}
		}
	}

	// 批量查询港股价格
	if len(hkStockMap) > 0 {
		logging.Debugf(c, "BatchQueryStockPrices querying %d HK stocks", len(hkStockMap))
		pureCodes := make([]string, 0, len(hkStockMap))
		for pc := range hkStockMap {
			pureCodes = append(pureCodes, pc)
		}

		hkPrices := core.GetHKStockPrices(c, pureCodes)
		for pc, price := range hkPrices {
			if upperCode, exists := hkStockMap[pc]; exists {
				prices[upperCode] = price
			}
		}
	}

	// 构建返回结果，保持原始顺序
	results := make([]gin.H, len(req.Stocks))
	for i, stock := range req.Stocks {
		// 使用大写代码匹配价格
		upperCode := strings.ToUpper(stock.Code)
		price := prices[upperCode]

		// 获取市值（A股有数据，港股暂时为0）
		marketCap := 0.0
		if !stock.IsHK {
			marketCap = marketCaps[upperCode]
		}
		// 港股市值暂时不获取，后续可以优化

		result := gin.H{
			"code":      stock.Code, // 返回原始代码（保持前端格式）
			"price":     price,
			"isHK":      stock.IsHK,
			"marketCap": marketCap, // 市值（单位：亿元）
		}
		// 合并基本面指标（A股，来自同一次接口调用）
		if f, ok := fundamentals[upperCode]; ok {
			for k, v := range f {
				result[k] = v
			}
		}
		results[i] = result
	}

	logging.Debugf(c, "BatchQueryStockPrices done, returning %d results", len(results))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    results,
	})
}

// ZenResearchReportHandler 研报数据页面
func ZenResearchReportHandler(c *gin.Context) {
	data := gin.H{
		"Env":       viper.GetString("env"),
		"HostURL":   viper.GetString("server.host_url"),
		"Version":   version.Version,
		"PageTitle": "研报管理",
		"Error":     "",
	}
	c.HTML(http.StatusOK, "zen_research_report.html", data)
}

// MarketClockHandler 市场风格时钟页面
func MarketClockHandler(c *gin.Context) {
	data := gin.H{
		"Env":       viper.GetString("env"),
		"HostURL":   viper.GetString("server.host_url"),
		"Version":   version.Version,
		"PageTitle": "市场风格时钟",
		"Error":     "",
	}
	c.HTML(http.StatusOK, "market_clock.html", data)
}
