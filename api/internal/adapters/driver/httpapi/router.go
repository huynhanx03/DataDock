package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/core/service"
)

type RouterOptions struct {
	Production   bool
	CORSOrigins  []string
	MaxBodyBytes int64
	Logger       *zap.Logger
	Metadata     *sql.DB
}

type RouterServices struct {
	Workspaces   *service.WorkspaceService
	Connections  *service.ConnectionService
	Catalog      *service.CatalogService
	Queries      *service.QueryService
	SavedQueries *service.SavedQueryService
	Tables       *service.TableService
	Transactions *service.TransactionService
	Schema       *service.SchemaService
	Operations   *service.OperationsService
}

func NewRouter(options RouterOptions, services RouterServices) *gin.Engine {
	if options.Production {
		gin.SetMode(gin.ReleaseMode)
	}
	logger := options.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	router := gin.New()
	router.HandleMethodNotAllowed = true
	_ = router.SetTrustedProxies(nil)
	router.Use(CorrelationID(logger), RequestLogger(logger), Recovery(), CORS(options.CORSOrigins), BodyLimit(options.MaxBodyBytes))
	router.GET("/healthz", healthHandler(options.Metadata))
	api := router.Group("/api/v1")
	api.GET("/status", statusHandler)
	api.GET("/engines", enginesHandler)
	NewWorkspaceHandler(services.Workspaces).Register(api)
	NewConnectionHandler(services.Connections).Register(api)
	NewCatalogHandler(services.Catalog).Register(api)
	NewQueryHandler(services.Queries).Register(api)
	NewSavedQueryHandler(services.SavedQueries).Register(api)
	NewTableHandler(services.Tables).Register(api)
	NewTransactionHandler(services.Transactions).Register(api)
	NewSchemaHandler(services.Schema).Register(api)
	NewOperationsHandler(services.Operations).Register(api)
	router.NoRoute(func(ctx *gin.Context) {
		WriteError(ctx, apperror.NewNotFound("Route not found", nil))
	})
	router.NoMethod(func(ctx *gin.Context) {
		WriteError(ctx, apperror.NewMethodNotAllowed("Method not allowed", nil))
	})
	return router
}

func healthHandler(database *sql.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if database == nil {
			WriteError(ctx, apperror.NewServiceUnavailable("Metadata database is unavailable", nil))
			return
		}
		checkContext, cancel := context.WithTimeout(ctx.Request.Context(), 2*time.Second)
		defer cancel()
		if err := database.PingContext(checkContext); err != nil {
			WriteError(ctx, apperror.NewServiceUnavailable("Metadata database is unavailable", err))
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}

func statusHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"data": gin.H{"service": "datadock-api", "version": "v1"}})
}

func enginesHandler(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"data": entity.EngineCapabilities()})
}
