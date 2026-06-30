package periphera

import "testing"

func TestParseNewsRSS(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss><channel><item>
<title>China economy story - Reuters</title>
<link>https://news.google.com/rss/articles/abc</link>
<pubDate>Sun, 28 Jun 2026 08:30:00 GMT</pubDate>
<description>&lt;a href="https://www.reuters.com/world/china/story-2026-06-28/"&gt;Full story&lt;/a&gt; Reuters summary</description>
<source url="https://www.reuters.com">Reuters</source>
</item></channel></rss>`)

	items, err := parseNewsRSS(body, newsRSSFeed{
		source:     "reuters",
		sourceName: "路透社",
		tags:       []string{"China"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("len=%d, want 1", len(items))
	}
	got := items[0]
	if got.Source != "reuters" || got.SourceName != "路透社" {
		t.Fatalf("unexpected source: %#v", got)
	}
	if got.Title != "China economy story - Reuters" {
		t.Fatalf("title=%q", got.Title)
	}
	if got.PublishedAt != 1782635400000 {
		t.Fatalf("published_at=%d", got.PublishedAt)
	}
	if got.Summary != "Full story Reuters summary" {
		t.Fatalf("summary=%q", got.Summary)
	}
	if got.ID == "" {
		t.Fatal("empty id")
	}
}
