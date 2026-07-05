package periphera

// 金十快讯真实源：抓免登录静态源 flash_newest.js（内容形如 `var newest=[...]`），
// 解析成统一的 twItem 并入新闻缓存（见 news_live.go 的 refreshNews/mergeNews）。
// 仅取 type==0 的快讯，正文为空或非快讯跳过；时间为北京时区，转 Unix 毫秒。

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

const jin10FlashURL = "https://www.jin10.com/flash_newest.js"

// 去掉 `var newest =` 前缀，只留 JSON 数组
var jin10VarPrefixRE = regexp.MustCompile(`(?s)^\s*var\s+\w+\s*=\s*`)

// 金十时间为北京时区（无 tz 后缀），用固定 +08:00 避免依赖系统 tz 库
var jin10CST = time.FixedZone("CST", 8*3600)

type jin10Flash struct {
	ID        string `json:"id"`
	Time      string `json:"time"`
	Type      int    `json:"type"`
	Important int    `json:"important"`
	Data      struct {
		Title      string `json:"title"`
		Content    string `json:"content"`
		SourceLink string `json:"source_link"`
	} `json:"data"`
}

func fetchJin10News(ctx context.Context) []twItem {
	body, err := fetchBytes(ctx, jin10FlashURL, map[string]string{"Referer": "https://www.jin10.com/"})
	if err != nil {
		return nil
	}
	return parseJin10Flash(body, time.Now())
}

func parseJin10Flash(body []byte, now time.Time) []twItem {
	raw := jin10VarPrefixRE.ReplaceAll(body, nil)
	trimmed := strings.TrimRight(strings.TrimSpace(string(raw)), ";")
	var flashes []jin10Flash
	if json.Unmarshal([]byte(trimmed), &flashes) != nil {
		return nil
	}
	out := make([]twItem, 0, len(flashes))
	for _, fl := range flashes {
		if fl.Type != 0 || fl.ID == "" {
			continue
		}
		content := cleanText(stripHTML(fl.Data.Content))
		if content == "" {
			continue
		}
		title := cleanText(stripHTML(fl.Data.Title))
		if title == "" {
			title = jin10Title(content)
		}
		url := strings.TrimSpace(fl.Data.SourceLink)
		if url == "" {
			url = "https://flash.jin10.com/detail/" + fl.ID
		}
		out = append(out, twItem{
			ID:          "jin10_" + fl.ID,
			Source:      "jin10",
			SourceName:  "金十数据",
			Title:       title,
			Summary:     content,
			URL:         url,
			PublishedAt: jin10Time(fl.Time, now),
		})
	}
	return out
}

// jin10Title 无独立标题时，用正文首句/截断作标题。
func jin10Title(content string) string {
	if idx := strings.IndexAny(content, "。！？!?"); idx > 0 {
		content = content[:idx]
	}
	r := []rune(content)
	if len(r) > 40 {
		return string(r[:40]) + "…"
	}
	return content
}

func jin10Time(s string, now time.Time) int64 {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", strings.TrimSpace(s), jin10CST)
	if err != nil {
		return now.UnixMilli()
	}
	return t.UnixMilli()
}
