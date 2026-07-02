package periphera

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestFedWatchLivePipelineEndToEnd 真跑一遍 live 接口：本地 httptest 起真实 HTTP 服务，
// 走完整链路 refreshFedWatch → fetchBytes(真实 TCP) → parseFedWatch → record → LiveFedWatch，
// 并校验代码发往 CME 的鉴权/应用标识 header。
func TestFedWatchLivePipelineEndToEnd(t *testing.T) {
	var gotAuth, gotAppName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAppName = r.Header.Get("CME-Application-Name")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"payload": [{
				"calculationTimestamp": "2026-07-01T08:30:00.000",
				"currentReportingRt": "350-375",
				"meetingDt": "2026-07-29",
				"rateRange": [
					{"lowerRt": 350, "upperRt": 375, "probability": 0.62},
					{"lowerRt": 325, "upperRt": 350, "probability": 0.38}
				]
			}]
		}`))
	}))
	defer srv.Close()

	t.Setenv("PERIPHERA_FEDWATCH_URL", srv.URL)
	t.Setenv("PERIPHERA_FEDWATCH_TOKEN", "test-token-123")
	t.Setenv("PERIPHERA_FEDWATCH_SOURCE", "cme")

	// 清掉可能被其它测试写入的缓存，保证断言的是本次真实抓取结果
	fedwatchLive.mu.Lock()
	fedwatchLive.current, fedwatchLive.ok = FedWatch{}, false
	fedwatchLive.mu.Unlock()

	refreshFedWatch()

	fw, ok := LiveFedWatch()
	if !ok {
		t.Fatal("LiveFedWatch not ready after real HTTP fetch")
	}
	if fw.MeetingDate != "2026-07-29" || fw.Meeting != "7月 FOMC" {
		t.Fatalf("unexpected meeting: %#v", fw)
	}
	assertOutcome(t, fw.Outcomes, 0, "维持不变", 62)
	assertOutcome(t, fw.Outcomes, 1, "降息25bp", 38)

	if gotAuth != "Bearer test-token-123" {
		t.Fatalf("Authorization header = %q, want Bearer test-token-123", gotAuth)
	}
	if gotAppName != "investool-periphera" {
		t.Fatalf("CME-Application-Name header = %q", gotAppName)
	}
}

// TestFedWatchLiveUnavailableFallsBack 未配置 URL 时 live 不就绪，API 层据此回退 seed。
func TestFedWatchLiveUnavailableFallsBack(t *testing.T) {
	t.Setenv("PERIPHERA_FEDWATCH_URL", "")
	t.Setenv("PERIPHERA_FEDWATCH_TOKEN", "")
	t.Setenv("PERIPHERA_FEDWATCH_SOURCE", "cme")
	if fedWatchURL() != "" {
		t.Fatalf("expected empty fedWatchURL when unconfigured, got %q", fedWatchURL())
	}
}
