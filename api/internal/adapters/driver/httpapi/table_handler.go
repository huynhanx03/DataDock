package httpapi

import (
	"github.com/gin-gonic/gin"

	httpmapper "github.com/huynhanx03/datadock/internal/adapters/driver/httpapi/mapper"
	"github.com/huynhanx03/datadock/internal/core/service"
)

type TableHandler struct {
	service *service.TableService
}

func NewTableHandler(tableService *service.TableService) *TableHandler {
	return &TableHandler{service: tableService}
}

func (handler *TableHandler) Register(router *gin.RouterGroup) {
	router.POST("/connections/:id/table/mutations", handler.apply)
}

func (handler *TableHandler) apply(ctx *gin.Context) {
	var request httpmapper.TableMutationRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.Apply(ctx, httpmapper.ToTableMutationInput(ctx.Param("id"), request))
	Respond(ctx, httpmapper.ToTableMutationResponse(result), err)
}
