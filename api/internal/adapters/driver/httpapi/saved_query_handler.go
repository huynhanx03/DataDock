package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	httpmapper "github.com/huynhanx03/datadock/internal/adapters/driver/httpapi/mapper"
	"github.com/huynhanx03/datadock/internal/core/service"
)

type SavedQueryHandler struct {
	service *service.SavedQueryService
}

func NewSavedQueryHandler(savedQueryService *service.SavedQueryService) *SavedQueryHandler {
	return &SavedQueryHandler{service: savedQueryService}
}

func (handler *SavedQueryHandler) Register(router *gin.RouterGroup) {
	router.GET("/saved-queries", handler.list)
	router.POST("/saved-queries", handler.create)
	router.GET("/saved-queries/:id", handler.get)
	router.PATCH("/saved-queries/:id", handler.update)
	router.DELETE("/saved-queries/:id", handler.delete)
	router.POST("/saved-queries/:id/duplicate", handler.duplicate)
	router.PATCH("/saved-queries/:id/favorite", handler.favorite)
	router.GET("/saved-queries/:id/share", handler.getShare)
	router.POST("/saved-queries/:id/share", handler.enableShare)
	router.DELETE("/saved-queries/:id/share", handler.revokeShare)
	router.GET("/shared-queries/:code", handler.publicShare)
	router.GET("/saved-query-folders", handler.listFolders)
	router.POST("/saved-query-folders", handler.createFolder)
	router.PATCH("/saved-query-folders/:id", handler.updateFolder)
	router.DELETE("/saved-query-folders/:id", handler.deleteFolder)
}

func (handler *SavedQueryHandler) list(ctx *gin.Context) {
	result, err := handler.service.List(ctx)
	Respond(ctx, httpmapper.ToSavedQueryResponses(result), err)
}

func (handler *SavedQueryHandler) create(ctx *gin.Context) {
	var request httpmapper.SavedQueryCreateRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.Create(ctx, httpmapper.ToSavedQueryCreateInput(request))
	RespondCreated(ctx, httpmapper.ToSavedQueryResponse(result), err)
}

func (handler *SavedQueryHandler) get(ctx *gin.Context) {
	result, err := handler.service.Get(ctx, ctx.Param("id"))
	Respond(ctx, httpmapper.ToSavedQueryResponse(result), err)
}

func (handler *SavedQueryHandler) update(ctx *gin.Context) {
	var request httpmapper.SavedQueryUpdateRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.Update(ctx, ctx.Param("id"), httpmapper.ToSavedQueryUpdateInput(request))
	Respond(ctx, httpmapper.ToSavedQueryResponse(result), err)
}

func (handler *SavedQueryHandler) delete(ctx *gin.Context) {
	if err := handler.service.Delete(ctx, ctx.Param("id")); err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (handler *SavedQueryHandler) duplicate(ctx *gin.Context) {
	result, err := handler.service.Duplicate(ctx, ctx.Param("id"))
	RespondCreated(ctx, httpmapper.ToSavedQueryResponse(result), err)
}

func (handler *SavedQueryHandler) favorite(ctx *gin.Context) {
	var request httpmapper.SavedQueryFavoriteRequest
	if !BindJSON(ctx, &request) {
		return
	}
	if request.Favorite == nil {
		BadRequest(ctx)
		return
	}
	result, err := handler.service.SetFavorite(ctx, ctx.Param("id"), *request.Favorite)
	Respond(ctx, httpmapper.ToSavedQueryResponse(result), err)
}

func (handler *SavedQueryHandler) enableShare(ctx *gin.Context) {
	var request httpmapper.SavedQueryShareRequest
	if !BindOptionalJSON(ctx, &request) {
		return
	}
	result, err := handler.service.EnableShare(ctx, ctx.Param("id"), httpmapper.ToSavedQueryShareInput(request))
	RespondCreated(ctx, httpmapper.ToSavedQueryShareResponse(result), err)
}

func (handler *SavedQueryHandler) getShare(ctx *gin.Context) {
	result, err := handler.service.GetShare(ctx, ctx.Param("id"))
	Respond(ctx, httpmapper.ToSavedQueryShareResponse(result), err)
}

func (handler *SavedQueryHandler) revokeShare(ctx *gin.Context) {
	if err := handler.service.RevokeShare(ctx, ctx.Param("id")); err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (handler *SavedQueryHandler) publicShare(ctx *gin.Context) {
	result, err := handler.service.PublicShare(ctx, ctx.Param("code"))
	Respond(ctx, httpmapper.ToPublicSavedQueryResponse(result), err)
}

func (handler *SavedQueryHandler) listFolders(ctx *gin.Context) {
	result, err := handler.service.ListFolders(ctx)
	Respond(ctx, httpmapper.ToSavedQueryFolderResponses(result), err)
}

func (handler *SavedQueryHandler) createFolder(ctx *gin.Context) {
	var request httpmapper.SavedQueryFolderCreateRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.CreateFolder(ctx, httpmapper.ToSavedQueryFolderCreateInput(request))
	RespondCreated(ctx, httpmapper.ToSavedQueryFolderResponse(result), err)
}

func (handler *SavedQueryHandler) updateFolder(ctx *gin.Context) {
	var request httpmapper.SavedQueryFolderUpdateRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.UpdateFolder(ctx, ctx.Param("id"), httpmapper.ToSavedQueryFolderUpdateInput(request))
	Respond(ctx, httpmapper.ToSavedQueryFolderResponse(result), err)
}

func (handler *SavedQueryHandler) deleteFolder(ctx *gin.Context) {
	var request httpmapper.SavedQueryFolderDeleteRequest
	if !BindOptionalJSON(ctx, &request) {
		return
	}
	if err := handler.service.DeleteFolder(ctx, ctx.Param("id"), httpmapper.ToSavedQueryFolderDeleteInput(request)); err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
