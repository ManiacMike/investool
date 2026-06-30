package periphera

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"
)

// 霍尔木兹海峡通行量：解析 Windward 公开页文本得到「某日真实通行总数」，
// 按日期累积进内存历史（仅真实观测点，不伪造）。series 随后台运行天数增长；
// change_pct 用真实相邻观测日计算，不足两天时为 0。解析失败保留上次缓存。

type hormuzObs struct {
	total    int
	inbound  int
	outbound int
	day      time.Time
}

type hormuzStore struct {
	mu        sync.RWMutex
	history   map[string]int // date(YYYY-MM-DD) -> 当日真实通行总数
	today     int
	todayDate string
	updatedAt int64
	ok        bool
}

var hormuzLive = &hormuzStore{history: map[string]int{}}
var hormuzOnce sync.Once

func StartHormuz() { ensureHormuz() }

func ensureHormuz() {
	hormuzOnce.Do(func() {
		go func() {
			refreshHormuz()
			t := time.NewTicker(15 * time.Minute)
			defer t.Stop()
			for range t.C {
				refreshHormuz()
			}
		}()
	})
}

func refreshHormuz() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	body, err := fetchBytes(ctx, "https://insights.windward.ai/", nil)
	if err != nil {
		return
	}
	obs, ok := parseWindwardHormuz(string(body), time.Now())
	if !ok {
		return
	}
	hormuzLive.record(obs)
}

// record 把一次真实观测并入历史（同日覆盖为最新值）。
func (s *hormuzStore) record(obs hormuzObs) {
	date := obs.day.Format("2006-01-02")
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.history == nil {
		s.history = map[string]int{}
	}
	s.history[date] = obs.total
	s.today = obs.total
	s.todayDate = date
	s.updatedAt = nowMS()
	s.ok = true
}

// LiveHormuz 用真实历史观测点构造响应；不足数据时 series 较短（诚实）。
func LiveHormuz() (Hormuz, bool) {
	ensureHormuz()
	return hormuzLive.build()
}

// build 从真实累积历史构造 Hormuz（仅真实观测日，最多近 30 个）。
func (s *hormuzStore) build() (Hormuz, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ok {
		return Hormuz{}, false
	}
	dates := make([]string, 0, len(s.history))
	for d := range s.history {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	if len(dates) > 30 {
		dates = dates[len(dates)-30:]
	}
	series := make([]HormuzPoint, 0, len(dates))
	for _, d := range dates {
		series = append(series, HormuzPoint{Date: d, Value: s.history[d]})
	}
	chg := 0.0
	if len(series) >= 2 {
		if prev := series[len(series)-2].Value; prev > 0 {
			chg = round2(float64(s.today-prev) / float64(prev) * 100)
		}
	}
	return Hormuz{
		Today:     s.today,
		ChangePct: chg,
		Unit:      "艘/日",
		Series:    series,
		Source:    "windward",
		UpdatedAt: s.updatedAt,
	}, true
}

var (
	windwardTotalRE = regexp.MustCompile(`(?i)(\d+)\s+vessels?\s+transited\s+the\s+Strait\s+of\s+Hormuz`)
	windwardAltRE   = regexp.MustCompile(`(?i)Strait\s+of\s+Hormuz\s+transits?\s+stand\s+at\s+(\d+)`)
	windwardDirA    = regexp.MustCompile(`(?i)(\d+)\s+inbound\s*/\s*(\d+)\s+outbound`)
	windwardDirB    = regexp.MustCompile(`(?i)(\d+)\s+outbound\s*,?\s*(\d+)\s+inbound`)
	windwardDirC    = regexp.MustCompile(`(?i)(\d+)\s+inbound\s+and\s+(\d+)\s+outbound`)
	// 句式："28 vessels transited inbound and 14 outbound through the Strait of Hormuz"
	windwardDirD = regexp.MustCompile(`(?i)(\d+)\s+vessels?\s+transited\s+inbound\s+and\s+(\d+)\s+outbound`)
	windwardDateRE  = regexp.MustCompile(`(?i)on\s+(\d{1,2}\s+[A-Za-z]+\s+\d{4})`)
	windwardDateB   = regexp.MustCompile(`(?i)On\s+(\d{1,2}\s+[A-Za-z]+\s+\d{4}),`)
	windwardDateC   = regexp.MustCompile(`(?i)on\s+(\d{1,2}\s+[A-Za-z]+)(?:\s|,|\.)`)
)

// parseWindwardHormuz 从页面文本提取一次真实观测（总数 + 方向 + 日期）；失败返回 ok=false。
func parseWindwardHormuz(raw string, now time.Time) (hormuzObs, bool) {
	text := cleanText(stripHTML(raw))
	total := firstInt(text, windwardTotalRE, windwardAltRE)
	inbound, outbound := 0, 0
	if m := windwardDirD.FindStringSubmatch(text); len(m) == 3 {
		inbound, _ = strconv.Atoi(m[1])
		outbound, _ = strconv.Atoi(m[2])
	} else if m := windwardDirA.FindStringSubmatch(text); len(m) == 3 {
		inbound, _ = strconv.Atoi(m[1])
		outbound, _ = strconv.Atoi(m[2])
	} else if m := windwardDirB.FindStringSubmatch(text); len(m) == 3 {
		outbound, _ = strconv.Atoi(m[1])
		inbound, _ = strconv.Atoi(m[2])
	} else if m := windwardDirC.FindStringSubmatch(text); len(m) == 3 {
		inbound, _ = strconv.Atoi(m[1])
		outbound, _ = strconv.Atoi(m[2])
	}
	if total == 0 && inbound+outbound > 0 {
		total = inbound + outbound
	}
	if total <= 0 {
		return hormuzObs{}, false
	}
	day := now
	if d, ok := parseWindwardDate(text, now); ok {
		day = d
	}
	return hormuzObs{total: total, inbound: inbound, outbound: outbound, day: day}, true
}

func parseWindwardDate(text string, now time.Time) (time.Time, bool) {
	if m := windwardDateRE.FindStringSubmatch(text); len(m) == 2 {
		if t, err := time.Parse("2 January 2006", m[1]); err == nil {
			return t, true
		}
	}
	if m := windwardDateB.FindStringSubmatch(text); len(m) == 2 {
		if t, err := time.Parse("2 January 2006", m[1]); err == nil {
			return t, true
		}
	}
	if m := windwardDateC.FindStringSubmatch(text); len(m) == 2 {
		if t, err := time.Parse("2 January 2006", m[1]+" "+strconv.Itoa(now.Year())); err == nil {
			if t.After(now.AddDate(0, 0, 1)) {
				t = t.AddDate(-1, 0, 0)
			}
			return t, true
		}
	}
	return time.Time{}, false
}

func firstInt(text string, patterns ...*regexp.Regexp) int {
	for _, p := range patterns {
		m := p.FindStringSubmatch(text)
		if len(m) >= 2 {
			n, _ := strconv.Atoi(m[1])
			if n > 0 {
				return n
			}
		}
	}
	return 0
}
