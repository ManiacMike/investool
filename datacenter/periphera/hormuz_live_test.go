package periphera

import (
	"testing"
	"time"
)

func TestParseWindwardHormuz(t *testing.T) {
	obs, ok := parseWindwardHormuz(`
		40 vessels transited the Strait of Hormuz on 27 June 2026.
		24 inbound / 16 outbound.
		~4.1M barrels of crude moved outbound.
	`, time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("parse failed")
	}
	if obs.total != 40 || obs.inbound != 24 || obs.outbound != 16 {
		t.Fatalf("unexpected obs: %#v", obs)
	}
	if got := obs.day.Format("2006-01-02"); got != "2026-06-27" {
		t.Fatalf("day=%s", got)
	}
}

func TestParseWindwardHormuzDirectionOnly(t *testing.T) {
	obs, ok := parseWindwardHormuz(`16 outbound, 24 inbound`, time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("parse failed")
	}
	if obs.total != 40 {
		t.Fatalf("total=%d", obs.total)
	}
}

func TestParseWindwardHormuzCurrentPageShape(t *testing.T) {
	obs, ok := parseWindwardHormuz(`On 23 May, 21 vessels transited the Strait of Hormuz, with 10 inbound and 11 outbound.`, time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("parse failed")
	}
	if obs.total != 21 {
		t.Fatalf("total=%d", obs.total)
	}
	if got := obs.day.Format("2006-01-02"); got != "2026-05-23" {
		t.Fatalf("day=%s", got)
	}
}

// 今天 Windward 的句式：方向数在 "transited inbound and X outbound" 里，无独立总数。
func TestParseWindwardHormuzInboundOutboundPhrase(t *testing.T) {
	obs, ok := parseWindwardHormuz(`On 28 June, 28 vessels transited inbound and 14 outbound through the Strait of Hormuz.`, time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC))
	if !ok {
		t.Fatal("parse failed")
	}
	if obs.inbound != 28 || obs.outbound != 14 || obs.total != 42 {
		t.Fatalf("unexpected obs: %#v", obs)
	}
	if got := obs.day.Format("2006-01-02"); got != "2026-06-28" {
		t.Fatalf("day=%s", got)
	}
}

// LiveHormuz 用真实累积的观测点构造序列：两天真实数据 → change_pct 基于真实相邻日。
func TestLiveHormuzRealSeries(t *testing.T) {
	s := &hormuzStore{history: map[string]int{}}
	s.record(hormuzObs{total: 40, day: time.Date(2026, 6, 27, 0, 0, 0, 0, time.UTC)})
	s.record(hormuzObs{total: 44, day: time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)})

	h, ok := s.build()
	if !ok {
		t.Fatal("not ok")
	}
	if h.Today != 44 || len(h.Series) != 2 {
		t.Fatalf("today=%d series=%d", h.Today, len(h.Series))
	}
	if h.Series[0].Date != "2026-06-27" || h.Series[1].Date != "2026-06-28" {
		t.Fatalf("dates=%v", h.Series)
	}
	if h.ChangePct != 10 { // (44-40)/40*100
		t.Fatalf("chg=%v", h.ChangePct)
	}
}
