package periphera

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

var scrapeHTTPClient = &http.Client{Timeout: 45 * time.Second}

// fetchBytes 直连抓取（Binance/Windward 等国内可直连的源）。
func fetchBytes(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	return fetchBytesClient(ctx, url, headers, scrapeHTTPClient)
}

// fetchBytesClient 用指定 client 抓取（供需走代理的源，如 Google News RSS）。
func fetchBytesClient(ctx context.Context, url string, headers map[string]string, client *http.Client) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
