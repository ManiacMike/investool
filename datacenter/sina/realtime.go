// 新浪财经实时行情 hq.sinajs.cn 封装。
//
// 接口形如 https://hq.sinajs.cn/list=hf_GC,int_sp500 ，需带 Referer 头，
// 返回 GBK 文本，每行形如：var hq_str_hf_GC="字段0,字段1,...";
// 我们只用 ASCII 数字字段（中文名忽略），GBK 中文不含逗号，按逗号切分安全，
// 因此无需 GBK 解码。

package sina

import (
	"context"
	"strings"

	"github.com/axiaoxin-com/goutils"
)

// SinaHQReferer hq.sinajs.cn 必须的 Referer，否则 403
const SinaHQReferer = "https://finance.sina.com.cn"

// FetchRaw 批量拉取实时行情，返回 symbol -> 逗号切分后的字段数组。
// 空字符串（无数据/无效 symbol）的会被跳过。
func (s Sina) FetchRaw(ctx context.Context, symbols []string) (map[string][]string, error) {
	if len(symbols) == 0 {
		return map[string][]string{}, nil
	}
	apiurl := "https://hq.sinajs.cn/list=" + strings.Join(symbols, ",")
	resp, err := goutils.HTTPGETRaw(ctx, s.HTTPClient, apiurl, map[string]string{"Referer": SinaHQReferer})
	if err != nil {
		return nil, err
	}
	out := make(map[string][]string, len(symbols))
	for _, line := range strings.Split(string(resp), "\n") {
		line = strings.TrimSpace(line)
		const prefix = "var hq_str_"
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		sym := strings.TrimSpace(line[len(prefix):eq])
		val := strings.Trim(strings.TrimSpace(line[eq+1:]), ";")
		val = strings.Trim(val, `"`)
		if val == "" {
			continue
		}
		out[sym] = strings.Split(val, ",")
	}
	return out, nil
}
