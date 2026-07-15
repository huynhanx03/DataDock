package service

import (
	"context"
	"errors"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/ports"
)

type OperationsService struct {
	profiles ports.ConnectionProfileResolver
	runtime  ports.OperationsRuntime
}

func NewOperationsService(profiles ports.ConnectionProfileResolver, runtime ports.OperationsRuntime) *OperationsService {
	return &OperationsService{profiles: profiles, runtime: runtime}
}

func (service *OperationsService) Dashboard(ctx context.Context, connectionID string, input dto.OperationsRangeInput) (dto.OperationsDashboard, error) {
	rangeInput, err := normalizeOperationsRange(connectionID, input)
	if err != nil {
		return dto.OperationsDashboard{}, err
	}
	connection, password, err := service.profiles.Resolve(ctx, connectionID)
	if err != nil {
		return dto.OperationsDashboard{}, serviceError(err)
	}
	result, err := service.runtime.Dashboard(ctx, connection, password, rangeInput)
	if message, unavailable := operationsUnavailable(err, "dashboard"); unavailable {
		result = dto.OperationsDashboard{Message: message}
	} else if err != nil {
		return dto.OperationsDashboard{}, operationsRuntimeError(err, "operations dashboard")
	}
	result.ConnectionID = connection.ID
	result.Engine = connection.Engine
	result.Window = rangeInput.Window
	if result.CollectedAt.IsZero() {
		result.CollectedAt = time.Now().UTC()
	}
	if !result.Available {
		result.Metrics = []dto.OperationsMetric{}
		if result.Message == "" {
			result.Message = "dashboard metrics are currently unavailable"
		}
	} else if result.Metrics == nil {
		result.Metrics = []dto.OperationsMetric{}
	}
	return result, nil
}

func (service *OperationsService) Sessions(ctx context.Context, connectionID string, input dto.OperationsSessionsInput) (dto.OperationsSessions, error) {
	input, err := normalizeOperationsSessionsInput(connectionID, input)
	if err != nil {
		return dto.OperationsSessions{}, err
	}
	connection, password, err := service.profiles.Resolve(ctx, connectionID)
	if err != nil {
		return dto.OperationsSessions{}, serviceError(err)
	}
	result, err := service.runtime.Sessions(ctx, connection, password, input)
	if message, unavailable := operationsUnavailable(err, "sessions"); unavailable {
		result = dto.OperationsSessions{Message: message}
	} else if err != nil {
		return dto.OperationsSessions{}, operationsRuntimeError(err, "session inspection")
	}
	result.ConnectionID = connection.ID
	result.Engine = connection.Engine
	if result.CollectedAt.IsZero() {
		result.CollectedAt = time.Now().UTC()
	}
	if !result.Available {
		result.Items = []dto.OperationsSession{}
		if result.Message == "" {
			result.Message = "session inspection is currently unavailable"
		}
	} else if result.Items == nil {
		result.Items = []dto.OperationsSession{}
	}
	return result, nil
}

func (service *OperationsService) Locks(ctx context.Context, connectionID string, input dto.OperationsLocksInput) (dto.OperationsLocks, error) {
	input, err := normalizeOperationsLocksInput(connectionID, input)
	if err != nil {
		return dto.OperationsLocks{}, err
	}
	connection, password, err := service.profiles.Resolve(ctx, connectionID)
	if err != nil {
		return dto.OperationsLocks{}, serviceError(err)
	}
	result, err := service.runtime.Locks(ctx, connection, password, input)
	if message, unavailable := operationsUnavailable(err, "locks"); unavailable {
		result = dto.OperationsLocks{Message: message}
	} else if err != nil {
		return dto.OperationsLocks{}, operationsRuntimeError(err, "lock inspection")
	}
	result.ConnectionID = connection.ID
	result.Engine = connection.Engine
	if result.CollectedAt.IsZero() {
		result.CollectedAt = time.Now().UTC()
	}
	if !result.Available {
		result.Items = []dto.OperationsLock{}
		result.BlockingChains = []dto.OperationsBlockingChain{}
		if result.Message == "" {
			result.Message = "lock inspection is currently unavailable"
		}
	} else {
		if result.Items == nil {
			result.Items = []dto.OperationsLock{}
		}
		if result.BlockingChains == nil {
			result.BlockingChains = []dto.OperationsBlockingChain{}
		}
	}
	return result, nil
}

func (service *OperationsService) Performance(ctx context.Context, connectionID string, input dto.OperationsPerformanceInput) (dto.OperationsPerformance, error) {
	input, err := normalizeOperationsPerformanceInput(connectionID, input)
	if err != nil {
		return dto.OperationsPerformance{}, err
	}
	connection, password, err := service.profiles.Resolve(ctx, connectionID)
	if err != nil {
		return dto.OperationsPerformance{}, serviceError(err)
	}
	result, err := service.runtime.Performance(ctx, connection, password, input)
	if message, unavailable := operationsUnavailable(err, "performance"); unavailable {
		result = dto.OperationsPerformance{Message: message}
	} else if err != nil {
		return dto.OperationsPerformance{}, operationsRuntimeError(err, "performance inspection")
	}
	result.ConnectionID = connection.ID
	result.Engine = connection.Engine
	result.Window = input.Range.Window
	if result.CollectedAt.IsZero() {
		result.CollectedAt = time.Now().UTC()
	}
	if !result.Available {
		result.Metrics = []dto.OperationsMetric{}
		result.SlowQueries = []dto.OperationsSlowQuery{}
		if result.Message == "" {
			result.Message = "performance metrics are currently unavailable"
		}
	} else {
		if result.Metrics == nil {
			result.Metrics = []dto.OperationsMetric{}
		}
		if result.SlowQueries == nil {
			result.SlowQueries = []dto.OperationsSlowQuery{}
		}
	}
	return result, nil
}

func (service *OperationsService) ControlSession(ctx context.Context, connectionID string, input dto.OperationsSessionControlInput) (dto.OperationsSessionControlResult, error) {
	if !validQueryID(connectionID, false) || !validOperationsSessionID(input.SessionID) || input.Action != dto.OperationsSessionCancel && input.Action != dto.OperationsSessionTerminate {
		return dto.OperationsSessionControlResult{}, apperror.NewValidation("invalid session control request", nil)
	}
	connection, password, err := service.profiles.Resolve(ctx, connectionID)
	if err != nil {
		return dto.OperationsSessionControlResult{}, serviceError(err)
	}
	if err := service.runtime.ControlSession(ctx, connection, password, input); err != nil {
		return dto.OperationsSessionControlResult{}, operationsRuntimeError(err, "session control")
	}
	return dto.OperationsSessionControlResult{SessionID: input.SessionID, Action: input.Action, AcceptedAt: time.Now().UTC()}, nil
}

func validOperationsSessionID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character != '-' && character != '_' && character != ':' && character != '.' && (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func normalizeOperationsRange(connectionID string, input dto.OperationsRangeInput) (dto.OperationsRangeInput, error) {
	if !validQueryID(connectionID, false) {
		return dto.OperationsRangeInput{}, apperror.NewValidation("invalid operations request", nil)
	}
	if input.Window == "" {
		input.Window = dto.OperationsWindow15Minutes
	}
	if input.Points == 0 {
		input.Points = 60
	}
	if !validOperationsWindow(input.Window) || input.Points < 10 || input.Points > 360 {
		return dto.OperationsRangeInput{}, apperror.NewValidation("invalid operations range", nil)
	}
	return input, nil
}

func normalizeOperationsSessionsInput(connectionID string, input dto.OperationsSessionsInput) (dto.OperationsSessionsInput, error) {
	if !validQueryID(connectionID, false) {
		return dto.OperationsSessionsInput{}, apperror.NewValidation("invalid session list request", nil)
	}
	if input.Limit == 0 {
		input.Limit = 100
	}
	if input.State == "" {
		input.State = dto.OperationsSessionStateAll
	}
	if input.Limit < 1 || input.Limit > 500 || !validOperationsSessionState(input.State) {
		return dto.OperationsSessionsInput{}, apperror.NewValidation("invalid session list request", nil)
	}
	return input, nil
}

func normalizeOperationsLocksInput(connectionID string, input dto.OperationsLocksInput) (dto.OperationsLocksInput, error) {
	if !validQueryID(connectionID, false) {
		return dto.OperationsLocksInput{}, apperror.NewValidation("invalid lock list request", nil)
	}
	if input.Limit == 0 {
		input.Limit = 100
	}
	if input.Limit < 1 || input.Limit > 500 {
		return dto.OperationsLocksInput{}, apperror.NewValidation("invalid lock list request", nil)
	}
	return input, nil
}

func normalizeOperationsPerformanceInput(connectionID string, input dto.OperationsPerformanceInput) (dto.OperationsPerformanceInput, error) {
	rangeInput, err := normalizeOperationsRange(connectionID, input.Range)
	if err != nil {
		return dto.OperationsPerformanceInput{}, err
	}
	input.Range = rangeInput
	if input.Limit == 0 {
		input.Limit = 50
	}
	if input.Limit < 1 || input.Limit > 100 {
		return dto.OperationsPerformanceInput{}, apperror.NewValidation("invalid performance request", nil)
	}
	return input, nil
}

func validOperationsWindow(value dto.OperationsWindow) bool {
	switch value {
	case dto.OperationsWindow5Minutes, dto.OperationsWindow15Minutes, dto.OperationsWindow1Hour, dto.OperationsWindow6Hours, dto.OperationsWindow24Hours:
		return true
	default:
		return false
	}
}

func validOperationsSessionState(value dto.OperationsSessionState) bool {
	switch value {
	case dto.OperationsSessionStateAll, dto.OperationsSessionStateActive, dto.OperationsSessionStateIdle, dto.OperationsSessionStateIdleInTransaction:
		return true
	default:
		return false
	}
}

func operationsUnavailable(err error, capability string) (string, bool) {
	if err == nil {
		return "", false
	}
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) {
		return "", false
	}
	suffix := ""
	if applicationError.Code == apperror.CodeUnsupportedCapability {
		suffix = " for this database engine"
	} else if applicationError.Code == apperror.CodePermissionDenied {
		suffix = " for this database user"
	} else {
		return "", false
	}
	switch capability {
	case "dashboard":
		return "dashboard metrics are unavailable" + suffix, true
	case "sessions":
		return "session inspection is unavailable" + suffix, true
	case "locks":
		return "lock inspection is unavailable" + suffix, true
	default:
		return "performance metrics are unavailable" + suffix, true
	}
}

func operationsRuntimeError(err error, operation string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return apperror.NewCancellation(operation+" was cancelled", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apperror.NewTimeout(operation+" timed out", err)
	}
	if errors.Is(err, ports.ErrNotFound) {
		return apperror.NewNotFound("operations resource not found", err)
	}
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) {
		return apperror.NewInternal(operation+" failed", err)
	}
	messages := map[string]string{
		apperror.CodeValidation:            operation + " request is invalid",
		apperror.CodeNotFound:              "operations resource not found",
		apperror.CodeConflict:              operation + " conflict",
		apperror.CodeConnectionFailed:      "database connection failed",
		apperror.CodeConnectionRequired:    "database connection is not connected",
		apperror.CodeConnectionBusy:        "database connection is busy",
		apperror.CodeQueryCancelled:        operation + " was cancelled",
		apperror.CodeQueryTimeout:          operation + " timed out",
		apperror.CodeReadonlyViolation:     operation + " is disabled for this read-only connection",
		apperror.CodePermissionDenied:      operation + " permission denied",
		apperror.CodeUnsupportedCapability: operation + " is unsupported",
		apperror.CodeTransportFailed:       "database transport failed",
		apperror.CodeInternal:              operation + " failed",
	}
	message, exists := messages[applicationError.Code]
	if !exists {
		return apperror.NewInternal(operation+" failed", err)
	}
	return apperror.New(applicationError.Code, message, err, applicationError.Temporary)
}
