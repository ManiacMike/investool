package core

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/axiaoxin-com/investool/datacenter"
	"github.com/axiaoxin-com/investool/models"
	"github.com/gin-gonic/gin"
)

// GenerateLLMPrompt 生成LLM查询prompt
func GenerateLLMPrompt(stock models.Stock) string {
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

// GetScoreLevel 根据分数返回等级
func GetScoreLevel(score float64) string {
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

// CalculateTargetPosition 计算目标仓位的辅助函数
func CalculateTargetPosition(stock models.Stock, expect, tech int) float64 {
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

// GetHKStockPrice 获取港股价格（使用腾讯行情接口）
// 参数 code 是纯数字代码，如 "00700"
func GetHKStockPrice(c *gin.Context, code string) float64 {
	// 使用腾讯行情接口，因为东方财富push2接口对Go的HTTP client可能会拦截导致EOF
	// 返回示例：v_hk01810="100~小米集团-W~01810~31.140~30.900~..."
	apiURL := fmt.Sprintf("http://qt.gtimg.cn/q=hk%s", code)

	// 打印调试信息
	fmt.Printf("🔍 [港股价格查询] Code: %s\n", code)
	fmt.Printf("🌐 [API URL] %s\n", apiURL)

	req, err := http.NewRequestWithContext(c, http.MethodGet, apiURL, nil)
	if err != nil {
		fmt.Printf("❌ [Error] Create request failed: %v\n", err)
		return 0.0
	}

	resp, err := datacenter.EastMoney.HTTPClient.Do(req)
	if err != nil {
		fmt.Printf("❌ [Error] Request failed: %v\n", err)
		return 0.0
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ [Error] Read body failed: %v\n", err)
		return 0.0
	}

	// 以 ~ 分割后的第4个元素（索引3）为当前最新价
	parts := strings.Split(string(bodyBytes), "~")
	if len(parts) >= 4 {
		if price, err := strconv.ParseFloat(parts[3], 64); err == nil {
			fmt.Printf("💰 [Final Price] %.2f 港元\n", price)
			return price
		}
	}

	fmt.Printf("❌ [Error] Invalid or empty response from Tencent: %s\n", string(bodyBytes))
	return 0.0
}

// GetHKStockPrices 批量获取港股价格（使用腾讯行情接口）
// 参数 codes 是纯数字代码列表，如 []string{"00700", "01810"}
// 返回 map[代码]价格
func GetHKStockPrices(c *gin.Context, codes []string) map[string]float64 {
	results := make(map[string]float64)
	if len(codes) == 0 {
		return results
	}

	// 腾讯接口支持批量查询，代码间用逗号分隔，每个代码前加 hk
	formattedCodes := []string{}
	for _, code := range codes {
		formattedCodes = append(formattedCodes, "hk"+code)
	}
	apiURL := fmt.Sprintf("http://qt.gtimg.cn/q=%s", strings.Join(formattedCodes, ","))

	fmt.Printf("🔍 [港股批量查询] 收到 %d 只股票\n", len(codes))
	fmt.Printf("🌐 [API URL] %s\n", apiURL)

	req, err := http.NewRequestWithContext(c, http.MethodGet, apiURL, nil)
	if err != nil {
		fmt.Printf("❌ [Error] Create request failed: %v\n", err)
		return results
	}

	resp, err := datacenter.EastMoney.HTTPClient.Do(req)
	if err != nil {
		fmt.Printf("❌ [Error] Request failed: %v\n", err)
		return results
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ [Error] Read body failed: %v\n", err)
		return results
	}

	// 返回结果是多行，每行以分号结束
	// 例如: v_hk00700="...~...~...~31.140~...";v_hk01810="...~...~...~15.200~...";
	lines := strings.Split(string(bodyBytes), ";")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 提取代码
		// 格式类似 v_hk00700="...
		startIdx := strings.Index(line, "v_hk")
		endIdx := strings.Index(line, "=")
		if startIdx == -1 || endIdx == -1 {
			continue
		}
		pureCode := line[startIdx+4 : endIdx]

		// 提取价格：以 ~ 分割后的第4个元素（索引3）
		parts := strings.Split(line, "~")
		if len(parts) >= 4 {
			if price, err := strconv.ParseFloat(parts[3], 64); err == nil {
				results[pureCode] = price
				fmt.Printf("💰 [港股批量结果] %s: %.2f 港元\n", pureCode, price)
			}
		}
	}

	return results
}
