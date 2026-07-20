// 在这个文件中注册 URL handler

package routes

import (
	"github.com/axiaoxin-com/investool/api"
	"github.com/axiaoxin-com/investool/datacenter/periphera"
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
	app.GET("/invest/zen-research-report", api.ZenResearchReportHandler)
	app.GET("/invest/market-clock", api.MarketClockHandler)
	app.GET("/invest/boom-tracker", api.BoomTrackerHandler)
	app.GET("/api/boom/data", api.BoomDataGetHandler)
	app.POST("/api/boom/data", api.BoomDataSaveHandler)
	app.GET("/invest/boom-scores", api.BoomScoresPageHandler)
	app.GET("/api/boom/scores", api.BoomScoresListHandler)
	app.GET("/api/boom/scores/:date", api.BoomScoresGetHandler)
	app.GET("/portal", api.PortalHandler)
	app.GET("/intro", api.IntroHandler)
	app.GET("/news", api.NewsHandler)
	app.GET("/research", api.ResearchHandler)
	app.GET("/api/stock/query", api.QueryStockInfoHandler)
	app.POST("/api/stock/batch-prices", api.BatchQueryStockPricesHandler)

	// PERIPHERA 指挥中心数据接口 /api/v1/*（契约见 PERIPHERA_API.md）
	v1 := app.Group("/api/v1")
	{
		v1.GET("/health", api.PeripheraHealth)
		v1.GET("/news", api.PeripheraNews)
		v1.GET("/news/sources", api.PeripheraNewsSources)
		v1.GET("/research", api.PeripheraResearch)
		v1.GET("/research/:id", api.PeripheraResearchByID)
		v1.POST("/research", api.PeripheraResearchCreate)
		v1.POST("/research/import", api.PeripheraResearchImport)
		v1.POST("/research/collect", api.PeripheraResearchCollect)
		v1.PUT("/research/:id", api.PeripheraResearchUpdate)
		v1.DELETE("/research/:id", api.PeripheraResearchDelete)
		v1.GET("/markets/indices", api.PeripheraIndices)
		v1.GET("/markets/us-sectors", api.PeripheraUSSectors)
		v1.GET("/commodities", api.PeripheraCommodities)
		v1.GET("/commodities/crude", api.PeripheraCrude)
		v1.GET("/crypto", api.PeripheraCrypto)
		v1.GET("/ticker", api.PeripheraTicker)
		v1.GET("/macro/hormuz", api.PeripheraHormuz)
		v1.GET("/macro/fedwatch", api.PeripheraFedWatch)
		v1.GET("/ai/briefing", api.PeripheraBriefing)
	}

	// 从 .env 注入凭据/配置（SEED_API_KEY、PERIPHERA_MYSQL_* 等；真实 env 优先）
	periphera.LoadDotEnv()
	// 启动时即开启 sina 实时行情后台刷新，让首个请求就能拿到真实数据
	periphera.StartLive()
	// 开启实时新闻后台抓取（twscrape 旁路脚本，cron 灌缓存；首拉异步不阻塞启动）
	periphera.StartNews()
	// 开启霍尔木兹公开日报抓取（失败自动回退 seed）
	periphera.StartHormuz()
	// 开启 FedWatch 概率抓取（默认 Investing 免费页；失败自动回退 seed）
	periphera.StartFedWatch()
	// 开启每日 AI 简报生成（火山 Ark Seed 模型，未配 SEED_API_KEY 时自动跳过走 seed）
	periphera.StartBriefing()
}
