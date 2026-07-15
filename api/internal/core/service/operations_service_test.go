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

func TestOperationsServiceDashboardUsesValidatedDefaultRangeAndLiveProfile(t *testing.T) {
	connection := operationsConnectionFixture()
	profiles := &operationsProfileResolverFake{connection: connection, password: "database-secret"}
	runtime := &operationsRuntimeFake{dashboard: dto.OperationsDashboard{Available: true, Version: "17.5", Metrics: []dto.OperationsMetric{{Key: "connections", Label: "Connections", Value: 42, Unit: "sessions"}}}}
	subject := service.NewOperationsService(profiles, runtime)

	result, err := subject.Dashboard(context.Background(), connection.ID, dto.OperationsRangeInput{})
	if err != nil {
		t.Fatalf("Dashboard() error = %v", err)
	}
	if !result.Available || result.Engine != entity.EnginePostgreSQL || result.ConnectionID != connection.ID || result.Window != dto.OperationsWindow15Minutes || result.CollectedAt.IsZero() || len(result.Metrics) != 1 {
		t.Fatalf("dashboard = %#v", result)
	}
	if runtime.dashboardCalls != 1 || runtime.lastRange.Window != dto.OperationsWindow15Minutes || runtime.lastRange.Points != 60 || runtime.lastConnection.ID != connection.ID || runtime.lastPassword != "database-secret" {
		t.Fatalf("runtime request=%#v connection=%#v password=%q calls=%d", runtime.lastRange, runtime.lastConnection, runtime.lastPassword, runtime.dashboardCalls)
	}
}

func TestOperationsServiceRejectsInvalidRangesBeforeResolvingProfile(t *testing.T) {
	profiles := &operationsProfileResolverFake{connection: operationsConnectionFixture()}
	runtime := &operationsRuntimeFake{}
	subject := service.NewOperationsService(profiles, runtime)
	invalid := []dto.OperationsRangeInput{
		{Window: "2h", Points: 60},
		{Window: dto.OperationsWindow1Hour, Points: 9},
		{Window: dto.OperationsWindow1Hour, Points: 361},
	}
	for index, input := range invalid {
		if _, err := subject.Dashboard(context.Background(), "connection-1", input); operationsApplicationErrorCode(err) != apperror.CodeValidation {
			t.Fatalf("Dashboard() invalid[%d] error = %#v", index, err)
		}
	}
	if profiles.calls != 0 || runtime.dashboardCalls != 0 {
		t.Fatalf("invalid range reached dependencies profiles=%d runtime=%d", profiles.calls, runtime.dashboardCalls)
	}
}

func TestOperationsServiceReturnsTruthfulUnavailableFeedsForUnsupportedAndPermissionErrors(t *testing.T) {
	connection := operationsConnectionFixture()
	runtime := &operationsRuntimeFake{
		dashboardErr: apperror.NewUnsupported("pg_stat_statements is missing", nil),
		sessionsErr:  apperror.NewPermission("pg_stat_activity denied", errors.New("role secret_monitor")),
		locksErr:     apperror.NewUnsupported("performance_schema is disabled", nil),
		performanceErr: apperror.NewPermission(
			"performance schema denied",
			errors.New("driver SQLSTATE 42501"),
		),
	}
	subject := service.NewOperationsService(&operationsProfileResolverFake{connection: connection}, runtime)

	dashboard, dashboardErr := subject.Dashboard(context.Background(), connection.ID, dto.OperationsRangeInput{})
	sessions, sessionsErr := subject.Sessions(context.Background(), connection.ID, dto.OperationsSessionsInput{})
	locks, locksErr := subject.Locks(context.Background(), connection.ID, dto.OperationsLocksInput{})
	performance, performanceErr := subject.Performance(context.Background(), connection.ID, dto.OperationsPerformanceInput{})
	if dashboardErr != nil || sessionsErr != nil || locksErr != nil || performanceErr != nil {
		t.Fatalf("unavailable errors dashboard=%v sessions=%v locks=%v performance=%v", dashboardErr, sessionsErr, locksErr, performanceErr)
	}
	if dashboard.Available || len(dashboard.Metrics) != 0 || dashboard.Message != "dashboard metrics are unavailable for this database engine" {
		t.Fatalf("dashboard fallback = %#v", dashboard)
	}
	if sessions.Available || len(sessions.Items) != 0 || sessions.Message != "session inspection is unavailable for this database user" {
		t.Fatalf("sessions fallback = %#v", sessions)
	}
	if locks.Available || len(locks.Items) != 0 || len(locks.BlockingChains) != 0 || locks.Message != "lock inspection is unavailable for this database engine" {
		t.Fatalf("locks fallback = %#v", locks)
	}
	if performance.Available || len(performance.Metrics) != 0 || len(performance.SlowQueries) != 0 || performance.Message != "performance metrics are unavailable for this database user" {
		t.Fatalf("performance fallback = %#v", performance)
	}
	if strings.Contains(sessions.Message, "secret_monitor") || strings.Contains(performance.Message, "SQLSTATE") {
		t.Fatalf("unavailable response leaked driver details")
	}
}

func TestOperationsServiceReturnsSessionsLocksAndPerformanceWithoutMockEnrichment(t *testing.T) {
	connection := operationsConnectionFixture()
	now := time.Now().UTC()
	runtime := &operationsRuntimeFake{
		sessions:    dto.OperationsSessions{Available: true, Items: []dto.OperationsSession{{ID: "18421", User: "api_service", State: "active", Query: "SELECT 1"}}},
		locks:       dto.OperationsLocks{Available: true, Items: []dto.OperationsLock{{ID: "lock-1", Type: "relation", Granted: false, WaitingSessionID: "18421", BlockingSessionID: "18409"}}, BlockingChains: []dto.OperationsBlockingChain{{WaitingSessionID: "18421", BlockingSessionIDs: []string{"18409"}}}},
		performance: dto.OperationsPerformance{Available: true, Metrics: []dto.OperationsMetric{{Key: "query_duration", Label: "Query duration", Value: 12.5, Unit: "ms", Series: []dto.OperationsMetricPoint{{Timestamp: now, Value: 12.5}}}}, SlowQueries: []dto.OperationsSlowQuery{{Fingerprint: "abc", Query: "SELECT pg_sleep($1)", Calls: 3, TotalMS: 3000, MeanMS: 1000, Rows: 3}}},
	}
	subject := service.NewOperationsService(&operationsProfileResolverFake{connection: connection}, runtime)

	sessions, err := subject.Sessions(context.Background(), connection.ID, dto.OperationsSessionsInput{Limit: 50, State: dto.OperationsSessionStateActive})
	if err != nil {
		t.Fatalf("Sessions() error = %v", err)
	}
	locks, err := subject.Locks(context.Background(), connection.ID, dto.OperationsLocksInput{Limit: 75})
	if err != nil {
		t.Fatalf("Locks() error = %v", err)
	}
	performance, err := subject.Performance(context.Background(), connection.ID, dto.OperationsPerformanceInput{Range: dto.OperationsRangeInput{Window: dto.OperationsWindow1Hour, Points: 120}, Limit: 25})
	if err != nil {
		t.Fatalf("Performance() error = %v", err)
	}
	if len(sessions.Items) != 1 || len(locks.Items) != 1 || len(locks.BlockingChains) != 1 || len(performance.Metrics) != 1 || len(performance.SlowQueries) != 1 {
		t.Fatalf("sessions=%#v locks=%#v performance=%#v", sessions, locks, performance)
	}
	if runtime.lastSessions.Limit != 50 || runtime.lastSessions.State != dto.OperationsSessionStateActive || runtime.lastLocks.Limit != 75 || runtime.lastPerformance.Range.Window != dto.OperationsWindow1Hour || runtime.lastPerformance.Range.Points != 120 || runtime.lastPerformance.Limit != 25 {
		t.Fatalf("runtime sessions=%#v locks=%#v performance=%#v", runtime.lastSessions, runtime.lastLocks, runtime.lastPerformance)
	}
}

func TestOperationsServiceDistinguishesCancelAndTerminateSession(t *testing.T) {
	connection := operationsConnectionFixture()
	runtime := &operationsRuntimeFake{}
	subject := service.NewOperationsService(&operationsProfileResolverFake{connection: connection}, runtime)

	cancelled, err := subject.ControlSession(context.Background(), connection.ID, dto.OperationsSessionControlInput{SessionID: "18421", Action: dto.OperationsSessionCancel})
	if err != nil {
		t.Fatalf("ControlSession(cancel) error = %v", err)
	}
	terminated, err := subject.ControlSession(context.Background(), connection.ID, dto.OperationsSessionControlInput{SessionID: "18409", Action: dto.OperationsSessionTerminate})
	if err != nil {
		t.Fatalf("ControlSession(terminate) error = %v", err)
	}
	if len(runtime.controlRequests) != 2 || runtime.controlRequests[0].Action != dto.OperationsSessionCancel || runtime.controlRequests[1].Action != dto.OperationsSessionTerminate {
		t.Fatalf("control requests = %#v", runtime.controlRequests)
	}
	if cancelled.SessionID != "18421" || cancelled.Action != dto.OperationsSessionCancel || cancelled.AcceptedAt.IsZero() || terminated.SessionID != "18409" || terminated.Action != dto.OperationsSessionTerminate || terminated.AcceptedAt.IsZero() {
		t.Fatalf("cancelled=%#v terminated=%#v", cancelled, terminated)
	}
}

func TestOperationsServiceMapsSessionControlPermissionAndUnsupportedErrors(t *testing.T) {
	connection := operationsConnectionFixture()
	runtime := &operationsRuntimeFake{controlErrors: []error{
		apperror.NewPermission("pg_terminate_backend denied for role private_role", errors.New("SQLSTATE 42501")),
		apperror.NewUnsupported("KILL QUERY is unavailable", nil),
	}}
	subject := service.NewOperationsService(&operationsProfileResolverFake{connection: connection}, runtime)

	_, permissionErr := subject.ControlSession(context.Background(), connection.ID, dto.OperationsSessionControlInput{SessionID: "18421", Action: dto.OperationsSessionTerminate})
	_, unsupportedErr := subject.ControlSession(context.Background(), connection.ID, dto.OperationsSessionControlInput{SessionID: "18422", Action: dto.OperationsSessionCancel})
	var permissionApplicationError *apperror.Error
	var unsupportedApplicationError *apperror.Error
	if !errors.As(permissionErr, &permissionApplicationError) || permissionApplicationError.Code != apperror.CodePermissionDenied || permissionApplicationError.Message != "session control permission denied" || strings.Contains(permissionApplicationError.Message, "private_role") {
		t.Fatalf("permission error = %#v", permissionErr)
	}
	if !errors.As(unsupportedErr, &unsupportedApplicationError) || unsupportedApplicationError.Code != apperror.CodeUnsupportedCapability || unsupportedApplicationError.Message != "session control is unsupported" {
		t.Fatalf("unsupported error = %#v", unsupportedErr)
	}
}

func TestOperationsServiceValidatesSessionAndListRequests(t *testing.T) {
	connection := operationsConnectionFixture()
	profiles := &operationsProfileResolverFake{connection: connection}
	runtime := &operationsRuntimeFake{}
	subject := service.NewOperationsService(profiles, runtime)

	if _, err := subject.Sessions(context.Background(), connection.ID, dto.OperationsSessionsInput{Limit: 501}); operationsApplicationErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("Sessions() error = %#v", err)
	}
	if _, err := subject.Locks(context.Background(), connection.ID, dto.OperationsLocksInput{Limit: -1}); operationsApplicationErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("Locks() error = %#v", err)
	}
	if _, err := subject.Performance(context.Background(), connection.ID, dto.OperationsPerformanceInput{Range: dto.OperationsRangeInput{Window: dto.OperationsWindow24Hours, Points: 60}, Limit: 101}); operationsApplicationErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("Performance() error = %#v", err)
	}
	if _, err := subject.ControlSession(context.Background(), connection.ID, dto.OperationsSessionControlInput{SessionID: "bad\nidentifier", Action: dto.OperationsSessionCancel}); operationsApplicationErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("ControlSession() invalid identifier error = %#v", err)
	}
	if _, err := subject.ControlSession(context.Background(), connection.ID, dto.OperationsSessionControlInput{SessionID: "1 OR 1=1", Action: dto.OperationsSessionCancel}); operationsApplicationErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("ControlSession() unsafe identifier error = %#v", err)
	}
	if _, err := subject.ControlSession(context.Background(), connection.ID, dto.OperationsSessionControlInput{SessionID: "18421", Action: "pause"}); operationsApplicationErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("ControlSession() invalid action error = %#v", err)
	}
	if profiles.calls != 0 || runtime.sessionsCalls != 0 || runtime.locksCalls != 0 || runtime.performanceCalls != 0 || len(runtime.controlRequests) != 0 {
		t.Fatalf("invalid request reached dependencies")
	}
}

type operationsProfileResolverFake struct {
	connection entity.Connection
	password   string
	err        error
	calls      int
}

func (fake *operationsProfileResolverFake) Resolve(context.Context, string) (entity.Connection, string, error) {
	fake.calls++
	return fake.connection, fake.password, fake.err
}

type operationsRuntimeFake struct {
	dashboard        dto.OperationsDashboard
	sessions         dto.OperationsSessions
	locks            dto.OperationsLocks
	performance      dto.OperationsPerformance
	dashboardErr     error
	sessionsErr      error
	locksErr         error
	performanceErr   error
	controlErrors    []error
	dashboardCalls   int
	sessionsCalls    int
	locksCalls       int
	performanceCalls int
	controlCalls     int
	lastConnection   entity.Connection
	lastPassword     string
	lastRange        dto.OperationsRangeInput
	lastSessions     dto.OperationsSessionsInput
	lastLocks        dto.OperationsLocksInput
	lastPerformance  dto.OperationsPerformanceInput
	controlRequests  []dto.OperationsSessionControlInput
}

func (fake *operationsRuntimeFake) Dashboard(_ context.Context, connection entity.Connection, password string, input dto.OperationsRangeInput) (dto.OperationsDashboard, error) {
	fake.dashboardCalls++
	fake.lastConnection = connection
	fake.lastPassword = password
	fake.lastRange = input
	return fake.dashboard, fake.dashboardErr
}

func (fake *operationsRuntimeFake) Sessions(_ context.Context, connection entity.Connection, password string, input dto.OperationsSessionsInput) (dto.OperationsSessions, error) {
	fake.sessionsCalls++
	fake.lastConnection = connection
	fake.lastPassword = password
	fake.lastSessions = input
	return fake.sessions, fake.sessionsErr
}

func (fake *operationsRuntimeFake) Locks(_ context.Context, connection entity.Connection, password string, input dto.OperationsLocksInput) (dto.OperationsLocks, error) {
	fake.locksCalls++
	fake.lastConnection = connection
	fake.lastPassword = password
	fake.lastLocks = input
	return fake.locks, fake.locksErr
}

func (fake *operationsRuntimeFake) Performance(_ context.Context, connection entity.Connection, password string, input dto.OperationsPerformanceInput) (dto.OperationsPerformance, error) {
	fake.performanceCalls++
	fake.lastConnection = connection
	fake.lastPassword = password
	fake.lastPerformance = input
	return fake.performance, fake.performanceErr
}

func (fake *operationsRuntimeFake) ControlSession(_ context.Context, connection entity.Connection, password string, input dto.OperationsSessionControlInput) error {
	fake.controlCalls++
	fake.lastConnection = connection
	fake.lastPassword = password
	fake.controlRequests = append(fake.controlRequests, input)
	if len(fake.controlErrors) >= fake.controlCalls {
		return fake.controlErrors[fake.controlCalls-1]
	}
	return nil
}

func operationsConnectionFixture() entity.Connection {
	return entity.Connection{ID: "connection-1", Name: "Primary", Engine: entity.EnginePostgreSQL, Host: "database", Port: 5432, Database: "datadock", Username: "datadock"}
}

func operationsApplicationErrorCode(err error) string {
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return applicationError.Code
	}
	return ""
}

var _ ports.OperationsRuntime = (*operationsRuntimeFake)(nil)
