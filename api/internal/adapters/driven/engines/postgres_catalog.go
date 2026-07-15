package engines

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/lib/pq"
)

type postgresCatalogGroup struct {
	label string
	kind  dto.CatalogObjectKind
}

type postgresCatalogCounts map[string]map[dto.CatalogObjectKind]int64

type postgresCatalogTarget struct {
	reference   catalogReference
	groupKind   dto.CatalogObjectKind
	objectName  string
	loadObjects bool
	loadColumns bool
	offset      int
	paginated   bool
	hasMore     bool
}

type postgresCatalogRelation struct {
	oid  int64
	name string
}

type postgresRelationMetadata struct {
	oid            int64
	kind           string
	persistence    string
	isPartition    bool
	isPopulated    bool
	partitionKey   string
	partitionBound string
	parentSchema   string
	parentName     string
	accessMethod   string
	options        string
	tablespace     string
}

type postgresDDLColumn struct {
	name            string
	databaseType    string
	nullable        bool
	defaultValue    sql.NullString
	identityKind    string
	generatedKind   string
	collationSchema string
	collationName   string
	identityOptions string
}

var postgresCatalogGroups = []postgresCatalogGroup{
	{label: "Tables", kind: dto.CatalogObjectTable},
	{label: "Views", kind: dto.CatalogObjectView},
	{label: "Materialized Views", kind: dto.CatalogObjectMaterializedView},
	{label: "Functions", kind: dto.CatalogObjectFunction},
	{label: "Procedures", kind: dto.CatalogObjectProcedure},
	{label: "Sequences", kind: dto.CatalogObjectSequence},
	{label: "Extensions", kind: dto.CatalogObjectExtension},
	{label: "Triggers", kind: dto.CatalogObjectTrigger},
}

func readPostgresCatalog(ctx context.Context, database *sql.DB, connection entity.Connection, input dto.CatalogInput) (dto.CatalogTree, error) {
	limit := input.Limit
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	target, err := postgresResolveCatalogTarget(input)
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
	var databaseName string
	if err := database.QueryRowContext(ctx, "SELECT current_database()").Scan(&databaseName); err != nil {
		return dto.CatalogTree{}, unwrapCatalogError(err)
	}
	if target != nil && target.reference.Database != "" && target.reference.Database != databaseName {
		return dto.CatalogTree{}, apperror.NewNotFound("catalog object was not found", nil)
	}
	schemaName := ""
	if target != nil {
		schemaName = target.reference.Schema
	}
	schemaLimit := limit
	schemaOffset := 0
	if rootPage {
		schemaLimit++
		schemaOffset = offset
	}
	schemas, err := postgresCatalogSchemas(ctx, database, schemaName, schemaLimit, schemaOffset)
	if err != nil {
		return dto.CatalogTree{}, unwrapCatalogError(err)
	}
	if schemaName != "" && len(schemas) == 0 {
		return dto.CatalogTree{}, apperror.NewNotFound("catalog object was not found", nil)
	}
	counts, err := postgresLoadCatalogCounts(ctx, database)
	if err != nil {
		return dto.CatalogTree{}, unwrapCatalogError(err)
	}
	rootHasMore := rootPage && len(schemas) > limit
	if rootHasMore {
		schemas = schemas[:limit]
	}
	databaseReference := catalogReference{Kind: dto.CatalogObjectDatabase, Database: databaseName}
	databaseNode, err := postgresNewCatalogObject(connection.ID, "", databaseName, databaseName, databaseReference)
	if err != nil {
		return dto.CatalogTree{}, err
	}
	databaseNode.Database = databaseName
	databaseNode.Capabilities = catalogCapabilities(connection.Engine)
	databaseNode.Children = make([]dto.CatalogObject, 0, len(schemas))
	for _, schema := range schemas {
		schemaNode, buildErr := postgresBuildSchemaNode(ctx, database, connection, databaseName, schema, databaseNode.ID, counts[schema], target, limit, input.Depth)
		if buildErr != nil {
			return dto.CatalogTree{}, unwrapCatalogError(buildErr)
		}
		databaseNode.Children = append(databaseNode.Children, schemaNode)
	}
	databaseNode.ChildrenState = catalogChildrenState(databaseNode.Children)
	result := dto.CatalogTree{
		ConnectionID: connection.ID,
		Engine:       connection.Engine,
		Capabilities: catalogCapabilities(connection.Engine),
		Databases:    []dto.CatalogObject{databaseNode},
		LoadedAt:     time.Now().UTC(),
	}
	if rootHasMore || groupPage && target.hasMore {
		result.NextCursor, err = encodeCatalogCursor(connection.ID, input.ParentReference, offset+limit)
		if err != nil {
			return dto.CatalogTree{}, err
		}
	}
	return result, nil
}

func readPostgresTableSchema(ctx context.Context, database *sql.DB, connection entity.Connection, reference catalogReference) (dto.TableSchema, error) {
	metadata, err := postgresReadRelationMetadata(ctx, database, connection, reference)
	if err != nil {
		return dto.TableSchema{}, err
	}
	columns, err := postgresInspectColumns(ctx, database, reference.Schema, reference.Name)
	if err != nil {
		return dto.TableSchema{}, unwrapCatalogError(err)
	}
	if len(columns) == 0 && metadata.kind != "v" && metadata.kind != "m" {
		return dto.TableSchema{}, apperror.NewNotFound("table or view was not found", nil)
	}
	indexes, err := postgresInspectIndexes(ctx, database, metadata.oid)
	if err != nil {
		return dto.TableSchema{}, unwrapCatalogError(err)
	}
	constraints, err := postgresInspectConstraints(ctx, database, metadata.oid)
	if err != nil {
		return dto.TableSchema{}, unwrapCatalogError(err)
	}
	return dto.TableSchema{Columns: columns, Indexes: indexes, Constraints: constraints}, nil
}

func readPostgresTableDDL(ctx context.Context, database *sql.DB, connection entity.Connection, reference catalogReference) (string, error) {
	metadata, err := postgresReadRelationMetadata(ctx, database, connection, reference)
	if err != nil {
		return "", err
	}
	dialect := postgresDialect{}
	qualified, err := dialect.qualified(reference)
	if err != nil {
		return "", err
	}
	if metadata.kind == "v" || metadata.kind == "m" {
		var definition string
		if err := database.QueryRowContext(ctx, "SELECT pg_catalog.pg_get_viewdef($1::oid, true)", metadata.oid).Scan(&definition); err != nil {
			return "", unwrapCatalogError(err)
		}
		prefix := "CREATE VIEW "
		if metadata.kind == "m" {
			prefix = "CREATE MATERIALIZED VIEW "
		}
		statement := prefix + qualified
		if metadata.accessMethod != "" {
			statement += " USING " + dialect.quoteIdentifier(metadata.accessMethod)
		}
		if metadata.options != "" {
			statement += " WITH (" + metadata.options + ")"
		}
		if metadata.tablespace != "" {
			statement += " TABLESPACE " + dialect.quoteIdentifier(metadata.tablespace)
		}
		statement += " AS\n" + strings.TrimSpace(definition)
		if metadata.kind == "m" && !metadata.isPopulated {
			statement += "\nWITH NO DATA"
		}
		statement += ";"
		indexes, indexErr := postgresDDLIndexes(ctx, database, metadata.oid)
		if indexErr != nil {
			return "", unwrapCatalogError(indexErr)
		}
		if len(indexes) > 0 {
			statement += "\n\n" + strings.Join(indexes, "\n")
		}
		return statement, nil
	}
	columns, err := postgresLoadDDLColumns(ctx, database, metadata.oid)
	if err != nil {
		return "", unwrapCatalogError(err)
	}
	constraints, err := postgresLoadDDLConstraints(ctx, database, metadata.oid)
	if err != nil {
		return "", unwrapCatalogError(err)
	}
	statement := ""
	if metadata.isPartition && metadata.parentSchema != "" && metadata.parentName != "" {
		parent := dialect.quoteIdentifier(metadata.parentSchema) + "." + dialect.quoteIdentifier(metadata.parentName)
		statement = "CREATE TABLE " + qualified + " PARTITION OF " + parent
		if metadata.partitionBound != "" {
			statement += " " + metadata.partitionBound
		}
	} else {
		prefix := "CREATE TABLE "
		if metadata.persistence == "u" {
			prefix = "CREATE UNLOGGED TABLE "
		} else if metadata.persistence == "t" {
			prefix = "CREATE TEMPORARY TABLE "
		}
		definitions := make([]string, 0, len(columns)+len(constraints))
		for _, column := range columns {
			definitions = append(definitions, "    "+postgresRenderDDLColumn(column, dialect))
		}
		for _, constraint := range constraints {
			definitions = append(definitions, "    "+constraint)
		}
		statement = prefix + qualified + " (\n" + strings.Join(definitions, ",\n") + "\n)"
		if metadata.kind == "p" && metadata.partitionKey != "" {
			statement += " PARTITION BY " + metadata.partitionKey
		} else {
			parents, parentErr := postgresLoadInheritanceParents(ctx, database, metadata.oid)
			if parentErr != nil {
				return "", unwrapCatalogError(parentErr)
			}
			if len(parents) > 0 {
				statement += " INHERITS (" + strings.Join(parents, ", ") + ")"
			}
		}
	}
	if metadata.accessMethod != "" {
		statement += " USING " + dialect.quoteIdentifier(metadata.accessMethod)
	}
	if metadata.options != "" {
		statement += " WITH (" + metadata.options + ")"
	}
	if metadata.tablespace != "" {
		statement += " TABLESPACE " + dialect.quoteIdentifier(metadata.tablespace)
	}
	statement += ";"
	indexes, err := postgresDDLIndexes(ctx, database, metadata.oid)
	if err != nil {
		return "", unwrapCatalogError(err)
	}
	if len(indexes) > 0 {
		statement += "\n\n" + strings.Join(indexes, "\n")
	}
	return statement, nil
}

func postgresResolveCatalogTarget(input dto.CatalogInput) (*postgresCatalogTarget, error) {
	if input.ParentReference == "" {
		return nil, nil
	}
	reference, err := decodeCatalogReference(input.ParentReference)
	if err != nil {
		return nil, err
	}
	target := &postgresCatalogTarget{reference: reference}
	switch reference.Kind {
	case dto.CatalogObjectDatabase, dto.CatalogObjectSchema:
	case dto.CatalogObjectGroup:
		kind, ok := postgresGroupKind(reference.Name)
		if !ok {
			return nil, apperror.NewUnsupported("catalog group is not supported by PostgreSQL", nil)
		}
		target.groupKind = kind
		target.loadObjects = true
	case dto.CatalogObjectTable, dto.CatalogObjectView, dto.CatalogObjectMaterializedView, dto.CatalogObjectFunction, dto.CatalogObjectProcedure, dto.CatalogObjectSequence, dto.CatalogObjectExtension, dto.CatalogObjectTrigger:
		target.groupKind = reference.Kind
		target.objectName = reference.Name
		target.loadObjects = true
		target.loadColumns = reference.Kind == dto.CatalogObjectTable || reference.Kind == dto.CatalogObjectView || reference.Kind == dto.CatalogObjectMaterializedView
	default:
		return nil, apperror.NewUnsupported("catalog object is not supported by PostgreSQL", nil)
	}
	if reference.Kind != dto.CatalogObjectDatabase && reference.Schema == "" {
		return nil, apperror.NewValidation("PostgreSQL catalog reference has no schema", nil)
	}
	return target, nil
}

func postgresGroupKind(value string) (dto.CatalogObjectKind, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, group := range postgresCatalogGroups {
		if normalized == string(group.kind) || normalized == strings.ToLower(group.label) {
			return group.kind, true
		}
	}
	return "", false
}

func postgresCatalogSchemas(ctx context.Context, database *sql.DB, schemaName string, limit, offset int) ([]string, error) {
	query := "SELECT nspname FROM pg_catalog.pg_namespace WHERE nspname <> 'information_schema' AND nspname NOT LIKE 'pg\\_%' ESCAPE '\\'"
	arguments := make([]any, 0, 2)
	if schemaName != "" {
		query += " AND nspname = $1"
		arguments = append(arguments, schemaName)
	}
	query += fmt.Sprintf(" ORDER BY nspname LIMIT $%d OFFSET $%d", len(arguments)+1, len(arguments)+2)
	if schemaName != "" {
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

func postgresLoadCatalogCounts(ctx context.Context, database *sql.DB) (postgresCatalogCounts, error) {
	query := `
SELECT schema_name, object_kind, SUM(object_count)::bigint
FROM (
    SELECT n.nspname AS schema_name,
           CASE c.relkind
               WHEN 'v' THEN 'view'
               WHEN 'm' THEN 'materialized-view'
               WHEN 'S' THEN 'sequence'
               ELSE 'table'
           END AS object_kind,
           COUNT(*)::bigint AS object_count
    FROM pg_catalog.pg_class c
    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname <> 'information_schema'
      AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
      AND c.relkind IN ('r', 'p', 'v', 'm', 'S')
    GROUP BY n.nspname, CASE c.relkind WHEN 'v' THEN 'view' WHEN 'm' THEN 'materialized-view' WHEN 'S' THEN 'sequence' ELSE 'table' END
    UNION ALL
    SELECT n.nspname,
           CASE WHEN p.prokind = 'p' THEN 'procedure' ELSE 'function' END,
           COUNT(*)::bigint
    FROM pg_catalog.pg_proc p
    JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
    WHERE n.nspname <> 'information_schema'
      AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
      AND p.prokind IN ('f', 'p', 'w', 'a')
    GROUP BY n.nspname, CASE WHEN p.prokind = 'p' THEN 'procedure' ELSE 'function' END
    UNION ALL
    SELECT n.nspname, 'extension', COUNT(*)::bigint
    FROM pg_catalog.pg_extension e
    JOIN pg_catalog.pg_namespace n ON n.oid = e.extnamespace
    WHERE n.nspname <> 'information_schema'
      AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
    GROUP BY n.nspname
    UNION ALL
    SELECT n.nspname, 'trigger', COUNT(*)::bigint
    FROM pg_catalog.pg_trigger t
    JOIN pg_catalog.pg_class c ON c.oid = t.tgrelid
    JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
    WHERE NOT t.tgisinternal
      AND n.nspname <> 'information_schema'
      AND n.nspname NOT LIKE 'pg\_%' ESCAPE '\'
    GROUP BY n.nspname
) counts
GROUP BY schema_name, object_kind`
	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := postgresCatalogCounts{}
	for rows.Next() {
		var schema, kind string
		var count int64
		if err := rows.Scan(&schema, &kind, &count); err != nil {
			return nil, err
		}
		if result[schema] == nil {
			result[schema] = map[dto.CatalogObjectKind]int64{}
		}
		result[schema][dto.CatalogObjectKind(kind)] = count
	}
	return result, rows.Err()
}

func postgresBuildSchemaNode(ctx context.Context, database *sql.DB, connection entity.Connection, databaseName, schemaName, parentID string, counts map[dto.CatalogObjectKind]int64, target *postgresCatalogTarget, limit int, depth dto.CatalogDepth) (dto.CatalogObject, error) {
	reference := catalogReference{Kind: dto.CatalogObjectSchema, Database: databaseName, Schema: schemaName}
	node, err := postgresNewCatalogObject(connection.ID, parentID, schemaName, databaseName+"."+schemaName, reference)
	if err != nil {
		return dto.CatalogObject{}, err
	}
	node.Database = databaseName
	node.Schema = schemaName
	node.Capabilities = catalogCapabilities(connection.Engine)
	groups := postgresCatalogGroups
	if target != nil && target.groupKind != "" {
		groups = make([]postgresCatalogGroup, 0, 1)
		for _, group := range postgresCatalogGroups {
			if group.kind == target.groupKind {
				groups = append(groups, group)
				break
			}
		}
	}
	node.Children = make([]dto.CatalogObject, 0, len(groups))
	for _, group := range groups {
		groupNode, buildErr := postgresBuildGroupNode(ctx, database, connection, databaseName, schemaName, node.ID, group, counts[group.kind], target, limit, depth)
		if buildErr != nil {
			return dto.CatalogObject{}, buildErr
		}
		node.Children = append(node.Children, groupNode)
	}
	node.ChildrenState = catalogChildrenState(node.Children)
	return node, nil
}

func postgresBuildGroupNode(ctx context.Context, database *sql.DB, connection entity.Connection, databaseName, schemaName, parentID string, group postgresCatalogGroup, count int64, target *postgresCatalogTarget, limit int, depth dto.CatalogDepth) (dto.CatalogObject, error) {
	reference := catalogReference{Kind: dto.CatalogObjectGroup, Database: databaseName, Schema: schemaName, Name: string(group.kind)}
	node, err := postgresNewCatalogObject(connection.ID, parentID, group.label, databaseName+"."+schemaName+"."+strings.ToLower(strings.ReplaceAll(group.label, " ", "-")), reference)
	if err != nil {
		return dto.CatalogObject{}, err
	}
	node.Database = databaseName
	node.Schema = schemaName
	node.Count = postgresInt64Pointer(count)
	node.Capabilities = []string{string(group.kind)}
	loadObjects := depth == dto.CatalogDepthAll
	loadColumns := depth == dto.CatalogDepthAll
	objectName := ""
	if target != nil && target.groupKind == group.kind {
		loadObjects = target.loadObjects || loadObjects
		loadColumns = target.loadColumns || loadColumns
		objectName = target.objectName
	}
	if !loadObjects {
		if count == 0 {
			node.ChildrenState = dto.CatalogChildrenEmpty
		} else {
			node.ChildrenState = dto.CatalogChildrenUnloaded
		}
		return node, nil
	}
	queryLimit := limit
	offset := 0
	paginated := target != nil && target.paginated && target.groupKind == group.kind
	if paginated {
		queryLimit++
		offset = target.offset
	}
	children, err := postgresLoadGroupObjects(ctx, database, connection.ID, databaseName, schemaName, node.ID, group.kind, objectName, queryLimit, offset, loadColumns)
	if err != nil {
		return dto.CatalogObject{}, err
	}
	if paginated && len(children) > limit {
		target.hasMore = true
		children = children[:limit]
	}
	if objectName != "" {
		expectedID := ""
		if target != nil {
			expectedID = catalogObjectID(connection.ID, target.reference)
		}
		matched := make([]dto.CatalogObject, 0, 1)
		for _, child := range children {
			if expectedID == "" || child.ID == expectedID {
				matched = append(matched, child)
			}
		}
		children = matched
		if len(children) == 0 {
			return dto.CatalogObject{}, apperror.NewNotFound("catalog object was not found", nil)
		}
	}
	node.Children = children
	node.ChildrenState = catalogChildrenState(children)
	return node, nil
}

func postgresLoadGroupObjects(ctx context.Context, database *sql.DB, connectionID, databaseName, schemaName, parentID string, kind dto.CatalogObjectKind, objectName string, limit, offset int, loadColumns bool) ([]dto.CatalogObject, error) {
	switch kind {
	case dto.CatalogObjectTable, dto.CatalogObjectView, dto.CatalogObjectMaterializedView:
		return postgresLoadRelations(ctx, database, connectionID, databaseName, schemaName, parentID, kind, objectName, limit, offset, loadColumns)
	case dto.CatalogObjectFunction, dto.CatalogObjectProcedure:
		return postgresLoadRoutines(ctx, database, connectionID, databaseName, schemaName, parentID, kind, objectName, limit, offset)
	case dto.CatalogObjectSequence:
		return postgresLoadSequences(ctx, database, connectionID, databaseName, schemaName, parentID, objectName, limit, offset)
	case dto.CatalogObjectExtension:
		return postgresLoadExtensions(ctx, database, connectionID, databaseName, schemaName, parentID, objectName, limit, offset)
	case dto.CatalogObjectTrigger:
		return postgresLoadTriggers(ctx, database, connectionID, databaseName, schemaName, parentID, objectName, limit, offset)
	default:
		return nil, apperror.NewUnsupported("catalog group is not supported by PostgreSQL", nil)
	}
}

func postgresLoadRelations(ctx context.Context, database *sql.DB, connectionID, databaseName, schemaName, parentID string, kind dto.CatalogObjectKind, objectName string, limit, offset int, loadColumns bool) ([]dto.CatalogObject, error) {
	relationKinds := "('r', 'p')"
	if kind == dto.CatalogObjectView {
		relationKinds = "('v')"
	} else if kind == dto.CatalogObjectMaterializedView {
		relationKinds = "('m')"
	}
	query := "SELECT c.oid::bigint, c.relname FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1 AND c.relkind IN " + relationKinds
	arguments := []any{schemaName}
	if objectName != "" {
		query += " AND c.relname = $2"
		arguments = append(arguments, objectName)
	}
	query += fmt.Sprintf(" ORDER BY c.relname LIMIT $%d OFFSET $%d", len(arguments)+1, len(arguments)+2)
	arguments = append(arguments, limit, offset)
	rows, err := database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	relations := make([]postgresCatalogRelation, 0)
	for rows.Next() {
		var relation postgresCatalogRelation
		if err := rows.Scan(&relation.oid, &relation.name); err != nil {
			rows.Close()
			return nil, err
		}
		relations = append(relations, relation)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	columns := map[int64][]dto.CatalogObject{}
	if loadColumns && len(relations) > 0 {
		columns, err = postgresLoadCatalogColumns(ctx, database, connectionID, databaseName, schemaName, relations)
		if err != nil {
			return nil, err
		}
	}
	result := make([]dto.CatalogObject, 0, len(relations))
	for _, relation := range relations {
		reference := catalogReference{Kind: kind, Database: databaseName, Schema: schemaName, Name: relation.name}
		object, err := postgresNewCatalogObject(connectionID, parentID, relation.name, schemaName+"."+relation.name, reference)
		if err != nil {
			return nil, err
		}
		object.Database = databaseName
		object.Schema = schemaName
		object.Capabilities = []string{"browse", "schema", "ddl"}
		if loadColumns {
			object.Children = columns[relation.oid]
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

func postgresLoadCatalogColumns(ctx context.Context, database *sql.DB, connectionID, databaseName, schemaName string, relations []postgresCatalogRelation) (map[int64][]dto.CatalogObject, error) {
	oids := make([]int64, 0, len(relations))
	names := make(map[int64]string, len(relations))
	for _, relation := range relations {
		oids = append(oids, relation.oid)
		names[relation.oid] = relation.name
	}
	rows, err := database.QueryContext(ctx, `
SELECT a.attrelid::bigint, a.attname, pg_catalog.format_type(a.atttypid, a.atttypmod)
FROM pg_catalog.pg_attribute a
WHERE a.attrelid = ANY($1::oid[])
  AND a.attnum > 0
  AND NOT a.attisdropped
ORDER BY a.attrelid, a.attnum`, pq.Array(oids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64][]dto.CatalogObject, len(relations))
	for rows.Next() {
		var oid int64
		var name, databaseType string
		if err := rows.Scan(&oid, &name, &databaseType); err != nil {
			return nil, err
		}
		owner := names[oid]
		reference := catalogReference{Kind: dto.CatalogObjectColumn, Database: databaseName, Schema: schemaName, Name: name, Owner: owner}
		object, err := postgresNewCatalogObject(connectionID, "", name, schemaName+"."+owner+"."+name, reference)
		if err != nil {
			return nil, err
		}
		object.Database = databaseName
		object.Schema = schemaName
		object.DataType = databaseType
		object.ChildrenState = dto.CatalogChildrenEmpty
		result[oid] = append(result[oid], object)
	}
	return result, rows.Err()
}

func postgresLoadRoutines(ctx context.Context, database *sql.DB, connectionID, databaseName, schemaName, parentID string, kind dto.CatalogObjectKind, objectName string, limit, offset int) ([]dto.CatalogObject, error) {
	condition := "p.prokind IN ('f', 'w', 'a')"
	if kind == dto.CatalogObjectProcedure {
		condition = "p.prokind = 'p'"
	}
	query := "SELECT p.proname, pg_catalog.pg_get_function_identity_arguments(p.oid), pg_catalog.pg_get_function_result(p.oid) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname = $1 AND " + condition
	arguments := []any{schemaName}
	if objectName != "" {
		query += " AND p.proname = $2"
		arguments = append(arguments, objectName)
	}
	query += fmt.Sprintf(" ORDER BY p.proname, pg_catalog.pg_get_function_identity_arguments(p.oid), p.oid LIMIT $%d OFFSET $%d", len(arguments)+1, len(arguments)+2)
	arguments = append(arguments, limit, offset)
	rows, err := database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dto.CatalogObject, 0)
	for rows.Next() {
		var name, arguments, resultType string
		if err := rows.Scan(&name, &arguments, &resultType); err != nil {
			return nil, err
		}
		signature := name + "(" + arguments + ")"
		reference := catalogReference{Kind: kind, Database: databaseName, Schema: schemaName, Name: name, Signature: signature}
		object, err := postgresNewCatalogObject(connectionID, parentID, name, schemaName+"."+signature, reference)
		if err != nil {
			return nil, err
		}
		object.Database = databaseName
		object.Schema = schemaName
		object.DataType = resultType
		object.Capabilities = []string{"ddl"}
		object.ChildrenState = dto.CatalogChildrenEmpty
		result = append(result, object)
	}
	return result, rows.Err()
}

func postgresLoadSequences(ctx context.Context, database *sql.DB, connectionID, databaseName, schemaName, parentID, objectName string, limit, offset int) ([]dto.CatalogObject, error) {
	query := "SELECT c.relname, pg_catalog.format_type(s.seqtypid, NULL) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace JOIN pg_catalog.pg_sequence s ON s.seqrelid = c.oid WHERE n.nspname = $1 AND c.relkind = 'S'"
	arguments := []any{schemaName}
	if objectName != "" {
		query += " AND c.relname = $2"
		arguments = append(arguments, objectName)
	}
	query += fmt.Sprintf(" ORDER BY c.relname LIMIT $%d OFFSET $%d", len(arguments)+1, len(arguments)+2)
	arguments = append(arguments, limit, offset)
	rows, err := database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dto.CatalogObject, 0)
	for rows.Next() {
		var name, dataType string
		if err := rows.Scan(&name, &dataType); err != nil {
			return nil, err
		}
		reference := catalogReference{Kind: dto.CatalogObjectSequence, Database: databaseName, Schema: schemaName, Name: name}
		object, err := postgresNewCatalogObject(connectionID, parentID, name, schemaName+"."+name, reference)
		if err != nil {
			return nil, err
		}
		object.Database = databaseName
		object.Schema = schemaName
		object.DataType = dataType
		object.Capabilities = []string{"ddl"}
		object.ChildrenState = dto.CatalogChildrenEmpty
		result = append(result, object)
	}
	return result, rows.Err()
}

func postgresLoadExtensions(ctx context.Context, database *sql.DB, connectionID, databaseName, schemaName, parentID, objectName string, limit, offset int) ([]dto.CatalogObject, error) {
	query := "SELECT e.extname, e.extversion FROM pg_catalog.pg_extension e JOIN pg_catalog.pg_namespace n ON n.oid = e.extnamespace WHERE n.nspname = $1"
	arguments := []any{schemaName}
	if objectName != "" {
		query += " AND e.extname = $2"
		arguments = append(arguments, objectName)
	}
	query += fmt.Sprintf(" ORDER BY e.extname LIMIT $%d OFFSET $%d", len(arguments)+1, len(arguments)+2)
	arguments = append(arguments, limit, offset)
	rows, err := database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dto.CatalogObject, 0)
	for rows.Next() {
		var name, version string
		if err := rows.Scan(&name, &version); err != nil {
			return nil, err
		}
		reference := catalogReference{Kind: dto.CatalogObjectExtension, Database: databaseName, Schema: schemaName, Name: name}
		object, err := postgresNewCatalogObject(connectionID, parentID, name, schemaName+"."+name, reference)
		if err != nil {
			return nil, err
		}
		object.Database = databaseName
		object.Schema = schemaName
		object.DataType = version
		object.Capabilities = []string{"ddl"}
		object.ChildrenState = dto.CatalogChildrenEmpty
		result = append(result, object)
	}
	return result, rows.Err()
}

func postgresLoadTriggers(ctx context.Context, database *sql.DB, connectionID, databaseName, schemaName, parentID, objectName string, limit, offset int) ([]dto.CatalogObject, error) {
	query := "SELECT t.tgname, c.relname, pg_catalog.pg_get_triggerdef(t.oid, true) FROM pg_catalog.pg_trigger t JOIN pg_catalog.pg_class c ON c.oid = t.tgrelid JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1 AND NOT t.tgisinternal"
	arguments := []any{schemaName}
	if objectName != "" {
		query += " AND t.tgname = $2"
		arguments = append(arguments, objectName)
	}
	query += fmt.Sprintf(" ORDER BY t.tgname, c.relname, t.oid LIMIT $%d OFFSET $%d", len(arguments)+1, len(arguments)+2)
	arguments = append(arguments, limit, offset)
	rows, err := database.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dto.CatalogObject, 0)
	for rows.Next() {
		var name, tableName, definition string
		if err := rows.Scan(&name, &tableName, &definition); err != nil {
			return nil, err
		}
		signature := name + " ON " + schemaName + "." + tableName
		reference := catalogReference{Kind: dto.CatalogObjectTrigger, Database: databaseName, Schema: schemaName, Name: name, Signature: signature, Owner: tableName}
		object, err := postgresNewCatalogObject(connectionID, parentID, name, schemaName+"."+signature, reference)
		if err != nil {
			return nil, err
		}
		object.Database = databaseName
		object.Schema = schemaName
		object.DataType = definition
		object.Capabilities = []string{"ddl"}
		object.ChildrenState = dto.CatalogChildrenEmpty
		result = append(result, object)
	}
	return result, rows.Err()
}

func postgresNewCatalogObject(connectionID, parentID, name, qualifiedName string, reference catalogReference) (dto.CatalogObject, error) {
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

func postgresReadRelationMetadata(ctx context.Context, database *sql.DB, connection entity.Connection, reference catalogReference) (postgresRelationMetadata, error) {
	if reference.Schema == "" || reference.Name == "" {
		return postgresRelationMetadata{}, apperror.NewValidation("invalid PostgreSQL table reference", nil)
	}
	var currentDatabase string
	if err := database.QueryRowContext(ctx, "SELECT current_database()").Scan(&currentDatabase); err != nil {
		return postgresRelationMetadata{}, unwrapCatalogError(err)
	}
	if reference.Database != "" && reference.Database != currentDatabase {
		return postgresRelationMetadata{}, apperror.NewNotFound("table or view was not found", nil)
	}
	var metadata postgresRelationMetadata
	err := database.QueryRowContext(ctx, `
SELECT c.oid::bigint,
       c.relkind::text,
       c.relpersistence::text,
       c.relispartition,
       c.relispopulated,
       COALESCE(pg_catalog.pg_get_partkeydef(c.oid), ''),
       COALESCE(pg_catalog.pg_get_expr(c.relpartbound, c.oid, true), ''),
       COALESCE((SELECT pn.nspname FROM pg_catalog.pg_inherits i JOIN pg_catalog.pg_class pc ON pc.oid = i.inhparent JOIN pg_catalog.pg_namespace pn ON pn.oid = pc.relnamespace WHERE i.inhrelid = c.oid ORDER BY i.inhseqno LIMIT 1), ''),
       COALESCE((SELECT pc.relname FROM pg_catalog.pg_inherits i JOIN pg_catalog.pg_class pc ON pc.oid = i.inhparent WHERE i.inhrelid = c.oid ORDER BY i.inhseqno LIMIT 1), ''),
       COALESCE(am.amname, ''),
       COALESCE(array_to_string(c.reloptions, ', '), ''),
       COALESCE(ts.spcname, '')
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_catalog.pg_am am ON am.oid = c.relam
LEFT JOIN pg_catalog.pg_tablespace ts ON ts.oid = c.reltablespace
WHERE n.nspname = $1
  AND c.relname = $2
  AND c.relkind IN ('r', 'p', 'v', 'm')`, reference.Schema, reference.Name).Scan(
		&metadata.oid,
		&metadata.kind,
		&metadata.persistence,
		&metadata.isPartition,
		&metadata.isPopulated,
		&metadata.partitionKey,
		&metadata.partitionBound,
		&metadata.parentSchema,
		&metadata.parentName,
		&metadata.accessMethod,
		&metadata.options,
		&metadata.tablespace,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return postgresRelationMetadata{}, apperror.NewNotFound("table or view was not found", nil)
	}
	if err != nil {
		return postgresRelationMetadata{}, unwrapCatalogError(err)
	}
	valid := reference.Kind == dto.CatalogObjectTable && (metadata.kind == "r" || metadata.kind == "p")
	valid = valid || reference.Kind == dto.CatalogObjectView && metadata.kind == "v"
	valid = valid || reference.Kind == dto.CatalogObjectMaterializedView && metadata.kind == "m"
	if !valid {
		return postgresRelationMetadata{}, apperror.NewNotFound("table or view was not found", nil)
	}
	if connection.Engine != entity.EnginePostgreSQL {
		return postgresRelationMetadata{}, apperror.NewUnsupported("table schema is not supported", nil)
	}
	return metadata, nil
}

func postgresInspectColumns(ctx context.Context, database *sql.DB, schemaName, tableName string) ([]dto.TableColumn, error) {
	rows, err := database.QueryContext(ctx, `
SELECT a.attname,
       COALESCE(ic.data_type, t.typname),
       pg_catalog.format_type(a.atttypid, a.atttypmod),
       NOT a.attnotnull,
       pg_catalog.pg_get_expr(ad.adbin, ad.adrelid, true),
       COALESCE(pg_catalog.col_description(a.attrelid, a.attnum), ''),
       ic.numeric_precision::bigint,
       ic.numeric_scale::bigint,
       ic.character_maximum_length::bigint,
       a.attidentity <> '',
       a.attgenerated <> '',
       EXISTS (
           SELECT 1
           FROM pg_catalog.pg_index i
           WHERE i.indrelid = a.attrelid
             AND i.indisprimary
             AND a.attnum = ANY(i.indkey::smallint[])
       ),
       COALESCE((SELECT json_agg(e.enumlabel ORDER BY e.enumsortorder)::text FROM pg_catalog.pg_enum e WHERE e.enumtypid = a.atttypid), '[]')
FROM pg_catalog.pg_attribute a
JOIN pg_catalog.pg_class cls ON cls.oid = a.attrelid
JOIN pg_catalog.pg_namespace n ON n.oid = cls.relnamespace
JOIN pg_catalog.pg_type t ON t.oid = a.atttypid
LEFT JOIN pg_catalog.pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
LEFT JOIN information_schema.columns ic ON ic.table_schema = n.nspname AND ic.table_name = cls.relname AND ic.ordinal_position = a.attnum
WHERE n.nspname = $1
  AND cls.relname = $2
  AND cls.relkind IN ('r', 'p', 'v', 'm')
  AND a.attnum > 0
  AND NOT a.attisdropped
ORDER BY a.attnum`, schemaName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dto.TableColumn, 0)
	for rows.Next() {
		var column dto.TableColumn
		var defaultValue sql.NullString
		var precision, scale, length sql.NullInt64
		var enumJSON string
		if err := rows.Scan(
			&column.Name,
			&column.DataType,
			&column.DatabaseType,
			&column.Nullable,
			&defaultValue,
			&column.Comment,
			&precision,
			&scale,
			&length,
			&column.Identity,
			&column.Generated,
			&column.PrimaryKey,
			&enumJSON,
		); err != nil {
			return nil, err
		}
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
		if err := json.Unmarshal([]byte(enumJSON), &column.EnumValues); err != nil {
			return nil, err
		}
		result = append(result, column)
	}
	return result, rows.Err()
}

func postgresInspectIndexes(ctx context.Context, database *sql.DB, relationOID int64) ([]dto.TableIndex, error) {
	rows, err := database.QueryContext(ctx, `
SELECT ic.relname,
       i.indisunique,
       i.indisprimary,
       am.amname,
       pg_catalog.pg_get_indexdef(i.indexrelid, 0, true)
FROM pg_catalog.pg_index i
JOIN pg_catalog.pg_class ic ON ic.oid = i.indexrelid
JOIN pg_catalog.pg_am am ON am.oid = ic.relam
WHERE i.indrelid = $1::oid
ORDER BY ic.relname`, relationOID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dto.TableIndex, 0)
	for rows.Next() {
		var index dto.TableIndex
		if err := rows.Scan(&index.Name, &index.Unique, &index.Primary, &index.Type, &index.Definition); err != nil {
			return nil, err
		}
		result = append(result, index)
	}
	return result, rows.Err()
}

func postgresInspectConstraints(ctx context.Context, database *sql.DB, relationOID int64) ([]dto.TableConstraint, error) {
	rows, err := database.QueryContext(ctx, `
SELECT con.conname,
       con.contype::text,
       COALESCE((
           SELECT json_agg(a.attname ORDER BY keys.ordinality)::text
           FROM unnest(con.conkey) WITH ORDINALITY AS keys(attnum, ordinality)
           JOIN pg_catalog.pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = keys.attnum
       ), '[]'),
       pg_catalog.pg_get_constraintdef(con.oid, true)
FROM pg_catalog.pg_constraint con
WHERE con.conrelid = $1::oid
ORDER BY con.conname`, relationOID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dto.TableConstraint, 0)
	for rows.Next() {
		var constraint dto.TableConstraint
		var kind, columnsJSON string
		if err := rows.Scan(&constraint.Name, &kind, &columnsJSON, &constraint.Definition); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(columnsJSON), &constraint.Columns); err != nil {
			return nil, err
		}
		constraint.Type = postgresConstraintType(kind)
		result = append(result, constraint)
	}
	return result, rows.Err()
}

func postgresConstraintType(kind string) string {
	switch kind {
	case "p":
		return "PRIMARY KEY"
	case "u":
		return "UNIQUE"
	case "f":
		return "FOREIGN KEY"
	case "c":
		return "CHECK"
	case "x":
		return "EXCLUSION"
	case "t":
		return "CONSTRAINT TRIGGER"
	case "n":
		return "NOT NULL"
	default:
		return strings.ToUpper(kind)
	}
}

func postgresLoadDDLColumns(ctx context.Context, database *sql.DB, relationOID int64) ([]postgresDDLColumn, error) {
	rows, err := database.QueryContext(ctx, `
SELECT a.attname,
       pg_catalog.format_type(a.atttypid, a.atttypmod),
       NOT a.attnotnull,
       pg_catalog.pg_get_expr(ad.adbin, ad.adrelid, true),
       a.attidentity::text,
       a.attgenerated::text,
       COALESCE(cn.nspname, ''),
       COALESCE(coll.collname, ''),
       COALESCE((
           SELECT format(
               'INCREMENT BY %s MINVALUE %s MAXVALUE %s START WITH %s CACHE %s%s',
               seq.seqincrement,
               seq.seqmin,
               seq.seqmax,
               seq.seqstart,
               seq.seqcache,
               CASE WHEN seq.seqcycle THEN ' CYCLE' ELSE ' NO CYCLE' END
           )
           FROM pg_catalog.pg_depend dep
           JOIN pg_catalog.pg_class seqcls ON seqcls.oid = dep.objid AND seqcls.relkind = 'S'
           JOIN pg_catalog.pg_sequence seq ON seq.seqrelid = seqcls.oid
           WHERE dep.refobjid = a.attrelid
             AND dep.refobjsubid = a.attnum
             AND dep.deptype = 'i'
           LIMIT 1
       ), '')
FROM pg_catalog.pg_attribute a
JOIN pg_catalog.pg_type t ON t.oid = a.atttypid
LEFT JOIN pg_catalog.pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
LEFT JOIN pg_catalog.pg_collation coll ON coll.oid = a.attcollation AND a.attcollation <> t.typcollation
LEFT JOIN pg_catalog.pg_namespace cn ON cn.oid = coll.collnamespace
WHERE a.attrelid = $1::oid
  AND a.attnum > 0
  AND NOT a.attisdropped
  AND a.attislocal
ORDER BY a.attnum`, relationOID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]postgresDDLColumn, 0)
	for rows.Next() {
		var column postgresDDLColumn
		if err := rows.Scan(
			&column.name,
			&column.databaseType,
			&column.nullable,
			&column.defaultValue,
			&column.identityKind,
			&column.generatedKind,
			&column.collationSchema,
			&column.collationName,
			&column.identityOptions,
		); err != nil {
			return nil, err
		}
		result = append(result, column)
	}
	return result, rows.Err()
}

func postgresRenderDDLColumn(column postgresDDLColumn, dialect postgresDialect) string {
	definition := dialect.quoteIdentifier(column.name) + " " + column.databaseType
	if column.collationName != "" {
		collation := dialect.quoteIdentifier(column.collationName)
		if column.collationSchema != "" {
			collation = dialect.quoteIdentifier(column.collationSchema) + "." + collation
		}
		definition += " COLLATE " + collation
	}
	if column.generatedKind != "" && column.defaultValue.Valid {
		definition += " GENERATED ALWAYS AS (" + column.defaultValue.String + ")"
		if column.generatedKind == "v" {
			definition += " VIRTUAL"
		} else {
			definition += " STORED"
		}
	} else if column.identityKind != "" {
		if column.identityKind == "a" {
			definition += " GENERATED ALWAYS AS IDENTITY"
		} else {
			definition += " GENERATED BY DEFAULT AS IDENTITY"
		}
		if column.identityOptions != "" {
			definition += " (" + column.identityOptions + ")"
		}
	} else if column.defaultValue.Valid {
		definition += " DEFAULT " + column.defaultValue.String
	}
	if !column.nullable {
		definition += " NOT NULL"
	}
	return definition
}

func postgresLoadDDLConstraints(ctx context.Context, database *sql.DB, relationOID int64) ([]string, error) {
	rows, err := database.QueryContext(ctx, `
SELECT 'CONSTRAINT ' || quote_ident(con.conname) || ' ' || pg_catalog.pg_get_constraintdef(con.oid, true)
FROM pg_catalog.pg_constraint con
WHERE con.conrelid = $1::oid
  AND con.contype NOT IN ('t', 'n')
  AND con.conislocal
ORDER BY con.conname`, relationOID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var definition string
		if err := rows.Scan(&definition); err != nil {
			return nil, err
		}
		result = append(result, definition)
	}
	return result, rows.Err()
}

func postgresLoadInheritanceParents(ctx context.Context, database *sql.DB, relationOID int64) ([]string, error) {
	rows, err := database.QueryContext(ctx, `
SELECT quote_ident(n.nspname) || '.' || quote_ident(c.relname)
FROM pg_catalog.pg_inherits i
JOIN pg_catalog.pg_class c ON c.oid = i.inhparent
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE i.inhrelid = $1::oid
ORDER BY i.inhseqno`, relationOID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var parent string
		if err := rows.Scan(&parent); err != nil {
			return nil, err
		}
		result = append(result, parent)
	}
	return result, rows.Err()
}

func postgresDDLIndexes(ctx context.Context, database *sql.DB, relationOID int64) ([]string, error) {
	rows, err := database.QueryContext(ctx, `
SELECT pg_catalog.pg_get_indexdef(i.indexrelid, 0, true)
FROM pg_catalog.pg_index i
WHERE i.indrelid = $1::oid
  AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_constraint con WHERE con.conindid = i.indexrelid)
ORDER BY i.indexrelid::regclass::text`, relationOID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var definition string
		if err := rows.Scan(&definition); err != nil {
			return nil, err
		}
		definition = strings.TrimSpace(definition)
		if !strings.HasSuffix(definition, ";") {
			definition += ";"
		}
		result = append(result, definition)
	}
	return result, rows.Err()
}

func postgresInt64Pointer(value int64) *int64 {
	result := value
	return &result
}
