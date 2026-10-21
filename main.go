package main

import (
	"context"
	"embed"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"deploy-lab/internal/buildinfo"
	"deploy-lab/internal/config"
	"deploy-lab/internal/db"
	"deploy-lab/internal/router"
)

// 前端构建产物被编进二进制，部署时只需要这一个可执行文件。
//go:embed all:web/dist
var buildFS embed.FS

func main() {
	cfg := config.Load()
	setupLogger(cfg)

	slog.Info("starting deploy-lab", "version", buildinfo.Version, "port", cfg.Port)

	// 数据库还没实现，这里先不阻塞启动。写完 db.Init 后，把错误返回改成
	// os.Exit(1)：连不上库就不要再对外提供服务。
	_ = db.Init(cfg)
	defer db.Close()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router.New(buildFS),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// 收到 Ctrl+C 或 docker stop 时先让请求处理完再退出，
	// 否则正在写的请求会被直接掐断。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP 服务异常退出", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("收到退出信号，开始优雅关闭")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("优雅关闭超时，强制退出", "err", err)
		os.Exit(1)
	}
	slog.Info("已退出")
}

func setupLogger(cfg config.Config) {
	level := slog.LevelInfo
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	// JSON 格式日志便于容器平台收集和检索，人读不友好但上线后更好用。
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
}
