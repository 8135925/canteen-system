package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	Port        string // Render 注入 PORT，本地缺省 8080
	DatabaseURL string
	JWTSecret   string
	AutoMigrate bool // 本地开发 true：启动时自动执行迁移；生产用迁移命令执行
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:        os.Getenv("PORT"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		// 本地开发兜底；生产由 Render generateValue 注入
		cfg.JWTSecret = "dev-secret-do-not-use-in-prod"
	}
	am := os.Getenv("AUTO_MIGRATE")
	cfg.AutoMigrate = am == "1" || am == "true"
	return cfg, nil
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return def
}
