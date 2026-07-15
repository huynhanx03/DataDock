package ports

import (
	"context"

	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type ConnectionProfileResolver interface {
	Resolve(context.Context, string) (entity.Connection, string, error)
}

type CatalogReader interface {
	ReadCatalog(context.Context, entity.Connection, string, dto.CatalogInput) (dto.CatalogTree, error)
}
