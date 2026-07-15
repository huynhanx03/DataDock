package engines

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type mysqlCatalogGroup struct {
	label string
	kind  dto.CatalogObjectKind
}

type mysqlCatalogCounts map[string]map[dto.CatalogObjectKind]int64

type mysqlCatalogTarget struct {
	reference   catalogReference
	groupKind   dto.CatalogObjectKind
	objectName  string
	loadObjects bool
	loadColumns bool
	offset      int
	paginated   bool
	hasMore     bool
}

type mysqlRoutineParameter struct {
	mode     string
	dataType string
}

type mysqlRoutineMetadata struct {
	name         string
	specificName string
	returnType   string
}

type mysqlIndexPart struct {
	column     string
	expression string
	prefix     int64
	descending bool
}

type mysqlIndexMetadata struct {
	name      string
	nonUnique bool
	typeName  string
	parts     []mysqlIndexPart
}

type mysqlConstraintMetadata struct {
	name             string
	typeName         string
	columns          []string
	referencedSchema string
	referencedTable  string
	referencedCols   []string
	updateRule       string
	deleteRule       string
	checkClause      string
}

var mysqlCatalogGroups = []mysqlCatalogGroup{
	{label: "Tables", kind: dto.CatalogObjectTable},
	{label: "Views", kind: dto.CatalogObjectView},
	{label: "Functions", kind: dto.CatalogObjectFunction},
	{label: "Procedures", kind: dto.CatalogObjectProcedure},
	{label: "Triggers", kind: dto.CatalogObjectTrigger},
}

func readMySQLCatalog(ctx context.Context, database *sql.DB, connection entity.Connection, input dto.CatalogInput) (dto.CatalogTree, error) {
	limit := input.Limit
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	target, err := mysqlResolveCatalogTarget(input)
	if err != nil {
		return dto.CatalogTree{}, err
	}
	rootPage := target == nil
	groupPage := target != nil && target.reference.Kind == dto.CatalogObjectGroup
	if input.Cursor != "" && !rootPage && !groupPage {
		return dto.CatalogTree{}, apperror.NewValidation("catalog cursor is not supported for this parent", nil)
	}
	offset, err := decodeCatalogCursor(input.Cursor, connection.ID, input.ParentReference)
	if err != nil {
		return dto.CatalogTree{}, err
	}
	if offset > int(^uint(0)>>1)-limit {
		return dto.CatalogTree{}, apperror.NewValidation("invalid catalog cursor", nil)
	}
	if groupPage {
		target.offset = offset
		target.paginated = true
	}
	databaseName := ""
	if target != nil {
		databaseName = target.reference.Database
		if databaseName == "" {
			databaseName = target.reference.Schema
		}
	}
	databaseLimit := limit
	databaseOffset := 0
	if rootPage {
		databaseLimit++
		databaseOffset = offset
	}
	databases, err := mysqlCatalogDatabases(ctx, database, databaseName, databaseLimit, databaseOffset)
	if err != nil {
		return dto.CatalogTree{}, unwrapCatalogError(err)
	}
	if databaseName != "" && len(databases) == 0 {
		return dto.CatalogTree{}, apperror.NewNotFound("catalog object was not found", nil)
	}
	rootHasMore := rootPage && len(databases) > limit
	if rootHasMore {
		databases = databases[:limit]
	}
	counts, err := mysqlLoadCatalogCounts(ctx, database, databases)
	if err != nil {
		return dto.CatalogTree{}, unwrapCatalogError(err)
	}
	result := dto.CatalogTree{
		ConnectionID: connection.ID,
		Engine:       connection.Engine,
		Capabilities: catalogCapabilities(connection.Engine),
		Databases:    make([]dto.CatalogObject, 0, len(databases)),
		LoadedAt:     time.Now().UTC(),
	}
	for _, name := range databases {
		databaseNode, err := mysqlBuildDatabaseNode(ctx, database, connection, name, counts[name], target, limit, input.Depth)
		if err != nil {
			return dto.CatalogTree{}, unwrapCatalogError(err)
		}
		result.Databases = append(result.Databases, databaseNode)
	}
	if rootHasMore || groupPage && target.hasMore {
		result.NextCursor, err = encodeCatalogCursor(connection.ID, input.ParentReference, offset+limit)
		if err != nil {
			return dto.CatalogTree{}, err
		}
	}
	return result, nil
}

func readMySQLTableSchema(ctx context.Context, database *sql.DB, connection entity.Connection, reference catalogReference) (dto.TableSchema, error) {
	if reference.Kind != dto.CatalogObjectTable && reference.Kind != dto.CatalogObjectView {
		return dto.TableSchema{}, apperror.NewValidation("catalog object is not a MySQL table or view", nil)
	}
	databaseName := mysqlReferenceDatabase(connection, reference)
	if databaseName == "" || reference.Name == "" {
		return dto.TableSchema{}, apperror.NewValidation("invalid MySQL table reference", nil)
	}
	columns, err := mysqlInspectColumns(ctx, database, databaseName, reference.Name)
	if err != nil {
		return dto.TableSchema{}, unwrapCatalogError(err)
	}
	if len(columns) == 0 {
		return dto.TableSchema{}, apperror.NewNotFound("table or view was not found", nil)
	}
	indexes, err := mysqlInspectIndexes(ctx, database, databaseName, reference.Name)
	if err != nil {
		return dto.TableSchema{}, unwrapCatalogError(err)
	}
	constraints, err := mysqlInspectConstraints(ctx, database, databaseName, reference.Name)
	if err != nil {
		return dto.TableSchema{}, unwrapCatalogError(err)
	}
	return dto.TableSchema{Columns: columns, Indexes: indexes, Constraints: constraints}, nil
}

func readMySQLTableDDL(ctx context.Context, database *sql.DB, connection entity.Connection, reference catalogReference) (string, error) {
	if reference.Kind != dto.CatalogObjectTable && reference.Kind != dto.CatalogObjectView {
		return "", apperror.NewValidation("catalog object is not a MySQL table or view", nil)
	}
	databaseName := mysqlReferenceDatabase(connection, reference)
	if databaseName == "" || reference.Name == "" {
		return "", apperror.NewValidation("invalid MySQL table reference", nil)
	}
	dialect, err := dialectFor(connection.Engine)
	if err != nil {
		return "", err
	}
	qualified, err := dialect.qualified(catalogReference{Kind: reference.Kind, Database: databaseName, Schema: databaseName, Name: reference.Name})
	if err != nil {
		return "", err
	}
	rows, err := database.QueryContext(ctx, "SHOW CREATE TABLE "+qualified)
	if err != nil {
		return "", unwrapCatalogError(err)
	}
	defer rows.Close()
	columnNames, err := rows.Columns()
	if err != nil {
		return "", unwrapCatalogError(err)
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", unwrapCatalogError(err)
		}
		return "", apperror.NewNotFound("table or view was not found", nil)
	}
	values := make([]sql.RawBytes, len(columnNames))
	destinations := make([]any, len(values))
	for index := range values {
		destinations[index] = &values[index]
	}
	if err := rows.Scan(destinations...); err != nil {
		return "", unwrapCatalogError(err)
	}
	ddlIndex := -1
	for index, name := range columnNames {
		normalized := strings.ToLower(strings.TrimSpace(name))
		if strings.HasPrefix(normalized, "create ") {
			ddlIndex = index
			break
		}
	}
	if ddlIndex < 0 && len(values) > 1 {
		ddlIndex = 1
	}
	if ddlIndex < 0 || strings.TrimSpace(string(values[ddlIndex])) == "" {
		return "", apperror.NewInternal("database returned an empty table definition", nil)
	}
	return string(values[ddlIndex]), nil
}

func mysqlResolveCatalogTarget(input dto.CatalogInput) (*mysqlCatalogTarget, error) {
	if input.ParentReference == "" {
		return nil, nil
	}
	reference, err := decodeCatalogReference(input.ParentReference)
	if err != nil {
		return nil, err
	}
	target := &mysqlCatalogTarget{reference: reference}
	switch reference.Kind {
	case dto.CatalogObjectDatabase, dto.CatalogObjectSchema:
	case dto.CatalogObjectGroup:
		kind, ok := mysqlGroupKind(reference.Name)
		if !ok {
			return nil, apperror.NewUnsupported("catalog group is not supported by MySQL", nil)
		}
		target.groupKind = kind
		target.loadObjects = true
	case dto.CatalogObjectTable, dto.CatalogObjectView, dto.CatalogObjectFunction, dto.CatalogObjectProcedure, dto.CatalogObjectTrigger:
		target.groupKind = reference.Kind
		target.objectName = reference.Name
		target.loadObjects = true
		target.loadColumns = reference.Kind == dto.CatalogObjectTable || reference.Kind == dto.CatalogObjectView
	case dto.CatalogObjectColumn:
		target.groupKind = dto.CatalogObjectTable
		target.objectName = reference.Owner
		target.loadObjects = true
		target.loadColumns = true
	default:
		return nil, apperror.NewUnsupported("catalog object is not supported by MySQL", nil)
	}
	if reference.Database == "" && reference.Schema == "" {
		return nil, apperror.NewValidation("MySQL catalog reference has no database", nil)
	}
	return target, nil
}

func mysqlGroupKind(value string) (dto.CatalogObjectKind, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "table", "tables":
		return dto.CatalogObjectTable, true
	case "view", "views":
		return dto.CatalogObjectView, true
	case "function", "functions":
		return dto.CatalogObjectFunction, true
	case "procedure", "procedures":
		return dto.CatalogObjectProcedure, true
	case "trigger", "triggers":
		return dto.CatalogObjectTrigger, true
	default:
		return "", false
	}
}

func mysqlCatalogDatabases(ctx context.Context, database *sql.DB, databaseName string, limit, offset int) ([]string, error) {
	query := "SELECT schema_name FROM information_schema.schemata WHERE LOWER(schema_name) NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys', 'ndbinfo')"
	arguments := make([]any, 0, 2)
	if databaseName != "" {
		query += " AND schema_name = ?"
		arguments = append(arguments, databaseName)
	}
	query += " ORDER BY BINARY schema_name LIMIT ? OFFSET ?"
	if databaseName != "" {
		limit = 1
		offset = 0
	}
	arguments = append(arguments, limit, offset)
	rows, err := database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result = append(result, name)
	}
	return result, rows.Err()
}

func mysqlLoadCatalogCounts(ctx context.Context, database *sql.DB, databases []string) (mysqlCatalogCounts, error) {
	result := make(mysqlCatalogCounts, len(databases))
	for _, name := range databases {
		result[name] = map[dto.CatalogObjectKind]int64{}
	}
	if len(databases) == 0 {
		return result, nil
	}
	clause, arguments := mysqlStringInClause(databases)
	rows, err := database.QueryContext(ctx, "SELECT table_schema, table_type, COUNT(*) FROM information_schema.tables WHERE table_schema IN ("+clause+") AND table_type IN ('BASE TABLE', 'VIEW') GROUP BY table_schema, table_type", arguments...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var databaseName, tableType string
		var count int64
		if err := rows.Scan(&databaseName, &tableType, &count); err != nil {
			rows.Close()
			return nil, err
		}
		kind := dto.CatalogObjectTable
		if strings.EqualFold(tableType, "VIEW") {
			kind = dto.CatalogObjectView
		}
		result[databaseName][kind] += count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rows, err = database.QueryContext(ctx, "SELECT routine_schema, routine_type, COUNT(*) FROM information_schema.routines WHERE routine_schema IN ("+clause+") GROUP BY routine_schema, routine_type", arguments...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var databaseName, routineType string
		var count int64
		if err := rows.Scan(&databaseName, &routineType, &count); err != nil {
			rows.Close()
			return nil, err
		}
		kind := dto.CatalogObjectFunction
		if strings.EqualFold(routineType, "PROCEDURE") {
			kind = dto.CatalogObjectProcedure
		}
		result[databaseName][kind] += count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	rows, err = database.QueryContext(ctx, "SELECT trigger_schema, COUNT(*) FROM information_schema.triggers WHERE trigger_schema IN ("+clause+") GROUP BY trigger_schema", arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var databaseName string
		var count int64
		if err := rows.Scan(&databaseName, &count); err != nil {
			return nil, err
		}
		result[databaseName][dto.CatalogObjectTrigger] = count
	}
	return result, rows.Err()
}

func mysqlBuildDatabaseNode(ctx context.Context, database *sql.DB, connection entity.Connection, databaseName string, counts map[dto.CatalogObjectKind]int64, target *mysqlCatalogTarget, limit int, depth dto.CatalogDepth) (dto.CatalogObject, error) {
	databaseReference := catalogReference{Kind: dto.CatalogObjectDatabase, Database: databaseName}
	databaseNode, err := mysqlNewCatalogObject(connection.ID, "", databaseName, databaseName, databaseReference)
	if err != nil {
		return dto.CatalogObject{}, err
	}
	databaseNode.Database = databaseName
	databaseNode.Capabilities = catalogCapabilities(connection.Engine)
	schemaReference := catalogReference{Kind: dto.CatalogObjectSchema, Database: databaseName, Schema: databaseName}
	schemaNode, err := mysqlNewCatalogObject(connection.ID, databaseNode.ID, databaseName, databaseName, schemaReference)
	if err != nil {
		return dto.CatalogObject{}, err
	}
	schemaNode.Database = databaseName
	schemaNode.Schema = databaseName
	schemaNode.Capabilities = catalogCapabilities(connection.Engine)
	groups := mysqlCatalogGroups
	if target != nil && target.groupKind != "" {
		groups = make([]mysqlCatalogGroup, 0, 1)
		for _, definition := range mysqlCatalogGroups {
			if definition.kind == target.groupKind {
				groups = append(groups, definition)
				break
			}
		}
	}
	schemaNode.Children = make([]dto.CatalogObject, 0, len(groups))
	for _, definition := range groups {
		group, err := mysqlBuildGroupNode(ctx, database, connection, databaseName, schemaNode.ID, definition, counts[definition.kind], target, limit, depth)
		if err != nil {
			return dto.CatalogObject{}, err
		}
		schemaNode.Children = append(schemaNode.Children, group)
	}
	schemaNode.ChildrenState = catalogChildrenState(schemaNode.Children)
	databaseNode.Children = []dto.CatalogObject{schemaNode}
	databaseNode.ChildrenState = dto.CatalogChildrenLoaded
	return databaseNode, nil
}

func mysqlBuildGroupNode(ctx context.Context, database *sql.DB, connection entity.Connection, databaseName, schemaID string, definition mysqlCatalogGroup, count int64, target *mysqlCatalogTarget, limit int, depth dto.CatalogDepth) (dto.CatalogObject, error) {
	reference := catalogReference{Kind: dto.CatalogObjectGroup, Database: databaseName, Schema: databaseName, Name: string(definition.kind)}
	group, err := mysqlNewCatalogObject(connection.ID, schemaID, definition.label, databaseName+"."+strings.ToLower(definition.label), reference)
	if err != nil {
		return dto.CatalogObject{}, err
	}
	group.Database = databaseName
	group.Schema = databaseName
	group.Count = mysqlInt64Pointer(count)
	group.Capabilities = []string{string(definition.kind)}
	loadObjects := depth == dto.CatalogDepthAll
	loadColumns := depth == dto.CatalogDepthAll
	objectName := ""
	if target != nil && target.groupKind == definition.kind {
		loadObjects = target.loadObjects || loadObjects
		loadColumns = target.loadColumns || loadColumns
		objectName = target.objectName
	}
	if !loadObjects {
		if count == 0 {
			group.ChildrenState = dto.CatalogChildrenEmpty
		} else {
			group.ChildrenState = dto.CatalogChildrenUnloaded
		}
		return group, nil
	}
	queryLimit := limit
	offset := 0
	paginated := target != nil && target.paginated && target.groupKind == definition.kind
	if paginated {
		queryLimit++
		offset = target.offset
	}
	children, err := mysqlLoadGroupObjects(ctx, database, connection.ID, databaseName, group.ID, definition.kind, objectName, queryLimit, offset, loadColumns)
	if err != nil {
		return dto.CatalogObject{}, err
	}
	if paginated && len(children) > limit {
		target.hasMore = true
		children = children[:limit]
	}
	if objectName != "" && len(children) == 0 {
		return dto.CatalogObject{}, apperror.NewNotFound("catalog object was not found", nil)
	}
	group.Children = children
	group.ChildrenState = catalogChildrenState(children)
	return group, nil
}

func mysqlLoadGroupObjects(ctx context.Context, database *sql.DB, connectionID, databaseName, parentID string, kind dto.CatalogObjectKind, objectName string, limit, offset int, loadColumns bool) ([]dto.CatalogObject, error) {
	switch kind {
	case dto.CatalogObjectTable, dto.CatalogObjectView:
		return mysqlLoadRelations(ctx, database, connectionID, databaseName, parentID, kind, objectName, limit, offset, loadColumns)
	case dto.CatalogObjectFunction, dto.CatalogObjectProcedure:
		return mysqlLoadRoutines(ctx, database, connectionID, databaseName, parentID, kind, objectName, limit, offset)
	case dto.CatalogObjectTrigger:
		return mysqlLoadTriggers(ctx, database, connectionID, databaseName, parentID, objectName, limit, offset)
	default:
		return nil, apperror.NewUnsupported("catalog group is not supported by MySQL", nil)
	}
}

func mysqlLoadRelations(ctx context.Context, database *sql.DB, connectionID, databaseName, parentID string, kind dto.CatalogObjectKind, objectName string, limit, offset int, loadColumns bool) ([]dto.CatalogObject, error) {
	tableType := "BASE TABLE"
	if kind == dto.CatalogObjectView {
		tableType = "VIEW"
	}
	query := "SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = ?"
	arguments := []any{databaseName, tableType}
	if objectName != "" {
		query += " AND table_name = ?"
		arguments = append(arguments, objectName)
	}
	query += " ORDER BY BINARY table_name LIMIT ? OFFSET ?"
	arguments = append(arguments, limit, offset)
	rows, err := database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	columns := map[string][]dto.CatalogObject{}
	if loadColumns && len(names) > 0 {
		columns, err = mysqlLoadCatalogColumns(ctx, database, connectionID, databaseName, names)
		if err != nil {
			return nil, err
		}
	}
	result := make([]dto.CatalogObject, 0, len(names))
	for _, name := range names {
		reference := catalogReference{Kind: kind, Database: databaseName, Schema: databaseName, Name: name}
		object, err := mysqlNewCatalogObject(connectionID, parentID, name, databaseName+"."+name, reference)
		if err != nil {
			return nil, err
		}
		object.Database = databaseName
		object.Schema = databaseName
		object.Capabilities = []string{"browse", "schema", "ddl"}
		if loadColumns {
			object.Children = columns[name]
			for index := range object.Children {
				object.Children[index].ParentID = object.ID
			}
			object.ChildrenState = catalogChildrenState(object.Children)
		} else {
			object.ChildrenState = dto.CatalogChildrenUnloaded
		}
		result = append(result, object)
	}
	return result, nil
}

func mysqlLoadCatalogColumns(ctx context.Context, database *sql.DB, connectionID, databaseName string, relationNames []string) (map[string][]dto.CatalogObject, error) {
	clause, arguments := mysqlStringInClause(relationNames)
	queryArguments := append([]any{databaseName}, arguments...)
	rows, err := database.QueryContext(ctx, "SELECT table_name, column_name, column_type FROM information_schema.columns WHERE table_schema = ? AND table_name IN ("+clause+") ORDER BY table_name, ordinal_position", queryArguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string][]dto.CatalogObject, len(relationNames))
	for rows.Next() {
		var tableName, columnName, columnType string
		if err := rows.Scan(&tableName, &columnName, &columnType); err != nil {
			return nil, err
		}
		reference := catalogReference{Kind: dto.CatalogObjectColumn, Database: databaseName, Schema: databaseName, Name: columnName, Owner: tableName}
		object, err := mysqlNewCatalogObject(connectionID, "", columnName, databaseName+"."+tableName+"."+columnName, reference)
		if err != nil {
			return nil, err
		}
		object.Database = databaseName
		object.Schema = databaseName
		object.DataType = columnType
		object.ChildrenState = dto.CatalogChildrenEmpty
		result[tableName] = append(result[tableName], object)
	}
	return result, rows.Err()
}

func mysqlLoadRoutines(ctx context.Context, database *sql.DB, connectionID, databaseName, parentID string, kind dto.CatalogObjectKind, objectName string, limit, offset int) ([]dto.CatalogObject, error) {
	routineType := "FUNCTION"
	if kind == dto.CatalogObjectProcedure {
		routineType = "PROCEDURE"
	}
	query := "SELECT routine_name, specific_name, COALESCE(data_type, '') FROM information_schema.routines WHERE routine_schema = ? AND routine_type = ?"
	arguments := []any{databaseName, routineType}
	if objectName != "" {
		query += " AND routine_name = ?"
		arguments = append(arguments, objectName)
	}
	query += " ORDER BY BINARY routine_name, BINARY specific_name LIMIT ? OFFSET ?"
	arguments = append(arguments, limit, offset)
	rows, err := database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	routines := make([]mysqlRoutineMetadata, 0)
	for rows.Next() {
		var item mysqlRoutineMetadata
		if err := rows.Scan(&item.name, &item.specificName, &item.returnType); err != nil {
			rows.Close()
			return nil, err
		}
		routines = append(routines, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	parameters, err := mysqlLoadRoutineParameters(ctx, database, databaseName, routines)
	if err != nil {
		return nil, err
	}
	result := make([]dto.CatalogObject, 0, len(routines))
	for _, routine := range routines {
		signature := mysqlRoutineSignature(routine.name, parameters[routine.specificName])
		reference := catalogReference{Kind: kind, Database: databaseName, Schema: databaseName, Name: routine.name, Signature: signature}
		object, err := mysqlNewCatalogObject(connectionID, parentID, routine.name, databaseName+"."+signature, reference)
		if err != nil {
			return nil, err
		}
		object.Database = databaseName
		object.Schema = databaseName
		object.DataType = routine.returnType
		object.Capabilities = []string{"ddl"}
		object.ChildrenState = dto.CatalogChildrenEmpty
		result = append(result, object)
	}
	return result, nil
}

func mysqlLoadRoutineParameters(ctx context.Context, database *sql.DB, databaseName string, routines []mysqlRoutineMetadata) (map[string][]mysqlRoutineParameter, error) {
	result := make(map[string][]mysqlRoutineParameter, len(routines))
	if len(routines) == 0 {
		return result, nil
	}
	specificNames := make([]string, 0, len(routines))
	for _, routine := range routines {
		specificNames = append(specificNames, routine.specificName)
	}
	clause, arguments := mysqlStringInClause(specificNames)
	queryArguments := append([]any{databaseName}, arguments...)
	rows, err := database.QueryContext(ctx, "SELECT specific_name, COALESCE(parameter_mode, ''), COALESCE(dtd_identifier, data_type, '') FROM information_schema.parameters WHERE specific_schema = ? AND ordinal_position > 0 AND specific_name IN ("+clause+") ORDER BY specific_name, ordinal_position", queryArguments...)
	if err != nil {
		if mysqlOptionalMetadataError(err) {
			return result, nil
		}
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var specificName string
		var parameter mysqlRoutineParameter
		if err := rows.Scan(&specificName, &parameter.mode, &parameter.dataType); err != nil {
			return nil, err
		}
		result[specificName] = append(result[specificName], parameter)
	}
	return result, rows.Err()
}

func mysqlRoutineSignature(name string, parameters []mysqlRoutineParameter) string {
	parts := make([]string, 0, len(parameters))
	for _, parameter := range parameters {
		part := strings.TrimSpace(strings.ToUpper(parameter.mode) + " " + parameter.dataType)
		parts = append(parts, part)
	}
	return name + "(" + strings.Join(parts, ", ") + ")"
}

func mysqlLoadTriggers(ctx context.Context, database *sql.DB, connectionID, databaseName, parentID, objectName string, limit, offset int) ([]dto.CatalogObject, error) {
	query := "SELECT trigger_name, event_object_table, action_timing, event_manipulation FROM information_schema.triggers WHERE trigger_schema = ?"
	arguments := []any{databaseName}
	if objectName != "" {
		query += " AND trigger_name = ?"
		arguments = append(arguments, objectName)
	}
	query += " ORDER BY BINARY trigger_name, BINARY event_object_table LIMIT ? OFFSET ?"
	arguments = append(arguments, limit, offset)
	rows, err := database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dto.CatalogObject, 0)
	for rows.Next() {
		var name, tableName, timing, event string
		if err := rows.Scan(&name, &tableName, &timing, &event); err != nil {
			return nil, err
		}
		signature := strings.ToUpper(timing) + " " + strings.ToUpper(event) + " ON " + tableName
		reference := catalogReference{Kind: dto.CatalogObjectTrigger, Database: databaseName, Schema: databaseName, Name: name, Signature: signature, Owner: tableName}
		object, err := mysqlNewCatalogObject(connectionID, parentID, name, databaseName+"."+name, reference)
		if err != nil {
			return nil, err
		}
		object.Database = databaseName
		object.Schema = databaseName
		object.DataType = strings.ToUpper(timing) + " " + strings.ToUpper(event)
		object.Capabilities = []string{"ddl"}
		object.ChildrenState = dto.CatalogChildrenEmpty
		result = append(result, object)
	}
	return result, rows.Err()
}

func mysqlNewCatalogObject(connectionID, parentID, name, qualifiedName string, reference catalogReference) (dto.CatalogObject, error) {
	encoded, err := encodeCatalogReference(reference)
	if err != nil {
		return dto.CatalogObject{}, err
	}
	return dto.CatalogObject{
		ID:            catalogObjectID(connectionID, reference),
		Reference:     encoded,
		ConnectionID:  connectionID,
		ParentID:      parentID,
		Name:          name,
		QualifiedName: qualifiedName,
		Kind:          reference.Kind,
		ChildrenState: dto.CatalogChildrenEmpty,
	}, nil
}

func mysqlInspectColumns(ctx context.Context, database *sql.DB, databaseName, tableName string) ([]dto.TableColumn, error) {
	hasGenerationExpression, err := mysqlInformationSchemaColumnExists(ctx, database, "COLUMNS", "GENERATION_EXPRESSION")
	if err != nil {
		return nil, err
	}
	generationExpression := "''"
	if hasGenerationExpression {
		generationExpression = "COALESCE(c.generation_expression, '')"
	}
	query := "SELECT c.column_name, c.data_type, c.column_type, c.is_nullable, c.column_default, COALESCE(c.column_comment, ''), c.numeric_precision, c.numeric_scale, c.character_maximum_length, COALESCE(c.extra, ''), COALESCE(c.column_key, ''), " + generationExpression + " FROM information_schema.columns c WHERE c.table_schema = ? AND c.table_name = ? ORDER BY c.ordinal_position"
	rows, err := database.QueryContext(ctx, query, databaseName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dto.TableColumn, 0)
	for rows.Next() {
		var column dto.TableColumn
		var nullable string
		var defaultValue sql.NullString
		var precision, scale, length sql.NullInt64
		var extra, key, generation string
		if err := rows.Scan(&column.Name, &column.DataType, &column.DatabaseType, &nullable, &defaultValue, &column.Comment, &precision, &scale, &length, &extra, &key, &generation); err != nil {
			return nil, err
		}
		column.Nullable = strings.EqualFold(nullable, "YES")
		if defaultValue.Valid {
			value := defaultValue.String
			column.DefaultValue = &value
		}
		if precision.Valid {
			value := precision.Int64
			column.Precision = &value
		}
		if scale.Valid {
			value := scale.Int64
			column.Scale = &value
		}
		if length.Valid {
			value := length.Int64
			column.Length = &value
		}
		column.EnumValues = parseMySQLEnumValues(column.DatabaseType)
		column.Identity = strings.Contains(strings.ToLower(extra), "auto_increment")
		lowerExtra := strings.ToLower(extra)
		column.Generated = generation != "" || strings.Contains(lowerExtra, "virtual generated") || strings.Contains(lowerExtra, "stored generated")
		column.PrimaryKey = strings.EqualFold(key, "PRI")
		result = append(result, column)
	}
	return result, rows.Err()
}

func mysqlInspectIndexes(ctx context.Context, database *sql.DB, databaseName, tableName string) ([]dto.TableIndex, error) {
	hasExpression, err := mysqlInformationSchemaColumnExists(ctx, database, "STATISTICS", "EXPRESSION")
	if err != nil {
		return nil, err
	}
	expressionColumn := "''"
	if hasExpression {
		expressionColumn = "COALESCE(s.`EXPRESSION`, '')"
	}
	query := "SELECT s.index_name, s.non_unique, s.index_type, s.seq_in_index, COALESCE(s.column_name, ''), COALESCE(s.sub_part, 0), COALESCE(s.collation, ''), " + expressionColumn + " FROM information_schema.statistics s WHERE s.table_schema = ? AND s.table_name = ? ORDER BY s.index_name, s.seq_in_index"
	rows, err := database.QueryContext(ctx, query, databaseName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	metadata := make(map[string]*mysqlIndexMetadata)
	order := make([]string, 0)
	for rows.Next() {
		var name, typeName, column, collation, expression string
		var nonUnique int64
		var sequence, prefix int64
		if err := rows.Scan(&name, &nonUnique, &typeName, &sequence, &column, &prefix, &collation, &expression); err != nil {
			return nil, err
		}
		item := metadata[name]
		if item == nil {
			item = &mysqlIndexMetadata{name: name, nonUnique: nonUnique != 0, typeName: typeName}
			metadata[name] = item
			order = append(order, name)
		}
		item.parts = append(item.parts, mysqlIndexPart{column: column, expression: expression, prefix: prefix, descending: strings.EqualFold(collation, "D")})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]dto.TableIndex, 0, len(order))
	dialect := mysqlDialect{}
	for _, name := range order {
		item := metadata[name]
		parts := make([]string, 0, len(item.parts))
		for _, part := range item.parts {
			value := ""
			if part.expression != "" {
				value = "(" + part.expression + ")"
			} else if part.column != "" {
				value = dialect.quoteIdentifier(part.column)
			} else {
				value = "(<expression>)"
			}
			if part.prefix > 0 && part.column != "" {
				value += "(" + strconv.FormatInt(part.prefix, 10) + ")"
			}
			if part.descending {
				value += " DESC"
			}
			parts = append(parts, value)
		}
		primary := strings.EqualFold(item.name, "PRIMARY")
		unique := !item.nonUnique
		definition := "INDEX " + dialect.quoteIdentifier(item.name) + " (" + strings.Join(parts, ", ") + ")"
		if primary {
			definition = "PRIMARY KEY (" + strings.Join(parts, ", ") + ")"
		} else if unique {
			definition = "UNIQUE " + definition
		}
		result = append(result, dto.TableIndex{Name: item.name, Unique: unique, Primary: primary, Type: item.typeName, Definition: definition})
	}
	return result, nil
}

func mysqlInspectConstraints(ctx context.Context, database *sql.DB, databaseName, tableName string) ([]dto.TableConstraint, error) {
	query := "SELECT tc.constraint_name, tc.constraint_type, COALESCE(kcu.column_name, ''), COALESCE(kcu.ordinal_position, 0), COALESCE(kcu.referenced_table_schema, ''), COALESCE(kcu.referenced_table_name, ''), COALESCE(kcu.referenced_column_name, ''), COALESCE(rc.update_rule, ''), COALESCE(rc.delete_rule, '') FROM information_schema.table_constraints tc LEFT JOIN information_schema.key_column_usage kcu ON tc.constraint_catalog = kcu.constraint_catalog AND tc.constraint_schema = kcu.constraint_schema AND tc.constraint_name = kcu.constraint_name AND tc.table_name = kcu.table_name LEFT JOIN information_schema.referential_constraints rc ON tc.constraint_catalog = rc.constraint_catalog AND tc.constraint_schema = rc.constraint_schema AND tc.constraint_name = rc.constraint_name WHERE tc.table_schema = ? AND tc.table_name = ? ORDER BY tc.constraint_name, kcu.ordinal_position"
	rows, err := database.QueryContext(ctx, query, databaseName, tableName)
	if err != nil {
		return nil, err
	}
	metadata := make(map[string]*mysqlConstraintMetadata)
	order := make([]string, 0)
	for rows.Next() {
		var name, typeName, column, referencedSchema, referencedTable, referencedColumn, updateRule, deleteRule string
		var ordinal int64
		if err := rows.Scan(&name, &typeName, &column, &ordinal, &referencedSchema, &referencedTable, &referencedColumn, &updateRule, &deleteRule); err != nil {
			rows.Close()
			return nil, err
		}
		item := metadata[name]
		if item == nil {
			item = &mysqlConstraintMetadata{name: name, typeName: strings.ToUpper(typeName), referencedSchema: referencedSchema, referencedTable: referencedTable, updateRule: updateRule, deleteRule: deleteRule}
			metadata[name] = item
			order = append(order, name)
		}
		if column != "" {
			item.columns = append(item.columns, column)
		}
		if referencedColumn != "" {
			item.referencedCols = append(item.referencedCols, referencedColumn)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	checks, err := mysqlLoadCheckClauses(ctx, database, databaseName, tableName)
	if err != nil {
		return nil, err
	}
	for name, clause := range checks {
		if item := metadata[name]; item != nil {
			item.checkClause = clause
		}
	}
	dialect := mysqlDialect{}
	result := make([]dto.TableConstraint, 0, len(order))
	for _, name := range order {
		item := metadata[name]
		columns := make([]string, len(item.columns))
		for index, column := range item.columns {
			columns[index] = dialect.quoteIdentifier(column)
		}
		definition := item.typeName
		switch item.typeName {
		case "PRIMARY KEY":
			definition = "PRIMARY KEY (" + strings.Join(columns, ", ") + ")"
		case "UNIQUE":
			definition = "UNIQUE (" + strings.Join(columns, ", ") + ")"
		case "FOREIGN KEY":
			referenced := make([]string, len(item.referencedCols))
			for index, column := range item.referencedCols {
				referenced[index] = dialect.quoteIdentifier(column)
			}
			referencedTable := dialect.quoteIdentifier(item.referencedTable)
			if item.referencedSchema != "" {
				referencedTable = dialect.quoteIdentifier(item.referencedSchema) + "." + referencedTable
			}
			definition = "FOREIGN KEY (" + strings.Join(columns, ", ") + ") REFERENCES " + referencedTable + " (" + strings.Join(referenced, ", ") + ")"
			if item.deleteRule != "" {
				definition += " ON DELETE " + strings.ToUpper(item.deleteRule)
			}
			if item.updateRule != "" {
				definition += " ON UPDATE " + strings.ToUpper(item.updateRule)
			}
		case "CHECK":
			if item.checkClause != "" {
				clause := strings.TrimSpace(item.checkClause)
				if strings.HasPrefix(clause, "(") && strings.HasSuffix(clause, ")") {
					definition = "CHECK " + clause
				} else {
					definition = "CHECK (" + clause + ")"
				}
			}
		}
		result = append(result, dto.TableConstraint{Name: item.name, Type: item.typeName, Columns: append([]string(nil), item.columns...), Definition: definition})
	}
	return result, nil
}

func mysqlLoadCheckClauses(ctx context.Context, database *sql.DB, databaseName, tableName string) (map[string]string, error) {
	result := map[string]string{}
	query := "SELECT cc.constraint_name, COALESCE(cc.check_clause, '') FROM information_schema.check_constraints cc JOIN information_schema.table_constraints tc ON cc.constraint_catalog = tc.constraint_catalog AND cc.constraint_schema = tc.constraint_schema AND cc.constraint_name = tc.constraint_name WHERE tc.table_schema = ? AND tc.table_name = ? AND tc.constraint_type = 'CHECK'"
	rows, err := database.QueryContext(ctx, query, databaseName, tableName)
	if err != nil {
		if mysqlOptionalMetadataError(err) {
			return result, nil
		}
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, clause string
		if err := rows.Scan(&name, &clause); err != nil {
			return nil, err
		}
		result[name] = clause
	}
	return result, rows.Err()
}

func mysqlInformationSchemaColumnExists(ctx context.Context, database *sql.DB, tableName, columnName string) (bool, error) {
	var count int64
	err := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = 'information_schema' AND UPPER(table_name) = UPPER(?) AND UPPER(column_name) = UPPER(?)", tableName, columnName).Scan(&count)
	return count > 0, err
}

func parseMySQLEnumValues(columnType string) []string {
	trimmed := strings.TrimSpace(columnType)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "enum") && !strings.HasPrefix(lower, "set") {
		return nil
	}
	open := strings.IndexByte(trimmed, '(')
	if open < 0 {
		return nil
	}
	normalizedType := strings.ToLower(strings.TrimSpace(trimmed[:open]))
	if normalizedType != "enum" && normalizedType != "set" {
		return nil
	}
	index := open + 1
	result := make([]string, 0)
	for {
		for index < len(trimmed) && (trimmed[index] == ' ' || trimmed[index] == '\t' || trimmed[index] == '\n' || trimmed[index] == '\r' || trimmed[index] == ',') {
			index++
		}
		if index >= len(trimmed) || trimmed[index] == ')' {
			break
		}
		if trimmed[index] != '\'' {
			return nil
		}
		index++
		var value strings.Builder
		closed := false
		for index < len(trimmed) {
			character := trimmed[index]
			if character == '\'' {
				if index+1 < len(trimmed) && trimmed[index+1] == '\'' {
					value.WriteByte('\'')
					index += 2
					continue
				}
				index++
				closed = true
				break
			}
			if character == '\\' && index+1 < len(trimmed) {
				index++
				escaped := trimmed[index]
				switch escaped {
				case '0':
					value.WriteByte(0)
				case 'b':
					value.WriteByte('\b')
				case 'n':
					value.WriteByte('\n')
				case 'r':
					value.WriteByte('\r')
				case 't':
					value.WriteByte('\t')
				case 'Z':
					value.WriteByte(26)
				default:
					value.WriteByte(escaped)
				}
				index++
				continue
			}
			value.WriteByte(character)
			index++
		}
		if !closed {
			return nil
		}
		result = append(result, value.String())
		for index < len(trimmed) && (trimmed[index] == ' ' || trimmed[index] == '\t' || trimmed[index] == '\n' || trimmed[index] == '\r') {
			index++
		}
		if index < len(trimmed) && trimmed[index] == ',' {
			index++
			continue
		}
		if index < len(trimmed) && trimmed[index] == ')' {
			index++
			if strings.TrimSpace(trimmed[index:]) != "" {
				return nil
			}
			break
		}
		return nil
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func mysqlOptionalMetadataError(err error) bool {
	var driverError *mysqldriver.MySQLError
	if !errors.As(err, &driverError) {
		return false
	}
	return driverError.Number == 1054 || driverError.Number == 1109 || driverError.Number == 1146
}

func mysqlStringInClause(values []string) (string, []any) {
	placeholders := make([]string, len(values))
	arguments := make([]any, len(values))
	for index, value := range values {
		placeholders[index] = "?"
		arguments[index] = value
	}
	return strings.Join(placeholders, ","), arguments
}

func mysqlReferenceDatabase(connection entity.Connection, reference catalogReference) string {
	if reference.Database != "" {
		return reference.Database
	}
	if reference.Schema != "" {
		return reference.Schema
	}
	return connection.Database
}

func mysqlInt64Pointer(value int64) *int64 {
	return &value
}
