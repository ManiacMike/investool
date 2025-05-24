// 股票对象封装

package models

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/axiaoxin-com/investool/datacenter"
	"github.com/axiaoxin-com/investool/datacenter/eastmoney"
	"github.com/axiaoxin-com/investool/datacenter/eniu"
	"github.com/axiaoxin-com/investool/datacenter/zszx"
	"github.com/axiaoxin-com/logging"
)

// BuffettScore 巴菲特评分结构体
type BuffettScore struct {
	ROEScore          float64 `json:"roe_score"`           // ROE评分（20分）
	CashFlowScore     float64 `json:"cash_flow_score"`     // 自由现金流评分（15分）
	ProfitGrowthScore float64 `json:"profit_growth_score"` // 利润增长评分（15分）
	DebtRatioScore    float64 `json:"debt_ratio_score"`    // 负债率评分（10分）
	MoatScore         float64 `json:"moat_score"`          // 护城河评分（10分）
	ManagementScore   float64 `json:"management_score"`    // 管理层评分（10分）
	ValuationScore    float64 `json:"valuation_score"`     // 估值评分（15分）
	TotalScore        float64 `json:"total_score"`         // 总分（100分）
	ScoreDescription  string  `json:"score_description"`   // 评分说明
	RDScore           float64 `json:"rd_score"`            // 研发投入评分（5分）
	DividendScore     float64 `json:"dividend_score"`      // 分红评分（5分）
	RepurchaseScore   float64 `json:"repurchase_score"`    // 回购评分（5分）
}

// Stock 接口返回的股票信息结构
type Stock struct {
	// 东方财富接口返回的基本信息
	BaseInfo eastmoney.StockInfo `json:"base_info"`
	// 历史财报信息
	HistoricalFinaMainData eastmoney.HistoricalFinaMainData `json:"historical_fina_main_data"`
	// 市盈率、市净率、市销率、市现率估值
	ValuationMap map[string]string `json:"valuation_map"`
	// 历史市盈率
	HistoricalPEList eastmoney.HistoricalPEList `json:"historical_pe_list"`
	// 合理价格（年报）：改进算法 - 使用多年EPS平均值 * 历史市盈率中位数 * (1 + 限制后的增长率) * 爆发增长调整系数
	RightPrice float64 `json:"right_price"`
	// 合理价差（%）
	PriceSpace float64 `json:"price_space"`
	// 按改进算法计算的去年合理价格：用于验证算法准确性
	LastYearRightPrice float64 `json:"last_year_right_price"`
	// 历史股价
	HistoricalPrice eniu.RespHistoricalStockPrice `json:"historical_price"`
	// 历史波动率
	HistoricalVolatility float64 `json:"historical_volatility"`
	// 公司资料
	CompanyProfile eastmoney.CompanyProfile `json:"company_profile"`
	// 预约财报披露日期
	FinaAppointPublishDate string `json:"fina_appoint_publish_date"`
	// 实际财报披露日期
	FinaActualPublishDate string `json:"fina_actual_publish_date"`
	// 财报披露日期
	FinaReportDate string `json:"fina_report_date"`
	// 机构评级
	OrgRatingList eastmoney.OrgRatingList `json:"org_rating_list"`
	// 盈利预测
	ProfitPredictList eastmoney.ProfitPredictList `json:"profit_predict_list"`
	// 价值评估
	JZPG eastmoney.JZPG `json:"jzpg"`
	// PEG=PE/净利润复合增长率
	PEG float64 `json:"peg"`
	// 历史利润表
	HistoricalGincomeList eastmoney.GincomeDataList `json:"historical_gincome_list"`
	// 本业营收比=营业利润/(营业利润+营业外收入)
	BYYSRatio float64 `json:"byys_ratio"`
	// 最新财报审计意见
	FinaReportOpinion string `json:"fina_report_opinion"`
	// 历史现金流量表
	HistoricalCashflowList eastmoney.CashflowDataList `json:"historical_cashdlow_list"`
	// 最新经营活动产生的现金流量净额
	NetcashOperate float64 `json:"netcash_operate"`
	// 最新投资活动产生的现金流量净额
	NetcashInvest float64 `json:"netcash_invest"`
	// 最新筹资活动产生的现金流量净额
	NetcashFinance float64 `json:"netcash_finance"`
	// 自由现金流
	NetcashFree float64 `json:"netcash_free"`
	// 十大流通股东
	FreeHoldersTop10 eastmoney.FreeHolderList `json:"free_holders_top_10"`
	// 主力资金净流入
	MainMoneyNetInflows zszx.NetInflowList `json:"main_money_net_inflows"`
	// 巴菲特评分
	BuffettScore BuffettScore `json:"buffett_score"`
}

// GetPrice 返回股价，没开盘时可能是字符串"-"，此时返回最近历史股价，无历史价则返回 -1
func (s Stock) GetPrice() float64 {
	p, ok := s.BaseInfo.NewPrice.(float64)
	if ok {
		return p
	}
	if len(s.HistoricalPrice.Price) == 0 {
		return -1.0
	}
	return s.HistoricalPrice.Price[len(s.HistoricalPrice.Price)-1]
}

// GetOrgType 获取机构类型
func (s Stock) GetOrgType() string {
	if len(s.HistoricalFinaMainData) == 0 {
		return ""
	}
	return s.HistoricalFinaMainData[0].OrgType
}

// StockList 股票列表
type StockList []Stock

// SortByROE 股票列表按 ROE 排序
func (s StockList) SortByROE() {
	sort.Slice(s, func(i, j int) bool {
		return s[i].BaseInfo.RoeWeight > s[j].BaseInfo.RoeWeight
	})
}

// SortByPriceSpace 股票列表按合理价差排序
func (s StockList) SortByPriceSpace() {
	sort.Slice(s, func(i, j int) bool {
		return s[i].PriceSpace > s[j].PriceSpace
	})
}

// NewStock 创建 Stock 对象
func NewStock(ctx context.Context, baseInfo eastmoney.StockInfo) (Stock, error) {
	s := Stock{
		BaseInfo: baseInfo,
	}

	// PEG 改进计算
	logging.Infof(ctx, "[%s] 开始计算PEG, PE=%.2f, 净利润3年复合增长率=%.2f%%",
		s.BaseInfo.SecurityNameAbbr,
		s.BaseInfo.PE,
		s.BaseInfo.NetprofitGrowthrate3Y)

	if s.BaseInfo.NetprofitGrowthrate3Y == 0 {
		// 增长率为0时，PEG设为-1表示无效
		s.PEG = -1
		logging.Infof(ctx, "[%s] NetprofitGrowthrate3Y为0, PEG设置为-1", s.BaseInfo.SecurityNameAbbr)
	} else if s.BaseInfo.NetprofitGrowthrate3Y < 0 {
		// 负增长率时，PEG设为-1表示无效
		s.PEG = -1
		logging.Infof(ctx, "[%s] NetprofitGrowthrate3Y为负值: %.2f%%, PEG设置为-1",
			s.BaseInfo.SecurityNameAbbr,
			s.BaseInfo.NetprofitGrowthrate3Y)
	} else {
		s.PEG = s.BaseInfo.PE / s.BaseInfo.NetprofitGrowthrate3Y
		// 检查计算结果是否为异常值
		if math.IsNaN(s.PEG) || math.IsInf(s.PEG, 0) {
			s.PEG = -1
			logging.Warnf(ctx, "[%s] PEG计算结果异常(NaN或Inf), 设置为-1. PE=%.2f, NetprofitGrowthrate3Y=%.2f%%",
				s.BaseInfo.SecurityNameAbbr,
				s.BaseInfo.PE,
				s.BaseInfo.NetprofitGrowthrate3Y)
		} else {
			logging.Infof(ctx, "[%s] PEG计算结果=%.2f (PE=%.2f / NetprofitGrowthrate3Y=%.2f%%)",
				s.BaseInfo.SecurityNameAbbr,
				s.PEG,
				s.BaseInfo.PE,
				s.BaseInfo.NetprofitGrowthrate3Y)
		}
	}
	price := s.GetPrice()

	var wg sync.WaitGroup
	// 获取财报
	wg.Add(1)
	go func(ctx context.Context, s *Stock) {
		defer wg.Done()
		logging.Info(ctx, "开始获取历史财务数据")
		hf, err := datacenter.EastMoney.QueryHistoricalFinaMainData(ctx, s.BaseInfo.Secucode)
		if err != nil {
			logging.Error(ctx, "NewStock QueryHistoricalFinaMainData err:"+err.Error())
			return
		}
		if len(hf) == 0 {
			logging.Error(ctx, "HistoricalFinaMainData is empty")
			return
		}
		logging.Info(ctx, fmt.Sprintf("获取到历史财务数据，数据条数: %d, 最新报告期: %s", len(hf), hf[0].ReportDate))
		s.HistoricalFinaMainData = hf

		// 历史市盈率 && 合理价格
		peList, err := datacenter.EastMoney.QueryHistoricalPEList(ctx, s.BaseInfo.Secucode)
		if err != nil {
			logging.Error(ctx, "NewStock QueryHistoricalPEList err:"+err.Error())
			return
		}
		s.HistoricalPEList = peList

		// 合理价格判断
		// 去年年报
		lastYearReport := s.HistoricalFinaMainData.GetReport(ctx, time.Now().Year()-1, eastmoney.FinaReportTypeYear)
		beforeLastYearReport := s.HistoricalFinaMainData.GetReport(ctx, time.Now().Year()-2, eastmoney.FinaReportTypeYear)
		thisYear := time.Now().Year()
		thisYearAvgRevIncrRatio := s.HistoricalFinaMainData.GetAvgRevenueIncreasingRatioByYear(ctx, thisYear)
		lastYearAvgRevIncrRatio := s.HistoricalFinaMainData.GetAvgRevenueIncreasingRatioByYear(ctx, thisYear-1)
		// nil fix: 新的一年刚开始时，上一年的年报还没披露，年份数据全部-1，保证有数据返回
		if lastYearReport == nil {
			logging.Debug(ctx, "NewStock get last year report nil, use before last year report")
			lastYearReport = beforeLastYearReport
			beforeLastYearReport = s.HistoricalFinaMainData.GetReport(ctx, time.Now().Year()-3, eastmoney.FinaReportTypeYear)
			thisYearAvgRevIncrRatio = s.HistoricalFinaMainData.GetAvgRevenueIncreasingRatioByYear(ctx, thisYear-1)
			lastYearAvgRevIncrRatio = s.HistoricalFinaMainData.GetAvgRevenueIncreasingRatioByYear(ctx, thisYear-2)
		}
		// pe 中位数
		peMidVal, err := peList.GetMidValue(ctx)
		if err != nil {
			logging.Error(ctx, "NewStock GetMidValue err:"+err.Error())
			return
		}

		// 改进的合理价计算
		// 1. 使用多年EPS平均值，避免单年爆发增长的影响
		epsHistory := s.HistoricalFinaMainData.ValueList(ctx, eastmoney.ValueListTypeEPS, 3, eastmoney.FinaReportTypeYear)
		var baseEPS float64
		if len(epsHistory) >= 3 {
			// 使用近3年EPS的平均值作为基准
			sum := 0.0
			for _, eps := range epsHistory {
				sum += eps
			}
			baseEPS = sum / float64(len(epsHistory))
			logging.Debugf(ctx, "Using 3-year average EPS: %v, history: %v", baseEPS, epsHistory)
		} else {
			// 数据不足时使用去年EPS
			baseEPS = lastYearReport.Epsjb
			logging.Debugf(ctx, "Using last year EPS: %v", baseEPS)
		}

		// 2. 对增长率进行上限限制，避免过度乐观
		adjustedGrowthRatio := thisYearAvgRevIncrRatio
		const maxGrowthRate = 50.0 // 最大增长率限制为50%
		if adjustedGrowthRatio > maxGrowthRate {
			adjustedGrowthRatio = maxGrowthRate
			logging.Debugf(ctx, "Growth rate capped from %v%% to %v%%", thisYearAvgRevIncrRatio, maxGrowthRate)
		}

		// 3. 检测爆发增长并进行调整
		var explosiveGrowthAdjustment float64 = 1.0
		if len(epsHistory) >= 2 {
			// 计算去年相对于前年的EPS增长率
			lastYearGrowth := (epsHistory[0] - epsHistory[1]) / epsHistory[1] * 100
			if lastYearGrowth > 100 { // 如果去年增长超过100%，认为是爆发增长
				// 对爆发增长进行折扣处理
				explosiveGrowthAdjustment = 0.7 // 70%的折扣
				logging.Debugf(ctx, "Explosive growth detected: %v%%, applying adjustment factor: %v", lastYearGrowth, explosiveGrowthAdjustment)
			}
		}

		// 4. 计算改进后的合理价
		s.RightPrice = peMidVal * (baseEPS * (1 + adjustedGrowthRatio/100.0)) * explosiveGrowthAdjustment
		s.PriceSpace = (s.RightPrice - price) / price * 100

		// 5. 计算去年的合理价（用于验证算法准确性）
		var lastYearBaseEPS float64
		if len(epsHistory) >= 3 {
			// 使用前年和大前年的EPS平均值
			lastYearBaseEPS = (epsHistory[1] + epsHistory[2]) / 2.0
		} else {
			lastYearBaseEPS = beforeLastYearReport.Epsjb
		}

		lastYearAdjustedGrowthRatio := lastYearAvgRevIncrRatio
		if lastYearAdjustedGrowthRatio > maxGrowthRate {
			lastYearAdjustedGrowthRatio = maxGrowthRate
		}

		s.LastYearRightPrice = peMidVal * (lastYearBaseEPS * (1 + lastYearAdjustedGrowthRatio/100.0))

		// 6. 异常值检测和日志记录
		if math.IsNaN(s.RightPrice) || math.IsInf(s.RightPrice, 0) || s.RightPrice <= 0 {
			logging.Warnf(ctx, "Invalid RightPrice calculated: %v, using fallback calculation", s.RightPrice)
			// 回退到简单计算
			s.RightPrice = peMidVal * lastYearReport.Epsjb
			s.PriceSpace = (s.RightPrice - price) / price * 100
		}

		logging.Debugf(ctx, "RightPrice calculation - BaseEPS: %v, AdjustedGrowth: %v%%, ExplosiveAdjustment: %v, FinalPrice: %v",
			baseEPS, adjustedGrowthRatio, explosiveGrowthAdjustment, s.RightPrice)
	}(ctx, &s)

	// 获取综合估值
	wg.Add(1)
	go func(ctx context.Context, s *Stock) {
		defer wg.Done()
		valMap, err := datacenter.EastMoney.QueryValuationStatus(ctx, s.BaseInfo.Secucode)
		if err != nil {
			logging.Error(ctx, "NewStock QueryValuationStatus err:"+err.Error())
			return
		}
		s.ValuationMap = valMap
	}(ctx, &s)

	// 历史股价 && 波动率
	wg.Add(1)
	go func(ctx context.Context, s *Stock) {
		defer wg.Done()
		hisPrice, err := datacenter.Eniu.QueryHistoricalStockPrice(ctx, s.BaseInfo.Secucode)
		if err != nil {
			logging.Error(ctx, "NewStock QueryHistoricalStockPrice err:"+err.Error())
			return
		}
		s.HistoricalPrice = hisPrice

		// 历史波动率
		hv, err := hisPrice.HistoricalVolatility(ctx, "YEAR")
		if err != nil {
			logging.Error(ctx, "NewStock HistoricalVolatility err:"+err.Error())
			return
		}
		s.HistoricalVolatility = hv
	}(ctx, &s)

	// 公司资料
	wg.Add(1)
	go func(ctx context.Context, s *Stock) {
		defer wg.Done()
		cp, err := datacenter.EastMoney.QueryCompanyProfile(ctx, s.BaseInfo.Secucode)
		if err != nil {
			logging.Error(ctx, "NewStock QueryCompanyProfile err:"+err.Error())
			return
		}
		s.CompanyProfile = cp
	}(ctx, &s)

	// 最新财报预约披露时间
	wg.Add(1)
	go func(ctx context.Context, s *Stock) {
		defer wg.Done()
		finaPubDateList, err := datacenter.EastMoney.QueryFinaPublishDateList(ctx, s.BaseInfo.SecurityCode)
		if err != nil {
			logging.Error(ctx, "NewStock QueryFinaPublishDateList err:"+err.Error())
			return
		}
		if len(finaPubDateList) > 0 {
			s.FinaAppointPublishDate = finaPubDateList[0].AppointPublishDate
			s.FinaActualPublishDate = finaPubDateList[0].ActualPublishDate
			s.FinaReportDate = finaPubDateList[0].ReportDate
		}
	}(ctx, &s)

	// 机构评级统计
	wg.Add(1)
	go func(ctx context.Context, s *Stock) {
		defer wg.Done()
		orgRatings, err := datacenter.EastMoney.QueryOrgRating(ctx, s.BaseInfo.Secucode)
		if err != nil {
			logging.Debug(ctx, "NewStock QueryOrgRating err:"+err.Error())
			return
		}
		s.OrgRatingList = orgRatings
	}(ctx, &s)

	// 盈利预测
	wg.Add(1)
	go func(ctx context.Context, s *Stock) {
		defer wg.Done()
		pps, err := datacenter.EastMoney.QueryProfitPredict(ctx, s.BaseInfo.Secucode)
		if err != nil {
			logging.Debug(ctx, "NewStock QueryProfitPredict err:"+err.Error())
			return
		}
		s.ProfitPredictList = pps
	}(ctx, &s)

	// 价值评估
	wg.Add(1)
	go func(ctx context.Context, s *Stock) {
		defer wg.Done()
		jzpg, err := datacenter.EastMoney.QueryJiaZhiPingGu(ctx, s.BaseInfo.Secucode)
		if err != nil {
			logging.Debug(ctx, "NewStock QueryJiaZhiPingGu err:"+err.Error())
			return
		}
		s.JZPG = jzpg
	}(ctx, &s)

	// 利润表数据
	wg.Add(1)
	go func(ctx context.Context, s *Stock) {
		defer wg.Done()
		gincomeList, err := datacenter.EastMoney.QueryFinaGincomeData(ctx, s.BaseInfo.Secucode)
		if err != nil {
			logging.Error(ctx, "NewStock QueryFinaGincomeData err:"+err.Error())
			return
		}
		s.HistoricalGincomeList = gincomeList
		if len(s.HistoricalGincomeList) > 0 {
			// 本业营收比
			gincome := s.HistoricalGincomeList[0]
			s.BYYSRatio = gincome.OperateProfit / (gincome.OperateProfit + gincome.NonbusinessIncome)
			// 审计意见
			s.FinaReportOpinion = gincome.OpinionType
		}
	}(ctx, &s)

	// 现金流量表数据
	wg.Add(1)
	go func(ctx context.Context, s *Stock) {
		defer wg.Done()
		cashflow, err := datacenter.EastMoney.QueryFinaCashflowData(ctx, s.BaseInfo.Secucode)
		if err != nil {
			logging.Error(ctx, "NewStock QueryFinaCashflowData err:"+err.Error())
			return
		}
		s.HistoricalCashflowList = cashflow
		if len(s.HistoricalCashflowList) > 0 {
			cf := s.HistoricalCashflowList[0]
			s.NetcashOperate = cf.NetcashOperate
			s.NetcashInvest = cf.NetcashInvest
			s.NetcashFinance = cf.NetcashFinance
			if cf.NetcashInvest < 0 {
				s.NetcashFree = s.NetcashOperate + s.NetcashInvest
			} else {
				s.NetcashFree = s.NetcashOperate - s.NetcashInvest
			}
		}
	}(ctx, &s)

	// 获取前10大流通股东
	wg.Add(1)
	go func(ctx context.Context, s *Stock) {
		defer wg.Done()
		holders, err := datacenter.EastMoney.QueryFreeHolders(ctx, s.BaseInfo.Secucode)
		if err != nil {
			logging.Error(ctx, "NewStock QueryFreeHolders err:"+err.Error())
			return
		}
		s.FreeHoldersTop10 = holders
	}(ctx, &s)

	// 获取最近60日的主力资金净流入
	wg.Add(1)
	go func(ctx context.Context, s *Stock) {
		defer wg.Done()
		now := time.Now()
		end := now.Format("2006-01-02")
		d, _ := time.ParseDuration("-1440h")
		start := now.Add(d).Format("2006-01-02")
		inflows, err := datacenter.Zszx.QueryMainMoneyNetInflows(ctx, s.BaseInfo.Secucode, start, end)
		if err != nil {
			logging.Error(ctx, "NewStock QueryMainMoneyNetInflows err:"+err.Error())
			return
		}
		s.MainMoneyNetInflows = inflows
	}(ctx, &s)

	// 等待所有goroutine完成
	wg.Wait()

	// 计算巴菲特评分
	s.BuffettScore = s.calculateBuffettScore(ctx)

	return s, nil
}

// calculateBuffettScore 计算巴菲特评分
func (s *Stock) calculateBuffettScore(ctx context.Context) BuffettScore {
	// 1. ROE评分（20分）
	s.calculateROEScore(ctx)

	// 2. 现金流评分（15分）
	s.calculateCashFlowScore(ctx)

	// 3. 利润增长评分（15分）
	s.calculateProfitGrowthScore(ctx)

	// 4. 负债率评分（10分）
	s.calculateDebtRatioScore(ctx)

	// 5. 护城河评分（10分）
	s.calculateMoatScore(ctx)

	// 6. 管理层评分（10分）
	s.calculateManagementScore(ctx)

	// 7. 估值评分（15分）
	s.calculateValuationScore(ctx)

	// 8. 研发投入评分（5分）
	s.calculateRDScore(ctx)

	// 9. 分红评分（5分）
	s.calculateDividendScore(ctx)

	// 10. 回购评分（5分）
	s.calculateRepurchaseScore(ctx)

	// 计算总分
	totalScore := s.BuffettScore.ROEScore +
		s.BuffettScore.CashFlowScore +
		s.BuffettScore.ProfitGrowthScore +
		s.BuffettScore.DebtRatioScore +
		s.BuffettScore.MoatScore +
		s.BuffettScore.ManagementScore +
		s.BuffettScore.ValuationScore +
		s.BuffettScore.RDScore +
		s.BuffettScore.DividendScore +
		s.BuffettScore.RepurchaseScore

	// 生成评分说明
	var desc strings.Builder
	desc.WriteString(fmt.Sprintf("总分(100分): %.1f\n", totalScore))
	desc.WriteString(fmt.Sprintf("ROE(20分): %.1f\n", s.BuffettScore.ROEScore))
	desc.WriteString(fmt.Sprintf("现金流(15分): %.1f\n", s.BuffettScore.CashFlowScore))
	desc.WriteString(fmt.Sprintf("利润增长(15分): %.1f\n", s.BuffettScore.ProfitGrowthScore))
	desc.WriteString(fmt.Sprintf("负债率(10分): %.1f\n", s.BuffettScore.DebtRatioScore))
	desc.WriteString(fmt.Sprintf("护城河(10分): %.1f\n", s.BuffettScore.MoatScore))
	desc.WriteString(fmt.Sprintf("管理层(10分): %.1f\n", s.BuffettScore.ManagementScore))
	desc.WriteString(fmt.Sprintf("估值(15分): %.1f - PE: %.1f, PEG: %.1f\n", s.BuffettScore.ValuationScore, s.BaseInfo.PE, s.PEG))
	desc.WriteString(fmt.Sprintf("研发投入(5分): %.1f\n", s.BuffettScore.RDScore))
	desc.WriteString(fmt.Sprintf("分红(5分): %.1f\n", s.BuffettScore.DividendScore))
	desc.WriteString(fmt.Sprintf("回购(5分): %.1f\n", s.BuffettScore.RepurchaseScore))

	s.BuffettScore.ScoreDescription = desc.String()
	s.BuffettScore.TotalScore = totalScore
	return s.BuffettScore
}

// calculateROEScore 计算ROE评分
func (s *Stock) calculateROEScore(ctx context.Context) {
	logging.Infof(ctx, "开始计算ROE评分，HistoricalFinaMainData长度: %d", len(s.HistoricalFinaMainData))

	// 获取近5年ROE数据
	roeList := s.HistoricalFinaMainData.ValueList(ctx, eastmoney.ValueListTypeROE, 5, eastmoney.FinaReportTypeYear)
	logging.Infof(ctx, "获取到的ROE列表长度: %d, 数据: %+v", len(roeList), roeList)

	if len(roeList) == 0 {
		logging.Info(ctx, "ROE评分: ROE列表为空，得分0分")
		s.BuffettScore.ROEScore = 0
		return
	}

	if len(roeList) < 5 {
		logging.Infof(ctx, "ROE评分: ROE列表长度(%d)小于5，得分0分", len(roeList))
		s.BuffettScore.ROEScore = 0
		return
	}

	// 计算平均ROE和波动率
	var sumROE float64
	for _, roe := range roeList {
		sumROE += roe
	}
	avgROE := sumROE / float64(len(roeList))
	logging.Infof(ctx, "ROE评分: 平均ROE: %.2f%%", avgROE)

	// 计算ROE波动率
	var variance float64
	for _, roe := range roeList {
		variance += math.Pow(roe-avgROE, 2)
	}
	volatility := math.Sqrt(variance/float64(len(roeList))) / avgROE
	logging.Infof(ctx, "ROE评分: ROE波动率: %.2f", volatility)

	// 根据ROE和波动率评分
	score := 0.0
	if avgROE >= 20 {
		score = 20
		logging.Info(ctx, "ROE评分: 平均ROE>=20，得20分")
	} else if avgROE >= 15 {
		score = 15
		logging.Info(ctx, "ROE评分: 平均ROE>=15，得15分")
	} else {
		score = (avgROE / 15) * 15
		logging.Infof(ctx, "ROE评分: 平均ROE<15，按比例得%.2f分", score)
	}

	// 根据波动率扣分
	if volatility > 0.3 {
		score *= 0.8 // 波动大扣20%分数
		logging.Infof(ctx, "ROE评分: 波动率>0.3，扣除20%%分数，最终得分%.2f分", score)
	} else {
		logging.Infof(ctx, "ROE评分: 波动率<=0.3，不扣分，最终得分%.2f分", score)
	}

	s.BuffettScore.ROEScore = score
	logging.Infof(ctx, "ROE评分计算完成，最终得分: %.2f分", score)
}

// calculateCashFlowScore 计算现金流评分
func (s *Stock) calculateCashFlowScore(ctx context.Context) {
	if len(s.HistoricalCashflowList) < 3 {
		s.BuffettScore.CashFlowScore = 0
		return
	}

	positiveCount := 0
	for i := 0; i < 3 && i < len(s.HistoricalCashflowList); i++ {
		if s.HistoricalCashflowList[i].NetcashOperate > 0 {
			positiveCount++
		}
	}

	switch positiveCount {
	case 3:
		s.BuffettScore.CashFlowScore = 15
	case 2:
		s.BuffettScore.CashFlowScore = 10
	case 1:
		s.BuffettScore.CashFlowScore = 5
	default:
		s.BuffettScore.CashFlowScore = 0
	}
}

// calculateProfitGrowthScore 计算利润增长评分
func (s *Stock) calculateProfitGrowthScore(ctx context.Context) {
	if len(s.HistoricalFinaMainData) < 5 {
		s.BuffettScore.ProfitGrowthScore = 0
		return
	}

	// 获取近5年净利润数据
	profitList := s.HistoricalFinaMainData.ValueList(ctx, eastmoney.ValueListTypeNetProfit, 5, eastmoney.FinaReportTypeYear)
	if len(profitList) < 5 {
		s.BuffettScore.ProfitGrowthScore = 0
		return
	}

	// 计算逐年增长率
	growthCount := 0
	volatilitySum := 0.0
	for i := 0; i < len(profitList)-1; i++ {
		if profitList[i] > profitList[i+1] {
			growthCount++
		}
		if i > 0 {
			// 计算增长率波动
			growth1 := (profitList[i] - profitList[i+1]) / math.Abs(profitList[i+1])
			growth2 := (profitList[i-1] - profitList[i]) / math.Abs(profitList[i])
			volatilitySum += math.Abs(growth1 - growth2)
		}
	}

	// 根据增长次数和波动性评分
	score := float64(growthCount) * 3
	if volatilitySum > 0.5 {
		score *= 0.8 // 波动大扣20%分数
	}

	s.BuffettScore.ProfitGrowthScore = score
}

// calculateDebtRatioScore 计算负债率评分
func (s *Stock) calculateDebtRatioScore(ctx context.Context) {
	if len(s.HistoricalFinaMainData) == 0 {
		s.BuffettScore.DebtRatioScore = 0
		return
	}

	// 获取最新负债率
	debtRatio := s.HistoricalFinaMainData[0].Zcfzl

	switch {
	case debtRatio < 30:
		s.BuffettScore.DebtRatioScore = 10
	case debtRatio < 50:
		s.BuffettScore.DebtRatioScore = 8
	case debtRatio < 70:
		s.BuffettScore.DebtRatioScore = 5
	default:
		s.BuffettScore.DebtRatioScore = 0
	}
}

// calculateValuationScore 计算估值评分
func (s *Stock) calculateValuationScore(ctx context.Context) {
	score := 0.0

	// PE估值评分
	switch {
	case s.BaseInfo.PE < 10:
		score = 15
	case s.BaseInfo.PE < 15:
		score = 12
	case s.BaseInfo.PE < 20:
		score = 8
	case s.BaseInfo.PE < 30:
		score = 5
	default:
		score = 0
	}

	// PEG估值加分
	if s.PEG > 0 && s.PEG < 1 {
		score = math.Max(score, 15) // PEG<1时至少得12分
	}

	s.BuffettScore.ValuationScore = score
}

// calculateMoatScore 计算护城河评分
func (s *Stock) calculateMoatScore(ctx context.Context) {
	// 默认给5分
	score := 5.0

	// 基于行业给分
	industry := s.BaseInfo.Industry
	switch industry {
	case "食品饮料", "医药生物", "家用电器", "银行", "保险":
		score = 8 // 这些行业通常有较强的护城河
	case "建筑", "采掘", "农林牧渔":
		score = 3 // 这些行业通常护城河较弱
	}

	s.BuffettScore.MoatScore = math.Min(10, score) // 最高10分
}

// calculateManagementScore 计算管理层评分
func (s *Stock) calculateManagementScore(ctx context.Context) {
	score := 5.0 // 基础分

	// 检查分红情况
	if len(s.HistoricalFinaMainData) >= 3 {
		// 暂时固定为7.5分,因为缺少分红数据
		score = 7.5
	}

	// 检查股份回购情况
	// 暂时固定为7.5分,因为缺少回购数据
	score = 7.5

	s.BuffettScore.ManagementScore = math.Min(10, score) // 最高10分
}

// calculateRDScore 计算研发投入评分
func (s *Stock) calculateRDScore(ctx context.Context) {
	// 暂时固定为5分,因为缺少研发投入数据
	s.BuffettScore.RDScore = 5.0
}

// calculateDividendScore 计算分红评分
func (s *Stock) calculateDividendScore(ctx context.Context) {
	// 暂时固定为5分,因为缺少分红数据
	s.BuffettScore.DividendScore = 5.0
}

// calculateRepurchaseScore 计算回购评分
func (s *Stock) calculateRepurchaseScore(ctx context.Context) {
	// 暂时固定为5分,因为缺少回购数据
	s.BuffettScore.RepurchaseScore = 5.0
}
