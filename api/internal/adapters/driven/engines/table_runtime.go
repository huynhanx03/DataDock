package engines

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

const tableResultMaxBytes int64 = 16 * 1024 * 1024

func (manager *Manager) ReadRows(ctx context.Context, connection entity.Connection, password string, input dto.TableRowsInput) (dto.TableRowsResult, error) {
	reference, err := decodeCatalogReference(input.Reference)
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	if err := ensureBrowsableReference(reference); err != nil {
		return dto.TableRowsResult{}, err
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	var schema dto.TableSchema
	if connection.Engine == entity.EnginePostgreSQL {
		schema, err = readPostgresTableSchema(ctx, database, connection, reference)
	} else if connection.Engine == entity.EngineMySQL || connection.Engine == entity.EngineMariaDB {
		schema, err = readMySQLTableSchema(ctx, database, connection, reference)
	} else {
		return dto.TableRowsResult{}, apperror.NewUnsupported("table browsing is not supported", nil)
	}
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	return readTypedTableRows(ctx, database, connection, reference, schema, input)
}

func readTypedTableRows(ctx context.Context, database *sql.DB, connection entity.Connection, reference catalogReference, schema dto.TableSchema, input dto.TableRowsInput) (dto.TableRowsResult, error) {
	startedAt := time.Now()
	dialect, err := dialectFor(connection.Engine)
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	qualified, err := dialect.qualified(reference)
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	allColumns := tableDataColumns(schema.Columns)
	if len(allColumns) == 0 {
		return dto.TableRowsResult{}, apperror.NewNotFound("table was not found", nil)
	}
	available := make(map[string]dto.DataColumn, len(allColumns))
	for _, column := range allColumns {
		available[column.Name] = column
	}
	selected, err := selectedTableColumns(input.Columns, allColumns, available)
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	if input.Sort != "" && len(input.Sorts) == 0 {
		direction := strings.ToLower(input.Order)
		if direction == "" {
			direction = "asc"
		}
		input.Sorts = []dto.TableSort{{Column: input.Sort, Direction: direction}}
	}
	where, arguments, err := buildTableWhere(dialect, available, input.Search, input.Filters)
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	order, err := buildTableOrder(dialect, available, allColumns, input.Sorts)
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	projection := make([]string, len(selected))
	for index, column := range selected {
		projection[index] = dialect.quoteIdentifier(column.Name)
	}
	limitPosition := len(arguments) + 1
	offsetPosition := limitPosition + 1
	statement := "SELECT " + strings.Join(projection, ", ") + " FROM " + qualified + where + order + " LIMIT " + dialect.placeholder(limitPosition) + " OFFSET " + dialect.placeholder(offsetPosition)
	arguments = append(arguments, input.Limit+1, input.Offset)
	rows, err := database.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return dto.TableRowsResult{}, unwrapCatalogError(err)
	}
	columns, values, byteTruncated, _, err := scanTypedRows(rows, selected, 0, tableResultMaxBytes)
	rows.Close()
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	hasMore := len(values) > input.Limit || byteTruncated && len(values) > 0
	if len(values) > input.Limit {
		values = values[:input.Limit]
	}
	var total *int64
	if input.IncludeTotal {
		countArguments := arguments[:len(arguments)-2]
		var count int64
		if err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+qualified+where, countArguments...).Scan(&count); err != nil {
			return dto.TableRowsResult{}, unwrapCatalogError(err)
		}
		total = &count
	}
	primaryKeys := make([]string, 0)
	for _, column := range allColumns {
		if column.PrimaryKey {
			primaryKeys = append(primaryKeys, column.Name)
		}
	}
	nextOffset := 0
	if hasMore {
		nextOffset = input.Offset + len(values)
	}
	return dto.TableRowsResult{
		Columns:           columns,
		Rows:              values,
		PrimaryKeyColumns: primaryKeys,
		Total:             total,
		Limit:             input.Limit,
		Offset:            input.Offset,
		HasMore:           hasMore,
		NextOffset:        nextOffset,
		DurationMS:        time.Since(startedAt).Milliseconds(),
		Truncated:         byteTruncated,
	}, nil
}

func tableDataColumns(columns []dto.TableColumn) []dto.DataColumn {
	result := make([]dto.DataColumn, len(columns))
	usedKeys := make(map[string]int, len(columns))
	for index, column := range columns {
		key := column.Name
		usedKeys[key]++
		if usedKeys[key] > 1 {
			key = fmt.Sprintf("%s__%d", key, usedKeys[key])
		}
		databaseType := column.DatabaseType
		if databaseType == "" {
			databaseType = column.DataType
		}
		logicalType := logicalTypeForDatabaseType(databaseType)
		result[index] = dto.DataColumn{
			Key:           key,
			Name:          column.Name,
			Type:          string(logicalType),
			DatabaseType:  databaseType,
			LogicalType:   logicalType,
			Nullable:      column.Nullable,
			DefaultValue:  column.DefaultValue,
			Precision:     column.Precision,
			Scale:         column.Scale,
			Length:        column.Length,
			EnumValues:    column.EnumValues,
			Identity:      column.Identity,
			Generated:     column.Generated,
			PrimaryKey:    column.PrimaryKey,
			ValueEncoding: valueEncodingForLogicalType(logicalType),
		}
	}
	return result
}

func selectedTableColumns(names []string, columns []dto.DataColumn, available map[string]dto.DataColumn) ([]dto.DataColumn, error) {
	if len(names) == 0 {
		return columns, nil
	}
	selected := make([]dto.DataColumn, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		column, ok := available[name]
		if !ok || seen[name] {
			return nil, apperror.NewValidation("selected column was not found", nil)
		}
		seen[name] = true
		selected = append(selected, column)
	}
	return selected, nil
}

func buildTableWhere(dialect engineDialect, columns map[string]dto.DataColumn, search string, filters []dto.TableFilter) (string, []any, error) {
	clauses := make([]string, 0, len(filters)+1)
	arguments := make([]any, 0, len(filters)+len(columns))
	search = strings.TrimSpace(search)
	if search != "" {
		conditions := make([]string, 0, len(columns))
		names := make([]string, 0, len(columns))
		for name := range columns {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			column := columns[name]
			arguments = append(arguments, "%"+search+"%")
			castType := "TEXT"
			operator := "ILIKE"
			if _, ok := dialect.(mysqlDialect); ok {
				castType = "CHAR"
				operator = "LIKE"
			}
			conditions = append(conditions, "CAST("+dialect.quoteIdentifier(column.Name)+" AS "+castType+") "+operator+" "+dialect.placeholder(len(arguments)))
		}
		clauses = append(clauses, "("+strings.Join(conditions, " OR ")+")")
	}
	for _, filter := range filters {
		column, ok := columns[filter.Column]
		if !ok {
			return "", nil, apperror.NewValidation("filter column was not found", nil)
		}
		quoted := dialect.quoteIdentifier(column.Name)
		switch filter.Operator {
		case "is_null":
			clauses = append(clauses, quoted+" IS NULL")
		case "is_not_null":
			clauses = append(clauses, quoted+" IS NOT NULL")
		case "in":
			items, ok := sliceValues(filter.Value)
			if !ok || len(items) == 0 || len(items) > 100 {
				return "", nil, apperror.NewValidation("invalid IN filter", nil)
			}
			placeholders := make([]string, len(items))
			for index, item := range items {
				arguments = append(arguments, item)
				placeholders[index] = dialect.placeholder(len(arguments))
			}
			clauses = append(clauses, quoted+" IN ("+strings.Join(placeholders, ", ")+")")
		case "contains", "starts_with", "ends_with":
			value := fmt.Sprint(filter.Value)
			if filter.Operator == "contains" || filter.Operator == "ends_with" {
				value = "%" + value
			}
			if filter.Operator == "contains" || filter.Operator == "starts_with" {
				value += "%"
			}
			arguments = append(arguments, value)
			clauses = append(clauses, quoted+" LIKE "+dialect.placeholder(len(arguments)))
		default:
			operators := map[string]string{"eq": "=", "ne": "<>", "lt": "<", "lte": "<=", "gt": ">", "gte": ">="}
			operator, ok := operators[filter.Operator]
			if !ok {
				return "", nil, apperror.NewValidation("filter operator is not supported", nil)
			}
			arguments = append(arguments, filter.Value)
			clauses = append(clauses, quoted+" "+operator+" "+dialect.placeholder(len(arguments)))
		}
	}
	if len(clauses) == 0 {
		return "", arguments, nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), arguments, nil
}

func buildTableOrder(dialect engineDialect, available map[string]dto.DataColumn, columns []dto.DataColumn, sorts []dto.TableSort) (string, error) {
	if len(sorts) == 0 {
		for _, column := range columns {
			if column.PrimaryKey {
				sorts = append(sorts, dto.TableSort{Column: column.Name, Direction: "asc"})
			}
		}
		if len(sorts) == 0 {
			sorts = []dto.TableSort{{Column: columns[0].Name, Direction: "asc"}}
		}
	}
	parts := make([]string, len(sorts))
	for index, sort := range sorts {
		column, ok := available[sort.Column]
		direction := strings.ToLower(sort.Direction)
		if !ok || direction != "asc" && direction != "desc" {
			return "", apperror.NewValidation("invalid table sort", nil)
		}
		parts[index] = dialect.quoteIdentifier(column.Name) + " " + strings.ToUpper(direction)
	}
	return " ORDER BY " + strings.Join(parts, ", "), nil
}

func sliceValues(value any) ([]any, bool) {
	if values, ok := value.([]any); ok {
		return values, true
	}
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() || reflected.Kind() != reflect.Slice && reflected.Kind() != reflect.Array {
		return nil, false
	}
	values := make([]any, reflected.Len())
	for index := range values {
		values[index] = reflected.Index(index).Interface()
	}
	return values, true
}
