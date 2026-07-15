package engines

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type managedPool struct {
	database    *sql.DB
	transport   *transport
	fingerprint string
	status      entity.ConnectionRuntimeStatus
}

type PoolRegistry struct {
	mu       sync.RWMutex
	pools    map[string]*managedPool
	statuses map[string]entity.ConnectionRuntimeStatus
	blocked  map[string]bool
	expected map[string]string
	removed  map[string]bool
	connects singleflight.Group
}

func NewPoolRegistry() *PoolRegistry {
	return &PoolRegistry{
		pools:    make(map[string]*managedPool),
		statuses: make(map[string]entity.ConnectionRuntimeStatus),
		blocked:  make(map[string]bool),
		expected: make(map[string]string),
		removed:  make(map[string]bool),
	}
}

func (registry *PoolRegistry) TestDraft(ctx context.Context, connection entity.Connection, password string) (entity.ConnectionRuntimeStatus, error) {
	startedAt := time.Now()
	database, transport, err := openPool(connection, password)
	if err != nil {
		return entity.ConnectionRuntimeStatus{State: entity.ConnectionStateError, LastErrorCode: errorCode(err)}, err
	}
	defer database.Close()
	defer transport.Close()
	if err := database.PingContext(ctx); err != nil {
		return entity.ConnectionRuntimeStatus{State: entity.ConnectionStateError, LastErrorCode: apperror.CodeConnectionFailed}, apperror.NewConnection("database connection failed", err)
	}
	return entity.ConnectionRuntimeStatus{State: entity.ConnectionStateConnected, LatencyMS: time.Since(startedAt).Milliseconds()}, nil
}

func (registry *PoolRegistry) Connect(ctx context.Context, connection entity.Connection, password string) (entity.ConnectionRuntimeStatus, error) {
	if connection.ID == "" {
		return entity.ConnectionRuntimeStatus{}, apperror.NewValidation("connection id is required", nil)
	}
	fingerprint, err := connectionFingerprint(connection, password)
	if err != nil {
		return entity.ConnectionRuntimeStatus{}, apperror.NewInternal("connection profile could not be prepared", err)
	}
	result, err, _ := registry.connects.Do(connection.ID, func() (any, error) {
		registry.mu.RLock()
		existing := registry.pools[connection.ID]
		expected := registry.expected[connection.ID]
		removed := registry.removed[connection.ID]
		registry.mu.RUnlock()
		if removed || expected != "" && expected != fingerprint {
			return entity.ConnectionRuntimeStatus{}, apperror.NewConflict("connection profile is stale", nil)
		}
		if existing != nil && existing.fingerprint == fingerprint {
			if pingErr := existing.database.PingContext(ctx); pingErr == nil {
				return existing.status, nil
			}
		}
		if existing != nil {
			registry.mu.Lock()
			if registry.pools[connection.ID] == existing {
				delete(registry.pools, connection.ID)
			}
			registry.mu.Unlock()
			_ = closeManagedPool(existing)
		}
		startedAt := time.Now()
		connecting := entity.ConnectionRuntimeStatus{State: entity.ConnectionStateConnecting}
		registry.setStatus(connection.ID, connecting)
		database, transport, openErr := openPool(connection, password)
		if openErr != nil {
			status := entity.ConnectionRuntimeStatus{State: entity.ConnectionStateError, LastErrorCode: errorCode(openErr)}
			registry.setStatus(connection.ID, status)
			return status, openErr
		}
		if pingErr := database.PingContext(ctx); pingErr != nil {
			database.Close()
			transport.Close()
			connectionErr := apperror.NewConnection("database connection failed", pingErr)
			status := entity.ConnectionRuntimeStatus{State: entity.ConnectionStateError, LastErrorCode: connectionErr.Code}
			registry.setStatus(connection.ID, status)
			return status, connectionErr
		}
		now := time.Now().UTC()
		status := entity.ConnectionRuntimeStatus{State: entity.ConnectionStateConnected, LatencyMS: time.Since(startedAt).Milliseconds(), LastConnectedAt: &now}
		pool := &managedPool{database: database, transport: transport, fingerprint: fingerprint, status: status}
		registry.mu.Lock()
		expected = registry.expected[connection.ID]
		removed = registry.removed[connection.ID]
		if removed || expected != "" && expected != fingerprint {
			registry.mu.Unlock()
			_ = closeManagedPool(pool)
			return entity.ConnectionRuntimeStatus{}, apperror.NewConflict("connection profile is stale", nil)
		}
		registry.pools[connection.ID] = pool
		registry.statuses[connection.ID] = status
		registry.expected[connection.ID] = fingerprint
		delete(registry.blocked, connection.ID)
		registry.mu.Unlock()
		return status, nil
	})
	if err != nil {
		return registry.Status(connection.ID), err
	}
	return result.(entity.ConnectionRuntimeStatus), nil
}

func (registry *PoolRegistry) Acquire(ctx context.Context, connection entity.Connection, password string) (*sql.DB, error) {
	fingerprint, err := connectionFingerprint(connection, password)
	if err != nil {
		return nil, apperror.NewInternal("connection profile could not be prepared", err)
	}
	registry.mu.RLock()
	pool := registry.pools[connection.ID]
	blocked := registry.blocked[connection.ID]
	expected := registry.expected[connection.ID]
	removed := registry.removed[connection.ID]
	registry.mu.RUnlock()
	if removed || expected != "" && expected != fingerprint {
		return nil, apperror.NewConflict("connection profile is stale", nil)
	}
	if pool != nil && pool.fingerprint == fingerprint {
		return pool.database, nil
	}
	if blocked || !connection.AutoReconnect {
		return nil, apperror.NewConnectionRequired("connection is disconnected", nil)
	}
	if _, err := registry.Connect(ctx, connection, password); err != nil {
		return nil, err
	}
	registry.mu.RLock()
	pool = registry.pools[connection.ID]
	registry.mu.RUnlock()
	if pool == nil {
		return nil, apperror.NewConnectionRequired("connection is disconnected", nil)
	}
	return pool.database, nil
}

func (registry *PoolRegistry) Disconnect(ctx context.Context, connectionID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	registry.mu.Lock()
	pool := registry.pools[connectionID]
	delete(registry.pools, connectionID)
	registry.blocked[connectionID] = true
	registry.statuses[connectionID] = entity.ConnectionRuntimeStatus{State: entity.ConnectionStateDisconnected}
	registry.mu.Unlock()
	return closeManagedPool(pool)
}

func (registry *PoolRegistry) Invalidate(ctx context.Context, connection entity.Connection, password string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	fingerprint, err := connectionFingerprint(connection, password)
	if err != nil {
		return apperror.NewInternal("connection profile could not be prepared", err)
	}
	registry.mu.Lock()
	pool := registry.pools[connection.ID]
	delete(registry.pools, connection.ID)
	delete(registry.blocked, connection.ID)
	delete(registry.removed, connection.ID)
	registry.expected[connection.ID] = fingerprint
	registry.statuses[connection.ID] = entity.ConnectionRuntimeStatus{State: entity.ConnectionStateDisconnected}
	registry.mu.Unlock()
	return closeManagedPool(pool)
}

func (registry *PoolRegistry) Remove(connectionID string) error {
	registry.mu.Lock()
	pool := registry.pools[connectionID]
	delete(registry.pools, connectionID)
	delete(registry.blocked, connectionID)
	delete(registry.expected, connectionID)
	registry.removed[connectionID] = true
	registry.statuses[connectionID] = entity.ConnectionRuntimeStatus{State: entity.ConnectionStateDisconnected}
	registry.mu.Unlock()
	return closeManagedPool(pool)
}

func (registry *PoolRegistry) Status(connectionID string) entity.ConnectionRuntimeStatus {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	status, ok := registry.statuses[connectionID]
	if !ok {
		return entity.ConnectionRuntimeStatus{State: entity.ConnectionStateDisconnected}
	}
	return status
}

func (registry *PoolRegistry) Close() error {
	registry.mu.Lock()
	pools := make([]*managedPool, 0, len(registry.pools))
	for _, pool := range registry.pools {
		pools = append(pools, pool)
	}
	registry.pools = make(map[string]*managedPool)
	registry.statuses = make(map[string]entity.ConnectionRuntimeStatus)
	registry.blocked = make(map[string]bool)
	registry.expected = make(map[string]string)
	registry.removed = make(map[string]bool)
	registry.mu.Unlock()
	var result error
	for _, pool := range pools {
		result = errors.Join(result, closeManagedPool(pool))
	}
	return result
}

func (registry *PoolRegistry) setStatus(connectionID string, status entity.ConnectionRuntimeStatus) {
	registry.mu.Lock()
	registry.statuses[connectionID] = status
	registry.mu.Unlock()
}

func openPool(connection entity.Connection, password string) (*sql.DB, *transport, error) {
	transport, err := newTransport(connection)
	if err != nil {
		return nil, nil, apperror.NewTransport("connection transport failed", err)
	}
	database, err := openDatabase(connection, password, transport)
	if err != nil {
		transport.Close()
		return nil, nil, apperror.NewConnection("database connection failed", err)
	}
	database.SetMaxOpenConns(max(1, connection.MaxOpenConns))
	database.SetMaxIdleConns(max(0, connection.MaxIdleConns))
	if connection.ConnMaxLifetime > 0 {
		database.SetConnMaxLifetime(time.Duration(connection.ConnMaxLifetime) * time.Second)
	}
	if connection.ConnMaxIdleTime > 0 {
		database.SetConnMaxIdleTime(time.Duration(connection.ConnMaxIdleTime) * time.Second)
	}
	return database, transport, nil
}

func closeManagedPool(pool *managedPool) error {
	if pool == nil {
		return nil
	}
	var result error
	if pool.database != nil {
		result = pool.database.Close()
	}
	if pool.transport != nil {
		pool.transport.Close()
	}
	return result
}

func connectionFingerprint(connection entity.Connection, password string) (string, error) {
	payload, err := json.Marshal(struct {
		Engine          entity.Engine
		Host            string
		Port            int
		Database        string
		Username        string
		Password        string
		SSLMode         entity.SSLMode
		SSLCAPath       string
		SSLCertPath     string
		SSLKeyPath      string
		ProxyURL        string
		ProxyUsername   string
		ProxyPassword   string
		SSHTunnel       entity.SSHTunnel
		SSHPassword     string
		ReadOnly        bool
		AutoReconnect   bool
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime int
		ConnMaxIdleTime int
	}{
		Engine:          connection.Engine,
		Host:            connection.Host,
		Port:            connection.Port,
		Database:        connection.Database,
		Username:        connection.Username,
		Password:        password,
		SSLMode:         connection.SSLMode,
		SSLCAPath:       connection.SSLCAPath,
		SSLCertPath:     connection.SSLCertPath,
		SSLKeyPath:      connection.SSLKeyPath,
		ProxyURL:        connection.ProxyURL,
		ProxyUsername:   connection.ProxyUsername,
		ProxyPassword:   connection.ProxyPassword,
		SSHTunnel:       connection.SSHTunnel,
		SSHPassword:     connection.SSHTunnel.Password,
		ReadOnly:        connection.ReadOnly,
		AutoReconnect:   connection.AutoReconnect,
		MaxOpenConns:    connection.MaxOpenConns,
		MaxIdleConns:    connection.MaxIdleConns,
		ConnMaxLifetime: connection.ConnMaxLifetime,
		ConnMaxIdleTime: connection.ConnMaxIdleTime,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func errorCode(err error) string {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return apperror.CodeConnectionFailed
}
