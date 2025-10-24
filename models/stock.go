// 股票对象封装

package models

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/axiaoxin-com/investool/datacenter"
	"github.com/axiaoxin-com/investool/datacenter/coze"
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

// LynchScore 彼得·林奇评分结构体（总分100分）
type LynchScore struct {
	PEGScore           float64 `json:"peg_score"`            // PEG评分（25分）
	EPSGrowthScore     float64 `json:"eps_growth_score"`     // EPS持续增长评分（15分）
	RevenueGrowthScore float64 `json:"revenue_growth_score"` // 营收增长评分（15分）
	ProfitGrowthScore  float64 `json:"profit_growth_score"`  // 净利润增速评分（10分）
	ROEScore           float64 `json:"roe_score"`            // ROE评分（10分）
	FreeCashFlowScore  float64 `json:"free_cash_flow_score"` // 自由现金流评分（10分）
	IndustryScore      float64 `json:"industry_score"`       // 行业前景评分（10分）
	MarketCapScore     float64 `json:"market_cap_score"`     // 市值评分（5分）
	TotalScore         float64 `json:"total_score"`          // 总分（100分）
	ScoreDescription   string  `json:"score_description"`    // 评分说明
}

// ONeilScore 威廉·欧奈尔（CANSLIM）评分结构体（总分100分）
type ONeilScore struct {
	CurrentQuarterScore float64 `json:"current_quarter_score"` // C：当前季度利润增长评分（15分）
	AnnualGrowthScore   float64 `json:"annual_growth_score"`   // A：年利润增长趋势评分（15分）
	NewConceptScore     float64 `json:"new_concept_score"`     // N：新品新概念评分（10分）
	SmallFloatScore     float64 `json:"small_float_score"`     // S：股本小易拉升评分（10分）
	LeaderScore         float64 `json:"leader_score"`          // L：行业龙头评分（10分）
	InstitutionScore    float64 `json:"institution_score"`     // I：机构增持评分（20分）
	MarketTrendScore    float64 `json:"market_trend_score"`    // M：技术趋势评分（20分）
	TotalScore          float64 `json:"total_score"`           // 总分（100分）
	ScoreDescription    string  `json:"score_description"`     // 评分说明
}

// LLMAnalysis LLM分析结果结构体
type LLMAnalysis struct {
	IndustryLeaderScore   float64 `json:"industry_leader_score"`   // 行业龙头评分
	NewConceptScore       float64 `json:"new_concept_score"`       // 新概念评分
	IndustryProspectScore float64 `json:"industry_prospect_score"` // 行业前景评分
	MoatScore             float64 `json:"moat_score"`              // 护城河评分
	ManagementScore       float64 `json:"management_score"`        // 管理层评分
	RepurchaseScore       float64 `json:"repurchase_score"`        // 回购评分
	InstitutionScore      float64 `json:"institution_score"`       // 机构增持评分
	TechnicalTrendScore   float64 `json:"technical_trend_score"`   // 技术趋势评分
	Analysis              string  `json:"analysis"`                // 分析说明
	DataSource            string  `json:"data_source"`             // 数据来源
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
	// 主营业务
	MainBusiness string `json:"main_business"`
	// 所属概念
	Concept string `json:"concept"`
	// 巴菲特评分
	BuffettScore BuffettScore `json:"buffett_score"`
	// 彼得·林奇评分
	LynchScore LynchScore `json:"lynch_score"`
	// 威廉·欧奈尔评分
	ONeilScore ONeilScore `json:"oneil_score"`
	// Coze AI分析结果
	CozeAnalysis *coze.IndustryAnalysisResponse `json:"coze_analysis,omitempty"`
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
		peMidVal, err := s.HistoricalPEList.GetMidValue(ctx)
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
	rawTotalScore := s.BuffettScore.ROEScore +
		s.BuffettScore.CashFlowScore +
		s.BuffettScore.ProfitGrowthScore +
		s.BuffettScore.DebtRatioScore +
		s.BuffettScore.MoatScore +
		s.BuffettScore.ManagementScore +
		s.BuffettScore.ValuationScore +
		s.BuffettScore.RDScore +
		s.BuffettScore.DividendScore +
		s.BuffettScore.RepurchaseScore

	// 将110分制换算成100分制
	totalScore := rawTotalScore * 100 / 110

	// 生成评分说明
	var desc strings.Builder
	desc.WriteString(fmt.Sprintf("总分(100分): %.1f (原始得分: %.1f)\n\n", totalScore, rawTotalScore))
	desc.WriteString(fmt.Sprintf("ROE(20分): %.1f\n", s.BuffettScore.ROEScore))
	desc.WriteString(fmt.Sprintf("现金流(15分): %.1f\n", s.BuffettScore.CashFlowScore))
	desc.WriteString(fmt.Sprintf("利润增长(15分): %.1f\n", s.BuffettScore.ProfitGrowthScore))
	desc.WriteString(fmt.Sprintf("负债率(10分): %.1f\n", s.BuffettScore.DebtRatioScore))
	desc.WriteString(fmt.Sprintf("护城河(10分): %.1f\n", s.BuffettScore.MoatScore))
	desc.WriteString(fmt.Sprintf("管理层(10分): %.1f\n", s.BuffettScore.ManagementScore))
	desc.WriteString(fmt.Sprintf("估值(15分): %.1f\n", s.BuffettScore.ValuationScore))
	desc.WriteString(fmt.Sprintf("  PE: %.1f\n", s.BaseInfo.PE))
	desc.WriteString(fmt.Sprintf("  PEG: %.1f\n\n", s.PEG))
	desc.WriteString(fmt.Sprintf("研发投入(5分): %.1f\n", s.BuffettScore.RDScore))
	desc.WriteString(fmt.Sprintf("分红(5分): %.1f\n", s.BuffettScore.DividendScore))
	desc.WriteString(fmt.Sprintf("回购(5分): %.1f", s.BuffettScore.RepurchaseScore))

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

	operatePositiveCount := 0
	freePositiveCount := 0
	for i := 0; i < 3 && i < len(s.HistoricalCashflowList); i++ {
		cf := s.HistoricalCashflowList[i]
		if cf.NetcashOperate > 0 {
			operatePositiveCount++
		}
		// 计算自由现金流
		var freeCashFlow float64
		if cf.NetcashInvest < 0 {
			freeCashFlow = cf.NetcashOperate + cf.NetcashInvest
		} else {
			freeCashFlow = cf.NetcashOperate - cf.NetcashInvest
		}
		if freeCashFlow > 0 {
			freePositiveCount++
		}
	}

	// 经营现金流和自由现金流各占一半分数
	operateScore := 0.0
	switch operatePositiveCount {
	case 3:
		operateScore = 7.5
	case 2:
		operateScore = 5.0
	case 1:
		operateScore = 2.5
	}

	freeScore := 0.0
	switch freePositiveCount {
	case 3:
		freeScore = 7.5
	case 2:
		freeScore = 5.0
	case 1:
		freeScore = 2.5
	}

	s.BuffettScore.CashFlowScore = operateScore + freeScore
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
	if len(s.HistoricalGincomeList) == 0 {
		s.BuffettScore.RDScore = 0
		return
	}

	// 获取最近三年的研发投入数据
	var rdExpenses []float64
	var revenues []float64
	count := 0

	for i := 0; i < len(s.HistoricalGincomeList) && count < 3; i++ {
		// 只统计年报数据
		if s.HistoricalGincomeList[i].ReportType != eastmoney.FinaReportTypeYear {
			continue
		}
		rdExpenses = append(rdExpenses, s.HistoricalGincomeList[i].ResearchExpense)
		revenues = append(revenues, s.HistoricalGincomeList[i].TotalOperateIncome)
		count++
	}

	if len(rdExpenses) == 0 {
		s.BuffettScore.RDScore = 0
		return
	}

	// 1. 计算研发投入占营收比例（最近一年）
	rdRatio := rdExpenses[0] / revenues[0] * 100

	// 2. 计算研发投入增长率
	var rdGrowth float64
	if len(rdExpenses) >= 2 {
		rdGrowth = (rdExpenses[0] - rdExpenses[1]) / math.Abs(rdExpenses[1]) * 100
	}

	// 3. 评分规则：
	// - 研发投入占营收比例 >= 5%: 3分
	// - 研发投入占营收比例 >= 3%: 2分
	// - 研发投入占营收比例 >= 1%: 1分
	// - 研发投入同比增长 >= 30%: 2分
	// - 研发投入同比增长 >= 10%: 1分
	score := 0.0

	if rdRatio >= 5 {
		score += 3
	} else if rdRatio >= 3 {
		score += 2
	} else if rdRatio >= 1 {
		score += 1
	}

	if rdGrowth >= 30 {
		score += 2
	} else if rdGrowth >= 10 {
		score += 1
	}

	s.BuffettScore.RDScore = score
	logging.Infof(ctx, "[%s] 研发投入评分：%.1f分 (投入占比:%.2f%%, 同比增长:%.2f%%)",
		s.BaseInfo.SecurityNameAbbr, score, rdRatio, rdGrowth)
}

// calculateDividendScore 计算分红评分
func (s *Stock) calculateDividendScore(ctx context.Context) {
	if len(s.HistoricalCashflowList) == 0 {
		s.BuffettScore.DividendScore = 0
		return
	}

	// 获取最近三年的分红数据
	var dividends []float64
	var profits []float64
	count := 0

	for i := 0; i < len(s.HistoricalCashflowList) && count < 3; i++ {
		// 只统计年报数据
		if s.HistoricalCashflowList[i].ReportType != eastmoney.FinaReportTypeYear {
			continue
		}
		dividends = append(dividends, s.HistoricalCashflowList[i].AssignDividendPorfit)
		// 从对应的利润表中获取净利润数据
		if i < len(s.HistoricalGincomeList) {
			profits = append(profits, s.HistoricalGincomeList[i].ParentNetprofit)
		}
		count++
	}

	if len(dividends) == 0 || len(profits) == 0 {
		s.BuffettScore.DividendScore = 0
		return
	}

	// 1. 计算最近一年的分红率
	dividendRatio := dividends[0] / profits[0] * 100

	// 2. 计算连续分红年数
	continuousYears := 0
	for _, dividend := range dividends {
		if dividend <= 0 {
			break
		}
		continuousYears++
	}

	// 3. 评分规则：
	// - 分红率 >= 50%: 3分
	// - 分红率 >= 30%: 2分
	// - 分红率 >= 10%: 1分
	// - 连续分红3年: 2分
	// - 连续分红2年: 1分
	score := 0.0

	if dividendRatio >= 50 {
		score += 3
	} else if dividendRatio >= 30 {
		score += 2
	} else if dividendRatio >= 10 {
		score += 1
	}

	if continuousYears >= 3 {
		score += 2
	} else if continuousYears >= 2 {
		score += 1
	}

	s.BuffettScore.DividendScore = score
	logging.Infof(ctx, "[%s] 分红评分：%.1f分 (分红率:%.2f%%, 连续分红年数:%d)",
		s.BaseInfo.SecurityNameAbbr, score, dividendRatio, continuousYears)
}

// calculateRepurchaseScore 计算回购评分
func (s *Stock) calculateRepurchaseScore(ctx context.Context) {
	// 暂时固定为5分,因为缺少回购数据
	s.BuffettScore.RepurchaseScore = 5.0
}

func (s *Stock) String() string {
	var sb strings.Builder

	rawTotalScore := s.BuffettScore.ROEScore +
		s.BuffettScore.CashFlowScore +
		s.BuffettScore.ProfitGrowthScore +
		s.BuffettScore.DebtRatioScore +
		s.BuffettScore.MoatScore +
		s.BuffettScore.ManagementScore +
		s.BuffettScore.ValuationScore +
		s.BuffettScore.RDScore +
		s.BuffettScore.DividendScore +
		s.BuffettScore.RepurchaseScore

	sb.WriteString(fmt.Sprintf("总分: %.1f分 (原始得分: %.1f)\n", s.BuffettScore.TotalScore, rawTotalScore))
	sb.WriteString(fmt.Sprintf("总分(100分): %.1f (原始得分: %.1f)\n", s.BuffettScore.TotalScore, rawTotalScore))
	sb.WriteString(fmt.Sprintf("ROE(20分): %.1f\n", s.BuffettScore.ROEScore))
	sb.WriteString(fmt.Sprintf("现金流(15分): %.1f\n", s.BuffettScore.CashFlowScore))
	sb.WriteString(fmt.Sprintf("利润增长(15分): %.1f\n", s.BuffettScore.ProfitGrowthScore))
	sb.WriteString(fmt.Sprintf("负债率(10分): %.1f\n", s.BuffettScore.DebtRatioScore))
	sb.WriteString(fmt.Sprintf("护城河(10分): %.1f\n", s.BuffettScore.MoatScore))
	sb.WriteString(fmt.Sprintf("管理层(10分): %.1f\n", s.BuffettScore.ManagementScore))
	sb.WriteString(fmt.Sprintf("估值(15分): %.1f\n", s.BuffettScore.ValuationScore))
	sb.WriteString(fmt.Sprintf("PE: %.1f  PEG: %.1f\n", s.BaseInfo.PE, s.PEG))
	sb.WriteString(fmt.Sprintf("研发投入(5分): %.1f\n", s.BuffettScore.RDScore))
	sb.WriteString(fmt.Sprintf("分红(5分): %.1f\n", s.BuffettScore.DividendScore))
	sb.WriteString(fmt.Sprintf("回购(5分): %.1f\n", s.BuffettScore.RepurchaseScore))

	return sb.String()
}

// GetCozeAnalysis 获取Coze AI分析结果
func (s Stock) GetCozeAnalysis(ctx context.Context, cozeClient *coze.CozeClient) error {
	if cozeClient == nil {
		return fmt.Errorf("coze client is nil")
	}

	// 构建分析请求
	req := coze.IndustryAnalysisRequest{
		StockName:    s.BaseInfo.SecurityNameAbbr,
		Industry:     s.BaseInfo.Industry,
		MarketCap:    s.BaseInfo.TotalMarketCap / 100000000, // 转换为亿元
		MainBusiness: s.MainBusiness,
		Concept:      s.Concept,
	}

	// 调用Coze API
	analysis, err := cozeClient.GetIndustryAnalysis(ctx, req)
	if err != nil {
		logging.Warnf(ctx, "Failed to get coze analysis: %v", err)
		// Coze分析失败不影响主要功能，设置默认值
		s.CozeAnalysis = &coze.IndustryAnalysisResponse{
			IndustryProspectScore: 5.0,
			IndustryLeaderScore:   5.0,
			NewConceptScore:       5.0,
			Analysis:              "Coze AI 分析暂时不可用，使用默认评分",
			DataSource:            "系统默认值",
		}
		return nil
	}

	// 更新分析结果
	s.CozeAnalysis = analysis
	return nil
}

// CalculateLynchScoreWithCoze 计算彼得·林奇评分（包含Coze分析）
func (s Stock) CalculateLynchScore() LynchScore {
	score := LynchScore{}

	// 1. PEG评分（25分）
	peg := s.PEG
	if peg <= 1.0 {
		score.PEGScore = 25.0
	} else if peg <= 1.5 {
		score.PEGScore = 18.0
	} else if peg <= 2.0 {
		score.PEGScore = 10.0
	} else {
		score.PEGScore = 0.0
	}

	// 2. EPS持续增长评分（15分）- 现在有历史数据了！
	if len(s.HistoricalFinaMainData) >= 5 {
		// 检查最近5年EPS是否持续增长
		growthCount := 0
		for i := 1; i < 5 && i < len(s.HistoricalFinaMainData); i++ {
			if s.HistoricalFinaMainData[i].Epsjb > s.HistoricalFinaMainData[i-1].Epsjb {
				growthCount++
			}
		}
		if growthCount >= 4 {
			score.EPSGrowthScore = 15.0 // 5年持续增长
		} else if growthCount >= 3 {
			score.EPSGrowthScore = 12.0 // 4年增长
		} else if growthCount >= 2 {
			score.EPSGrowthScore = 8.0 // 3年增长
		} else {
			score.EPSGrowthScore = 4.0 // 有波动
		}
	} else if len(s.HistoricalFinaMainData) >= 3 {
		// 至少3年数据
		growthCount := 0
		for i := 1; i < 3 && i < len(s.HistoricalFinaMainData); i++ {
			if s.HistoricalFinaMainData[i].Epsjb > s.HistoricalFinaMainData[i-1].Epsjb {
				growthCount++
			}
		}
		score.EPSGrowthScore = float64(growthCount) / 2.0 * 15.0
	} else {
		score.EPSGrowthScore = 0.0 // 数据不足
	}

	// 3. 营收增长评分（15分）
	revenueGrowth := s.BaseInfo.ToiYoyRatio
	if revenueGrowth >= 30.0 {
		score.RevenueGrowthScore = 15.0
	} else if revenueGrowth >= 15.0 {
		score.RevenueGrowthScore = 10.0
	} else if revenueGrowth > 0 {
		score.RevenueGrowthScore = float64(revenueGrowth) / 30.0 * 15.0
	} else {
		score.RevenueGrowthScore = 0.0
	}

	// 4. 净利润增速评分（10分）
	profitGrowth := s.BaseInfo.NetprofitYoyRatio
	if profitGrowth >= 20.0 {
		score.ProfitGrowthScore = 10.0
	} else if profitGrowth >= 10.0 {
		score.ProfitGrowthScore = 6.0 + (profitGrowth-10.0)/10.0*4.0
	} else if profitGrowth > 0 {
		score.ProfitGrowthScore = 2.0 + (profitGrowth/10.0)*4.0
	} else {
		score.ProfitGrowthScore = 0.0
	}

	// 5. ROE评分（10分）
	roe := s.BaseInfo.RoeWeight
	if roe >= 20.0 {
		score.ROEScore = 10.0
	} else if roe >= 15.0 {
		score.ROEScore = 8.0
	} else if roe > 0 {
		score.ROEScore = (roe / 20.0) * 10.0
	} else {
		score.ROEScore = 0.0
	}

	// 6. 自由现金流评分（10分）- 现在有历史数据了！
	if len(s.HistoricalCashflowList) >= 3 {
		// 检查最近3年经营活动现金流是否为正
		positiveCount := 0
		for i := 0; i < 3 && i < len(s.HistoricalCashflowList); i++ {
			if s.HistoricalCashflowList[i].NetcashOperate > 0 {
				positiveCount++
			}
		}
		if positiveCount == 3 {
			score.FreeCashFlowScore = 10.0 // 3年连续为正
		} else if positiveCount == 2 {
			score.FreeCashFlowScore = 7.0 // 2年为正
		} else if positiveCount == 1 {
			score.FreeCashFlowScore = 4.0 // 1年为正
		} else {
			score.FreeCashFlowScore = 0.0 // 都为负
		}
	} else if s.NetcashOperate > 0 {
		score.FreeCashFlowScore = 5.0 // 只有最新一期为正
	} else {
		score.FreeCashFlowScore = 0.0
	}

	// 7. 行业前景评分（10分）- 使用Coze API获取最新分析
	if s.CozeAnalysis != nil {
		score.IndustryScore = s.CozeAnalysis.IndustryProspectScore
	} else {
		// 如果没有Coze分析结果，使用默认分
		score.IndustryScore = 5.0 // 默认中等分
	}

	// 8. 市值评分（5分）
	marketCap := s.BaseInfo.TotalMarketCap / 100000000 // 转换为亿元
	if marketCap < 100 {
		score.MarketCapScore = 5.0
	} else if marketCap < 300 {
		score.MarketCapScore = 3.0
	} else if marketCap < 500 {
		score.MarketCapScore = 1.0
	} else {
		score.MarketCapScore = 0.0
	}

	// 计算总分
	score.TotalScore = score.PEGScore + score.EPSGrowthScore + score.RevenueGrowthScore +
		score.ProfitGrowthScore + score.ROEScore + score.FreeCashFlowScore +
		score.IndustryScore + score.MarketCapScore

	// 生成评分说明
	var desc strings.Builder
	desc.WriteString(fmt.Sprintf("彼得·林奇评分: %.1f/100\n", score.TotalScore))
	desc.WriteString(fmt.Sprintf("PEG评分(25分): %.1f\n", score.PEGScore))
	desc.WriteString(fmt.Sprintf("EPS增长评分(15分): %.1f\n", score.EPSGrowthScore))
	desc.WriteString(fmt.Sprintf("营收增长评分(15分): %.1f\n", score.RevenueGrowthScore))
	desc.WriteString(fmt.Sprintf("净利润增速评分(10分): %.1f\n", score.ProfitGrowthScore))
	desc.WriteString(fmt.Sprintf("ROE评分(10分): %.1f\n", score.ROEScore))
	desc.WriteString(fmt.Sprintf("自由现金流评分(10分): %.1f\n", score.FreeCashFlowScore))
	desc.WriteString(fmt.Sprintf("行业前景评分(10分): %.1f\n", score.IndustryScore))
	desc.WriteString(fmt.Sprintf("市值评分(5分): %.1f\n", score.MarketCapScore))

	score.ScoreDescription = desc.String()
	return score
}

// CalculateONeilScore 计算威廉·欧奈尔（CANSLIM）评分
func (s Stock) CalculateONeilScore() ONeilScore {
	score := ONeilScore{}

	// 1. C：当前季度利润增长评分（15分）
	profitGrowth := s.BaseInfo.NetprofitYoyRatio
	if profitGrowth >= 50.0 {
		score.CurrentQuarterScore = 15.0
	} else if profitGrowth >= 20.0 {
		score.CurrentQuarterScore = 10.0 + (profitGrowth-20.0)/30.0*5.0
	} else if profitGrowth >= 10.0 {
		score.CurrentQuarterScore = 5.0 + (profitGrowth-10.0)/10.0*5.0
	} else {
		score.CurrentQuarterScore = 0.0
	}

	// 2. A：年利润增长趋势评分（15分）- 需要历史数据，暂时标记为不确定
	if len(s.HistoricalFinaMainData) >= 3 {
		// 检查最近3年利润增长趋势
		avgGrowth := 0.0
		count := 0
		for i := 0; i < 3 && i < len(s.HistoricalFinaMainData); i++ {
			if s.HistoricalFinaMainData[i].Parentnetprofittz > 0 {
				avgGrowth += s.HistoricalFinaMainData[i].Parentnetprofittz
				count++
			}
		}
		if count > 0 {
			avgGrowth /= float64(count)
			if avgGrowth >= 20.0 {
				score.AnnualGrowthScore = 15.0
			} else if avgGrowth >= 10.0 {
				score.AnnualGrowthScore = 10.0 + (avgGrowth-10.0)/10.0*5.0
			} else {
				score.AnnualGrowthScore = avgGrowth / 10.0 * 10.0
			}
		} else {
			score.AnnualGrowthScore = 0.0
		}
	} else {
		score.AnnualGrowthScore = 0.0 // 数据不足
	}

	// 3. N：新品新概念评分（10分）- 使用Coze API获取最新分析
	if s.CozeAnalysis != nil {
		score.NewConceptScore = s.CozeAnalysis.NewConceptScore
	} else {
		// 如果没有Coze分析结果，使用默认分
		score.NewConceptScore = 5.0 // 默认中等分
	}

	// 4. S：股本小易拉升评分（10分）- 现在有数据了！
	// 通过总市值和股价计算流通股本（简化计算）
	currentPrice := s.GetPrice()
	if currentPrice > 0 {
		totalShares := s.BaseInfo.TotalMarketCap / (currentPrice * 100000000) // 转换为亿股
		if totalShares < 2 {
			score.SmallFloatScore = 10.0 // 小于2亿股
		} else if totalShares < 5 {
			score.SmallFloatScore = 7.0 // 2-5亿股
		} else if totalShares < 10 {
			score.SmallFloatScore = 4.0 // 5-10亿股
		} else {
			score.SmallFloatScore = 0.0 // 大于10亿股
		}
	} else {
		score.SmallFloatScore = 5.0 // 默认中等分
	}

	// 5. L：行业龙头评分（10分）- 使用Coze API获取最新分析
	if s.CozeAnalysis != nil {
		score.LeaderScore = s.CozeAnalysis.IndustryLeaderScore
	} else {
		// 如果没有Coze分析结果，使用默认分
		score.LeaderScore = 5.0 // 默认中等分
	}

	// 6. I：机构增持评分（20分）- 现在有数据了！
	if len(s.MainMoneyNetInflows) >= 20 {
		// 计算最近20日主力资金净流入
		recent20Days := s.MainMoneyNetInflows[:20]
		totalNetInflow := 0.0
		for _, inflow := range recent20Days {
			if netInflow, err := strconv.ParseFloat(inflow.MainMnyNetIn, 64); err == nil {
				totalNetInflow += netInflow
			}
		}

		if totalNetInflow > 10000 { // 净流入超过1亿元
			score.InstitutionScore = 20.0
		} else if totalNetInflow > 5000 { // 净流入超过5000万元
			score.InstitutionScore = 15.0
		} else if totalNetInflow > 0 { // 净流入为正
			score.InstitutionScore = 10.0
		} else if totalNetInflow > -5000 { // 净流出小于5000万元
			score.InstitutionScore = 5.0
		} else {
			score.InstitutionScore = 0.0 // 大幅净流出
		}
	} else {
		score.InstitutionScore = 10.0 // 默认中等分
	}

	// 7. M：技术趋势评分（20分）- 现在有数据了！
	if len(s.HistoricalPrice.Price) >= 20 {
		// 计算最近20日股价趋势
		recentPrices := s.HistoricalPrice.Price[len(s.HistoricalPrice.Price)-20:]
		if len(recentPrices) >= 20 {
			// 计算20日涨幅
			priceChange := (recentPrices[19] - recentPrices[0]) / recentPrices[0] * 100

			// 计算是否创新高
			maxPrice := recentPrices[0]
			for _, price := range recentPrices {
				if price > maxPrice {
					maxPrice = price
				}
			}
			isNewHigh := recentPrices[19] >= maxPrice*0.95 // 接近最高价

			if priceChange > 20 && isNewHigh {
				score.MarketTrendScore = 20.0 // 大涨且创新高
			} else if priceChange > 10 && isNewHigh {
				score.MarketTrendScore = 15.0 // 上涨且创新高
			} else if priceChange > 5 {
				score.MarketTrendScore = 10.0 // 温和上涨
			} else if priceChange > -5 {
				score.MarketTrendScore = 5.0 // 横盘
			} else {
				score.MarketTrendScore = 0.0 // 下跌
			}
		} else {
			score.MarketTrendScore = 10.0 // 默认中等分
		}
	} else {
		score.MarketTrendScore = 10.0 // 默认中等分
	}

	// 计算总分
	score.TotalScore = score.CurrentQuarterScore + score.AnnualGrowthScore +
		score.NewConceptScore + score.SmallFloatScore + score.LeaderScore +
		score.InstitutionScore + score.MarketTrendScore

	// 生成评分说明
	var desc strings.Builder
	desc.WriteString(fmt.Sprintf("威廉·欧奈尔评分: %.1f/100\n", score.TotalScore))
	desc.WriteString(fmt.Sprintf("当前季度利润增长(15分): %.1f\n", score.CurrentQuarterScore))
	desc.WriteString(fmt.Sprintf("年利润增长趋势(15分): %.1f\n", score.AnnualGrowthScore))
	desc.WriteString(fmt.Sprintf("新品新概念(10分): %.1f\n", score.NewConceptScore))
	desc.WriteString(fmt.Sprintf("股本小易拉升(10分): %.1f\n", score.SmallFloatScore))
	desc.WriteString(fmt.Sprintf("行业龙头(10分): %.1f\n", score.LeaderScore))
	desc.WriteString(fmt.Sprintf("机构增持(20分): %.1f\n", score.InstitutionScore))
	desc.WriteString(fmt.Sprintf("技术趋势(20分): %.1f\n", score.MarketTrendScore))

	score.ScoreDescription = desc.String()
	return score
}

// CalculateBuffettScoreWithLLM 计算巴菲特评分（包含LLM分析）
func (s Stock) CalculateBuffettScoreWithLLM(llmAnalysis LLMAnalysis) BuffettScore {
	score := BuffettScore{}

	// 存储原始数据用于生成详细描述
	var roeData, cashFlowData, profitGrowthData, debtRatioData, valuationData string

	// 1. ROE评分（20分）
	score.ROEScore = s.calculateROEScoreWithData(context.Background())
	roeData = s.getROEDataDescription()

	// 2. 现金流评分（15分）
	score.CashFlowScore = s.calculateCashFlowScoreWithData(context.Background())
	cashFlowData = s.getCashFlowDataDescription()

	// 3. 利润增长评分（15分）
	score.ProfitGrowthScore = s.calculateProfitGrowthScoreWithData(context.Background())
	profitGrowthData = s.getProfitGrowthDataDescription()

	// 4. 负债率评分（10分）
	score.DebtRatioScore = s.calculateDebtRatioScoreWithData(context.Background())
	debtRatioData = s.getDebtRatioDataDescription()

	// 5. 护城河评分（10分）- 使用LLM分析结果
	score.MoatScore = llmAnalysis.MoatScore / 100.0 * 10.0

	// 6. 管理层评分（10分）- 使用LLM分析结果
	score.ManagementScore = llmAnalysis.ManagementScore / 100.0 * 10.0

	// 7. 估值评分（15分）
	score.ValuationScore = s.calculateValuationScoreWithData(context.Background())
	valuationData = s.getValuationDataDescription()

	// 8. 研发投入评分（5分）
	s.calculateRDScore(context.Background())
	score.RDScore = s.BuffettScore.RDScore

	// 9. 分红评分（5分）
	s.calculateDividendScore(context.Background())
	score.DividendScore = s.BuffettScore.DividendScore

	// 10. 回购评分（5分）- 使用LLM分析结果
	score.RepurchaseScore = llmAnalysis.RepurchaseScore / 100.0 * 5.0

	// 计算总分
	rawTotalScore := score.ROEScore +
		score.CashFlowScore +
		score.ProfitGrowthScore +
		score.DebtRatioScore +
		score.MoatScore +
		score.ManagementScore +
		score.ValuationScore +
		score.RDScore +
		score.DividendScore +
		score.RepurchaseScore

	// 将110分制换算成100分制
	totalScore := rawTotalScore * 100 / 110

	// 生成评分说明
	var desc strings.Builder
	desc.WriteString(fmt.Sprintf("总分(100分): %.1f (原始得分: %.1f)\n\n", totalScore, rawTotalScore))
	desc.WriteString(fmt.Sprintf("ROE(20分): %.1f %s\n", score.ROEScore, roeData))
	desc.WriteString(fmt.Sprintf("现金流(15分): %.1f %s\n", score.CashFlowScore, cashFlowData))
	desc.WriteString(fmt.Sprintf("利润增长(15分): %.1f %s\n", score.ProfitGrowthScore, profitGrowthData))
	desc.WriteString(fmt.Sprintf("负债率(10分): %.1f %s\n", score.DebtRatioScore, debtRatioData))
	desc.WriteString(fmt.Sprintf("护城河(10分): %.1f (LLM评分: %.1f)\n", score.MoatScore, llmAnalysis.MoatScore))
	desc.WriteString(fmt.Sprintf("管理层(10分): %.1f (LLM评分: %.1f)\n", score.ManagementScore, llmAnalysis.ManagementScore))
	desc.WriteString(fmt.Sprintf("估值(15分): %.1f %s\n", score.ValuationScore, valuationData))
	desc.WriteString(fmt.Sprintf("研发投入(5分): %.1f\n", score.RDScore))
	desc.WriteString(fmt.Sprintf("分红(5分): %.1f\n", score.DividendScore))
	desc.WriteString(fmt.Sprintf("回购(5分): %.1f (LLM评分: %.1f)", score.RepurchaseScore, llmAnalysis.RepurchaseScore))

	score.ScoreDescription = desc.String()
	score.TotalScore = totalScore
	return score
}

// CalculateLynchScoreWithLLM 计算彼得·林奇评分（包含LLM分析）
func (s Stock) CalculateLynchScoreWithLLM(llmAnalysis LLMAnalysis) LynchScore {
	score := LynchScore{}

	// 1. PEG评分（25分）
	peg := s.PEG
	if peg <= 1.0 {
		score.PEGScore = 25.0
	} else if peg <= 1.5 {
		score.PEGScore = 18.0
	} else if peg <= 2.0 {
		score.PEGScore = 10.0
	} else {
		score.PEGScore = 0.0
	}

	// 2. EPS持续增长评分（15分）
	if len(s.HistoricalFinaMainData) >= 5 {
		growthCount := 0
		for i := 1; i < 5 && i < len(s.HistoricalFinaMainData); i++ {
			if s.HistoricalFinaMainData[i].Epsjb > s.HistoricalFinaMainData[i-1].Epsjb {
				growthCount++
			}
		}
		if growthCount >= 4 {
			score.EPSGrowthScore = 15.0
		} else if growthCount >= 3 {
			score.EPSGrowthScore = 12.0
		} else if growthCount >= 2 {
			score.EPSGrowthScore = 8.0
		} else {
			score.EPSGrowthScore = 4.0
		}
	} else if len(s.HistoricalFinaMainData) >= 3 {
		growthCount := 0
		for i := 1; i < 3 && i < len(s.HistoricalFinaMainData); i++ {
			if s.HistoricalFinaMainData[i].Epsjb > s.HistoricalFinaMainData[i-1].Epsjb {
				growthCount++
			}
		}
		score.EPSGrowthScore = float64(growthCount) / 2.0 * 15.0
	} else {
		score.EPSGrowthScore = 0.0
	}

	// 3. 营收增长评分（15分）
	revenueGrowth := s.BaseInfo.ToiYoyRatio
	if revenueGrowth >= 30.0 {
		score.RevenueGrowthScore = 15.0
	} else if revenueGrowth >= 15.0 {
		score.RevenueGrowthScore = 10.0
	} else if revenueGrowth > 0 {
		score.RevenueGrowthScore = float64(revenueGrowth) / 30.0 * 15.0
	} else {
		score.RevenueGrowthScore = 0.0
	}

	// 4. 净利润增速评分（10分）
	profitGrowth := s.BaseInfo.NetprofitYoyRatio
	if profitGrowth >= 20.0 {
		score.ProfitGrowthScore = 10.0
	} else if profitGrowth >= 10.0 {
		score.ProfitGrowthScore = 6.0 + (profitGrowth-10.0)/10.0*4.0
	} else if profitGrowth > 0 {
		score.ProfitGrowthScore = 2.0 + profitGrowth/10.0*4.0
	} else {
		score.ProfitGrowthScore = 0.0
	}

	// 5. ROE评分（10分）
	if len(s.HistoricalFinaMainData) > 0 {
		roe := s.HistoricalFinaMainData[0].Roejq
		if roe >= 20.0 {
			score.ROEScore = 10.0
		} else if roe >= 15.0 {
			score.ROEScore = 8.0
		} else if roe > 0 {
			score.ROEScore = roe / 15.0 * 8.0
		} else {
			score.ROEScore = 0.0
		}
	} else {
		score.ROEScore = 0.0
	}

	// 6. 自由现金流评分（10分）
	if len(s.HistoricalCashflowList) >= 3 {
		positiveCount := 0
		for i := 0; i < 3 && i < len(s.HistoricalCashflowList); i++ {
			cf := s.HistoricalCashflowList[i]
			var freeCashFlow float64
			if cf.NetcashInvest < 0 {
				freeCashFlow = cf.NetcashOperate + cf.NetcashInvest
			} else {
				freeCashFlow = cf.NetcashOperate - cf.NetcashInvest
			}
			if freeCashFlow > 0 {
				positiveCount++
			}
		}
		score.FreeCashFlowScore = float64(positiveCount) / 3.0 * 10.0
	} else {
		score.FreeCashFlowScore = 0.0
	}

	// 7. 行业前景评分（10分）- 使用LLM分析结果
	score.IndustryScore = llmAnalysis.IndustryProspectScore / 100.0 * 10.0

	// 8. 市值评分（5分）
	marketCap := s.BaseInfo.TotalMarketCap / 100000000 // 转换为亿元
	if marketCap < 100 {
		score.MarketCapScore = 5.0
	} else if marketCap < 300 {
		score.MarketCapScore = 3.0
	} else if marketCap < 500 {
		score.MarketCapScore = 1.0
	} else {
		score.MarketCapScore = 0.0
	}

	// 计算总分
	score.TotalScore = score.PEGScore + score.EPSGrowthScore + score.RevenueGrowthScore +
		score.ProfitGrowthScore + score.ROEScore + score.FreeCashFlowScore +
		score.IndustryScore + score.MarketCapScore

	// 生成评分说明
	var desc strings.Builder
	desc.WriteString(fmt.Sprintf("彼得·林奇评分: %.1f/100\n", score.TotalScore))
	desc.WriteString(fmt.Sprintf("PEG评分(25分): %.1f (PEG: %.2f)\n", score.PEGScore, s.PEG))
	desc.WriteString(fmt.Sprintf("EPS增长评分(15分): %.1f\n", score.EPSGrowthScore))
	desc.WriteString(fmt.Sprintf("营收增长评分(15分): %.1f (增长率: %.1f%%)\n", score.RevenueGrowthScore, revenueGrowth))
	desc.WriteString(fmt.Sprintf("净利润增速评分(10分): %.1f (增长率: %.1f%%)\n", score.ProfitGrowthScore, profitGrowth))
	desc.WriteString(fmt.Sprintf("ROE评分(10分): %.1f\n", score.ROEScore))
	desc.WriteString(fmt.Sprintf("自由现金流评分(10分): %.1f\n", score.FreeCashFlowScore))
	desc.WriteString(fmt.Sprintf("行业前景评分(10分): %.1f (LLM评分: %.1f)\n", score.IndustryScore, llmAnalysis.IndustryProspectScore))
	desc.WriteString(fmt.Sprintf("市值评分(5分): %.1f (市值: %.1f亿元)\n", score.MarketCapScore, marketCap))

	score.ScoreDescription = desc.String()
	return score
}

// CalculateONeilScoreWithLLM 计算威廉·欧奈尔评分（包含LLM分析）
func (s Stock) CalculateONeilScoreWithLLM(llmAnalysis LLMAnalysis) ONeilScore {
	score := ONeilScore{}

	// 1. C：当前季度利润增长评分（15分）
	profitGrowth := s.BaseInfo.NetprofitYoyRatio
	if profitGrowth >= 50.0 {
		score.CurrentQuarterScore = 15.0
	} else if profitGrowth >= 20.0 {
		score.CurrentQuarterScore = 10.0 + (profitGrowth-20.0)/30.0*5.0
	} else if profitGrowth >= 10.0 {
		score.CurrentQuarterScore = 5.0 + (profitGrowth-10.0)/10.0*5.0
	} else {
		score.CurrentQuarterScore = 0.0
	}

	// 2. A：年利润增长趋势评分（15分）
	if len(s.HistoricalFinaMainData) >= 3 {
		avgGrowth := 0.0
		count := 0
		for i := 0; i < 3 && i < len(s.HistoricalFinaMainData); i++ {
			if s.HistoricalFinaMainData[i].Parentnetprofittz > 0 {
				avgGrowth += s.HistoricalFinaMainData[i].Parentnetprofittz
				count++
			}
		}
		if count > 0 {
			avgGrowth /= float64(count)
			if avgGrowth >= 20.0 {
				score.AnnualGrowthScore = 15.0
			} else if avgGrowth >= 10.0 {
				score.AnnualGrowthScore = 10.0 + (avgGrowth-10.0)/10.0*5.0
			} else {
				score.AnnualGrowthScore = avgGrowth / 10.0 * 10.0
			}
		} else {
			score.AnnualGrowthScore = 0.0
		}
	} else {
		score.AnnualGrowthScore = 0.0
	}

	// 3. N：新品新概念评分（10分）- 使用LLM分析结果
	score.NewConceptScore = llmAnalysis.NewConceptScore / 100.0 * 10.0

	// 4. S：股本小易拉升评分（10分）
	currentPrice := s.GetPrice()
	if currentPrice > 0 {
		totalShares := s.BaseInfo.TotalMarketCap / (currentPrice * 100000000)
		if totalShares < 2 {
			score.SmallFloatScore = 10.0
		} else if totalShares < 5 {
			score.SmallFloatScore = 7.0
		} else if totalShares < 10 {
			score.SmallFloatScore = 4.0
		} else {
			score.SmallFloatScore = 0.0
		}
	} else {
		score.SmallFloatScore = 5.0
	}

	// 5. L：行业龙头评分（10分）- 使用LLM分析结果
	score.LeaderScore = llmAnalysis.IndustryLeaderScore / 100.0 * 10.0

	// 6. I：机构增持评分（20分）- 使用LLM分析结果
	score.InstitutionScore = llmAnalysis.InstitutionScore / 100.0 * 20.0

	// 7. M：技术趋势评分（20分）- 使用LLM分析结果
	score.MarketTrendScore = llmAnalysis.TechnicalTrendScore / 100.0 * 20.0

	// 计算总分
	score.TotalScore = score.CurrentQuarterScore + score.AnnualGrowthScore +
		score.NewConceptScore + score.SmallFloatScore + score.LeaderScore +
		score.InstitutionScore + score.MarketTrendScore

	// 生成评分说明
	var desc strings.Builder
	desc.WriteString(fmt.Sprintf("威廉·欧奈尔评分: %.1f/100\n", score.TotalScore))
	desc.WriteString(fmt.Sprintf("当前季度利润增长(15分): %.1f (增长率: %.1f%%)\n", score.CurrentQuarterScore, profitGrowth))
	desc.WriteString(fmt.Sprintf("年利润增长趋势(15分): %.1f\n", score.AnnualGrowthScore))
	desc.WriteString(fmt.Sprintf("新品新概念(10分): %.1f (LLM评分: %.1f)\n", score.NewConceptScore, llmAnalysis.NewConceptScore))
	desc.WriteString(fmt.Sprintf("股本小易拉升(10分): %.1f\n", score.SmallFloatScore))
	desc.WriteString(fmt.Sprintf("行业龙头(10分): %.1f (LLM评分: %.1f)\n", score.LeaderScore, llmAnalysis.IndustryLeaderScore))
	desc.WriteString(fmt.Sprintf("机构增持(20分): %.1f (LLM评分: %.1f)\n", score.InstitutionScore, llmAnalysis.InstitutionScore))
	desc.WriteString(fmt.Sprintf("技术趋势(20分): %.1f (LLM评分: %.1f)\n", score.MarketTrendScore, llmAnalysis.TechnicalTrendScore))

	score.ScoreDescription = desc.String()
	return score
}

// calculateROEScoreWithData 计算ROE评分并返回分数
func (s Stock) calculateROEScoreWithData(ctx context.Context) float64 {
	// 获取近5年ROE数据
	roeList := s.HistoricalFinaMainData.ValueList(ctx, eastmoney.ValueListTypeROE, 5, eastmoney.FinaReportTypeYear)

	if len(roeList) == 0 || len(roeList) < 5 {
		return 0
	}

	// 计算平均ROE和波动率
	var sumROE float64
	for _, roe := range roeList {
		sumROE += roe
	}
	avgROE := sumROE / float64(len(roeList))

	// 计算ROE波动率
	var variance float64
	for _, roe := range roeList {
		variance += math.Pow(roe-avgROE, 2)
	}
	volatility := math.Sqrt(variance/float64(len(roeList))) / avgROE

	// 根据ROE和波动率评分
	score := 0.0
	if avgROE >= 20 {
		score = 20
	} else if avgROE >= 15 {
		score = 15
	} else {
		score = (avgROE / 15) * 15
	}

	// 根据波动率扣分
	if volatility > 0.3 {
		score *= 0.8 // 波动大扣20%分数
	}

	return score
}

// getROEDataDescription 获取ROE数据描述
func (s Stock) getROEDataDescription() string {
	if len(s.HistoricalFinaMainData) < 5 {
		return "(数据不足)"
	}

	roeList := s.HistoricalFinaMainData.ValueList(context.Background(), eastmoney.ValueListTypeROE, 5, eastmoney.FinaReportTypeYear)
	if len(roeList) < 5 {
		return "(数据不足)"
	}

	var sumROE float64
	for _, roe := range roeList {
		sumROE += roe
	}
	avgROE := sumROE / float64(len(roeList))

	var variance float64
	for _, roe := range roeList {
		variance += math.Pow(roe-avgROE, 2)
	}
	volatility := math.Sqrt(variance/float64(len(roeList))) / avgROE

	return fmt.Sprintf("(平均ROE: %.1f%%, 波动率: %.2f)", avgROE, volatility)
}

// calculateCashFlowScoreWithData 计算现金流评分并返回分数
func (s Stock) calculateCashFlowScoreWithData(ctx context.Context) float64 {
	if len(s.HistoricalCashflowList) < 3 {
		return 0
	}

	operatePositiveCount := 0
	freePositiveCount := 0
	for i := 0; i < 3 && i < len(s.HistoricalCashflowList); i++ {
		cf := s.HistoricalCashflowList[i]
		if cf.NetcashOperate > 0 {
			operatePositiveCount++
		}
		// 计算自由现金流
		var freeCashFlow float64
		if cf.NetcashInvest < 0 {
			freeCashFlow = cf.NetcashOperate + cf.NetcashInvest
		} else {
			freeCashFlow = cf.NetcashOperate - cf.NetcashInvest
		}
		if freeCashFlow > 0 {
			freePositiveCount++
		}
	}

	// 经营现金流和自由现金流各占一半分数
	operateScore := 0.0
	switch operatePositiveCount {
	case 3:
		operateScore = 7.5
	case 2:
		operateScore = 5.0
	case 1:
		operateScore = 2.5
	}

	freeScore := 0.0
	switch freePositiveCount {
	case 3:
		freeScore = 7.5
	case 2:
		freeScore = 5.0
	case 1:
		freeScore = 2.5
	}

	return operateScore + freeScore
}

// getCashFlowDataDescription 获取现金流数据描述
func (s Stock) getCashFlowDataDescription() string {
	if len(s.HistoricalCashflowList) < 3 {
		return "(数据不足)"
	}

	operatePositiveCount := 0
	freePositiveCount := 0
	for i := 0; i < 3 && i < len(s.HistoricalCashflowList); i++ {
		cf := s.HistoricalCashflowList[i]
		if cf.NetcashOperate > 0 {
			operatePositiveCount++
		}
		var freeCashFlow float64
		if cf.NetcashInvest < 0 {
			freeCashFlow = cf.NetcashOperate + cf.NetcashInvest
		} else {
			freeCashFlow = cf.NetcashOperate - cf.NetcashInvest
		}
		if freeCashFlow > 0 {
			freePositiveCount++
		}
	}

	return fmt.Sprintf("(经营现金流正数年数: %d, 自由现金流正数年数: %d)", operatePositiveCount, freePositiveCount)
}

// calculateProfitGrowthScoreWithData 计算利润增长评分并返回分数
func (s Stock) calculateProfitGrowthScoreWithData(ctx context.Context) float64 {
	if len(s.HistoricalFinaMainData) < 5 {
		return 0
	}

	// 获取近5年净利润数据
	profitList := s.HistoricalFinaMainData.ValueList(ctx, eastmoney.ValueListTypeNetProfit, 5, eastmoney.FinaReportTypeYear)
	if len(profitList) < 5 {
		return 0
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

	return score
}

// getProfitGrowthDataDescription 获取利润增长数据描述
func (s Stock) getProfitGrowthDataDescription() string {
	if len(s.HistoricalFinaMainData) < 5 {
		return "(数据不足)"
	}

	profitList := s.HistoricalFinaMainData.ValueList(context.Background(), eastmoney.ValueListTypeNetProfit, 5, eastmoney.FinaReportTypeYear)
	if len(profitList) < 5 {
		return "(数据不足)"
	}

	growthCount := 0
	for i := 0; i < len(profitList)-1; i++ {
		if profitList[i] > profitList[i+1] {
			growthCount++
		}
	}

	return fmt.Sprintf("(增长年数: %d/4)", growthCount)
}

// calculateDebtRatioScoreWithData 计算负债率评分并返回分数
func (s Stock) calculateDebtRatioScoreWithData(ctx context.Context) float64 {
	if len(s.HistoricalFinaMainData) == 0 {
		return 0
	}

	// 获取最新负债率
	debtRatio := s.HistoricalFinaMainData[0].Zcfzl

	switch {
	case debtRatio < 30:
		return 10
	case debtRatio < 50:
		return 8
	case debtRatio < 70:
		return 5
	default:
		return 0
	}
}

// getDebtRatioDataDescription 获取负债率数据描述
func (s Stock) getDebtRatioDataDescription() string {
	if len(s.HistoricalFinaMainData) == 0 {
		return "(数据不足)"
	}

	debtRatio := s.HistoricalFinaMainData[0].Zcfzl
	return fmt.Sprintf("(负债率: %.1f%%)", debtRatio)
}

// calculateValuationScoreWithData 计算估值评分并返回分数
func (s Stock) calculateValuationScoreWithData(ctx context.Context) float64 {
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
		score = math.Max(score, 15) // PEG<1时至少得15分
	}

	return score
}

// getValuationDataDescription 获取估值数据描述
func (s Stock) getValuationDataDescription() string {
	return fmt.Sprintf("(PE: %.1f, PEG: %.1f)", s.BaseInfo.PE, s.PEG)
}
