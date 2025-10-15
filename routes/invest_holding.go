// 你的新页面

package routes

import (
	"net/http"

	"github.com/axiaoxin-com/investool/core"
	"github.com/axiaoxin-com/investool/version"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// InvestHoldingHandler 你的新页面处理函数
func InvestHoldingHandler(c *gin.Context) {
	data := gin.H{
		"Env":       viper.GetString("env"),
		"HostURL":   viper.GetString("server.host_url"),
		"Version":   version.Version,
		"PageTitle": "InvesTool | 投资持仓计算器",
		"Error":     "",
		// 在这里添加你需要的其他数据
	}
	c.HTML(http.StatusOK, "invest_holding.html", data)
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
