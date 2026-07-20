// 股票池五维评分报告
// 评分由 .claude/skills/boom-advisor/scoring/run.sh 生成，
// JSON 报告默认存放在 misc/data/boom_scores/，可通过配置 boom_tracker.scores_dir 覆盖。

package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/axiaoxin-com/investool/version"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// boomScoresDir 返回评分报告目录
func boomScoresDir() string {
	d := viper.GetString("boom_tracker.scores_dir")
	if d == "" {
		d = "misc/data/boom_scores"
	}
	return d
}

var boomScoresDatePattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

// BoomScoresPageHandler 评分报告页面
func BoomScoresPageHandler(c *gin.Context) {
	data := gin.H{
		"Env":       viper.GetString("env"),
		"HostURL":   viper.GetString("server.host_url"),
		"Version":   version.Version,
		"PageTitle": "股票池评分报告",
		"Error":     "",
	}
	c.HTML(http.StatusOK, "zen_boom_scores.html", data)
}

// BoomScoresListHandler 报告列表，按数据日期倒序返回每份报告的元信息
func BoomScoresListHandler(c *gin.Context) {
	entries, err := os.ReadDir(boomScoresDir())
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": []any{}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "读取报告目录失败: " + err.Error()})
		return
	}
	list := []gin.H{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "boom_scores_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(boomScoresDir(), name))
		if err != nil {
			continue
		}
		var doc struct {
			Date        string `json:"date"`
			GeneratedAt string `json:"generatedAt"`
			Total       int    `json:"total"`
			Held        int    `json:"held"`
			Buys        int    `json:"buys"`
			Sells       int    `json:"sells"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil || doc.Date == "" {
			continue
		}
		list = append(list, gin.H{
			"date":        doc.Date,
			"generatedAt": doc.GeneratedAt,
			"total":       doc.Total,
			"held":        doc.Held,
			"buyCount":    doc.Buys,
			"sellCount":   doc.Sells,
		})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i]["date"].(string) > list[j]["date"].(string)
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": list})
}

// BoomScoresGetHandler 按数据日期返回完整报告
func BoomScoresGetHandler(c *gin.Context) {
	date := c.Param("date")
	if !boomScoresDatePattern.MatchString(date) {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "日期格式错误，应为 YYYY-MM-DD"})
		return
	}
	raw, err := os.ReadFile(filepath.Join(boomScoresDir(), "boom_scores_"+date+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"success": false, "error": "报告不存在: " + date})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "读取报告失败: " + err.Error()})
		return
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "报告文件损坏: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": doc})
}
