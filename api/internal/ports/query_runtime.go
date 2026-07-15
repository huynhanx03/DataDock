package ports

import (
	"context"

	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type QueryStatement struct {
	Type     dto.QueryStatementType
	ReadOnly bool
}

type QueryRuntimeRequest struct {
	ExecutionID   string
	SQL           string
	TransactionID string
	Limits        dto.QueryExecutionLimits
}

type QueryRuntime interface {
	ClassifyStatement(entity.Engine, string) (QueryStatement, error)
	Execute(context.Context, entity.Connection, string, QueryRuntimeRequest) (dto.QueryExecutionResult, error)
	Explain(context.Context, entity.Connection, string, QueryRuntimeRequest, dto.QueryExplainMode) (dto.QueryExecutionResult, error)
	Cancel(context.Context, string) error
}
