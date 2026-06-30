package periphera

// 每日 AI 简报真实源：火山引擎方舟（Ark，OpenAI 兼容）Seed/豆包模型生成，替代 Coze。
// 后台 cron 定期用当前真实行情 + 新闻头条作为上下文，让模型产出结构化简报，按日期缓存。
// 生成失败/未就绪时由 api 层回退 SeedBriefing，绝不打挂前端。
//
// 凭据/配置走环境变量（由 LoadDotEnv 从 .env 注入，真实 env 优先）：
//   SEED_API_KEY              Ark API Key（必填，缺失则不启用，直接走 seed）
//   SEED_BASE_URL             Ark 基址（默认 https://ark.cn-beijing.volces.com）
//   SEED_MODEL                模型 id（默认 doubao-seed-1-6-250615）
//   PERIPHERA_BRIEFING_INTERVAL  重生成间隔秒（默认 21600＝6h）

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultArkBaseURL = "https://ark.cn-beijing.volces.com"
	defaultArkModel   = "doubao-seed-1-6-250615"
)

// Ark 生成耗时（思考模型），用独立长超时 client，与 20s 的 scrapeHTTPClient 区分。
var arkHTTPClient = &http.Client{Timeout: 100 * time.Second}

type briefingStore struct {
	mu     sync.RWMutex
	byDate map[string]AIBriefing
}

var briefing = &briefingStore{byDate: map[string]AIBriefing{}}
var briefingOnce sync.Once

// StartBriefing 服务启动时开启简报后台生成（首次延迟，等行情/新闻缓存预热；不阻塞启动）。
func StartBriefing() {
	if os.Getenv("SEED_API_KEY") == "" {
		return // 未配置 key，保持 seed 回退
	}
	briefingOnce.Do(func() {
		interval := envIntDefault("PERIPHERA_BRIEFING_INTERVAL", 21600)
		go func() {
			time.Sleep(60 * time.Second) // 等 live 行情/新闻预热，让首篇简报有真实上下文
			genBriefingToday()
			t := time.NewTicker(time.Duration(interval) * time.Second)
			defer t.Stop()
			for range t.C {
				genBriefingToday()
			}
		}()
	})
}

// LiveBriefing 读缓存；命中返回 (简报, true)，否则 (空, false) 由 api 回退 seed。
func LiveBriefing(date string) (AIBriefing, bool) {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	briefing.mu.RLock()
	defer briefing.mu.RUnlock()
	b, ok := briefing.byDate[date]
	return b, ok
}

func genBriefingToday() {
	date := time.Now().Format("2006-01-02")
	b, err := generateBriefing(context.Background(), date)
	if err != nil {
		return // 保留上次缓存（若有），否则维持 seed 回退
	}
	briefing.mu.Lock()
	briefing.byDate[date] = b
	briefing.mu.Unlock()
}

// ---- Ark (OpenAI 兼容) 客户端 ----

type arkMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type arkRequest struct {
	Model       string       `json:"model"`
	Messages    []arkMessage `json:"messages"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Temperature float64      `json:"temperature,omitempty"`
}

type arkResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

// arkChat 调一次 Ark chat/completions，返回助手正文（content，忽略 reasoning_content）。
func arkChat(ctx context.Context, system, user string) (string, error) {
	key := os.Getenv("SEED_API_KEY")
	if key == "" {
		return "", fmt.Errorf("SEED_API_KEY not set")
	}
	base := envOr("SEED_BASE_URL", defaultArkBaseURL)
	model := envOr("SEED_MODEL", defaultArkModel)
	url := strings.TrimRight(base, "/") + "/api/v3/chat/completions"

	reqBody, _ := json.Marshal(arkRequest{
		Model: model,
		Messages: []arkMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		MaxTokens:   1200,
		Temperature: 0.7,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	resp, err := arkHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out arkResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Error != nil {
		return "", fmt.Errorf("ark error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("ark empty choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// ---- 简报生成 ----

const briefingSystemPrompt = `你是一名资深的全球宏观与大类资产策略分析师，为中文投资者撰写「每日外围市场 AI 简报」。
要求：客观、专业、信息密度高，聚焦原油/贵金属/全球股指/加密/宏观信号；不编造未提供的具体数字。
必须严格输出 JSON（不要 markdown、不要多余文字），格式：
{"headline": "一句话核心判断(40字内)", "body": "150-220字综述", "points": ["要点1","要点2","要点3","要点4","要点5"], "tags": ["标签1","标签2","标签3"]}`

// generateBriefing 用当前真实数据作上下文，请求模型产出结构化简报。
func generateBriefing(ctx context.Context, date string) (AIBriefing, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	userPrompt := fmt.Sprintf("日期：%s\n\n实时市场快照：\n%s\n\n近期外围新闻头条：\n%s\n\n请基于以上数据撰写今日简报。",
		date, briefingMarketContext(), briefingNewsContext())

	content, err := arkChat(ctx, briefingSystemPrompt, userPrompt)
	if err != nil {
		return AIBriefing{}, err
	}

	var parsed struct {
		Headline string   `json:"headline"`
		Body     string   `json:"body"`
		Points   []string `json:"points"`
		Tags     []string `json:"tags"`
	}
	if err := json.Unmarshal([]byte(extractJSON(content)), &parsed); err != nil {
		return AIBriefing{}, fmt.Errorf("parse briefing json: %w", err)
	}
	if parsed.Headline == "" || parsed.Body == "" {
		return AIBriefing{}, fmt.Errorf("briefing missing fields")
	}
	return AIBriefing{
		Date:        date,
		Headline:    parsed.Headline,
		Body:        parsed.Body,
		Points:      parsed.Points,
		Tags:        parsed.Tags,
		GeneratedAt: nowMS(),
	}, nil
}

// briefingMarketContext 汇总当前 live 行情（取不到则用 seed），供模型参考。
func briefingMarketContext() string {
	var sb strings.Builder
	coms, ok := LiveCommodities(nil)
	if !ok {
		coms = SeedCommodities(nil)
	}
	for _, c := range coms {
		fmt.Fprintf(&sb, "- %s %.2f %s (%+.2f%%)\n", c.Name, c.Price, c.Unit, c.ChangePct)
	}
	idx, ok := LiveIndices(nil)
	if !ok {
		idx = SeedIndices(nil)
	}
	for _, i := range idx {
		fmt.Fprintf(&sb, "- %s %.2f (%+.2f%%)\n", i.Name, i.Value, i.ChangePct)
	}
	cry, ok := LiveCrypto(nil)
	if !ok {
		cry = SeedCrypto(nil)
	}
	for _, c := range cry {
		fmt.Fprintf(&sb, "- %s %.2f (%+.2f%%)\n", c.Name, c.Price, c.ChangePct)
	}
	return sb.String()
}

// briefingNewsContext 取近 12 条新闻头条（live 优先）。
func briefingNewsContext() string {
	items, _, ok := LiveNews("all", 0, 12)
	if !ok {
		items = SeedNews()
		if len(items) > 12 {
			items = items[:12]
		}
	}
	var sb strings.Builder
	for _, n := range items {
		fmt.Fprintf(&sb, "- [%s] %s\n", n.SourceName, n.Title)
	}
	return sb.String()
}

// extractJSON 从模型输出里截取第一个 {...} JSON 块（容忍 markdown 包裹）。
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
