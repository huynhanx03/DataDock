package di

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/huynhanx03/datadock/internal/adapters/driven/sqlite"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/service"
)

func (c *Container) registerWorkspaceRoutes(router *gin.RouterGroup) {
	router.GET("/workspaces", func(ctx *gin.Context) {
		workspaces, err := c.workspaces.List(ctx)
		respond(ctx, workspaces, err)
	})
	router.POST("/workspaces", func(ctx *gin.Context) {
		var input dto.CreateWorkspaceInput
		if err := ctx.ShouldBindJSON(&input); err != nil {
			badRequest(ctx, err)
			return
		}
		workspace, err := c.workspaces.Create(ctx, input)
		respondCreated(ctx, workspace, err)
	})
	router.PATCH("/workspaces/:id", func(ctx *gin.Context) {
		var input dto.UpdateWorkspaceInput
		if err := ctx.ShouldBindJSON(&input); err != nil {
			badRequest(ctx, err)
			return
		}
		workspace, err := c.workspaces.Update(ctx, ctx.Param("id"), input)
		respond(ctx, workspace, err)
	})
	router.DELETE("/workspaces/:id", func(ctx *gin.Context) {
		if err := c.workspaces.Delete(ctx, ctx.Param("id")); err != nil {
			respond(ctx, nil, err)
			return
		}
		ctx.Status(http.StatusNoContent)
	})
	router.PUT("/workspaces/order", func(ctx *gin.Context) {
		var input dto.ReorderWorkspaceInput
		if err := ctx.ShouldBindJSON(&input); err != nil {
			badRequest(ctx, err)
			return
		}
		if err := c.workspaces.Reorder(ctx, input.IDs); err != nil {
			respond(ctx, nil, err)
			return
		}
		ctx.Status(http.StatusNoContent)
	})
}

func (c *Container) registerConnectionRoutes(router *gin.RouterGroup) {
	router.GET("/connections", func(ctx *gin.Context) {
		connections, err := c.connections.List(ctx, ctx.Query("workspaceId"))
		respond(ctx, connections, err)
	})
	router.POST("/connections", func(ctx *gin.Context) {
		var input dto.ConnectionInput
		if err := ctx.ShouldBindJSON(&input); err != nil {
			badRequest(ctx, err)
			return
		}
		connection, err := c.connections.Create(ctx, input)
		respondCreated(ctx, connection, err)
	})
	router.GET("/connections/:id", func(ctx *gin.Context) {
		connection, err := c.connections.Get(ctx, ctx.Param("id"))
		respond(ctx, connection, err)
	})
	router.PATCH("/connections/:id", func(ctx *gin.Context) {
		var input dto.ConnectionInput
		if err := ctx.ShouldBindJSON(&input); err != nil {
			badRequest(ctx, err)
			return
		}
		connection, err := c.connections.Update(ctx, ctx.Param("id"), input)
		respond(ctx, connection, err)
	})
	router.POST("/connections/:id/duplicate", func(ctx *gin.Context) {
		connection, err := c.connections.Duplicate(ctx, ctx.Param("id"))
		respondCreated(ctx, connection, err)
	})
	router.POST("/connections/:id/test", func(ctx *gin.Context) {
		if err := c.connections.Test(ctx, ctx.Param("id")); err != nil {
			respond(ctx, nil, err)
			return
		}
		ctx.JSON(http.StatusOK, gin.H{"data": gin.H{"ok": true}})
	})
	router.GET("/connections/:id/dashboard", func(ctx *gin.Context) {
		result, err := c.databases.Dashboard(ctx, ctx.Param("id"))
		respond(ctx, result, err)
	})
	router.GET("/connections/:id/sessions", func(ctx *gin.Context) {
		result, err := c.databases.Sessions(ctx, ctx.Param("id"))
		respond(ctx, result, err)
	})
	router.POST("/connections/:id/sessions/:sessionId/cancel", func(ctx *gin.Context) {
		var input dto.SessionActionInput
		_ = ctx.ShouldBindJSON(&input)
		input.ConnectionID = ctx.Param("id")
		input.SessionID = ctx.Param("sessionId")
		if err := c.databases.CancelSession(ctx, input); err != nil {
			respond(ctx, nil, err)
			return
		}
		ctx.Status(http.StatusNoContent)
	})
	router.GET("/connections/:id/locks", func(ctx *gin.Context) {
		result, err := c.databases.Locks(ctx, ctx.Param("id"))
		respond(ctx, result, err)
	})
	router.GET("/connections/:id/performance", func(ctx *gin.Context) {
		result, err := c.databases.Performance(ctx, ctx.Param("id"))
		respond(ctx, result, err)
	})
	router.GET("/connections/:id/tables", func(ctx *gin.Context) {
		tables, err := c.databases.ListTables(ctx, ctx.Param("id"))
		respond(ctx, tables, err)
	})
	router.GET("/connections/:id/tables/:table/rows", func(ctx *gin.Context) {
		limit, err := positiveQueryInt(ctx, "limit", 100, 200)
		if err != nil {
			badRequest(ctx, err)
			return
		}
		offset, err := nonNegativeQueryInt(ctx, "offset", 0)
		if err != nil {
			badRequest(ctx, err)
			return
		}
		result, err := c.databases.BrowseTable(ctx, dto.TableRowsInput{ConnectionID: ctx.Param("id"), Table: ctx.Param("table"), Limit: limit, Offset: offset, Search: ctx.Query("search"), Sort: ctx.Query("sort"), Order: ctx.Query("order")})
		respond(ctx, result, err)
	})
	router.GET("/connections/:id/tables/:table/ddl", func(ctx *gin.Context) {
		ddl, err := c.databases.TableDDL(ctx, ctx.Param("id"), ctx.Param("table"))
		respond(ctx, gin.H{"ddl": ddl}, err)
	})
	router.POST("/connections/:id/tables/:table/rows/mutate", func(ctx *gin.Context) {
		var input dto.TableMutateInput
		if err := ctx.ShouldBindJSON(&input); err != nil {
			badRequest(ctx, err)
			return
		}
		result, err := c.databases.MutateRows(ctx, ctx.Param("id"), ctx.Param("table"), input.Mutations)
		respond(ctx, result, err)
	})
	router.GET("/connections/:id/tables/:table/schema", func(ctx *gin.Context) {
		schema, err := c.databases.TableSchema(ctx, ctx.Param("id"), ctx.Param("table"))
		respond(ctx, schema, err)
	})
	router.POST("/connections/:id/schema/tables", func(ctx *gin.Context) {
		var input dto.CreateTableInput
		if err := ctx.ShouldBindJSON(&input); err != nil {
			badRequest(ctx, err)
			return
		}
		if err := c.databases.CreateTable(ctx, ctx.Param("id"), input); err != nil {
			respond(ctx, nil, err)
			return
		}
		ctx.Status(http.StatusCreated)
	})
	router.PATCH("/connections/:id/tables/:table/schema", func(ctx *gin.Context) {
		var input dto.AlterTableInput
		if err := ctx.ShouldBindJSON(&input); err != nil {
			badRequest(ctx, err)
			return
		}
		if err := c.databases.AlterTable(ctx, ctx.Param("id"), ctx.Param("table"), input); err != nil {
			respond(ctx, nil, err)
			return
		}
		ctx.Status(http.StatusNoContent)
	})
	router.POST("/connections/:id/tables/:table/indexes", func(ctx *gin.Context) {
		var input dto.CreateIndexInput
		if err := ctx.ShouldBindJSON(&input); err != nil {
			badRequest(ctx, err)
			return
		}
		if err := c.databases.CreateIndex(ctx, ctx.Param("id"), ctx.Param("table"), input); err != nil {
			respond(ctx, nil, err)
			return
		}
		ctx.Status(http.StatusCreated)
	})
	router.DELETE("/connections/:id/tables/:table/indexes/:index", func(ctx *gin.Context) {
		if err := c.databases.DropIndex(ctx, ctx.Param("id"), ctx.Param("table"), ctx.Param("index")); err != nil {
			respond(ctx, nil, err)
			return
		}
		ctx.Status(http.StatusNoContent)
	})
	router.DELETE("/connections/:id", func(ctx *gin.Context) {
		if err := c.connections.Delete(ctx, ctx.Param("id")); err != nil {
			respond(ctx, nil, err)
			return
		}
		ctx.Status(http.StatusNoContent)
	})
}

func positiveQueryInt(ctx *gin.Context, name string, fallback, maximum int) (int, error) {
	value := ctx.Query(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > maximum {
		return 0, errors.New(name + " must be between 1 and " + strconv.Itoa(maximum))
	}
	return parsed, nil
}

func nonNegativeQueryInt(ctx *gin.Context, name string, fallback int) (int, error) {
	value := ctx.Query(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, errors.New(name + " must be a non-negative integer")
	}
	return parsed, nil
}

func (c *Container) registerQueryRoutes(router *gin.RouterGroup) {
	router.POST("/queries/execute", func(ctx *gin.Context) {
		var input dto.ExecuteQueryInput
		if err := ctx.ShouldBindJSON(&input); err != nil {
			badRequest(ctx, err)
			return
		}
		result, err := c.databases.Execute(ctx, input)
		respond(ctx, result, err)
	})
	router.POST("/transactions/begin", func(ctx *gin.Context) {
		var input dto.TransactionInput
		if err := ctx.ShouldBindJSON(&input); err != nil {
			badRequest(ctx, err)
			return
		}
		result, err := c.databases.BeginTransaction(ctx, input.ConnectionID)
		respondCreated(ctx, result, err)
	})
	router.POST("/transactions/:id/:action", func(ctx *gin.Context) {
		var input dto.TransactionInput
		_ = ctx.ShouldBindJSON(&input)
		result, err := c.databases.TransactionAction(ctx, ctx.Param("id"), ctx.Param("action"), input.Name)
		respond(ctx, result, err)
	})
	router.GET("/saved-queries", func(ctx *gin.Context) { result, err := c.savedQueries.List(ctx); respond(ctx, result, err) })
	router.POST("/saved-queries", func(ctx *gin.Context) {
		var input dto.SavedQueryInput
		if err := ctx.ShouldBindJSON(&input); err != nil {
			badRequest(ctx, err)
			return
		}
		result, err := c.savedQueries.Create(ctx, input)
		respondCreated(ctx, result, err)
	})
	router.DELETE("/saved-queries/:id", func(ctx *gin.Context) {
		if err := c.savedQueries.Delete(ctx, ctx.Param("id")); err != nil {
			respond(ctx, nil, err)
			return
		}
		ctx.Status(http.StatusNoContent)
	})
}

func respond(ctx *gin.Context, data any, err error) {
	if err == nil {
		ctx.JSON(http.StatusOK, gin.H{"data": data})
		return
	}
	status := http.StatusInternalServerError
	code := "internal_error"
	if errors.Is(err, sqlite.ErrNotFound) {
		status = http.StatusNotFound
		code = "not_found"
	} else if errors.Is(err, service.ErrWorkspaceNameRequired) || errors.Is(err, service.ErrConnectionNameRequired) || errors.Is(err, service.ErrConnectionHostRequired) || errors.Is(err, service.ErrUnsupportedEngine) || errors.Is(err, service.ErrQueryRequired) || errors.Is(err, service.ErrReadOnlyConnection) || errors.Is(err, service.ErrTableRequired) || errors.Is(err, service.ErrInvalidTableBrowse) || errors.Is(err, service.ErrInvalidSchemaInput) || errors.Is(err, service.ErrInvalidRowMutation) {
		status = http.StatusBadRequest
		code = "validation_error"
	}
	ctx.JSON(status, gin.H{"error": gin.H{"code": code, "message": err.Error()}})
}

func respondCreated(ctx *gin.Context, data any, err error) {
	if err != nil {
		respond(ctx, nil, err)
		return
	}
	ctx.JSON(http.StatusCreated, gin.H{"data": data})
}

func badRequest(ctx *gin.Context, err error) {
	ctx.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "validation_error", "message": err.Error()}})
}
