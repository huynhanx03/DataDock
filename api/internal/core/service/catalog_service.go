package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/ports"
)

type CatalogService struct {
	profiles ports.ConnectionProfileResolver
	catalog  ports.CatalogReader
	tables   ports.TableReader
}

func NewCatalogService(profiles ports.ConnectionProfileResolver, catalog ports.CatalogReader, tables ports.TableReader) *CatalogService {
	return &CatalogService{profiles: profiles, catalog: catalog, tables: tables}
}

func (service *CatalogService) Catalog(ctx context.Context, connectionID string, input dto.CatalogInput) (dto.CatalogTree, error) {
	if input.Limit == 0 {
		input.Limit = 100
	}
	if input.Depth == "" {
		input.Depth = dto.CatalogDepthSummary
	}
	if input.Limit < 1 || input.Limit > 500 || input.Depth != dto.CatalogDepthSummary && input.Depth != dto.CatalogDepthAll || invalidOpaqueValue(input.ParentReference, 4096) || invalidOpaqueValue(input.Cursor, 2048) {
		return dto.CatalogTree{}, apperror.NewValidation("invalid catalog request", nil)
	}
	connection, password, err := service.profiles.Resolve(ctx, connectionID)
	if err != nil {
		return dto.CatalogTree{}, catalogServiceError(err)
	}
	result, err := service.catalog.ReadCatalog(ctx, connection, password, input)
	if err != nil {
		return dto.CatalogTree{}, catalogServiceError(err)
	}
	result.ConnectionID = connection.ID
	result.Engine = connection.Engine
	if result.LoadedAt.IsZero() {
		result.LoadedAt = time.Now().UTC()
	}
	normalizeCatalogObjects(result.Databases, connection.ID)
	return result, nil
}

func (service *CatalogService) TableRows(ctx context.Context, connectionID string, input dto.TableRowsInput) (dto.TableRowsResult, error) {
	if input.Limit == 0 {
		input.Limit = 100
	}
	if invalidTableRequest(input) {
		return dto.TableRowsResult{}, apperror.NewValidation("invalid table request", nil)
	}
	connection, password, err := service.profiles.Resolve(ctx, connectionID)
	if err != nil {
		return dto.TableRowsResult{}, catalogServiceError(err)
	}
	input.ConnectionID = connection.ID
	result, err := service.tables.ReadRows(ctx, connection, password, input)
	if err != nil {
		return dto.TableRowsResult{}, catalogServiceError(err)
	}
	return result, nil
}

func (service *CatalogService) TableSchema(ctx context.Context, connectionID, reference string) (dto.TableSchema, error) {
	if invalidReference(reference) {
		return dto.TableSchema{}, apperror.NewValidation("invalid table reference", nil)
	}
	connection, password, err := service.profiles.Resolve(ctx, connectionID)
	if err != nil {
		return dto.TableSchema{}, catalogServiceError(err)
	}
	result, err := service.tables.ReadTableSchema(ctx, connection, password, reference)
	return result, catalogServiceError(err)
}

func (service *CatalogService) TableDDL(ctx context.Context, connectionID, reference string) (string, error) {
	if invalidReference(reference) {
		return "", apperror.NewValidation("invalid table reference", nil)
	}
	connection, password, err := service.profiles.Resolve(ctx, connectionID)
	if err != nil {
		return "", catalogServiceError(err)
	}
	result, err := service.tables.ReadTableDDL(ctx, connection, password, reference)
	return result, catalogServiceError(err)
}

func invalidTableRequest(input dto.TableRowsInput) bool {
	if invalidReference(input.Reference) || input.Limit < 1 || input.Limit > 500 || input.Offset < 0 || len(input.Search) > 500 || len(input.Columns) > 200 || len(input.Sorts) > 16 || len(input.Filters) > 32 {
		return true
	}
	for _, sort := range input.Sorts {
		if strings.TrimSpace(sort.Column) == "" || sort.Direction != "asc" && sort.Direction != "desc" {
			return true
		}
	}
	allowedOperators := map[string]bool{"eq": true, "ne": true, "lt": true, "lte": true, "gt": true, "gte": true, "contains": true, "starts_with": true, "ends_with": true, "is_null": true, "is_not_null": true, "in": true}
	for _, filter := range input.Filters {
		if strings.TrimSpace(filter.Column) == "" || !allowedOperators[filter.Operator] {
			return true
		}
	}
	return false
}

func invalidReference(value string) bool {
	return strings.TrimSpace(value) == "" || invalidOpaqueValue(value, 4096)
}

func invalidOpaqueValue(value string, maximum int) bool {
	return len(value) > maximum || strings.ContainsRune(value, 0)
}

func normalizeCatalogObjects(objects []dto.CatalogObject, connectionID string) {
	for index := range objects {
		objects[index].ConnectionID = connectionID
		normalizeCatalogObjects(objects[index].Children, connectionID)
	}
}

func catalogServiceError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, ports.ErrNotFound) {
		return apperror.NewNotFound("database object not found", err)
	}
	return apperror.NewInternal("database metadata operation failed", err)
}
