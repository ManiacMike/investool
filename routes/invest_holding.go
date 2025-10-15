// 你的新页面

package routes

import (
	"net/http"

	"github.com/axiaoxin-com/investool/core"
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
		Tech         int     `json:"tech" binding:"required"`
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
	techScore := float64(req.Tech) / 3.0

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
