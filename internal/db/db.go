package db

import (
	"context"
	"deploy-lab/internal/config"
	"deploy-lab/internal/model"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// 这个文件是留给你自己写的。目标：把数据库接上，并且能区分
// “本地没配库” 和 “配了库但连不上” 两种情况。
//
// 练习步骤（每一步都先让它编译通过，再往下走）：
//
//  1. go get gorm.io/gorm@latest
//     go get github.com/glebarez/sqlite@latest      // 纯 Go 实现，不需要 CGO
//     go get gorm.io/driver/postgres@latest
//
//  2. 把下面的 TODO 换成真正的连接逻辑。判断方式建议：
//     cfg.SQLDSN 为空          -> 用 SQLite（本地跑得动就行）
//     cfg.SQLDSN 以 postgres 开头 -> 用 PostgreSQL
//     其他情况直接返回错误，别静默回退到 SQLite，线上会掩盖配置错误。
//
//  3. 连接成功后必须做的三件事：
//     - 设置连接池上限（MaxOpenConns / MaxIdleConns / ConnMaxLifetime）
//     - sqlDB.Ping() 一次，确认真的连上了
//     - AutoMigrate 建表
//
//  4. 写 Close()，在 main 的 defer 里调用，保证进程退出时连接被释放。
//
//  5. 加 Redis 同理：连不上不要 return error 退出，而是记一条 warn 日志
//     并把句柄置空，业务层自己判断“没有缓存”该怎么跑。
//
// 写完后启动项目，用 docker compose 里的 postgres 验证一次：
// 进容器执行 psql，能看到 AutoMigrate 建出来的表就算过关。

// DB 留给你填：连上之后把 *gorm.DB 放这里。
// 提示：也可以改成 New() (db *gorm.DB, err error) 由 main 持有，更干净。
var DB *gorm.DB // 改成 *gorm.DB 类型

func Init(cfg config.Config) error {
	var dialector gorm.Dialector

	if cfg.SQLDSN == "" {
		// TODO 1: 用 sqlite.Open("lab.db") 创建 dialector
		dialector = sqlite.Open("lab.db?_pragma=encoding=UTF-8")
		slog.Info("使用 SQLite", "path", "lab.db")
	} else if strings.HasPrefix(cfg.SQLDSN, "postgres://") {
		// TODO 2: 用 postgres.Open(cfg.SQLDSN) 创建 dialector
		dialector = postgres.Open(cfg.SQLDSN)
		slog.Info("使用 Postgres", "dsn", cfg.SQLDSN)
	} else {
		return fmt.Errorf("不支持的 SQL_DSN 格式: %s", cfg.SQLDSN)
	}

	// TODO 3: 调用 gorm.Open(dialector, &gorm.Config{}) 连接数据库，赋值给 DB
	var err error
	DB, err = gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// TODO 4: 拿到底层的 *sql.DB，设置连接池
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// TODO 5: Ping 一次，确认真的连上了
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}
	// AutoMigrate 建表（如果表不存在就创建，已存在则检查字段差异）
	if err := DB.AutoMigrate(&model.Note{}); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	// TODO 6: 以后这里加 AutoMigrate，现在先不加（等你定义 Note struct）
	// 例如: DB.AutoMigrate(&Note{})

	slog.Info("数据库已连接")
	return nil
}

func Close() {
	// TODO 7: 拿到 sqlDB 并调用 Close()
	if DB != nil {
		sqlDB, _ := DB.DB()
		sqlDB.Close()
	}
}
