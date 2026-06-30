package periphera

import (
	"context"
	"os"
	"testing"
)

// 手动联调（会产生一次付费模型调用，故需显式开关）：
//   PERIPHERA_MANUAL_TEST=1 go test ./datacenter/periphera/ -run TestGenerateBriefingManual -v
func TestGenerateBriefingManual(t *testing.T) {
	if os.Getenv("PERIPHERA_MANUAL_TEST") != "1" {
		t.Skip("set PERIPHERA_MANUAL_TEST=1 to run (makes a paid model call)")
	}
	LoadDotEnv()
	if os.Getenv("SEED_API_KEY") == "" {
		t.Skip("SEED_API_KEY not set")
	}
	b, err := generateBriefing(context.Background(), "2026-06-28")
	if err != nil {
		t.Fatalf("generateBriefing: %v", err)
	}
	t.Logf("headline: %s", b.Headline)
	t.Logf("body: %s", b.Body)
	t.Logf("points: %v", b.Points)
	t.Logf("tags: %v", b.Tags)
	if b.Headline == "" || b.Body == "" || len(b.Points) == 0 {
		t.Fatalf("briefing incomplete: %+v", b)
	}
}
