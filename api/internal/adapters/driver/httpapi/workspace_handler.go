package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	httpmapper "github.com/huynhanx03/datadock/internal/adapters/driver/httpapi/mapper"
	"github.com/huynhanx03/datadock/internal/core/service"
)

type WorkspaceHandler struct {
	service *service.WorkspaceService
}

func NewWorkspaceHandler(workspaceService *service.WorkspaceService) *WorkspaceHandler {
	return &WorkspaceHandler{service: workspaceService}
}

func (handler *WorkspaceHandler) Register(router *gin.RouterGroup) {
	router.GET("/workspaces", handler.list)
	router.POST("/workspaces", handler.create)
	router.PATCH("/workspaces/:id", handler.update)
	router.DELETE("/workspaces/:id", handler.delete)
	router.PUT("/workspaces/order", handler.reorder)
}

func (handler *WorkspaceHandler) list(ctx *gin.Context) {
	result, err := handler.service.List(ctx)
	Respond(ctx, httpmapper.ToWorkspaceResponses(result), err)
}

func (handler *WorkspaceHandler) create(ctx *gin.Context) {
	var request httpmapper.WorkspaceCreateRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.Create(ctx, httpmapper.ToWorkspaceCreateInput(request))
	RespondCreated(ctx, httpmapper.ToWorkspaceResponse(result), err)
}

func (handler *WorkspaceHandler) update(ctx *gin.Context) {
	var request httpmapper.WorkspaceUpdateRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.Update(ctx, ctx.Param("id"), httpmapper.ToWorkspaceUpdateInput(request))
	Respond(ctx, httpmapper.ToWorkspaceResponse(result), err)
}

func (handler *WorkspaceHandler) delete(ctx *gin.Context) {
	if err := handler.service.Delete(ctx, ctx.Param("id")); err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (handler *WorkspaceHandler) reorder(ctx *gin.Context) {
	var request httpmapper.WorkspaceOrderRequest
	if !BindJSON(ctx, &request) {
		return
	}
	if err := handler.service.Reorder(ctx, httpmapper.ToWorkspaceOrderInput(request).IDs); err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
