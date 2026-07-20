// 景气行业追踪（半量化系统）
// 页面与 boom-advisor skill 共享同一份服务端 JSON 数据，
// 数据文件默认存放在 misc/data/boom_tracker.json，可通过配置 boom_tracker.data_file 覆盖。

package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/axiaoxin-com/investool/version"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

var boomDataMu sync.Mutex

// boomDataFile 返回数据文件路径
func boomDataFile() string {
	f := viper.GetString("boom_tracker.data_file")
	if f == "" {
		f = "misc/data/boom_tracker.json"
	}
	return f
}

// boomDefaultData 空数据骨架，页面和 skill 都依赖这些顶层字段存在
func boomDefaultData() gin.H {
	return gin.H{
		"settings": gin.H{
			"cash":         0,
			"cnyToHkdRate": 1.1179,
			"maxSectorPct": 40,
			"notes":        "",
		},
		"sectors":   []any{},
		"stocks":    []any{},
		"advices":   []any{},
		"summary":   nil,
		"updatedAt": "",
	}
}

// BoomTrackerHandler 景气行业追踪页面
func BoomTrackerHandler(c *gin.Context) {
	data := gin.H{
		"Env":       viper.GetString("env"),
		"HostURL":   viper.GetString("server.host_url"),
		"Version":   version.Version,
		"PageTitle": "景气行业追踪",
		"Error":     "",
	}
	c.HTML(http.StatusOK, "zen_boom_tracker.html", data)
}

// BoomDataGetHandler 读取完整数据文档
func BoomDataGetHandler(c *gin.Context) {
	boomDataMu.Lock()
	defer boomDataMu.Unlock()

	raw, err := os.ReadFile(boomDataFile())
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": boomDefaultData()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "读取数据失败: " + err.Error()})
		return
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "数据文件损坏: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": doc})
}

// BoomDataSaveHandler 全量覆盖保存数据文档（页面和 skill 均先读后写）
func BoomDataSaveHandler(c *gin.Context) {
	var doc map[string]any
	if err := c.ShouldBindJSON(&doc); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "参数错误: " + err.Error()})
		return
	}
	doc["updatedAt"] = time.Now().Format(time.RFC3339)

	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "序列化失败: " + err.Error()})
		return
	}

	boomDataMu.Lock()
	defer boomDataMu.Unlock()

	file := boomDataFile()
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "创建数据目录失败: " + err.Error()})
		return
	}
	// 先写临时文件再 rename，避免写一半损坏数据
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "写入数据失败: " + err.Error()})
		return
	}
	if err := os.Rename(tmp, file); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "error": "保存数据失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": doc})
}
