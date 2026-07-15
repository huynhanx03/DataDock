package engines

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

var _ ports.QueryRuntime = (*Manager)(nil)

type queryExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (manager *Manager) Execute(ctx context.Context, connection entity.Connection, password string, request ports.QueryRuntimeRequest) (dto.QueryExecutionResult, error) {
	statement, err := manager.ClassifyStatement(connection.Engine, request.SQL)
	if err != nil {
		return dto.QueryExecutionResult{}, err
	}
	if statement.Type == dto.QueryStatementTransaction {
		return dto.QueryExecutionResult{}, apperror.NewValidation("use the transaction API for transaction control", nil)
	}
	executionContext, release, err := manager.executions.Begin(ctx, request.ExecutionID)
	if err != nil {
		return dto.QueryExecutionResult{}, err
	}
	defer release()
	startedAt := time.Now()
	result := dto.QueryExecutionResult{
		ExecutionID:   request.ExecutionID,
		StatementType: statement.Type,
		Columns:       []dto.DataColumn{},
		Rows:          [][]any{},
		Limits:        normalizedQueryLimits(request.Limits),
		Notices:       []dto.QueryNotice{},
	}
	err = manager.withQueryExecutor(executionContext, connection, password, request.TransactionID, func(executor queryExecutor) error {
		var executionErr error
		if statementReturnsRows(request.SQL, statement) {
			result, executionErr = executeRows(executionContext, executor, request, result)
		} else {
			result, executionErr = executeCommand(executionContext, executor, request.SQL, result)
		}
		return executionErr
	})
	result.DurationMS = time.Since(startedAt).Milliseconds()
	if err != nil {
		return dto.QueryExecutionResult{}, queryRuntimeError(executionContext, err)
	}
	return result, nil
}

func (manager *Manager) Explain(ctx context.Context, connection entity.Connection, password string, request ports.QueryRuntimeRequest, mode dto.QueryExplainMode) (dto.QueryExecutionResult, error) {
	statement, err := manager.ClassifyStatement(connection.Engine, request.SQL)
	if err != nil {
		return dto.QueryExecutionResult{}, err
	}
	if mode != dto.QueryExplainOnly && mode != dto.QueryExplainAnalyze {
		return dto.QueryExecutionResult{}, apperror.NewValidation("invalid explain mode", nil)
	}
	executionContext, release, err := manager.executions.Begin(ctx, request.ExecutionID)
	if err != nil {
		return dto.QueryExecutionResult{}, err
	}
	defer release()
	startedAt := time.Now()
	var plan dto.QueryExplainPlan
	err = manager.withQueryExecutor(executionContext, connection, password, request.TransactionID, func(executor queryExecutor) error {
		var explainErr error
		if connection.Engine == entity.EnginePostgreSQL {
			plan, explainErr = explainPostgres(executionContext, executor, request.SQL, mode)
		} else {
			plan, explainErr = explainMySQL(executionContext, executor, connection.Engine, request.SQL, mode)
		}
		return explainErr
	})
	if err != nil {
		return dto.QueryExecutionResult{}, queryRuntimeError(executionContext, err)
	}
	rowsRead := len(plan.Table.Rows)
	return dto.QueryExecutionResult{
		ExecutionID:   request.ExecutionID,
		StatementType: statement.Type,
		Columns:       []dto.DataColumn{},
		Rows:          [][]any{},
		DurationMS:    time.Since(startedAt).Milliseconds(),
		Limits: dto.QueryExecutionLimits{
			MaxRows:   request.Limits.MaxRows,
			MaxBytes:  request.Limits.MaxBytes,
			RowsRead:  rowsRead,
			BytesRead: 0,
		},
		Notices: []dto.QueryNotice{},
		Plan:    &plan,
	}, nil
}

func (manager *Manager) Cancel(ctx context.Context, executionID string) error {
	return manager.executions.Cancel(ctx, executionID)
}

func (manager *Manager) withQueryExecutor(ctx context.Context, connection entity.Connection, password, transactionID string, callback func(queryExecutor) error) error {
	if transactionID != "" {
		return manager.withTransactionExecutor(ctx, connection.ID, transactionID, callback)
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return err
	}
	if !connection.ReadOnly {
		return callback(database)
	}
	transaction, err := database.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	if err := callback(transaction); err != nil {
		_ = transaction.Rollback()
		return err
	}
	return transaction.Commit()
}

func executeRows(ctx context.Context, executor queryExecutor, request ports.QueryRuntimeRequest, result dto.QueryExecutionResult) (dto.QueryExecutionResult, error) {
	rows, err := executor.QueryContext(ctx, request.SQL)
	if err != nil {
		return dto.QueryExecutionResult{}, err
	}
	defer rows.Close()
	columns, values, truncated, bytesRead, err := scanTypedRows(rows, nil, result.Limits.MaxRows, result.Limits.MaxBytes)
	if err != nil {
		return dto.QueryExecutionResult{}, err
	}
	result.Columns = columns
	result.Rows = values
	result.Truncated = truncated
	result.Limits.RowsRead = len(values)
	result.Limits.BytesRead = bytesRead
	return result, nil
}

func executeCommand(ctx context.Context, executor queryExecutor, sqlText string, result dto.QueryExecutionResult) (dto.QueryExecutionResult, error) {
	execution, err := executor.ExecContext(ctx, sqlText)
	if err != nil {
		return dto.QueryExecutionResult{}, err
	}
	rowsAffected, err := execution.RowsAffected()
	if err != nil {
		return dto.QueryExecutionResult{}, err
	}
	result.RowsAffected = rowsAffected
	return result, nil
}

func normalizedQueryLimits(limits dto.QueryExecutionLimits) dto.QueryExecutionLimits {
	if limits.MaxRows < 1 {
		limits.MaxRows = 1000
	}
	if limits.MaxBytes < 1 {
		limits.MaxBytes = 10 * 1024 * 1024
	}
	limits.RowsRead = 0
	limits.BytesRead = 0
	return limits
}

func queryRuntimeError(ctx context.Context, err error) error {
	if ctx.Err() != nil || errors.Is(context.Cause(ctx), errExecutionCancelled) {
		return executionContextError(ctx)
	}
	if errors.Is(err, ports.ErrTransactionExpired) {
		return apperror.NewTransactionExpired("transaction was not found or has expired", err)
	}
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return err
	}
	return err
}
