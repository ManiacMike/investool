package periphera

// 实时新闻真实源：后台 cron 通过 os/exec 调 Python 旁路脚本（twscrape 抓 X），
// 解析 JSON 灌入内存环形缓存（按 id 去重、按 published_at 倒序、上限 ~200 条）。
// 抓取失败时保留上次缓存（降级，绝不打挂前端）。读接口 LiveNews* 缓存为空时
// 返回 ok=false，由 api 层回退 Seed*。
//
// 凭据/代理全在 Python 脚本侧从项目根 .env 读取（x_auth_token/x_ct0/X_PROXY），
// 不经过 Go 进程。可用环境变量覆盖运行参数：
//   PERIPHERA_TWITTER_PYTHON   python 解释器路径（默认 scripts/venv/bin/python）
//   PERIPHERA_TWITTER_SCRIPT   抓取脚本路径（默认 scripts/twitter_fetch.py）
//   PERIPHERA_TWITTER_TARGETS  目标账号（默认脚本内置 elonmusk,aleabitoreddit）
//   PERIPHERA_TWITTER_LIMIT    每账号抓取条数（默认 20）
//   PERIPHERA_TWITTER_INTERVAL 刷新间隔秒（默认 300）

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"sync"
	"time"
)

// twItem 对应 scripts/twitter_fetch.py 输出的单条
type twItem struct {
	ID          string   `json:"id"`
	Source      string   `json:"source"`
	SourceName  string   `json:"source_name"`
	Author      string   `json:"author"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	URL         string   `json:"url"`
	PublishedAt int64    `json:"published_at"`
	Media       []string `json:"media"`
	Tags        []string `json:"tags"`
}

// twOut 抓取脚本的整体输出
type twOut struct {
	OK     bool     `json:"ok"`
	Items  []twItem `json:"items"`
	Errors []string `json:"errors"`
	Error  string   `json:"error"`
}

// 来源 code -> 展示元数据（tag/color，与 SeedNewsSources 对齐）
var newsSourceMeta = map[string]struct{ tag, color, desc string }{
	"musk":      {"EM", "#E7A265", "科技 / 情绪面"},
	"baimao":    {"BM", "#8a6d3b", "盘面观点解读"},
	"reuters":   {"RT", "#D0643E", "全球财经快讯"},
	"bloomberg": {"BB", "#3D362A", "市场与宏观"},
}

type newsStore struct {
	mu        sync.RWMutex
	byID      map[string]NewsItem
	order     []string // 按 published_at 倒序的 id 列表
	updatedAt int64
	ok        bool
}

var news = &newsStore{byID: map[string]NewsItem{}}
var newsOnce sync.Once

const newsCap = 200

// StartNews 服务启动时开启新闻后台抓取（首拉异步，不阻塞启动）。
func StartNews() { ensureNews() }

func ensureNews() {
	newsOnce.Do(func() {
		interval := envIntDefault("PERIPHERA_TWITTER_INTERVAL", 300)
		go func() {
			refreshNews()
			t := time.NewTicker(time.Duration(interval) * time.Second)
			defer t.Stop()
			for range t.C {
				refreshNews()
			}
		}()
	})
}

func envIntDefault(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// refreshNews 合并 RSS 与 Python 抓取结果；失败源不影响成功源。
func refreshNews() {
	rssCtx, rssCancel := context.WithTimeout(context.Background(), 60*time.Second)
	items := fetchNewsRSS(rssCtx)
	rssCancel()

	python := envOr("PERIPHERA_TWITTER_PYTHON", "scripts/venv/bin/python")
	script := envOr("PERIPHERA_TWITTER_SCRIPT", "scripts/twitter_fetch.py")
	limit := envIntDefault("PERIPHERA_TWITTER_LIMIT", 20)

	args := []string{script, "--limit", strconv.Itoa(limit)}
	if t := os.Getenv("PERIPHERA_TWITTER_TARGETS"); t != "" {
		args = append(args, "--targets", t)
	}

	twCtx, twCancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer twCancel()
	cmd := exec.CommandContext(twCtx, python, args...)
	out, err := cmd.Output() // 仅取 stdout（脚本日志走 stderr）
	if err == nil && len(out) > 0 {
		var res twOut
		if json.Unmarshal(out, &res) == nil && res.OK && len(res.Items) > 0 {
			items = append(items, res.Items...)
		}
	}
	if len(items) == 0 {
		return
	}
	mergeNews(items)
}

// mergeNews 把抓取项并入缓存（去重、补 tag/color、限容、按时间倒序）。
func mergeNews(items []twItem) {
	news.mu.Lock()
	defer news.mu.Unlock()
	for _, it := range items {
		if it.ID == "" {
			continue
		}
		meta := newsSourceMeta[it.Source]
		news.byID[it.ID] = NewsItem{
			ID:          it.ID,
			Source:      it.Source,
			SourceName:  it.SourceName,
			Color:       meta.color,
			Tag:         meta.tag,
			Title:       it.Title,
			Summary:     it.Summary,
			URL:         it.URL,
			PublishedAt: it.PublishedAt,
			Hot:         false,
			Tags:        it.Tags,
		}
	}
	// 重建倒序 order
	ids := make([]string, 0, len(news.byID))
	for id := range news.byID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return news.byID[ids[i]].PublishedAt > news.byID[ids[j]].PublishedAt
	})
	if len(ids) > newsCap {
		for _, id := range ids[newsCap:] {
			delete(news.byID, id)
		}
		ids = ids[:newsCap]
	}
	news.order = ids
	news.updatedAt = nowMS()
	news.ok = true
}

// LiveNews 读缓存：按 source 过滤 + since 增量 + limit 截断，返回 (items, latest, ok)。
func LiveNews(source string, since int64, limit int) ([]NewsItem, int64, bool) {
	ensureNews()
	news.mu.RLock()
	defer news.mu.RUnlock()
	if !news.ok {
		return nil, 0, false
	}
	out := make([]NewsItem, 0, limit)
	var latest int64
	for _, id := range news.order {
		n := news.byID[id]
		if source != "" && source != "all" && n.Source != source {
			continue
		}
		if since > 0 && n.PublishedAt <= since {
			continue
		}
		out = append(out, n)
		if n.PublishedAt > latest {
			latest = n.PublishedAt
		}
		if len(out) >= limit {
			break
		}
	}
	return out, latest, true
}

// LiveNewsSources 读缓存生成来源栏（含计数），缓存为空返回 ok=false。
func LiveNewsSources() ([]NewsSource, bool) {
	ensureNews()
	news.mu.RLock()
	defer news.mu.RUnlock()
	if !news.ok {
		return nil, false
	}
	counts := map[string]int{}
	for _, id := range news.order {
		counts[news.byID[id].Source]++
	}
	// 固定来源顺序，计数为 0 也展示，便于前端筛选栏稳定
	order := []string{"reuters", "bloomberg", "musk", "baimao"}
	names := map[string]string{"reuters": "路透社", "bloomberg": "彭博社", "musk": "马斯克", "baimao": "白毛女神"}
	out := make([]NewsSource, 0, len(order))
	for _, code := range order {
		meta := newsSourceMeta[code]
		out = append(out, NewsSource{
			Code: code, Name: names[code], Tag: meta.tag,
			Color: meta.color, Desc: meta.desc, Count: counts[code],
		})
	}
	return out, true
}
