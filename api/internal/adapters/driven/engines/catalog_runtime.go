package engines

import (
	"context"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

var _ ports.CatalogReader = (*Manager)(nil)
var _ ports.TableReader = (*Manager)(nil)

func (manager *Manager) ReadCatalog(ctx context.Context, connection entity.Connection, password string, input dto.CatalogInput) (dto.CatalogTree, error) {
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return dto.CatalogTree{}, err
	}
	if connection.Engine == entity.EnginePostgreSQL {
		return readPostgresCatalog(ctx, database, connection, input)
	}
	if connection.Engine == entity.EngineMySQL || connection.Engine == entity.EngineMariaDB {
		return readMySQLCatalog(ctx, database, connection, input)
	}
	return dto.CatalogTree{}, apperror.NewUnsupported("database catalog is not supported", nil)
}

func (manager *Manager) ReadTableSchema(ctx context.Context, connection entity.Connection, password, encodedReference string) (dto.TableSchema, error) {
	reference, err := decodeCatalogReference(encodedReference)
	if err != nil {
		return dto.TableSchema{}, err
	}
	if err := ensureBrowsableReference(reference); err != nil {
		return dto.TableSchema{}, err
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return dto.TableSchema{}, err
	}
	if connection.Engine == entity.EnginePostgreSQL {
		return readPostgresTableSchema(ctx, database, connection, reference)
	}
	if connection.Engine == entity.EngineMySQL || connection.Engine == entity.EngineMariaDB {
		return readMySQLTableSchema(ctx, database, connection, reference)
	}
	return dto.TableSchema{}, apperror.NewUnsupported("table schema is not supported", nil)
}

func (manager *Manager) ReadTableDDL(ctx context.Context, connection entity.Connection, password, encodedReference string) (string, error) {
	reference, err := decodeCatalogReference(encodedReference)
	if err != nil {
		return "", err
	}
	if err := ensureBrowsableReference(reference); err != nil {
		return "", err
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return "", err
	}
	if connection.Engine == entity.EnginePostgreSQL {
		return readPostgresTableDDL(ctx, database, connection, reference)
	}
	if connection.Engine == entity.EngineMySQL || connection.Engine == entity.EngineMariaDB {
		return readMySQLTableDDL(ctx, database, connection, reference)
	}
	return "", apperror.NewUnsupported("table DDL is not supported", nil)
}
