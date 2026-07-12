package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment   string
	HTTPHost      string
	HTTPPort      int
	DataPath      string
	CORSOrigins   []string
	EncryptionKey string
	DefaultPool   PoolConfig
}

type PoolConfig struct {
	MaxOpenConnections int
	MaxIdleConnections int
	MaxLifetime        time.Duration
	MaxIdleTime        time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Environment:   value("DATADOCK_ENV", "development"),
		HTTPHost:      value("DATADOCK_HTTP_HOST", "0.0.0.0"),
		HTTPPort:      integer("DATADOCK_HTTP_PORT", 8080),
		DataPath:      value("DATADOCK_DATA_PATH", "./data"),
		CORSOrigins:   csv(value("DATADOCK_CORS_ORIGINS", "http://localhost:5173,http://localhost:3000")),
		EncryptionKey: strings.TrimSpace(os.Getenv("DATADOCK_ENCRYPTION_KEY")),
		DefaultPool: PoolConfig{
			MaxOpenConnections: integer("DATADOCK_POOL_MAX_OPEN", 10),
			MaxIdleConnections: integer("DATADOCK_POOL_MAX_IDLE", 5),
			MaxLifetime:        duration("DATADOCK_POOL_MAX_LIFETIME", 30*time.Minute),
			MaxIdleTime:        duration("DATADOCK_POOL_MAX_IDLE_TIME", 5*time.Minute),
		},
	}

	if cfg.HTTPPort < 1 || cfg.HTTPPort > 65535 {
		return Config{}, fmt.Errorf("DATADOCK_HTTP_PORT must be between 1 and 65535")
	}
	if cfg.DefaultPool.MaxOpenConnections < 1 {
		return Config{}, fmt.Errorf("DATADOCK_POOL_MAX_OPEN must be positive")
	}
	if cfg.DefaultPool.MaxIdleConnections < 0 || cfg.DefaultPool.MaxIdleConnections > cfg.DefaultPool.MaxOpenConnections {
		return Config{}, fmt.Errorf("DATADOCK_POOL_MAX_IDLE must be between 0 and DATADOCK_POOL_MAX_OPEN")
	}
	if cfg.DefaultPool.MaxLifetime <= 0 || cfg.DefaultPool.MaxIdleTime <= 0 {
		return Config{}, fmt.Errorf("database pool durations must be positive")
	}
	if len(cfg.CORSOrigins) == 0 {
		return Config{}, fmt.Errorf("DATADOCK_CORS_ORIGINS must contain at least one origin")
	}
	if cfg.EncryptionKey != "" {
		key, err := base64.StdEncoding.DecodeString(cfg.EncryptionKey)
		if err != nil || len(key) != 32 {
			return Config{}, fmt.Errorf("DATADOCK_ENCRYPTION_KEY must be a base64-encoded 32-byte key")
		}
	}
	if cfg.IsProduction() && cfg.EncryptionKey == "" {
		return Config{}, fmt.Errorf("DATADOCK_ENCRYPTION_KEY is required in production")
	}

	cfg.DataPath = filepath.Clean(cfg.DataPath)
	return cfg, nil
}

func (c Config) IsProduction() bool {
	return strings.EqualFold(c.Environment, "production")
}

func value(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func integer(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func duration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func csv(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			values = append(values, item)
		}
	}
	return values
}
