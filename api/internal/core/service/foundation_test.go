package service_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/huynhanx03/datadock/config"
	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

func TestConfigLoadUsesRuntimeLimits(t *testing.T) {
	setValidConfigEnvironment(t)
	t.Setenv("DATADOCK_POOL_MAX_IDLE_TIME", "7m")
	t.Setenv("DATADOCK_QUERY_MAX_ROWS", "2500")
	t.Setenv("DATADOCK_QUERY_MAX_BYTES", "33554432")
	t.Setenv("DATADOCK_QUERY_MAX_CONCURRENCY", "12")
	t.Setenv("DATADOCK_TRANSACTION_TTL", "20m")
	t.Setenv("DATADOCK_SHUTDOWN_TIMEOUT", "15s")
	t.Setenv("DATADOCK_LOG_LEVEL", "warn")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DefaultPool.MaxIdleTime != 7*time.Minute {
		t.Fatalf("MaxIdleTime = %v", cfg.DefaultPool.MaxIdleTime)
	}
	if cfg.MaxQueryRows != 2500 {
		t.Fatalf("MaxQueryRows = %d", cfg.MaxQueryRows)
	}
	if cfg.MaxQueryBytes != 33554432 {
		t.Fatalf("MaxQueryBytes = %d", cfg.MaxQueryBytes)
	}
	if cfg.MaxConcurrentQueries != 12 {
		t.Fatalf("MaxConcurrentQueries = %d", cfg.MaxConcurrentQueries)
	}
	if cfg.TransactionTTL != 20*time.Minute {
		t.Fatalf("TransactionTTL = %v", cfg.TransactionTTL)
	}
	if cfg.ShutdownTimeout != 15*time.Second {
		t.Fatalf("ShutdownTimeout = %v", cfg.ShutdownTimeout)
	}
	if cfg.LogLevel != "warn" {
		t.Fatalf("LogLevel = %q", cfg.LogLevel)
	}
}

func TestConfigLoadRejectsInvalidRuntimeLimits(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{key: "DATADOCK_POOL_MAX_IDLE_TIME", value: "invalid"},
		{key: "DATADOCK_QUERY_MAX_ROWS", value: "0"},
		{key: "DATADOCK_QUERY_MAX_BYTES", value: "-1"},
		{key: "DATADOCK_QUERY_MAX_CONCURRENCY", value: "invalid"},
		{key: "DATADOCK_TRANSACTION_TTL", value: "0s"},
		{key: "DATADOCK_SHUTDOWN_TIMEOUT", value: "-1s"},
		{key: "DATADOCK_LOG_LEVEL", value: "verbose"},
	}

	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			setValidConfigEnvironment(t)
			t.Setenv(test.key, test.value)

			_, err := config.Load()
			if err == nil || !strings.Contains(err.Error(), test.key) {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}
}

func TestApplicationErrorConstructors(t *testing.T) {
	cause := errors.New("cause")
	tests := []struct {
		name      string
		construct func(string, error) *apperror.Error
		code      string
		temporary bool
	}{
		{name: "validation", construct: apperror.NewValidation, code: apperror.CodeValidation},
		{name: "not found", construct: apperror.NewNotFound, code: apperror.CodeNotFound},
		{name: "conflict", construct: apperror.NewConflict, code: apperror.CodeConflict},
		{name: "connection", construct: apperror.NewConnection, code: apperror.CodeConnectionFailed, temporary: true},
		{name: "timeout", construct: apperror.NewTimeout, code: apperror.CodeQueryTimeout, temporary: true},
		{name: "cancellation", construct: apperror.NewCancellation, code: apperror.CodeQueryCancelled},
		{name: "readonly", construct: apperror.NewReadonly, code: apperror.CodeReadonlyViolation},
		{name: "permission", construct: apperror.NewPermission, code: apperror.CodePermissionDenied},
		{name: "unsupported", construct: apperror.NewUnsupported, code: apperror.CodeUnsupportedCapability},
		{name: "transaction expiry", construct: apperror.NewTransactionExpired, code: apperror.CodeTransactionExpired},
		{name: "transport", construct: apperror.NewTransport, code: apperror.CodeTransportFailed, temporary: true},
		{name: "internal", construct: apperror.NewInternal, code: apperror.CodeInternal},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.construct("safe message", cause)
			if err.Code != test.code || err.Message != "safe message" || err.Temporary != test.temporary {
				t.Fatalf("error = %#v", err)
			}
			if err.Error() != "safe message" {
				t.Fatalf("Error() = %q", err.Error())
			}
			if !errors.Is(err, cause) {
				t.Fatal("error does not unwrap cause")
			}
		})
	}
}

func TestConnectionViewIncludesSecretSafeRuntimeStatus(t *testing.T) {
	lastConnectedAt := time.Date(2026, time.July, 15, 10, 30, 0, 0, time.UTC)
	view := dto.ConnectionView{
		ID:              "connection-1",
		ConnMaxIdleTime: 60,
		ConnectionRuntimeStatus: entity.ConnectionRuntimeStatus{
			State:              entity.ConnectionStateConnected,
			LatencyMS:          18,
			LastConnectedAt:    &lastConnectedAt,
			LastErrorCode:      "",
			ActiveTransactions: 2,
		},
	}

	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	payload := string(encoded)
	for _, expected := range []string{`"status":"connected"`, `"latencyMs":18`, `"lastConnectedAt":"2026-07-15T10:30:00Z"`, `"activeTransactions":2`, `"connMaxIdleTimeSeconds":60`} {
		if !strings.Contains(payload, expected) {
			t.Fatalf("payload %s does not contain %s", payload, expected)
		}
	}
	for _, secret := range []string{`"password":`, `"passwordCipher":`, `"proxyCredentials":`, `"privateKey":`} {
		if strings.Contains(strings.ToLower(payload), strings.ToLower(secret)) {
			t.Fatalf("payload exposes %s: %s", secret, payload)
		}
	}
}

func setValidConfigEnvironment(t *testing.T) {
	t.Helper()
	values := map[string]string{
		"DATADOCK_ENV":                   "development",
		"DATADOCK_HTTP_PORT":             "8080",
		"DATADOCK_CORS_ORIGINS":          "http://localhost:5173",
		"DATADOCK_ENCRYPTION_KEY":        "",
		"DATADOCK_POOL_MAX_OPEN":         "10",
		"DATADOCK_POOL_MAX_IDLE":         "5",
		"DATADOCK_POOL_MAX_LIFETIME":     "30m",
		"DATADOCK_POOL_MAX_IDLE_TIME":    "5m",
		"DATADOCK_QUERY_MAX_ROWS":        "1000",
		"DATADOCK_QUERY_MAX_BYTES":       "10485760",
		"DATADOCK_QUERY_MAX_CONCURRENCY": "8",
		"DATADOCK_TRANSACTION_TTL":       "15m",
		"DATADOCK_SHUTDOWN_TIMEOUT":      "10s",
		"DATADOCK_LOG_LEVEL":             "info",
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
}
