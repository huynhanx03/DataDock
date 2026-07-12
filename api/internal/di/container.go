package di

import (
	"database/sql"
	"errors"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/huynhanx03/datadock/config"
	"github.com/huynhanx03/datadock/internal/adapters/driven/engines"
	"github.com/huynhanx03/datadock/internal/adapters/driven/security"
	"github.com/huynhanx03/datadock/internal/adapters/driven/sqlite"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/core/service"
	commonlogger "github.com/huynhanx03/go-common/pkg/logger"
	commonsettings "github.com/huynhanx03/go-common/pkg/settings"
)

type Container struct {
	config       config.Config
	logger       *commonlogger.LoggerZap
	database     *sql.DB
	workspaces   *service.WorkspaceService
	connections  *service.ConnectionService
	databases    *service.DatabaseService
	savedQueries *service.SavedQueryService
}

func New(cfg config.Config) (*Container, error) {
	if cfg.EncryptionKey == "" {
		return nil, errors.New("DATADOCK_ENCRYPTION_KEY is required")
	}
	databasePath := cfg.DataPath
	if filepath.Ext(databasePath) != ".db" {
		databasePath = filepath.Join(databasePath, "datadock.db")
	}
	database, err := sqlite.Open(databasePath)
	if err != nil {
		return nil, err
	}
	cipher, err := security.NewAESGCMCipher(cfg.EncryptionKey)
	if err != nil {
		database.Close()
		return nil, err
	}
	mode := commonsettings.EnvDev
	if cfg.IsProduction() {
		mode = commonsettings.EnvProd
	}
	runtime := engines.NewRuntime()
	return &Container{
		config:   cfg,
		database: database,
		logger: commonlogger.NewLogger(commonlogger.LoggerConfig{
			Mode:    mode,
			Service: "datadock-api",
			Env:     mode,
		}),
		workspaces:   service.NewWorkspaceService(sqlite.NewWorkspaceRepository(database)),
		connections:  service.NewConnectionService(sqlite.NewConnectionRepository(database), cipher, runtime),
		databases:    service.NewDatabaseService(sqlite.NewConnectionRepository(database), cipher, runtime),
		savedQueries: service.NewSavedQueryService(database),
	}, nil
}

func (c *Container) Router() *gin.Engine {
	if c.config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.New()
	router.Use(c.recovery(), c.requestLogger(), c.cors())
	router.GET("/healthz", c.healthz)
	api := router.Group("/api/v1")
	api.GET("/status", c.status)
	api.GET("/engines", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"data": entity.EngineCapabilities()})
	})
	c.registerWorkspaceRoutes(api)
	c.registerConnectionRoutes(api)
	c.registerQueryRoutes(api)
	return router
}

func (c *Container) Close() error {
	databaseError := c.database.Close()
	loggerError := c.logger.Sync()
	if databaseError != nil {
		return databaseError
	}
	return loggerError
}

func (c *Container) healthz(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (c *Container) status(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"data": gin.H{"service": "datadock-api", "version": "v1"}})
}

func (c *Container) recovery() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				c.logger.Error("request panic", zap.Any("panic", recovered), zap.String("path", ctx.Request.URL.Path))
				ctx.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "internal_error", "message": "An unexpected error occurred"}})
			}
		}()
		ctx.Next()
	}
}

func (c *Container) requestLogger() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		started := time.Now()
		ctx.Next()
		c.logger.Info("http request",
			zap.String("method", ctx.Request.Method),
			zap.String("path", ctx.Request.URL.Path),
			zap.Int("status", ctx.Writer.Status()),
			zap.Duration("duration", time.Since(started)),
		)
	}
}

func (c *Container) cors() gin.HandlerFunc {
	allowedOrigins := make(map[string]struct{}, len(c.config.CORSOrigins))
	for _, origin := range c.config.CORSOrigins {
		allowedOrigins[origin] = struct{}{}
	}
	return func(ctx *gin.Context) {
		origin := ctx.GetHeader("Origin")
		if _, ok := allowedOrigins[origin]; ok {
			ctx.Header("Access-Control-Allow-Origin", origin)
			ctx.Header("Vary", "Origin")
			ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			ctx.Header("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
		}
		if ctx.Request.Method == http.MethodOptions {
			if _, ok := allowedOrigins[origin]; !ok && origin != "" {
				ctx.AbortWithStatus(http.StatusForbidden)
				return
			}
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}
		ctx.Next()
	}
}
