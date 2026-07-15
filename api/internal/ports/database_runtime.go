package ports

import (
	"context"

	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type DatabaseRuntime interface {
	ListTables(context.Context, entity.Connection, string) ([]string, error)
	BrowseTable(context.Context, entity.Connection, string, dto.TableRowsInput) (dto.TableRowsResult, error)
	MutateRows(context.Context, entity.Connection, string, string, []dto.RowMutation) (dto.TableMutateResult, error)
	TableDDL(context.Context, entity.Connection, string, string) (string, error)
	TableSchema(context.Context, entity.Connection, string, string) (dto.TableSchema, error)
	CreateTable(context.Context, entity.Connection, string, string, []dto.ColumnDefinition) error
	AlterTable(context.Context, entity.Connection, string, string, []dto.TableAlteration) error
	CreateIndex(context.Context, entity.Connection, string, string, dto.CreateIndexInput) error
	DropIndex(context.Context, entity.Connection, string, string, string) error
	Dashboard(context.Context, entity.Connection, string) (dto.DatabaseDashboard, error)
	Sessions(context.Context, entity.Connection, string) (dto.DatabaseSessions, error)
	Locks(context.Context, entity.Connection, string) (dto.DatabaseLocks, error)
	Performance(context.Context, entity.Connection, string) (dto.DatabasePerformance, error)
	Execute(context.Context, entity.Connection, string, string, int) (dto.QueryResult, error)
	ExecuteTransaction(context.Context, string, string, int) (dto.QueryResult, error)
	BeginTransaction(context.Context, entity.Connection, string) (dto.TransactionState, error)
	TransactionAction(context.Context, string, string, string) (dto.TransactionState, error)
	CancelSession(context.Context, entity.Connection, string, string, bool) error
	IsWriteSQL(string) bool
}
