package periphera

import "testing"

func TestParseBinanceCryptoGold(t *testing.T) {
	q, err := parseBinanceCryptoGold([]byte(`{
		"lastPrice": "4068.12000000",
		"priceChangePercent": "0.183",
		"closeTime": 1782650000000
	}`), "PAXG", "PAXG")
	if err != nil {
		t.Fatal(err)
	}
	if q.Code != "PAXG" || q.Price != 4068.12 || q.ChangePct != 0.18 || q.UpdatedAt != 1782650000000 {
		t.Fatalf("unexpected quote: %#v", q)
	}
}

func TestParseCoinGeckoCryptoGold(t *testing.T) {
	quotes := parseCoinGeckoCryptoGold([]byte(`{
		"pax-gold": {"usd": 4068.12, "usd_24h_change": 0.18, "last_updated_at": 1782650000},
		"tether-gold": {"usd": 4064.55, "usd_24h_change": -0.10, "last_updated_at": 1782650001}
	}`))
	if len(quotes) != 2 {
		t.Fatalf("len=%d, want 2", len(quotes))
	}
	if quotes["PAXG"].Price != 4068.12 || quotes["XAUT"].Price != 4064.55 {
		t.Fatalf("unexpected quotes: %#v", quotes)
	}
	if quotes["XAUT"].UpdatedAt != 1782650001000 {
		t.Fatalf("updated_at=%d", quotes["XAUT"].UpdatedAt)
	}
}
