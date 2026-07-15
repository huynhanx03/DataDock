package httpapi

import (
	"github.com/gin-gonic/gin"

	httpmapper "github.com/huynhanx03/datadock/internal/adapters/driver/httpapi/mapper"
	"github.com/huynhanx03/datadock/internal/core/service"
)

type SchemaHandler struct {
	service *service.SchemaService
}

func NewSchemaHandler(schemaService *service.SchemaService) *SchemaHandler {
	return &SchemaHandler{service: schemaService}
}

func (handler *SchemaHandler) Register(router *gin.RouterGroup) {
	router.POST("/connections/:id/schema/preview", handler.preview)
	router.POST("/connections/:id/schema/apply", handler.apply)
}

func (handler *SchemaHandler) preview(ctx *gin.Context) {
	var request httpmapper.SchemaPreviewRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.Preview(ctx, ctx.Param("id"), httpmapper.ToSchemaPreviewInput(request))
	Respond(ctx, httpmapper.ToSchemaPreviewResponse(result), err)
}

func (handler *SchemaHandler) apply(ctx *gin.Context) {
	var request httpmapper.SchemaApplyRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.Apply(ctx, ctx.Param("id"), httpmapper.ToSchemaApplyInput(request))
	Respond(ctx, httpmapper.ToSchemaApplyResponse(result), err)
}
