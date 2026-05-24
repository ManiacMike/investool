package webserver

import (
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/axiaoxin-com/goutils"
	"github.com/axiaoxin-com/investool/statics"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/json-iterator/go/extra"
	"github.com/spf13/viper"
)

func init() {
	// 替换 gin 默认的 validator，更加友好的错误信息
	binding.Validator = &goutils.GinStructValidator{}
	// causes the json binding Decoder to unmarshal a number into an interface{} as a Number instead of as a float64.
	binding.EnableDecoderUseNumber = true

	// jsoniter 启动模糊模式来支持 PHP 传递过来的 JSON。容忍字符串和数字互转
	extra.RegisterFuzzyDecoders()
	// jsoniter 设置支持 private 的 field
	extra.SupportPrivateFields()
}

// NewGinEngine 根据参数创建 gin 的 router engine
// middlewares 需要使用到的中间件列表，默认不为 engine 添加任何中间件
func NewGinEngine(middlewares ...gin.HandlerFunc) *gin.Engine {
	// set gin mode
	gin.SetMode(viper.GetString("server.mode"))

	engine := gin.New()
	// ///a///b -> /a/b
	engine.RemoveExtraSlash = true

	// use middlewares
	for _, middleware := range middlewares {
		engine.Use(middleware)
	}

	// load html template
	tmplPath := viper.GetString("statics.tmpl_path")
	staticsSource := viper.GetString("statics.source")
	staticsRoot := viper.GetString("statics.root")
	if tmplPath != "" {
		// add temp func for template parse
		// template func usage: {{ funcname xx }}
		tmpl := template.New("").Funcs(TemplFuncs)
		var t *template.Template
		if staticsSource == "disk" {
			t = template.Must(tmpl.ParseGlob(filepath.Join(staticsRoot, tmplPath)))
		} else {
			t = template.Must(tmpl.ParseFS(&statics.Files, tmplPath))
		}
		engine.SetHTMLTemplate(t)
	}
	// register statics
	staticsURL := viper.GetString("statics.url")
	if staticsURL != "" {
		if staticsSource == "disk" {
			engine.Static(staticsURL, staticsRoot)
		} else {
			engine.StaticFS(staticsURL, http.FS(&statics.Files))
		}
	}

	return engine
}
