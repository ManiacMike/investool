// Package periphera 为 PERIPHERA 投资资讯指挥中心（/portal /news /research）提供数据。
//
// M0 阶段：所有数据由本包的 Seed* 函数生成「契约合法」的数据，供前端按
// PERIPHERA_API.md 立即对接；真实数据源（sina/qq 行情、twscrape 新闻、
// MySQL 研报、宏观）在后续里程碑逐个替换，对外 JSON 契约保持不变。
package periphera

import (
	"math"
	"math/rand"
	"strings"
	"time"
)

// nowMS 当前 Unix 毫秒
func nowMS() int64 { return time.Now().UnixMilli() }

// NowMS 当前 Unix 毫秒（供 api 层 server_ts 使用）
func NowMS() int64 { return nowMS() }

// walk 生成一段拟真随机游走序列（用于 sparkline / 分时占位）
func walk(n int, base, vol, drift float64) []float64 {
	out := make([]float64, n)
	v := base
	for i := 0; i < n; i++ {
		v += (rand.Float64()-0.5)*vol + drift
		out[i] = round2(v)
	}
	return out
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

// jitter 在基准值上加一个相对小幅扰动，让占位行情看起来在跳动
func jitter(base, pct float64) float64 { return round2(base * (1 + (rand.Float64()-0.5)*pct)) }

// wantCode 用于 ?codes= 过滤：codes 为空时全要
func wantCode(codes []string, code string) bool {
	if len(codes) == 0 {
		return true
	}
	for _, c := range codes {
		if strings.EqualFold(strings.TrimSpace(c), code) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 类型定义（json tag 严格对齐 PERIPHERA_API.md，字段尽量与前端 mock 同名）
// ---------------------------------------------------------------------------

// NewsItem 新闻条目
type NewsItem struct {
	ID          string   `json:"id"`
	Source      string   `json:"source"`
	SourceName  string   `json:"source_name"`
	Color       string   `json:"color"`
	Tag         string   `json:"tag"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	URL         string   `json:"url"`
	PublishedAt int64    `json:"published_at"`
	Hot         bool     `json:"hot"`
	Tags        []string `json:"tags"`
}

// NewsSource 新闻来源（左侧来源栏）
type NewsSource struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Tag   string `json:"tag"`
	Color string `json:"color"`
	Desc  string `json:"desc"`
	Count int    `json:"count"`
}

// ResearchReport 外资研报（字段同 zen-research-report 的 REPORT_FIELDS）
type ResearchReport struct {
	ID               string `json:"id"`
	PublishTime      string `json:"publish_time"`
	IndustryCategory string `json:"industry_category"`
	InstitutionName  string `json:"institution_name"`
	ResearchTarget   string `json:"research_target"`
	ReportType       string `json:"report_type"`
	CoreContent      string `json:"core_content"`
	TargetPrice      string `json:"target_price"`
	InvestmentRating string `json:"investment_rating"`
	RatingChange     string `json:"rating_change"`
	CoreCatalyst     string `json:"core_catalyst"`
	CoreRiskWarning  string `json:"core_risk_warning"`
	// 以下字段由抖音→豆包脚本产出
	EarningsForecastAdjustment string `json:"earnings_forecast_adjustment"`
	VideoID                    string `json:"video_id"`
	Author                     string `json:"author"`
	SourceURL                  string `json:"source_url"`
	CreatedAt                  int64  `json:"created_at"`
}

// IndexQuote 股指行情
type IndexQuote struct {
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Flag      string    `json:"flag"`
	Value     float64   `json:"value"`
	Change    float64   `json:"change"`
	ChangePct float64   `json:"change_pct"`
	PrevClose float64   `json:"prev_close"`
	IsOpen    bool      `json:"is_open"`
	Spark     []float64 `json:"spark"`
	UpdatedAt int64     `json:"updated_at"`
}

// SectorItem 美股板块涨幅
type SectorItem struct {
	Name      string  `json:"name"`
	ChangePct float64 `json:"change_pct"`
}

// Commodity 商品行情（金银铜油 + 暗金暗油）
type Commodity struct {
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	Change    float64   `json:"change"`
	ChangePct float64   `json:"change_pct"`
	Dark      bool      `json:"dark"`
	Spark     []float64 `json:"spark"`
	Unit      string    `json:"unit"`
	UpdatedAt int64     `json:"updated_at"`
}

// CrudeLeg 原油单腿（WTI / Brent）
type CrudeLeg struct {
	Price     float64 `json:"price"`
	ChangePct float64 `json:"change_pct"`
}

// CrudeResp 原油实时分时
type CrudeResp struct {
	Wti         CrudeLeg  `json:"wti"`
	Brent       CrudeLeg  `json:"brent"`
	Labels      []string  `json:"labels"`
	WtiSeries   []float64 `json:"wti_series"`
	BrentSeries []float64 `json:"brent_series"`
	UpdatedAt   int64     `json:"updated_at"`
}

// CryptoQuote 加密行情
type CryptoQuote struct {
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Price     float64   `json:"price"`
	ChangePct float64   `json:"change_pct"`
	Spark     []float64 `json:"spark"`
	UpdatedAt int64     `json:"updated_at"`
}

// TickerItem 顶部跑马灯条目
type TickerItem struct {
	Key       string  `json:"key"`
	Value     string  `json:"value"`
	ChangePct float64 `json:"change_pct"`
}

// HormuzPoint 霍尔木兹历史点
type HormuzPoint struct {
	Date  string `json:"date"`
	Value int    `json:"value"`
}

// Hormuz 霍尔木兹通行量
type Hormuz struct {
	Today     int           `json:"today"`
	ChangePct float64       `json:"change_pct"`
	Unit      string        `json:"unit"`
	Series    []HormuzPoint `json:"series"`
	Source    string        `json:"source"`
	UpdatedAt int64         `json:"updated_at"`
}

// FedOutcome FedWatch 单个结果概率
type FedOutcome struct {
	Label string  `json:"label"`
	Prob  float64 `json:"prob"`
}

// FedWatch 议息概率
type FedWatch struct {
	Meeting     string       `json:"meeting"`
	MeetingDate string       `json:"meeting_date"`
	Outcomes    []FedOutcome `json:"outcomes"`
	UpdatedAt   int64        `json:"updated_at"`
}

// AIBriefing 每日 AI 简报
type AIBriefing struct {
	Date        string   `json:"date"`
	Headline    string   `json:"headline"`
	Body        string   `json:"body"`
	Points      []string `json:"points"`
	Tags        []string `json:"tags"`
	GeneratedAt int64    `json:"generated_at"`
}
