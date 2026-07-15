package engines

import (
	"strings"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/lib/pq"
)

func generatePostgresSchemaSteps(connection entity.Connection, actions []dto.SchemaAction) ([]dto.SchemaStep, error) {
	steps := make([]dto.SchemaStep, 0, len(actions))
	aliases := make(map[string]dto.SchemaTarget)
	for _, original := range actions {
		action := original
		resolved, err := schemaResolveRuntimeTarget(aliases, action.Target, "public")
		if err != nil {
			return nil, err
		}
		action.Target = resolved
		if action.Constraint != nil && action.Constraint.ReferencedTarget != nil {
			constraint := *action.Constraint
			referencedTarget, err := schemaResolveRuntimeTarget(aliases, *constraint.ReferencedTarget, "public")
			if err != nil {
				return nil, err
			}
			constraint.ReferencedTarget = &referencedTarget
			action.Constraint = &constraint
		}
		generated, err := generatePostgresSchemaAction(connection, action)
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

func generatePostgresSchemaAction(_ entity.Connection, action dto.SchemaAction) ([]dto.SchemaStep, error) {
	dialect := postgresDialect{}
	target, schemaName, err := postgresSchemaQualifiedTarget(action.Target, "public")
	if err != nil {
		return nil, err
	}
	one := func(statement string) []dto.SchemaStep {
		return []dto.SchemaStep{schemaStep(action, statement)}
	}
	switch action.Kind {
	case dto.SchemaActionCreateTable:
		if len(action.Columns) < 1 || len(action.Columns) > 200 {
			return nil, apperror.NewValidation("invalid PostgreSQL table definition", nil)
		}
		definitions := make([]string, 0, len(action.Columns))
		for _, column := range action.Columns {
			definition, err := postgresSchemaColumnDefinition(column)
			if err != nil {
				return nil, err
			}
			definitions = append(definitions, definition)
		}
		steps := one("CREATE TABLE " + target + " (" + strings.Join(definitions, ", ") + ")")
		for _, column := range action.Columns {
			if column.Comment == "" {
				continue
			}
			steps = append(steps, schemaStep(action, "COMMENT ON COLUMN "+target+"."+dialect.quoteIdentifier(column.Name)+" IS "+pq.QuoteLiteral(column.Comment)))
		}
		return steps, nil
	case dto.SchemaActionRenameTable:
		if !schemaRuntimeIdentifier(action.NewName) {
			return nil, apperror.NewValidation("invalid PostgreSQL table rename", nil)
		}
		return one("ALTER TABLE " + target + " RENAME TO " + dialect.quoteIdentifier(action.NewName)), nil
	case dto.SchemaActionDropTable:
		statement := "DROP TABLE " + target
		if action.Cascade {
			statement += " CASCADE"
		}
		return one(statement), nil
	case dto.SchemaActionAddColumn:
		if action.Column == nil {
			return nil, apperror.NewValidation("PostgreSQL column definition is required", nil)
		}
		definition, err := postgresSchemaColumnDefinition(*action.Column)
		if err != nil {
			return nil, err
		}
		steps := one("ALTER TABLE " + target + " ADD COLUMN " + definition)
		if action.Column.Comment != "" {
			steps = append(steps, schemaStep(action, "COMMENT ON COLUMN "+target+"."+dialect.quoteIdentifier(action.Column.Name)+" IS "+pq.QuoteLiteral(action.Column.Comment)))
		}
		return steps, nil
	case dto.SchemaActionRenameColumn:
		if !schemaRuntimeIdentifier(action.Name) || !schemaRuntimeIdentifier(action.NewName) {
			return nil, apperror.NewValidation("invalid PostgreSQL column rename", nil)
		}
		return one("ALTER TABLE " + target + " RENAME COLUMN " + dialect.quoteIdentifier(action.Name) + " TO " + dialect.quoteIdentifier(action.NewName)), nil
	case dto.SchemaActionAlterColumnType:
		if !schemaRuntimeIdentifier(action.Name) || !schemaDataTypeSafe(action.DataType) {
			return nil, apperror.NewValidation("invalid PostgreSQL column type", nil)
		}
		return one("ALTER TABLE " + target + " ALTER COLUMN " + dialect.quoteIdentifier(action.Name) + " TYPE " + strings.TrimSpace(action.DataType)), nil
	case dto.SchemaActionSetColumnNullable:
		if !schemaRuntimeIdentifier(action.Name) || action.Nullable == nil {
			return nil, apperror.NewValidation("invalid PostgreSQL nullable change", nil)
		}
		operation := " SET NOT NULL"
		if *action.Nullable {
			operation = " DROP NOT NULL"
		}
		return one("ALTER TABLE " + target + " ALTER COLUMN " + dialect.quoteIdentifier(action.Name) + operation), nil
	case dto.SchemaActionSetColumnDefault:
		if !schemaRuntimeIdentifier(action.Name) {
			return nil, apperror.NewValidation("invalid PostgreSQL default change", nil)
		}
		statement := "ALTER TABLE " + target + " ALTER COLUMN " + dialect.quoteIdentifier(action.Name)
		if action.DefaultValue == nil {
			return one(statement + " DROP DEFAULT"), nil
		}
		if !schemaExpressionSafe(*action.DefaultValue) {
			return nil, apperror.NewValidation("invalid PostgreSQL default expression", nil)
		}
		return one(statement + " SET DEFAULT " + strings.TrimSpace(*action.DefaultValue)), nil
	case dto.SchemaActionSetColumnComment:
		if !schemaRuntimeIdentifier(action.Name) || action.Comment == nil || len(*action.Comment) > 2000 || strings.ContainsRune(*action.Comment, 0) {
			return nil, apperror.NewValidation("invalid PostgreSQL column comment", nil)
		}
		literal := "NULL"
		if *action.Comment != "" {
			literal = pq.QuoteLiteral(*action.Comment)
		}
		return one("COMMENT ON COLUMN " + target + "." + dialect.quoteIdentifier(action.Name) + " IS " + literal), nil
	case dto.SchemaActionDropColumn:
		if !schemaRuntimeIdentifier(action.Name) {
			return nil, apperror.NewValidation("invalid PostgreSQL column drop", nil)
		}
		statement := "ALTER TABLE " + target + " DROP COLUMN " + dialect.quoteIdentifier(action.Name)
		if action.Cascade {
			statement += " CASCADE"
		}
		return one(statement), nil
	case dto.SchemaActionAddConstraint:
		if action.Constraint == nil {
			return nil, apperror.NewValidation("PostgreSQL constraint definition is required", nil)
		}
		definition, err := postgresSchemaConstraint(*action.Constraint, schemaName)
		if err != nil {
			return nil, err
		}
		return one("ALTER TABLE " + target + " ADD CONSTRAINT " + dialect.quoteIdentifier(action.Constraint.Name) + " " + definition), nil
	case dto.SchemaActionDropConstraint:
		if !schemaRuntimeIdentifier(action.Name) {
			return nil, apperror.NewValidation("invalid PostgreSQL constraint drop", nil)
		}
		statement := "ALTER TABLE " + target + " DROP CONSTRAINT " + dialect.quoteIdentifier(action.Name)
		if action.Cascade {
			statement += " CASCADE"
		}
		return one(statement), nil
	case dto.SchemaActionCreateIndex:
		if action.Index == nil {
			return nil, apperror.NewValidation("PostgreSQL index definition is required", nil)
		}
		statement, err := postgresSchemaCreateIndex(action.Target, *action.Index)
		if err != nil {
			return nil, err
		}
		return one(statement), nil
	case dto.SchemaActionDropIndex:
		if !schemaRuntimeIdentifier(action.Name) {
			return nil, apperror.NewValidation("invalid PostgreSQL index drop", nil)
		}
		statement := "DROP INDEX " + dialect.quoteIdentifier(schemaName) + "." + dialect.quoteIdentifier(action.Name)
		if action.Cascade {
			statement += " CASCADE"
		}
		return one(statement), nil
	case dto.SchemaActionRebuildIndex:
		if !schemaRuntimeIdentifier(action.Name) {
			return nil, apperror.NewValidation("invalid PostgreSQL index rebuild", nil)
		}
		return one("REINDEX INDEX " + dialect.quoteIdentifier(schemaName) + "." + dialect.quoteIdentifier(action.Name)), nil
	case dto.SchemaActionAnalyzeIndex:
		if !schemaRuntimeIdentifier(action.Name) {
			return nil, apperror.NewValidation("invalid PostgreSQL index analysis", nil)
		}
		return one("ANALYZE " + target), nil
	default:
		return nil, apperror.NewUnsupported("schema action is unsupported by PostgreSQL", nil)
	}
}

func postgresSchemaQualifiedTarget(target dto.SchemaTarget, fallbackSchema string) (string, string, error) {
	schemaName := target.Schema
	if schemaName == "" {
		schemaName = fallbackSchema
	}
	if !schemaRuntimeIdentifier(schemaName) || !schemaRuntimeIdentifier(target.Table) {
		return "", "", apperror.NewValidation("invalid PostgreSQL schema target", nil)
	}
	dialect := postgresDialect{}
	return dialect.quoteIdentifier(schemaName) + "." + dialect.quoteIdentifier(target.Table), schemaName, nil
}

func postgresSchemaColumnDefinition(column dto.SchemaColumnDefinition) (string, error) {
	if !schemaRuntimeIdentifier(column.Name) || !schemaDataTypeSafe(column.DataType) || len(column.Comment) > 2000 || strings.ContainsRune(column.Comment, 0) || column.Identity && (column.Nullable || column.GeneratedExpression != nil) || (column.Identity || column.GeneratedExpression != nil) && column.DefaultValue != nil {
		return "", apperror.NewValidation("invalid PostgreSQL column definition", nil)
	}
	dialect := postgresDialect{}
	definition := dialect.quoteIdentifier(column.Name) + " " + strings.TrimSpace(column.DataType)
	if column.GeneratedExpression != nil {
		if !schemaExpressionSafe(*column.GeneratedExpression) {
			return "", apperror.NewValidation("invalid PostgreSQL generated expression", nil)
		}
		definition += " GENERATED ALWAYS AS (" + strings.TrimSpace(*column.GeneratedExpression) + ") STORED"
	} else if column.Identity {
		definition += " GENERATED ALWAYS AS IDENTITY"
	} else if column.DefaultValue != nil {
		if !schemaExpressionSafe(*column.DefaultValue) {
			return "", apperror.NewValidation("invalid PostgreSQL default expression", nil)
		}
		definition += " DEFAULT " + strings.TrimSpace(*column.DefaultValue)
	}
	if !column.Nullable {
		definition += " NOT NULL"
	}
	return definition, nil
}

func postgresSchemaConstraint(constraint dto.SchemaConstraintDefinition, fallbackSchema string) (string, error) {
	if !schemaRuntimeIdentifier(constraint.Name) {
		return "", apperror.NewValidation("invalid PostgreSQL constraint name", nil)
	}
	columns, err := postgresSchemaColumns(constraint.Columns)
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
			return "", apperror.NewValidation("PostgreSQL foreign key target is required", nil)
		}
		referencedTarget, _, err := postgresSchemaQualifiedTarget(*constraint.ReferencedTarget, fallbackSchema)
		if err != nil {
			return "", err
		}
		referencedColumns, err := postgresSchemaColumns(constraint.ReferencedColumns)
		if err != nil || len(constraint.Columns) != len(constraint.ReferencedColumns) {
			return "", apperror.NewValidation("invalid PostgreSQL foreign key columns", err)
		}
		definition := "FOREIGN KEY (" + columns + ") REFERENCES " + referencedTarget + " (" + referencedColumns + ")"
		actions, err := postgresReferentialActions(constraint)
		if err != nil {
			return "", err
		}
		return definition + actions, nil
	case dto.SchemaConstraintCheck:
		if !schemaExpressionSafe(constraint.Expression) {
			return "", apperror.NewValidation("invalid PostgreSQL check expression", nil)
		}
		return "CHECK (" + strings.TrimSpace(constraint.Expression) + ")", nil
	default:
		return "", apperror.NewUnsupported("constraint type is unsupported by PostgreSQL", nil)
	}
}

func postgresSchemaColumns(columns []string) (string, error) {
	if len(columns) < 1 || len(columns) > 32 {
		return "", apperror.NewValidation("invalid PostgreSQL constraint columns", nil)
	}
	dialect := postgresDialect{}
	quoted := make([]string, len(columns))
	for index, column := range columns {
		if !schemaRuntimeIdentifier(column) {
			return "", apperror.NewValidation("invalid PostgreSQL constraint column", nil)
		}
		quoted[index] = dialect.quoteIdentifier(column)
	}
	return strings.Join(quoted, ", "), nil
}

func postgresReferentialActions(constraint dto.SchemaConstraintDefinition) (string, error) {
	result := ""
	if constraint.OnUpdate != "" {
		action, ok := schemaReferentialAction(constraint.OnUpdate)
		if !ok {
			return "", apperror.NewValidation("invalid PostgreSQL ON UPDATE action", nil)
		}
		result += " ON UPDATE " + action
	}
	if constraint.OnDelete != "" {
		action, ok := schemaReferentialAction(constraint.OnDelete)
		if !ok {
			return "", apperror.NewValidation("invalid PostgreSQL ON DELETE action", nil)
		}
		result += " ON DELETE " + action
	}
	return result, nil
}

func postgresSchemaCreateIndex(target dto.SchemaTarget, index dto.SchemaIndexDefinition) (string, error) {
	qualifiedTable, _, err := postgresSchemaQualifiedTarget(target, "public")
	if err != nil {
		return "", err
	}
	if !schemaRuntimeIdentifier(index.Name) {
		return "", apperror.NewValidation("invalid PostgreSQL index name", nil)
	}
	columns, err := postgresSchemaColumns(index.Columns)
	if err != nil {
		return "", err
	}
	dialect := postgresDialect{}
	unique := ""
	if index.Unique {
		unique = "UNIQUE "
	}
	statement := "CREATE " + unique + "INDEX " + dialect.quoteIdentifier(index.Name) + " ON " + qualifiedTable
	if index.Method != "" {
		if !schemaRuntimeIdentifier(index.Method) {
			return "", apperror.NewValidation("invalid PostgreSQL index method", nil)
		}
		statement += " USING " + dialect.quoteIdentifier(strings.ToLower(index.Method))
	}
	statement += " (" + columns + ")"
	if index.Predicate != "" {
		if !schemaExpressionSafe(index.Predicate) {
			return "", apperror.NewValidation("invalid PostgreSQL index predicate", nil)
		}
		statement += " WHERE " + strings.TrimSpace(index.Predicate)
	}
	return statement, nil
}
