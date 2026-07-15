package di

import (
	"database/sql"
	"errors"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/datadock/config"
	"github.com/huynhanx03/datadock/internal/adapters/driven/engines"
	"github.com/huynhanx03/datadock/internal/adapters/driven/security"
	"github.com/huynhanx03/datadock/internal/adapters/driven/sqlite"
	"github.com/huynhanx03/datadock/internal/adapters/driver/httpapi"
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
	manager      *engines.Manager
	catalog      *service.CatalogService
	queries      *service.QueryService
	tables       *service.TableService
	transactions *service.TransactionService
	schema       *service.SchemaService
	operations   *service.OperationsService
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
	manager := engines.NewManagerWithOptions(engines.ManagerOptions{
		MaxConcurrentQueries: cfg.MaxConcurrentQueries,
		TransactionIdleTTL:   cfg.TransactionTTL,
		ShutdownTimeout:      cfg.ShutdownTimeout,
	})
	connectionRepository := sqlite.NewConnectionRepository(database)
	profileResolver := service.NewConnectionProfileResolver(connectionRepository, cipher)
	queryHistoryRepository := sqlite.NewQueryHistoryRepository(database)
	savedQueryRepository := sqlite.NewSavedQueryRepository(database)
	savedQueryFolderRepository := sqlite.NewSavedQueryFolderRepository(database)
	sharedQueryRepository := sqlite.NewSharedQueryRepository(database)
	return &Container{
		config:   cfg,
		database: database,
		logger: commonlogger.NewLogger(commonlogger.LoggerConfig{
			Mode:    mode,
			Level:   cfg.LogLevel,
			Service: "datadock-api",
			Env:     mode,
		}),
		workspaces: service.NewWorkspaceService(sqlite.NewWorkspaceRepository(database)),
		connections: service.NewConnectionServiceWithDefaults(connectionRepository, cipher, manager, service.ConnectionProfileDefaults{
			MaxOpenConns:    cfg.DefaultPool.MaxOpenConnections,
			MaxIdleConns:    cfg.DefaultPool.MaxIdleConnections,
			ConnMaxLifetime: int(cfg.DefaultPool.MaxLifetime / time.Second),
			ConnMaxIdleTime: int(cfg.DefaultPool.MaxIdleTime / time.Second),
		}),
		manager: manager,
		catalog: service.NewCatalogService(profileResolver, manager, manager),
		queries: service.NewQueryService(profileResolver, manager, queryHistoryRepository, service.QueryServiceOptions{
			MaxRows:        cfg.MaxQueryRows,
			MaxBytes:       cfg.MaxQueryBytes,
			DefaultTimeout: 30 * time.Second,
			MaxTimeout:     60 * time.Second,
			HistoryTimeout: 2 * time.Second,
		}),
		tables:       service.NewTableService(profileResolver, manager),
		transactions: service.NewTransactionService(profileResolver, manager),
		schema:       service.NewSchemaService(profileResolver, manager),
		operations:   service.NewOperationsService(profileResolver, manager),
		savedQueries: service.NewSavedQueryService(savedQueryRepository, savedQueryFolderRepository, sharedQueryRepository),
	}, nil
}

func (c *Container) Router() *gin.Engine {
	return httpapi.NewRouter(httpapi.RouterOptions{
		Production:   c.config.IsProduction(),
		CORSOrigins:  c.config.CORSOrigins,
		MaxBodyBytes: c.config.MaxRequestBytes,
		Logger:       c.logger.Logger,
		Metadata:     c.database,
	}, httpapi.RouterServices{
		Workspaces:   c.workspaces,
		Connections:  c.connections,
		Catalog:      c.catalog,
		Queries:      c.queries,
		SavedQueries: c.savedQueries,
		Tables:       c.tables,
		Transactions: c.transactions,
		Schema:       c.schema,
		Operations:   c.operations,
	})
}

func (c *Container) Close() error {
	return errors.Join(c.manager.Close(), c.database.Close(), c.logger.Sync())
}
