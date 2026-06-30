package periphera

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"
)

type cryptoGoldStore struct {
	mu        sync.RWMutex
	quotes    map[string]CryptoQuote
	hist      map[string][]float64
	updatedAt int64
	ok        bool
}

var goldCrypto = &cryptoGoldStore{
	quotes: map[string]CryptoQuote{},
	hist:   map[string][]float64{},
}
var goldCryptoOnce sync.Once

var cryptoGoldSpecs = map[string]struct {
	name          string
	binanceSymbol string
	coingeckoID   string
}{
	"PAXG": {"PAXG", "PAXGUSDT", "pax-gold"},
	"XAUT": {"XAUT", "XAUTUSDT", "tether-gold"},
}

func ensureCryptoGold() {
	goldCryptoOnce.Do(func() {
		go func() {
			refreshCryptoGold()
			t := time.NewTicker(30 * time.Second)
			defer t.Stop()
			for range t.C {
				refreshCryptoGold()
			}
		}()
	})
}

func refreshCryptoGold() {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	quotes := map[string]CryptoQuote{}
	for code, spec := range cryptoGoldSpecs {
		if q, err := fetchBinanceCryptoGold(ctx, code, spec.name, spec.binanceSymbol); err == nil {
			quotes[code] = q
		}
	}
	if len(quotes) < len(cryptoGoldSpecs) {
		for code, q := range fetchCoinGeckoCryptoGold(ctx) {
			if _, ok := quotes[code]; !ok {
				quotes[code] = q
			}
		}
	}
	if len(quotes) == 0 {
		return
	}

	now := nowMS()
	goldCrypto.mu.Lock()
	defer goldCrypto.mu.Unlock()
	for code, q := range quotes {
		q.UpdatedAt = now
		h := append(goldCrypto.hist[code], q.Price)
		if len(h) > 60 {
			h = h[len(h)-60:]
		}
		q.Spark = append([]float64{}, h...)
		goldCrypto.hist[code] = h
		goldCrypto.quotes[code] = q
	}
	goldCrypto.updatedAt = now
	goldCrypto.ok = true
}

type binanceTicker24h struct {
	LastPrice          string `json:"lastPrice"`
	PriceChangePercent string `json:"priceChangePercent"`
	CloseTime          int64  `json:"closeTime"`
}

func fetchBinanceCryptoGold(ctx context.Context, code, name, symbol string) (CryptoQuote, error) {
	body, err := fetchBytes(ctx, "https://data-api.binance.vision/api/v3/ticker/24hr?symbol="+symbol, nil)
	if err != nil {
		return CryptoQuote{}, err
	}
	return parseBinanceCryptoGold(body, code, name)
}

func parseBinanceCryptoGold(body []byte, code, name string) (CryptoQuote, error) {
	var t binanceTicker24h
	if err := json.Unmarshal(body, &t); err != nil {
		return CryptoQuote{}, err
	}
	price, err := strconv.ParseFloat(t.LastPrice, 64)
	if err != nil || price <= 0 {
		return CryptoQuote{}, err
	}
	pct, _ := strconv.ParseFloat(t.PriceChangePercent, 64)
	updatedAt := t.CloseTime
	if updatedAt <= 0 {
		updatedAt = nowMS()
	}
	return CryptoQuote{
		Code: code, Name: name, Price: round2(price),
		ChangePct: round2(pct), UpdatedAt: updatedAt,
	}, nil
}

type coinGeckoPrice map[string]struct {
	USD           float64 `json:"usd"`
	USD24HChange  float64 `json:"usd_24h_change"`
	LastUpdatedAt int64   `json:"last_updated_at"`
}

func fetchCoinGeckoCryptoGold(ctx context.Context) map[string]CryptoQuote {
	body, err := fetchBytes(ctx, "https://api.coingecko.com/api/v3/simple/price?ids=pax-gold,tether-gold&vs_currencies=usd&include_24hr_change=true&include_last_updated_at=true", nil)
	if err != nil {
		return nil
	}
	return parseCoinGeckoCryptoGold(body)
}

func parseCoinGeckoCryptoGold(body []byte) map[string]CryptoQuote {
	var cg coinGeckoPrice
	if json.Unmarshal(body, &cg) != nil {
		return nil
	}
	out := map[string]CryptoQuote{}
	for code, spec := range cryptoGoldSpecs {
		p := cg[spec.coingeckoID]
		if p.USD <= 0 {
			continue
		}
		updatedAt := p.LastUpdatedAt * 1000
		if updatedAt <= 0 {
			updatedAt = nowMS()
		}
		out[code] = CryptoQuote{
			Code: code, Name: spec.name, Price: round2(p.USD),
			ChangePct: round2(p.USD24HChange), UpdatedAt: updatedAt,
		}
	}
	return out
}

func liveCryptoGoldQuote(code string) (CryptoQuote, bool) {
	ensureCryptoGold()
	goldCrypto.mu.RLock()
	defer goldCrypto.mu.RUnlock()
	if !goldCrypto.ok {
		return CryptoQuote{}, false
	}
	q, ok := goldCrypto.quotes[code]
	if !ok {
		return CryptoQuote{}, false
	}
	q.Spark = append([]float64{}, q.Spark...)
	return q, true
}
