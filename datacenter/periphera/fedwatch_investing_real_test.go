package periphera

import (
	"os"
	"testing"
	"time"
)

// TestParseInvestingFedWatchRealMarkup 用真实 Investing Fed Rate Monitor 页面抓取到的
// 第一张会议卡（testdata/investing_fedwatch_card.html，2026-07-02 通过本地代理真抓）做回归，
// 防止 Investing 改版后正则失配而静默回退 seed。
func TestParseInvestingFedWatchRealMarkup(t *testing.T) {
	body, err := os.ReadFile("testdata/investing_fedwatch_card.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	fw, err := parseInvestingFedWatch(body, time.Date(2026, 7, 2, 22, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("parseInvestingFedWatch on real markup: %v", err)
	}
	if fw.Meeting != "7月 FOMC" || fw.MeetingDate != "2026-07-29" {
		t.Fatalf("unexpected meeting: %#v", fw)
	}
	// 真实卡片：3.50-3.75 @ 80.1%（降息25bp），3.75-4.00 @ 19.9%（维持不变）
	assertOutcome(t, fw.Outcomes, 0, "降息25bp", 80.1)
	assertOutcome(t, fw.Outcomes, 1, "维持不变", 19.9)
	if fw.UpdatedAt == 0 {
		t.Fatal("expected updated_at parsed from fedUpdate")
	}
}
