package router

import (
	"embed"
	"io/fs"
	"net/http"
	"time"

	"deploy-lab/internal/api"

	"github.com/gin-gonic/gin"
)

// New 组装路由。顺序很重要：
//
//	日志 -> Recovery -> 限流/鉴权 -> 业务路由 -> 静态资源兜底
//
// 中间件顺序写反了，panic 可能绕过日志，或者静态文件把 /api 请求吃掉。
func New(buildFS embed.FS) http.Handler {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())

	apiGroup := r.Group("/api")
	{
		apiGroup.GET("/health", api.Health)
		apiGroup.GET("/notes", api.ListNotes)
		apiGroup.GET("/notes/:id", api.GetNote)
		apiGroup.PUT("/notes/:id", api.UpdateNote)
		apiGroup.DELETE("/notes/:id", api.DeleteNote)

		apiGroup.POST("/notes", api.CreateNote)
	}

	serveFrontend(r, buildFS)
	return r
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		// 只记必要信息，敏感字段（token、密码）不要进日志
		gin.DefaultWriter.Write([]byte(
			time.Now().Format(time.RFC3339) +
				" | " + c.Request.Method +
				" | " + c.FullPath() +
				" | " + http.StatusText(c.Writer.Status()) +
				" | " + time.Since(start).String() + "\n",
		))
	}
}

// serveFrontend 让同一个端口同时提供 API 和前端：
// 找不到的路径一律回 index.html，交给前端路由处理。
func serveFrontend(r *gin.Engine, buildFS embed.FS) {
	dist, err := fs.Sub(buildFS, "web/dist")
	if err != nil {
		return
	}
	fileServer := http.FileServer(http.FS(dist))

	r.NoRoute(func(c *gin.Context) {
		if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "not found"})
			return
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
