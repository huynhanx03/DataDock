package engines

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type mysqlSchemaColumnState struct {
	dataType             string
	nullable             bool
	defaultValue         sql.NullString
	defaultSQL           sql.NullString
	extra                string
	comment              string
	generationExpression string
	characterSet         sql.NullString
	collation            sql.NullString
}

type mysqlSchemaPreviewState struct {
	database             *sql.DB
	connection           entity.Connection
	columns              map[string]mysqlSchemaColumnState
	constraints          map[string]string
	tableSources         map[string]string
	generationChecked    bool
	generationExpression bool
}

func generateMySQLSchemaSteps(ctx context.Context, database *sql.DB, connection entity.Connection, actions []dto.SchemaAction) ([]dto.SchemaStep, error) {
	state := &mysqlSchemaPreviewState{database: database, connection: connection, columns: make(map[string]mysqlSchemaColumnState), constraints: make(map[string]string), tableSources: make(map[string]string)}
	steps := make([]dto.SchemaStep, 0, len(actions))
	aliases := make(map[string]dto.SchemaTarget)
	for _, original := range actions {
		action := original
		resolved, err := schemaResolveRuntimeTarget(aliases, action.Target, connection.Database)
		if err != nil {
			return nil, err
		}
		action.Target = resolved
		if action.Constraint != nil && action.Constraint.ReferencedTarget != nil {
			constraint := *action.Constraint
			referencedTarget, err := schemaResolveRuntimeTarget(aliases, *constraint.ReferencedTarget, connection.Database)
			if err != nil {
				return nil, err
			}
			constraint.ReferencedTarget = &referencedTarget
			action.Constraint = &constraint
		}
		generated, err := state.generateAction(ctx, action)
		if err != nil {
			return nil, err
		}
		steps = append(steps, generated...)
		if action.Kind == dto.SchemaActionRenameTable {
			schemaTrackRuntimeRename(aliases, action.Target, action.NewName)
		}
	}
	return steps, nil
}

func (state *mysqlSchemaPreviewState) generateAction(ctx context.Context, action dto.SchemaAction) ([]dto.SchemaStep, error) {
	dialect := mysqlDialect{}
	target, databaseName, err := mysqlSchemaQualifiedTarget(state.connection, action.Target)
	if err != nil {
		return nil, err
	}
	one := func(statement string) []dto.SchemaStep {
		return []dto.SchemaStep{schemaStep(action, statement)}
	}
	switch action.Kind {
	case dto.SchemaActionCreateTable:
		if len(action.Columns) < 1 || len(action.Columns) > 200 {
			return nil, apperror.NewValidation("invalid MySQL table definition", nil)
		}
		definitions := make([]string, 0, len(action.Columns))
		identities := 0
		for _, column := range action.Columns {
			if column.Identity {
				identities++
			}
			definition, err := mysqlSchemaNewColumnDefinition(column)
			if err != nil {
				return nil, err
			}
			definitions = append(definitions, definition)
			state.setColumn(databaseName, action.Target.Table, column.Name, mysqlSchemaStateFromDefinition(column))
		}
		if identities > 1 {
			return nil, apperror.NewValidation("MySQL tables support one identity column", nil)
		}
		return one("CREATE TABLE " + target + " (" + strings.Join(definitions, ", ") + ")"), nil
	case dto.SchemaActionRenameTable:
		if !schemaRuntimeIdentifier(action.NewName) {
			return nil, apperror.NewValidation("invalid MySQL table rename", nil)
		}
		newTarget := dialect.quoteIdentifier(databaseName) + "." + dialect.quoteIdentifier(action.NewName)
		state.renameTable(databaseName, action.Target.Table, action.NewName)
		return one("RENAME TABLE " + target + " TO " + newTarget), nil
	case dto.SchemaActionDropTable:
		if action.Cascade {
			return nil, apperror.NewUnsupported("cascading table drop is unsupported by MySQL and MariaDB", nil)
		}
		return one("DROP TABLE " + target), nil
	case dto.SchemaActionAddColumn:
		if action.Column == nil {
			return nil, apperror.NewValidation("MySQL column definition is required", nil)
		}
		definition, err := mysqlSchemaNewColumnDefinition(*action.Column)
		if err != nil {
			return nil, err
		}
		state.setColumn(databaseName, action.Target.Table, action.Column.Name, mysqlSchemaStateFromDefinition(*action.Column))
		return one("ALTER TABLE " + target + " ADD COLUMN " + definition), nil
	case dto.SchemaActionRenameColumn:
		if !schemaRuntimeIdentifier(action.Name) || !schemaRuntimeIdentifier(action.NewName) {
			return nil, apperror.NewValidation("invalid MySQL column rename", nil)
		}
		column, err := state.column(ctx, databaseName, action.Target.Table, action.Name)
		if err != nil {
			return nil, err
		}
		state.deleteColumn(databaseName, action.Target.Table, action.Name)
		state.setColumn(databaseName, action.Target.Table, action.NewName, column)
		return one("ALTER TABLE " + target + " RENAME COLUMN " + dialect.quoteIdentifier(action.Name) + " TO " + dialect.quoteIdentifier(action.NewName)), nil
	case dto.SchemaActionAlterColumnType:
		if !schemaRuntimeIdentifier(action.Name) || !schemaDataTypeSafe(action.DataType) {
			return nil, apperror.NewValidation("invalid MySQL column type", nil)
		}
		column, err := state.column(ctx, databaseName, action.Target.Table, action.Name)
		if err != nil {
			return nil, err
		}
		if strings.Contains(strings.ToLower(column.extra), "auto_increment") && !mysqlSchemaIntegerType(action.DataType) {
			return nil, apperror.NewValidation("MySQL identity columns require an integer type", nil)
		}
		definition, err := mysqlSchemaExistingColumnDefinition(action.Name, column, action.DataType, nil, nil)
		if err != nil {
			return nil, err
		}
		column.dataType = strings.TrimSpace(action.DataType)
		if !mysqlSchemaCharacterType(action.DataType) {
			column.characterSet = sql.NullString{}
			column.collation = sql.NullString{}
		}
		state.setColumn(databaseName, action.Target.Table, action.Name, column)
		return one("ALTER TABLE " + target + " MODIFY COLUMN " + definition), nil
	case dto.SchemaActionSetColumnNullable:
		if !schemaRuntimeIdentifier(action.Name) || action.Nullable == nil {
			return nil, apperror.NewValidation("invalid MySQL nullable change", nil)
		}
		column, err := state.column(ctx, databaseName, action.Target.Table, action.Name)
		if err != nil {
			return nil, err
		}
		if *action.Nullable && strings.Contains(strings.ToLower(column.extra), "auto_increment") {
			return nil, apperror.NewValidation("MySQL identity columns cannot be nullable", nil)
		}
		definition, err := mysqlSchemaExistingColumnDefinition(action.Name, column, "", action.Nullable, nil)
		if err != nil {
			return nil, err
		}
		column.nullable = *action.Nullable
		state.setColumn(databaseName, action.Target.Table, action.Name, column)
		return one("ALTER TABLE " + target + " MODIFY COLUMN " + definition), nil
	case dto.SchemaActionSetColumnDefault:
		if !schemaRuntimeIdentifier(action.Name) {
			return nil, apperror.NewValidation("invalid MySQL default change", nil)
		}
		column, err := state.column(ctx, databaseName, action.Target.Table, action.Name)
		if err != nil {
			return nil, err
		}
		if action.DefaultValue != nil && (column.generationExpression != "" || strings.Contains(strings.ToLower(column.extra), "auto_increment")) {
			return nil, apperror.NewValidation("MySQL generated and identity columns cannot define a default", nil)
		}
		statement := "ALTER TABLE " + target + " ALTER COLUMN " + dialect.quoteIdentifier(action.Name)
		if action.DefaultValue == nil {
			column.defaultSQL = sql.NullString{}
			state.setColumn(databaseName, action.Target.Table, action.Name, column)
			return one(statement + " DROP DEFAULT"), nil
		}
		if !schemaMySQLExpressionSafe(*action.DefaultValue) {
			return nil, apperror.NewValidation("invalid MySQL default expression", nil)
		}
		value := strings.TrimSpace(*action.DefaultValue)
		column.defaultSQL = sql.NullString{String: value, Valid: true}
		state.setColumn(databaseName, action.Target.Table, action.Name, column)
		return one(statement + " SET DEFAULT " + value), nil
	case dto.SchemaActionSetColumnComment:
		if !schemaRuntimeIdentifier(action.Name) || action.Comment == nil || len(*action.Comment) > 1024 || strings.ContainsRune(*action.Comment, 0) {
			return nil, apperror.NewValidation("invalid MySQL column comment", nil)
		}
		column, err := state.column(ctx, databaseName, action.Target.Table, action.Name)
		if err != nil {
			return nil, err
		}
		definition, err := mysqlSchemaExistingColumnDefinition(action.Name, column, "", nil, action.Comment)
		if err != nil {
			return nil, err
		}
		column.comment = *action.Comment
		state.setColumn(databaseName, action.Target.Table, action.Name, column)
		return one("ALTER TABLE " + target + " MODIFY COLUMN " + definition), nil
	case dto.SchemaActionDropColumn:
		if !schemaRuntimeIdentifier(action.Name) {
			return nil, apperror.NewValidation("invalid MySQL column drop", nil)
		}
		if action.Cascade {
			return nil, apperror.NewUnsupported("cascading column drop is unsupported by MySQL and MariaDB", nil)
		}
		state.deleteColumn(databaseName, action.Target.Table, action.Name)
		return one("ALTER TABLE " + target + " DROP COLUMN " + dialect.quoteIdentifier(action.Name)), nil
	case dto.SchemaActionAddConstraint:
		if action.Constraint == nil {
			return nil, apperror.NewValidation("MySQL constraint definition is required", nil)
		}
		definition, err := mysqlSchemaConstraint(state.connection, *action.Constraint, databaseName)
		if err != nil {
			return nil, err
		}
		state.setConstraint(databaseName, action.Target.Table, action.Constraint.Name, mysqlSchemaConstraintType(action.Constraint.Type))
		return one("ALTER TABLE " + target + " ADD CONSTRAINT " + dialect.quoteIdentifier(action.Constraint.Name) + " " + definition), nil
	case dto.SchemaActionDropConstraint:
		if !schemaRuntimeIdentifier(action.Name) {
			return nil, apperror.NewValidation("invalid MySQL constraint drop", nil)
		}
		if action.Cascade {
			return nil, apperror.NewUnsupported("cascading constraint drop is unsupported by MySQL and MariaDB", nil)
		}
		constraintType, err := state.constraintType(ctx, databaseName, action.Target.Table, action.Name)
		if err != nil {
			return nil, err
		}
		operation, err := mysqlSchemaDropConstraintOperation(state.connection.Engine, constraintType, action.Name)
		if err != nil {
			return nil, err
		}
		state.deleteConstraint(databaseName, action.Target.Table, action.Name)
		return one("ALTER TABLE " + target + " " + operation), nil
	case dto.SchemaActionCreateIndex:
		if action.Index == nil {
			return nil, apperror.NewValidation("MySQL index definition is required", nil)
		}
		statement, err := mysqlSchemaCreateIndex(target, *action.Index)
		if err != nil {
			return nil, err
		}
		return one(statement), nil
	case dto.SchemaActionDropIndex:
		if !schemaRuntimeIdentifier(action.Name) {
			return nil, apperror.NewValidation("invalid MySQL index drop", nil)
		}
		if action.Cascade {
			return nil, apperror.NewUnsupported("cascading index drop is unsupported by MySQL and MariaDB", nil)
		}
		return one("DROP INDEX " + dialect.quoteIdentifier(action.Name) + " ON " + target), nil
	case dto.SchemaActionRebuildIndex:
		return nil, apperror.NewUnsupported("individual index rebuild is unsupported by MySQL and MariaDB", nil)
	case dto.SchemaActionAnalyzeIndex:
		if !schemaRuntimeIdentifier(action.Name) {
			return nil, apperror.NewValidation("invalid MySQL index analysis", nil)
		}
		return one("ANALYZE TABLE " + target), nil
	default:
		return nil, apperror.NewUnsupported("schema action is unsupported by MySQL and MariaDB", nil)
	}
}

func mysqlSchemaQualifiedTarget(connection entity.Connection, target dto.SchemaTarget) (string, string, error) {
	databaseName := target.Schema
	if databaseName == "" {
		databaseName = connection.Database
	}
	if !schemaRuntimeIdentifier(databaseName) || !schemaRuntimeIdentifier(target.Table) {
		return "", "", apperror.NewValidation("invalid MySQL schema target", nil)
	}
	dialect := mysqlDialect{}
	return dialect.quoteIdentifier(databaseName) + "." + dialect.quoteIdentifier(target.Table), databaseName, nil
}

func mysqlSchemaNewColumnDefinition(column dto.SchemaColumnDefinition) (string, error) {
	if !schemaRuntimeIdentifier(column.Name) || !schemaDataTypeSafe(column.DataType) || len(column.Comment) > 1024 || strings.ContainsRune(column.Comment, 0) || column.Identity && (column.Nullable || column.GeneratedExpression != nil || !mysqlSchemaIntegerType(column.DataType)) || (column.Identity || column.GeneratedExpression != nil) && column.DefaultValue != nil {
		return "", apperror.NewValidation("invalid MySQL column definition", nil)
	}
	dialect := mysqlDialect{}
	definition := dialect.quoteIdentifier(column.Name) + " " + strings.TrimSpace(column.DataType)
	if column.GeneratedExpression != nil {
		if !schemaMySQLExpressionSafe(*column.GeneratedExpression) {
			return "", apperror.NewValidation("invalid MySQL generated expression", nil)
		}
		definition += " GENERATED ALWAYS AS (" + strings.TrimSpace(*column.GeneratedExpression) + ") STORED"
		if column.Nullable {
			definition += " NULL"
		} else {
			definition += " NOT NULL"
		}
	} else {
		if column.Nullable {
			definition += " NULL"
		} else {
			definition += " NOT NULL"
		}
		if column.DefaultValue != nil {
			if !schemaMySQLExpressionSafe(*column.DefaultValue) {
				return "", apperror.NewValidation("invalid MySQL default expression", nil)
			}
			definition += " DEFAULT " + strings.TrimSpace(*column.DefaultValue)
		}
		if column.Identity {
			definition += " AUTO_INCREMENT UNIQUE"
		}
	}
	if column.Comment != "" {
		definition += " COMMENT " + mysqlSchemaQuoteLiteral(column.Comment)
	}
	return definition, nil
}

func mysqlSchemaExistingColumnDefinition(name string, column mysqlSchemaColumnState, dataType string, nullable *bool, comment *string) (string, error) {
	if !schemaRuntimeIdentifier(name) {
		return "", apperror.NewValidation("invalid MySQL column name", nil)
	}
	if dataType == "" {
		dataType = column.dataType
	}
	if !schemaDataTypeSafe(dataType) {
		return "", apperror.NewValidation("invalid MySQL column type metadata", nil)
	}
	resolvedNullable := column.nullable
	if nullable != nil {
		resolvedNullable = *nullable
	}
	resolvedComment := column.comment
	if comment != nil {
		resolvedComment = *comment
	}
	if len(resolvedComment) > 1024 || strings.ContainsRune(resolvedComment, 0) {
		return "", apperror.NewValidation("invalid MySQL column comment", nil)
	}
	dialect := mysqlDialect{}
	definition := dialect.quoteIdentifier(name) + " " + strings.TrimSpace(dataType)
	characterType := mysqlSchemaCharacterType(dataType)
	if characterType && column.characterSet.Valid && schemaRuntimeIdentifier(column.characterSet.String) {
		definition += " CHARACTER SET " + dialect.quoteIdentifier(column.characterSet.String)
	}
	if characterType && column.collation.Valid && schemaRuntimeIdentifier(column.collation.String) {
		definition += " COLLATE " + dialect.quoteIdentifier(column.collation.String)
	}
	lowerExtra := strings.ToLower(column.extra)
	if column.generationExpression != "" {
		if !schemaMySQLExpressionSafe(column.generationExpression) {
			return "", apperror.NewInternal("database returned unsafe generated column metadata", nil)
		}
		definition += " GENERATED ALWAYS AS (" + strings.TrimSpace(column.generationExpression) + ")"
		if strings.Contains(lowerExtra, "stored generated") || strings.Contains(lowerExtra, "persistent") {
			definition += " STORED"
		} else {
			definition += " VIRTUAL"
		}
		if resolvedNullable {
			definition += " NULL"
		} else {
			definition += " NOT NULL"
		}
		if strings.Contains(lowerExtra, "invisible") {
			definition += " INVISIBLE"
		}
	} else {
		if resolvedNullable {
			definition += " NULL"
		} else {
			definition += " NOT NULL"
		}
		if column.defaultSQL.Valid {
			definition += " DEFAULT " + column.defaultSQL.String
		}
		if updateExpression := mysqlSchemaOnUpdate(column.extra); updateExpression != "" {
			if !schemaMySQLExpressionSafe(updateExpression) {
				return "", apperror.NewInternal("database returned unsafe update expression metadata", nil)
			}
			definition += " ON UPDATE " + updateExpression
		}
		if strings.Contains(lowerExtra, "invisible") {
			definition += " INVISIBLE"
		}
		if strings.Contains(lowerExtra, "auto_increment") {
			definition += " AUTO_INCREMENT"
		}
	}
	if resolvedComment != "" {
		definition += " COMMENT " + mysqlSchemaQuoteLiteral(resolvedComment)
	}
	return definition, nil
}

func (state *mysqlSchemaPreviewState) column(ctx context.Context, databaseName, tableName, columnName string) (mysqlSchemaColumnState, error) {
	key := mysqlSchemaColumnKey(databaseName, tableName, columnName)
	if column, exists := state.columns[key]; exists {
		return column, nil
	}
	if !state.generationChecked {
		available, err := mysqlInformationSchemaColumnExists(ctx, state.database, "COLUMNS", "GENERATION_EXPRESSION")
		if err != nil {
			return mysqlSchemaColumnState{}, err
		}
		state.generationExpression = available
		state.generationChecked = true
	}
	generation := "''"
	if state.generationExpression {
		generation = "COALESCE(generation_expression, '')"
	}
	query := "SELECT column_type, is_nullable, column_default, COALESCE(extra, ''), COALESCE(column_comment, ''), " + generation + ", character_set_name, collation_name FROM information_schema.columns WHERE table_schema = ? AND table_name = ? AND column_name = ?"
	var column mysqlSchemaColumnState
	var nullable string
	err := state.database.QueryRowContext(ctx, query, databaseName, state.sourceTable(databaseName, tableName), columnName).Scan(&column.dataType, &nullable, &column.defaultValue, &column.extra, &column.comment, &column.generationExpression, &column.characterSet, &column.collation)
	if errors.Is(err, sql.ErrNoRows) {
		return mysqlSchemaColumnState{}, apperror.NewNotFound("MySQL column was not found", err)
	}
	if err != nil {
		return mysqlSchemaColumnState{}, err
	}
	column.nullable = strings.EqualFold(nullable, "YES")
	if column.defaultValue.Valid {
		defaultSQL, err := mysqlSchemaMetadataDefault(state.connection.Engine, column)
		if err != nil {
			return mysqlSchemaColumnState{}, err
		}
		column.defaultSQL = sql.NullString{String: defaultSQL, Valid: true}
	}
	state.columns[key] = column
	return column, nil
}

func (state *mysqlSchemaPreviewState) setColumn(databaseName, tableName, columnName string, column mysqlSchemaColumnState) {
	state.columns[mysqlSchemaColumnKey(databaseName, tableName, columnName)] = column
}

func (state *mysqlSchemaPreviewState) deleteColumn(databaseName, tableName, columnName string) {
	delete(state.columns, mysqlSchemaColumnKey(databaseName, tableName, columnName))
}

func mysqlSchemaColumnKey(databaseName, tableName, columnName string) string {
	return databaseName + "\x00" + tableName + "\x00" + columnName
}

func mysqlSchemaTableKey(databaseName, tableName string) string {
	return databaseName + "\x00" + tableName
}

func (state *mysqlSchemaPreviewState) sourceTable(databaseName, tableName string) string {
	if source, exists := state.tableSources[mysqlSchemaTableKey(databaseName, tableName)]; exists {
		return source
	}
	return tableName
}

func (state *mysqlSchemaPreviewState) renameTable(databaseName, oldName, newName string) {
	oldPrefix := mysqlSchemaColumnKey(databaseName, oldName, "")
	newPrefix := mysqlSchemaColumnKey(databaseName, newName, "")
	for key, column := range state.columns {
		if !strings.HasPrefix(key, oldPrefix) {
			continue
		}
		delete(state.columns, key)
		state.columns[newPrefix+strings.TrimPrefix(key, oldPrefix)] = column
	}
	for key, constraintType := range state.constraints {
		if !strings.HasPrefix(key, oldPrefix) {
			continue
		}
		delete(state.constraints, key)
		state.constraints[newPrefix+strings.TrimPrefix(key, oldPrefix)] = constraintType
	}
	oldKey := mysqlSchemaTableKey(databaseName, oldName)
	newKey := mysqlSchemaTableKey(databaseName, newName)
	state.tableSources[newKey] = state.sourceTable(databaseName, oldName)
	delete(state.tableSources, oldKey)
}

func mysqlSchemaStateFromDefinition(column dto.SchemaColumnDefinition) mysqlSchemaColumnState {
	state := mysqlSchemaColumnState{dataType: strings.TrimSpace(column.DataType), nullable: column.Nullable, comment: column.Comment}
	if column.DefaultValue != nil {
		state.defaultSQL = sql.NullString{String: strings.TrimSpace(*column.DefaultValue), Valid: true}
	}
	if column.GeneratedExpression != nil {
		state.generationExpression = strings.TrimSpace(*column.GeneratedExpression)
		state.extra = "STORED GENERATED"
	}
	if column.Identity {
		state.extra = "auto_increment"
	}
	return state
}

func (state *mysqlSchemaPreviewState) constraintType(ctx context.Context, databaseName, tableName, constraintName string) (string, error) {
	key := mysqlSchemaColumnKey(databaseName, tableName, constraintName)
	if constraintType, exists := state.constraints[key]; exists {
		return constraintType, nil
	}
	var constraintType string
	err := state.database.QueryRowContext(ctx, "SELECT constraint_type FROM information_schema.table_constraints WHERE table_schema = ? AND table_name = ? AND constraint_name = ?", databaseName, state.sourceTable(databaseName, tableName), constraintName).Scan(&constraintType)
	if errors.Is(err, sql.ErrNoRows) {
		return "", apperror.NewNotFound("MySQL constraint was not found", err)
	}
	if err != nil {
		return "", err
	}
	state.constraints[key] = constraintType
	return constraintType, nil
}

func (state *mysqlSchemaPreviewState) setConstraint(databaseName, tableName, constraintName, constraintType string) {
	state.constraints[mysqlSchemaColumnKey(databaseName, tableName, constraintName)] = constraintType
}

func (state *mysqlSchemaPreviewState) deleteConstraint(databaseName, tableName, constraintName string) {
	delete(state.constraints, mysqlSchemaColumnKey(databaseName, tableName, constraintName))
}

func mysqlSchemaConstraintType(constraintType dto.SchemaConstraintType) string {
	switch constraintType {
	case dto.SchemaConstraintPrimaryKey:
		return "PRIMARY KEY"
	case dto.SchemaConstraintForeignKey:
		return "FOREIGN KEY"
	case dto.SchemaConstraintUnique:
		return "UNIQUE"
	case dto.SchemaConstraintCheck:
		return "CHECK"
	default:
		return ""
	}
}

func mysqlSchemaMetadataDefault(engine entity.Engine, column mysqlSchemaColumnState) (string, error) {
	value := strings.TrimSpace(column.defaultValue.String)
	lowerType := strings.ToLower(column.dataType)
	lowerExtra := strings.ToLower(column.extra)
	if engine == entity.EngineMariaDB {
		if !schemaMySQLExpressionSafe(value) {
			return "", apperror.NewInternal("database returned unsafe default metadata", nil)
		}
		return value, nil
	}
	if strings.Contains(lowerExtra, "default_generated") || strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") || mysqlSchemaTemporalDefault(lowerType, value) {
		if !schemaMySQLExpressionSafe(value) {
			return "", apperror.NewInternal("database returned unsafe default expression metadata", nil)
		}
		return value, nil
	}
	if mysqlSchemaNumericType(lowerType) && mysqlSchemaNumericLiteral(value) {
		return value, nil
	}
	if strings.HasPrefix(strings.ToLower(value), "b'") && strings.HasSuffix(value, "'") && schemaMySQLExpressionSafe(value) {
		return value, nil
	}
	return mysqlSchemaQuoteLiteral(column.defaultValue.String), nil
}

func mysqlSchemaTemporalDefault(dataType, value string) bool {
	if !strings.Contains(dataType, "timestamp") && !strings.Contains(dataType, "datetime") && !strings.HasPrefix(dataType, "date") && !strings.HasPrefix(dataType, "time") {
		return false
	}
	normalized := strings.ToUpper(strings.TrimSpace(value))
	return strings.HasPrefix(normalized, "CURRENT_TIMESTAMP") || normalized == "CURRENT_DATE" || normalized == "CURRENT_TIME" || normalized == "LOCALTIME" || normalized == "LOCALTIMESTAMP"
}

func mysqlSchemaNumericType(dataType string) bool {
	for _, prefix := range []string{"tinyint", "smallint", "mediumint", "int", "integer", "bigint", "decimal", "numeric", "float", "double", "real", "bit", "bool", "boolean"} {
		if strings.HasPrefix(dataType, prefix) {
			return true
		}
	}
	return false
}

func mysqlSchemaIntegerType(dataType string) bool {
	typeName := strings.ToLower(strings.TrimSpace(dataType))
	for _, prefix := range []string{"tinyint", "smallint", "mediumint", "int", "integer", "bigint"} {
		if typeName == prefix || strings.HasPrefix(typeName, prefix+"(") || strings.HasPrefix(typeName, prefix+" ") {
			return true
		}
	}
	return false
}

func mysqlSchemaCharacterType(dataType string) bool {
	typeName := strings.ToLower(strings.TrimSpace(dataType))
	for _, prefix := range []string{"char", "character", "varchar", "national char", "national varchar", "nchar", "nvarchar", "tinytext", "text", "mediumtext", "longtext", "enum", "set"} {
		if typeName == prefix || strings.HasPrefix(typeName, prefix+"(") || strings.HasPrefix(typeName, prefix+" ") {
			return true
		}
	}
	return false
}

func mysqlSchemaNumericLiteral(value string) bool {
	if value == "" {
		return false
	}
	digits := 0
	for index, character := range value {
		if character >= '0' && character <= '9' {
			digits++
			continue
		}
		if (character == '+' || character == '-') && (index == 0 || value[index-1] == 'e' || value[index-1] == 'E') || character == '.' || character == 'e' || character == 'E' {
			continue
		}
		return false
	}
	return digits > 0
}

func mysqlSchemaOnUpdate(extra string) string {
	lower := strings.ToLower(extra)
	index := strings.Index(lower, "on update ")
	if index < 0 {
		return ""
	}
	value := strings.TrimSpace(extra[index+len("on update "):])
	lowerValue := strings.ToLower(value)
	for _, suffix := range []string{" invisible", " stored generated", " virtual generated", " auto_increment"} {
		if end := strings.Index(lowerValue, suffix); end >= 0 {
			value = strings.TrimSpace(value[:end])
			lowerValue = lowerValue[:end]
		}
	}
	return value
}

func mysqlSchemaConstraint(connection entity.Connection, constraint dto.SchemaConstraintDefinition, fallbackDatabase string) (string, error) {
	if !schemaRuntimeIdentifier(constraint.Name) {
		return "", apperror.NewValidation("invalid MySQL constraint name", nil)
	}
	columns, err := mysqlSchemaColumns(constraint.Columns)
	if err != nil && constraint.Type != dto.SchemaConstraintCheck {
		return "", err
	}
	switch constraint.Type {
	case dto.SchemaConstraintPrimaryKey:
		return "PRIMARY KEY (" + columns + ")", nil
	case dto.SchemaConstraintUnique:
		return "UNIQUE (" + columns + ")", nil
	case dto.SchemaConstraintForeignKey:
		if constraint.ReferencedTarget == nil {
			return "", apperror.NewValidation("MySQL foreign key target is required", nil)
		}
		referencedTarget := *constraint.ReferencedTarget
		if referencedTarget.Schema == "" {
			referencedTarget.Schema = fallbackDatabase
		}
		qualified, _, err := mysqlSchemaQualifiedTarget(connection, referencedTarget)
		if err != nil {
			return "", err
		}
		referencedColumns, err := mysqlSchemaColumns(constraint.ReferencedColumns)
		if err != nil || len(constraint.Columns) != len(constraint.ReferencedColumns) {
			return "", apperror.NewValidation("invalid MySQL foreign key columns", err)
		}
		definition := "FOREIGN KEY (" + columns + ") REFERENCES " + qualified + " (" + referencedColumns + ")"
		actions, err := mysqlReferentialActions(constraint)
		if err != nil {
			return "", err
		}
		return definition + actions, nil
	case dto.SchemaConstraintCheck:
		if !schemaMySQLExpressionSafe(constraint.Expression) {
			return "", apperror.NewValidation("invalid MySQL check expression", nil)
		}
		return "CHECK (" + strings.TrimSpace(constraint.Expression) + ")", nil
	default:
		return "", apperror.NewUnsupported("constraint type is unsupported by MySQL and MariaDB", nil)
	}
}

func mysqlSchemaColumns(columns []string) (string, error) {
	if len(columns) < 1 || len(columns) > 32 {
		return "", apperror.NewValidation("invalid MySQL constraint columns", nil)
	}
	dialect := mysqlDialect{}
	quoted := make([]string, len(columns))
	for index, column := range columns {
		if !schemaRuntimeIdentifier(column) {
			return "", apperror.NewValidation("invalid MySQL constraint column", nil)
		}
		quoted[index] = dialect.quoteIdentifier(column)
	}
	return strings.Join(quoted, ", "), nil
}

func mysqlReferentialActions(constraint dto.SchemaConstraintDefinition) (string, error) {
	result := ""
	if constraint.OnUpdate != "" {
		action, ok := schemaReferentialAction(constraint.OnUpdate)
		if !ok {
			return "", apperror.NewValidation("invalid MySQL ON UPDATE action", nil)
		}
		if action == "SET DEFAULT" {
			return "", apperror.NewUnsupported("MySQL and MariaDB do not support ON UPDATE SET DEFAULT", nil)
		}
		result += " ON UPDATE " + action
	}
	if constraint.OnDelete != "" {
		action, ok := schemaReferentialAction(constraint.OnDelete)
		if !ok {
			return "", apperror.NewValidation("invalid MySQL ON DELETE action", nil)
		}
		if action == "SET DEFAULT" {
			return "", apperror.NewUnsupported("MySQL and MariaDB do not support ON DELETE SET DEFAULT", nil)
		}
		result += " ON DELETE " + action
	}
	return result, nil
}

func mysqlSchemaDropConstraintOperation(engine entity.Engine, constraintType, name string) (string, error) {
	dialect := mysqlDialect{}
	switch strings.ToUpper(strings.TrimSpace(constraintType)) {
	case "PRIMARY KEY":
		return "DROP PRIMARY KEY", nil
	case "FOREIGN KEY":
		return "DROP FOREIGN KEY " + dialect.quoteIdentifier(name), nil
	case "UNIQUE":
		return "DROP INDEX " + dialect.quoteIdentifier(name), nil
	case "CHECK":
		if engine == entity.EngineMariaDB {
			return "DROP CONSTRAINT " + dialect.quoteIdentifier(name), nil
		}
		return "DROP CHECK " + dialect.quoteIdentifier(name), nil
	default:
		return "", apperror.NewUnsupported("constraint drop is unsupported by MySQL and MariaDB", nil)
	}
}

func mysqlSchemaCreateIndex(qualifiedTable string, index dto.SchemaIndexDefinition) (string, error) {
	if !schemaRuntimeIdentifier(index.Name) {
		return "", apperror.NewValidation("invalid MySQL index name", nil)
	}
	if index.Predicate != "" {
		return "", apperror.NewUnsupported("partial indexes are unsupported by MySQL and MariaDB", nil)
	}
	columns, err := mysqlSchemaColumns(index.Columns)
	if err != nil {
		return "", err
	}
	dialect := mysqlDialect{}
	method := strings.ToLower(strings.TrimSpace(index.Method))
	prefix := ""
	using := ""
	switch method {
	case "":
	case "btree", "hash":
		using = " USING " + strings.ToUpper(method)
	case "fulltext":
		if index.Unique {
			return "", apperror.NewUnsupported("unique full-text indexes are unsupported by MySQL and MariaDB", nil)
		}
		prefix = "FULLTEXT "
	case "spatial":
		if index.Unique {
			return "", apperror.NewUnsupported("unique spatial indexes are unsupported by MySQL and MariaDB", nil)
		}
		prefix = "SPATIAL "
	default:
		return "", apperror.NewUnsupported("index method is unsupported by MySQL and MariaDB", nil)
	}
	if index.Unique {
		prefix = "UNIQUE "
	}
	return "CREATE " + prefix + "INDEX " + dialect.quoteIdentifier(index.Name) + " ON " + qualifiedTable + " (" + columns + ")" + using, nil
}

func mysqlSchemaQuoteLiteral(value string) string {
	var result strings.Builder
	result.Grow(len(value) + 2)
	result.WriteByte('\'')
	for _, character := range value {
		switch character {
		case 0:
			result.WriteString("\\0")
		case '\b':
			result.WriteString("\\b")
		case '\n':
			result.WriteString("\\n")
		case '\r':
			result.WriteString("\\r")
		case '\t':
			result.WriteString("\\t")
		case 26:
			result.WriteString("\\Z")
		case '\\':
			result.WriteString("\\\\")
		case '\'':
			result.WriteString("''")
		default:
			result.WriteRune(character)
		}
	}
	result.WriteByte('\'')
	return result.String()
}
