// 你的新页面

package routes

import (
	"net/http"

	"github.com/axiaoxin-com/investool/version"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// YourNewPageHandler 你的新页面处理函数
func YourNewPageHandler(c *gin.Context) {
	data := gin.H{
		"Env":       viper.GetString("env"),
		"HostURL":   viper.GetString("server.host_url"),
		"Version":   version.Version,
		"PageTitle": "InvesTool | 你的新页面",
		"Error":     "",
		// 在这里添加你需要的其他数据
	}
	c.HTML(http.StatusOK, "your_new_page.html", data)
	return
}
