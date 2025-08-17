package main

import (
	handler "essay-show/biz/adaptor/controller"
	"essay-show/biz/adaptor/controller/apigateway"

	"github.com/cloudwego/hertz/pkg/app/server"
)

// customizeRegister registers customize routers.
func customizedRegister(r *server.Hertz) {
	r.GET("/ping", handler.Ping)

	// 静态文件服务 - 直接提供文件访问
	r.StaticFile("/static/test_stream.html", "./static/test_stream.html")
	r.StaticFile("/static/test_exercise_stream.html", "./static/test_exercise_stream.html")

	// 版本化API路由 - 用于外部API客户端
	apiV1 := r.Group("/api/v1")
	{
		essay := apiV1.Group("/essay")
		{
			evaluate := essay.Group("/evaluate")
			evaluate.POST("/stream", apigateway.APIEssayEvaluateStreamV1)
		}

		sts := apiV1.Group("/sts")
		{
			sts.POST("/ocr", apigateway.APIOCRV1)
		}
	}
}
