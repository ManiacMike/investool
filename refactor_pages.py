import os

page_content = """package api

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
"""

with open('d:/go-projects/investool/api/page.go', 'w', encoding='utf-8') as f:
    f.write(page_content)

sys_content = """package api

import (
	"github.com/axiaoxin-com/investool/routes/response"
	"github.com/gin-gonic/gin"
)

// Ping godoc
// @Summary ping server
// @Description ping server
// @Tags sys
// @Accept json
// @Produce json
// @Success 200 {object} response.Response
// @Router /x/ping [get]
// @Router /x/ping [post]
func Ping(c *gin.Context) {
	data := gin.H{
		"msg": "pong",
	}
	response.JSON(c, data)
	return
}
"""

with open('d:/go-projects/investool/api/sys.go', 'w', encoding='utf-8') as f:
    f.write(sys_content)

print("Page and Sys API files created!")
