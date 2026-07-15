package engines

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
	"github.com/lib/pq"
)

var _ ports.SchemaRuntime = (*Manager)(nil)

func (manager *Manager) Preview(ctx context.Context, connection entity.Connection, password string, request ports.SchemaRuntimePreviewRequest) ([]dto.SchemaStep, error) {
	if err := validateSchemaRuntimeActions(request.Actions); err != nil {
		return nil, err
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return nil, err
	}
	var steps []dto.SchemaStep
	if connection.Engine == entity.EnginePostgreSQL {
		steps, err = generatePostgresSchemaSteps(connection, request.Actions)
	} else if connection.Engine == entity.EngineMySQL || connection.Engine == entity.EngineMariaDB {
		steps, err = generateMySQLSchemaSteps(ctx, database, connection, request.Actions)
	} else {
		return nil, apperror.NewUnsupported("schema management is unsupported for this database engine", nil)
	}
	if err != nil {
		return nil, schemaDriverError(ctx, err)
	}
	for index := range steps {
		steps[index].Position = index + 1
	}
	return steps, nil
}

func (manager *Manager) Apply(ctx context.Context, connection entity.Connection, password string, request ports.SchemaRuntimeApplyRequest) (ports.SchemaRuntimeApplyResult, error) {
	if connection.ReadOnly {
		return ports.SchemaRuntimeApplyResult{}, apperror.NewReadonly("schema changes are disabled for this read-only connection", nil)
	}
	if err := validateSchemaRuntimeApplyRequest(connection.Engine, request); err != nil {
		return ports.SchemaRuntimeApplyResult{}, err
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return ports.SchemaRuntimeApplyResult{}, err
	}
	if connection.Engine == entity.EnginePostgreSQL {
		return applyPostgresSchemaSteps(ctx, database, request.Steps)
	}
	if connection.Engine == entity.EngineMySQL || connection.Engine == entity.EngineMariaDB {
		return applyMySQLSchemaSteps(ctx, database, request.Steps)
	}
	return ports.SchemaRuntimeApplyResult{}, apperror.NewUnsupported("schema management is unsupported for this database engine", nil)
}

func applyPostgresSchemaSteps(ctx context.Context, database *sql.DB, steps []dto.SchemaStep) (ports.SchemaRuntimeApplyResult, error) {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return ports.SchemaRuntimeApplyResult{}, schemaDriverError(ctx, err)
	}
	for _, step := range steps {
		if _, err := transaction.ExecContext(ctx, step.SQL); err != nil {
			_ = transaction.Rollback()
			return ports.SchemaRuntimeApplyResult{}, schemaDriverError(ctx, err)
		}
	}
	if err := transaction.Commit(); err != nil {
		_ = transaction.Rollback()
		return ports.SchemaRuntimeApplyResult{}, schemaDriverError(ctx, err)
	}
	return ports.SchemaRuntimeApplyResult{AppliedSteps: len(steps)}, nil
}

func applyMySQLSchemaSteps(ctx context.Context, database *sql.DB, steps []dto.SchemaStep) (ports.SchemaRuntimeApplyResult, error) {
	applied := 0
	for _, step := range steps {
		if _, err := database.ExecContext(ctx, step.SQL); err != nil {
			return ports.SchemaRuntimeApplyResult{AppliedSteps: applied}, schemaDriverError(ctx, err)
		}
		applied++
	}
	return ports.SchemaRuntimeApplyResult{AppliedSteps: applied}, nil
}

func validateSchemaRuntimeActions(actions []dto.SchemaAction) error {
	if len(actions) < 1 || len(actions) > 100 {
		return apperror.NewValidation("invalid schema preview request", nil)
	}
	lastRank := 0
	seen := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		rank := schemaRuntimeActionRank(action.Kind)
		if rank == 0 || rank < lastRank || !schemaRuntimeToken(action.ID, 64) {
			return apperror.NewValidation("invalid ordered schema actions", nil)
		}
		if _, exists := seen[action.ID]; exists {
			return apperror.NewValidation("duplicate schema action identifier", nil)
		}
		seen[action.ID] = struct{}{}
		lastRank = rank
	}
	return nil
}

func validateSchemaRuntimeApplyRequest(engine entity.Engine, request ports.SchemaRuntimeApplyRequest) error {
	if !validSchemaRuntimeHash(request.PreviewHash) || len(request.Steps) < 1 || len(request.Steps) > 1000 {
		return apperror.NewValidation("invalid schema apply request", nil)
	}
	total := 0
	for index, step := range request.Steps {
		statement := strings.TrimSpace(step.SQL)
		if step.Position != index+1 || !schemaRuntimeToken(step.ActionID, 64) || schemaRuntimeActionRank(step.Kind) == 0 || statement == "" || statement != step.SQL || len(statement) > 100000 || strings.ContainsRune(statement, 0) || !schemaStatementIsSingle(engine, statement) || !schemaStatementMatchesKind(engine, step.Kind, statement) {
			return apperror.NewValidation("invalid schema apply step", nil)
		}
		if schemaRuntimeActionDestructive(step.Kind) && !step.Destructive {
			return apperror.NewValidation("invalid destructive schema apply step", nil)
		}
		total += len(statement)
		if total > 1024*1024 {
			return apperror.NewValidation("schema apply request is too large", nil)
		}
	}
	return nil
}

func validSchemaRuntimeHash(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	decoded, err := hex.DecodeString(value[len("sha256:"):])
	return err == nil && len(decoded) == sha256.Size
}

func schemaRuntimeToken(value string, maximum int) bool {
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

func schemaRuntimeIdentifier(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index, character := range value {
		if character != '_' && (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') && (index == 0 || character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func schemaReferentialAction(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "no action":
		return "NO ACTION", true
	case "restrict":
		return "RESTRICT", true
	case "cascade":
		return "CASCADE", true
	case "set null":
		return "SET NULL", true
	case "set default":
		return "SET DEFAULT", true
	default:
		return "", false
	}
}

func schemaResolveRuntimeTarget(aliases map[string]dto.SchemaTarget, target dto.SchemaTarget, fallbackSchema string) (dto.SchemaTarget, error) {
	if target.Schema == "" {
		target.Schema = fallbackSchema
	}
	for index := 0; index <= len(aliases); index++ {
		next, exists := aliases[schemaRuntimeTargetKey(target)]
		if !exists {
			return target, nil
		}
		target = next
	}
	return dto.SchemaTarget{}, apperror.NewValidation("schema table rename cycle is invalid", nil)
}

func schemaTrackRuntimeRename(aliases map[string]dto.SchemaTarget, target dto.SchemaTarget, newName string) {
	aliases[schemaRuntimeTargetKey(target)] = dto.SchemaTarget{Schema: target.Schema, Table: newName}
}

func schemaRuntimeTargetKey(target dto.SchemaTarget) string {
	return target.Schema + "\x00" + target.Table
}

func schemaRuntimeActionRank(kind dto.SchemaActionKind) int {
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

func schemaRuntimeActionDestructive(kind dto.SchemaActionKind) bool {
	switch kind {
	case dto.SchemaActionDropTable, dto.SchemaActionDropColumn, dto.SchemaActionDropConstraint, dto.SchemaActionDropIndex, dto.SchemaActionAlterColumnType:
		return true
	default:
		return false
	}
}

func schemaStep(action dto.SchemaAction, statement string) dto.SchemaStep {
	return dto.SchemaStep{ActionID: action.ID, Kind: action.Kind, SQL: statement, Destructive: schemaRuntimeActionDestructive(action.Kind)}
}

func schemaStatementMatchesKind(engine entity.Engine, kind dto.SchemaActionKind, statement string) bool {
	upper := strings.ToUpper(strings.TrimSpace(statement))
	switch kind {
	case dto.SchemaActionCreateTable:
		return strings.HasPrefix(upper, "CREATE TABLE ") || engine == entity.EnginePostgreSQL && strings.HasPrefix(upper, "COMMENT ON COLUMN ")
	case dto.SchemaActionRenameTable:
		return strings.HasPrefix(upper, "ALTER TABLE ") || strings.HasPrefix(upper, "RENAME TABLE ")
	case dto.SchemaActionDropTable:
		return strings.HasPrefix(upper, "DROP TABLE ")
	case dto.SchemaActionAddColumn:
		return strings.HasPrefix(upper, "ALTER TABLE ") || engine == entity.EnginePostgreSQL && strings.HasPrefix(upper, "COMMENT ON COLUMN ")
	case dto.SchemaActionRenameColumn, dto.SchemaActionAlterColumnType, dto.SchemaActionSetColumnNullable, dto.SchemaActionSetColumnDefault, dto.SchemaActionDropColumn, dto.SchemaActionAddConstraint, dto.SchemaActionDropConstraint:
		return strings.HasPrefix(upper, "ALTER TABLE ")
	case dto.SchemaActionSetColumnComment:
		return strings.HasPrefix(upper, "ALTER TABLE ") || engine == entity.EnginePostgreSQL && strings.HasPrefix(upper, "COMMENT ON COLUMN ")
	case dto.SchemaActionCreateIndex:
		return strings.HasPrefix(upper, "CREATE INDEX ") || strings.HasPrefix(upper, "CREATE UNIQUE INDEX ") || strings.HasPrefix(upper, "CREATE FULLTEXT INDEX ") || strings.HasPrefix(upper, "CREATE SPATIAL INDEX ")
	case dto.SchemaActionDropIndex:
		return strings.HasPrefix(upper, "DROP INDEX ")
	case dto.SchemaActionRebuildIndex:
		return engine == entity.EnginePostgreSQL && strings.HasPrefix(upper, "REINDEX INDEX ")
	case dto.SchemaActionAnalyzeIndex:
		return strings.HasPrefix(upper, "ANALYZE ")
	default:
		return false
	}
}

func schemaStatementIsSingle(engine entity.Engine, statement string) bool {
	depth := 0
	for index := 0; index < len(statement); {
		character := statement[index]
		if character == '\'' || character == '"' || character == '`' {
			next, ok := schemaQuotedEnd(statement, index, character)
			if !ok {
				return false
			}
			index = next
			continue
		}
		if character == ';' || schemaCommentAt(statement, index) || (engine == entity.EngineMySQL || engine == entity.EngineMariaDB) && character == '#' {
			return false
		}
		switch character {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
		index++
	}
	return depth == 0
}

func schemaDataTypeSafe(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 || strings.ContainsRune(value, 0) {
		return false
	}
	depth := 0
	var word strings.Builder
	words := make([]string, 0)
	flush := func() {
		if word.Len() > 0 {
			words = append(words, strings.ToLower(word.String()))
			word.Reset()
		}
	}
	for index := 0; index < len(value); {
		character, size := utf8.DecodeRuneInString(value[index:])
		if character == utf8.RuneError && size == 1 {
			return false
		}
		if character == '\'' || character == '"' || character == '`' {
			flush()
			if character == '\'' && depth == 0 {
				return false
			}
			next, ok := schemaQuotedEnd(value, index, byte(character))
			if !ok {
				return false
			}
			index = next
			continue
		}
		if character == ';' || schemaCommentAt(value, index) {
			return false
		}
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '_' {
			word.WriteRune(character)
			index += size
			continue
		}
		flush()
		switch character {
		case '(':
			depth++
			if depth > 2 {
				return false
			}
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		case ',':
			if depth == 0 {
				return false
			}
		case ' ', '\t', '\n', '\r', '.', '[', ']':
		default:
			return false
		}
		index += size
	}
	flush()
	if depth != 0 {
		return false
	}
	dangerous := map[string]bool{"add": true, "alter": true, "auto_increment": true, "check": true, "column": true, "comment": true, "constraint": true, "create": true, "default": true, "delete": true, "drop": true, "generated": true, "identity": true, "key": true, "not": true, "null": true, "on": true, "primary": true, "references": true, "rename": true, "unique": true, "update": true}
	for _, token := range words {
		if dangerous[token] {
			return false
		}
	}
	return true
}

func schemaExpressionSafe(value string) bool {
	return schemaExpressionSafeDialect(value, false)
}

func schemaMySQLExpressionSafe(value string) bool {
	return schemaExpressionSafeDialect(value, true)
}

func schemaExpressionSafeDialect(value string, rejectHashComment bool) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 4096 || strings.ContainsRune(value, 0) {
		return false
	}
	depth := 0
	bracketDepth := 0
	for index := 0; index < len(value); {
		character, size := utf8.DecodeRuneInString(value[index:])
		if character == utf8.RuneError && size == 1 {
			return false
		}
		if character == '\'' || character == '"' || character == '`' {
			next, ok := schemaQuotedEnd(value, index, byte(character))
			if !ok {
				return false
			}
			index = next
			continue
		}
		if character == ';' || schemaCommentAt(value, index) || rejectHashComment && character == '#' {
			return false
		}
		switch character {
		case '(':
			depth++
			if depth > 64 {
				return false
			}
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		case '[':
			bracketDepth++
		case ']':
			bracketDepth--
			if bracketDepth < 0 {
				return false
			}
		case ',':
			if depth == 0 && bracketDepth == 0 {
				return false
			}
		}
		index += size
	}
	return depth == 0 && bracketDepth == 0
}

func schemaQuotedEnd(value string, start int, quote byte) (int, bool) {
	for index := start + 1; index < len(value); index++ {
		if value[index] != quote {
			continue
		}
		if index+1 < len(value) && value[index+1] == quote {
			index++
			continue
		}
		return index + 1, true
	}
	return 0, false
}

func schemaCommentAt(value string, index int) bool {
	if index+1 >= len(value) {
		return false
	}
	return value[index] == '-' && value[index+1] == '-' || value[index] == '/' && value[index+1] == '*' || value[index] == '*' && value[index+1] == '/'
}

func schemaDriverError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
		return apperror.NewCancellation("schema operation was cancelled", err)
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return apperror.NewTimeout("schema operation timed out", err)
	}
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return err
	}
	var postgresError *pq.Error
	if errors.As(err, &postgresError) {
		return postgresSchemaError(postgresError, err)
	}
	var mysqlError *mysqldriver.MySQLError
	if errors.As(err, &mysqlError) {
		return mysqlSchemaError(mysqlError, err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return apperror.NewNotFound("schema object not found", err)
	}
	return apperror.NewInternal("schema operation failed", err)
}

func postgresSchemaError(driverError *pq.Error, cause error) error {
	switch string(driverError.Code) {
	case "42501":
		return apperror.NewPermission("schema change permission denied", cause)
	case "42P01", "42703", "42704", "3F000":
		return apperror.NewNotFound("schema object not found", cause)
	case "42601", "42804", "42846", "42P16", "22023":
		return apperror.NewValidation("schema change is invalid", cause)
	case "42P07", "42701", "42710", "23502", "23503", "23505", "23514", "2BP01", "55006":
		return apperror.NewConflict("schema change conflicts with the current database state", cause)
	case "0A000":
		return apperror.NewUnsupported("schema change is unsupported by this database version", cause)
	case "55P03", "40P01", "40001":
		return apperror.NewConnectionBusy("schema object is busy", cause)
	case "57014":
		return apperror.NewCancellation("schema operation was cancelled", cause)
	default:
		return apperror.NewInternal("schema operation failed", cause)
	}
}

func mysqlSchemaError(driverError *mysqldriver.MySQLError, cause error) error {
	switch driverError.Number {
	case 1044, 1045, 1142, 1143, 1227, 1370:
		return apperror.NewPermission("schema change permission denied", cause)
	case 1051, 1054, 1091, 1146:
		return apperror.NewNotFound("schema object not found", cause)
	case 1072:
		return apperror.NewNotFound("schema column was not found", cause)
	case 1064, 1067, 1071, 1075, 1171, 1210, 1582, 1901:
		return apperror.NewValidation("schema change is invalid", cause)
	case 1025, 1050, 1060, 1061, 1215, 1216, 1217, 1412, 1451, 1452, 1826, 3819, 4025:
		return apperror.NewConflict("schema change conflicts with the current database state", cause)
	case 1205, 1213:
		return apperror.NewConnectionBusy("schema object is busy", cause)
	case 1235:
		return apperror.NewUnsupported("schema change is unsupported by this database version", cause)
	case 1317:
		return apperror.NewCancellation("schema operation was cancelled", cause)
	default:
		return apperror.NewInternal("schema operation failed", cause)
	}
}
