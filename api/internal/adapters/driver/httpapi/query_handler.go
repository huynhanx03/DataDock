package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	httpmapper "github.com/huynhanx03/datadock/internal/adapters/driver/httpapi/mapper"
	"github.com/huynhanx03/datadock/internal/core/service"
)

type QueryHandler struct {
	service *service.QueryService
}

func NewQueryHandler(queryService *service.QueryService) *QueryHandler {
	return &QueryHandler{service: queryService}
}

func (handler *QueryHandler) Register(router *gin.RouterGroup) {
	router.POST("/queries/execute", handler.execute)
	router.POST("/queries/explain", handler.explain)
	router.POST("/queries/:executionId/cancel", handler.cancel)
	router.GET("/query-history", handler.history)
	router.DELETE("/query-history", handler.clearHistory)
	router.POST("/query-history/delete", handler.deleteHistory)
}

func (handler *QueryHandler) execute(ctx *gin.Context) {
	var request httpmapper.QueryExecuteRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.Execute(ctx, httpmapper.ToQueryExecutionInput(request))
	Respond(ctx, httpmapper.ToQueryExecutionResponse(result), err)
}

func (handler *QueryHandler) explain(ctx *gin.Context) {
	var request httpmapper.QueryExplainRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.Explain(ctx, httpmapper.ToQueryExplainInput(request))
	Respond(ctx, httpmapper.ToQueryExecutionResponse(result), err)
}

func (handler *QueryHandler) cancel(ctx *gin.Context) {
	if err := handler.service.Cancel(ctx, ctx.Param("executionId")); err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (handler *QueryHandler) history(ctx *gin.Context) {
	result, err := handler.service.ListHistory(ctx, ctx.Query("connectionId"))
	Respond(ctx, httpmapper.ToQueryHistoryResponses(result), err)
}

func (handler *QueryHandler) clearHistory(ctx *gin.Context) {
	if err := handler.service.ClearHistory(ctx, ctx.Query("connectionId")); err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

func (handler *QueryHandler) deleteHistory(ctx *gin.Context) {
	var request httpmapper.QueryHistoryDeleteRequest
	if !BindJSON(ctx, &request) {
		return
	}
	if err := handler.service.DeleteHistory(ctx, httpmapper.ToQueryHistoryDeleteInput(request)); err != nil {
		WriteError(ctx, err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
