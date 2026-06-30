package periphera

import (
	"fmt"
	"math/rand"
	"sort"
	"time"
)

// 本文件提供 M0 阶段的「契约合法」种子数据。真实源接入后逐个替换，
// 对外 JSON 结构不变。所有 *_at 时间戳为当前 Unix 毫秒，价格带轻微抖动，
// 让前端在轮询时能看到跳动效果。

// ---------------- 新闻 ----------------

var newsPool = []NewsItem{
	{Source: "reuters", SourceName: "路透社", Color: "#D0643E", Tag: "RT", Title: "霍尔木兹海峡油轮通行量环比下降，布伦特原油盘中跳涨逾 2%", Summary: "地缘紧张推升供给担忧，交易员关注护航与保险费率变化。", Hot: true, Tags: []string{"原油", "霍尔木兹"}},
	{Source: "bloomberg", SourceName: "彭博社", Color: "#3D362A", Tag: "BB", Title: "美联储官员重申「数据依赖」，市场下调 9 月降息押注", Summary: "隔夜利率期货显示年内降息预期降温，美元指数小幅走强。", Tags: []string{"FedWatch", "美元"}},
	{Source: "musk", SourceName: "马斯克", Color: "#E7A265", Tag: "EM", Title: "Musk：下季度将公布全新 AI 算力集群，相关供应链盘后异动", Summary: "算力扩张预期带动半导体与电力设备板块情绪。", Hot: true, Tags: []string{"AI", "半导体"}},
	{Source: "baimao", SourceName: "白毛女神", Color: "#8a6d3b", Tag: "BM", Title: "白毛复盘：能源与黄金共振，关注隔夜暗盘金价对 A 股有色的指引", Summary: "避险与再通胀交易并存，留意贵金属夜盘溢价。", Tags: []string{"黄金", "有色"}},
	{Source: "bloomberg", SourceName: "彭博社", Color: "#3D362A", Tag: "BB", Title: "台积电 ADR 走强，带动台湾加权指数早盘创近月新高", Summary: "先进制程订单饱满，AI 加速卡需求维持高景气。", Tags: []string{"半导体", "台股"}},
	{Source: "reuters", SourceName: "路透社", Color: "#D0643E", Tag: "RT", Title: "韩国 6 月出口数据好于预期，半导体出口同比转正", Summary: "外需回暖支撑 KOSPI，存储芯片价格连续两月回升。", Tags: []string{"韩国", "半导体"}},
	{Source: "musk", SourceName: "马斯克", Color: "#E7A265", Tag: "EM", Title: "特斯拉能源业务部署量创纪录，储能板块情绪升温", Summary: "Megapack 产能爬坡，储能成为新增长极。", Tags: []string{"储能", "特斯拉"}},
	{Source: "reuters", SourceName: "路透社", Color: "#D0643E", Tag: "RT", Title: "OPEC+ 维持减产基调，分析师上调下半年油价中枢", Summary: "供给端纪律性增强，库存去化快于预期。", Tags: []string{"原油", "OPEC"}},
	{Source: "bloomberg", SourceName: "彭博社", Color: "#3D362A", Tag: "BB", Title: "黄金站上阶段新高，央行购金需求延续", Summary: "实际利率见顶预期叠加地缘避险，金价获双重支撑。", Tags: []string{"黄金"}},
	{Source: "baimao", SourceName: "白毛女神", Color: "#8a6d3b", Tag: "BM", Title: "白毛盯盘：铜价短线回调，关注电气化长期需求逻辑", Summary: "工业金属短期承压，但绿色转型主线未变。", Tags: []string{"铜", "电气化"}},
}

// SeedNews 返回新闻列表（按 published_at 倒序）。时间戳基于当前时间生成，
// 每条间隔约 4 分钟，便于前端用 since 做增量与红点测试。
func SeedNews() []NewsItem {
	now := nowMS()
	out := make([]NewsItem, len(newsPool))
	for i, n := range newsPool {
		n.ID = fmt.Sprintf("n_%d", i)
		n.PublishedAt = now - int64(i)*4*60*1000
		n.URL = "https://example.com/news/" + n.ID
		out[i] = n
	}
	return out
}

// SeedNewsSources 返回来源栏（含计数）
func SeedNewsSources() []NewsSource {
	counts := map[string]int{}
	for _, n := range newsPool {
		counts[n.Source]++
	}
	return []NewsSource{
		{Code: "reuters", Name: "路透社", Tag: "RT", Color: "#D0643E", Desc: "全球财经快讯", Count: counts["reuters"]},
		{Code: "bloomberg", Name: "彭博社", Tag: "BB", Color: "#3D362A", Desc: "市场与宏观", Count: counts["bloomberg"]},
		{Code: "musk", Name: "马斯克", Tag: "EM", Color: "#E7A265", Desc: "科技 / 情绪面", Count: counts["musk"]},
		{Code: "baimao", Name: "白毛女神", Tag: "BM", Color: "#8a6d3b", Desc: "盘面观点解读", Count: counts["baimao"]},
	}
}

// ---------------- 研报 ----------------

var researchPool = []ResearchReport{
	{PublishTime: "2026-06-27", IndustryCategory: "能源", InstitutionName: "高盛 GS", ResearchTarget: "能源板块", ReportType: "行业深度", InvestmentRating: "增持", TargetPrice: "—", RatingChange: "上调", CoreContent: "地缘风险溢价回归，上调原油全年中枢至 85 美元，看好上游与油服。", CoreCatalyst: "霍尔木兹局势 + OPEC+ 减产纪律", CoreRiskWarning: "需求走弱、地缘缓和导致溢价回吐"},
	{PublishTime: "2026-06-27", IndustryCategory: "贵金属", InstitutionName: "摩根士丹利", ResearchTarget: "黄金", ReportType: "商品策略", InvestmentRating: "买入", TargetPrice: "$2480", RatingChange: "上调", CoreContent: "实际利率见顶 + 央行购金延续，上调金价 12 个月目标价。", CoreCatalyst: "降息周期开启、央行购金", CoreRiskWarning: "美元超预期走强"},
	{PublishTime: "2026-06-26", IndustryCategory: "半导体", InstitutionName: "摩根大通", ResearchTarget: "台积电", ReportType: "公司点评", InvestmentRating: "买入", TargetPrice: "NT$1180", RatingChange: "维持", CoreContent: "AI 加速卡需求强劲，先进制程产能利用率维持高位。", CoreCatalyst: "CoWoS 扩产、N2 量产", CoreRiskWarning: "消费电子复苏不及预期"},
	{PublishTime: "2026-06-26", IndustryCategory: "工业金属", InstitutionName: "瑞银 UBS", ResearchTarget: "铜", ReportType: "商品策略", InvestmentRating: "中性", TargetPrice: "$4.6", RatingChange: "下调", CoreContent: "短期需求走弱，下调铜价预期，但长期电气化逻辑不变。", CoreCatalyst: "电网投资、新能源装机", CoreRiskWarning: "地产链拖累、库存累积"},
	{PublishTime: "2026-06-25", IndustryCategory: "科技", InstitutionName: "美银 BofA", ResearchTarget: "美股科技", ReportType: "策略", InvestmentRating: "增持", TargetPrice: "—", RatingChange: "维持", CoreContent: "盈利上修推动估值消化，AI 资本开支周期仍在早中期。", CoreCatalyst: "AI 资本开支、盈利上修", CoreRiskWarning: "利率高企压制成长估值"},
	{PublishTime: "2026-06-25", IndustryCategory: "海外股指", InstitutionName: "花旗 Citi", ResearchTarget: "日经225", ReportType: "策略", InvestmentRating: "增持", TargetPrice: "42000", RatingChange: "上调", CoreContent: "治理改革 + 弱日元支撑出口盈利，上调指数目标。", CoreCatalyst: "治理改革、股东回报", CoreRiskWarning: "日元快速升值"},
	{PublishTime: "2026-06-24", IndustryCategory: "原油", InstitutionName: "巴克莱 Barclays", ResearchTarget: "布伦特原油", ReportType: "商品策略", InvestmentRating: "买入", TargetPrice: "$92", RatingChange: "上调", CoreContent: "供给端纪律性增强，下半年去库加速，上调布伦特目标价。", CoreCatalyst: "OPEC+ 减产、地缘风险", CoreRiskWarning: "美国页岩增产超预期"},
	{PublishTime: "2026-06-24", IndustryCategory: "海外股指", InstitutionName: "野村 Nomura", ResearchTarget: "韩国KOSPI", ReportType: "策略", InvestmentRating: "中性", TargetPrice: "2900", RatingChange: "维持", CoreContent: "半导体出口回暖支撑盈利，但估值已部分反映，维持中性。", CoreCatalyst: "存储芯片涨价周期", CoreRiskWarning: "全球需求二次走弱"},
	{PublishTime: "2026-06-23", IndustryCategory: "新能源", InstitutionName: "德银 DB", ResearchTarget: "储能板块", ReportType: "行业深度", InvestmentRating: "增持", TargetPrice: "—", RatingChange: "上调", CoreContent: "电网级储能需求爆发，订单能见度提升，上调板块评级。", CoreCatalyst: "电网投资、数据中心用电", CoreRiskWarning: "锂价反弹、并网政策变化"},
}

// SeedResearch 返回研报列表（按 created_at 倒序）
func SeedResearch() []ResearchReport {
	now := nowMS()
	out := make([]ResearchReport, len(researchPool))
	for i, r := range researchPool {
		r.ID = fmt.Sprintf("r_%d", i)
		r.CreatedAt = now - int64(i)*30*60*1000
		out[i] = r
	}
	return out
}

// ResearchSummary 研报概览统计
type ResearchSummary struct {
	Total    int `json:"total"`
	Upgrades int `json:"upgrades"`
	Buy      int `json:"buy"`
	Targets  int `json:"targets"`
}

// SummarizeResearch 统计概览
func SummarizeResearch(list []ResearchReport) ResearchSummary {
	s := ResearchSummary{Total: len(list)}
	seen := map[string]bool{}
	for _, r := range list {
		if r.RatingChange == "上调" {
			s.Upgrades++
		}
		if r.InvestmentRating == "买入" || r.InvestmentRating == "增持" {
			s.Buy++
		}
		if !seen[r.ResearchTarget] {
			seen[r.ResearchTarget] = true
			s.Targets++
		}
	}
	return s
}

// ---------------- 股指 ----------------

type idxSeed struct {
	code, name, flag string
	base             float64
}

var idxSeeds = []idxSeed{
	{"SPX", "标普500", "🇺🇸", 5487},
	{"IXIC", "纳指", "🇺🇸", 17860},
	{"DJI", "道指", "🇺🇸", 39120},
	{"N225", "日经225", "🇯🇵", 39120},
	{"KOSPI", "韩国KOSPI", "🇰🇷", 2745},
	{"TWSE", "台湾加权", "🇹🇼", 23180},
}

// SeedIndices 返回股指行情（可选 codes 过滤）
func SeedIndices(codes []string) []IndexQuote {
	now := nowMS()
	var out []IndexQuote
	for _, s := range idxSeeds {
		if !wantCode(codes, s.code) {
			continue
		}
		val := jitter(s.base, 0.004)
		pct := round2((rand.Float64() - 0.4) * 1.6)
		out = append(out, IndexQuote{
			Code: s.code, Name: s.name, Flag: s.flag, Value: val,
			Change: round2(val * pct / 100), ChangePct: pct,
			PrevClose: round2(val / (1 + pct/100)), IsOpen: true,
			Spark: walk(40, s.base, s.base*0.002, 0), UpdatedAt: now,
		})
	}
	if out == nil {
		out = []IndexQuote{}
	}
	return out
}

// SeedSectors 返回美股板块涨幅（倒序，便于直接画横向柱状图）
func SeedSectors() []SectorItem {
	out := []SectorItem{
		{"能源", 2.4}, {"信息技术", 1.8}, {"通信", 1.2}, {"金融", 0.7}, {"材料", 0.5},
		{"工业", 0.3}, {"医疗", -0.2}, {"消费", -0.4}, {"房地产", -0.8}, {"公用", -1.1},
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChangePct > out[j].ChangePct })
	return out
}

// ---------------- 商品 ----------------

type comSeed struct {
	code, name, unit string
	base             float64
	dark             bool
}

var comSeeds = []comSeed{
	{"XAU", "黄金", "USD/oz", 2356.4, false},
	{"XAG", "白银", "USD/oz", 30.18, false},
	{"HG", "铜", "美分/磅", 4.41, false},
	{"WTI", "WTI原油", "USD/bbl", 81.92, false},
	{"XAU_AH", "暗金", "USD/oz", 2361.0, true},
	{"WTI_AH", "暗油", "USD/bbl", 82.40, true},
}

// SeedCommodities 返回商品行情（金银铜油 + 暗金暗油，可选 codes 过滤）
func SeedCommodities(codes []string) []Commodity {
	now := nowMS()
	var out []Commodity
	for _, s := range comSeeds {
		if !wantCode(codes, s.code) {
			continue
		}
		price := jitter(s.base, 0.003)
		pct := round2((rand.Float64() - 0.45) * 2.4)
		spark := []float64{}
		if !s.dark {
			spark = walk(30, s.base, s.base*0.004, 0)
		}
		out = append(out, Commodity{
			Code: s.code, Name: s.name, Price: price,
			Change: round2(price * pct / 100), ChangePct: pct,
			Dark: s.dark, Spark: spark, Unit: s.unit, UpdatedAt: now,
		})
	}
	if out == nil {
		out = []Commodity{}
	}
	return out
}

// SeedCrude 返回原油实时分时（WTI + Brent）
func SeedCrude() CrudeResp {
	wti := walk(40, 80, 0.5, 0.05)
	brent := walk(40, 84, 0.5, 0.05)
	labels := make([]string, len(wti))
	return CrudeResp{
		Wti:    CrudeLeg{Price: wti[len(wti)-1], ChangePct: round2((rand.Float64() - 0.3) * 2)},
		Brent:  CrudeLeg{Price: brent[len(brent)-1], ChangePct: round2((rand.Float64() - 0.3) * 2)},
		Labels: labels, WtiSeries: wti, BrentSeries: brent, UpdatedAt: nowMS(),
	}
}

// ---------------- 加密 ----------------

type cryptoSeed struct {
	code, name string
	base       float64
}

var cryptoSeeds = []cryptoSeed{
	{"BTC", "BTC", 61240},
	{"ETH", "ETH", 3410},
	{"PAXG", "PAXG", 2360},
	{"XAUT", "XAUT", 2360},
}

// SeedCrypto 返回加密行情（可选 codes 过滤）
func SeedCrypto(codes []string) []CryptoQuote {
	now := nowMS()
	var out []CryptoQuote
	for _, s := range cryptoSeeds {
		if !wantCode(codes, s.code) {
			continue
		}
		out = append(out, CryptoQuote{
			Code: s.code, Name: s.name, Price: jitter(s.base, 0.006),
			ChangePct: round2((rand.Float64() - 0.4) * 5),
			Spark:     walk(30, s.base, s.base*0.01, 0), UpdatedAt: now,
		})
	}
	if out == nil {
		out = []CryptoQuote{}
	}
	return out
}

// ---------------- 顶部行情条 ----------------

// SeedTicker 聚合跑马灯快照
func SeedTicker() []TickerItem {
	mk := func(k string, base, pctRange float64) TickerItem {
		return TickerItem{Key: k, Value: fmt.Sprintf("%.2f", jitter(base, 0.003)), ChangePct: round2((rand.Float64() - 0.45) * pctRange)}
	}
	return []TickerItem{
		mk("黄金", 2356.4, 1.6), mk("白银", 30.18, 2.4), mk("铜", 4.41, 1.6), mk("WTI", 81.92, 3),
		mk("标普500", 5487, 1.4), mk("纳指", 17860, 1.8), mk("日经225", 39120, 1.2),
		mk("KOSPI", 2745, 1.2), mk("台湾加权", 23180, 1.6), mk("美元指数", 104.3, 0.6),
		mk("BTC", 61240, 4),
	}
}

// ---------------- 宏观 ----------------

// SeedHormuz 返回霍尔木兹通行量（近 30 日序列 + 当日）
func SeedHormuz() Hormuz {
	const days = 30
	series := make([]HormuzPoint, days)
	v := 100.0
	day := time.Now().AddDate(0, 0, -days+1)
	for i := 0; i < days; i++ {
		v += (rand.Float64() - 0.5) * 6
		series[i] = HormuzPoint{Date: day.Format("2006-01-02"), Value: int(v + 0.5)}
		day = day.AddDate(0, 0, 1)
	}
	today := series[days-1].Value
	prev := series[days-2].Value
	chg := 0.0
	if prev != 0 {
		chg = round2(float64(today-prev) / float64(prev) * 100)
	}
	return Hormuz{Today: today, ChangePct: chg, Unit: "艘/日", Series: series, Source: "seed", UpdatedAt: nowMS()}
}

// SeedFedWatch 返回 FedWatch 议息概率
func SeedFedWatch() FedWatch {
	return FedWatch{
		Meeting: "7月 FOMC", MeetingDate: "2026-07-29",
		Outcomes: []FedOutcome{
			{Label: "维持不变", Prob: 58}, {Label: "降息25bp", Prob: 38}, {Label: "降息50bp", Prob: 4},
		},
		UpdatedAt: nowMS(),
	}
}

// SeedBriefing 返回每日 AI 简报
func SeedBriefing(date string) AIBriefing {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	return AIBriefing{
		Date:     date,
		Headline: "原油与避险金属同步走强，市场在霍尔木兹紧张与降息预期之间反复定价。",
		Body:     "隔夜美股科技与能源领涨，标普收高 0.6%；WTI 原油因霍尔木兹海峡通行量回落而跳涨逾 2%，黄金、白银同步走高。CME FedWatch 显示 7 月维持利率不变概率升至 58%。亚洲早盘日经高开、台股受半导体带动走强，韩国 KOSPI 震荡。",
		Points: []string{
			"WTI 原油 +2.3%，霍尔木兹通行量环比回落",
			"黄金创阶段新高，避险与再通胀交易共振",
			"FedWatch：7 月维持不变概率升至 58%",
			"隔夜美股能源 / 科技领涨，公用事业走弱",
			"日经高开、台股半导体强势，KOSPI 震荡",
		},
		Tags:        []string{"原油", "避险金属", "FedWatch", "霍尔木兹", "半导体"},
		GeneratedAt: nowMS(),
	}
}
