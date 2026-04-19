package api

import (
	"net/http"

	"github.com/axiaoxin-com/investool/version"
	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// About godoc
func About(c *gin.Context) {
	data := gin.H{
		"Env":       viper.GetString("env"),
		"Version":   version.Version,
		"PageTitle": "InvesTool | 关于",
		"HostURL":   viper.GetString("server.host_url"),
	}
	c.HTML(http.StatusOK, "about.html", data)
}

// Comment godoc
func Comment(c *gin.Context) {
	data := gin.H{
		"Env":       viper.GetString("env"),
		"Version":   version.Version,
		"PageTitle": "InvesTool | 留言板",
		"HostURL":   viper.GetString("server.host_url"),
	}
	c.HTML(http.StatusOK, "comment.html", data)
}

// Materials godoc
func Materials(c *gin.Context) {
	data := gin.H{
		"Env":       viper.GetString("env"),
		"Version":   version.Version,
		"PageTitle": "InvesTool | 学习资料",
		"HostURL":   viper.GetString("server.host_url"),
	}
	c.HTML(http.StatusOK, "materials.html", data)
}
