package periphera

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

// PageNewsItem is a best-effort item from public HTML pages such as CLS and Jin10.
// It is intentionally not wired into the live API until the plain HTML proves stable.
type PageNewsItem struct {
	ID     string
	Source string
	Title  string
	Body   string
	URL    string
}

func fetchCLSTelegraphPage(ctx context.Context) ([]PageNewsItem, error) {
	body, err := fetchBytes(ctx, "https://www.cls.cn/telegraph", map[string]string{
		"Referer": "https://www.cls.cn/",
	})
	if err != nil {
		return nil, err
	}
	return parseCLSHTML(string(body)), nil
}

func fetchJin10FlashPage(ctx context.Context) ([]PageNewsItem, error) {
	body, err := fetchBytes(ctx, "https://flash.jin10.com/", map[string]string{
		"Referer": "https://www.jin10.com/",
	})
	if err != nil {
		return nil, err
	}
	return parseJin10HTML(string(body)), nil
}

var (
	clsLinkRE   = regexp.MustCompile(`(?i)<a[^>]+href=["']([^"']*(?:telegraph|detail)[^"']*)["'][^>]*>(.*?)</a>`)
	jin10LinkRE = regexp.MustCompile(`(?i)<a[^>]+href=["']([^"']*/detail/(\d+)[^"']*)["'][^>]*>(.*?)</a>`)
)

func parseCLSHTML(raw string) []PageNewsItem {
	return parsePageLinks(raw, "cls", "https://www.cls.cn", clsLinkRE, 1, 2, 0)
}

func parseJin10HTML(raw string) []PageNewsItem {
	return parsePageLinks(raw, "jin10", "https://flash.jin10.com", jin10LinkRE, 1, 3, 2)
}

func parsePageLinks(raw, source, base string, re *regexp.Regexp, hrefIdx, titleIdx, idIdx int) []PageNewsItem {
	matches := re.FindAllStringSubmatch(raw, -1)
	seen := map[string]bool{}
	out := make([]PageNewsItem, 0, len(matches))
	for _, m := range matches {
		if len(m) <= hrefIdx || len(m) <= titleIdx {
			continue
		}
		link := absolutizeURL(base, strings.TrimSpace(m[hrefIdx]))
		title := cleanText(stripHTML(m[titleIdx]))
		if link == "" || title == "" || seen[link] {
			continue
		}
		id := link
		if idIdx > 0 && len(m) > idIdx && strings.TrimSpace(m[idIdx]) != "" {
			id = strings.TrimSpace(m[idIdx])
		}
		seen[link] = true
		out = append(out, PageNewsItem{
			ID:     source + "_" + id,
			Source: source,
			Title:  title,
			URL:    link,
		})
	}
	return out
}

func absolutizeURL(base, raw string) string {
	if raw == "" || strings.HasPrefix(raw, "javascript:") {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.IsAbs() {
		return u.String()
	}
	b, err := url.Parse(base)
	if err != nil {
		return ""
	}
	return b.ResolveReference(u).String()
}

func PublicVerificationLinks() map[string]string {
	return map[string]string{
		"marinetraffic_hormuz": "https://www.marinetraffic.com/en/ais/home/centerx:56.5/centery:26.4/zoom:7",
		"vesselfinder_hormuz":  "https://www.vesselfinder.com/ports/IRHOR001",
		"shipfinder_hormuz":    "https://www.shipfinder.com/special/hormuz",
		"tradingview_widget":   "https://s3.tradingview.com/external-embedding/embed-widget-symbol-overview.js",
		"sina_dark_gold_note":  "https://finance.sina.com.cn/money/nmetal/hjzx/2026-03-20/doc-inhrrrew0789900.shtml",
		"sina_dark_oil_note":   "https://finance.sina.com.cn/money/future/fmnews/2026-03-13/doc-inhqvnzp7907262.shtml",
	}
}
