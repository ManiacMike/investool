// PERIPHERA 投资资讯指挥中心数据接口（/api/v1/*）。
// 契约见仓库根目录 PERIPHERA_API.md。M0 阶段由 datacenter/periphera 的
// Seed* 提供契约合法数据，真实源后续逐个替换，对外 JSON 不变。
package api

import (
	"strconv"
	"strings"

	"github.com/axiaoxin-com/investool/datacenter/periphera"
	"github.com/axiaoxin-com/investool/routes/response"
	"github.com/axiaoxin-com/investool/version"
	"github.com/axiaoxin-com/logging"
	"github.com/gin-gonic/gin"
)

// ---- 参数解析小工具 ----

func pCodes(c *gin.Context) []string {
	raw := strings.TrimSpace(c.Query("codes"))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func pLimit(c *gin.Context, def, max int) int {
	n, err := strconv.Atoi(c.Query("limit"))
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

func pSince(c *gin.Context) int64 {
	n, err := strconv.ParseInt(c.Query("since"), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func pServerTS() int64 { return periphera.NowMS() }

// ---- 健康检查 ----

// PeripheraHealth GET /api/v1/health
func PeripheraHealth(c *gin.Context) {
	response.JSON(c, gin.H{"ok": true, "version": version.Version, "server_ts": pServerTS()})
}

// ---- 新闻 ----

// PeripheraNews GET /api/v1/news?source=&since=&limit=
func PeripheraNews(c *gin.Context) {
	source := strings.TrimSpace(c.Query("source"))
	since := pSince(c)
	limit := pLimit(c, 30, 100)

	// live（twscrape 抓 X）优先；缓存未就绪时回退 seed
	if items, latest, ok := periphera.LiveNews(source, since, limit); ok {
		response.JSON(c, gin.H{"items": items, "server_ts": pServerTS(), "latest_ts": latest})
		return
	}

	all := periphera.SeedNews()
	items := make([]periphera.NewsItem, 0, len(all))
	var latest int64
	for _, n := range all {
		if source != "" && source != "all" && n.Source != source {
			continue
		}
		if since > 0 && n.PublishedAt <= since {
			continue
		}
		items = append(items, n)
		if n.PublishedAt > latest {
			latest = n.PublishedAt
		}
		if len(items) >= limit {
			break
		}
	}
	response.JSON(c, gin.H{"items": items, "server_ts": pServerTS(), "latest_ts": latest})
}

// PeripheraNewsSources GET /api/v1/news/sources
func PeripheraNewsSources(c *gin.Context) {
	if sources, ok := periphera.LiveNewsSources(); ok {
		response.JSON(c, gin.H{"sources": sources})
		return
	}
	response.JSON(c, gin.H{"sources": periphera.SeedNewsSources()})
}

// ---- 研报 ----

// PeripheraResearch GET /api/v1/research?rating=&institution=&q=&since=&limit=
// 数据来自 MySQL；库不可用时降级到 seed，保证前端不空白。
func PeripheraResearch(c *gin.Context) {
	rating := strings.TrimSpace(c.Query("rating"))
	inst := strings.TrimSpace(c.Query("institution"))
	q := strings.TrimSpace(c.Query("q"))
	since := pSince(c)
	limit := pLimit(c, 30, 100)

	all, err := periphera.ListResearch(c.Request.Context(), periphera.ResearchFilter{Rating: rating, Institution: inst, Q: q})
	if err != nil {
		logging.Errorf(c, "periphera ListResearch db error, fallback seed: %v", err)
		all = filterSeedResearch(rating, inst, q)
	}

	items := make([]periphera.ResearchReport, 0, len(all))
	var latest int64
	for _, r := range all {
		if since > 0 && r.CreatedAt <= since {
			continue
		}
		items = append(items, r)
		if r.CreatedAt > latest {
			latest = r.CreatedAt
		}
		if len(items) >= limit {
			break
		}
	}
	response.JSON(c, gin.H{
		"items":     items,
		"summary":   periphera.SummarizeResearch(all),
		"server_ts": pServerTS(),
		"latest_ts": latest,
	})
}

// filterSeedResearch 库不可用时的内存过滤回退
func filterSeedResearch(rating, inst, q string) []periphera.ResearchReport {
	out := []periphera.ResearchReport{}
	for _, r := range periphera.SeedResearch() {
		if rating != "" && rating != "all" && rating != "全部" && r.InvestmentRating != rating {
			continue
		}
		if inst != "" && inst != "全部机构" && r.InstitutionName != inst {
			continue
		}
		if q != "" && !strings.Contains(r.ResearchTarget+r.CoreContent+r.IndustryCategory, q) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// PeripheraResearchByID GET /api/v1/research/:id
func PeripheraResearchByID(c *gin.Context) {
	r, found, err := periphera.GetResearch(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.ErrJSON(c, response.CodeInternalError, err.Error())
		return
	}
	if !found {
		response.ErrJSON(c, response.CodeNotFound)
		return
	}
	response.JSON(c, r)
}

// PeripheraResearchCreate POST /api/v1/research  （body=研报对象，按 dedup_key upsert）
func PeripheraResearchCreate(c *gin.Context) {
	var r periphera.ResearchReport
	if err := c.ShouldBindJSON(&r); err != nil {
		response.ErrJSON(c, response.CodeInvalidParam, err.Error())
		return
	}
	action, saved, err := periphera.UpsertResearch(c.Request.Context(), r)
	if err != nil {
		response.ErrJSON(c, response.CodeInternalError, err.Error())
		return
	}
	response.JSON(c, gin.H{"action": action, "report": saved})
}

// PeripheraResearchUpdate PUT /api/v1/research/:id
func PeripheraResearchUpdate(c *gin.Context) {
	var r periphera.ResearchReport
	if err := c.ShouldBindJSON(&r); err != nil {
		response.ErrJSON(c, response.CodeInvalidParam, err.Error())
		return
	}
	r.ID = c.Param("id")
	action, saved, err := periphera.UpsertResearch(c.Request.Context(), r)
	if err != nil {
		response.ErrJSON(c, response.CodeInternalError, err.Error())
		return
	}
	response.JSON(c, gin.H{"action": action, "report": saved})
}

// PeripheraResearchDelete DELETE /api/v1/research/:id
func PeripheraResearchDelete(c *gin.Context) {
	ok, err := periphera.DeleteResearch(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.ErrJSON(c, response.CodeInternalError, err.Error())
		return
	}
	if !ok {
		response.ErrJSON(c, response.CodeNotFound)
		return
	}
	response.JSON(c, gin.H{"deleted": true})
}

// PeripheraResearchImport POST /api/v1/research/import  （body={items:[...]}）
func PeripheraResearchImport(c *gin.Context) {
	var body struct {
		Items []periphera.ResearchReport `json:"items"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.ErrJSON(c, response.CodeInvalidParam, err.Error())
		return
	}
	added, updated, err := periphera.ImportResearch(c.Request.Context(), body.Items)
	if err != nil {
		response.ErrJSON(c, response.CodeInternalError, err.Error())
		return
	}
	response.JSON(c, gin.H{"added": added, "updated": updated})
}

// ---- 行情 ----

// PeripheraIndices GET /api/v1/markets/indices?codes=
func PeripheraIndices(c *gin.Context) {
	codes := pCodes(c)
	if items, ok := periphera.LiveIndices(codes); ok {
		response.JSON(c, gin.H{"items": items, "server_ts": pServerTS()})
		return
	}
	response.JSON(c, gin.H{"items": periphera.SeedIndices(codes), "server_ts": pServerTS()})
}

// PeripheraUSSectors GET /api/v1/markets/us-sectors
func PeripheraUSSectors(c *gin.Context) {
	response.JSON(c, gin.H{"items": periphera.SeedSectors(), "trade_date": "2026-06-27", "updated_at": pServerTS()})
}

// PeripheraCommodities GET /api/v1/commodities?codes=
func PeripheraCommodities(c *gin.Context) {
	codes := pCodes(c)
	if items, ok := periphera.LiveCommodities(codes); ok {
		response.JSON(c, gin.H{"items": items, "server_ts": pServerTS()})
		return
	}
	response.JSON(c, gin.H{"items": periphera.SeedCommodities(codes), "server_ts": pServerTS()})
}

// PeripheraCrude GET /api/v1/commodities/crude
func PeripheraCrude(c *gin.Context) {
	if crude, ok := periphera.LiveCrude(); ok {
		response.JSON(c, crude)
		return
	}
	response.JSON(c, periphera.SeedCrude())
}

// PeripheraCrypto GET /api/v1/crypto?codes=
func PeripheraCrypto(c *gin.Context) {
	codes := pCodes(c)
	if items, ok := periphera.LiveCrypto(codes); ok {
		response.JSON(c, gin.H{"items": items, "server_ts": pServerTS()})
		return
	}
	response.JSON(c, gin.H{"items": periphera.SeedCrypto(codes), "server_ts": pServerTS()})
}

// PeripheraTicker GET /api/v1/ticker
func PeripheraTicker(c *gin.Context) {
	if items, ok := periphera.LiveTicker(); ok {
		response.JSON(c, gin.H{"items": items, "server_ts": pServerTS()})
		return
	}
	response.JSON(c, gin.H{"items": periphera.SeedTicker(), "server_ts": pServerTS()})
}

// ---- 宏观 ----

// PeripheraHormuz GET /api/v1/macro/hormuz
func PeripheraHormuz(c *gin.Context) {
	if h, ok := periphera.LiveHormuz(); ok {
		response.JSON(c, h)
		return
	}
	response.JSON(c, periphera.SeedHormuz())
}

// PeripheraFedWatch GET /api/v1/macro/fedwatch
func PeripheraFedWatch(c *gin.Context) {
	response.JSON(c, periphera.SeedFedWatch())
}

// PeripheraBriefing GET /api/v1/ai/briefing?date=
func PeripheraBriefing(c *gin.Context) {
	date := strings.TrimSpace(c.Query("date"))
	// live（火山 Ark Seed 模型生成）优先；未就绪时回退 seed
	if b, ok := periphera.LiveBriefing(date); ok {
		response.JSON(c, b)
		return
	}
	response.JSON(c, periphera.SeedBriefing(date))
}
