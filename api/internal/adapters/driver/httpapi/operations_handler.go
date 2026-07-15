package httpapi

import (
	"github.com/gin-gonic/gin"

	httpmapper "github.com/huynhanx03/datadock/internal/adapters/driver/httpapi/mapper"
	"github.com/huynhanx03/datadock/internal/core/service"
)

type OperationsHandler struct {
	service *service.OperationsService
}

func NewOperationsHandler(operationsService *service.OperationsService) *OperationsHandler {
	return &OperationsHandler{service: operationsService}
}

func (handler *OperationsHandler) Register(router *gin.RouterGroup) {
	router.GET("/connections/:id/operations/dashboard", handler.dashboard)
	router.GET("/connections/:id/operations/sessions", handler.sessions)
	router.POST("/connections/:id/operations/sessions/:sessionId/control", handler.controlSession)
	router.GET("/connections/:id/operations/locks", handler.locks)
	router.GET("/connections/:id/operations/performance", handler.performance)
}

func (handler *OperationsHandler) dashboard(ctx *gin.Context) {
	var query httpmapper.OperationsRangeQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		BadRequest(ctx)
		return
	}
	result, err := handler.service.Dashboard(ctx, ctx.Param("id"), httpmapper.ToOperationsRangeInput(query))
	Respond(ctx, httpmapper.ToOperationsDashboardResponse(result), err)
}

func (handler *OperationsHandler) sessions(ctx *gin.Context) {
	var query httpmapper.OperationsSessionsQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		BadRequest(ctx)
		return
	}
	result, err := handler.service.Sessions(ctx, ctx.Param("id"), httpmapper.ToOperationsSessionsInput(query))
	Respond(ctx, httpmapper.ToOperationsSessionsResponse(result), err)
}

func (handler *OperationsHandler) controlSession(ctx *gin.Context) {
	var request httpmapper.OperationsSessionControlRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.ControlSession(ctx, ctx.Param("id"), httpmapper.ToOperationsSessionControlInput(ctx.Param("sessionId"), request))
	Respond(ctx, httpmapper.ToOperationsSessionControlResponse(result), err)
}

func (handler *OperationsHandler) locks(ctx *gin.Context) {
	var query httpmapper.OperationsLocksQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		BadRequest(ctx)
		return
	}
	result, err := handler.service.Locks(ctx, ctx.Param("id"), httpmapper.ToOperationsLocksInput(query))
	Respond(ctx, httpmapper.ToOperationsLocksResponse(result), err)
}

func (handler *OperationsHandler) performance(ctx *gin.Context) {
	var query httpmapper.OperationsPerformanceQuery
	if err := ctx.ShouldBindQuery(&query); err != nil {
		BadRequest(ctx)
		return
	}
	result, err := handler.service.Performance(ctx, ctx.Param("id"), httpmapper.ToOperationsPerformanceInput(query))
	Respond(ctx, httpmapper.ToOperationsPerformanceResponse(result), err)
}
