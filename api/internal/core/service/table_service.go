package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/ports"
)

const (
	maximumTableMutations       = 100
	maximumMutationColumns      = 200
	maximumMutationKeyColumns   = 32
	maximumMutationColumnLength = 128
	maximumMutationValueBytes   = 1024 * 1024
	maximumMutationBatchBytes   = 8 * 1024 * 1024
	maximumMutationValueDepth   = 16
	maximumMutationValueNodes   = 10000
)

type TableService struct {
	profiles ports.ConnectionProfileResolver
	runtime  ports.TableMutationRuntime
}

type mutationBudget struct {
	bytes int
	nodes int
}

func NewTableService(profiles ports.ConnectionProfileResolver, runtime ports.TableMutationRuntime) *TableService {
	return &TableService{profiles: profiles, runtime: runtime}
}

func (service *TableService) Apply(ctx context.Context, input dto.TableMutationBatchInput) (dto.TableMutationBatchResult, error) {
	if err := validateMutationBatchShape(input); err != nil {
		return dto.TableMutationBatchResult{}, err
	}
	connection, password, err := service.profiles.Resolve(ctx, input.ConnectionID)
	if err != nil {
		return dto.TableMutationBatchResult{}, serviceError(err)
	}
	if connection.ID != input.ConnectionID {
		return dto.TableMutationBatchResult{}, apperror.NewInternal("connection resolution failed", nil)
	}
	if connection.ReadOnly {
		return dto.TableMutationBatchResult{}, apperror.NewReadonly("table mutation rejected by read-only connection", nil)
	}
	metadata, err := service.runtime.MutationMetadata(ctx, connection, password, input.Reference, input.TransactionID)
	if err != nil {
		return dto.TableMutationBatchResult{}, tableMutationServiceError(err)
	}
	if err := validateMutationsAgainstMetadata(input.Mutations, metadata); err != nil {
		return dto.TableMutationBatchResult{}, err
	}
	result, err := service.runtime.ApplyMutations(ctx, connection, password, ports.TableMutationRuntimeRequest{
		Reference:     input.Reference,
		TransactionID: input.TransactionID,
		Mutations:     input.Mutations,
	})
	if err != nil {
		return dto.TableMutationBatchResult{}, tableMutationServiceError(err)
	}
	if !validMutationBatchResult(result, input.Mutations) {
		return dto.TableMutationBatchResult{}, apperror.NewInternal("table mutation returned an invalid atomic result", nil)
	}
	return result, nil
}

func validateMutationBatchShape(input dto.TableMutationBatchInput) error {
	if !validServiceIdentifier(input.ConnectionID, false) || !validServiceIdentifier(input.TransactionID, true) || !validOpaqueTableReference(input.Reference) || len(input.Mutations) < 1 || len(input.Mutations) > maximumTableMutations {
		return apperror.NewValidation("invalid table mutation request", nil)
	}
	budget := &mutationBudget{}
	for _, mutation := range input.Mutations {
		if err := validateMutationShape(mutation, budget); err != nil {
			return err
		}
	}
	return nil
}

func validateMutationShape(mutation dto.TableMutation, budget *mutationBudget) error {
	if len(mutation.Values) > maximumMutationColumns || len(mutation.Keys) > maximumMutationKeyColumns || len(mutation.ExpectedValues) > maximumMutationColumns {
		return apperror.NewValidation("invalid table mutation request", nil)
	}
	switch mutation.Kind {
	case dto.TableMutationInsert:
		if len(mutation.Values) == 0 || len(mutation.Keys) != 0 || len(mutation.ExpectedValues) != 0 {
			return apperror.NewValidation("invalid insert mutation", nil)
		}
	case dto.TableMutationUpdate:
		if len(mutation.Values) == 0 || len(mutation.Keys) == 0 {
			return apperror.NewValidation("invalid update mutation", nil)
		}
	case dto.TableMutationDelete:
		if len(mutation.Values) != 0 || len(mutation.Keys) == 0 {
			return apperror.NewValidation("invalid delete mutation", nil)
		}
	default:
		return apperror.NewValidation("invalid table mutation kind", nil)
	}
	for _, values := range []map[string]any{mutation.Values, mutation.Keys, mutation.ExpectedValues} {
		for column, value := range values {
			if !validMutationColumn(column) || !validateMutationValue(value, budget, 0) {
				return apperror.NewValidation("invalid table mutation value", nil)
			}
		}
	}
	for _, value := range mutation.Keys {
		if value == nil {
			return apperror.NewValidation("primary key values cannot be null", nil)
		}
	}
	return nil
}

func validateMutationsAgainstMetadata(mutations []dto.TableMutation, metadata dto.TableMutationMetadata) error {
	columns := make(map[string]dto.TableMutationColumn, len(metadata.Columns))
	for _, column := range metadata.Columns {
		if !validMutationColumn(column.Name) {
			return apperror.NewInternal("table mutation metadata is invalid", nil)
		}
		if _, exists := columns[column.Name]; exists {
			return apperror.NewInternal("table mutation metadata is invalid", nil)
		}
		columns[column.Name] = column
	}
	if len(columns) == 0 || len(columns) > maximumMutationColumns {
		return apperror.NewInternal("table mutation metadata is invalid", nil)
	}
	primaryKeys := make(map[string]struct{}, len(metadata.PrimaryKeyColumns))
	for _, key := range metadata.PrimaryKeyColumns {
		if _, exists := columns[key]; !exists {
			return apperror.NewInternal("table mutation metadata is invalid", nil)
		}
		if _, exists := primaryKeys[key]; exists {
			return apperror.NewInternal("table mutation metadata is invalid", nil)
		}
		primaryKeys[key] = struct{}{}
	}
	for _, mutation := range mutations {
		if mutation.Kind != dto.TableMutationInsert {
			if len(primaryKeys) == 0 {
				return apperror.NewValidation("table mutations require a primary key", nil)
			}
			if !hasExactMutationKeys(mutation.Keys, primaryKeys) {
				return apperror.NewValidation("mutation keys must exactly match the table primary key", nil)
			}
		}
		for column := range mutation.Values {
			definition, exists := columns[column]
			if !exists || !definition.Writable || definition.Generated {
				return apperror.NewValidation("mutation contains a non-writable column", nil)
			}
		}
		for column := range mutation.ExpectedValues {
			if _, exists := columns[column]; !exists {
				return apperror.NewValidation("mutation contains an unknown expected column", nil)
			}
		}
	}
	return nil
}

func hasExactMutationKeys(values map[string]any, primaryKeys map[string]struct{}) bool {
	if len(values) != len(primaryKeys) {
		return false
	}
	for key := range values {
		if _, exists := primaryKeys[key]; !exists {
			return false
		}
	}
	return true
}

func validMutationBatchResult(result dto.TableMutationBatchResult, mutations []dto.TableMutation) bool {
	if !result.Atomic {
		return false
	}
	switch result.Status {
	case dto.TableMutationBatchApplied:
		return result.Applied == len(mutations) && len(result.Conflicts) == 0
	case dto.TableMutationBatchConflict:
		if result.Applied != 0 || len(result.Conflicts) == 0 || len(result.Conflicts) > len(mutations) {
			return false
		}
		seen := make(map[int]struct{}, len(result.Conflicts))
		for _, conflict := range result.Conflicts {
			if conflict.Index < 0 || conflict.Index >= len(mutations) || conflict.Kind != mutations[conflict.Index].Kind || conflict.Reason != dto.TableMutationConflictOptimistic && conflict.Reason != dto.TableMutationConflictMissingRow {
				return false
			}
			if _, exists := seen[conflict.Index]; exists {
				return false
			}
			seen[conflict.Index] = struct{}{}
		}
		return true
	default:
		return false
	}
}

func validOpaqueTableReference(value string) bool {
	if value == "" || len(value) > 4096 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validMutationColumn(value string) bool {
	if value == "" || len(value) > maximumMutationColumnLength || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 32 || character == 127 {
			return false
		}
	}
	return true
}

func validateMutationValue(value any, budget *mutationBudget, depth int) bool {
	if depth > maximumMutationValueDepth {
		return false
	}
	budget.nodes++
	if budget.nodes > maximumMutationValueNodes {
		return false
	}
	addBytes := func(count int) bool {
		if count > maximumMutationValueBytes {
			return false
		}
		budget.bytes += count
		return budget.bytes <= maximumMutationBatchBytes
	}
	switch typed := value.(type) {
	case nil, bool:
		return true
	case string:
		return addBytes(len(typed))
	case []byte:
		return addBytes(len(typed))
	case json.Number:
		text := string(typed)
		return text != "" && len(text) <= 128 && strings.TrimSpace(text) == text && addBytes(len(text))
	case float32:
		return !math.IsNaN(float64(typed)) && !math.IsInf(float64(typed), 0)
	case float64:
		return !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case time.Time:
		return true
	case map[string]any:
		if len(typed) > maximumMutationValueNodes {
			return false
		}
		for key, item := range typed {
			if len(key) > maximumMutationColumnLength || !addBytes(len(key)) || !validateMutationValue(item, budget, depth+1) {
				return false
			}
		}
		return true
	case []any:
		if len(typed) > maximumMutationValueNodes {
			return false
		}
		for _, item := range typed {
			if !validateMutationValue(item, budget, depth+1) {
				return false
			}
		}
		return true
	}
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return true
	}
	switch reflected.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	case reflect.Float32, reflect.Float64:
		floatValue := reflected.Float()
		return !math.IsNaN(floatValue) && !math.IsInf(floatValue, 0)
	case reflect.Array, reflect.Slice:
		if reflected.Len() > maximumMutationValueNodes {
			return false
		}
		if reflected.Type().Elem().Kind() == reflect.Uint8 {
			return addBytes(reflected.Len())
		}
		for index := 0; index < reflected.Len(); index++ {
			if !validateMutationValue(reflected.Index(index).Interface(), budget, depth+1) {
				return false
			}
		}
		return true
	case reflect.Map:
		if reflected.Type().Key().Kind() != reflect.String || reflected.Len() > maximumMutationValueNodes {
			return false
		}
		iterator := reflected.MapRange()
		for iterator.Next() {
			key := iterator.Key().String()
			if len(key) > maximumMutationColumnLength || !addBytes(len(key)) || !validateMutationValue(iterator.Value().Interface(), budget, depth+1) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func validServiceIdentifier(value string, optional bool) bool {
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

func tableMutationServiceError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return apperror.NewCancellation("table mutation was cancelled", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apperror.NewTimeout("table mutation timed out", err)
	}
	if errors.Is(err, ports.ErrTransactionExpired) {
		return apperror.NewTransactionExpired("transaction expired", err)
	}
	if errors.Is(err, ports.ErrNotFound) {
		return apperror.NewNotFound("table was not found", err)
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		messages := map[string]string{
			apperror.CodeValidation:            "invalid table mutation",
			apperror.CodeNotFound:              "table was not found",
			apperror.CodeConflict:              "table mutation conflict",
			apperror.CodeConnectionFailed:      "database connection failed",
			apperror.CodeConnectionRequired:    "database connection is not connected",
			apperror.CodeConnectionBusy:        "database connection is busy",
			apperror.CodeQueryCancelled:        "table mutation was cancelled",
			apperror.CodeQueryTimeout:          "table mutation timed out",
			apperror.CodeReadonlyViolation:     "table mutation rejected by read-only connection",
			apperror.CodePermissionDenied:      "table mutation permission denied",
			apperror.CodeUnsupportedCapability: "table mutation is unsupported",
			apperror.CodeTransactionExpired:    "transaction expired",
			apperror.CodeTransportFailed:       "database transport failed",
			apperror.CodeInternal:              "table mutation failed",
		}
		if message, exists := messages[appErr.Code]; exists {
			return apperror.New(appErr.Code, message, err, appErr.Temporary)
		}
	}
	return apperror.NewInternal("table mutation failed", err)
}
