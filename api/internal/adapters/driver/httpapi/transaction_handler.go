package httpapi

import (
	"github.com/gin-gonic/gin"

	httpmapper "github.com/huynhanx03/datadock/internal/adapters/driver/httpapi/mapper"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/service"
)

type TransactionHandler struct {
	service *service.TransactionService
}

func NewTransactionHandler(transactionService *service.TransactionService) *TransactionHandler {
	return &TransactionHandler{service: transactionService}
}

func (handler *TransactionHandler) Register(router *gin.RouterGroup) {
	router.POST("/connections/:id/transactions", handler.begin)
	router.GET("/connections/:id/transactions/:transactionId", handler.get)
	router.POST("/connections/:id/transactions/:transactionId/commit", handler.commit)
	router.POST("/connections/:id/transactions/:transactionId/rollback", handler.rollback)
	router.POST("/connections/:id/transactions/:transactionId/savepoints", handler.savepoint)
	router.POST("/connections/:id/transactions/:transactionId/savepoints/:name/rollback", handler.rollbackTo)
	router.DELETE("/connections/:id/transactions/:transactionId/savepoints/:name", handler.release)
}

func (handler *TransactionHandler) begin(ctx *gin.Context) {
	result, err := handler.service.Begin(ctx, httpmapper.ToTransactionBeginInput(ctx.Param("id")))
	RespondCreated(ctx, httpmapper.ToTransactionResponse(result), err)
}

func (handler *TransactionHandler) get(ctx *gin.Context) {
	result, err := handler.service.Get(ctx, handler.actionInput(ctx))
	Respond(ctx, httpmapper.ToTransactionResponse(result), err)
}

func (handler *TransactionHandler) commit(ctx *gin.Context) {
	result, err := handler.service.Commit(ctx, handler.actionInput(ctx))
	Respond(ctx, httpmapper.ToTransactionResponse(result), err)
}

func (handler *TransactionHandler) rollback(ctx *gin.Context) {
	result, err := handler.service.Rollback(ctx, handler.actionInput(ctx))
	Respond(ctx, httpmapper.ToTransactionResponse(result), err)
}

func (handler *TransactionHandler) savepoint(ctx *gin.Context) {
	var request httpmapper.TransactionSavepointRequest
	if !BindJSON(ctx, &request) {
		return
	}
	result, err := handler.service.Savepoint(ctx, httpmapper.ToTransactionSavepointInput(ctx.Param("id"), ctx.Param("transactionId"), request))
	RespondCreated(ctx, httpmapper.ToTransactionResponse(result), err)
}

func (handler *TransactionHandler) rollbackTo(ctx *gin.Context) {
	result, err := handler.service.RollbackTo(ctx, handler.pathSavepointInput(ctx))
	Respond(ctx, httpmapper.ToTransactionResponse(result), err)
}

func (handler *TransactionHandler) release(ctx *gin.Context) {
	result, err := handler.service.Release(ctx, handler.pathSavepointInput(ctx))
	Respond(ctx, httpmapper.ToTransactionResponse(result), err)
}

func (handler *TransactionHandler) actionInput(ctx *gin.Context) dto.TransactionActionInput {
	return httpmapper.ToTransactionActionInput(ctx.Param("id"), ctx.Param("transactionId"))
}

func (handler *TransactionHandler) pathSavepointInput(ctx *gin.Context) dto.TransactionSavepointInput {
	return httpmapper.ToTransactionSavepointInput(ctx.Param("id"), ctx.Param("transactionId"), httpmapper.TransactionSavepointRequest{Name: ctx.Param("name")})
}
