package periphera

// 真实行情层：后台每 3s 从 sina 拉一次，写入内存缓存（含每个 symbol 的价格
// 滚动窗口用于 sparkline / 原油分时）。Live* 读缓存返回契约类型；缓存未就绪
// 或对应 symbol 缺失时返回 ok=false，由 api 层回退到 Seed*。

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/axiaoxin-com/investool/datacenter/sina"
)

// ---- symbol 映射 ----

// 商品 code -> sina hf_ symbol（暗金/暗油＝连续电子盘，与日盘同源）
var comSymbols = map[string]string{
	"XAU": "hf_GC", "XAG": "hf_SI", "HG": "hf_HG", "WTI": "hf_CL",
	"XAU_AH": "hf_GC", "WTI_AH": "hf_CL",
}

type idxMap struct{ sym, fam string } // fam: int / znb

// 股指 code -> sina symbol + family
var idxSymbols = map[string]idxMap{
	"SPX": {"int_sp500", "int"}, "IXIC": {"int_nasdaq", "int"}, "DJI": {"int_dji", "int"},
	"N225": {"int_nikkei", "int"}, "TWSE": {"znb_TWSE", "znb"}, "KOSPI": {"znb_KOSPI", "znb"},
}

// 加密 code -> sina symbol（仅 BTC；ETH 走 Binance，见 crypto_gold_live.go）
var cryptoSymbols = map[string]string{"BTC": "hf_BTC"}

// 美股板块：SPDR 11 只行业 ETF -> 板块中文名，走 sina gb_（字段[2]=涨跌幅%）。
// 顺序仅为定义用，输出按涨跌幅倒序。
var sectorSymbols = []struct{ sym, name string }{
	{"gb_xle", "能源"}, {"gb_xlk", "信息技术"}, {"gb_xlc", "通信"},
	{"gb_xlf", "金融"}, {"gb_xlb", "材料"}, {"gb_xli", "工业"},
	{"gb_xlv", "医疗"}, {"gb_xly", "可选消费"}, {"gb_xlp", "必需消费"},
	{"gb_xlre", "房地产"}, {"gb_xlu", "公用"},
}

// crude 用到的两腿
const symWTI, symBrent = "hf_CL", "hf_OIL"

// allSymbols 收集所有要拉的 sina symbol（去重）
func allSymbols() []string {
	set := map[string]bool{}
	for _, v := range comSymbols {
		set[v] = true
	}
	for _, v := range idxSymbols {
		set[v.sym] = true
	}
	for _, v := range cryptoSymbols {
		set[v] = true
	}
	for _, v := range sectorSymbols {
		set[v.sym] = true
	}
	set[symBrent] = true
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// ---- 缓存 ----

type liveStore struct {
	mu        sync.RWMutex
	raw       map[string][]string  // 最近一次原始字段
	hist      map[string][]float64 // 每个 symbol 的价格滚动窗口
	updatedAt int64
	ok        bool
}

var store = &liveStore{raw: map[string][]string{}, hist: map[string][]float64{}}
var startOnce sync.Once

// StartLive 在服务启动时主动开启后台行情刷新，让首个请求即可拿到真实数据。
func StartLive() { ensureLive() }

// ensureLive 惰性启动后台刷新（仅一次）
func ensureLive() {
	startOnce.Do(func() {
		ensureCryptoGold()
		go func() {
			refreshLive()
			t := time.NewTicker(3 * time.Second)
			defer t.Stop()
			for range t.C {
				refreshLive()
			}
		}()
	})
}

// refreshLive 拉取一次并更新缓存；失败则保留上次缓存（降级）
func refreshLive() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	raw, err := sina.NewSina().FetchRaw(ctx, allSymbols())
	if err != nil || len(raw) == 0 {
		return
	}
	store.mu.Lock()
	store.raw = raw
	store.updatedAt = nowMS()
	store.ok = true
	for sym, f := range raw {
		p := priceOf(sym, f)
		if p > 0 {
			h := append(store.hist[sym], p)
			if len(h) > 60 {
				h = h[len(h)-60:]
			}
			store.hist[sym] = h
		}
	}
	store.mu.Unlock()
}

// ---- 字段解析 ----

func atof(f []string, i int) float64 {
	if i < 0 || i >= len(f) {
		return 0
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(f[i]), 64)
	if err != nil {
		return 0
	}
	return v
}

// priceOf 当前价：hf_ 在字段0，int_/znb_/gb_ 在字段1
func priceOf(sym string, f []string) float64 {
	switch {
	case strings.HasPrefix(sym, "hf_"):
		return atof(f, 0)
	case strings.HasPrefix(sym, "int_"), strings.HasPrefix(sym, "znb_"), strings.HasPrefix(sym, "gb_"):
		return atof(f, 1)
	}
	return 0
}

// chgPct 涨跌幅%：hf_ 用 (现价-昨结)/昨结；gb_ 在字段2；int_/znb_ 在字段3
func chgPct(sym string, f []string) float64 {
	if strings.HasPrefix(sym, "hf_") {
		cur, prev := atof(f, 0), atof(f, 7)
		if prev > 0 {
			return round2((cur - prev) / prev * 100)
		}
		return 0
	}
	if strings.HasPrefix(sym, "gb_") {
		return round2(atof(f, 2))
	}
	return round2(atof(f, 3))
}

func histCopy(sym string) []float64 {
	h := store.hist[sym]
	out := make([]float64, len(h))
	copy(out, h)
	return out
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---- Live 构造器（读缓存，返回契约类型 + ok）----

// LiveCommodities 真实商品行情
func LiveCommodities(codes []string) ([]Commodity, bool) {
	ensureLive()
	store.mu.RLock()
	defer store.mu.RUnlock()
	if !store.ok {
		return nil, false
	}
	out := []Commodity{}
	for _, s := range comSeeds {
		if !wantCode(codes, s.code) {
			continue
		}
		sym := comSymbols[s.code]
		f := store.raw[sym]
		if f == nil {
			continue
		}
		price := round2(priceOf(sym, f))
		pct := chgPct(sym, f)
		spark := histCopy(sym)
		if s.dark {
			spark = []float64{}
		}
		out = append(out, Commodity{
			Code: s.code, Name: s.name, Price: price,
			Change: round2(price * pct / 100), ChangePct: pct,
			Dark: s.dark, Spark: spark, Unit: s.unit, UpdatedAt: store.updatedAt,
		})
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// LiveIndices 真实股指行情
func LiveIndices(codes []string) ([]IndexQuote, bool) {
	ensureLive()
	store.mu.RLock()
	defer store.mu.RUnlock()
	if !store.ok {
		return nil, false
	}
	out := []IndexQuote{}
	for _, s := range idxSeeds {
		if !wantCode(codes, s.code) {
			continue
		}
		im, ok := idxSymbols[s.code]
		if !ok {
			continue
		}
		f := store.raw[im.sym]
		if f == nil {
			continue
		}
		value := round2(atof(f, 1))
		chg := round2(atof(f, 2))
		pct := round2(atof(f, 3))
		out = append(out, IndexQuote{
			Code: s.code, Name: s.name, Flag: s.flag, Value: value,
			Change: chg, ChangePct: pct, PrevClose: round2(value - chg),
			IsOpen: true, Spark: histCopy(im.sym), UpdatedAt: store.updatedAt,
		})
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// LiveSectors 真实美股板块涨幅（SPDR 11 行业 ETF via sina gb_），按涨跌幅倒序。
// 返回 trade_date（取 ETF 报价时间的日期）与 updated_at。
func LiveSectors() ([]SectorItem, string, int64, bool) {
	ensureLive()
	store.mu.RLock()
	defer store.mu.RUnlock()
	if !store.ok {
		return nil, "", 0, false
	}
	out := make([]SectorItem, 0, len(sectorSymbols))
	tradeDate := ""
	for _, s := range sectorSymbols {
		f := store.raw[s.sym]
		if f == nil {
			continue
		}
		if tradeDate == "" {
			tradeDate = sectorTradeDate(f)
		}
		out = append(out, SectorItem{Name: s.name, ChangePct: chgPct(s.sym, f)})
	}
	if len(out) == 0 {
		return nil, "", 0, false
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ChangePct > out[j].ChangePct })
	return out, tradeDate, store.updatedAt, true
}

// sectorTradeDate 从 gb_ 报价时间字段（形如 "2026-07-02 22:55:20"）取日期部分。
func sectorTradeDate(f []string) string {
	if len(f) > 3 {
		if d, _, ok := strings.Cut(strings.TrimSpace(f[3]), " "); ok {
			return d
		}
	}
	return ""
}

// LiveCrypto 真实加密行情（BTC via sina，ETH/PAXG/XAUT via Binance，均失败回退 seed）
func LiveCrypto(codes []string) ([]CryptoQuote, bool) {
	ensureLive()
	store.mu.RLock()
	defer store.mu.RUnlock()
	out := []CryptoQuote{}
	for _, s := range cryptoSeeds {
		if !wantCode(codes, s.code) {
			continue
		}
		if sym, ok := cryptoSymbols[s.code]; ok && store.ok {
			if f := store.raw[sym]; f != nil {
				out = append(out, CryptoQuote{
					Code: s.code, Name: s.name, Price: round2(priceOf(sym, f)),
					ChangePct: chgPct(sym, f), Spark: histCopy(sym), UpdatedAt: store.updatedAt,
				})
				continue
			}
		}
		if q, ok := liveCryptoGoldQuote(s.code); ok {
			out = append(out, q)
			continue
		}
		// 回退：单条 seed
		if seed := SeedCrypto([]string{s.code}); len(seed) > 0 {
			out = append(out, seed[0])
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// LiveCrude 真实原油分时（WTI + Brent，序列对齐到等长）
func LiveCrude() (CrudeResp, bool) {
	ensureLive()
	store.mu.RLock()
	defer store.mu.RUnlock()
	wf, bf := store.raw[symWTI], store.raw[symBrent]
	if !store.ok || wf == nil || bf == nil {
		return CrudeResp{}, false
	}
	w := histCopy(symWTI)
	b := histCopy(symBrent)
	n := minInt(len(w), len(b))
	w, b = w[len(w)-n:], b[len(b)-n:]
	return CrudeResp{
		Wti:    CrudeLeg{Price: round2(priceOf(symWTI, wf)), ChangePct: chgPct(symWTI, wf)},
		Brent:  CrudeLeg{Price: round2(priceOf(symBrent, bf)), ChangePct: chgPct(symBrent, bf)},
		Labels: make([]string, n), WtiSeries: w, BrentSeries: b, UpdatedAt: store.updatedAt,
	}, true
}

// tickerSpec 顶部行情条聚合项
type tickerSpec struct {
	key, sym string
}

var tickerSpecs = []tickerSpec{
	{"黄金", "hf_GC"}, {"白银", "hf_SI"}, {"铜", "hf_HG"}, {"WTI", "hf_CL"},
	{"标普500", "int_sp500"}, {"纳指", "int_nasdaq"}, {"日经225", "int_nikkei"},
	{"KOSPI", "znb_KOSPI"}, {"台湾加权", "znb_TWSE"}, {"BTC", "hf_BTC"},
}

// LiveTicker 真实跑马灯快照
func LiveTicker() ([]TickerItem, bool) {
	ensureLive()
	store.mu.RLock()
	defer store.mu.RUnlock()
	if !store.ok {
		return nil, false
	}
	out := []TickerItem{}
	for _, t := range tickerSpecs {
		f := store.raw[t.sym]
		if f == nil {
			continue
		}
		out = append(out, TickerItem{
			Key: t.key, Value: fmt.Sprintf("%.2f", priceOf(t.sym, f)), ChangePct: chgPct(t.sym, f),
		})
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}
