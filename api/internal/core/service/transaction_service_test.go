package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/core/service"
	"github.com/huynhanx03/datadock/internal/ports"
)

func TestTransactionServiceBeginsAndReturnsExpiryMetadata(t *testing.T) {
	view := activeTransactionFixture("transaction-1", "connection-1")
	profiles := &transactionProfileResolverFake{connection: tableConnectionFixture(false), password: "database-secret"}
	runtime := &transactionRuntimeFake{beginView: view}
	subject := service.NewTransactionService(profiles, runtime)

	result, err := subject.Begin(context.Background(), dto.TransactionBeginInput{ConnectionID: "connection-1"})
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if result.ID != "transaction-1" || result.ConnectionID != "connection-1" || result.State != dto.TransactionActive || result.StartedAt.IsZero() || result.LastActivityAt.IsZero() || result.ExpiresAt.IsZero() || !result.ExpiresAt.After(result.LastActivityAt) {
		t.Fatalf("Begin() result = %#v", result)
	}
	if runtime.beginCalls != 1 || runtime.lastConnection.ID != "connection-1" || runtime.lastPassword != "database-secret" {
		t.Fatalf("runtime begin calls=%d connection=%#v password=%q", runtime.beginCalls, runtime.lastConnection, runtime.lastPassword)
	}
}

func TestTransactionServiceGetsOwnedStatus(t *testing.T) {
	view := activeTransactionFixture("transaction-1", "connection-1")
	view.Savepoints = []dto.TransactionSavepoint{{Name: "before_update", CreatedAt: time.Now().UTC()}}
	profiles := &transactionProfileResolverFake{connection: tableConnectionFixture(false)}
	runtime := &transactionRuntimeFake{getView: view}
	subject := service.NewTransactionService(profiles, runtime)

	result, err := subject.Get(context.Background(), dto.TransactionActionInput{ConnectionID: "connection-1", TransactionID: "transaction-1"})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if result.ID != view.ID || len(result.Savepoints) != 1 || result.Savepoints[0].Name != "before_update" {
		t.Fatalf("Get() result = %#v", result)
	}
}

func TestTransactionServiceEnforcesConnectionOwnershipBeforeAction(t *testing.T) {
	profiles := &transactionProfileResolverFake{connection: tableConnectionFixture(false)}
	runtime := &transactionRuntimeFake{getView: activeTransactionFixture("transaction-1", "connection-other")}
	subject := service.NewTransactionService(profiles, runtime)

	_, err := subject.Commit(context.Background(), dto.TransactionActionInput{ConnectionID: "connection-1", TransactionID: "transaction-1"})
	if transactionErrorCode(err) != apperror.CodeConflict {
		t.Fatalf("Commit() error = %#v", err)
	}
	if runtime.commitCalls != 0 || runtime.rollbackCalls != 0 {
		t.Fatalf("ownership mismatch reached action commit=%d rollback=%d", runtime.commitCalls, runtime.rollbackCalls)
	}
}

func TestTransactionServiceDispatchesAllValidActions(t *testing.T) {
	active := activeTransactionFixture("transaction-1", "connection-1")
	committed := active
	committed.State = dto.TransactionCommitted
	completedAt := time.Now().UTC()
	committed.CompletedAt = &completedAt
	rolledBack := committed
	rolledBack.State = dto.TransactionRolledBack

	tests := []struct {
		name   string
		invoke func(*service.TransactionService) (dto.TransactionView, error)
		setup  func(*transactionRuntimeFake)
		calls  func(*transactionRuntimeFake) int
	}{
		{
			name: "commit",
			invoke: func(subject *service.TransactionService) (dto.TransactionView, error) {
				return subject.Commit(context.Background(), dto.TransactionActionInput{ConnectionID: "connection-1", TransactionID: "transaction-1"})
			},
			setup: func(runtime *transactionRuntimeFake) { runtime.commitView = committed },
			calls: func(runtime *transactionRuntimeFake) int { return runtime.commitCalls },
		},
		{
			name: "rollback",
			invoke: func(subject *service.TransactionService) (dto.TransactionView, error) {
				return subject.Rollback(context.Background(), dto.TransactionActionInput{ConnectionID: "connection-1", TransactionID: "transaction-1"})
			},
			setup: func(runtime *transactionRuntimeFake) { runtime.rollbackView = rolledBack },
			calls: func(runtime *transactionRuntimeFake) int { return runtime.rollbackCalls },
		},
		{
			name: "savepoint",
			invoke: func(subject *service.TransactionService) (dto.TransactionView, error) {
				return subject.Savepoint(context.Background(), dto.TransactionSavepointInput{ConnectionID: "connection-1", TransactionID: "transaction-1", Name: "before_update"})
			},
			setup: func(runtime *transactionRuntimeFake) { runtime.savepointView = active },
			calls: func(runtime *transactionRuntimeFake) int { return runtime.savepointCalls },
		},
		{
			name: "rollback to",
			invoke: func(subject *service.TransactionService) (dto.TransactionView, error) {
				return subject.RollbackTo(context.Background(), dto.TransactionSavepointInput{ConnectionID: "connection-1", TransactionID: "transaction-1", Name: "before_update"})
			},
			setup: func(runtime *transactionRuntimeFake) { runtime.rollbackToView = active },
			calls: func(runtime *transactionRuntimeFake) int { return runtime.rollbackToCalls },
		},
		{
			name: "release",
			invoke: func(subject *service.TransactionService) (dto.TransactionView, error) {
				return subject.Release(context.Background(), dto.TransactionSavepointInput{ConnectionID: "connection-1", TransactionID: "transaction-1", Name: "before_update"})
			},
			setup: func(runtime *transactionRuntimeFake) { runtime.releaseView = active },
			calls: func(runtime *transactionRuntimeFake) int { return runtime.releaseCalls },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profiles := &transactionProfileResolverFake{connection: tableConnectionFixture(false)}
			runtime := &transactionRuntimeFake{getView: active}
			test.setup(runtime)
			subject := service.NewTransactionService(profiles, runtime)

			result, err := test.invoke(subject)
			if err != nil {
				t.Fatalf("action error = %v", err)
			}
			if result.ID != "transaction-1" || test.calls(runtime) != 1 || runtime.lastAction.ConnectionID != "connection-1" || runtime.lastAction.TransactionID != "transaction-1" {
				t.Fatalf("action result=%#v calls=%d request=%#v", result, test.calls(runtime), runtime.lastAction)
			}
			if test.name == "savepoint" || test.name == "rollback to" || test.name == "release" {
				if runtime.lastAction.Savepoint != "before_update" {
					t.Fatalf("savepoint request = %#v", runtime.lastAction)
				}
			}
		})
	}
}

func TestTransactionServiceRejectsInvalidSavepointBeforeDependencies(t *testing.T) {
	for _, name := range []string{"", "1_before", "before update", "before;COMMIT", "trước_update", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} {
		t.Run(name, func(t *testing.T) {
			profiles := &transactionProfileResolverFake{connection: tableConnectionFixture(false)}
			runtime := &transactionRuntimeFake{}
			subject := service.NewTransactionService(profiles, runtime)

			_, err := subject.Savepoint(context.Background(), dto.TransactionSavepointInput{ConnectionID: "connection-1", TransactionID: "transaction-1", Name: name})
			if transactionErrorCode(err) != apperror.CodeValidation {
				t.Fatalf("Savepoint(%q) error = %#v", name, err)
			}
			if profiles.calls != 0 || runtime.getCalls != 0 || runtime.savepointCalls != 0 {
				t.Fatalf("invalid savepoint reached dependencies")
			}
		})
	}
}

func TestTransactionServiceMapsExpiredHandleAndLeavesCleanupToRuntime(t *testing.T) {
	profiles := &transactionProfileResolverFake{connection: tableConnectionFixture(false)}
	runtime := &transactionRuntimeFake{getErr: ports.ErrTransactionExpired}
	subject := service.NewTransactionService(profiles, runtime)

	_, err := subject.Commit(context.Background(), dto.TransactionActionInput{ConnectionID: "connection-1", TransactionID: "transaction-expired"})
	if transactionErrorCode(err) != apperror.CodeTransactionExpired || err.Error() != "transaction expired" {
		t.Fatalf("Commit() error = %#v", err)
	}
	if runtime.commitCalls != 0 || runtime.rollbackCalls != 0 {
		t.Fatalf("service performed cleanup commit=%d rollback=%d", runtime.commitCalls, runtime.rollbackCalls)
	}
}

func TestTransactionServiceMapsExpiryRaceWithoutCompensatingRollback(t *testing.T) {
	profiles := &transactionProfileResolverFake{connection: tableConnectionFixture(false)}
	runtime := &transactionRuntimeFake{
		getView:   activeTransactionFixture("transaction-1", "connection-1"),
		commitErr: ports.ErrTransactionExpired,
	}
	subject := service.NewTransactionService(profiles, runtime)

	_, err := subject.Commit(context.Background(), dto.TransactionActionInput{ConnectionID: "connection-1", TransactionID: "transaction-1"})
	if transactionErrorCode(err) != apperror.CodeTransactionExpired {
		t.Fatalf("Commit() error = %#v", err)
	}
	if runtime.commitCalls != 1 || runtime.rollbackCalls != 0 {
		t.Fatalf("cleanup calls commit=%d rollback=%d", runtime.commitCalls, runtime.rollbackCalls)
	}
}

func TestTransactionServiceMapsExpiredState(t *testing.T) {
	view := activeTransactionFixture("transaction-1", "connection-1")
	view.State = dto.TransactionExpired
	profiles := &transactionProfileResolverFake{connection: tableConnectionFixture(false)}
	runtime := &transactionRuntimeFake{getView: view}
	subject := service.NewTransactionService(profiles, runtime)

	_, err := subject.Get(context.Background(), dto.TransactionActionInput{ConnectionID: "connection-1", TransactionID: "transaction-1"})
	if transactionErrorCode(err) != apperror.CodeTransactionExpired {
		t.Fatalf("Get() error = %#v", err)
	}
}

func TestTransactionServiceRejectsInvalidIdentifiersBeforeDependencies(t *testing.T) {
	profiles := &transactionProfileResolverFake{connection: tableConnectionFixture(false)}
	runtime := &transactionRuntimeFake{}
	subject := service.NewTransactionService(profiles, runtime)

	_, err := subject.Get(context.Background(), dto.TransactionActionInput{ConnectionID: " connection-1", TransactionID: "transaction-1\x00"})
	if transactionErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("Get() error = %#v", err)
	}
	if profiles.calls != 0 || runtime.getCalls != 0 {
		t.Fatalf("invalid identifiers reached dependencies")
	}
}

type transactionProfileResolverFake struct {
	connection entity.Connection
	password   string
	err        error
	calls      int
}

func (fake *transactionProfileResolverFake) Resolve(context.Context, string) (entity.Connection, string, error) {
	fake.calls++
	return fake.connection, fake.password, fake.err
}

type transactionRuntimeFake struct {
	beginView       dto.TransactionView
	getView         dto.TransactionView
	commitView      dto.TransactionView
	rollbackView    dto.TransactionView
	savepointView   dto.TransactionView
	rollbackToView  dto.TransactionView
	releaseView     dto.TransactionView
	beginErr        error
	getErr          error
	commitErr       error
	rollbackErr     error
	savepointErr    error
	rollbackToErr   error
	releaseErr      error
	beginCalls      int
	getCalls        int
	commitCalls     int
	rollbackCalls   int
	savepointCalls  int
	rollbackToCalls int
	releaseCalls    int
	lastConnection  entity.Connection
	lastPassword    string
	lastAction      ports.TransactionRuntimeAction
}

func (fake *transactionRuntimeFake) Begin(_ context.Context, connection entity.Connection, password string) (dto.TransactionView, error) {
	fake.beginCalls++
	fake.lastConnection = connection
	fake.lastPassword = password
	return fake.beginView, fake.beginErr
}

func (fake *transactionRuntimeFake) Get(context.Context, string) (dto.TransactionView, error) {
	fake.getCalls++
	return fake.getView, fake.getErr
}

func (fake *transactionRuntimeFake) Commit(_ context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	fake.commitCalls++
	fake.lastAction = action
	return fake.commitView, fake.commitErr
}

func (fake *transactionRuntimeFake) Rollback(_ context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	fake.rollbackCalls++
	fake.lastAction = action
	return fake.rollbackView, fake.rollbackErr
}

func (fake *transactionRuntimeFake) Savepoint(_ context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	fake.savepointCalls++
	fake.lastAction = action
	return fake.savepointView, fake.savepointErr
}

func (fake *transactionRuntimeFake) RollbackTo(_ context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	fake.rollbackToCalls++
	fake.lastAction = action
	return fake.rollbackToView, fake.rollbackToErr
}

func (fake *transactionRuntimeFake) Release(_ context.Context, action ports.TransactionRuntimeAction) (dto.TransactionView, error) {
	fake.releaseCalls++
	fake.lastAction = action
	return fake.releaseView, fake.releaseErr
}

func activeTransactionFixture(id, connectionID string) dto.TransactionView {
	now := time.Now().UTC()
	return dto.TransactionView{
		ID:             id,
		ConnectionID:   connectionID,
		State:          dto.TransactionActive,
		StartedAt:      now.Add(-time.Minute),
		LastActivityAt: now,
		ExpiresAt:      now.Add(15 * time.Minute),
		Savepoints:     []dto.TransactionSavepoint{},
	}
}

func transactionErrorCode(err error) string {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}
