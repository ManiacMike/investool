package periphera

import (
	"testing"
	"time"
)

func TestParseJin10Flash(t *testing.T) {
	// 结构取自真实 flash_newest.js（2026-07-02 抓）：含标题为空、正文中英、
	// 非快讯类型(type!=0)、source_link 缺失等真实情况。
	body := []byte(`var newest = [
		{"id":"20260702225617884800","time":"2026-07-02 22:56:17","type":0,"important":1,"data":{"title":"","content":"闪迪(SNDK.O)盘中跌幅扩大至10%。后续还有很多字要被截断需要处理成标题的情况测试一下超长文本","source_link":""}},
		{"id":"20260702225329451800","time":"2026-07-02 22:53:29","type":0,"important":0,"data":{"title":"金十整理：要闻汇总","content":"<p>Shipping giant signaled openness.</p>","source_link":"https://example.com/x"}},
		{"id":"skip1","time":"2026-07-02 22:00:00","type":1,"important":0,"data":{"content":"这是行情数据不是快讯"}},
		{"id":"skip2","time":"2026-07-02 22:00:00","type":0,"important":0,"data":{"content":""}}
	];`)

	items := parseJin10Flash(body, time.Date(2026, 7, 2, 23, 0, 0, 0, time.UTC))
	if len(items) != 2 {
		t.Fatalf("want 2 flashes (type!=0 与空正文应过滤), got %d: %#v", len(items), items)
	}

	// 第一条：标题为空 → 用首句(。之前)，source_link 空 → 回退 detail URL
	a := items[0]
	if a.ID != "jin10_20260702225617884800" || a.Source != "jin10" || a.SourceName != "金十数据" {
		t.Fatalf("bad meta: %#v", a)
	}
	if a.Title != "闪迪(SNDK.O)盘中跌幅扩大至10%" {
		t.Fatalf("title should be first sentence, got %q", a.Title)
	}
	if a.URL != "https://flash.jin10.com/detail/20260702225617884800" {
		t.Fatalf("url fallback wrong: %q", a.URL)
	}
	// 北京时间 22:56:17 → UTC 14:56:17
	wantTS := time.Date(2026, 7, 2, 14, 56, 17, 0, time.UTC).UnixMilli()
	if a.PublishedAt != wantTS {
		t.Fatalf("published_at = %d, want %d (Beijing→UTC)", a.PublishedAt, wantTS)
	}

	// 第二条：有标题，正文 HTML 被清洗，source_link 生效
	b := items[1]
	if b.Title != "金十整理：要闻汇总" {
		t.Fatalf("title = %q", b.Title)
	}
	if b.Summary != "Shipping giant signaled openness." {
		t.Fatalf("summary should strip HTML, got %q", b.Summary)
	}
	if b.URL != "https://example.com/x" {
		t.Fatalf("url should use source_link, got %q", b.URL)
	}
}
