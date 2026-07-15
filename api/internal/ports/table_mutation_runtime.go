package ports

import (
	"context"

	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type TableMutationRuntimeRequest struct {
	Reference     string
	TransactionID string
	Mutations     []dto.TableMutation
}

type TableMutationRuntime interface {
	MutationMetadata(context.Context, entity.Connection, string, string, string) (dto.TableMutationMetadata, error)
	ApplyMutations(context.Context, entity.Connection, string, TableMutationRuntimeRequest) (dto.TableMutationBatchResult, error)
}
