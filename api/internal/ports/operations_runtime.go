package ports

import (
	"context"

	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type OperationsRuntime interface {
	Dashboard(context.Context, entity.Connection, string, dto.OperationsRangeInput) (dto.OperationsDashboard, error)
	Sessions(context.Context, entity.Connection, string, dto.OperationsSessionsInput) (dto.OperationsSessions, error)
	Locks(context.Context, entity.Connection, string, dto.OperationsLocksInput) (dto.OperationsLocks, error)
	Performance(context.Context, entity.Connection, string, dto.OperationsPerformanceInput) (dto.OperationsPerformance, error)
	ControlSession(context.Context, entity.Connection, string, dto.OperationsSessionControlInput) error
}
