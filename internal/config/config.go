package config

import (
	"os"
	"strconv"
)

// Config 全部来自环境变量，代码里不写死任何地址和密码。
type Config struct {
	Port      string
	SQLDSN    string
	RedisAddr string
	RedisPass string
	RedisDB   int
	LogLevel  string
}

func Load() Config {
	return Config{
		Port:      env("PORT", "8080"),
		SQLDSN:    os.Getenv("SQL_DSN"),
		RedisAddr: os.Getenv("REDIS_ADDR"),
		RedisPass: os.Getenv("REDIS_PASSWORD"),
		RedisDB:   envInt("REDIS_DB", 0),
		LogLevel:  env("LOG_LEVEL", "info"),
	}
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
