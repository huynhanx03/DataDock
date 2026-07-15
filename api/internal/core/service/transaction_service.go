package service

import (
	"context"
	"errors"
	"strings"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/ports"
)

type TransactionService struct {
	profiles ports.ConnectionProfileResolver
	runtime  ports.TransactionRuntime
}

type transactionAction func(context.Context, ports.TransactionRuntimeAction) (dto.TransactionView, error)

func NewTransactionService(profiles ports.ConnectionProfileResolver, runtime ports.TransactionRuntime) *TransactionService {
	return &TransactionService{profiles: profiles, runtime: runtime}
}

func (service *TransactionService) Begin(ctx context.Context, input dto.TransactionBeginInput) (dto.TransactionView, error) {
	if !validServiceIdentifier(input.ConnectionID, false) {
		return dto.TransactionView{}, apperror.NewValidation("invalid transaction request", nil)
	}
	connection, password, err := service.profiles.Resolve(ctx, input.ConnectionID)
	if err != nil {
		return dto.TransactionView{}, serviceError(err)
	}
	if connection.ID != input.ConnectionID {
		return dto.TransactionView{}, apperror.NewInternal("connection resolution failed", nil)
	}
	view, err := service.runtime.Begin(ctx, connection, password)
	if err != nil {
		return dto.TransactionView{}, transactionServiceError(err)
	}
	if err := validateTransactionRuntimeView(view, "", input.ConnectionID); err != nil {
		return dto.TransactionView{}, err
	}
	if view.State != dto.TransactionActive {
		return dto.TransactionView{}, apperror.NewInternal("transaction runtime returned an invalid state", nil)
	}
	return view, nil
}

func (service *TransactionService) Get(ctx context.Context, input dto.TransactionActionInput) (dto.TransactionView, error) {
	if err := validateTransactionActionInput(input); err != nil {
		return dto.TransactionView{}, err
	}
	if _, _, err := service.resolveOwnedConnection(ctx, input.ConnectionID); err != nil {
		return dto.TransactionView{}, err
	}
	view, err := service.runtime.Get(ctx, input.TransactionID)
	if err != nil {
		return dto.TransactionView{}, transactionServiceError(err)
	}
	if view.State == dto.TransactionExpired {
		return dto.TransactionView{}, apperror.NewTransactionExpired("transaction expired", nil)
	}
	if view.ConnectionID != input.ConnectionID {
		return dto.TransactionView{}, apperror.NewConflict("transaction belongs to another connection", nil)
	}
	if err := validateTransactionRuntimeView(view, input.TransactionID, ""); err != nil {
		return dto.TransactionView{}, err
	}
	return view, nil
}

func (service *TransactionService) Commit(ctx context.Context, input dto.TransactionActionInput) (dto.TransactionView, error) {
	return service.runAction(ctx, input, "", dto.TransactionCommitted, service.runtime.Commit)
}

func (service *TransactionService) Rollback(ctx context.Context, input dto.TransactionActionInput) (dto.TransactionView, error) {
	return service.runAction(ctx, input, "", dto.TransactionRolledBack, service.runtime.Rollback)
}

func (service *TransactionService) Savepoint(ctx context.Context, input dto.TransactionSavepointInput) (dto.TransactionView, error) {
	return service.runSavepointAction(ctx, input, service.runtime.Savepoint)
}

func (service *TransactionService) RollbackTo(ctx context.Context, input dto.TransactionSavepointInput) (dto.TransactionView, error) {
	return service.runSavepointAction(ctx, input, service.runtime.RollbackTo)
}

func (service *TransactionService) Release(ctx context.Context, input dto.TransactionSavepointInput) (dto.TransactionView, error) {
	return service.runSavepointAction(ctx, input, service.runtime.Release)
}

func (service *TransactionService) runSavepointAction(ctx context.Context, input dto.TransactionSavepointInput, action transactionAction) (dto.TransactionView, error) {
	if !validSavepointIdentifier(input.Name) {
		return dto.TransactionView{}, apperror.NewValidation("invalid transaction savepoint", nil)
	}
	return service.runAction(ctx, dto.TransactionActionInput{ConnectionID: input.ConnectionID, TransactionID: input.TransactionID}, input.Name, dto.TransactionActive, action)
}

func (service *TransactionService) runAction(ctx context.Context, input dto.TransactionActionInput, savepoint string, expectedState dto.TransactionLifecycleState, action transactionAction) (dto.TransactionView, error) {
	if err := validateTransactionActionInput(input); err != nil {
		return dto.TransactionView{}, err
	}
	if _, _, err := service.resolveOwnedConnection(ctx, input.ConnectionID); err != nil {
		return dto.TransactionView{}, err
	}
	current, err := service.runtime.Get(ctx, input.TransactionID)
	if err != nil {
		return dto.TransactionView{}, transactionServiceError(err)
	}
	if current.State == dto.TransactionExpired {
		return dto.TransactionView{}, apperror.NewTransactionExpired("transaction expired", nil)
	}
	if current.ConnectionID != input.ConnectionID {
		return dto.TransactionView{}, apperror.NewConflict("transaction belongs to another connection", nil)
	}
	if err := validateTransactionRuntimeView(current, input.TransactionID, ""); err != nil {
		return dto.TransactionView{}, err
	}
	if current.State != dto.TransactionActive {
		return dto.TransactionView{}, apperror.NewConflict("transaction is not active", nil)
	}
	view, err := action(ctx, ports.TransactionRuntimeAction{
		ConnectionID:  input.ConnectionID,
		TransactionID: input.TransactionID,
		Savepoint:     savepoint,
	})
	if err != nil {
		return dto.TransactionView{}, transactionServiceError(err)
	}
	if view.State == dto.TransactionExpired {
		return dto.TransactionView{}, apperror.NewTransactionExpired("transaction expired", nil)
	}
	if err := validateTransactionRuntimeView(view, input.TransactionID, input.ConnectionID); err != nil {
		return dto.TransactionView{}, err
	}
	if view.State != expectedState {
		return dto.TransactionView{}, apperror.NewInternal("transaction runtime returned an invalid state", nil)
	}
	return view, nil
}

func (service *TransactionService) resolveOwnedConnection(ctx context.Context, connectionID string) (string, string, error) {
	connection, password, err := service.profiles.Resolve(ctx, connectionID)
	if err != nil {
		return "", "", serviceError(err)
	}
	if connection.ID != connectionID {
		return "", "", apperror.NewInternal("connection resolution failed", nil)
	}
	return connection.ID, password, nil
}

func validateTransactionActionInput(input dto.TransactionActionInput) error {
	if !validServiceIdentifier(input.ConnectionID, false) || !validServiceIdentifier(input.TransactionID, false) {
		return apperror.NewValidation("invalid transaction request", nil)
	}
	return nil
}

func validateTransactionRuntimeView(view dto.TransactionView, transactionID, connectionID string) error {
	if !validServiceIdentifier(view.ID, false) || !validServiceIdentifier(view.ConnectionID, false) || transactionID != "" && view.ID != transactionID || connectionID != "" && view.ConnectionID != connectionID || view.StartedAt.IsZero() || view.LastActivityAt.IsZero() || view.ExpiresAt.IsZero() || view.LastActivityAt.Before(view.StartedAt) {
		return apperror.NewInternal("transaction runtime returned invalid metadata", nil)
	}
	switch view.State {
	case dto.TransactionActive:
		if !view.ExpiresAt.After(view.LastActivityAt) || view.CompletedAt != nil {
			return apperror.NewInternal("transaction runtime returned invalid metadata", nil)
		}
	case dto.TransactionCommitted, dto.TransactionRolledBack:
		if view.CompletedAt == nil || view.CompletedAt.Before(view.StartedAt) {
			return apperror.NewInternal("transaction runtime returned invalid metadata", nil)
		}
	default:
		return apperror.NewInternal("transaction runtime returned invalid metadata", nil)
	}
	for _, savepoint := range view.Savepoints {
		if !validSavepointIdentifier(savepoint.Name) || savepoint.CreatedAt.IsZero() || savepoint.CreatedAt.Before(view.StartedAt) {
			return apperror.NewInternal("transaction runtime returned invalid metadata", nil)
		}
	}
	return nil
}

func validSavepointIdentifier(value string) bool {
	if value == "" || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for index, character := range value {
		letter := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
		if index == 0 {
			if !letter && character != '_' {
				return false
			}
			continue
		}
		if !letter && !(character >= '0' && character <= '9') && character != '_' {
			return false
		}
	}
	return true
}

func transactionServiceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ports.ErrTransactionExpired) || errors.Is(err, ports.ErrNotFound) {
		return apperror.NewTransactionExpired("transaction expired", err)
	}
	if errors.Is(err, context.Canceled) {
		return apperror.NewCancellation("transaction operation was cancelled", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apperror.NewTimeout("transaction operation timed out", err)
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		messages := map[string]string{
			apperror.CodeValidation:            "invalid transaction request",
			apperror.CodeNotFound:              "transaction expired",
			apperror.CodeConflict:              "transaction conflict",
			apperror.CodeConnectionFailed:      "database connection failed",
			apperror.CodeConnectionRequired:    "database connection is not connected",
			apperror.CodeConnectionBusy:        "database connection is busy",
			apperror.CodeQueryCancelled:        "transaction operation was cancelled",
			apperror.CodeQueryTimeout:          "transaction operation timed out",
			apperror.CodeReadonlyViolation:     "transaction is read-only",
			apperror.CodePermissionDenied:      "transaction permission denied",
			apperror.CodeUnsupportedCapability: "transaction operation is unsupported",
			apperror.CodeTransactionExpired:    "transaction expired",
			apperror.CodeTransportFailed:       "database transport failed",
			apperror.CodeInternal:              "transaction operation failed",
		}
		if message, exists := messages[appErr.Code]; exists {
			code := appErr.Code
			if code == apperror.CodeNotFound {
				code = apperror.CodeTransactionExpired
			}
			return apperror.New(code, message, err, appErr.Temporary)
		}
	}
	return apperror.NewInternal("transaction operation failed", err)
}
