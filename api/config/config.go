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
	Environment          string
	LogLevel             string
	HTTPHost             string
	HTTPPort             int
	MaxRequestBytes      int64
	DataPath             string
	CORSOrigins          []string
	EncryptionKey        string
	DefaultPool          PoolConfig
	MaxQueryRows         int
	MaxQueryBytes        int64
	MaxConcurrentQueries int
	TransactionTTL       time.Duration
	ShutdownTimeout      time.Duration
}

type PoolConfig struct {
	MaxOpenConnections int
	MaxIdleConnections int
	MaxLifetime        time.Duration
	MaxIdleTime        time.Duration
}

func Load() (Config, error) {
	httpPort, err := integer("DATADOCK_HTTP_PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	maxRequestBytes, err := integer64("DATADOCK_HTTP_MAX_BODY_BYTES", 10*1024*1024)
	if err != nil {
		return Config{}, err
	}
	maxOpenConnections, err := integer("DATADOCK_POOL_MAX_OPEN", 10)
	if err != nil {
		return Config{}, err
	}
	maxIdleConnections, err := integer("DATADOCK_POOL_MAX_IDLE", 5)
	if err != nil {
		return Config{}, err
	}
	maxLifetime, err := duration("DATADOCK_POOL_MAX_LIFETIME", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	maxIdleTime, err := duration("DATADOCK_POOL_MAX_IDLE_TIME", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	maxQueryRows, err := integer("DATADOCK_QUERY_MAX_ROWS", 1000)
	if err != nil {
		return Config{}, err
	}
	maxQueryBytes, err := integer64("DATADOCK_QUERY_MAX_BYTES", 10*1024*1024)
	if err != nil {
		return Config{}, err
	}
	maxConcurrentQueries, err := integer("DATADOCK_QUERY_MAX_CONCURRENCY", 8)
	if err != nil {
		return Config{}, err
	}
	transactionTTL, err := duration("DATADOCK_TRANSACTION_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := duration("DATADOCK_SHUTDOWN_TIMEOUT", 10*time.Second)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Environment:          value("DATADOCK_ENV", "development"),
		LogLevel:             strings.ToLower(value("DATADOCK_LOG_LEVEL", "info")),
		HTTPHost:             value("DATADOCK_HTTP_HOST", "0.0.0.0"),
		HTTPPort:             httpPort,
		MaxRequestBytes:      maxRequestBytes,
		DataPath:             value("DATADOCK_DATA_PATH", "./data"),
		CORSOrigins:          csv(value("DATADOCK_CORS_ORIGINS", "http://localhost:5173,http://localhost:3000")),
		EncryptionKey:        strings.TrimSpace(os.Getenv("DATADOCK_ENCRYPTION_KEY")),
		MaxQueryRows:         maxQueryRows,
		MaxQueryBytes:        maxQueryBytes,
		MaxConcurrentQueries: maxConcurrentQueries,
		TransactionTTL:       transactionTTL,
		ShutdownTimeout:      shutdownTimeout,
		DefaultPool: PoolConfig{
			MaxOpenConnections: maxOpenConnections,
			MaxIdleConnections: maxIdleConnections,
			MaxLifetime:        maxLifetime,
			MaxIdleTime:        maxIdleTime,
		},
	}

	if cfg.HTTPPort < 1 || cfg.HTTPPort > 65535 {
		return Config{}, fmt.Errorf("DATADOCK_HTTP_PORT must be between 1 and 65535")
	}
	if !validLogLevel(cfg.LogLevel) {
		return Config{}, fmt.Errorf("DATADOCK_LOG_LEVEL must be one of debug, info, warn, error, dpanic, panic, fatal")
	}
	if cfg.MaxRequestBytes < 1024 || cfg.MaxRequestBytes > 64*1024*1024 {
		return Config{}, fmt.Errorf("DATADOCK_HTTP_MAX_BODY_BYTES must be between 1024 and 67108864")
	}
	if cfg.DefaultPool.MaxOpenConnections < 1 {
		return Config{}, fmt.Errorf("DATADOCK_POOL_MAX_OPEN must be positive")
	}
	if cfg.DefaultPool.MaxIdleConnections < 0 || cfg.DefaultPool.MaxIdleConnections > cfg.DefaultPool.MaxOpenConnections {
		return Config{}, fmt.Errorf("DATADOCK_POOL_MAX_IDLE must be between 0 and DATADOCK_POOL_MAX_OPEN")
	}
	if cfg.DefaultPool.MaxLifetime <= 0 {
		return Config{}, fmt.Errorf("DATADOCK_POOL_MAX_LIFETIME must be positive")
	}
	if cfg.DefaultPool.MaxIdleTime <= 0 {
		return Config{}, fmt.Errorf("DATADOCK_POOL_MAX_IDLE_TIME must be positive")
	}
	if cfg.MaxQueryRows < 1 {
		return Config{}, fmt.Errorf("DATADOCK_QUERY_MAX_ROWS must be positive")
	}
	if cfg.MaxQueryBytes < 1 {
		return Config{}, fmt.Errorf("DATADOCK_QUERY_MAX_BYTES must be positive")
	}
	if cfg.MaxConcurrentQueries < 1 {
		return Config{}, fmt.Errorf("DATADOCK_QUERY_MAX_CONCURRENCY must be positive")
	}
	if cfg.TransactionTTL <= 0 {
		return Config{}, fmt.Errorf("DATADOCK_TRANSACTION_TTL must be positive")
	}
	if cfg.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("DATADOCK_SHUTDOWN_TIMEOUT must be positive")
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

func integer(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return parsed, nil
}

func integer64(key string, fallback int64) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return parsed, nil

}

func duration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration", key)
	}
	return parsed, nil
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

func validLogLevel(value string) bool {
	switch value {
	case "", "debug", "info", "warn", "error", "dpanic", "panic", "fatal":
		return true
	default:
		return false
	}
}
