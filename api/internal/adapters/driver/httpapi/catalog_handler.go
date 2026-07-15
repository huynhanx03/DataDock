package httpapi

import (
	"strconv"

	"github.com/gin-gonic/gin"
	httpmapper "github.com/huynhanx03/datadock/internal/adapters/driver/httpapi/mapper"
	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/service"
)

type CatalogHandler struct {
	service *service.CatalogService
}

func NewCatalogHandler(catalogService *service.CatalogService) *CatalogHandler {
	return &CatalogHandler{service: catalogService}
}

func (handler *CatalogHandler) Register(router *gin.RouterGroup) {
	router.GET("/connections/:id/catalog", handler.catalog)
	router.POST("/connections/:id/table/rows", handler.tableRows)
	router.GET("/connections/:id/table/schema", handler.tableSchema)
	router.GET("/connections/:id/table/ddl", handler.tableDDL)
}

func (handler *CatalogHandler) catalog(ctx *gin.Context) {
	limit, err := catalogLimit(ctx.Query("limit"))
	if err != nil {
		WriteError(ctx, err)
		return
	}
	result, err := handler.service.Catalog(ctx, ctx.Param("id"), httpmapper.ToCatalogInput(httpmapper.CatalogQuery{
		ParentReference: ctx.Query("parent"),
		Cursor:          ctx.Query("cursor"),
		Limit:           limit,
		Depth:           dto.CatalogDepth(ctx.Query("depth")),
	}))
	Respond(ctx, httpmapper.ToCatalogResponse(result), err)
}

func (handler *CatalogHandler) tableRows(ctx *gin.Context) {
	var request httpmapper.TableRowsRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.TableRows(ctx, ctx.Param("id"), httpmapper.ToTableRowsInput(request))
	Respond(ctx, httpmapper.ToTableRowsResponse(result), err)
}

func (handler *CatalogHandler) tableSchema(ctx *gin.Context) {
	result, err := handler.service.TableSchema(ctx, ctx.Param("id"), ctx.Query("reference"))
	Respond(ctx, httpmapper.ToTableSchemaResponse(result), err)
}

func (handler *CatalogHandler) tableDDL(ctx *gin.Context) {
	result, err := handler.service.TableDDL(ctx, ctx.Param("id"), ctx.Query("reference"))
	Respond(ctx, httpmapper.TableDDLResponse{DDL: result}, err)
}

func catalogLimit(value string) (int, error) {
	if value == "" {
		return 100, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > 500 {
		return 0, apperror.NewValidation("catalog limit must be between 1 and 500", err)
	}
	return parsed, nil
}
