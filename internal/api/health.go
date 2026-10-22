package api

import (
	"context"
	"net/http"
	"os"
	"time"

	"deploy-lab/internal/buildinfo"
	"deploy-lab/internal/db"

	"github.com/gin-gonic/gin"
)

var startedAt = time.Now()

// Health 是最先该写的接口：部署、探活、排查都靠它。
// 练习目标：
//   - 数据库没连上时返回 503 而不是 200（编排器据此判断节点是否可用）
//   - 不要在健康检查里做重活，最慢几秒内必须返回
//   - 响应里带上 hostname，多副本部署时能看出是哪台机器应答的
func Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	sqlDB, err := db.DB.DB()
	if err != nil || sqlDB.PingContext(ctx) != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":        "ok",
		"version":       buildinfo.Version,
		"hostname":      hostname(),
		"uptime_second": int64(time.Since(startedAt).Seconds()),
	})
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
