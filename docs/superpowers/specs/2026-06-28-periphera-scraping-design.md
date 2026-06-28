# PERIPHERA Scraping Design

Date: 2026-06-28

## Scope

Add backend scraping data for the existing PERIPHERA REST APIs without changing the API contract in `PERIPHERA_API.md`.

Phase 1 connects stable public sources:

- Reuters and Bloomberg news through Google News RSS, merged into `/api/v1/news`.
- PAXG and XAUT prices through Binance REST with CoinGecko fallback, exposed by `/api/v1/crypto`.
- Strait of Hormuz daily transit data through Windward public HTML text parsing, exposed by `/api/v1/macro/hormuz`.

Phase 2 adds page-source groundwork:

- CLS telegraph and Jin10 flash public page fetchers/parsers where plain HTTP returns usable data.
- Manual verification links for MarineTraffic, VesselFinder, TradingView, and Sina dark-market items; no private API reverse engineering.

## Architecture

Keep the existing `datacenter/periphera` live/seed pattern:

- Background refresh goroutines populate in-memory stores.
- API handlers read live stores first.
- Failed refreshes keep the previous cache.
- Empty live stores fall back to existing seed data.

New source code stays in `datacenter/periphera` because these feeds are specific to the PERIPHERA dashboard contract.

## Data Flow

News:

1. Refresh RSS feeds every five minutes.
2. Parse standard RSS fields: title, link, pubDate, description, source.
3. Normalize into `NewsItem` with stable IDs derived from source and URL/title.
4. Merge with the existing news cache and preserve source filtering.

Crypto gold:

1. Refresh Binance `ticker/24hr` for `PAXGUSDT` and `XAUTUSDT`.
2. If Binance rejects a symbol or the request fails, refresh CoinGecko `simple/price`.
3. Normalize into `CryptoQuote` for `PAXG` and `XAUT`.
4. Existing BTC handling remains unchanged.

Hormuz:

1. Refresh Windward page text every 15 minutes.
2. Parse total transits, inbound/outbound counts, and a best-effort date.
3. Return `Hormuz` with today's total, source set to `windward`, and a short rolling series.
4. If parsing fails, keep old cache or fall back to seed.

Phase 2 page sources:

1. Fetch public CLS and Jin10 pages with browser-like headers.
2. Parse obvious links/text only when present in HTML.
3. Store helper functions and tests, but do not depend on unstable private XHR signatures.

## Error Handling

All scrapers use bounded HTTP timeouts. Parse errors and network errors do not propagate to API users. A failed refresh leaves the last good value in memory. The API layer only falls back to seed data when no live value exists.

## Testing

Add parser tests with fixed RSS, Binance, CoinGecko, Windward, CLS, and Jin10 fixtures. Run focused `go test` for `datacenter/periphera`, then broader package tests where practical.
