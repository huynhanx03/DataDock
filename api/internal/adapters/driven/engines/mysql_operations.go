package engines

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

func mysqlOperationsDashboard(ctx context.Context, database *sql.DB, connection entity.Connection, input dto.OperationsRangeInput) (dto.OperationsDashboard, error) {
	collectedAt := time.Now().UTC()
	var version string
	if err := database.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		return dto.OperationsDashboard{}, operationsRuntimeError(err, "operations dashboard")
	}
	status, err := mysqlGlobalStatus(ctx, database)
	if err != nil {
		return dto.OperationsDashboard{}, err
	}
	databaseName := connection.Database
	if databaseName == "" {
		if err := database.QueryRowContext(ctx, "SELECT COALESCE(DATABASE(), '')").Scan(&databaseName); err != nil {
			return dto.OperationsDashboard{}, operationsRuntimeError(err, "operations dashboard")
		}
	}
	var tables, storageBytes int64
	if err := database.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(SUM(data_length + index_length), 0) FROM information_schema.tables WHERE table_schema = ? AND table_type IN ('BASE TABLE', 'SYSTEM VERSIONED')", databaseName).Scan(&tables, &storageBytes); err != nil {
		return dto.OperationsDashboard{}, operationsRuntimeError(err, "operations dashboard")
	}
	metrics := make([]dto.OperationsMetric, 0, 7)
	if value, available := parseStatusFloat(status, "Threads_connected"); available {
		metrics = append(metrics, operationMetric("connections", "Connections", "sessions", value, collectedAt))
	}
	if value, available := parseStatusFloat(status, "Questions"); available {
		metrics = append(metrics, operationMetric("queries", "Questions since reset", "total", value, collectedAt))
	}
	committed, committedAvailable := parseStatusFloat(status, "Com_commit")
	rolledBack, rolledBackAvailable := parseStatusFloat(status, "Com_rollback")
	if committedAvailable && rolledBackAvailable {
		metrics = append(metrics, operationMetric("transactions", "Explicit transactions since reset", "total", committed+rolledBack, collectedAt))
	}
	readRequests, readRequestsAvailable := parseStatusFloat(status, "Innodb_buffer_pool_read_requests")
	physicalReads, physicalReadsAvailable := parseStatusFloat(status, "Innodb_buffer_pool_reads")
	if readRequestsAvailable && physicalReadsAvailable && readRequests > 0 {
		cacheHit := operationRatio(readRequests-physicalReads, readRequests)
		if cacheHit < 0 {
			cacheHit = 0
		}
		metrics = append(metrics, operationMetric("cache_hit", "Buffer pool hit", "percent", cacheHit, collectedAt))
	}
	metrics = append(metrics,
		operationMetric("storage", "Storage", "bytes", float64(storageBytes), collectedAt),
		operationMetric("tables", "Tables", "tables", float64(tables), collectedAt),
	)
	if value, available := parseStatusFloat(status, "Uptime"); available {
		metrics = append(metrics, operationMetric("uptime", "Uptime", "seconds", value, collectedAt))
	}
	return dto.OperationsDashboard{
		Available:   true,
		Version:     version,
		Window:      input.Window,
		CollectedAt: collectedAt,
		Metrics:     metrics,
	}, nil
}

func mysqlGlobalStatus(ctx context.Context, database *sql.DB) (map[string]string, error) {
	rows, err := database.QueryContext(ctx, "SHOW GLOBAL STATUS WHERE Variable_name IN ('Threads_connected','Questions','Com_commit','Com_rollback','Innodb_buffer_pool_read_requests','Innodb_buffer_pool_reads','Uptime')")
	if err != nil {
		return nil, operationsRuntimeError(err, "operations dashboard")
	}
	defer rows.Close()
	values := make(map[string]string)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, operationsRuntimeError(err, "operations dashboard")
		}
		values[strings.ToLower(name)] = value
	}
	if err := rows.Err(); err != nil {
		return nil, operationsRuntimeError(err, "operations dashboard")
	}
	return values, nil
}

func mysqlOperationsSessions(ctx context.Context, database *sql.DB, input dto.OperationsSessionsInput) (dto.OperationsSessions, error) {
	join := ""
	condition := " WHERE process.ID <> CONNECTION_ID()"
	idleInTransaction := false
	switch input.State {
	case dto.OperationsSessionStateActive:
		condition += " AND process.COMMAND <> 'Sleep'"
	case dto.OperationsSessionStateIdle:
		condition += " AND process.COMMAND = 'Sleep'"
	case dto.OperationsSessionStateIdleInTransaction:
		join = " JOIN information_schema.INNODB_TRX AS transactions ON transactions.TRX_MYSQL_THREAD_ID = process.ID"
		condition += " AND process.COMMAND = 'Sleep'"
		idleInTransaction = true
	}
	query := "SELECT process.ID, process.USER, COALESCE(process.DB, ''), process.COMMAND, LEFT(COALESCE(process.INFO, ''), 4000), process.TIME, COALESCE(process.HOST, ''), COALESCE(process.STATE, '') FROM information_schema.PROCESSLIST AS process" + join + condition + " ORDER BY process.TIME DESC, process.ID LIMIT ?"
	rows, err := database.QueryContext(ctx, query, input.Limit)
	if err != nil {
		return dto.OperationsSessions{}, operationsRuntimeError(err, "session inspection")
	}
	defer rows.Close()
	collectedAt := time.Now().UTC()
	items := make([]dto.OperationsSession, 0)
	for rows.Next() {
		var id uint64
		var seconds int64
		var command string
		var item dto.OperationsSession
		if err := rows.Scan(&id, &item.User, &item.Database, &command, &item.Query, &seconds, &item.Client, &item.WaitEvent); err != nil {
			return dto.OperationsSessions{}, operationsRuntimeError(err, "session inspection")
		}
		item.ID = fmt.Sprint(id)
		item.DurationMS, seconds = mysqlSessionDuration(seconds)
		startedAt := collectedAt.Add(-time.Duration(seconds) * time.Second)
		item.StartedAt = &startedAt
		if idleInTransaction {
			item.State = "idle in transaction"
		} else if strings.EqualFold(command, "sleep") {
			item.State = "idle"
		} else {
			item.State = "active"
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dto.OperationsSessions{}, operationsRuntimeError(err, "session inspection")
	}
	return dto.OperationsSessions{Available: true, CollectedAt: collectedAt, Items: items}, nil
}

func mysqlSessionDuration(seconds int64) (int64, int64) {
	const maximumInt64 = int64(^uint64(0) >> 1)
	if seconds < 0 {
		seconds = 0
	}
	durationMS := maximumInt64
	if seconds <= maximumInt64/1000 {
		durationMS = seconds * 1000
	}
	maximumDurationSeconds := maximumInt64 / int64(time.Second)
	if seconds > maximumDurationSeconds {
		seconds = maximumDurationSeconds
	}
	return durationMS, seconds
}

func mysqlOperationsLocks(ctx context.Context, database *sql.DB, input dto.OperationsLocksInput) (dto.OperationsLocks, error) {
	result, err := mysqlPerformanceSchemaLocks(ctx, database, input.Limit)
	if err == nil {
		return result, nil
	}
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) || applicationError.Code != apperror.CodeUnsupportedCapability {
		return dto.OperationsLocks{}, err
	}
	return mysqlInformationSchemaLocks(ctx, database, input.Limit)
}

func mysqlPerformanceSchemaLocks(ctx context.Context, database *sql.DB, limit int) (dto.OperationsLocks, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT CONCAT(waits.REQUESTING_ENGINE_LOCK_ID, '|', waits.BLOCKING_ENGINE_LOCK_ID),
		       COALESCE(requested.OBJECT_TYPE, 'lock'),
		       CONCAT_WS('.', requested.OBJECT_SCHEMA, requested.OBJECT_NAME),
		       COALESCE(requested.LOCK_MODE, ''),
		       COALESCE(requesting_thread.PROCESSLIST_ID, waits.REQUESTING_THREAD_ID),
		       COALESCE(blocking_thread.PROCESSLIST_ID, waits.BLOCKING_THREAD_ID),
		       LEFT(COALESCE(requesting_statement.SQL_TEXT, ''), 4000)
		FROM performance_schema.data_lock_waits AS waits
		LEFT JOIN performance_schema.data_locks AS requested
		  ON requested.ENGINE = waits.ENGINE AND requested.ENGINE_LOCK_ID = waits.REQUESTING_ENGINE_LOCK_ID
		LEFT JOIN performance_schema.threads AS requesting_thread
		  ON requesting_thread.THREAD_ID = waits.REQUESTING_THREAD_ID
		LEFT JOIN performance_schema.threads AS blocking_thread
		  ON blocking_thread.THREAD_ID = waits.BLOCKING_THREAD_ID
		LEFT JOIN performance_schema.events_statements_current AS requesting_statement
		  ON requesting_statement.THREAD_ID = requested.THREAD_ID
		 AND requesting_statement.EVENT_ID = requested.EVENT_ID
		ORDER BY requesting_thread.PROCESSLIST_ID, blocking_thread.PROCESSLIST_ID
		LIMIT ?
	`, limit)
	if err != nil {
		return dto.OperationsLocks{}, operationsRuntimeError(err, "lock inspection")
	}
	defer rows.Close()
	return scanMySQLLocks(rows)
}

func mysqlInformationSchemaLocks(ctx context.Context, database *sql.DB, limit int) (dto.OperationsLocks, error) {
	rows, err := database.QueryContext(ctx, `
		SELECT CONCAT(waits.requested_lock_id, '|', waits.blocking_lock_id),
		       'transaction',
		       COALESCE(requested.lock_table, ''),
		       COALESCE(requested.lock_mode, ''),
		       requesting.trx_mysql_thread_id,
		       blocking.trx_mysql_thread_id,
		       LEFT(COALESCE(process.INFO, ''), 4000)
		FROM information_schema.innodb_lock_waits AS waits
		JOIN information_schema.innodb_trx AS requesting ON requesting.trx_id = waits.requesting_trx_id
		JOIN information_schema.innodb_trx AS blocking ON blocking.trx_id = waits.blocking_trx_id
		LEFT JOIN information_schema.innodb_locks AS requested ON requested.lock_id = waits.requested_lock_id
		LEFT JOIN information_schema.PROCESSLIST AS process ON process.ID = requesting.trx_mysql_thread_id
		ORDER BY requesting.trx_mysql_thread_id, blocking.trx_mysql_thread_id
		LIMIT ?
	`, limit)
	if err != nil {
		return dto.OperationsLocks{}, operationsRuntimeError(err, "lock inspection")
	}
	defer rows.Close()
	return scanMySQLLocks(rows)
}

func scanMySQLLocks(rows *sql.Rows) (dto.OperationsLocks, error) {
	items := make([]dto.OperationsLock, 0)
	blockers := make(map[string][]string)
	for rows.Next() {
		var item dto.OperationsLock
		if err := rows.Scan(&item.ID, &item.Type, &item.Object, &item.Mode, &item.WaitingSessionID, &item.BlockingSessionID, &item.Query); err != nil {
			return dto.OperationsLocks{}, operationsRuntimeError(err, "lock inspection")
		}
		item.Granted = false
		blockers[item.WaitingSessionID] = appendBlockingSession(blockers[item.WaitingSessionID], item.BlockingSessionID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return dto.OperationsLocks{}, operationsRuntimeError(err, "lock inspection")
	}
	return dto.OperationsLocks{Available: true, CollectedAt: time.Now().UTC(), Items: items, BlockingChains: buildBlockingChains(blockers)}, nil
}

func mysqlOperationsPerformance(ctx context.Context, database *sql.DB, connection entity.Connection, input dto.OperationsPerformanceInput) (dto.OperationsPerformance, error) {
	var performanceSchemaEnabled int
	if err := database.QueryRowContext(ctx, "SELECT IF(@@performance_schema, 1, 0)").Scan(&performanceSchemaEnabled); err != nil {
		return dto.OperationsPerformance{}, operationsRuntimeError(err, "performance inspection")
	}
	if performanceSchemaEnabled == 0 {
		return dto.OperationsPerformance{}, apperror.NewUnsupported("performance schema is disabled", nil)
	}
	databaseName := connection.Database
	if databaseName == "" {
		if err := database.QueryRowContext(ctx, "SELECT COALESCE(DATABASE(), '')").Scan(&databaseName); err != nil {
			return dto.OperationsPerformance{}, operationsRuntimeError(err, "performance inspection")
		}
	}
	rows, err := database.QueryContext(ctx, `
		SELECT DIGEST,
		       LEFT(DIGEST_TEXT, 4000),
		       COUNT_STAR,
		       SUM_TIMER_WAIT / 1000000000,
		       AVG_TIMER_WAIT / 1000000000,
		       SUM_ROWS_SENT
		FROM performance_schema.events_statements_summary_by_digest
		WHERE SCHEMA_NAME = ?
		  AND DIGEST IS NOT NULL
		  AND DIGEST_TEXT IS NOT NULL
		  AND COUNT_STAR > 0
		ORDER BY SUM_TIMER_WAIT DESC, DIGEST
		LIMIT ?
	`, databaseName, input.Limit)
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
	dashboard, err := mysqlOperationsDashboard(ctx, database, connection, input.Range)
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

func mysqlControlSession(ctx context.Context, database *sql.DB, input dto.OperationsSessionControlInput) error {
	sessionID, err := parseOperationsSessionID(input.SessionID)
	if err != nil {
		return err
	}
	statement := "KILL QUERY " + fmt.Sprint(sessionID)
	if input.Action == dto.OperationsSessionTerminate {
		statement = "KILL CONNECTION " + fmt.Sprint(sessionID)
	}
	if _, err := database.ExecContext(ctx, statement); err != nil {
		return operationsRuntimeError(err, "session control")
	}
	return nil
}
