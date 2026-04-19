package api

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
}
