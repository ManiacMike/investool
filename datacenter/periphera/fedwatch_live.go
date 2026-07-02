package periphera

// FedWatch 真实源：默认通过 Investing Fed Rate Monitor 免费页抓取第一张会议卡，
// 写入内存缓存；也保留 CME FedWatch REST API（或兼容 JSON 代理）接入能力。
// 失败或未就绪时由 api 层回退 SeedFedWatch。
//
// 配置：
//   PERIPHERA_FEDWATCH_SOURCE    investing(默认) / cme / json
//   PERIPHERA_FEDWATCH_URL       数据 URL；investing 默认 Fed Rate Monitor，cme 默认 intraday latest
//   PERIPHERA_FEDWATCH_PROXY     investing 抓取代理（默认 http://127.0.0.1:33210，设 none 禁用）
//   PERIPHERA_FEDWATCH_TOKEN     CME/代理 Bearer token；若值已带 Bearer/Basic 前缀则原样使用
//   PERIPHERA_FEDWATCH_INTERVAL  刷新间隔秒（默认 300）

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultFedWatchURL = "https://markets.api.cmegroup.com/fedwatch_rt/v1/forecasts/latest"
const defaultInvestingFedWatchURL = "https://www.investing.com/central-banks/fed-rate-monitor"
const defaultInvestingProxyURL = "http://127.0.0.1:33210"

type fedWatchStore struct {
	mu      sync.RWMutex
	current FedWatch
	ok      bool
}

var fedwatchLive = &fedWatchStore{}
var fedwatchOnce sync.Once

func StartFedWatch() { ensureFedWatch() }

func ensureFedWatch() {
	fedwatchOnce.Do(func() {
		if fedWatchURL() == "" {
			return
		}
		interval := envIntDefault("PERIPHERA_FEDWATCH_INTERVAL", 300)
		go func() {
			refreshFedWatch()
			t := time.NewTicker(time.Duration(interval) * time.Second)
			defer t.Stop()
			for range t.C {
				refreshFedWatch()
			}
		}()
	})
}

func fedWatchURL() string {
	if u := strings.TrimSpace(os.Getenv("PERIPHERA_FEDWATCH_URL")); u != "" {
		return u
	}
	if fedWatchSource() == "investing" {
		return defaultInvestingFedWatchURL
	}
	if strings.TrimSpace(os.Getenv("PERIPHERA_FEDWATCH_TOKEN")) != "" {
		return defaultFedWatchURL
	}
	return ""
}

func refreshFedWatch() {
	url := fedWatchURL()
	if url == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var (
		fw  FedWatch
		err error
	)
	if fedWatchSource() == "investing" {
		var body []byte
		body, err = fetchBytesClient(ctx, url, investingHeaders(), investingHTTPClient())
		if err == nil {
			fw, err = parseInvestingFedWatch(body, time.Now())
		}
	} else {
		var body []byte
		body, err = fetchBytes(ctx, url, fedWatchHeaders())
		if err == nil {
			fw, err = parseFedWatch(body, time.Now())
		}
	}
	if err != nil {
		return
	}
	fedwatchLive.record(fw)
}

func fedWatchSource() string {
	source := strings.ToLower(strings.TrimSpace(os.Getenv("PERIPHERA_FEDWATCH_SOURCE")))
	switch source {
	case "", "investing", "cme", "json":
		if source == "" {
			return "investing"
		}
		return source
	default:
		return "investing"
	}
}

func fedWatchHeaders() map[string]string {
	h := map[string]string{
		"Accept":                  "application/json",
		"CME-Application-Name":    envOr("PERIPHERA_FEDWATCH_APP_NAME", "investool-periphera"),
		"CME-Application-Vendor":  envOr("PERIPHERA_FEDWATCH_APP_VENDOR", "investool"),
		"CME-Application-Version": envOr("PERIPHERA_FEDWATCH_APP_VERSION", "1"),
	}
	if token := strings.TrimSpace(os.Getenv("PERIPHERA_FEDWATCH_TOKEN")); token != "" {
		lower := strings.ToLower(token)
		if !strings.HasPrefix(lower, "bearer ") && !strings.HasPrefix(lower, "basic ") {
			token = "Bearer " + token
		}
		h["Authorization"] = token
	}
	return h
}

func investingHeaders() map[string]string {
	return map[string]string{
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language":           "en-US,en;q=0.9",
		"Cache-Control":             "no-cache",
		"Pragma":                    "no-cache",
		"Referer":                   "https://www.investing.com/",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "same-origin",
		"Upgrade-Insecure-Requests": "1",
	}
}

func investingHTTPClient() *http.Client {
	proxyRaw := strings.TrimSpace(envOr("PERIPHERA_FEDWATCH_PROXY", defaultInvestingProxyURL))
	if proxyRaw == "" || strings.EqualFold(proxyRaw, "none") {
		return scrapeHTTPClient
	}
	proxyURL, err := url.Parse(proxyRaw)
	if err != nil {
		return scrapeHTTPClient
	}
	return &http.Client{
		Timeout: 45 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
}

func (s *fedWatchStore) record(fw FedWatch) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = fw
	s.ok = true
}

func LiveFedWatch() (FedWatch, bool) {
	ensureFedWatch()
	fedwatchLive.mu.RLock()
	defer fedwatchLive.mu.RUnlock()
	if !fedwatchLive.ok {
		return FedWatch{}, false
	}
	return fedwatchLive.current, true
}

type fedWatchCompat struct {
	Meeting     string       `json:"meeting"`
	MeetingDate string       `json:"meeting_date"`
	Outcomes    []FedOutcome `json:"outcomes"`
	UpdatedAt   int64        `json:"updated_at"`
}

type cmeFedWatchResp struct {
	Payload []cmeFedWatchForecast `json:"payload"`
}

type cmeFedWatchForecast struct {
	CalculationTimestamp string             `json:"calculationTimestamp"`
	CurrentReportingRt   string             `json:"currentReportingRt"`
	MeetingDt            string             `json:"meetingDt"`
	ReportingDt          string             `json:"reportingDt"`
	RateRange            []cmeFedWatchRange `json:"rateRange"`
}

type cmeFedWatchRange struct {
	LowerRt     int      `json:"lowerRt"`
	UpperRt     int      `json:"upperRt"`
	Probability *float64 `json:"probability"`
}

func parseFedWatch(body []byte, now time.Time) (FedWatch, error) {
	if fw, ok := parseFedWatchCompat(body); ok {
		return fw, nil
	}

	var cme cmeFedWatchResp
	if err := json.Unmarshal(body, &cme); err != nil {
		return FedWatch{}, err
	}
	f, ok := chooseFedWatchForecast(cme.Payload, now)
	if !ok {
		return FedWatch{}, fmt.Errorf("fedwatch empty payload")
	}
	return cmeForecastToFedWatch(f, now)
}

var (
	investingFirstCardRE = regexp.MustCompile(`(?is)<div\s+class=["'][^"']*\bcardWrapper\b[^"']*["'][^>]*>(.*?)(?:<div\s+class=["'][^"']*\bcardWrapper\b|$)`)
	investingDateRE      = regexp.MustCompile(`(?is)<div\s+class=["'][^"']*\bfedRateDate\b[^"']*["'][^>]*>\s*(.*?)\s*</div>`)
	investingRowRE       = regexp.MustCompile(`(?is)<tr>\s*<td\s+class=["']left["'][^>]*>\s*([0-9.]+\s*-\s*[0-9.]+).*?</td>\s*<td>\s*([0-9.]+)%\s*</td>`)
	investingUpdateRE    = regexp.MustCompile(`(?is)<div\s+class=["'][^"']*\bfedUpdate\b[^"']*["'][^>]*>\s*Updated:\s*(.*?)\s*</div>`)
)

func parseInvestingFedWatch(body []byte, now time.Time) (FedWatch, error) {
	card := firstInvestingFedWatchCard(string(body))
	if card == "" {
		return FedWatch{}, fmt.Errorf("investing fedwatch card not found")
	}
	meetingDateRaw := htmlText(firstSubmatch(investingDateRE, card))
	meetingDate, ok := parseInvestingDate(meetingDateRaw)
	if !ok {
		return FedWatch{}, fmt.Errorf("investing fedwatch meeting date not found")
	}
	outcomes, err := parseInvestingOutcomes(card)
	if err != nil {
		return FedWatch{}, err
	}
	updatedAt := now.UnixMilli()
	if updated, ok := parseInvestingUpdatedAt(htmlText(firstSubmatch(investingUpdateRE, card))); ok {
		updatedAt = updated.UnixMilli()
	}
	return FedWatch{
		Meeting:     meetingLabel(meetingDate),
		MeetingDate: meetingDate,
		Outcomes:    outcomes,
		UpdatedAt:   updatedAt,
	}, nil
}

func firstInvestingFedWatchCard(raw string) string {
	if m := investingFirstCardRE.FindStringSubmatch(raw); len(m) == 2 {
		return m[1]
	}
	return ""
}

func parseInvestingOutcomes(card string) ([]FedOutcome, error) {
	matches := investingRowRE.FindAllStringSubmatch(card, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("investing fedwatch probabilities not found")
	}
	type row struct {
		label string
		low   float64
		high  float64
		prob  float64
	}
	rows := make([]row, 0, len(matches))
	var currentHigh float64
	for _, m := range matches {
		low, high, ok := parsePercentRateRange(htmlText(m[1]))
		if !ok {
			continue
		}
		prob, err := strconv.ParseFloat(strings.TrimSpace(m[2]), 64)
		if err != nil {
			continue
		}
		if high > currentHigh {
			currentHigh = high
		}
		rows = append(rows, row{low: low, high: high, prob: prob})
	}
	if len(rows) == 0 || currentHigh <= 0 {
		return nil, fmt.Errorf("investing fedwatch invalid probabilities")
	}
	out := make([]FedOutcome, 0, len(rows))
	for _, r := range rows {
		out = append(out, FedOutcome{Label: investingRateLabel(r.high, currentHigh), Prob: r.prob})
	}
	return normalizeOutcomes(out), nil
}

func parsePercentRateRange(s string) (float64, float64, bool) {
	lo, hi, ok := strings.Cut(strings.TrimSpace(s), "-")
	if !ok {
		return 0, 0, false
	}
	l, err1 := strconv.ParseFloat(strings.TrimSpace(lo), 64)
	h, err2 := strconv.ParseFloat(strings.TrimSpace(hi), 64)
	return l, h, err1 == nil && err2 == nil
}

func investingRateLabel(high, currentHigh float64) string {
	diffBP := int(mathRound((currentHigh - high) * 100))
	switch {
	case diffBP > 0:
		return fmt.Sprintf("降息%dbp", diffBP)
	case diffBP < 0:
		return fmt.Sprintf("加息%dbp", -diffBP)
	default:
		return "维持不变"
	}
}

func parseInvestingDate(s string) (string, bool) {
	t, err := time.Parse("Jan 02, 2006", strings.TrimSpace(s))
	if err != nil {
		return "", false
	}
	return t.Format("2006-01-02"), true
}

func parseInvestingUpdatedAt(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		"Jan 02, 2006 03:04PM MST",
		"Jan 2, 2006 03:04PM MST",
		"Jan 02, 2006 3:04PM MST",
		"Jan 2, 2006 3:04PM MST",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func htmlText(s string) string {
	s = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}

func firstSubmatch(re *regexp.Regexp, raw string) string {
	if m := re.FindStringSubmatch(raw); len(m) >= 2 {
		return m[1]
	}
	return ""
}

func mathRound(v float64) float64 {
	if v < 0 {
		return float64(int(v - 0.5))
	}
	return float64(int(v + 0.5))
}

func parseFedWatchCompat(body []byte) (FedWatch, bool) {
	var compat fedWatchCompat
	if err := json.Unmarshal(body, &compat); err != nil {
		return FedWatch{}, false
	}
	if compat.MeetingDate == "" || len(compat.Outcomes) == 0 {
		return FedWatch{}, false
	}
	fw := FedWatch(compat)
	if fw.Meeting == "" {
		fw.Meeting = meetingLabel(fw.MeetingDate)
	}
	if fw.UpdatedAt == 0 {
		fw.UpdatedAt = nowMS()
	}
	fw.Outcomes = normalizeOutcomes(fw.Outcomes)
	return fw, len(fw.Outcomes) > 0
}

func chooseFedWatchForecast(items []cmeFedWatchForecast, now time.Time) (cmeFedWatchForecast, bool) {
	if len(items) == 0 {
		return cmeFedWatchForecast{}, false
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].MeetingDt < items[j].MeetingDt
	})
	for _, it := range items {
		if d, ok := parseYYYYMMDD(it.MeetingDt); ok && !d.Before(startOfDay(now)) {
			return it, true
		}
	}
	return items[0], true
}

func cmeForecastToFedWatch(f cmeFedWatchForecast, now time.Time) (FedWatch, error) {
	if f.MeetingDt == "" || len(f.RateRange) == 0 {
		return FedWatch{}, fmt.Errorf("fedwatch missing meeting/rates")
	}
	curLow, curHigh, ok := parseRateRange(f.CurrentReportingRt)
	if !ok {
		return FedWatch{}, fmt.Errorf("fedwatch invalid currentReportingRt: %s", f.CurrentReportingRt)
	}
	outcomes := make([]FedOutcome, 0, len(f.RateRange))
	for _, r := range f.RateRange {
		if r.Probability == nil || *r.Probability <= 0 {
			continue
		}
		outcomes = append(outcomes, FedOutcome{
			Label: cmeRateLabel(curLow, curHigh, r.LowerRt, r.UpperRt),
			Prob:  round2(probabilityPct(*r.Probability)),
		})
	}
	outcomes = normalizeOutcomes(outcomes)
	if len(outcomes) == 0 {
		return FedWatch{}, fmt.Errorf("fedwatch empty outcomes")
	}
	updatedAt := now.UnixMilli()
	if ts, ok := parseCMETimestamp(f.CalculationTimestamp); ok {
		updatedAt = ts.UnixMilli()
	}
	return FedWatch{
		Meeting:     meetingLabel(f.MeetingDt),
		MeetingDate: f.MeetingDt,
		Outcomes:    outcomes,
		UpdatedAt:   updatedAt,
	}, nil
}

func normalizeOutcomes(items []FedOutcome) []FedOutcome {
	out := make([]FedOutcome, 0, len(items))
	for _, it := range items {
		it.Label = strings.TrimSpace(it.Label)
		if it.Label == "" || it.Prob <= 0 {
			continue
		}
		out = append(out, FedOutcome{Label: it.Label, Prob: round2(it.Prob)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Prob > out[j].Prob })
	return out
}

func parseRateRange(s string) (int, int, bool) {
	lo, hi, ok := strings.Cut(strings.TrimSpace(s), "-")
	if !ok {
		return 0, 0, false
	}
	l, err1 := strconv.Atoi(strings.TrimSpace(lo))
	h, err2 := strconv.Atoi(strings.TrimSpace(hi))
	return l, h, err1 == nil && err2 == nil
}

func cmeRateLabel(curLow, curHigh, low, high int) string {
	step := 25
	if curHigh > curLow {
		step = curHigh - curLow
	}
	switch {
	case high <= curLow:
		return fmt.Sprintf("降息%dbp", (curLow-low)/step*step)
	case low >= curHigh:
		return fmt.Sprintf("加息%dbp", (low-curHigh)/step*step)
	default:
		return "维持不变"
	}
}

func probabilityPct(v float64) float64 {
	if v <= 1 {
		return v * 100
	}
	return v
}

func meetingLabel(date string) string {
	t, ok := parseYYYYMMDD(date)
	if !ok {
		return "下次 FOMC"
	}
	return fmt.Sprintf("%d月 FOMC", int(t.Month()))
}

func parseYYYYMMDD(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	return t, err == nil
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func parseCMETimestamp(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
