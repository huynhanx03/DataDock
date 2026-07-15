package ports

import (
	"context"
	"errors"

	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

var ErrTransactionExpired = errors.New("transaction expired")

type TransactionRuntimeAction struct {
	ConnectionID  string
	TransactionID string
	Savepoint     string
}

type TransactionRuntime interface {
	Begin(context.Context, entity.Connection, string) (dto.TransactionView, error)
	Get(context.Context, string) (dto.TransactionView, error)
	Commit(context.Context, TransactionRuntimeAction) (dto.TransactionView, error)
	Rollback(context.Context, TransactionRuntimeAction) (dto.TransactionView, error)
	Savepoint(context.Context, TransactionRuntimeAction) (dto.TransactionView, error)
	RollbackTo(context.Context, TransactionRuntimeAction) (dto.TransactionView, error)
	Release(context.Context, TransactionRuntimeAction) (dto.TransactionView, error)
}
