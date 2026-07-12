package ports

import (
	"context"

	"github.com/huynhanx03/datadock/internal/core/entity"
)

type ConnectionRepository interface {
	Create(context.Context, entity.Connection) (entity.Connection, error)
	List(context.Context, string) ([]entity.Connection, error)
	Get(context.Context, string) (entity.Connection, error)
	Update(context.Context, entity.Connection) (entity.Connection, error)
	Delete(context.Context, string) error
}
