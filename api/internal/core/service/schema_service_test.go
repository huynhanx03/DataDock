package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/core/service"
	"github.com/huynhanx03/datadock/internal/ports"
)

func TestSchemaServicePreviewOrdersActionsAndHashesDeterministically(t *testing.T) {
	profiles := &schemaProfileResolverFake{connection: schemaConnectionFixture(entity.EnginePostgreSQL), password: "database-secret"}
	runtime := &schemaRuntimeFake{}
	subject := service.NewSchemaService(profiles, runtime)
	input := dto.SchemaPreviewInput{Actions: []dto.SchemaAction{
		{ID: "drop-table", Kind: dto.SchemaActionDropTable, Target: dto.SchemaTarget{Schema: "public", Table: "archived_users"}},
		{ID: "create-index", Kind: dto.SchemaActionCreateIndex, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, Index: &dto.SchemaIndexDefinition{Name: "idx_users_email", Columns: []string{"email"}, Unique: true, Method: "btree"}},
		{ID: "add-column", Kind: dto.SchemaActionAddColumn, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, Column: &dto.SchemaColumnDefinition{Name: "last_seen_at", DataType: "timestamptz", Nullable: true}},
		{ID: "create-table", Kind: dto.SchemaActionCreateTable, Target: dto.SchemaTarget{Schema: "public", Table: "audit_events"}, Columns: []dto.SchemaColumnDefinition{{Name: "id", DataType: "bigint", Nullable: false}}},
	}}

	first, err := subject.Preview(context.Background(), "connection-1", input)
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	second, err := subject.Preview(context.Background(), "connection-1", input)
	if err != nil {
		t.Fatalf("Preview() second error = %v", err)
	}
	wantOrder := []dto.SchemaActionKind{dto.SchemaActionCreateTable, dto.SchemaActionAddColumn, dto.SchemaActionCreateIndex, dto.SchemaActionDropTable}
	if len(runtime.previewRequests) != 2 {
		t.Fatalf("preview calls = %d", len(runtime.previewRequests))
	}
	for index, kind := range wantOrder {
		if runtime.previewRequests[0].Actions[index].Kind != kind || first.Actions[index].Kind != kind {
			t.Fatalf("action %d runtime=%q result=%q want=%q", index, runtime.previewRequests[0].Actions[index].Kind, first.Actions[index].Kind, kind)
		}
		if first.Steps[index].Position != index+1 || first.Steps[index].ActionID != first.Actions[index].ID {
			t.Fatalf("step %d = %#v", index, first.Steps[index])
		}
	}
	if first.ConnectionID != "connection-1" || first.Engine != entity.EnginePostgreSQL || first.Hash == "" || !strings.HasPrefix(first.Hash, "sha256:") || first.SQL == "" || !first.Destructive || first.GeneratedAt.IsZero() {
		t.Fatalf("preview = %#v", first)
	}
	if first.Hash != second.Hash || first.SQL != second.SQL {
		t.Fatalf("preview is not deterministic first=%q second=%q", first.Hash, second.Hash)
	}
	if runtime.lastConnection.ID != "connection-1" || runtime.lastPassword != "database-secret" {
		t.Fatalf("runtime profile = %#v password=%q", runtime.lastConnection, runtime.lastPassword)
	}
}

func TestSchemaServiceOrdersTableRenameThenDependencyDropsBeforeChanges(t *testing.T) {
	runtime := &schemaRuntimeFake{}
	subject := service.NewSchemaService(&schemaProfileResolverFake{connection: schemaConnectionFixture(entity.EnginePostgreSQL)}, runtime)
	input := dto.SchemaPreviewInput{Actions: []dto.SchemaAction{
		{ID: "rename-table", Kind: dto.SchemaActionRenameTable, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, NewName: "customers"},
		{ID: "create-index", Kind: dto.SchemaActionCreateIndex, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, Index: &dto.SchemaIndexDefinition{Name: "idx_users_email", Columns: []string{"email"}}},
		{ID: "drop-constraint", Kind: dto.SchemaActionDropConstraint, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, Name: "users_email_key"},
		{ID: "drop-index", Kind: dto.SchemaActionDropIndex, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, Name: "idx_users_email"},
		{ID: "add-column", Kind: dto.SchemaActionAddColumn, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, Column: &dto.SchemaColumnDefinition{Name: "email", DataType: "text", Nullable: false}},
	}}

	preview, err := subject.Preview(context.Background(), "connection-1", input)
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	want := []dto.SchemaActionKind{dto.SchemaActionRenameTable, dto.SchemaActionDropIndex, dto.SchemaActionDropConstraint, dto.SchemaActionAddColumn, dto.SchemaActionCreateIndex}
	for index, kind := range want {
		if preview.Actions[index].Kind != kind {
			t.Fatalf("action %d = %q want %q", index, preview.Actions[index].Kind, kind)
		}
	}
}

func TestSchemaServiceApplyRequiresMatchingPreviewHash(t *testing.T) {
	profiles := &schemaProfileResolverFake{connection: schemaConnectionFixture(entity.EnginePostgreSQL)}
	runtime := &schemaRuntimeFake{}
	subject := service.NewSchemaService(profiles, runtime)
	actions := []dto.SchemaAction{{ID: "rename", Kind: dto.SchemaActionRenameTable, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, NewName: "customers"}}

	_, err := subject.Apply(context.Background(), "connection-1", dto.SchemaApplyInput{
		Actions:     actions,
		PreviewHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if schemaApplicationErrorCode(err) != apperror.CodeConflict {
		t.Fatalf("Apply() error = %#v", err)
	}
	if runtime.applyCalls != 0 {
		t.Fatalf("mismatched preview reached apply runtime")
	}
}

func TestSchemaServicePreviewHashIsBoundToConnection(t *testing.T) {
	runtime := &schemaRuntimeFake{}
	profiles := &schemaProfileResolverByIDFake{engine: entity.EnginePostgreSQL}
	subject := service.NewSchemaService(profiles, runtime)
	actions := []dto.SchemaAction{{ID: "rename", Kind: dto.SchemaActionRenameTable, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, NewName: "customers"}}
	preview, err := subject.Preview(context.Background(), "connection-1", dto.SchemaPreviewInput{Actions: actions})
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}

	_, err = subject.Apply(context.Background(), "connection-2", dto.SchemaApplyInput{Actions: actions, PreviewHash: preview.Hash})
	if schemaApplicationErrorCode(err) != apperror.CodeConflict {
		t.Fatalf("Apply() cross-connection error = %#v", err)
	}
	if runtime.applyCalls != 0 {
		t.Fatalf("cross-connection preview reached apply runtime")
	}
}

func TestSchemaServiceApplyRequiresDestructiveConfirmation(t *testing.T) {
	profiles := &schemaProfileResolverFake{connection: schemaConnectionFixture(entity.EnginePostgreSQL)}
	runtime := &schemaRuntimeFake{}
	subject := service.NewSchemaService(profiles, runtime)
	actions := []dto.SchemaAction{{ID: "drop", Kind: dto.SchemaActionDropColumn, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, Name: "legacy_code"}}
	preview, err := subject.Preview(context.Background(), "connection-1", dto.SchemaPreviewInput{Actions: actions})
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}

	_, err = subject.Apply(context.Background(), "connection-1", dto.SchemaApplyInput{Actions: actions, PreviewHash: preview.Hash})
	if schemaApplicationErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("Apply() error = %#v", err)
	}
	if runtime.applyCalls != 0 {
		t.Fatalf("unconfirmed destructive preview reached apply runtime")
	}

	result, err := subject.Apply(context.Background(), "connection-1", dto.SchemaApplyInput{Actions: actions, PreviewHash: preview.Hash, ConfirmDestructive: true})
	if err != nil {
		t.Fatalf("Apply() confirmed error = %v", err)
	}
	if runtime.applyCalls != 1 || runtime.lastApply.PreviewHash != preview.Hash || len(runtime.lastApply.Steps) != 1 {
		t.Fatalf("apply request = %#v calls=%d", runtime.lastApply, runtime.applyCalls)
	}
	if result.PreviewHash != preview.Hash || result.AppliedSteps != 1 || result.AppliedAt.IsZero() {
		t.Fatalf("apply result = %#v", result)
	}
}

func TestSchemaServiceRejectsPartialApplyResult(t *testing.T) {
	partial := ports.SchemaRuntimeApplyResult{AppliedSteps: 0}
	runtime := &schemaRuntimeFake{applyResult: &partial}
	subject := service.NewSchemaService(&schemaProfileResolverFake{connection: schemaConnectionFixture(entity.EnginePostgreSQL)}, runtime)
	actions := []dto.SchemaAction{{ID: "rename", Kind: dto.SchemaActionRenameTable, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, NewName: "customers"}}
	preview, err := subject.Preview(context.Background(), "connection-1", dto.SchemaPreviewInput{Actions: actions})
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}

	_, err = subject.Apply(context.Background(), "connection-1", dto.SchemaApplyInput{Actions: actions, PreviewHash: preview.Hash})
	if schemaApplicationErrorCode(err) != apperror.CodeInternal {
		t.Fatalf("Apply() partial result error = %#v", err)
	}
}

func TestSchemaServiceReportsPartialNonTransactionalApply(t *testing.T) {
	partial := ports.SchemaRuntimeApplyResult{AppliedSteps: 1}
	runtime := &schemaRuntimeFake{applyResult: &partial, applyErr: errors.New("second MySQL DDL step failed")}
	subject := service.NewSchemaService(&schemaProfileResolverFake{connection: schemaConnectionFixture(entity.EngineMySQL)}, runtime)
	actions := []dto.SchemaAction{
		{ID: "rename", Kind: dto.SchemaActionRenameTable, Target: dto.SchemaTarget{Table: "users"}, NewName: "customers"},
		{ID: "add", Kind: dto.SchemaActionAddColumn, Target: dto.SchemaTarget{Table: "customers"}, Column: &dto.SchemaColumnDefinition{Name: "status", DataType: "varchar(32)", Nullable: true}},
	}
	preview, err := subject.Preview(context.Background(), "connection-1", dto.SchemaPreviewInput{Actions: actions})
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}

	result, err := subject.Apply(context.Background(), "connection-1", dto.SchemaApplyInput{Actions: actions, PreviewHash: preview.Hash})
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) || applicationError.Code != apperror.CodeInternal {
		t.Fatalf("Apply() error = %#v", err)
	}
	if result.AppliedSteps != 1 || result.PreviewHash != preview.Hash || result.AppliedAt.IsZero() {
		t.Fatalf("Apply() result = %#v", result)
	}
	if applicationError.Details["appliedSteps"] != 1 || applicationError.Details["totalSteps"] != 2 || applicationError.Details["previewHash"] != preview.Hash || applicationError.Details["partial"] != true {
		t.Fatalf("Apply() details = %#v", applicationError.Details)
	}
}

func TestSchemaServiceRejectsApplyOnReadonlyConnection(t *testing.T) {
	connection := schemaConnectionFixture(entity.EnginePostgreSQL)
	connection.ReadOnly = true
	runtime := &schemaRuntimeFake{}
	subject := service.NewSchemaService(&schemaProfileResolverFake{connection: connection}, runtime)

	_, err := subject.Apply(context.Background(), connection.ID, dto.SchemaApplyInput{
		Actions:     []dto.SchemaAction{{Kind: dto.SchemaActionRenameTable, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, NewName: "customers"}},
		PreviewHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if schemaApplicationErrorCode(err) != apperror.CodeReadonlyViolation {
		t.Fatalf("Apply() error = %#v", err)
	}
	if runtime.previewCalls != 0 || runtime.applyCalls != 0 {
		t.Fatalf("readonly apply reached runtime preview=%d apply=%d", runtime.previewCalls, runtime.applyCalls)
	}
}

func TestSchemaServiceRejectsEngineUnsupportedAction(t *testing.T) {
	connection := schemaConnectionFixture(entity.EngineMySQL)
	runtime := &schemaRuntimeFake{}
	subject := service.NewSchemaService(&schemaProfileResolverFake{connection: connection}, runtime)

	_, err := subject.Preview(context.Background(), connection.ID, dto.SchemaPreviewInput{Actions: []dto.SchemaAction{{
		Kind:   dto.SchemaActionRebuildIndex,
		Target: dto.SchemaTarget{Table: "users"},
		Name:   "idx_users_email",
	}}})
	if schemaApplicationErrorCode(err) != apperror.CodeUnsupportedCapability {
		t.Fatalf("Preview() error = %#v", err)
	}
	if runtime.previewCalls != 0 {
		t.Fatalf("unsupported action reached runtime")
	}
}

func TestSchemaServiceValidatesIdentifiersAndExpressionsBeforeRuntime(t *testing.T) {
	connection := schemaConnectionFixture(entity.EnginePostgreSQL)
	runtime := &schemaRuntimeFake{}
	subject := service.NewSchemaService(&schemaProfileResolverFake{connection: connection}, runtime)
	invalid := []dto.SchemaPreviewInput{
		{Actions: nil},
		{Actions: []dto.SchemaAction{{Kind: dto.SchemaActionRenameTable, Target: dto.SchemaTarget{Schema: "public", Table: strings.Repeat("x", 129)}, NewName: "users_v2"}}},
		{Actions: []dto.SchemaAction{{Kind: dto.SchemaActionAddColumn, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, Column: &dto.SchemaColumnDefinition{Name: "unsafe", DataType: "text; DROP TABLE users", Nullable: true}}}},
		{Actions: []dto.SchemaAction{{Kind: dto.SchemaActionAddConstraint, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, Constraint: &dto.SchemaConstraintDefinition{Name: "users_check", Type: dto.SchemaConstraintCheck, Expression: "true; DROP TABLE users"}}}},
	}
	for index, input := range invalid {
		if _, err := subject.Preview(context.Background(), connection.ID, input); schemaApplicationErrorCode(err) != apperror.CodeValidation {
			t.Fatalf("Preview() invalid[%d] error = %#v", index, err)
		}
	}
	if runtime.previewCalls != 0 {
		t.Fatalf("invalid actions reached runtime")
	}
}

func TestSchemaServiceMapsRuntimePermissionWithoutLeakingDriverDetails(t *testing.T) {
	runtime := &schemaRuntimeFake{previewErr: apperror.NewPermission("ALTER denied for role datadock_app", errors.New("driver SQLSTATE 42501"))}
	subject := service.NewSchemaService(&schemaProfileResolverFake{connection: schemaConnectionFixture(entity.EnginePostgreSQL)}, runtime)

	_, err := subject.Preview(context.Background(), "connection-1", dto.SchemaPreviewInput{Actions: []dto.SchemaAction{{Kind: dto.SchemaActionRenameTable, Target: dto.SchemaTarget{Schema: "public", Table: "users"}, NewName: "customers"}}})
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) || applicationError.Code != apperror.CodePermissionDenied || applicationError.Message != "schema change permission denied" || strings.Contains(applicationError.Message, "datadock_app") {
		t.Fatalf("Preview() error = %#v", err)
	}
}

type schemaProfileResolverFake struct {
	connection entity.Connection
	password   string
	err        error
}

type schemaProfileResolverByIDFake struct {
	engine entity.Engine
}

func (fake *schemaProfileResolverByIDFake) Resolve(_ context.Context, connectionID string) (entity.Connection, string, error) {
	connection := schemaConnectionFixture(fake.engine)
	connection.ID = connectionID
	return connection, "", nil
}

func (fake *schemaProfileResolverFake) Resolve(context.Context, string) (entity.Connection, string, error) {
	return fake.connection, fake.password, fake.err
}

type schemaRuntimeFake struct {
	previewCalls    int
	applyCalls      int
	previewRequests []ports.SchemaRuntimePreviewRequest
	lastApply       ports.SchemaRuntimeApplyRequest
	lastConnection  entity.Connection
	lastPassword    string
	previewErr      error
	applyErr        error
	applyResult     *ports.SchemaRuntimeApplyResult
}

func (fake *schemaRuntimeFake) Preview(_ context.Context, connection entity.Connection, password string, request ports.SchemaRuntimePreviewRequest) ([]dto.SchemaStep, error) {
	fake.previewCalls++
	fake.lastConnection = connection
	fake.lastPassword = password
	fake.previewRequests = append(fake.previewRequests, request)
	if fake.previewErr != nil {
		return nil, fake.previewErr
	}
	steps := make([]dto.SchemaStep, 0, len(request.Actions))
	for _, action := range request.Actions {
		steps = append(steps, dto.SchemaStep{ActionID: action.ID, Kind: action.Kind, SQL: "EXECUTE " + string(action.Kind)})
	}
	return steps, nil
}

func (fake *schemaRuntimeFake) Apply(_ context.Context, connection entity.Connection, password string, request ports.SchemaRuntimeApplyRequest) (ports.SchemaRuntimeApplyResult, error) {
	fake.applyCalls++
	fake.lastConnection = connection
	fake.lastPassword = password
	fake.lastApply = request
	if fake.applyErr != nil {
		if fake.applyResult != nil {
			return *fake.applyResult, fake.applyErr
		}
		return ports.SchemaRuntimeApplyResult{}, fake.applyErr
	}
	if fake.applyResult != nil {
		return *fake.applyResult, nil
	}
	return ports.SchemaRuntimeApplyResult{AppliedSteps: len(request.Steps)}, nil
}

func schemaConnectionFixture(engine entity.Engine) entity.Connection {
	return entity.Connection{ID: "connection-1", Name: "Primary", Engine: engine, Host: "database", Port: 5432, Database: "datadock", Username: "datadock"}
}

func schemaApplicationErrorCode(err error) string {
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return applicationError.Code
	}
	return ""
}
