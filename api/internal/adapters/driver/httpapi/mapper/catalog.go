package mapper

import "github.com/huynhanx03/datadock/internal/core/dto"

type CatalogQuery struct {
	ParentReference string
	Cursor          string
	Limit           int
	Depth           dto.CatalogDepth
}

type TableRowsRequest dto.TableRowsInput
type CatalogResponse dto.CatalogTree
type TableRowsResponse dto.TableRowsResult
type TableSchemaResponse dto.TableSchema
type TableDDLResponse struct {
	DDL string `json:"ddl"`
}

func ToCatalogInput(query CatalogQuery) dto.CatalogInput {
	return dto.CatalogInput{
		ParentReference: query.ParentReference,
		Cursor:          query.Cursor,
		Limit:           query.Limit,
		Depth:           query.Depth,
	}
}

func ToTableRowsInput(request TableRowsRequest) dto.TableRowsInput {
	return dto.TableRowsInput(request)
}

func ToCatalogResponse(tree dto.CatalogTree) CatalogResponse {
	return CatalogResponse(tree)
}

func ToTableRowsResponse(result dto.TableRowsResult) TableRowsResponse {
	return TableRowsResponse(result)
}

func ToTableSchemaResponse(schema dto.TableSchema) TableSchemaResponse {
	return TableSchemaResponse(schema)
}
