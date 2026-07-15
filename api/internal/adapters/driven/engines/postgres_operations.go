package engines

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
)

func postgresOperationsDashboard(ctx context.Context, database *sql.DB, input dto.OperationsRangeInput) (dto.OperationsDashboard, error) {
	collectedAt := time.Now().UTC()
	var version string
	var connections, committed, rolledBack, blocksRead, blocksHit, temporaryBytes, deadlocks, storageBytes, tables int64
	err := database.QueryRowContext(ctx, `
		SELECT current_setting('server_version'),
		       statistics.numbackends,
		       statistics.xact_commit,
		       statistics.xact_rollback,
		       statistics.blks_read,
		       statistics.blks_hit,
		       statistics.temp_bytes,
		       statistics.deadlocks,
		       pg_database_size(current_database()),
		       (SELECT COUNT(*) FROM information_schema.tables WHERE table_type = 'BASE TABLE' AND table_schema NOT IN ('pg_catalog', 'information_schema'))
		FROM pg_stat_database AS statistics
		WHERE statistics.datname = current_database()
	`).Scan(&version, &connections, &committed, &rolledBack, &blocksRead, &blocksHit, &temporaryBytes, &deadlocks, &storageBytes, &tables)
	if err != nil {
		return dto.OperationsDashboard{}, operationsRuntimeError(err, "operations dashboard")
	}
	metrics := []dto.OperationsMetric{
		operationMetric("connections", "Connections", "sessions", float64(connections), collectedAt),
		operationMetric("transactions", "Transactions since reset", "total", float64(committed+rolledBack), collectedAt),
		operationMetric("storage", "Storage", "bytes", float64(storageBytes), collectedAt),
		operationMetric("tables", "Tables", "tables", float64(tables), collectedAt),
		operationMetric("temp_bytes", "Temporary data since reset", "bytes", float64(temporaryBytes), collectedAt),
		operationMetric("deadlocks", "Deadlocks since reset", "total", float64(deadlocks), collectedAt),
	}
	if totalReads := blocksRead + blocksHit; totalReads > 0 {
		metrics = append(metrics, operationMetric("cache_hit", "Cache hit", "percent", operationRatio(float64(blocksHit), float64(totalReads)), collectedAt))
	}
	return dto.OperationsDashboard{
		Available:   true,
		Version:     version,
		Window:      input.Window,
		CollectedAt: collectedAt,
		Metrics:     metrics,
	}, nil
}

func postgresOperationsSessions(ctx context.Context, database *sql.DB, input dto.OperationsSessionsInput) (dto.OperationsSessions, error) {
	condition := ""
	switch input.State {
	case dto.OperationsSessionStateActive:
		condition = " AND state = 'active'"
	case dto.OperationsSessionStateIdle:
		condition = " AND state = 'idle'"
	case dto.OperationsSessionStateIdleInTransaction:
		condition = " AND state IN ('idle in transaction', 'idle in transaction (aborted)')"
	}
	query := `
		SELECT pid::text,
		       COALESCE(usename, ''),
		       COALESCE(datname, ''),
		       COALESCE(state, ''),
		       LEFT(COALESCE(query, ''), 4000),
		       GREATEST(COALESCE((EXTRACT(EPOCH FROM (clock_timestamp() - CASE WHEN state = 'active' THEN query_start ELSE state_change END)) * 1000)::bigint, 0), 0),
		       CASE WHEN state = 'active' THEN query_start ELSE state_change END,
		       COALESCE(client_addr::text, ''),
		       COALESCE(wait_event_type || ':' || wait_event, '')
		FROM pg_stat_activity
		WHERE pid <> pg_backend_pid()
		  AND backend_type = 'client backend'` + condition + `
		ORDER BY CASE WHEN state = 'active' THEN query_start ELSE state_change END DESC NULLS LAST, pid
		LIMIT $1
	`
	rows, err := database.QueryContext(ctx, query, input.Limit)
	if err != nil {
		return dto.OperationsSessions{}, operationsRuntimeError(err, "session inspection")
	}
	defer rows.Close()
	items := make([]dto.OperationsSession, 0)
	for rows.Next() {
		var item dto.OperationsSession
		var startedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.User, &item.Database, &item.State, &item.Query, &item.DurationMS, &startedAt, &item.Client, &item.WaitEvent); err != nil {
			return dto.OperationsSessions{}, operationsRuntimeError(err, "session inspection")
		}
		if startedAt.Valid {
			value := startedAt.Time.UTC()
			item.StartedAt = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dto.OperationsSessions{}, operationsRuntimeError(err, "session inspection")
	}
	return dto.OperationsSessions{Available: true, CollectedAt: time.Now().UTC(), Items: items}, nil
}

func postgresOperationsLocks(ctx context.Context, database *sql.DB, input dto.OperationsLocksInput) (dto.OperationsLocks, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT locks.pid::text,
		       locks.locktype,
		       COALESCE(
		         namespace.nspname || '.' || relation.relname,
		         CASE WHEN locks.relation IS NOT NULL THEN 'relation:' || locks.relation::text END,
		         locks.transactionid::text,
		         locks.virtualxid,
		         CASE WHEN locks.classid IS NOT NULL THEN 'object:' || locks.classid::text || ':' || locks.objid::text || ':' || locks.objsubid::text END,
		         ''
		       ),
		       locks.mode,
		       locks.granted,
		       CASE WHEN locks.granted THEN '' ELSE locks.pid::text END,
		       COALESCE(blockers.blocking_pid::text, ''),
		       LEFT(COALESCE(activity.query, ''), 4000)
		FROM pg_locks AS locks
		LEFT JOIN pg_class AS relation
		  ON relation.oid = locks.relation
		 AND locks.database = (SELECT oid FROM pg_database WHERE datname = current_database())
		LEFT JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
		LEFT JOIN pg_stat_activity AS activity ON activity.pid = locks.pid
		LEFT JOIN LATERAL unnest(pg_blocking_pids(locks.pid)) AS blockers(blocking_pid) ON NOT locks.granted
		WHERE locks.pid <> pg_backend_pid()
		ORDER BY locks.granted, locks.pid, locks.locktype, locks.mode
		LIMIT $1
	`, input.Limit)
	if err != nil {
		return dto.OperationsLocks{}, operationsRuntimeError(err, "lock inspection")
	}
	defer rows.Close()
	items := make([]dto.OperationsLock, 0)
	blockers := make(map[string][]string)
	for index := 0; rows.Next(); index++ {
		var processID string
		var item dto.OperationsLock
		if err := rows.Scan(&processID, &item.Type, &item.Object, &item.Mode, &item.Granted, &item.WaitingSessionID, &item.BlockingSessionID, &item.Query); err != nil {
			return dto.OperationsLocks{}, operationsRuntimeError(err, "lock inspection")
		}
		item.ID = operationLockID(processID, ":", item.Type, ":", item.Mode, ":", index)
		if item.WaitingSessionID != "" && item.BlockingSessionID != "" {
			blockers[item.WaitingSessionID] = appendBlockingSession(blockers[item.WaitingSessionID], item.BlockingSessionID)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dto.OperationsLocks{}, operationsRuntimeError(err, "lock inspection")
	}
	return dto.OperationsLocks{Available: true, CollectedAt: time.Now().UTC(), Items: items, BlockingChains: buildBlockingChains(blockers)}, nil
}

func postgresOperationsPerformance(ctx context.Context, database *sql.DB, input dto.OperationsPerformanceInput) (dto.OperationsPerformance, error) {
	source, totalColumn, meanColumn, err := postgresStatementStatisticsSource(ctx, database)
	if err != nil {
		return dto.OperationsPerformance{}, err
	}
	statement := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(queryid, 0)::text, md5(COALESCE(query, ''))),
		       LEFT(COALESCE(query, ''), 4000),
		       calls,
		       %s,
		       %s,
		       rows
		FROM %s
		WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
		  AND calls > 0
		ORDER BY %s DESC, queryid NULLS LAST
		LIMIT $1
	`, totalColumn, meanColumn, source, totalColumn)
	rows, err := database.QueryContext(ctx, statement, input.Limit)
	if err != nil {
		return dto.OperationsPerformance{}, operationsRuntimeError(err, "performance inspection")
	}
	defer rows.Close()
	slowQueries := make([]dto.OperationsSlowQuery, 0)
	for rows.Next() {
		var item dto.OperationsSlowQuery
		if err := rows.Scan(&item.Fingerprint, &item.Query, &item.Calls, &item.TotalMS, &item.MeanMS, &item.Rows); err != nil {
			return dto.OperationsPerformance{}, operationsRuntimeError(err, "performance inspection")
		}
		slowQueries = append(slowQueries, item)
	}
	if err := rows.Err(); err != nil {
		return dto.OperationsPerformance{}, operationsRuntimeError(err, "performance inspection")
	}
	if err := rows.Close(); err != nil {
		return dto.OperationsPerformance{}, operationsRuntimeError(err, "performance inspection")
	}
	dashboard, err := postgresOperationsDashboard(ctx, database, input.Range)
	if err != nil {
		return dto.OperationsPerformance{}, err
	}
	return dto.OperationsPerformance{
		Available:   true,
		Window:      input.Range.Window,
		CollectedAt: time.Now().UTC(),
		Metrics:     dashboard.Metrics,
		SlowQueries: slowQueries,
	}, nil
}

func postgresStatementStatisticsSource(ctx context.Context, database *sql.DB) (string, string, string, error) {
	var schema string
	err := database.QueryRowContext(ctx, `
		SELECT namespace.nspname
		FROM pg_extension AS extension
		JOIN pg_namespace AS namespace ON namespace.oid = extension.extnamespace
		WHERE extension.extname = 'pg_stat_statements'
	`).Scan(&schema)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", "", apperror.NewUnsupported("pg_stat_statements is not enabled", err)
		}
		return "", "", "", operationsRuntimeError(err, "performance inspection")
	}
	rows, err := database.QueryContext(ctx, `
		SELECT attribute.attname
		FROM pg_class AS relation
		JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
		JOIN pg_attribute AS attribute ON attribute.attrelid = relation.oid
		WHERE namespace.nspname = $1
		  AND relation.relname = 'pg_stat_statements'
		  AND attribute.attnum > 0
		  AND NOT attribute.attisdropped
	`, schema)
	if err != nil {
		return "", "", "", operationsRuntimeError(err, "performance inspection")
	}
	defer rows.Close()
	columns := make(map[string]bool)
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return "", "", "", operationsRuntimeError(err, "performance inspection")
		}
		columns[column] = true
	}
	if err := rows.Err(); err != nil {
		return "", "", "", operationsRuntimeError(err, "performance inspection")
	}
	for _, required := range []string{"queryid", "query", "calls", "rows", "dbid"} {
		if !columns[required] {
			return "", "", "", apperror.NewUnsupported("pg_stat_statements columns are unavailable", nil)
		}
	}
	totalName, meanName := "", ""
	if columns["total_exec_time"] && columns["mean_exec_time"] {
		totalName, meanName = "total_exec_time", "mean_exec_time"
	} else if columns["total_time"] && columns["mean_time"] {
		totalName, meanName = "total_time", "mean_time"
	} else {
		return "", "", "", apperror.NewUnsupported("pg_stat_statements timing columns are unavailable", nil)
	}
	dialect := postgresDialect{}
	return dialect.quoteIdentifier(schema) + "." + dialect.quoteIdentifier("pg_stat_statements"), dialect.quoteIdentifier(totalName), dialect.quoteIdentifier(meanName), nil
}

func postgresControlSession(ctx context.Context, database *sql.DB, input dto.OperationsSessionControlInput) error {
	sessionID, err := parseOperationsSessionID(input.SessionID)
	if err != nil {
		return err
	}
	if sessionID > 1<<31-1 {
		return apperror.NewValidation("invalid database session identifier", nil)
	}
	functionName := "pg_cancel_backend"
	if input.Action == dto.OperationsSessionTerminate {
		functionName = "pg_terminate_backend"
	}
	var accepted bool
	if err := database.QueryRowContext(ctx, "SELECT "+functionName+"($1::integer)", int64(sessionID)).Scan(&accepted); err != nil {
		return operationsRuntimeError(err, "session control")
	}
	if !accepted {
		return apperror.NewNotFound("database session was not found", nil)
	}
	return nil
}
