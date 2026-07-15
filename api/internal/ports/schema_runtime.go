package ports

import (
	"context"

	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type SchemaRuntimePreviewRequest struct {
	Actions []dto.SchemaAction
}

type SchemaRuntimeApplyRequest struct {
	PreviewHash string
	Steps       []dto.SchemaStep
}

type SchemaRuntimeApplyResult struct {
	AppliedSteps int
}

type SchemaRuntime interface {
	Preview(context.Context, entity.Connection, string, SchemaRuntimePreviewRequest) ([]dto.SchemaStep, error)
	Apply(context.Context, entity.Connection, string, SchemaRuntimeApplyRequest) (SchemaRuntimeApplyResult, error)
}
