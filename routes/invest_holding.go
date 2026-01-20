// 你的新页面

package routes

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
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
	// 构建详细的股票信息
	var infoParts []string

	// ========== 基本信息 ==========
	infoParts = append(infoParts, "【基本信息】")
	infoParts = append(infoParts, fmt.Sprintf("股票名称：%s", stock.BaseInfo.SecurityNameAbbr))
	infoParts = append(infoParts, fmt.Sprintf("股票代码：%s", stock.BaseInfo.Secucode))
	infoParts = append(infoParts, fmt.Sprintf("所属行业：%s", stock.BaseInfo.Industry))
	if stock.BaseInfo.ListingDate != "" {
		infoParts = append(infoParts, fmt.Sprintf("上市时间：%s", stock.BaseInfo.ListingDate))
	}
	if stock.BaseInfo.ListingYieldYear > 0 {
		infoParts = append(infoParts, fmt.Sprintf("上市以来年化收益率：%.2f%%", stock.BaseInfo.ListingYieldYear))
	}
	if stock.BaseInfo.ListingVolatilityYear > 0 {
		infoParts = append(infoParts, fmt.Sprintf("上市以来年化波动率：%.2f%%", stock.BaseInfo.ListingVolatilityYear))
	}

	// ========== 价格和市值 ==========
	infoParts = append(infoParts, "\n【价格与市值】")
	if price := stock.GetPrice(); price > 0 {
		infoParts = append(infoParts, fmt.Sprintf("当前价格：%.2f元", price))
	}
	if stock.BaseInfo.TotalMarketCap > 0 {
		infoParts = append(infoParts, fmt.Sprintf("总市值：%.2f亿元", stock.BaseInfo.TotalMarketCap/100000000))
	}
	if stock.RightPrice > 0 {
		infoParts = append(infoParts, fmt.Sprintf("合理价格：%.2f元", stock.RightPrice))
		if stock.PriceSpace != 0 {
			infoParts = append(infoParts, fmt.Sprintf("合理价差：%.2f%%", stock.PriceSpace))
		}
	}

	// ========== 估值指标 ==========
	infoParts = append(infoParts, "\n【估值指标】")
	if stock.BaseInfo.PE > 0 {
		infoParts = append(infoParts, fmt.Sprintf("市盈率(PE)：%.2f", stock.BaseInfo.PE))
	}
	if stock.BaseInfo.PBNewMRQ > 0 {
		infoParts = append(infoParts, fmt.Sprintf("市净率(PB)：%.2f", stock.BaseInfo.PBNewMRQ))
	}
	if stock.PEG > 0 {
		infoParts = append(infoParts, fmt.Sprintf("PEG比率：%.2f", stock.PEG))
	}
	if len(stock.ValuationMap) > 0 {
		valuationInfo := "估值指标："
		for k, v := range stock.ValuationMap {
			valuationInfo += fmt.Sprintf("%s=%s ", k, v)
		}
		infoParts = append(infoParts, valuationInfo)
	}
	if len(stock.HistoricalPEList) > 0 {
		peInfo := "历史市盈率："
		for i, pe := range stock.HistoricalPEList {
			if i >= 8 { // 显示最近8个
				break
			}
			peInfo += fmt.Sprintf("%s:%.2f ", pe.Date, pe.Value)
		}
		infoParts = append(infoParts, peInfo)
	}

	// ========== 盈利能力指标 ==========
	infoParts = append(infoParts, "\n【盈利能力】")
	if stock.BaseInfo.RoeWeight > 0 {
		infoParts = append(infoParts, fmt.Sprintf("净资产收益率(ROE)：%.2f%%", stock.BaseInfo.RoeWeight))
	}
	if stock.BaseInfo.ROA > 0 {
		infoParts = append(infoParts, fmt.Sprintf("总资产收益率(ROA)：%.2f%%", stock.BaseInfo.ROA))
	}
	if stock.BaseInfo.Zxgxl > 0 {
		infoParts = append(infoParts, fmt.Sprintf("股息率：%.2f%%", stock.BaseInfo.Zxgxl))
	}

	// 价值评估
	if stock.JZPG.Valueranking != "" {
		infoParts = append(infoParts, fmt.Sprintf("价值评估排名：%s/%s", stock.JZPG.GetValueRanking(), stock.JZPG.Total))
		infoParts = append(infoParts, fmt.Sprintf("整体质地评分：%s", stock.JZPG.GetValueTotalScore()))
		infoParts = append(infoParts, fmt.Sprintf("盈利能力评分：%s", stock.JZPG.GetProfitabilityScore()))
		infoParts = append(infoParts, fmt.Sprintf("成长能力评分：%s", stock.JZPG.GetGrowUpScore()))
		infoParts = append(infoParts, fmt.Sprintf("营运偿债能力评分：%s", stock.JZPG.GetOperationScore()))
		infoParts = append(infoParts, fmt.Sprintf("现金流评分：%s", stock.JZPG.GetCashFlowScore()))
		infoParts = append(infoParts, fmt.Sprintf("估值评分：%s", stock.JZPG.GetValuationScore()))
	}

	// ========== 成长能力指标 ==========
	infoParts = append(infoParts, "\n【成长能力】")
	if stock.BaseInfo.NetprofitYoyRatio != 0 {
		infoParts = append(infoParts, fmt.Sprintf("净利润增长率：%.2f%%", stock.BaseInfo.NetprofitYoyRatio))
	}
	if stock.BaseInfo.ToiYoyRatio != 0 {
		infoParts = append(infoParts, fmt.Sprintf("营业收入增长率：%.2f%%", stock.BaseInfo.ToiYoyRatio))
	}
	if stock.BaseInfo.NetprofitGrowthrate3Y != 0 {
		infoParts = append(infoParts, fmt.Sprintf("净利润3年复合增长率：%.2f%%", stock.BaseInfo.NetprofitGrowthrate3Y))
	}
	if stock.BaseInfo.IncomeGrowthrate3Y != 0 {
		infoParts = append(infoParts, fmt.Sprintf("营收3年复合增长率：%.2f%%", stock.BaseInfo.IncomeGrowthrate3Y))
	}
	if stock.BaseInfo.PredictNetprofitRatio != 0 {
		infoParts = append(infoParts, fmt.Sprintf("预测净利润同比增长：%.2f%%", stock.BaseInfo.PredictNetprofitRatio))
	}
	if stock.BaseInfo.PredictIncomeRatio != 0 {
		infoParts = append(infoParts, fmt.Sprintf("预测营收同比增长：%.2f%%", stock.BaseInfo.PredictIncomeRatio))
	}

	// ========== 历史财报数据 ==========
	if len(stock.HistoricalFinaMainData) > 0 {
		infoParts = append(infoParts, "\n【历史财报数据（最近3期）】")
		for i, fina := range stock.HistoricalFinaMainData {
			if i >= 3 { // 只显示最近3期
				break
			}
			infoParts = append(infoParts, fmt.Sprintf("\n--- %s (%s) ---", fina.ReportDateName, fina.ReportDate))

			// 每股指标
			if fina.Epsjb > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  基本每股收益：%.2f元 (同比%.2f%%)", fina.Epsjb, fina.Epsjbtz))
			}
			if fina.Epskcjb > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  扣非每股收益：%.2f元", fina.Epskcjb))
			}
			if fina.Bps > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  每股净资产：%.2f元 (同比%.2f%%)", fina.Bps, fina.Bpstz))
			}
			if fina.Mgjyxjje != 0 {
				infoParts = append(infoParts, fmt.Sprintf("  每股经营现金流：%.2f元 (同比%.2f%%)", fina.Mgjyxjje, fina.Mgjyxjjetz))
			}

			// 成长能力
			if fina.Totaloperatereve > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  营业总收入：%.2f亿元 (同比%.2f%%)", fina.Totaloperatereve/100000000, fina.Totaloperaterevetz))
			}
			if fina.Parentnetprofit > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  归属净利润：%.2f亿元 (同比%.2f%%)", fina.Parentnetprofit/100000000, fina.Parentnetprofittz))
			}
			if fina.Kcfjcxsyjlr > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  扣非净利润：%.2f亿元 (同比%.2f%%)", fina.Kcfjcxsyjlr/100000000, fina.Kcfjcxsyjlrtz))
			}
			if fina.Mlr > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  毛利润：%.2f亿元", fina.Mlr/100000000))
			}

			// 盈利能力
			if fina.Roejq > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  净资产收益率(加权)：%.2f%% (同比%.2f%%)", fina.Roejq, fina.Roejqtz))
			}
			if fina.Roekcjq > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  净资产收益率(扣非/加权)：%.2f%%", fina.Roekcjq))
			}
			if fina.Zzcjll > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  总资产收益率(ROA)：%.2f%% (同比%.2f%%)", fina.Zzcjll, fina.Zzcjlltz))
			}
			if fina.Roic > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  投入资本回报率(ROIC)：%.2f%% (同比%.2f%%)", fina.Roic, fina.Roictz))
			}
			if fina.Xsmll > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  毛利率：%.2f%%", fina.Xsmll))
			}
			if fina.Xsjll > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  净利率：%.2f%%", fina.Xsjll))
			}

			// 收益质量
			if fina.Xsjxlyysr > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  销售净现金流/营业收入：%.2f", fina.Xsjxlyysr))
			}
			if fina.Jyxjlyysr > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  经营净现金流/营业收入：%.2f", fina.Jyxjlyysr))
			}

			// 偿债能力
			if fina.Zcfzl > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  资产负债率：%.2f%% (同比%.2f%%)", fina.Zcfzl, fina.Zcfzltz))
			}
			if fina.Ld > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  流动比率：%.2f", fina.Ld))
			}
			if fina.Sd > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  速动比率：%.2f", fina.Sd))
			}

			// 营运能力
			if fina.Toazzl > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  总资产周转率：%.2f次", fina.Toazzl))
			}
			if fina.Chzzl > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  存货周转率：%.2f次", fina.Chzzl))
			}
			if fina.Yszkzzl > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  应收账款周转率：%.2f次", fina.Yszkzzl))
			}
		}
	}

	// ========== 现金流量 ==========
	infoParts = append(infoParts, "\n【现金流量】")
	if stock.NetcashOperate != 0 {
		infoParts = append(infoParts, fmt.Sprintf("经营活动现金流量净额：%.2f万元", stock.NetcashOperate/10000))
	}
	if stock.NetcashInvest != 0 {
		infoParts = append(infoParts, fmt.Sprintf("投资活动产生的现金流量净额：%.2f万元", stock.NetcashInvest/10000))
	}
	if stock.NetcashFinance != 0 {
		infoParts = append(infoParts, fmt.Sprintf("筹资活动产生的现金流量净额：%.2f万元", stock.NetcashFinance/10000))
	}
	if stock.NetcashFree != 0 {
		infoParts = append(infoParts, fmt.Sprintf("自由现金流：%.2f万元", stock.NetcashFree/10000))
	}

	// 历史现金流量表数据
	if len(stock.HistoricalCashflowList) > 0 {
		infoParts = append(infoParts, "历史现金流量（最近3期）：")
		for i, cf := range stock.HistoricalCashflowList {
			if i >= 3 {
				break
			}
			if cf.NetcashOperate != 0 {
				infoParts = append(infoParts, fmt.Sprintf("  %s: 经营%.2f万元, 投资%.2f万元, 筹资%.2f万元",
					cf.ReportDateName, cf.NetcashOperate/10000, cf.NetcashInvest/10000, cf.NetcashFinance/10000))
			}
		}
	}

	// ========== 负债情况 ==========
	infoParts = append(infoParts, "\n【财务结构】")
	if stock.BaseInfo.DebtAssetRatio > 0 {
		infoParts = append(infoParts, fmt.Sprintf("资产负债率：%.2f%%", stock.BaseInfo.DebtAssetRatio))
	}
	if stock.BYYSRatio > 0 {
		infoParts = append(infoParts, fmt.Sprintf("本业营收比：%.2f%%", stock.BYYSRatio*100))
	}

	// ========== 公司资料 ==========
	if stock.CompanyProfile.Name != "" {
		infoParts = append(infoParts, "\n【公司资料】")
		infoParts = append(infoParts, fmt.Sprintf("公司名称：%s", stock.CompanyProfile.Name))
		if stock.CompanyProfile.Profile != "" {
			infoParts = append(infoParts, fmt.Sprintf("公司简介：%s", stock.CompanyProfile.Profile))
		}
		if stock.CompanyProfile.MainBusiness != "" {
			infoParts = append(infoParts, fmt.Sprintf("主营业务：%s", stock.CompanyProfile.MainBusiness))
		}
		if len(stock.CompanyProfile.Keywords) > 0 {
			keywords := strings.Join(stock.CompanyProfile.Keywords, "、")
			infoParts = append(infoParts, fmt.Sprintf("题材关键词：%s", keywords))
		}
		if len(stock.CompanyProfile.MainForms) > 0 {
			infoParts = append(infoParts, "主营构成：")
			infoParts = append(infoParts, stock.CompanyProfile.MainFormsString())
		}
	}

	// ========== 主营业务和概念 ==========
	infoParts = append(infoParts, "\n【业务与概念】")
	if stock.MainBusiness != "" {
		infoParts = append(infoParts, fmt.Sprintf("主营业务：%s", stock.MainBusiness))
	}
	if stock.Concept != "" {
		infoParts = append(infoParts, fmt.Sprintf("所属概念：%s", stock.Concept))
	}

	// ========== 机构评级和盈利预测 ==========
	infoParts = append(infoParts, "\n【机构评级与预测】")
	if len(stock.OrgRatingList) > 0 {
		ratingInfo := "机构评级："
		for i, rating := range stock.OrgRatingList {
			if i >= 5 { // 显示前5个
				break
			}
			ratingInfo += fmt.Sprintf("%s(%s) ", rating.DateType, rating.CompreRating)
		}
		infoParts = append(infoParts, ratingInfo)
	}
	if len(stock.ProfitPredictList) > 0 {
		infoParts = append(infoParts, "盈利预测：")
		for i, predict := range stock.ProfitPredictList {
			if i >= 5 { // 显示前5个
				break
			}
			infoParts = append(infoParts, fmt.Sprintf("  %d年：预测EPS%.2f元，预测PE%.2f", predict.PredictYear, predict.Eps, predict.Pe))
		}
	}

	// ========== 股东信息 ==========
	if len(stock.FreeHoldersTop10) > 0 {
		infoParts = append(infoParts, "\n【十大流通股东】")
		for i, holder := range stock.FreeHoldersTop10 {
			if i >= 10 { // 显示全部10个
				break
			}
			holderType := "个人"
			if holder.IsHoldorg == "1" {
				holderType = "机构"
			}
			infoParts = append(infoParts, fmt.Sprintf("%d. %s (%s) 持股%.2f%% 排名%d",
				i+1, holder.HolderName, holderType, holder.FreeHoldnumRatio, holder.HolderRank))
		}
	}

	// ========== 历史利润表数据 ==========
	if len(stock.HistoricalGincomeList) > 0 {
		infoParts = append(infoParts, "\n【历史利润表数据（最近3期）】")
		for i, income := range stock.HistoricalGincomeList {
			if i >= 3 {
				break
			}
			infoParts = append(infoParts, fmt.Sprintf("\n--- %s (%s) ---", income.ReportDateName, income.ReportDate))
			if income.TotalOperateIncome > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  营业总收入：%.2f亿元 (同比%.2f%%)",
					income.TotalOperateIncome/100000000, income.TotalOperateIncomeYoy))
			}
			if income.OperateIncome > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  营业收入：%.2f亿元 (同比%.2f%%)",
					income.OperateIncome/100000000, income.OperateIncomeYoy))
			}
			if income.TotalOperateCost > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  营业总成本：%.2f亿元 (同比%.2f%%)",
					income.TotalOperateCost/100000000, income.TotalOperateCostYoy))
			}
			if income.ResearchExpense > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  研发费用：%.2f亿元 (同比%.2f%%)",
					income.ResearchExpense/100000000, income.ResearchExpenseYoy))
			}
			if income.SaleExpense > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  销售费用：%.2f亿元 (同比%.2f%%)",
					income.SaleExpense/100000000, income.SaleExpenseYoy))
			}
			if income.ManageExpense > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  管理费用：%.2f亿元 (同比%.2f%%)",
					income.ManageExpense/100000000, income.ManageExpenseYoy))
			}
			if income.FinanceExpense > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  财务费用：%.2f亿元 (同比%.2f%%)",
					income.FinanceExpense/100000000, income.FinanceExpenseYoy))
			}
			if income.Netprofit > 0 {
				infoParts = append(infoParts, fmt.Sprintf("  净利润：%.2f亿元 (同比%.2f%%)",
					income.Netprofit/100000000, income.NetprofitYoy))
			}
		}
	}

	// ========== 资金流向 ==========
	if len(stock.MainMoneyNetInflows) > 0 {
		infoParts = append(infoParts, "\n【主力资金流向（最近5日）】")
		for i, inflow := range stock.MainMoneyNetInflows {
			if i >= 5 {
				break
			}
			mainNetIn := 0.0
			if inflow.MainMnyNetIn != "" {
				if val, err := strconv.ParseFloat(inflow.MainMnyNetIn, 64); err == nil {
					mainNetIn = val
				}
			}
			infoParts = append(infoParts, fmt.Sprintf("  %s: 主力净流入%.2f万元", inflow.TrdDt, mainNetIn))
		}
	}

	// ========== 财报信息 ==========
	infoParts = append(infoParts, "\n【财报披露信息】")
	if stock.FinaReportDate != "" {
		infoParts = append(infoParts, fmt.Sprintf("最新财报日期：%s", stock.FinaReportDate))
	}
	if stock.FinaAppointPublishDate != "" {
		infoParts = append(infoParts, fmt.Sprintf("预约披露日期：%s", stock.FinaAppointPublishDate))
	}
	if stock.FinaActualPublishDate != "" {
		infoParts = append(infoParts, fmt.Sprintf("实际披露日期：%s", stock.FinaActualPublishDate))
	}
	if stock.FinaReportOpinion != "" {
		infoParts = append(infoParts, fmt.Sprintf("审计意见：%s", stock.FinaReportOpinion))
	}

	// ========== 历史波动率 ==========
	if stock.HistoricalVolatility > 0 {
		infoParts = append(infoParts, fmt.Sprintf("\n历史波动率：%.2f%%", stock.HistoricalVolatility))
	}

	// 组装完整prompt
	prompt := "请搜索最新的信息分析以下股票信息：\n\n"
	prompt += strings.Join(infoParts, "\n")
	prompt += "\n\n"
	prompt += fmt.Sprintf("这是关于%s(%s)的详细准确信息，请给出详细多维度分析包括行业前景，公司护城河，业务预期，2026年增长机构预期。",
		stock.BaseInfo.SecurityNameAbbr, stock.BaseInfo.Secucode)

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
	marketCaps := make(map[string]float64) // 存储市值（单位：亿元）

	// 批量查询A股价格和市值
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
				fmt.Printf("📊 [股票信息] Secucode: %s, SecurityCode: %s, Name: %s, NewPrice: %v (type: %T), MarketCap: %.2f\n",
					stock.Secucode, stock.SecurityCode, stock.SecurityNameAbbr, stock.NewPrice, stock.NewPrice, stock.TotalMarketCap)

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

				// 保存市值（TotalMarketCap单位是元，转换为亿元）
				if stock.TotalMarketCap > 0 {
					marketCaps[stock.Secucode] = stock.TotalMarketCap / 100000000 // 转换为亿元
					fmt.Printf("📈 [A股市值] %s: %.2f亿元\n", stock.Secucode, marketCaps[stock.Secucode])
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

		// 获取市值（A股有数据，港股暂时为0）
		marketCap := 0.0
		if !stock.IsHK {
			// A股市值
			if cap, exists := marketCaps[upperCode]; exists {
				marketCap = cap
			}
		}
		// 港股市值暂时不获取，后续可以优化

		results[i] = gin.H{
			"code":      stock.Code, // 返回原始代码（保持前端格式）
			"price":     price,
			"isHK":      stock.IsHK,
			"marketCap": marketCap, // 市值（单位：亿元）
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
