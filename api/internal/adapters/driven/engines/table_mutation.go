package engines

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

var _ ports.TableMutationRuntime = (*Manager)(nil)

type mutationColumnDefinition struct {
	logicalType dto.LogicalType
	writable    bool
	generated   bool
}

type mutationTableDefinition struct {
	qualified   string
	dialect     engineDialect
	columns     map[string]mutationColumnDefinition
	columnOrder []string
	primaryKeys []string
}

type mutationConflictError struct {
	conflict dto.TableMutationConflict
}

func (err *mutationConflictError) Error() string {
	return "table mutation conflict"
}

func (manager *Manager) MutationMetadata(ctx context.Context, connection entity.Connection, password, encodedReference, transactionID string) (dto.TableMutationMetadata, error) {
	if connection.ReadOnly {
		return dto.TableMutationMetadata{}, apperror.NewReadonly("table mutation rejected by read-only connection", nil)
	}
	reference, err := mutationTableReference(encodedReference)
	if err != nil {
		return dto.TableMutationMetadata{}, err
	}
	if transactionID != "" {
		var metadata dto.TableMutationMetadata
		err := manager.withTransactionExecutor(ctx, connection.ID, transactionID, func(executor queryExecutor) error {
			definition, loadErr := loadMutationTableDefinition(ctx, executor, connection, reference)
			if loadErr != nil {
				return loadErr
			}
			metadata = definition.publicMetadata()
			return nil
		})
		return metadata, err
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return dto.TableMutationMetadata{}, err
	}
	definition, err := loadMutationTableDefinition(ctx, database, connection, reference)
	if err != nil {
		return dto.TableMutationMetadata{}, err
	}
	return definition.publicMetadata(), nil
}

func (manager *Manager) ApplyMutations(ctx context.Context, connection entity.Connection, password string, request ports.TableMutationRuntimeRequest) (dto.TableMutationBatchResult, error) {
	if connection.ReadOnly {
		return dto.TableMutationBatchResult{}, apperror.NewReadonly("table mutation rejected by read-only connection", nil)
	}
	if len(request.Mutations) < 1 || len(request.Mutations) > 100 {
		return dto.TableMutationBatchResult{}, apperror.NewValidation("invalid table mutation request", nil)
	}
	reference, err := mutationTableReference(request.Reference)
	if err != nil {
		return dto.TableMutationBatchResult{}, err
	}
	if request.TransactionID != "" {
		return manager.applyMutationsInTransaction(ctx, connection, request, reference)
	}
	database, err := manager.Database(ctx, connection, password)
	if err != nil {
		return dto.TableMutationBatchResult{}, err
	}
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return dto.TableMutationBatchResult{}, err
	}
	result, applyErr := executeMutationBatch(ctx, transaction, connection, reference, request.Mutations)
	if applyErr != nil {
		rollbackErr := transaction.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return dto.TableMutationBatchResult{}, apperror.NewInternal("table mutation rollback failed", errors.Join(applyErr, rollbackErr))
		}
		var conflict *mutationConflictError
		if errors.As(applyErr, &conflict) {
			return mutationConflictResult(conflict.conflict), nil
		}
		return dto.TableMutationBatchResult{}, applyErr
	}
	if err := transaction.Commit(); err != nil {
		return dto.TableMutationBatchResult{}, err
	}
	return result, nil
}

func (manager *Manager) applyMutationsInTransaction(ctx context.Context, connection entity.Connection, request ports.TableMutationRuntimeRequest, reference catalogReference) (dto.TableMutationBatchResult, error) {
	var result dto.TableMutationBatchResult
	var conflict *mutationConflictError
	err := manager.withTransactionExecutor(ctx, connection.ID, request.TransactionID, func(executor queryExecutor) error {
		name, err := mutationSavepointName()
		if err != nil {
			return err
		}
		dialect, err := dialectFor(connection.Engine)
		if err != nil {
			return err
		}
		quoted := dialect.quoteIdentifier(name)
		if _, err := executor.ExecContext(ctx, "SAVEPOINT "+quoted); err != nil {
			return err
		}
		result, err = executeMutationBatch(ctx, executor, connection, reference, request.Mutations)
		if err == nil {
			if _, releaseErr := executor.ExecContext(ctx, "RELEASE SAVEPOINT "+quoted); releaseErr == nil {
				return nil
			} else if rollbackErr := rollbackMutationSavepoint(ctx, executor, quoted); rollbackErr != nil {
				return errors.Join(sql.ErrTxDone, apperror.NewInternal("table mutation savepoint cleanup failed", errors.Join(releaseErr, rollbackErr)))
			} else {
				return releaseErr
			}
		}
		rollbackErr := rollbackMutationSavepoint(ctx, executor, quoted)
		if rollbackErr != nil {
			return errors.Join(sql.ErrTxDone, apperror.NewInternal("table mutation savepoint rollback failed", errors.Join(err, rollbackErr)))
		}
		if errors.As(err, &conflict) {
			return nil
		}
		return err
	})
	if err != nil {
		return dto.TableMutationBatchResult{}, err
	}
	if conflict != nil {
		return mutationConflictResult(conflict.conflict), nil
	}
	return result, nil
}

func rollbackMutationSavepoint(ctx context.Context, executor queryExecutor, quoted string) error {
	cleanupContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, rollbackErr := executor.ExecContext(cleanupContext, "ROLLBACK TO SAVEPOINT "+quoted)
	if rollbackErr != nil {
		return rollbackErr
	}
	_, releaseErr := executor.ExecContext(cleanupContext, "RELEASE SAVEPOINT "+quoted)
	return releaseErr
}

func mutationSavepointName() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", apperror.NewInternal("table mutation savepoint could not be allocated", err)
	}
	return "datadock_mutation_" + hex.EncodeToString(value), nil
}

func executeMutationBatch(ctx context.Context, executor queryExecutor, connection entity.Connection, reference catalogReference, mutations []dto.TableMutation) (dto.TableMutationBatchResult, error) {
	definition, err := loadMutationTableDefinition(ctx, executor, connection, reference)
	if err != nil {
		return dto.TableMutationBatchResult{}, err
	}
	if err := validateRuntimeMutations(mutations, definition); err != nil {
		return dto.TableMutationBatchResult{}, err
	}
	for index, mutation := range mutations {
		if err := executeTableMutation(ctx, executor, definition, mutation); err != nil {
			var conflict *mutationConflictError
			if errors.As(err, &conflict) {
				conflict.conflict.Index = index
				conflict.conflict.Kind = mutation.Kind
				return dto.TableMutationBatchResult{}, conflict
			}
			if isDuplicateMutationError(err) {
				return dto.TableMutationBatchResult{}, &mutationConflictError{conflict: dto.TableMutationConflict{Index: index, Kind: mutation.Kind, Reason: dto.TableMutationConflictOptimistic}}
			}
			return dto.TableMutationBatchResult{}, err
		}
	}
	return dto.TableMutationBatchResult{
		Status:    dto.TableMutationBatchApplied,
		Atomic:    true,
		Applied:   len(mutations),
		Conflicts: []dto.TableMutationConflict{},
	}, nil
}

func executeTableMutation(ctx context.Context, executor queryExecutor, definition mutationTableDefinition, mutation dto.TableMutation) error {
	if mutation.Kind != dto.TableMutationInsert {
		reason, err := lockMutationTarget(ctx, executor, definition, mutation)
		if err != nil {
			return err
		}
		if reason != "" {
			return &mutationConflictError{conflict: dto.TableMutationConflict{Reason: reason}}
		}
	}
	statement, arguments, err := mutationStatement(definition, mutation)
	if err != nil {
		return err
	}
	execution, err := executor.ExecContext(ctx, statement, arguments...)
	if err != nil {
		return err
	}
	affected, err := execution.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		reason := dto.TableMutationConflictMissingRow
		if mutation.Kind == dto.TableMutationInsert || len(mutation.ExpectedValues) > 0 {
			reason = dto.TableMutationConflictOptimistic
		}
		return &mutationConflictError{conflict: dto.TableMutationConflict{Reason: reason}}
	}
	if affected != 1 {
		return apperror.NewInternal("table mutation affected an unexpected number of rows", nil)
	}
	return nil
}

func lockMutationTarget(ctx context.Context, executor queryExecutor, definition mutationTableDefinition, mutation dto.TableMutation) (dto.TableMutationConflictReason, error) {
	keyPredicates, keyArguments, err := mutationKeyPredicates(definition, mutation.Keys)
	if err != nil {
		return "", err
	}
	matchExpression := "TRUE"
	_, mysqlParameters := definition.dialect.(mysqlDialect)
	if mysqlParameters {
		matchExpression = "1"
	}
	arguments := keyArguments
	if len(mutation.ExpectedValues) > 0 {
		expectedColumns := sortedMapKeys(mutation.ExpectedValues)
		expressions := make([]string, len(expectedColumns))
		expectedArguments := make([]any, 0, len(expectedColumns))
		for index, name := range expectedColumns {
			column := definition.columns[name]
			argument, convertErr := mutationArgument(mutation.ExpectedValues[name], column)
			if convertErr != nil {
				return "", convertErr
			}
			expectedArguments = append(expectedArguments, argument)
			operator := " IS NOT DISTINCT FROM "
			position := len(keyArguments) + len(expectedArguments)
			if mysqlParameters {
				operator = " <=> "
				position = len(expectedArguments)
			}
			expressions[index] = definition.dialect.quoteIdentifier(name) + operator + definition.dialect.placeholder(position)
		}
		if mysqlParameters {
			arguments = append(expectedArguments, keyArguments...)
		} else {
			arguments = append(arguments, expectedArguments...)
		}
		matchExpression = strings.Join(expressions, " AND ")
	}
	statement := "SELECT " + matchExpression + " FROM " + definition.qualified + " WHERE " + strings.Join(keyPredicates, " AND ") + " LIMIT 1 FOR UPDATE"
	var matches bool
	err = executor.QueryRowContext(ctx, statement, arguments...).Scan(&matches)
	if errors.Is(err, sql.ErrNoRows) {
		return dto.TableMutationConflictMissingRow, nil
	}
	if err != nil {
		return "", err
	}
	if !matches {
		return dto.TableMutationConflictOptimistic, nil
	}
	return "", nil
}

func mutationStatement(definition mutationTableDefinition, mutation dto.TableMutation) (string, []any, error) {
	switch mutation.Kind {
	case dto.TableMutationInsert:
		columns := sortedMapKeys(mutation.Values)
		quoted := make([]string, len(columns))
		placeholders := make([]string, len(columns))
		arguments := make([]any, 0, len(columns))
		for index, name := range columns {
			argument, err := mutationArgument(mutation.Values[name], definition.columns[name])
			if err != nil {
				return "", nil, err
			}
			arguments = append(arguments, argument)
			quoted[index] = definition.dialect.quoteIdentifier(name)
			placeholders[index] = definition.dialect.placeholder(len(arguments))
		}
		return "INSERT INTO " + definition.qualified + " (" + strings.Join(quoted, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")", arguments, nil
	case dto.TableMutationUpdate:
		columns := sortedMapKeys(mutation.Values)
		assignments := make([]string, len(columns))
		arguments := make([]any, 0, len(columns)+len(definition.primaryKeys))
		for index, name := range columns {
			argument, err := mutationArgument(mutation.Values[name], definition.columns[name])
			if err != nil {
				return "", nil, err
			}
			arguments = append(arguments, argument)
			assignments[index] = definition.dialect.quoteIdentifier(name) + " = " + definition.dialect.placeholder(len(arguments))
		}
		keys, keyArguments, err := mutationKeyPredicatesAt(definition, mutation.Keys, len(arguments))
		if err != nil {
			return "", nil, err
		}
		arguments = append(arguments, keyArguments...)
		return "UPDATE " + definition.qualified + " SET " + strings.Join(assignments, ", ") + " WHERE " + strings.Join(keys, " AND "), arguments, nil
	case dto.TableMutationDelete:
		keys, arguments, err := mutationKeyPredicates(definition, mutation.Keys)
		if err != nil {
			return "", nil, err
		}
		return "DELETE FROM " + definition.qualified + " WHERE " + strings.Join(keys, " AND "), arguments, nil
	default:
		return "", nil, apperror.NewValidation("invalid table mutation kind", nil)
	}
}

func mutationKeyPredicates(definition mutationTableDefinition, values map[string]any) ([]string, []any, error) {
	return mutationKeyPredicatesAt(definition, values, 0)
}

func mutationKeyPredicatesAt(definition mutationTableDefinition, values map[string]any, offset int) ([]string, []any, error) {
	predicates := make([]string, len(definition.primaryKeys))
	arguments := make([]any, 0, len(definition.primaryKeys))
	for index, name := range definition.primaryKeys {
		argument, err := mutationArgument(values[name], definition.columns[name])
		if err != nil {
			return nil, nil, err
		}
		arguments = append(arguments, argument)
		predicates[index] = definition.dialect.quoteIdentifier(name) + " = " + definition.dialect.placeholder(offset+len(arguments))
	}
	return predicates, arguments, nil
}

func validateRuntimeMutations(mutations []dto.TableMutation, definition mutationTableDefinition) error {
	primaryKeys := make(map[string]struct{}, len(definition.primaryKeys))
	for _, name := range definition.primaryKeys {
		primaryKeys[name] = struct{}{}
	}
	for _, mutation := range mutations {
		switch mutation.Kind {
		case dto.TableMutationInsert:
			if len(mutation.Values) == 0 || len(mutation.Keys) != 0 || len(mutation.ExpectedValues) != 0 {
				return apperror.NewValidation("invalid insert mutation", nil)
			}
		case dto.TableMutationUpdate:
			if len(mutation.Values) == 0 || !mutationKeysMatch(mutation.Keys, primaryKeys) {
				return apperror.NewValidation("invalid update mutation", nil)
			}
		case dto.TableMutationDelete:
			if len(mutation.Values) != 0 || !mutationKeysMatch(mutation.Keys, primaryKeys) {
				return apperror.NewValidation("invalid delete mutation", nil)
			}
		default:
			return apperror.NewValidation("invalid table mutation kind", nil)
		}
		for name := range mutation.Values {
			column, exists := definition.columns[name]
			if !exists || !column.writable || column.generated {
				return apperror.NewValidation("mutation contains a non-writable column", nil)
			}
		}
		for name := range mutation.ExpectedValues {
			if _, exists := definition.columns[name]; !exists {
				return apperror.NewValidation("mutation contains an unknown expected column", nil)
			}
		}
		for _, value := range mutation.Keys {
			if value == nil {
				return apperror.NewValidation("primary key values cannot be null", nil)
			}
		}
	}
	return nil
}

func mutationKeysMatch(values map[string]any, primaryKeys map[string]struct{}) bool {
	if len(primaryKeys) == 0 || len(values) != len(primaryKeys) {
		return false
	}
	for name := range values {
		if _, exists := primaryKeys[name]; !exists {
			return false
		}
	}
	return true
}

func mutationArgument(value any, column mutationColumnDefinition) (any, error) {
	if value == nil {
		return nil, nil
	}
	if column.logicalType == dto.LogicalTypeBinary {
		switch typed := value.(type) {
		case []byte:
			return typed, nil
		case string:
			decoded, err := base64.StdEncoding.DecodeString(typed)
			if err != nil {
				return nil, apperror.NewValidation("invalid base64 mutation value", err)
			}
			return decoded, nil
		default:
			return nil, apperror.NewValidation("invalid binary mutation value", nil)
		}
	}
	switch typed := value.(type) {
	case json.RawMessage:
		if !json.Valid(typed) {
			return nil, apperror.NewValidation("invalid JSON mutation value", nil)
		}
		return string(typed), nil
	case map[string]any, []any:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return nil, apperror.NewValidation("invalid JSON mutation value", err)
		}
		return string(encoded), nil
	case json.Number:
		return typed.String(), nil
	default:
		return value, nil
	}
}

func mutationTableReference(encoded string) (catalogReference, error) {
	reference, err := decodeCatalogReference(encoded)
	if err != nil {
		return catalogReference{}, err
	}
	if reference.Kind != dto.CatalogObjectTable {
		return catalogReference{}, apperror.NewValidation("catalog object is not a writable table", nil)
	}
	return reference, nil
}

func loadMutationTableDefinition(ctx context.Context, executor queryExecutor, connection entity.Connection, reference catalogReference) (mutationTableDefinition, error) {
	dialect, err := dialectFor(connection.Engine)
	if err != nil {
		return mutationTableDefinition{}, err
	}
	if connection.Engine == entity.EngineMySQL || connection.Engine == entity.EngineMariaDB {
		if reference.Database == "" && reference.Schema == "" {
			reference.Database = connection.Database
		}
	}
	qualified, err := dialect.qualified(reference)
	if err != nil {
		return mutationTableDefinition{}, err
	}
	definition := mutationTableDefinition{qualified: qualified, dialect: dialect}
	if connection.Engine == entity.EnginePostgreSQL {
		definition.columns, definition.columnOrder, definition.primaryKeys, err = loadPostgresMutationMetadata(ctx, executor, reference)
	} else if connection.Engine == entity.EngineMySQL || connection.Engine == entity.EngineMariaDB {
		definition.columns, definition.columnOrder, definition.primaryKeys, err = loadMySQLMutationMetadata(ctx, executor, connection, reference)
	} else {
		return mutationTableDefinition{}, apperror.NewUnsupported("table mutation is not supported", nil)
	}
	if err != nil {
		return mutationTableDefinition{}, err
	}
	if len(definition.columns) == 0 {
		return mutationTableDefinition{}, apperror.NewNotFound("table was not found", nil)
	}
	return definition, nil
}

func loadPostgresMutationMetadata(ctx context.Context, executor queryExecutor, reference catalogReference) (map[string]mutationColumnDefinition, []string, []string, error) {
	if reference.Schema == "" || reference.Name == "" {
		return nil, nil, nil, apperror.NewValidation("invalid PostgreSQL table reference", nil)
	}
	var currentDatabase string
	if err := executor.QueryRowContext(ctx, "SELECT current_database()").Scan(&currentDatabase); err != nil {
		return nil, nil, nil, err
	}
	if reference.Database != "" && reference.Database != currentDatabase {
		return nil, nil, nil, apperror.NewNotFound("table was not found", nil)
	}
	var relationOID int64
	err := executor.QueryRowContext(ctx, `
SELECT c.oid::bigint
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname = $1
  AND c.relname = $2
  AND c.relkind IN ('r', 'p')`, reference.Schema, reference.Name).Scan(&relationOID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, apperror.NewNotFound("table was not found", nil)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	rows, err := executor.QueryContext(ctx, `
SELECT a.attname,
       pg_catalog.format_type(a.atttypid, a.atttypmod),
       a.attidentity::text,
       a.attgenerated::text
FROM pg_catalog.pg_attribute a
WHERE a.attrelid = $1::oid
  AND a.attnum > 0
  AND NOT a.attisdropped
ORDER BY a.attnum`, relationOID)
	if err != nil {
		return nil, nil, nil, err
	}
	columns := make(map[string]mutationColumnDefinition)
	order := make([]string, 0)
	for rows.Next() {
		var name, databaseType, identityKind, generatedKind string
		if err := rows.Scan(&name, &databaseType, &identityKind, &generatedKind); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		generated := generatedKind != ""
		columns[name] = mutationColumnDefinition{
			logicalType: mutationLogicalType(databaseType),
			writable:    !generated && identityKind != "a",
			generated:   generated,
		}
		order = append(order, name)
	}
	if err := rows.Close(); err != nil {
		return nil, nil, nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	primaryRows, err := executor.QueryContext(ctx, `
SELECT a.attname
FROM pg_catalog.pg_index i
JOIN LATERAL unnest(i.indkey::smallint[]) WITH ORDINALITY AS key(attnum, position) ON true
JOIN pg_catalog.pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = key.attnum
WHERE i.indrelid = $1::oid
  AND i.indisprimary
ORDER BY key.position`, relationOID)
	if err != nil {
		return nil, nil, nil, err
	}
	primaryKeys := make([]string, 0)
	for primaryRows.Next() {
		var name string
		if err := primaryRows.Scan(&name); err != nil {
			primaryRows.Close()
			return nil, nil, nil, err
		}
		primaryKeys = append(primaryKeys, name)
	}
	if err := primaryRows.Close(); err != nil {
		return nil, nil, nil, err
	}
	if err := primaryRows.Err(); err != nil {
		return nil, nil, nil, err
	}
	return columns, order, primaryKeys, nil
}

func loadMySQLMutationMetadata(ctx context.Context, executor queryExecutor, connection entity.Connection, reference catalogReference) (map[string]mutationColumnDefinition, []string, []string, error) {
	databaseName := mysqlReferenceDatabase(connection, reference)
	if databaseName == "" || reference.Name == "" {
		return nil, nil, nil, apperror.NewValidation("invalid MySQL table reference", nil)
	}
	var tableType string
	err := executor.QueryRowContext(ctx, "SELECT table_type FROM information_schema.tables WHERE table_schema = ? AND table_name = ?", databaseName, reference.Name).Scan(&tableType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, apperror.NewNotFound("table was not found", nil)
	}
	if err != nil {
		return nil, nil, nil, err
	}
	if !strings.EqualFold(tableType, "BASE TABLE") {
		return nil, nil, nil, apperror.NewValidation("catalog object is not a writable table", nil)
	}
	rows, err := executor.QueryContext(ctx, "SELECT column_name, column_type, COALESCE(extra, '') FROM information_schema.columns WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position", databaseName, reference.Name)
	if err != nil {
		return nil, nil, nil, err
	}
	columns := make(map[string]mutationColumnDefinition)
	order := make([]string, 0)
	for rows.Next() {
		var name, databaseType, extra string
		if err := rows.Scan(&name, &databaseType, &extra); err != nil {
			rows.Close()
			return nil, nil, nil, err
		}
		lowerExtra := strings.ToLower(extra)
		generated := strings.Contains(lowerExtra, "virtual generated") || strings.Contains(lowerExtra, "stored generated") || strings.Contains(lowerExtra, "generated virtual") || strings.Contains(lowerExtra, "generated stored")
		columns[name] = mutationColumnDefinition{
			logicalType: mutationLogicalType(databaseType),
			writable:    !generated,
			generated:   generated,
		}
		order = append(order, name)
	}
	if err := rows.Close(); err != nil {
		return nil, nil, nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, nil, nil, err
	}
	primaryRows, err := executor.QueryContext(ctx, "SELECT column_name FROM information_schema.statistics WHERE table_schema = ? AND table_name = ? AND index_name = 'PRIMARY' ORDER BY seq_in_index", databaseName, reference.Name)
	if err != nil {
		return nil, nil, nil, err
	}
	primaryKeys := make([]string, 0)
	for primaryRows.Next() {
		var name string
		if err := primaryRows.Scan(&name); err != nil {
			primaryRows.Close()
			return nil, nil, nil, err
		}
		primaryKeys = append(primaryKeys, name)
	}
	if err := primaryRows.Close(); err != nil {
		return nil, nil, nil, err
	}
	if err := primaryRows.Err(); err != nil {
		return nil, nil, nil, err
	}
	return columns, order, primaryKeys, nil
}

func (definition mutationTableDefinition) publicMetadata() dto.TableMutationMetadata {
	columns := make([]dto.TableMutationColumn, 0, len(definition.columnOrder))
	for _, name := range definition.columnOrder {
		column := definition.columns[name]
		columns = append(columns, dto.TableMutationColumn{Name: name, Writable: column.writable, Generated: column.generated})
	}
	return dto.TableMutationMetadata{Columns: columns, PrimaryKeyColumns: append([]string(nil), definition.primaryKeys...)}
}

func mutationConflictResult(conflict dto.TableMutationConflict) dto.TableMutationBatchResult {
	return dto.TableMutationBatchResult{
		Status:    dto.TableMutationBatchConflict,
		Atomic:    true,
		Applied:   0,
		Conflicts: []dto.TableMutationConflict{conflict},
	}
}

func sortedMapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for name := range values {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

func isDuplicateMutationError(err error) bool {
	var postgresError *pq.Error
	if errors.As(err, &postgresError) {
		return string(postgresError.Code) == "23505"
	}
	var mysqlError *mysql.MySQLError
	if errors.As(err, &mysqlError) {
		return mysqlError.Number == 1022 || mysqlError.Number == 1062 || mysqlError.Number == 1586
	}
	return false
}

func mutationLogicalType(databaseType string) dto.LogicalType {
	normalized := normalizeDatabaseType(databaseType)
	if normalized == "bit" || strings.HasPrefix(normalized, "bit(") || strings.HasPrefix(normalized, "bit varying") {
		return dto.LogicalTypeBinary
	}
	return logicalTypeForDatabaseType(databaseType)
}
