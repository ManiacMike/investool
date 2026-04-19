// 在这个文件中注册 URL handler

package routes

import (
	"github.com/axiaoxin-com/investool/api"
	"github.com/gin-gonic/gin"
)

// Routes 注册 API URL 路由
func Routes(app *gin.Engine) {
	app.GET("/", api.FundIndex)
	app.GET("/stock", api.StockIndex)
	app.POST("/selector", api.StockSelector)
	app.POST("/checker", api.StockChecker)
	app.GET("/fund", api.FundIndex)
	app.GET("/fund/filter", api.FundFilter)
	app.POST("/fund/check", api.FundCheck)
	app.GET("/about", api.About)
	app.GET("/comment", api.Comment)
	app.GET("/fund/similarity", api.FundSimilarity)
	app.GET("/materials", api.Materials)
	app.POST("/fund/query_by_stock", api.QueryFundByStock)
	app.GET("/fund/managers", api.FundManagers)
	app.GET("/invest/holding-calculator", api.InvestHoldingHandler)
	app.GET("/invest/stock-analyzer", api.StockAnalyzerHandler)
	app.GET("/invest/query-stock", api.QueryStockDataHandler)
	app.POST("/invest/calculate-stock-score", api.CalculateStockScoreHandler)
	app.POST("/invest/position-deviation", api.PositionDeviationHandler)
	app.GET("/invest/zen-holding", api.ZenStockHoldingHandler)
	app.GET("/invest/zen-note", api.ZenStockNoteHandler)
	app.GET("/api/stock/query", api.QueryStockInfoHandler)
	app.POST("/api/stock/batch-prices", api.BatchQueryStockPricesHandler)
}
