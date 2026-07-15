package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/ports"
)

type QueryServiceOptions struct {
	MaxRows        int
	MaxBytes       int64
	DefaultTimeout time.Duration
	MaxTimeout     time.Duration
	HistoryTimeout time.Duration
}

type QueryService struct {
	profiles ports.ConnectionProfileResolver
	runtime  ports.QueryRuntime
	history  ports.QueryHistoryRepository
	options  QueryServiceOptions
}

type queryRequest struct {
	executionID    string
	connectionID   string
	sql            string
	timeoutSeconds int
	transactionID  string
}

func NewQueryService(profiles ports.ConnectionProfileResolver, runtime ports.QueryRuntime, history ports.QueryHistoryRepository, options QueryServiceOptions) *QueryService {
	return &QueryService{profiles: profiles, runtime: runtime, history: history, options: normalizeQueryServiceOptions(options)}
}

func (service *QueryService) Execute(ctx context.Context, input dto.QueryExecutionInput) (dto.QueryExecutionResult, error) {
	request := queryRequest{
		executionID:    input.ExecutionID,
		connectionID:   input.ConnectionID,
		sql:            input.SQL,
		timeoutSeconds: input.TimeoutSeconds,
		transactionID:  input.TransactionID,
	}
	return service.run(ctx, request, "")
}

func (service *QueryService) Explain(ctx context.Context, input dto.QueryExplainInput) (dto.QueryExecutionResult, error) {
	mode := dto.QueryExplainOnly
	if input.Analyze {
		mode = dto.QueryExplainAnalyze
	}
	request := queryRequest{
		executionID:    input.ExecutionID,
		connectionID:   input.ConnectionID,
		sql:            input.SQL,
		timeoutSeconds: input.TimeoutSeconds,
		transactionID:  input.TransactionID,
	}
	return service.run(ctx, request, mode)
}

func (service *QueryService) Cancel(ctx context.Context, executionID string) error {
	if !validQueryID(executionID, false) {
		return apperror.NewValidation("invalid query execution identifier", nil)
	}
	if err := service.runtime.Cancel(ctx, executionID); err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return apperror.NewNotFound("query execution not found", err)
		}
		var appErr *apperror.Error
		if errors.As(err, &appErr) && appErr.Code == apperror.CodeNotFound {
			return apperror.NewNotFound("query execution not found", err)
		}
		mapped := safeQueryError(err)
		if mapped.Code == apperror.CodeInternal {
			return apperror.NewInternal("query cancellation failed", err)
		}
		return mapped
	}
	return nil
}

func (service *QueryService) ListHistory(ctx context.Context, connectionID string) ([]dto.QueryHistoryItem, error) {
	if !validQueryID(connectionID, true) {
		return nil, apperror.NewValidation("invalid query history filter", nil)
	}
	items, err := service.history.List(ctx, connectionID)
	if err != nil {
		return nil, queryHistoryError(err)
	}
	result := make([]dto.QueryHistoryItem, 0, len(items))
	for _, item := range items {
		status := dto.QueryExecutionStatus(item.Status)
		result = append(result, dto.QueryHistoryItem{
			ID:           item.ID,
			ConnectionID: item.ConnectionID,
			SQL:          item.SQLText,
			Status:       status,
			DurationMS:   item.DurationMS,
			RowCount:     item.RowCount,
			Error:        safeStoredQueryError(status, item.Error),
			ExecutedAt:   item.ExecutedAt,
		})
	}
	return result, nil
}

func (service *QueryService) ClearHistory(ctx context.Context, connectionID string) error {
	if !validQueryID(connectionID, true) {
		return apperror.NewValidation("invalid query history filter", nil)
	}
	if err := service.history.ClearByConnection(ctx, connectionID); err != nil {
		return queryHistoryError(err)
	}
	return nil
}

func (service *QueryService) DeleteHistory(ctx context.Context, input dto.QueryHistoryDeleteInput) error {
	if len(input.IDs) < 1 || len(input.IDs) > 100 {
		return apperror.NewValidation("invalid query history selection", nil)
	}
	seen := make(map[string]struct{}, len(input.IDs))
	ids := make([]string, len(input.IDs))
	for index, id := range input.IDs {
		if !validQueryID(id, false) {
			return apperror.NewValidation("invalid query history selection", nil)
		}
		if _, exists := seen[id]; exists {
			return apperror.NewValidation("duplicate query history identifier", nil)
		}
		seen[id] = struct{}{}
		ids[index] = id
	}
	if err := service.history.DeleteByIDs(ctx, ids); err != nil {
		return queryHistoryError(err)
	}
	return nil
}

func (service *QueryService) run(ctx context.Context, request queryRequest, explainMode dto.QueryExplainMode) (dto.QueryExecutionResult, error) {
	if err := service.validateRequest(request); err != nil {
		return dto.QueryExecutionResult{}, err
	}
	if request.executionID == "" {
		request.executionID = uuid.NewString()
	}
	startedAt := time.Now()
	connection, password, err := service.profiles.Resolve(ctx, request.connectionID)
	if err != nil {
		return dto.QueryExecutionResult{}, service.finishFailure(ctx, request, startedAt, err)
	}
	statement, err := service.runtime.ClassifyStatement(connection.Engine, request.sql)
	if err != nil {
		return dto.QueryExecutionResult{}, service.finishFailure(ctx, request, startedAt, err)
	}
	if explainMode == dto.QueryExplainAnalyze && !statement.ReadOnly {
		err = apperror.NewValidation("EXPLAIN ANALYZE is limited to read-only statements", nil)
		return dto.QueryExecutionResult{}, service.finishFailure(ctx, request, startedAt, err)
	}
	if connection.ReadOnly && !statement.ReadOnly {
		err = apperror.NewReadonly("query rejected by read-only connection", nil)
		return dto.QueryExecutionResult{}, service.finishFailure(ctx, request, startedAt, err)
	}

	runtimeRequest := ports.QueryRuntimeRequest{
		ExecutionID:   request.executionID,
		SQL:           request.sql,
		TransactionID: request.transactionID,
		Limits: dto.QueryExecutionLimits{
			MaxRows:  service.options.MaxRows,
			MaxBytes: service.options.MaxBytes,
		},
	}
	executionContext, cancel := context.WithTimeout(ctx, service.queryTimeout(request.timeoutSeconds))
	defer cancel()
	var result dto.QueryExecutionResult
	if explainMode == "" {
		result, err = service.runtime.Execute(executionContext, connection, password, runtimeRequest)
	} else {
		result, err = service.runtime.Explain(executionContext, connection, password, runtimeRequest, explainMode)
	}
	if err != nil {
		if executionContext.Err() != nil {
			err = executionContext.Err()
		}
		return dto.QueryExecutionResult{}, service.finishFailure(ctx, request, startedAt, err)
	}
	result.ExecutionID = request.executionID
	result.StatementType = statement.Type
	result.Limits.MaxRows = service.options.MaxRows
	result.Limits.MaxBytes = service.options.MaxBytes
	if result.Columns == nil {
		result.Columns = []dto.DataColumn{}
	}
	if result.Rows == nil {
		result.Rows = [][]any{}
	}
	if result.Notices == nil {
		result.Notices = []dto.QueryNotice{}
	}
	if result.DurationMS == 0 {
		result.DurationMS = elapsedMilliseconds(startedAt)
	}
	if result.Plan != nil && explainMode != "" {
		result.Plan.Mode = explainMode
		if result.Plan.Table.Columns == nil {
			result.Plan.Table.Columns = []dto.DataColumn{}
		}
		if result.Plan.Table.Rows == nil {
			result.Plan.Table.Rows = [][]any{}
		}
		if result.Plan.Tree == nil {
			result.Plan.Tree = []dto.QueryPlanNode{}
		}
	}
	if err := service.persistHistory(ctx, ports.QueryHistory{
		ID:           request.executionID,
		ConnectionID: request.connectionID,
		SQLText:      request.sql,
		Status:       string(dto.QueryExecutionSucceeded),
		DurationMS:   result.DurationMS,
		RowCount:     queryResultRowCount(result),
		ExecutedAt:   time.Now().UTC(),
	}); err != nil {
		return dto.QueryExecutionResult{}, err
	}
	return result, nil
}

func (service *QueryService) validateRequest(request queryRequest) error {
	if !validQueryID(request.connectionID, false) || !validQueryID(request.executionID, true) || !validQueryID(request.transactionID, true) {
		return apperror.NewValidation("invalid query execution request", nil)
	}
	if strings.TrimSpace(request.sql) == "" || len(request.sql) > 100000 || strings.ContainsRune(request.sql, 0) {
		return apperror.NewValidation("invalid query execution request", nil)
	}
	maximumSeconds := int(service.options.MaxTimeout / time.Second)
	if request.timeoutSeconds < 0 || request.timeoutSeconds > 0 && (maximumSeconds < 1 || request.timeoutSeconds > maximumSeconds) {
		return apperror.NewValidation("invalid query timeout", nil)
	}
	return nil
}

func (service *QueryService) queryTimeout(seconds int) time.Duration {
	if seconds == 0 {
		return service.options.DefaultTimeout
	}
	return time.Duration(seconds) * time.Second
}

func (service *QueryService) finishFailure(ctx context.Context, request queryRequest, startedAt time.Time, err error) error {
	mapped := safeQueryError(err)
	status := dto.QueryExecutionFailed
	if mapped.Code == apperror.CodeQueryTimeout {
		status = dto.QueryExecutionTimedOut
	}
	if mapped.Code == apperror.CodeQueryCancelled {
		status = dto.QueryExecutionCancelled
	}
	persistErr := service.persistHistory(ctx, ports.QueryHistory{
		ID:           request.executionID,
		ConnectionID: request.connectionID,
		SQLText:      request.sql,
		Status:       string(status),
		DurationMS:   elapsedMilliseconds(startedAt),
		Error:        mapped.Message,
		ExecutedAt:   time.Now().UTC(),
	})
	if persistErr != nil {
		mapped.Cause = errors.Join(mapped.Cause, persistErr)
	}
	return mapped
}

func (service *QueryService) persistHistory(ctx context.Context, item ports.QueryHistory) error {
	baseContext := context.WithoutCancel(ctx)
	historyContext, cancel := context.WithTimeout(baseContext, service.options.HistoryTimeout)
	defer cancel()
	if _, err := service.history.Create(historyContext, item); err != nil {
		return queryHistoryError(err)
	}
	return nil
}

func normalizeQueryServiceOptions(options QueryServiceOptions) QueryServiceOptions {
	if options.MaxRows <= 0 {
		options.MaxRows = 1000
	}
	if options.MaxBytes <= 0 {
		options.MaxBytes = 10 * 1024 * 1024
	}
	if options.MaxTimeout <= 0 {
		options.MaxTimeout = 60 * time.Second
	}
	if options.DefaultTimeout <= 0 {
		options.DefaultTimeout = 30 * time.Second
	}
	if options.DefaultTimeout > options.MaxTimeout {
		options.DefaultTimeout = options.MaxTimeout
	}
	if options.HistoryTimeout <= 0 {
		options.HistoryTimeout = 2 * time.Second
	}
	return options
}

func validQueryID(value string, optional bool) bool {
	if value == "" {
		return optional
	}
	if len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 32 || character == 127 {
			return false
		}
	}
	return true
}

func safeQueryError(err error) *apperror.Error {
	if errors.Is(err, context.DeadlineExceeded) {
		return apperror.NewTimeout("query timed out", err)
	}
	if errors.Is(err, context.Canceled) {
		return apperror.NewCancellation("query was cancelled", err)
	}
	var appErr *apperror.Error
	if !errors.As(err, &appErr) {
		if errors.Is(err, ports.ErrNotFound) {
			return apperror.NewNotFound("query resource not found", err)
		}
		return apperror.NewInternal("query execution failed", err)
	}
	messages := map[string]string{
		apperror.CodeValidation:            "query is invalid",
		apperror.CodeNotFound:              "query resource not found",
		apperror.CodeConflict:              "query execution conflict",
		apperror.CodeConnectionFailed:      "database connection failed",
		apperror.CodeConnectionRequired:    "database connection is not connected",
		apperror.CodeConnectionBusy:        "database connection is busy",
		apperror.CodeQueryCancelled:        "query was cancelled",
		apperror.CodeQueryTimeout:          "query timed out",
		apperror.CodeReadonlyViolation:     "query rejected by read-only connection",
		apperror.CodePermissionDenied:      "query permission denied",
		apperror.CodeUnsupportedCapability: "query operation is unsupported",
		apperror.CodeTransactionExpired:    "transaction expired",
		apperror.CodeTransportFailed:       "database transport failed",
		apperror.CodeInternal:              "query execution failed",
	}
	message, exists := messages[appErr.Code]
	if !exists {
		return apperror.NewInternal("query execution failed", err)
	}
	return apperror.New(appErr.Code, message, err, appErr.Temporary)
}

func queryHistoryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return apperror.NewCancellation("request was cancelled", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apperror.NewTimeout("query history operation timed out", err)
	}
	return apperror.NewInternal("query history operation failed", err)
}

func queryResultRowCount(result dto.QueryExecutionResult) int64 {
	if result.Plan != nil && len(result.Plan.Table.Rows) > 0 {
		return int64(len(result.Plan.Table.Rows))
	}
	if len(result.Rows) > 0 {
		return int64(len(result.Rows))
	}
	if result.RowsAffected > 0 {
		return result.RowsAffected
	}
	return 0
}

func elapsedMilliseconds(startedAt time.Time) int64 {
	return time.Since(startedAt).Milliseconds()
}

func safeStoredQueryError(status dto.QueryExecutionStatus, value string) string {
	if status == dto.QueryExecutionTimedOut {
		return "query timed out"
	}
	if status == dto.QueryExecutionCancelled {
		return "query was cancelled"
	}
	if status != dto.QueryExecutionFailed || value == "" {
		return ""
	}
	switch value {
	case "query is invalid",
		"query resource not found",
		"query execution conflict",
		"database connection failed",
		"database connection is not connected",
		"database connection is busy",
		"query rejected by read-only connection",
		"query permission denied",
		"query operation is unsupported",
		"transaction expired",
		"database transport failed",
		"query execution failed":
		return value
	}
	return "query execution failed"
}
