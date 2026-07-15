package engines

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
	"github.com/lib/pq"
)

var _ ports.OperationsRuntime = (*Manager)(nil)

func (manager *Manager) Dashboard(ctx context.Context, connection entity.Connection, password string, input dto.OperationsRangeInput) (dto.OperationsDashboard, error) {
	if !validOperationsRuntimeRange(input) {
		return dto.OperationsDashboard{}, apperror.NewValidation("invalid operations range", nil)
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return dto.OperationsDashboard{}, err
	}
	if connection.Engine == entity.EnginePostgreSQL {
		return postgresOperationsDashboard(ctx, database, input)
	}
	if connection.Engine == entity.EngineMySQL || connection.Engine == entity.EngineMariaDB {
		return mysqlOperationsDashboard(ctx, database, connection, input)
	}
	return dto.OperationsDashboard{}, apperror.NewUnsupported("operations dashboard is unsupported for this database engine", nil)
}

func (manager *Manager) Sessions(ctx context.Context, connection entity.Connection, password string, input dto.OperationsSessionsInput) (dto.OperationsSessions, error) {
	if input.Limit < 1 || input.Limit > 500 || !validOperationsRuntimeSessionState(input.State) {
		return dto.OperationsSessions{}, apperror.NewValidation("invalid session list request", nil)
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return dto.OperationsSessions{}, err
	}
	if connection.Engine == entity.EnginePostgreSQL {
		return postgresOperationsSessions(ctx, database, input)
	}
	if connection.Engine == entity.EngineMySQL || connection.Engine == entity.EngineMariaDB {
		return mysqlOperationsSessions(ctx, database, input)
	}
	return dto.OperationsSessions{}, apperror.NewUnsupported("session inspection is unsupported for this database engine", nil)
}

func (manager *Manager) Locks(ctx context.Context, connection entity.Connection, password string, input dto.OperationsLocksInput) (dto.OperationsLocks, error) {
	if input.Limit < 1 || input.Limit > 500 {
		return dto.OperationsLocks{}, apperror.NewValidation("invalid lock list request", nil)
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return dto.OperationsLocks{}, err
	}
	if connection.Engine == entity.EnginePostgreSQL {
		return postgresOperationsLocks(ctx, database, input)
	}
	if connection.Engine == entity.EngineMySQL || connection.Engine == entity.EngineMariaDB {
		return mysqlOperationsLocks(ctx, database, input)
	}
	return dto.OperationsLocks{}, apperror.NewUnsupported("lock inspection is unsupported for this database engine", nil)
}

func (manager *Manager) Performance(ctx context.Context, connection entity.Connection, password string, input dto.OperationsPerformanceInput) (dto.OperationsPerformance, error) {
	if input.Limit < 1 || input.Limit > 100 || !validOperationsRuntimeRange(input.Range) {
		return dto.OperationsPerformance{}, apperror.NewValidation("invalid performance request", nil)
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return dto.OperationsPerformance{}, err
	}
	if connection.Engine == entity.EnginePostgreSQL {
		return postgresOperationsPerformance(ctx, database, input)
	}
	if connection.Engine == entity.EngineMySQL || connection.Engine == entity.EngineMariaDB {
		return mysqlOperationsPerformance(ctx, database, connection, input)
	}
	return dto.OperationsPerformance{}, apperror.NewUnsupported("performance inspection is unsupported for this database engine", nil)
}

func (manager *Manager) ControlSession(ctx context.Context, connection entity.Connection, password string, input dto.OperationsSessionControlInput) error {
	if input.Action != dto.OperationsSessionCancel && input.Action != dto.OperationsSessionTerminate {
		return apperror.NewValidation("invalid session control request", nil)
	}
	if _, err := parseOperationsSessionID(input.SessionID); err != nil {
		return err
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return err
	}
	if connection.Engine == entity.EnginePostgreSQL {
		return postgresControlSession(ctx, database, input)
	}
	if connection.Engine == entity.EngineMySQL || connection.Engine == entity.EngineMariaDB {
		return mysqlControlSession(ctx, database, input)
	}
	return apperror.NewUnsupported("session control is unsupported for this database engine", nil)
}

func operationMetric(key, label, unit string, value float64, collectedAt time.Time) dto.OperationsMetric {
	return dto.OperationsMetric{
		Key:   key,
		Label: label,
		Value: value,
		Unit:  unit,
		Series: []dto.OperationsMetricPoint{{
			Timestamp: collectedAt,
			Value:     value,
		}},
	}
}

func operationRatio(numerator, denominator float64) float64 {
	if denominator <= 0 {
		return 0
	}
	return numerator / denominator * 100
}

func appendBlockingSession(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func buildBlockingChains(blockers map[string][]string) []dto.OperationsBlockingChain {
	waiting := make([]string, 0, len(blockers))
	for sessionID := range blockers {
		waiting = append(waiting, sessionID)
	}
	sortStrings(waiting)
	result := make([]dto.OperationsBlockingChain, 0, len(waiting))
	for _, sessionID := range waiting {
		blocking := blockers[sessionID]
		sortStrings(blocking)
		result = append(result, dto.OperationsBlockingChain{WaitingSessionID: sessionID, BlockingSessionIDs: blocking})
	}
	return result
}

func sortStrings(values []string) {
	for index := 1; index < len(values); index++ {
		for current := index; current > 0 && values[current] < values[current-1]; current-- {
			values[current], values[current-1] = values[current-1], values[current]
		}
	}
}

func truncateOperationsText(value string) string {
	const maximum = 4000
	if len(value) <= maximum {
		return value
	}
	return value[:maximum]
}

func parseOperationsSessionID(value string) (uint64, error) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return 0, apperror.NewValidation("invalid database session identifier", err)
	}
	return parsed, nil
}

func validOperationsRuntimeRange(input dto.OperationsRangeInput) bool {
	if input.Points < 10 || input.Points > 360 {
		return false
	}
	switch input.Window {
	case dto.OperationsWindow5Minutes, dto.OperationsWindow15Minutes, dto.OperationsWindow1Hour, dto.OperationsWindow6Hours, dto.OperationsWindow24Hours:
		return true
	default:
		return false
	}
}

func validOperationsRuntimeSessionState(state dto.OperationsSessionState) bool {
	switch state {
	case dto.OperationsSessionStateAll, dto.OperationsSessionStateActive, dto.OperationsSessionStateIdle, dto.OperationsSessionStateIdleInTransaction:
		return true
	default:
		return false
	}
}

func operationsRuntimeError(err error, capability string) error {
	if err == nil {
		return nil
	}
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return err
	}
	if errors.Is(err, context.Canceled) {
		return apperror.NewCancellation(capability+" was cancelled", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apperror.NewTimeout(capability+" timed out", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.NewNotFound(capability+" resource was not found", err)
	}
	var postgresError *pq.Error
	if errors.As(err, &postgresError) {
		switch string(postgresError.Code) {
		case "42501":
			return apperror.NewPermission(capability+" permission denied", err)
		case "42P01", "42703", "42704", "42883", "55000", "0A000":
			return apperror.NewUnsupported(capability+" is unavailable", err)
		case "57014":
			return apperror.NewCancellation(capability+" was cancelled", err)
		}
	}
	var mysqlError *mysqldriver.MySQLError
	if errors.As(err, &mysqlError) {
		switch mysqlError.Number {
		case 1044, 1045, 1095, 1142, 1227, 1370:
			return apperror.NewPermission(capability+" permission denied", err)
		case 1054, 1109, 1146, 1193, 1235, 1286, 1932:
			return apperror.NewUnsupported(capability+" is unavailable", err)
		case 1094:
			return apperror.NewNotFound("database session was not found", err)
		case 1317:
			return apperror.NewCancellation(capability+" was cancelled", err)
		}
	}
	return apperror.NewInternal(capability+" failed", err)
}

func parseStatusFloat(values map[string]string, name string) (float64, bool) {
	value, exists := values[strings.ToLower(name)]
	if !exists {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed, err == nil
}

func operationLockID(parts ...any) string {
	return fmt.Sprint(parts...)
}
