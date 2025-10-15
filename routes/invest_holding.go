// 你的新页面

package routes

import (
	"math"
	"net/http"

	"github.com/axiaoxin-com/investool/core"
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
	return
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
	return
}

// QueryStockDataHandler 查询股票数据API
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
			"name":          stock.BaseInfo.SecurityNameAbbr,
			"code":          stock.BaseInfo.Secucode,
			"pe":            stock.BaseInfo.PE,
			"growth":        stock.BaseInfo.NetprofitYoyRatio,
			"industry":      stock.BaseInfo.Industry,
			"market_cap":    stock.BaseInfo.TotalMarketCap,
			"current_price": stock.BaseInfo.NewPrice,
			"buffett_score": gin.H{
				"total_score":         stock.BuffettScore.TotalScore,
				"roe_score":           stock.BuffettScore.ROEScore,
				"cash_flow_score":     stock.BuffettScore.CashFlowScore,
				"profit_growth_score": stock.BuffettScore.ProfitGrowthScore,
				"debt_ratio_score":    stock.BuffettScore.DebtRatioScore,
				"moat_score":          stock.BuffettScore.MoatScore,
				"management_score":    stock.BuffettScore.ManagementScore,
				"valuation_score":     stock.BuffettScore.ValuationScore,
				"rd_score":            stock.BuffettScore.RDScore,
				"dividend_score":      stock.BuffettScore.DividendScore,
				"repurchase_score":    stock.BuffettScore.RepurchaseScore,
				"score_description":   stock.BuffettScore.ScoreDescription,
			},
		}
		break
	}

	data["StockData"] = stockData
	c.JSON(http.StatusOK, data)
	return
}

// CalculatePositionHandler 计算持仓建议API
func CalculatePositionHandler(c *gin.Context) {
	data := gin.H{
		"HostURL":   viper.GetString("server.host_url"),
		"Env":       viper.GetString("env"),
		"Version":   version.Version,
		"PageTitle": "InvesTool | 持仓计算",
		"Error":     "",
		"Result":    nil,
	}

	var req struct {
		StockName    string  `json:"stock_name" binding:"required"`
		PE           float64 `json:"pe" binding:"required"`
		Growth       float64 `json:"growth" binding:"required"`
		Expect       int     `json:"expect" binding:"required"`
		Tech         int     `json:"tech"`
		BuffettScore float64 `json:"buffett_score"`
		CurrentPrice float64 `json:"current_price"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		data["Error"] = "参数错误: " + err.Error()
		c.JSON(http.StatusOK, data)
		return
	}

	if req.Growth == 0 {
		data["Error"] = "利润增长率不能为0"
		c.JSON(http.StatusOK, data)
		return
	}

	// 处理tech参数的默认值
	if req.Tech == 0 {
		req.Tech = 2 // 默认中性
	}

	// 计算PEG
	peg := req.PE / req.Growth

	// 计算各项得分
	pegScore := 0.0
	if peg <= 0.5 {
		pegScore = 1.0
	} else if peg <= 0.9 {
		pegScore = (0.9 - peg) / 0.4
	} else {
		pegScore = 0.0
	}

	expectScore := float64(req.Expect-1) / 4.0
	techScore := float64(req.Tech-1) / 2.0 // 1->0, 2->0.5, 3->1

	// 使用前端提交的巴菲特评分，如果没有则使用默认值
	buffettScore := req.BuffettScore
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
	isPegHigh := peg > 1
	if isPegHigh {
		finalAmount = 0
	}

	// 计算持股数量（使用前端提交的股价）
	shareCount := 0
	currentPrice := req.CurrentPrice
	if currentPrice > 0 && finalAmount > 0 {
		totalValue := finalAmount * 10000        // 万元转元
		shares := int(totalValue / currentPrice) // 总股数
		shareCount = (shares / 100) * 100        // 按100股取整
	}

	result := gin.H{
		"stock_name":    req.StockName,
		"pe":            req.PE,
		"growth":        req.Growth,
		"peg":           peg,
		"expect":        req.Expect,
		"tech":          req.Tech,
		"buffett_score": buffettScore,
		"peg_score":     pegScore,
		"expect_score":  expectScore,
		"tech_score":    techScore,
		"total_score":   totalScore,
		"final_amount":  finalAmount,
		"is_peg_high":   isPegHigh,
		"share_count":   shareCount,
		"current_price": currentPrice,
		"timestamp":     "",
	}

	data["Result"] = result
	c.JSON(http.StatusOK, data)
	return
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
	return
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
