package ports

import (
	"context"

	"github.com/huynhanx03/datadock/internal/core/entity"
)

type WorkspaceRepository interface {
	Create(context.Context, entity.Workspace) (entity.Workspace, error)
	List(context.Context) ([]entity.Workspace, error)
	Get(context.Context, string) (entity.Workspace, error)
	Update(context.Context, entity.Workspace) (entity.Workspace, error)
	Delete(context.Context, string) error
	Reorder(context.Context, []string) error
}
