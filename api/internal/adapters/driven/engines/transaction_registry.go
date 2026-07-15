package engines

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

const (
	defaultTransactionIdleTTL  = 15 * time.Minute
	maximumTransactionIdleTTL  = 24 * time.Hour
	transactionTerminalTTL     = 5 * time.Minute
	transactionIdentifierBytes = 24
)

var _ ports.TransactionRuntime = (*Manager)(nil)

type managedTransaction struct {
	mu             sync.Mutex
	id             string
	connectionID   string
	engine         entity.Engine
	connection     *sql.Conn
	transaction    *sql.Tx
	state          dto.TransactionLifecycleState
	startedAt      time.Time
	lastActivityAt time.Time
	expiresAt      time.Time
	completedAt    *time.Time
	savepoints     []dto.TransactionSavepoint
}

type TransactionRegistry struct {
	mu                 sync.RWMutex
	handles            map[string]*managedTransaction
	activeByConnection map[string]int
	idleTTL            time.Duration
	terminalRetention  time.Duration
	closed             bool
	stop               chan struct{}
	done               chan struct{}
	closeOnce          sync.Once
	closeErr           error
}

func NewTransactionRegistry(idleTTL time.Duration) *TransactionRegistry {
	idleTTL = normalizeTransactionIdleTTL(idleTTL)
	registry := &TransactionRegistry{
		handles:            make(map[string]*managedTransaction),
		activeByConnection: make(map[string]int),
		idleTTL:            idleTTL,
		terminalRetention:  transactionTerminalTTL,
		stop:               make(chan struct{}),
		done:               make(chan struct{}),
	}
	go registry.reap()
	return registry
}

func (manager *Manager) Begin(ctx context.Context, connection entity.Connection, password string) (dto.TransactionView, error) {
	manager.lifecycleMu.RLock()
	defer manager.lifecycleMu.RUnlock()
	if err := ctx.Err(); err != nil {
		return dto.TransactionView{}, err
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return dto.TransactionView{}, err
	}
	return manager.transactions.Begin(ctx, database, connection)
}

func (manager *Manager) Get(ctx context.Context, transactionID string) (dto.TransactionView, error) {
	return manager.transactions.Get(ctx, transactionID)
}

func (manager *Manager) Commit(ctx context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	return manager.transactions.Commit(ctx, action)
}

func (manager *Manager) Rollback(ctx context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	return manager.transactions.Rollback(ctx, action)
}

func (manager *Manager) Savepoint(ctx context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	return manager.transactions.Savepoint(ctx, action)
}

func (manager *Manager) RollbackTo(ctx context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	return manager.transactions.RollbackTo(ctx, action)
}

func (manager *Manager) Release(ctx context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	return manager.transactions.Release(ctx, action)
}

func (manager *Manager) withTransactionExecutor(ctx context.Context, connectionID, transactionID string, callback func(queryExecutor) error) error {
	return manager.transactions.WithExecutor(ctx, connectionID, transactionID, callback)
}

func (registry *TransactionRegistry) Begin(ctx context.Context, database *sql.DB, connection entity.Connection) (dto.TransactionView, error) {
	if database == nil || connection.ID == "" {
		return dto.TransactionView{}, apperror.NewValidation("invalid transaction request", nil)
	}
	if err := registry.available(); err != nil {
		return dto.TransactionView{}, err
	}
	id, err := newTransactionIdentifier()
	if err != nil {
		return dto.TransactionView{}, apperror.NewInternal("transaction identifier could not be generated", err)
	}
	reserved, err := database.Conn(ctx)
	if err != nil {
		return dto.TransactionView{}, err
	}
	if err := ctx.Err(); err != nil {
		_ = reserved.Close()
		return dto.TransactionView{}, err
	}
	transaction, err := reserved.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: connection.ReadOnly})
	if err != nil {
		_ = reserved.Close()
		return dto.TransactionView{}, err
	}
	if err := ctx.Err(); err != nil {
		_ = transaction.Rollback()
		_ = reserved.Close()
		return dto.TransactionView{}, err
	}
	now := time.Now().UTC()
	handle := &managedTransaction{
		id:             id,
		connectionID:   connection.ID,
		engine:         connection.Engine,
		connection:     reserved,
		transaction:    transaction,
		state:          dto.TransactionActive,
		startedAt:      now,
		lastActivityAt: now,
		expiresAt:      now.Add(registry.idleTTL),
		savepoints:     []dto.TransactionSavepoint{},
	}
	registry.mu.Lock()
	if registry.closed {
		registry.mu.Unlock()
		_ = transaction.Rollback()
		_ = reserved.Close()
		return dto.TransactionView{}, apperror.NewConflict("transaction runtime is shutting down", nil)
	}
	for registry.handles[id] != nil {
		id, err = newTransactionIdentifier()
		if err != nil {
			registry.mu.Unlock()
			_ = transaction.Rollback()
			_ = reserved.Close()
			return dto.TransactionView{}, apperror.NewInternal("transaction identifier could not be generated", err)
		}
		handle.id = id
	}
	registry.handles[id] = handle
	registry.activeByConnection[connection.ID]++
	registry.mu.Unlock()
	return handle.viewLocked(), nil
}

func (registry *TransactionRegistry) Get(ctx context.Context, transactionID string) (dto.TransactionView, error) {
	if err := ctx.Err(); err != nil {
		return dto.TransactionView{}, err
	}
	handle := registry.lookup(transactionID)
	if handle == nil {
		return dto.TransactionView{}, ports.ErrTransactionExpired
	}
	handle.mu.Lock()
	defer handle.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return dto.TransactionView{}, err
	}
	if handle.state == dto.TransactionExpired {
		return dto.TransactionView{}, ports.ErrTransactionExpired
	}
	if handle.state == dto.TransactionActive {
		now := time.Now().UTC()
		if !now.Before(handle.expiresAt) {
			return dto.TransactionView{}, registry.expireLocked(handle, now)
		}
		handle.touchLocked(now, registry.idleTTL)
	}
	return handle.viewLocked(), nil
}

func (registry *TransactionRegistry) Commit(ctx context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	handle, err := registry.lockActive(ctx, action.ConnectionID, action.TransactionID)
	if err != nil {
		return dto.TransactionView{}, err
	}
	defer handle.mu.Unlock()
	if err := handle.transaction.Commit(); err != nil {
		registry.discardLocked(handle, time.Now().UTC())
		return dto.TransactionView{}, err
	}
	registry.completeLocked(handle, dto.TransactionCommitted, time.Now().UTC())
	return handle.viewLocked(), nil
}

func (registry *TransactionRegistry) Rollback(ctx context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	handle, err := registry.lockActive(ctx, action.ConnectionID, action.TransactionID)
	if err != nil {
		return dto.TransactionView{}, err
	}
	defer handle.mu.Unlock()
	if err := handle.transaction.Rollback(); err != nil {
		registry.discardLocked(handle, time.Now().UTC())
		if errors.Is(err, sql.ErrTxDone) {
			return dto.TransactionView{}, ports.ErrTransactionExpired
		}
		return dto.TransactionView{}, err
	}
	registry.completeLocked(handle, dto.TransactionRolledBack, time.Now().UTC())
	return handle.viewLocked(), nil
}

func (registry *TransactionRegistry) Savepoint(ctx context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	if !validTransactionSavepoint(action.Savepoint) {
		return dto.TransactionView{}, apperror.NewValidation("invalid transaction savepoint", nil)
	}
	handle, err := registry.lockActive(ctx, action.ConnectionID, action.TransactionID)
	if err != nil {
		return dto.TransactionView{}, err
	}
	defer handle.mu.Unlock()
	quoted, err := quotedTransactionSavepoint(handle.engine, action.Savepoint)
	if err != nil {
		return dto.TransactionView{}, err
	}
	if _, err := handle.transaction.ExecContext(ctx, "SAVEPOINT "+quoted); err != nil {
		return dto.TransactionView{}, err
	}
	now := time.Now().UTC()
	if handle.engine == entity.EngineMySQL || handle.engine == entity.EngineMariaDB {
		handle.removeSavepointsNamedLocked(action.Savepoint)
	}
	handle.savepoints = append(handle.savepoints, dto.TransactionSavepoint{Name: action.Savepoint, CreatedAt: now})
	handle.touchLocked(now, registry.idleTTL)
	return handle.viewLocked(), nil
}

func (registry *TransactionRegistry) RollbackTo(ctx context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	if !validTransactionSavepoint(action.Savepoint) {
		return dto.TransactionView{}, apperror.NewValidation("invalid transaction savepoint", nil)
	}
	handle, err := registry.lockActive(ctx, action.ConnectionID, action.TransactionID)
	if err != nil {
		return dto.TransactionView{}, err
	}
	defer handle.mu.Unlock()
	position := handle.savepointPositionLocked(action.Savepoint)
	if position < 0 {
		return dto.TransactionView{}, apperror.NewConflict("transaction savepoint does not exist", nil)
	}
	quoted, err := quotedTransactionSavepoint(handle.engine, action.Savepoint)
	if err != nil {
		return dto.TransactionView{}, err
	}
	if _, err := handle.transaction.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+quoted); err != nil {
		return dto.TransactionView{}, err
	}
	handle.savepoints = append([]dto.TransactionSavepoint(nil), handle.savepoints[:position+1]...)
	handle.touchLocked(time.Now().UTC(), registry.idleTTL)
	return handle.viewLocked(), nil
}

func (registry *TransactionRegistry) Release(ctx context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	if !validTransactionSavepoint(action.Savepoint) {
		return dto.TransactionView{}, apperror.NewValidation("invalid transaction savepoint", nil)
	}
	handle, err := registry.lockActive(ctx, action.ConnectionID, action.TransactionID)
	if err != nil {
		return dto.TransactionView{}, err
	}
	defer handle.mu.Unlock()
	position := handle.savepointPositionLocked(action.Savepoint)
	if position < 0 {
		return dto.TransactionView{}, apperror.NewConflict("transaction savepoint does not exist", nil)
	}
	quoted, err := quotedTransactionSavepoint(handle.engine, action.Savepoint)
	if err != nil {
		return dto.TransactionView{}, err
	}
	if _, err := handle.transaction.ExecContext(ctx, "RELEASE SAVEPOINT "+quoted); err != nil {
		return dto.TransactionView{}, err
	}
	if handle.engine == entity.EnginePostgreSQL {
		handle.savepoints = append([]dto.TransactionSavepoint(nil), handle.savepoints[:position]...)
	} else {
		handle.savepoints = append(handle.savepoints[:position:position], handle.savepoints[position+1:]...)
	}
	handle.touchLocked(time.Now().UTC(), registry.idleTTL)
	return handle.viewLocked(), nil
}

func (registry *TransactionRegistry) WithExecutor(ctx context.Context, connectionID, transactionID string, callback func(queryExecutor) error) error {
	if callback == nil {
		return apperror.NewInternal("transaction executor callback is required", nil)
	}
	handle, err := registry.lockActive(ctx, connectionID, transactionID)
	if err != nil {
		return err
	}
	defer handle.mu.Unlock()
	err = callback(handle.transaction)
	now := time.Now().UTC()
	if errors.Is(err, sql.ErrTxDone) {
		registry.discardLocked(handle, now)
		return ports.ErrTransactionExpired
	}
	if handle.state == dto.TransactionActive {
		handle.touchLocked(now, registry.idleTTL)
	}
	return err
}

func (registry *TransactionRegistry) ActiveCount(connectionID string) int {
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	return registry.activeByConnection[connectionID]
}

func (registry *TransactionRegistry) CloseConnection(ctx context.Context, connectionID string) error {
	if connectionID == "" {
		return nil
	}
	handles := registry.detachConnection(connectionID)
	var result error
	for _, handle := range handles {
		handle.mu.Lock()
		if handle.state == dto.TransactionActive {
			result = errors.Join(result, handle.expireDetachedLocked(time.Now().UTC()))
		}
		handle.mu.Unlock()
	}
	if err := ctx.Err(); err != nil {
		result = errors.Join(result, err)
	}
	return result
}

func (registry *TransactionRegistry) Close(ctx context.Context) error {
	registry.closeOnce.Do(func() {
		registry.mu.Lock()
		registry.closed = true
		close(registry.stop)
		handles := make([]*managedTransaction, 0, len(registry.handles))
		for _, handle := range registry.handles {
			handles = append(handles, handle)
		}
		registry.handles = make(map[string]*managedTransaction)
		registry.activeByConnection = make(map[string]int)
		registry.mu.Unlock()
		select {
		case <-registry.done:
		case <-ctx.Done():
			registry.closeErr = errors.Join(registry.closeErr, ctx.Err())
		}
		for _, handle := range handles {
			handle.mu.Lock()
			if handle.state == dto.TransactionActive {
				registry.closeErr = errors.Join(registry.closeErr, handle.expireDetachedLocked(time.Now().UTC()))
			}
			handle.mu.Unlock()
		}
	})
	return registry.closeErr
}

func (registry *TransactionRegistry) lockActive(ctx context.Context, connectionID, transactionID string) (*managedTransaction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	handle := registry.lookup(transactionID)
	if handle == nil {
		return nil, ports.ErrTransactionExpired
	}
	handle.mu.Lock()
	if err := ctx.Err(); err != nil {
		handle.mu.Unlock()
		return nil, err
	}
	if handle.state == dto.TransactionExpired {
		handle.mu.Unlock()
		return nil, ports.ErrTransactionExpired
	}
	if handle.state != dto.TransactionActive {
		handle.mu.Unlock()
		return nil, apperror.NewConflict("transaction is not active", nil)
	}
	now := time.Now().UTC()
	if !now.Before(handle.expiresAt) {
		err := registry.expireLocked(handle, now)
		handle.mu.Unlock()
		return nil, err
	}
	if handle.connectionID != connectionID {
		handle.mu.Unlock()
		return nil, apperror.NewConflict("transaction belongs to another connection", nil)
	}
	handle.touchLocked(now, registry.idleTTL)
	return handle, nil
}

func (registry *TransactionRegistry) lookup(transactionID string) *managedTransaction {
	if transactionID == "" || len(transactionID) > 128 || strings.TrimSpace(transactionID) != transactionID {
		return nil
	}
	registry.mu.RLock()
	handle := registry.handles[transactionID]
	registry.mu.RUnlock()
	return handle
}

func (registry *TransactionRegistry) available() error {
	registry.mu.RLock()
	closed := registry.closed
	registry.mu.RUnlock()
	if closed {
		return apperror.NewConflict("transaction runtime is shutting down", nil)
	}
	return nil
}

func (registry *TransactionRegistry) completeLocked(handle *managedTransaction, state dto.TransactionLifecycleState, now time.Time) {
	handle.state = state
	handle.lastActivityAt = now
	handle.completedAt = &now
	handle.transaction = nil
	if handle.connection != nil {
		_ = handle.connection.Close()
		handle.connection = nil
	}
	registry.markInactive(handle)
}

func (registry *TransactionRegistry) discardLocked(handle *managedTransaction, now time.Time) {
	_ = handle.expireDetachedLocked(now)
	registry.remove(handle, true)
}

func (registry *TransactionRegistry) expireLocked(handle *managedTransaction, now time.Time) error {
	err := handle.expireDetachedLocked(now)
	registry.remove(handle, true)
	return errors.Join(ports.ErrTransactionExpired, err)
}

func (handle *managedTransaction) expireDetachedLocked(now time.Time) error {
	var result error
	if handle.transaction != nil {
		if err := handle.transaction.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			result = errors.Join(result, err)
		}
		handle.transaction = nil
	}
	if handle.connection != nil {
		result = errors.Join(result, handle.connection.Close())
		handle.connection = nil
	}
	handle.state = dto.TransactionExpired
	handle.lastActivityAt = now
	handle.completedAt = &now
	return result
}

func (registry *TransactionRegistry) markInactive(handle *managedTransaction) {
	registry.mu.Lock()
	if registry.handles[handle.id] == handle && registry.activeByConnection[handle.connectionID] > 0 {
		registry.activeByConnection[handle.connectionID]--
		if registry.activeByConnection[handle.connectionID] == 0 {
			delete(registry.activeByConnection, handle.connectionID)
		}
	}
	registry.mu.Unlock()
}

func (registry *TransactionRegistry) remove(handle *managedTransaction, active bool) {
	registry.mu.Lock()
	if registry.handles[handle.id] == handle {
		delete(registry.handles, handle.id)
		if active && registry.activeByConnection[handle.connectionID] > 0 {
			registry.activeByConnection[handle.connectionID]--
			if registry.activeByConnection[handle.connectionID] == 0 {
				delete(registry.activeByConnection, handle.connectionID)
			}
		}
	}
	registry.mu.Unlock()
}

func (registry *TransactionRegistry) detachConnection(connectionID string) []*managedTransaction {
	registry.mu.Lock()
	handles := make([]*managedTransaction, 0)
	for id, handle := range registry.handles {
		if handle.connectionID == connectionID {
			handles = append(handles, handle)
			delete(registry.handles, id)
		}
	}
	delete(registry.activeByConnection, connectionID)
	registry.mu.Unlock()
	return handles
}

func (registry *TransactionRegistry) reap() {
	ticker := time.NewTicker(transactionSweepInterval(registry.idleTTL))
	defer ticker.Stop()
	defer close(registry.done)
	for {
		select {
		case now := <-ticker.C:
			registry.sweep(now.UTC())
		case <-registry.stop:
			return
		}
	}
}

func (registry *TransactionRegistry) sweep(now time.Time) {
	registry.mu.RLock()
	handles := make([]*managedTransaction, 0, len(registry.handles))
	for _, handle := range registry.handles {
		handles = append(handles, handle)
	}
	registry.mu.RUnlock()
	for _, handle := range handles {
		handle.mu.Lock()
		switch handle.state {
		case dto.TransactionActive:
			if !now.Before(handle.expiresAt) {
				_ = registry.expireLocked(handle, now)
			}
		case dto.TransactionCommitted, dto.TransactionRolledBack:
			if handle.completedAt != nil && !now.Before(handle.completedAt.Add(registry.terminalRetention)) {
				registry.remove(handle, false)
			}
		case dto.TransactionExpired:
			registry.remove(handle, false)
		}
		handle.mu.Unlock()
	}
}

func (handle *managedTransaction) touchLocked(now time.Time, idleTTL time.Duration) {
	handle.lastActivityAt = now
	handle.expiresAt = now.Add(idleTTL)
}

func (handle *managedTransaction) viewLocked() dto.TransactionView {
	savepoints := append([]dto.TransactionSavepoint(nil), handle.savepoints...)
	if savepoints == nil {
		savepoints = []dto.TransactionSavepoint{}
	}
	return dto.TransactionView{
		ID:             handle.id,
		ConnectionID:   handle.connectionID,
		State:          handle.state,
		StartedAt:      handle.startedAt,
		LastActivityAt: handle.lastActivityAt,
		ExpiresAt:      handle.expiresAt,
		CompletedAt:    handle.completedAt,
		Savepoints:     savepoints,
	}
}

func (handle *managedTransaction) savepointPositionLocked(name string) int {
	for index := len(handle.savepoints) - 1; index >= 0; index-- {
		if handle.savepoints[index].Name == name {
			return index
		}
	}
	return -1
}

func (handle *managedTransaction) removeSavepointsNamedLocked(name string) {
	result := handle.savepoints[:0]
	for _, savepoint := range handle.savepoints {
		if savepoint.Name != name {
			result = append(result, savepoint)
		}
	}
	handle.savepoints = result
}

func newTransactionIdentifier() (string, error) {
	payload := make([]byte, transactionIdentifierBytes)
	if _, err := rand.Read(payload); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func normalizeTransactionIdleTTL(idleTTL time.Duration) time.Duration {
	if idleTTL <= 0 {
		return defaultTransactionIdleTTL
	}
	return min(idleTTL, maximumTransactionIdleTTL)
}

func transactionSweepInterval(idleTTL time.Duration) time.Duration {
	interval := idleTTL / 4
	if interval < 100*time.Millisecond {
		return 100 * time.Millisecond
	}
	return min(interval, 30*time.Second)
}

func validTransactionSavepoint(value string) bool {
	if value == "" || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for index, character := range value {
		letter := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
		if index == 0 {
			if !letter && character != '_' {
				return false
			}
			continue
		}
		if !letter && !(character >= '0' && character <= '9') && character != '_' {
			return false
		}
	}
	return true
}

func quotedTransactionSavepoint(engine entity.Engine, value string) (string, error) {
	if !validTransactionSavepoint(value) {
		return "", apperror.NewValidation("invalid transaction savepoint", nil)
	}
	dialect, err := dialectFor(engine)
	if err != nil {
		return "", err
	}
	return dialect.quoteIdentifier(value), nil
}
