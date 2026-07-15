package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	httpmapper "github.com/huynhanx03/datadock/internal/adapters/driver/httpapi/mapper"
	"github.com/huynhanx03/datadock/internal/core/service"
)

type ConnectionHandler struct {
	service *service.ConnectionService
}

func NewConnectionHandler(connectionService *service.ConnectionService) *ConnectionHandler {
	return &ConnectionHandler{service: connectionService}
}

func (handler *ConnectionHandler) Register(router *gin.RouterGroup) {
	router.GET("/connections", handler.list)
	router.POST("/connections", handler.create)
	router.POST("/connections/test", handler.testDraft)
	router.GET("/connections/:id", handler.get)
	router.PATCH("/connections/:id", handler.update)
	router.DELETE("/connections/:id", handler.delete)
	router.POST("/connections/:id/duplicate", handler.duplicate)
	router.POST("/connections/:id/test", handler.testSaved)
	router.POST("/connections/:id/connect", handler.connect)
	router.POST("/connections/:id/disconnect", handler.disconnect)
	router.GET("/connections/:id/status", handler.status)
	router.PATCH("/connections/:id/favorite", handler.favorite)
}

func (handler *ConnectionHandler) list(ctx *gin.Context) {
	result, err := handler.service.List(ctx, ctx.Query("workspaceId"))
	Respond(ctx, httpmapper.ToConnectionResponses(result), err)
}

func (handler *ConnectionHandler) create(ctx *gin.Context) {
	var request httpmapper.ConnectionCreateRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.Create(ctx, httpmapper.ToConnectionCreateInput(request))
	RespondCreated(ctx, httpmapper.ToConnectionResponse(result), err)
}

func (handler *ConnectionHandler) testDraft(ctx *gin.Context) {
	var request httpmapper.ConnectionCreateRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.TestDraft(ctx, httpmapper.ToConnectionCreateInput(request))
	Respond(ctx, httpmapper.ToConnectionTestResponse(result), err)
}

func (handler *ConnectionHandler) get(ctx *gin.Context) {
	result, err := handler.service.Get(ctx, ctx.Param("id"))
	Respond(ctx, httpmapper.ToConnectionResponse(result), err)
}

func (handler *ConnectionHandler) update(ctx *gin.Context) {
	var request httpmapper.ConnectionPatchRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.Patch(ctx, ctx.Param("id"), httpmapper.ToConnectionPatchInput(request))
	Respond(ctx, httpmapper.ToConnectionResponse(result), err)
}

func (handler *ConnectionHandler) delete(ctx *gin.Context) {
	if err := handler.service.Delete(ctx, ctx.Param("id")); err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (handler *ConnectionHandler) duplicate(ctx *gin.Context) {
	result, err := handler.service.Duplicate(ctx, ctx.Param("id"))
	RespondCreated(ctx, httpmapper.ToConnectionResponse(result), err)
}

func (handler *ConnectionHandler) testSaved(ctx *gin.Context) {
	result, err := handler.service.TestSaved(ctx, ctx.Param("id"))
	Respond(ctx, httpmapper.ToConnectionTestResponse(result), err)
}

func (handler *ConnectionHandler) connect(ctx *gin.Context) {
	result, err := handler.service.Connect(ctx, ctx.Param("id"))
	Respond(ctx, httpmapper.ToConnectionResponse(result), err)
}

func (handler *ConnectionHandler) disconnect(ctx *gin.Context) {
	result, err := handler.service.Disconnect(ctx, ctx.Param("id"))
	Respond(ctx, httpmapper.ToConnectionResponse(result), err)
}

func (handler *ConnectionHandler) status(ctx *gin.Context) {
	result, err := handler.service.Status(ctx, ctx.Param("id"))
	Respond(ctx, httpmapper.ToConnectionStatusResponse(result), err)
}

func (handler *ConnectionHandler) favorite(ctx *gin.Context) {
	var request httpmapper.ConnectionFavoriteRequest
	if !BindJSON(ctx, &request) {
		return
	}
	input := httpmapper.ToConnectionFavoriteInput(request)
	if input.Favorite == nil {
		BadRequest(ctx)
		return
	}
	result, err := handler.service.SetFavorite(ctx, ctx.Param("id"), *input.Favorite)
	Respond(ctx, httpmapper.ToConnectionResponse(result), err)
}
