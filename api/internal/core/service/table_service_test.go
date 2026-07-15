package service_test

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/core/service"
	"github.com/huynhanx03/datadock/internal/ports"
)

func TestTableServiceRejectsReadonlyBeforeMutationRuntime(t *testing.T) {
	profiles := &tableProfileResolverFake{connection: tableConnectionFixture(true), password: "secret"}
	runtime := &tableMutationRuntimeFake{}
	subject := service.NewTableService(profiles, runtime)

	_, err := subject.Apply(context.Background(), validTableMutationInput())
	if tableErrorCode(err) != apperror.CodeReadonlyViolation {
		t.Fatalf("Apply() error = %#v", err)
	}
	if runtime.metadataCalls != 0 || runtime.applyCalls != 0 {
		t.Fatalf("readonly mutation reached runtime metadata=%d apply=%d", runtime.metadataCalls, runtime.applyCalls)
	}
}

func TestTableServiceRejectsUpdateWithoutPrimaryKey(t *testing.T) {
	profiles := &tableProfileResolverFake{connection: tableConnectionFixture(false)}
	runtime := &tableMutationRuntimeFake{metadata: dto.TableMutationMetadata{
		Columns: []dto.TableMutationColumn{{Name: "email", Writable: true}},
	}}
	subject := service.NewTableService(profiles, runtime)
	input := validTableMutationInput()
	input.Mutations = []dto.TableMutation{{Kind: dto.TableMutationUpdate, Keys: map[string]any{"id": "usr-1"}, Values: map[string]any{"email": "new@example.com"}}}

	_, err := subject.Apply(context.Background(), input)
	if tableErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("Apply() error = %#v", err)
	}
	if runtime.applyCalls != 0 {
		t.Fatalf("mutation without primary key reached apply runtime")
	}
}

func TestTableServiceRequiresExactCompositePrimaryKeyAndForwardsTransaction(t *testing.T) {
	profiles := &tableProfileResolverFake{connection: tableConnectionFixture(false), password: "database-secret"}
	runtime := &tableMutationRuntimeFake{
		metadata: compositeTableMutationMetadata(),
		result: dto.TableMutationBatchResult{
			Status:  dto.TableMutationBatchApplied,
			Atomic:  true,
			Applied: 2,
		},
	}
	subject := service.NewTableService(profiles, runtime)
	input := validTableMutationInput()
	input.TransactionID = "transaction-1"
	input.Mutations = []dto.TableMutation{
		{Kind: dto.TableMutationUpdate, Keys: map[string]any{"tenant_id": "tenant-1", "id": "usr-1"}, Values: map[string]any{"email": "new@example.com"}},
		{Kind: dto.TableMutationDelete, Keys: map[string]any{"id": "usr-2", "tenant_id": "tenant-1"}},
	}

	result, err := subject.Apply(context.Background(), input)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.Status != dto.TableMutationBatchApplied || !result.Atomic || result.Applied != 2 {
		t.Fatalf("Apply() result = %#v", result)
	}
	if runtime.metadataCalls != 1 || runtime.applyCalls != 1 {
		t.Fatalf("runtime calls metadata=%d apply=%d", runtime.metadataCalls, runtime.applyCalls)
	}
	if runtime.lastConnection.ID != "connection-1" || runtime.lastPassword != "database-secret" || runtime.lastMetadataTransaction != "transaction-1" || runtime.lastRequest.TransactionID != "transaction-1" || runtime.lastRequest.Reference != input.Reference || len(runtime.lastRequest.Mutations) != 2 {
		t.Fatalf("runtime request connection=%#v password=%q request=%#v", runtime.lastConnection, runtime.lastPassword, runtime.lastRequest)
	}

	for name, keys := range map[string]map[string]any{
		"missing component": {"id": "usr-1"},
		"extra component":   {"id": "usr-1", "tenant_id": "tenant-1", "version": 3},
	} {
		t.Run(name, func(t *testing.T) {
			runtime.applyCalls = 0
			invalid := validTableMutationInput()
			invalid.Mutations[0].Keys = keys
			_, err := subject.Apply(context.Background(), invalid)
			if tableErrorCode(err) != apperror.CodeValidation {
				t.Fatalf("Apply() error = %#v", err)
			}
			if runtime.applyCalls != 0 {
				t.Fatalf("invalid composite key reached apply runtime")
			}
		})
	}
}

func TestTableServiceValidatesMutationColumnsAndValuesAgainstMetadata(t *testing.T) {
	tests := []struct {
		name     string
		mutation dto.TableMutation
	}{
		{name: "unknown column", mutation: dto.TableMutation{Kind: dto.TableMutationUpdate, Keys: map[string]any{"id": "usr-1", "tenant_id": "tenant-1"}, Values: map[string]any{"missing": "value"}}},
		{name: "generated column", mutation: dto.TableMutation{Kind: dto.TableMutationUpdate, Keys: map[string]any{"id": "usr-1", "tenant_id": "tenant-1"}, Values: map[string]any{"computed": "value"}}},
		{name: "nil primary key", mutation: dto.TableMutation{Kind: dto.TableMutationDelete, Keys: map[string]any{"id": nil, "tenant_id": "tenant-1"}}},
		{name: "oversized scalar", mutation: dto.TableMutation{Kind: dto.TableMutationUpdate, Keys: map[string]any{"id": "usr-1", "tenant_id": "tenant-1"}, Values: map[string]any{"email": strings.Repeat("x", 1024*1024+1)}}},
		{name: "non finite scalar", mutation: dto.TableMutation{Kind: dto.TableMutationUpdate, Keys: map[string]any{"id": "usr-1", "tenant_id": "tenant-1"}, Values: map[string]any{"score": math.Inf(1)}}},
		{name: "delete values", mutation: dto.TableMutation{Kind: dto.TableMutationDelete, Keys: map[string]any{"id": "usr-1", "tenant_id": "tenant-1"}, Values: map[string]any{"email": "value"}}},
		{name: "insert expected values", mutation: dto.TableMutation{Kind: dto.TableMutationInsert, Values: map[string]any{"email": "value"}, ExpectedValues: map[string]any{"email": "old"}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profiles := &tableProfileResolverFake{connection: tableConnectionFixture(false)}
			runtime := &tableMutationRuntimeFake{metadata: compositeTableMutationMetadata()}
			subject := service.NewTableService(profiles, runtime)
			input := validTableMutationInput()
			input.Mutations = []dto.TableMutation{test.mutation}

			_, err := subject.Apply(context.Background(), input)
			if tableErrorCode(err) != apperror.CodeValidation {
				t.Fatalf("Apply() error = %#v", err)
			}
			if runtime.applyCalls != 0 {
				t.Fatalf("invalid mutation reached apply runtime")
			}
		})
	}
}

func TestTableServiceForwardsExpectedValuesAndReturnsAtomicConflict(t *testing.T) {
	profiles := &tableProfileResolverFake{connection: tableConnectionFixture(false)}
	conflict := dto.TableMutationConflict{Index: 0, Kind: dto.TableMutationUpdate, Reason: dto.TableMutationConflictOptimistic}
	runtime := &tableMutationRuntimeFake{
		metadata: compositeTableMutationMetadata(),
		result: dto.TableMutationBatchResult{
			Status:    dto.TableMutationBatchConflict,
			Atomic:    true,
			Conflicts: []dto.TableMutationConflict{conflict},
		},
	}
	subject := service.NewTableService(profiles, runtime)
	input := validTableMutationInput()
	input.Mutations[0].ExpectedValues = map[string]any{"email": "old@example.com", "version": float64(4)}

	result, err := subject.Apply(context.Background(), input)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if result.Status != dto.TableMutationBatchConflict || !result.Atomic || result.Applied != 0 || len(result.Conflicts) != 1 || result.Conflicts[0] != conflict {
		t.Fatalf("Apply() result = %#v", result)
	}
	if runtime.lastRequest.Mutations[0].ExpectedValues["email"] != "old@example.com" || runtime.lastRequest.Mutations[0].ExpectedValues["version"] != float64(4) {
		t.Fatalf("expected values = %#v", runtime.lastRequest.Mutations[0].ExpectedValues)
	}
}

func TestTableServiceRejectsPartialAtomicResult(t *testing.T) {
	profiles := &tableProfileResolverFake{connection: tableConnectionFixture(false)}
	runtime := &tableMutationRuntimeFake{
		metadata: compositeTableMutationMetadata(),
		result: dto.TableMutationBatchResult{
			Status:  dto.TableMutationBatchApplied,
			Atomic:  true,
			Applied: 0,
		},
	}
	subject := service.NewTableService(profiles, runtime)

	_, err := subject.Apply(context.Background(), validTableMutationInput())
	if tableErrorCode(err) != apperror.CodeInternal {
		t.Fatalf("Apply() error = %#v", err)
	}
}

func TestTableServiceMapsAtomicRuntimeFailureWithoutRetry(t *testing.T) {
	profiles := &tableProfileResolverFake{connection: tableConnectionFixture(false)}
	runtime := &tableMutationRuntimeFake{metadata: compositeTableMutationMetadata(), applyErr: errors.New("driver leaked partial transaction state")}
	subject := service.NewTableService(profiles, runtime)

	_, err := subject.Apply(context.Background(), validTableMutationInput())
	if tableErrorCode(err) != apperror.CodeInternal || err.Error() != "table mutation failed" {
		t.Fatalf("Apply() error = %#v", err)
	}
	if runtime.applyCalls != 1 {
		t.Fatalf("runtime apply calls = %d", runtime.applyCalls)
	}
}

func TestTableServiceRejectsInvalidBatchBeforeResolvingConnection(t *testing.T) {
	profiles := &tableProfileResolverFake{connection: tableConnectionFixture(false)}
	runtime := &tableMutationRuntimeFake{}
	subject := service.NewTableService(profiles, runtime)
	input := validTableMutationInput()
	input.Reference = " bad-reference "

	_, err := subject.Apply(context.Background(), input)
	if tableErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("Apply() error = %#v", err)
	}
	if profiles.calls != 0 || runtime.metadataCalls != 0 || runtime.applyCalls != 0 {
		t.Fatalf("invalid batch reached dependencies profiles=%d metadata=%d apply=%d", profiles.calls, runtime.metadataCalls, runtime.applyCalls)
	}
}

type tableProfileResolverFake struct {
	connection entity.Connection
	password   string
	err        error
	calls      int
}

func (fake *tableProfileResolverFake) Resolve(context.Context, string) (entity.Connection, string, error) {
	fake.calls++
	return fake.connection, fake.password, fake.err
}

type tableMutationRuntimeFake struct {
	metadata                dto.TableMutationMetadata
	metadataErr             error
	result                  dto.TableMutationBatchResult
	applyErr                error
	metadataCalls           int
	applyCalls              int
	lastConnection          entity.Connection
	lastPassword            string
	lastReference           string
	lastMetadataTransaction string
	lastRequest             ports.TableMutationRuntimeRequest
}

func (fake *tableMutationRuntimeFake) MutationMetadata(_ context.Context, connection entity.Connection, password, reference, transactionID string) (dto.TableMutationMetadata, error) {
	fake.metadataCalls++
	fake.lastConnection = connection
	fake.lastPassword = password
	fake.lastReference = reference
	fake.lastMetadataTransaction = transactionID
	return fake.metadata, fake.metadataErr
}

func (fake *tableMutationRuntimeFake) ApplyMutations(_ context.Context, connection entity.Connection, password string, request ports.TableMutationRuntimeRequest) (dto.TableMutationBatchResult, error) {
	fake.applyCalls++
	fake.lastConnection = connection
	fake.lastPassword = password
	fake.lastRequest = request
	return fake.result, fake.applyErr
}

func tableConnectionFixture(readonly bool) entity.Connection {
	return entity.Connection{ID: "connection-1", Engine: entity.EnginePostgreSQL, ReadOnly: readonly}
}

func compositeTableMutationMetadata() dto.TableMutationMetadata {
	return dto.TableMutationMetadata{
		Columns: []dto.TableMutationColumn{
			{Name: "id", Writable: true},
			{Name: "tenant_id", Writable: true},
			{Name: "email", Writable: true},
			{Name: "version", Writable: true},
			{Name: "score", Writable: true},
			{Name: "computed", Generated: true},
		},
		PrimaryKeyColumns: []string{"tenant_id", "id"},
	}
}

func validTableMutationInput() dto.TableMutationBatchInput {
	return dto.TableMutationBatchInput{
		ConnectionID: "connection-1",
		Reference:    "eyJ2IjoxLCJraW5kIjoidGFibGUifQ",
		Mutations: []dto.TableMutation{{
			Kind:   dto.TableMutationUpdate,
			Keys:   map[string]any{"tenant_id": "tenant-1", "id": "usr-1"},
			Values: map[string]any{"email": "new@example.com"},
		}},
	}
}

func tableErrorCode(err error) string {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}
