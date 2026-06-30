package periphera

import "testing"

func TestParseCLSHTML(t *testing.T) {
	items := parseCLSHTML(`<a href="/telegraph/detail/123"> 财联社6月28日电，测试正文 </a>`)
	if len(items) != 1 {
		t.Fatalf("len=%d, want 1", len(items))
	}
	if items[0].Source != "cls" || items[0].URL != "https://www.cls.cn/telegraph/detail/123" {
		t.Fatalf("unexpected item: %#v", items[0])
	}
}

func TestParseJin10HTML(t *testing.T) {
	items := parseJin10HTML(`<a href="/detail/20260218092753501800">金十数据快讯正文</a>`)
	if len(items) != 1 {
		t.Fatalf("len=%d, want 1", len(items))
	}
	if items[0].ID != "jin10_20260218092753501800" {
		t.Fatalf("id=%s", items[0].ID)
	}
	if items[0].URL != "https://flash.jin10.com/detail/20260218092753501800" {
		t.Fatalf("url=%s", items[0].URL)
	}
}
