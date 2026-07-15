package engines

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

var _ ports.ConnectionLifecycle = (*Manager)(nil)
var _ ports.ConnectionTester = (*Manager)(nil)

type Manager struct {
	registry     *PoolRegistry
	executions   *ExecutionRegistry
	transactions *TransactionRegistry
	lifecycleMu  sync.RWMutex
	closeTimeout time.Duration
}

type ManagerOptions struct {
	MaxConcurrentQueries int
	TransactionIdleTTL   time.Duration
	ShutdownTimeout      time.Duration
}

func NewManager() *Manager {
	return NewManagerWithConcurrency(8)
}

func NewManagerWithConcurrency(maxConcurrentQueries int) *Manager {
	return NewManagerWithOptions(ManagerOptions{MaxConcurrentQueries: maxConcurrentQueries})
}

func NewManagerWithOptions(options ManagerOptions) *Manager {
	closeTimeout := options.ShutdownTimeout
	if closeTimeout <= 0 {
		closeTimeout = 10 * time.Second
	}
	return &Manager{
		registry:     NewPoolRegistry(),
		executions:   NewExecutionRegistry(options.MaxConcurrentQueries),
		transactions: NewTransactionRegistry(options.TransactionIdleTTL),
		closeTimeout: closeTimeout,
	}
}

func NewManagerWithRegistry(registry *PoolRegistry) *Manager {
	if registry == nil {
		registry = NewPoolRegistry()
	}
	return &Manager{registry: registry, executions: NewExecutionRegistry(8), transactions: NewTransactionRegistry(0), closeTimeout: 10 * time.Second}
}

func (manager *Manager) TestDraft(ctx context.Context, connection entity.Connection, password string) (entity.ConnectionRuntimeStatus, error) {
	return manager.registry.TestDraft(ctx, connection, password)
}

func (manager *Manager) Test(ctx context.Context, connection entity.Connection, password string) error {
	_, err := manager.TestDraft(ctx, connection, password)
	return err
}

func (manager *Manager) Connect(ctx context.Context, connection entity.Connection, password string) (entity.ConnectionRuntimeStatus, error) {
	status, err := manager.registry.Connect(ctx, connection, password)
	status.ActiveTransactions = manager.transactions.ActiveCount(connection.ID)
	return status, err
}

func (manager *Manager) Disconnect(ctx context.Context, connectionID string) error {
	manager.lifecycleMu.Lock()
	defer manager.lifecycleMu.Unlock()
	return errors.Join(manager.transactions.CloseConnection(ctx, connectionID), manager.registry.Disconnect(ctx, connectionID))
}

func (manager *Manager) Status(connectionID string) entity.ConnectionRuntimeStatus {
	status := manager.registry.Status(connectionID)
	status.ActiveTransactions = manager.transactions.ActiveCount(connectionID)
	return status
}

func (manager *Manager) Invalidate(ctx context.Context, connection entity.Connection, password string) error {
	manager.lifecycleMu.Lock()
	defer manager.lifecycleMu.Unlock()
	return errors.Join(manager.transactions.CloseConnection(ctx, connection.ID), manager.registry.Invalidate(ctx, connection, password))
}

func (manager *Manager) Remove(connectionID string) error {
	manager.lifecycleMu.Lock()
	defer manager.lifecycleMu.Unlock()
	return errors.Join(manager.transactions.CloseConnection(context.Background(), connectionID), manager.registry.Remove(connectionID))
}

func (manager *Manager) Database(ctx context.Context, connection entity.Connection, password string) (*sql.DB, error) {
	return manager.registry.Acquire(ctx, connection, password)
}

func (manager *Manager) Close() error {
	manager.lifecycleMu.Lock()
	defer manager.lifecycleMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), manager.closeTimeout)
	defer cancel()
	return errors.Join(manager.executions.Close(ctx), manager.transactions.Close(ctx), manager.registry.Close())
}

func openDatabase(connection entity.Connection, password string, transport *transport) (*sql.DB, error) {
	if !connection.Engine.Valid() {
		return nil, apperror.NewUnsupported("database engine is not supported", nil)
	}
	if connection.Engine == entity.EnginePostgreSQL {
		return openPostgresDatabase(connection, password, transport)
	}
	return openMySQLDatabase(connection, password, transport)
}
