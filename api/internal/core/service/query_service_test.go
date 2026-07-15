package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/core/service"
	"github.com/huynhanx03/datadock/internal/ports"
)

func TestQueryServiceExecutesTypedBoundedQueryAndPersistsSuccess(t *testing.T) {
	connection := queryConnectionFixture("connection-1", "Primary")
	profiles := &queryProfileResolverFake{connection: connection, password: "database-secret"}
	runtime := &queryRuntimeFake{
		statement: ports.QueryStatement{Type: dto.QueryStatementSelect, ReadOnly: true},
		executeResult: dto.QueryExecutionResult{
			Columns: []dto.DataColumn{
				{Key: "id", Name: "id", DatabaseType: "int8", LogicalType: dto.LogicalTypeBigInt, ValueEncoding: dto.ValueEncodingDecimal},
				{Key: "amount", Name: "amount", DatabaseType: "numeric", LogicalType: dto.LogicalTypeDecimal, ValueEncoding: dto.ValueEncodingDecimal},
			},
			Rows:       [][]any{{"9223372036854775807", "12345678901234567890.123456789"}},
			DurationMS: 12,
			Truncated:  true,
			Notices:    []dto.QueryNotice{{Severity: "notice", Message: "using cached plan"}},
		},
	}
	history := &queryHistoryRepositoryFake{}
	subject := newQueryService(profiles, runtime, history)

	result, err := subject.Execute(context.Background(), dto.QueryExecutionInput{
		ExecutionID:   "execution-1",
		ConnectionID:  connection.ID,
		SQL:           "SELECT id, amount FROM invoices",
		TransactionID: "transaction-1",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ExecutionID != "execution-1" || result.StatementType != dto.QueryStatementSelect {
		t.Fatalf("Execute() identity = %#v", result)
	}
	if len(result.Columns) != 2 || len(result.Rows) != 1 || result.Rows[0][0] != "9223372036854775807" || result.Rows[0][1] != "12345678901234567890.123456789" {
		t.Fatalf("Execute() typed rows = %#v", result)
	}
	if !result.Truncated || result.Limits.MaxRows != 250 || result.Limits.MaxBytes != 2*1024*1024 || len(result.Notices) != 1 {
		t.Fatalf("Execute() bounds = %#v", result)
	}
	if runtime.executeCalls != 1 || runtime.lastConnection.ID != connection.ID || runtime.lastPassword != "database-secret" {
		t.Fatalf("runtime profile = %#v, %q, calls=%d", runtime.lastConnection, runtime.lastPassword, runtime.executeCalls)
	}
	if runtime.lastRequest.ExecutionID != "execution-1" || runtime.lastRequest.TransactionID != "transaction-1" || runtime.lastRequest.Limits.MaxRows != 250 || runtime.lastRequest.Limits.MaxBytes != 2*1024*1024 {
		t.Fatalf("runtime request = %#v", runtime.lastRequest)
	}
	if len(history.created) != 1 {
		t.Fatalf("history count = %d", len(history.created))
	}
	created := history.created[0]
	if created.ID != "execution-1" || created.ConnectionID != connection.ID || created.Status != string(dto.QueryExecutionSucceeded) || created.RowCount != 1 || created.Error != "" || created.ExecutedAt.IsZero() {
		t.Fatalf("history item = %#v", created)
	}
	if history.createContextErr != nil || !history.createContextHasDeadline {
		t.Fatalf("history context err=%v deadline=%v", history.createContextErr, history.createContextHasDeadline)
	}
}

func TestQueryServiceGeneratesExecutionID(t *testing.T) {
	profiles := &queryProfileResolverFake{connection: queryConnectionFixture("connection-1", "Primary")}
	runtime := &queryRuntimeFake{statement: ports.QueryStatement{Type: dto.QueryStatementSelect, ReadOnly: true}}
	history := &queryHistoryRepositoryFake{}
	subject := newQueryService(profiles, runtime, history)

	result, err := subject.Execute(context.Background(), dto.QueryExecutionInput{ConnectionID: "connection-1", SQL: "SELECT 1"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if strings.TrimSpace(result.ExecutionID) == "" || runtime.lastRequest.ExecutionID != result.ExecutionID || history.created[0].ID != result.ExecutionID {
		t.Fatalf("generated execution identity result=%q request=%q history=%q", result.ExecutionID, runtime.lastRequest.ExecutionID, history.created[0].ID)
	}
}

func TestQueryServiceRejectsReadonlyWriteBeforeExecutionAndPersistsOutcome(t *testing.T) {
	connection := queryConnectionFixture("connection-1", "Readonly")
	connection.ReadOnly = true
	profiles := &queryProfileResolverFake{connection: connection}
	runtime := &queryRuntimeFake{statement: ports.QueryStatement{Type: dto.QueryStatementUpdate, ReadOnly: false}}
	history := &queryHistoryRepositoryFake{}
	subject := newQueryService(profiles, runtime, history)

	_, err := subject.Execute(context.Background(), dto.QueryExecutionInput{ExecutionID: "execution-readonly", ConnectionID: connection.ID, SQL: "UPDATE users SET active = false"})
	if queryApplicationErrorCode(err) != apperror.CodeReadonlyViolation {
		t.Fatalf("Execute() error = %#v", err)
	}
	if runtime.executeCalls != 0 || runtime.explainCalls != 0 {
		t.Fatalf("readonly query reached execution runtime")
	}
	if len(history.created) != 1 || history.created[0].Status != string(dto.QueryExecutionFailed) || history.created[0].Error != "query rejected by read-only connection" {
		t.Fatalf("readonly history = %#v", history.created)
	}
}

func TestQueryServicePersistsTimeoutWithDetachedBoundedContext(t *testing.T) {
	profiles := &queryProfileResolverFake{connection: queryConnectionFixture("connection-1", "Primary")}
	runtime := &queryRuntimeFake{
		statement: ports.QueryStatement{Type: dto.QueryStatementSelect, ReadOnly: true},
		executeFn: func(ctx context.Context, _ entity.Connection, _ string, _ ports.QueryRuntimeRequest) (dto.QueryExecutionResult, error) {
			<-ctx.Done()
			return dto.QueryExecutionResult{}, ctx.Err()
		},
	}
	history := &queryHistoryRepositoryFake{}
	subject := service.NewQueryService(profiles, runtime, history, service.QueryServiceOptions{
		MaxRows:        250,
		MaxBytes:       2 * 1024 * 1024,
		DefaultTimeout: 5 * time.Millisecond,
		MaxTimeout:     time.Second,
		HistoryTimeout: time.Second,
	})

	_, err := subject.Execute(context.Background(), dto.QueryExecutionInput{ExecutionID: "execution-timeout", ConnectionID: "connection-1", SQL: "SELECT pg_sleep(10)"})
	if queryApplicationErrorCode(err) != apperror.CodeQueryTimeout {
		t.Fatalf("Execute() error = %#v", err)
	}
	if len(history.created) != 1 || history.created[0].Status != string(dto.QueryExecutionTimedOut) || history.created[0].Error != "query timed out" {
		t.Fatalf("timeout history = %#v", history.created)
	}
	if history.createContextErr != nil || !history.createContextHasDeadline {
		t.Fatalf("history context err=%v deadline=%v", history.createContextErr, history.createContextHasDeadline)
	}
}

func TestQueryServicePersistsCancellationAfterRequestContextIsCancelled(t *testing.T) {
	profiles := &queryProfileResolverFake{connection: queryConnectionFixture("connection-1", "Primary")}
	runtime := &queryRuntimeFake{
		statement: ports.QueryStatement{Type: dto.QueryStatementSelect, ReadOnly: true},
		executeFn: func(ctx context.Context, _ entity.Connection, _ string, _ ports.QueryRuntimeRequest) (dto.QueryExecutionResult, error) {
			return dto.QueryExecutionResult{}, ctx.Err()
		},
	}
	history := &queryHistoryRepositoryFake{}
	subject := newQueryService(profiles, runtime, history)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := subject.Execute(ctx, dto.QueryExecutionInput{ExecutionID: "execution-cancelled", ConnectionID: "connection-1", SQL: "SELECT * FROM events"})
	if queryApplicationErrorCode(err) != apperror.CodeQueryCancelled {
		t.Fatalf("Execute() error = %#v", err)
	}
	if len(history.created) != 1 || history.created[0].Status != string(dto.QueryExecutionCancelled) || history.created[0].Error != "query was cancelled" {
		t.Fatalf("cancelled history = %#v", history.created)
	}
	if history.createContextErr != nil || !history.createContextHasDeadline {
		t.Fatalf("history context err=%v deadline=%v", history.createContextErr, history.createContextHasDeadline)
	}
}

func TestQueryServiceRedactsRawDriverErrors(t *testing.T) {
	profiles := &queryProfileResolverFake{connection: queryConnectionFixture("connection-1", "Primary")}
	runtime := &queryRuntimeFake{
		statement:  ports.QueryStatement{Type: dto.QueryStatementSelect, ReadOnly: true},
		executeErr: errors.New("pq: password=super-secret host=database.internal query=SELECT private_data permission denied"),
	}
	history := &queryHistoryRepositoryFake{}
	subject := newQueryService(profiles, runtime, history)

	_, err := subject.Execute(context.Background(), dto.QueryExecutionInput{ExecutionID: "execution-error", ConnectionID: "connection-1", SQL: "SELECT private_data FROM secrets"})
	if queryApplicationErrorCode(err) != apperror.CodeInternal || err.Error() != "query execution failed" {
		t.Fatalf("Execute() error = %#v", err)
	}
	for _, secret := range []string{"super-secret", "database.internal", "private_data"} {
		if strings.Contains(err.Error(), secret) || strings.Contains(history.created[0].Error, secret) {
			t.Fatalf("exposed driver detail %q in error=%q history=%q", secret, err.Error(), history.created[0].Error)
		}
	}
	if history.created[0].Status != string(dto.QueryExecutionFailed) || history.created[0].Error != "query execution failed" {
		t.Fatalf("error history = %#v", history.created[0])
	}
}

func TestQueryServiceForwardsExplainAnalyzeAndNormalizesIdentity(t *testing.T) {
	profiles := &queryProfileResolverFake{connection: queryConnectionFixture("connection-1", "Primary")}
	plan := &dto.QueryExplainPlan{
		Table: dto.QueryPlanTable{
			Columns: []dto.DataColumn{{Key: "operation", Name: "Operation", LogicalType: dto.LogicalTypeString, ValueEncoding: dto.ValueEncodingNative}},
			Rows:    [][]any{{"Index Scan"}},
		},
		Tree: []dto.QueryPlanNode{{ID: "node-1", Operation: "Index Scan", Relation: "users", Cost: 12.5, ActualTimeMS: 0.8, Rows: 2}},
	}
	runtime := &queryRuntimeFake{
		statement:     ports.QueryStatement{Type: dto.QueryStatementSelect, ReadOnly: true},
		explainResult: dto.QueryExecutionResult{DurationMS: 9, Plan: plan, Notices: []dto.QueryNotice{{Severity: "warning", Message: "statistics are stale"}}},
	}
	history := &queryHistoryRepositoryFake{}
	subject := newQueryService(profiles, runtime, history)

	result, err := subject.Explain(context.Background(), dto.QueryExplainInput{
		ExecutionID:   "execution-explain",
		ConnectionID:  "connection-1",
		SQL:           "SELECT * FROM users WHERE id = 1",
		TransactionID: "transaction-2",
		Analyze:       true,
	})
	if err != nil {
		t.Fatalf("Explain() error = %v", err)
	}
	if runtime.explainCalls != 1 || runtime.lastExplainMode != dto.QueryExplainAnalyze || runtime.lastRequest.TransactionID != "transaction-2" {
		t.Fatalf("Explain() runtime mode=%q request=%#v calls=%d", runtime.lastExplainMode, runtime.lastRequest, runtime.explainCalls)
	}
	if result.ExecutionID != "execution-explain" || result.StatementType != dto.QueryStatementSelect || result.Plan == nil || len(result.Plan.Table.Rows) != 1 || len(result.Plan.Tree) != 1 {
		t.Fatalf("Explain() result = %#v", result)
	}
	if len(history.created) != 1 || history.created[0].Status != string(dto.QueryExecutionSucceeded) || history.created[0].RowCount != 1 {
		t.Fatalf("Explain() history = %#v", history.created)
	}
}

func TestQueryServiceForwardsPlainExplainMode(t *testing.T) {
	profiles := &queryProfileResolverFake{connection: queryConnectionFixture("connection-1", "Primary")}
	runtime := &queryRuntimeFake{
		statement:     ports.QueryStatement{Type: dto.QueryStatementSelect, ReadOnly: true},
		explainResult: dto.QueryExecutionResult{Plan: &dto.QueryExplainPlan{}},
	}
	history := &queryHistoryRepositoryFake{}
	subject := newQueryService(profiles, runtime, history)

	result, err := subject.Explain(context.Background(), dto.QueryExplainInput{ExecutionID: "execution-explain", ConnectionID: "connection-1", SQL: "SELECT 1"})
	if err != nil {
		t.Fatalf("Explain() error = %v", err)
	}
	if runtime.lastExplainMode != dto.QueryExplainOnly || result.Plan == nil || result.Plan.Mode != dto.QueryExplainOnly {
		t.Fatalf("Explain() mode runtime=%q result=%#v", runtime.lastExplainMode, result)
	}
}

func TestQueryServiceRejectsExplainAnalyzeForWriteStatement(t *testing.T) {
	profiles := &queryProfileResolverFake{connection: queryConnectionFixture("connection-1", "Primary")}
	runtime := &queryRuntimeFake{statement: ports.QueryStatement{Type: dto.QueryStatementUpdate, ReadOnly: false}}
	history := &queryHistoryRepositoryFake{}
	subject := newQueryService(profiles, runtime, history)

	_, err := subject.Explain(context.Background(), dto.QueryExplainInput{
		ExecutionID:  "execution-explain-write",
		ConnectionID: "connection-1",
		SQL:          "UPDATE users SET active = false",
		Analyze:      true,
	})
	if queryApplicationErrorCode(err) != apperror.CodeValidation || err.Error() != "query is invalid" {
		t.Fatalf("Explain() error = %#v", err)
	}
	if runtime.explainCalls != 0 {
		t.Fatalf("Explain() runtime calls = %d", runtime.explainCalls)
	}
	if len(history.created) != 1 || history.created[0].Status != string(dto.QueryExecutionFailed) {
		t.Fatalf("Explain() history = %#v", history.created)
	}
}

func TestQueryServiceCancelsExecutionExplicitly(t *testing.T) {
	profiles := &queryProfileResolverFake{}
	runtime := &queryRuntimeFake{}
	history := &queryHistoryRepositoryFake{}
	subject := newQueryService(profiles, runtime, history)

	if err := subject.Cancel(context.Background(), "execution-1"); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if runtime.cancelCalls != 1 || runtime.lastCancelledID != "execution-1" {
		t.Fatalf("Cancel() calls=%d id=%q", runtime.cancelCalls, runtime.lastCancelledID)
	}
	if err := subject.Cancel(context.Background(), " "); queryApplicationErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("Cancel() invalid error = %#v", err)
	}
	if runtime.cancelCalls != 1 {
		t.Fatalf("invalid cancellation reached runtime")
	}
	runtime.cancelErr = apperror.NewNotFound("active query execution was not found", nil)
	if err := subject.Cancel(context.Background(), "execution-missing"); queryApplicationErrorCode(err) != apperror.CodeNotFound || err.Error() != "query execution not found" {
		t.Fatalf("Cancel() missing error = %#v", err)
	}
	runtime.cancelErr = errors.New("driver cancel failed password=super-secret")
	if err := subject.Cancel(context.Background(), "execution-failed"); queryApplicationErrorCode(err) != apperror.CodeInternal || err.Error() != "query cancellation failed" || strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("Cancel() driver error = %#v", err)
	}
}

func TestQueryServiceValidatesAndManagesPersistentHistory(t *testing.T) {
	now := time.Date(2026, time.July, 15, 9, 30, 0, 0, time.UTC)
	repository := &queryHistoryRepositoryFake{listed: []ports.QueryHistory{
		{ID: "execution-1", ConnectionID: "connection-1", SQLText: "SELECT 1", Status: string(dto.QueryExecutionSucceeded), DurationMS: 3, RowCount: 1, ExecutedAt: now},
		{ID: "execution-2", ConnectionID: "connection-1", SQLText: "SELECT secret", Status: string(dto.QueryExecutionFailed), Error: "pq: password=legacy-secret host=database.internal", ExecutedAt: now},
	}}
	subject := newQueryService(&queryProfileResolverFake{}, &queryRuntimeFake{}, repository)

	items, err := subject.ListHistory(context.Background(), "connection-1")
	if err != nil {
		t.Fatalf("ListHistory() error = %v", err)
	}
	if len(items) != 2 || items[0].ID != "execution-1" || items[0].Status != dto.QueryExecutionSucceeded || !items[0].ExecutedAt.Equal(now) {
		t.Fatalf("ListHistory() = %#v", items)
	}
	if items[1].Error != "query execution failed" || strings.Contains(items[1].Error, "legacy-secret") {
		t.Fatalf("ListHistory() exposed stored driver error = %#v", items[1])
	}
	if err := subject.DeleteHistory(context.Background(), dto.QueryHistoryDeleteInput{IDs: []string{"execution-1", "execution-2"}}); err != nil {
		t.Fatalf("DeleteHistory() error = %v", err)
	}
	if len(repository.deletedIDs) != 2 {
		t.Fatalf("DeleteHistory() ids = %#v", repository.deletedIDs)
	}
	if err := subject.ClearHistory(context.Background(), "connection-1"); err != nil {
		t.Fatalf("ClearHistory() error = %v", err)
	}
	if repository.clearedConnectionID != "connection-1" {
		t.Fatalf("ClearHistory() connection = %q", repository.clearedConnectionID)
	}
	if err := subject.ClearHistory(context.Background(), ""); err != nil {
		t.Fatalf("ClearHistory() all error = %v", err)
	}
	if repository.clearedConnectionID != "" {
		t.Fatalf("ClearHistory() all connection = %q", repository.clearedConnectionID)
	}

	invalid := []struct {
		name string
		run  func() error
	}{
		{name: "list connection", run: func() error { _, err := subject.ListHistory(context.Background(), "connection\x00bad"); return err }},
		{name: "clear connection", run: func() error { return subject.ClearHistory(context.Background(), "connection\x00bad") }},
		{name: "delete empty", run: func() error { return subject.DeleteHistory(context.Background(), dto.QueryHistoryDeleteInput{}) }},
		{name: "delete duplicate", run: func() error {
			return subject.DeleteHistory(context.Background(), dto.QueryHistoryDeleteInput{IDs: []string{"same", "same"}})
		}},
		{name: "delete invalid", run: func() error {
			return subject.DeleteHistory(context.Background(), dto.QueryHistoryDeleteInput{IDs: []string{"bad\x00id"}})
		}},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); queryApplicationErrorCode(err) != apperror.CodeValidation {
				t.Fatalf("error = %#v", err)
			}
		})
	}
}

func TestQueryServiceValidatesExecutionBeforeResolvingProfile(t *testing.T) {
	profiles := &queryProfileResolverFake{}
	runtime := &queryRuntimeFake{}
	history := &queryHistoryRepositoryFake{}
	subject := newQueryService(profiles, runtime, history)
	tests := []dto.QueryExecutionInput{
		{ConnectionID: "", SQL: "SELECT 1"},
		{ConnectionID: "connection-1", SQL: ""},
		{ConnectionID: "connection-1", SQL: "SELECT 1", TimeoutSeconds: 61},
		{ExecutionID: "bad\x00id", ConnectionID: "connection-1", SQL: "SELECT 1"},
		{ConnectionID: "connection-1", SQL: "SELECT 1", TransactionID: "bad\x00transaction"},
	}
	for _, input := range tests {
		if _, err := subject.Execute(context.Background(), input); queryApplicationErrorCode(err) != apperror.CodeValidation {
			t.Fatalf("Execute(%#v) error = %#v", input, err)
		}
	}
	if profiles.calls != 0 || runtime.executeCalls != 0 || len(history.created) != 0 {
		t.Fatalf("invalid execution reached dependencies profiles=%d runtime=%d history=%d", profiles.calls, runtime.executeCalls, len(history.created))
	}
}

func newQueryService(profiles ports.ConnectionProfileResolver, runtime ports.QueryRuntime, history ports.QueryHistoryRepository) *service.QueryService {
	return service.NewQueryService(profiles, runtime, history, service.QueryServiceOptions{
		MaxRows:        250,
		MaxBytes:       2 * 1024 * 1024,
		DefaultTimeout: 30 * time.Second,
		MaxTimeout:     60 * time.Second,
		HistoryTimeout: time.Second,
	})
}

type queryProfileResolverFake struct {
	connection entity.Connection
	password   string
	err        error
	calls      int
}

func (resolver *queryProfileResolverFake) Resolve(_ context.Context, _ string) (entity.Connection, string, error) {
	resolver.calls++
	return resolver.connection, resolver.password, resolver.err
}

type queryRuntimeFake struct {
	statement       ports.QueryStatement
	classifyErr     error
	executeResult   dto.QueryExecutionResult
	executeErr      error
	executeFn       func(context.Context, entity.Connection, string, ports.QueryRuntimeRequest) (dto.QueryExecutionResult, error)
	explainResult   dto.QueryExecutionResult
	explainErr      error
	cancelErr       error
	executeCalls    int
	explainCalls    int
	cancelCalls     int
	lastConnection  entity.Connection
	lastPassword    string
	lastRequest     ports.QueryRuntimeRequest
	lastExplainMode dto.QueryExplainMode
	lastCancelledID string
}

func (runtime *queryRuntimeFake) ClassifyStatement(_ entity.Engine, _ string) (ports.QueryStatement, error) {
	return runtime.statement, runtime.classifyErr
}

func (runtime *queryRuntimeFake) Execute(ctx context.Context, connection entity.Connection, password string, request ports.QueryRuntimeRequest) (dto.QueryExecutionResult, error) {
	runtime.executeCalls++
	runtime.lastConnection = connection
	runtime.lastPassword = password
	runtime.lastRequest = request
	if runtime.executeFn != nil {
		return runtime.executeFn(ctx, connection, password, request)
	}
	return runtime.executeResult, runtime.executeErr
}

func (runtime *queryRuntimeFake) Explain(_ context.Context, connection entity.Connection, password string, request ports.QueryRuntimeRequest, mode dto.QueryExplainMode) (dto.QueryExecutionResult, error) {
	runtime.explainCalls++
	runtime.lastConnection = connection
	runtime.lastPassword = password
	runtime.lastRequest = request
	runtime.lastExplainMode = mode
	return runtime.explainResult, runtime.explainErr
}

func (runtime *queryRuntimeFake) Cancel(_ context.Context, executionID string) error {
	runtime.cancelCalls++
	runtime.lastCancelledID = executionID
	return runtime.cancelErr
}

type queryHistoryRepositoryFake struct {
	created                  []ports.QueryHistory
	listed                   []ports.QueryHistory
	createErr                error
	listErr                  error
	deleteErr                error
	clearErr                 error
	deletedIDs               []string
	clearedConnectionID      string
	createContextErr         error
	createContextHasDeadline bool
}

func (repository *queryHistoryRepositoryFake) Create(ctx context.Context, item ports.QueryHistory) (ports.QueryHistory, error) {
	repository.createContextErr = ctx.Err()
	_, repository.createContextHasDeadline = ctx.Deadline()
	repository.created = append(repository.created, item)
	return item, repository.createErr
}

func (repository *queryHistoryRepositoryFake) List(_ context.Context, _ string) ([]ports.QueryHistory, error) {
	return repository.listed, repository.listErr
}

func (repository *queryHistoryRepositoryFake) DeleteByIDs(_ context.Context, ids []string) error {
	repository.deletedIDs = append([]string(nil), ids...)
	return repository.deleteErr
}

func (repository *queryHistoryRepositoryFake) ClearByConnection(_ context.Context, connectionID string) error {
	repository.clearedConnectionID = connectionID
	return repository.clearErr
}

func queryConnectionFixture(id, name string) entity.Connection {
	now := time.Date(2026, time.July, 15, 8, 0, 0, 0, time.UTC)
	return entity.Connection{
		ID:              id,
		WorkspaceID:     "workspace-1",
		Name:            name,
		Engine:          entity.EnginePostgreSQL,
		Host:            "database.internal",
		Port:            5432,
		Database:        "app",
		Username:        "app",
		SSLMode:         entity.SSLModeDisable,
		AutoReconnect:   true,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 1800,
		ConnMaxIdleTime: 300,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func queryApplicationErrorCode(err error) string {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}
