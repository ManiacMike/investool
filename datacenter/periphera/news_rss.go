package periphera

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"html"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Google News（news.google.com）国内需代理；复用 X 抓取同一 X_PROXY。
// 未配 X_PROXY 时退化为直连（海外环境可直接用）。
var (
	rssClientOnce sync.Once
	rssClient     *http.Client
)

func newsRSSClient() *http.Client {
	rssClientOnce.Do(func() {
		c := &http.Client{Timeout: 20 * time.Second}
		if p := os.Getenv("X_PROXY"); p != "" {
			if u, err := url.Parse(p); err == nil {
				c.Transport = &http.Transport{Proxy: http.ProxyURL(u)}
			}
		}
		rssClient = c
	})
	return rssClient
}

type newsRSSFeed struct {
	source     string
	sourceName string
	url        string
	tags       []string
}

var newsRSSFeeds = []newsRSSFeed{
	{
		source:     "reuters",
		sourceName: "路透社",
		url:        "https://news.google.com/rss/search?q=site%3Areuters.com%2Fworld%2Fchina%20China&hl=en-US&gl=US&ceid=US%3Aen",
		tags:       []string{"China", "Reuters"},
	},
	{
		source:     "bloomberg",
		sourceName: "彭博社",
		url:        "https://news.google.com/rss/search?q=site%3Abloomberg.com%20China%20economy&hl=en-US&gl=US&ceid=US%3Aen",
		tags:       []string{"China", "Bloomberg"},
	},
}

type rssDoc struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
	Source      string `xml:"source"`
}

func fetchNewsRSS(ctx context.Context) []twItem {
	var out []twItem
	for _, feed := range newsRSSFeeds {
		if items, err := fetchNewsRSSFeed(ctx, feed); err == nil {
			out = append(out, items...)
		}
	}
	return out
}

func fetchNewsRSSFeed(ctx context.Context, feed newsRSSFeed) ([]twItem, error) {
	body, err := fetchBytesClient(ctx, feed.url, nil, newsRSSClient())
	if err != nil {
		return nil, err
	}
	return parseNewsRSS(body, feed)
}

func parseNewsRSS(body []byte, feed newsRSSFeed) ([]twItem, error) {
	var doc rssDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	out := make([]twItem, 0, len(doc.Channel.Items))
	for _, item := range doc.Channel.Items {
		title := cleanText(item.Title)
		link := strings.TrimSpace(item.Link)
		if title == "" || link == "" {
			continue
		}
		out = append(out, twItem{
			ID:          rssNewsID(feed.source, link, title),
			Source:      feed.source,
			SourceName:  feed.sourceName,
			Author:      cleanText(item.Source),
			Title:       title,
			Summary:     cleanText(stripHTML(item.Description)),
			URL:         link,
			PublishedAt: parseRSSPubDate(item.PubDate),
			Tags:        append([]string{}, feed.tags...),
		})
	}
	return out, nil
}

func rssNewsID(source, link, title string) string {
	h := sha1.Sum([]byte(source + "|" + link + "|" + title))
	return source + "_rss_" + hex.EncodeToString(h[:])[:12]
}

func parseRSSPubDate(s string) int64 {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UnixMilli()
		}
	}
	return nowMS()
}

var htmlTagRE = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	return htmlTagRE.ReplaceAllString(s, " ")
}

func cleanText(s string) string {
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
