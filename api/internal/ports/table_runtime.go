package ports

import (
	"context"

	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type TableReader interface {
	ReadRows(context.Context, entity.Connection, string, dto.TableRowsInput) (dto.TableRowsResult, error)
	ReadTableSchema(context.Context, entity.Connection, string, string) (dto.TableSchema, error)
	ReadTableDDL(context.Context, entity.Connection, string, string) (string, error)
}
