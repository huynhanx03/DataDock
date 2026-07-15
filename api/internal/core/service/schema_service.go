package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

type SchemaService struct {
	profiles ports.ConnectionProfileResolver
	runtime  ports.SchemaRuntime
}

func NewSchemaService(profiles ports.ConnectionProfileResolver, runtime ports.SchemaRuntime) *SchemaService {
	return &SchemaService{profiles: profiles, runtime: runtime}
}

func (service *SchemaService) Preview(ctx context.Context, connectionID string, input dto.SchemaPreviewInput) (dto.SchemaPreview, error) {
	actions, err := prepareSchemaActions(connectionID, input.Actions)
	if err != nil {
		return dto.SchemaPreview{}, err
	}
	connection, password, err := service.profiles.Resolve(ctx, connectionID)
	if err != nil {
		return dto.SchemaPreview{}, serviceError(err)
	}
	if err := validateEngineSchemaActions(connection.Engine, actions); err != nil {
		return dto.SchemaPreview{}, err
	}
	return service.generatePreview(ctx, connection, password, actions)
}

func (service *SchemaService) Apply(ctx context.Context, connectionID string, input dto.SchemaApplyInput) (dto.SchemaApplyResult, error) {
	actions, err := prepareSchemaActions(connectionID, input.Actions)
	if err != nil {
		return dto.SchemaApplyResult{}, err
	}
	providedHash, err := parseSchemaPreviewHash(input.PreviewHash)
	if err != nil {
		return dto.SchemaApplyResult{}, err
	}
	connection, password, err := service.profiles.Resolve(ctx, connectionID)
	if err != nil {
		return dto.SchemaApplyResult{}, serviceError(err)
	}
	if connection.ReadOnly {
		return dto.SchemaApplyResult{}, apperror.NewReadonly("schema changes are disabled for this read-only connection", nil)
	}
	if err := validateEngineSchemaActions(connection.Engine, actions); err != nil {
		return dto.SchemaApplyResult{}, err
	}
	preview, err := service.generatePreview(ctx, connection, password, actions)
	if err != nil {
		return dto.SchemaApplyResult{}, err
	}
	generatedHash, _ := parseSchemaPreviewHash(preview.Hash)
	if subtle.ConstantTimeCompare(providedHash, generatedHash) != 1 {
		return dto.SchemaApplyResult{}, apperror.NewConflict("schema preview is stale", nil)
	}
	if preview.Destructive && !input.ConfirmDestructive {
		return dto.SchemaApplyResult{}, apperror.NewValidation("destructive schema changes require confirmation", nil)
	}
	runtimeResult, err := service.runtime.Apply(ctx, connection, password, ports.SchemaRuntimeApplyRequest{PreviewHash: preview.Hash, Steps: preview.Steps})
	if err != nil {
		if runtimeResult.AppliedSteps < 0 || runtimeResult.AppliedSteps > len(preview.Steps) {
			return dto.SchemaApplyResult{}, apperror.NewInternal("schema apply returned an invalid partial result", nil)
		}
		result := dto.SchemaApplyResult{PreviewHash: preview.Hash, AppliedSteps: runtimeResult.AppliedSteps, AppliedAt: time.Now().UTC()}
		mapped := schemaOperationError(err)
		if runtimeResult.AppliedSteps > 0 {
			mapped = apperror.WithDetails(mapped, map[string]any{
				"appliedSteps": runtimeResult.AppliedSteps,
				"totalSteps":   len(preview.Steps),
				"previewHash":  preview.Hash,
				"partial":      true,
			})
		}
		return result, mapped
	}
	if runtimeResult.AppliedSteps != len(preview.Steps) {
		return dto.SchemaApplyResult{}, apperror.NewInternal("schema apply returned an invalid result", nil)
	}
	return dto.SchemaApplyResult{PreviewHash: preview.Hash, AppliedSteps: runtimeResult.AppliedSteps, AppliedAt: time.Now().UTC()}, nil
}

func (service *SchemaService) generatePreview(ctx context.Context, connection entity.Connection, password string, actions []dto.SchemaAction) (dto.SchemaPreview, error) {
	steps, err := service.runtime.Preview(ctx, connection, password, ports.SchemaRuntimePreviewRequest{Actions: actions})
	if err != nil {
		return dto.SchemaPreview{}, schemaOperationError(err)
	}
	steps, sqlText, destructive, err := normalizeSchemaSteps(actions, steps)
	if err != nil {
		return dto.SchemaPreview{}, err
	}
	hash, err := hashSchemaPreview(connection, actions, steps, sqlText, destructive)
	if err != nil {
		return dto.SchemaPreview{}, apperror.NewInternal("schema preview could not be hashed", err)
	}
	return dto.SchemaPreview{
		ConnectionID: connection.ID,
		Engine:       connection.Engine,
		Actions:      actions,
		Steps:        steps,
		SQL:          sqlText,
		Hash:         hash,
		Destructive:  destructive,
		GeneratedAt:  time.Now().UTC(),
	}, nil
}

func prepareSchemaActions(connectionID string, input []dto.SchemaAction) ([]dto.SchemaAction, error) {
	if !validQueryID(connectionID, false) || len(input) < 1 || len(input) > 100 {
		return nil, apperror.NewValidation("invalid schema change request", nil)
	}
	actions := append([]dto.SchemaAction(nil), input...)
	for _, action := range actions {
		if err := validateSchemaAction(action); err != nil {
			return nil, err
		}
	}
	sort.SliceStable(actions, func(left, right int) bool {
		return schemaActionOrder(actions[left].Kind) < schemaActionOrder(actions[right].Kind)
	})
	seen := make(map[string]struct{}, len(actions))
	for index := range actions {
		if actions[index].ID == "" {
			actions[index].ID = fmt.Sprintf("action-%03d", index+1)
		}
		if !validSchemaToken(actions[index].ID, 64) {
			return nil, apperror.NewValidation("invalid schema action identifier", nil)
		}
		if _, exists := seen[actions[index].ID]; exists {
			return nil, apperror.NewValidation("duplicate schema action identifier", nil)
		}
		seen[actions[index].ID] = struct{}{}
	}
	return actions, nil
}

func validateSchemaAction(action dto.SchemaAction) error {
	if !knownSchemaAction(action.Kind) {
		return apperror.NewValidation("invalid schema action", nil)
	}
	if action.ID != "" && !validSchemaToken(action.ID, 64) || !validSchemaTarget(action.Target) {
		return apperror.NewValidation("invalid schema action", nil)
	}
	switch action.Kind {
	case dto.SchemaActionCreateTable:
		if len(action.Columns) < 1 || len(action.Columns) > 200 || !validSchemaColumns(action.Columns) {
			return apperror.NewValidation("invalid table definition", nil)
		}
	case dto.SchemaActionRenameTable:
		if !validIdentifier(action.NewName) {
			return apperror.NewValidation("invalid table rename", nil)
		}
	case dto.SchemaActionDropTable:
	case dto.SchemaActionAddColumn:
		if action.Column == nil || !validSchemaColumn(*action.Column) {
			return apperror.NewValidation("invalid column definition", nil)
		}
	case dto.SchemaActionRenameColumn:
		if !validIdentifier(action.Name) || !validIdentifier(action.NewName) {
			return apperror.NewValidation("invalid column rename", nil)
		}
	case dto.SchemaActionAlterColumnType:
		if !validIdentifier(action.Name) || !safeSchemaFragment(action.DataType, 128) {
			return apperror.NewValidation("invalid column type change", nil)
		}
	case dto.SchemaActionSetColumnNullable:
		if !validIdentifier(action.Name) || action.Nullable == nil {
			return apperror.NewValidation("invalid column nullable change", nil)
		}
	case dto.SchemaActionSetColumnDefault:
		if !validIdentifier(action.Name) || action.DefaultValue != nil && !safeSchemaFragment(*action.DefaultValue, 4096) {
			return apperror.NewValidation("invalid column default change", nil)
		}
	case dto.SchemaActionSetColumnComment:
		if !validIdentifier(action.Name) || action.Comment == nil || !boundedSchemaText(*action.Comment, 2000) {
			return apperror.NewValidation("invalid column comment change", nil)
		}
	case dto.SchemaActionDropColumn:
		if !validIdentifier(action.Name) {
			return apperror.NewValidation("invalid column drop", nil)
		}
	case dto.SchemaActionAddConstraint:
		if action.Constraint == nil || !validSchemaConstraint(*action.Constraint) {
			return apperror.NewValidation("invalid constraint definition", nil)
		}
	case dto.SchemaActionDropConstraint:
		if !validIdentifier(action.Name) {
			return apperror.NewValidation("invalid constraint drop", nil)
		}
	case dto.SchemaActionCreateIndex:
		if action.Index == nil || !validSchemaIndex(*action.Index) {
			return apperror.NewValidation("invalid index definition", nil)
		}
	case dto.SchemaActionDropIndex, dto.SchemaActionRebuildIndex, dto.SchemaActionAnalyzeIndex:
		if !validIdentifier(action.Name) {
			return apperror.NewValidation("invalid index action", nil)
		}
	}
	return nil
}

func validSchemaTarget(target dto.SchemaTarget) bool {
	return validIdentifier(target.Table) && (target.Schema == "" || validIdentifier(target.Schema))
}

func validSchemaColumns(columns []dto.SchemaColumnDefinition) bool {
	seen := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		if !validSchemaColumn(column) {
			return false
		}
		key := strings.ToLower(column.Name)
		if _, exists := seen[key]; exists {
			return false
		}
		seen[key] = struct{}{}
	}
	return true
}

func validSchemaColumn(column dto.SchemaColumnDefinition) bool {
	if !validIdentifier(column.Name) || !safeSchemaFragment(column.DataType, 128) || !boundedSchemaText(column.Comment, 2000) || column.Identity && column.GeneratedExpression != nil {
		return false
	}
	if column.DefaultValue != nil && !safeSchemaFragment(*column.DefaultValue, 4096) {
		return false
	}
	return column.GeneratedExpression == nil || safeSchemaFragment(*column.GeneratedExpression, 4096)
}

func validSchemaConstraint(constraint dto.SchemaConstraintDefinition) bool {
	if !validIdentifier(constraint.Name) || !validReferentialAction(constraint.OnUpdate) || !validReferentialAction(constraint.OnDelete) {
		return false
	}
	switch constraint.Type {
	case dto.SchemaConstraintPrimaryKey, dto.SchemaConstraintUnique:
		return validSchemaIdentifiers(constraint.Columns, 1, 32)
	case dto.SchemaConstraintForeignKey:
		return validSchemaIdentifiers(constraint.Columns, 1, 32) && constraint.ReferencedTarget != nil && validSchemaTarget(*constraint.ReferencedTarget) && validSchemaIdentifiers(constraint.ReferencedColumns, len(constraint.Columns), len(constraint.Columns))
	case dto.SchemaConstraintCheck:
		return safeSchemaFragment(constraint.Expression, 4096)
	default:
		return false
	}
}

func validSchemaIndex(index dto.SchemaIndexDefinition) bool {
	return validIdentifier(index.Name) && validSchemaIdentifiers(index.Columns, 1, 32) && (index.Method == "" || validIdentifier(index.Method)) && (index.Predicate == "" || safeSchemaFragment(index.Predicate, 4096))
}

func validSchemaIdentifiers(values []string, minimum, maximum int) bool {
	if len(values) < minimum || len(values) > maximum {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validIdentifier(value) {
			return false
		}
		key := strings.ToLower(value)
		if _, exists := seen[key]; exists {
			return false
		}
		seen[key] = struct{}{}
	}
	return true
}

func validReferentialAction(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "no action", "restrict", "cascade", "set null", "set default":
		return true
	default:
		return false
	}
}

func safeSchemaFragment(value string, maximum int) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && len(value) <= maximum && !strings.ContainsRune(value, 0) && !strings.Contains(value, ";") && !strings.Contains(value, "--") && !strings.Contains(value, "/*") && !strings.Contains(value, "*/")
}

func boundedSchemaText(value string, maximum int) bool {
	return len(value) <= maximum && !strings.ContainsRune(value, 0)
}

func validSchemaToken(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character != '-' && character != '_' && (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func knownSchemaAction(kind dto.SchemaActionKind) bool {
	return schemaActionOrder(kind) > 0
}

func schemaActionOrder(kind dto.SchemaActionKind) int {
	switch kind {
	case dto.SchemaActionCreateTable:
		return 10
	case dto.SchemaActionRenameTable:
		return 20
	case dto.SchemaActionDropIndex:
		return 30
	case dto.SchemaActionDropConstraint:
		return 40
	case dto.SchemaActionAddColumn:
		return 50
	case dto.SchemaActionRenameColumn, dto.SchemaActionAlterColumnType, dto.SchemaActionSetColumnNullable, dto.SchemaActionSetColumnDefault, dto.SchemaActionSetColumnComment:
		return 60
	case dto.SchemaActionAddConstraint:
		return 70
	case dto.SchemaActionCreateIndex, dto.SchemaActionRebuildIndex, dto.SchemaActionAnalyzeIndex:
		return 80
	case dto.SchemaActionDropColumn:
		return 90
	case dto.SchemaActionDropTable:
		return 100
	default:
		return 0
	}
}

func validateEngineSchemaActions(engine entity.Engine, actions []dto.SchemaAction) error {
	if engine != entity.EnginePostgreSQL && engine != entity.EngineMySQL && engine != entity.EngineMariaDB {
		return apperror.NewUnsupported("schema management is unsupported for this database engine", nil)
	}
	if engine == entity.EngineMySQL || engine == entity.EngineMariaDB {
		for _, action := range actions {
			if action.Kind == dto.SchemaActionRebuildIndex {
				return apperror.NewUnsupported("index rebuild is unsupported for this database engine", nil)
			}
		}
	}
	return nil
}

func normalizeSchemaSteps(actions []dto.SchemaAction, input []dto.SchemaStep) ([]dto.SchemaStep, string, bool, error) {
	if len(input) < 1 || len(input) > 1000 {
		return nil, "", false, apperror.NewInternal("schema preview returned invalid steps", nil)
	}
	actionKinds := make(map[string]dto.SchemaActionKind, len(actions))
	for _, action := range actions {
		actionKinds[action.ID] = action.Kind
	}
	steps := append([]dto.SchemaStep(nil), input...)
	statements := make([]string, len(steps))
	destructive := false
	totalLength := 0
	for index := range steps {
		kind, exists := actionKinds[steps[index].ActionID]
		statement := strings.TrimSpace(steps[index].SQL)
		if !exists || steps[index].Kind != "" && steps[index].Kind != kind || statement == "" || len(statement) > 100000 || strings.ContainsRune(statement, 0) {
			return nil, "", false, apperror.NewInternal("schema preview returned invalid steps", nil)
		}
		totalLength += len(statement)
		if totalLength > 1024*1024 {
			return nil, "", false, apperror.NewInternal("schema preview is too large", nil)
		}
		steps[index].Position = index + 1
		steps[index].Kind = kind
		steps[index].SQL = statement
		steps[index].Destructive = steps[index].Destructive || destructiveSchemaAction(kind)
		destructive = destructive || steps[index].Destructive
		statements[index] = statement
	}
	return steps, strings.Join(statements, "\n"), destructive, nil
}

func destructiveSchemaAction(kind dto.SchemaActionKind) bool {
	switch kind {
	case dto.SchemaActionDropTable, dto.SchemaActionDropColumn, dto.SchemaActionDropConstraint, dto.SchemaActionDropIndex, dto.SchemaActionAlterColumnType:
		return true
	default:
		return false
	}
}

func hashSchemaPreview(connection entity.Connection, actions []dto.SchemaAction, steps []dto.SchemaStep, sqlText string, destructive bool) (string, error) {
	payload, err := json.Marshal(struct {
		ConnectionID string
		Engine       entity.Engine
		Host         string
		Port         int
		Database     string
		Username     string
		Actions      []dto.SchemaAction
		Steps        []dto.SchemaStep
		SQL          string
		Destructive  bool
	}{
		ConnectionID: connection.ID,
		Engine:       connection.Engine,
		Host:         connection.Host,
		Port:         connection.Port,
		Database:     connection.Database,
		Username:     connection.Username,
		Actions:      actions,
		Steps:        steps,
		SQL:          sqlText,
		Destructive:  destructive,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func parseSchemaPreviewHash(value string) ([]byte, error) {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return nil, apperror.NewValidation("invalid schema preview hash", nil)
	}
	decoded, err := hex.DecodeString(value[7:])
	if err != nil || len(decoded) != sha256.Size {
		return nil, apperror.NewValidation("invalid schema preview hash", err)
	}
	return decoded, nil
}

func schemaOperationError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return apperror.NewCancellation("schema operation was cancelled", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apperror.NewTimeout("schema operation timed out", err)
	}
	if errors.Is(err, ports.ErrNotFound) {
		return apperror.NewNotFound("schema object not found", err)
	}
	var applicationError *apperror.Error
	if !errors.As(err, &applicationError) {
		return apperror.NewInternal("schema operation failed", err)
	}
	messages := map[string]string{
		apperror.CodeValidation:            "schema change is invalid",
		apperror.CodeNotFound:              "schema object not found",
		apperror.CodeConflict:              "schema change conflicts with the current database state",
		apperror.CodeConnectionFailed:      "database connection failed",
		apperror.CodeConnectionRequired:    "database connection is not connected",
		apperror.CodeConnectionBusy:        "database connection is busy",
		apperror.CodeQueryCancelled:        "schema operation was cancelled",
		apperror.CodeQueryTimeout:          "schema operation timed out",
		apperror.CodeReadonlyViolation:     "schema changes are disabled for this read-only connection",
		apperror.CodePermissionDenied:      "schema change permission denied",
		apperror.CodeUnsupportedCapability: "schema change is unsupported",
		apperror.CodeTransportFailed:       "database transport failed",
		apperror.CodeInternal:              "schema operation failed",
	}
	message, exists := messages[applicationError.Code]
	if !exists {
		return apperror.NewInternal("schema operation failed", err)
	}
	return apperror.New(applicationError.Code, message, err, applicationError.Temporary)
}
