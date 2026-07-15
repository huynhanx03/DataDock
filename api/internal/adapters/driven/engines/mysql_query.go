package engines

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

var mysqlTreeCostPattern = regexp.MustCompile(`cost=([0-9eE+.-]+)\.\.([0-9eE+.-]+)\s+rows=([0-9]+)`)
var mysqlTreeActualPattern = regexp.MustCompile(`actual time=([0-9eE+.-]+)\.\.([0-9eE+.-]+)\s+rows=([0-9]+)\s+loops=([0-9]+)`)
var mysqlTreeRelationPattern = regexp.MustCompile(`(?i)\bon\s+([^\s(]+)`)

func explainMySQL(ctx context.Context, executor queryExecutor, engine entity.Engine, sqlText string, mode dto.QueryExplainMode) (dto.QueryExplainPlan, error) {
	if mode == dto.QueryExplainAnalyze && engine == entity.EngineMySQL {
		return explainMySQLTree(ctx, executor, sqlText, mode)
	}
	prefix := "EXPLAIN FORMAT=JSON "
	if mode == dto.QueryExplainAnalyze {
		prefix = "ANALYZE FORMAT=JSON "
	}
	var raw []byte
	if err := executor.QueryRowContext(ctx, prefix+sqlText).Scan(&raw); err != nil {
		return dto.QueryExplainPlan{}, err
	}
	return parseMySQLJSONPlan(raw, mode)
}

func parseMySQLJSONPlan(raw []byte, mode dto.QueryExplainMode) (dto.QueryExplainPlan, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var payload any
	if err := decoder.Decode(&payload); err != nil {
		return dto.QueryExplainPlan{}, apperror.NewInternal("MySQL returned an invalid query plan", err)
	}
	sequence := 0
	rows := make([][]any, 0)
	tree := mysqlJSONPlanNodes(payload, "", &sequence, &rows)
	if len(tree) == 0 {
		return dto.QueryExplainPlan{}, apperror.NewInternal("MySQL query plan has no nodes", nil)
	}
	return dto.QueryExplainPlan{Mode: mode, Table: dto.QueryPlanTable{Columns: queryPlanColumns(), Rows: rows}, Tree: tree}, nil
}

func mysqlJSONPlanNodes(value any, parentID string, sequence *int, rows *[][]any) []dto.QueryPlanNode {
	switch typed := value.(type) {
	case []any:
		result := make([]dto.QueryPlanNode, 0)
		for _, item := range typed {
			result = append(result, mysqlJSONPlanNodes(item, parentID, sequence, rows)...)
		}
		return result
	case map[string]any:
		if table, ok := typed["table"].(map[string]any); ok {
			return []dto.QueryPlanNode{mysqlJSONTableNode(table, parentID, sequence, rows)}
		}
		wrappers := []struct {
			key       string
			operation string
		}{
			{key: "query_block", operation: "Query Block"},
			{key: "union_result", operation: "Union"},
			{key: "duplicates_removal", operation: "Duplicate Removal"},
			{key: "grouping_operation", operation: "Grouping"},
			{key: "ordering_operation", operation: "Ordering"},
			{key: "buffer_result", operation: "Buffer Result"},
			{key: "windowing", operation: "Window"},
			{key: "materialized_from_subquery", operation: "Materialized Subquery"},
			{key: "attached_subqueries", operation: "Attached Subqueries"},
		}
		for _, wrapper := range wrappers {
			if child, present := typed[wrapper.key]; present {
				return []dto.QueryPlanNode{mysqlJSONWrapperNode(wrapper.operation, child, typed, parentID, sequence, rows)}
			}
		}
		if nested, present := typed["nested_loop"]; present {
			return mysqlJSONPlanNodes(nested, parentID, sequence, rows)
		}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		result := make([]dto.QueryPlanNode, 0)
		for _, key := range keys {
			if child, ok := typed[key].(map[string]any); ok {
				result = append(result, mysqlJSONPlanNodes(child, parentID, sequence, rows)...)
			} else if child, ok := typed[key].([]any); ok {
				result = append(result, mysqlJSONPlanNodes(child, parentID, sequence, rows)...)
			}
		}
		return result
	default:
		return nil
	}
}

func mysqlJSONWrapperNode(operation string, child any, metadata map[string]any, parentID string, sequence *int, rows *[][]any) dto.QueryPlanNode {
	id := nextPlanNodeID(sequence)
	cost := mysqlJSONCost(metadata)
	rowCount := mysqlJSONRows(metadata)
	node := dto.QueryPlanNode{ID: id, ParentID: parentID, Operation: operation, Cost: cost, Rows: rowCount, Details: mysqlJSONDetails(metadata), Children: []dto.QueryPlanNode{}}
	*rows = append(*rows, queryPlanRow(operation, "", 0, cost, rowCount, 0, 0, 0))
	node.Children = mysqlJSONPlanNodes(child, id, sequence, rows)
	return node
}

func mysqlJSONTableNode(table map[string]any, parentID string, sequence *int, rows *[][]any) dto.QueryPlanNode {
	id := nextPlanNodeID(sequence)
	relation := planString(table["table_name"])
	access := strings.ToUpper(planString(table["access_type"]))
	operation := "Table"
	if access != "" {
		operation += " " + access
	}
	cost := mysqlJSONCost(table)
	rowCount := mysqlJSONRows(table)
	actualTime := planFloat(table["r_total_time_ms"])
	actualRows := planInt(table["r_rows"])
	loops := planInt(table["r_loops"])
	node := dto.QueryPlanNode{ID: id, ParentID: parentID, Operation: operation, Relation: relation, Cost: cost, ActualTimeMS: actualTime, Rows: rowCount, Loops: loops, Details: mysqlJSONDetails(table), Children: []dto.QueryPlanNode{}}
	*rows = append(*rows, queryPlanRow(operation, relation, 0, cost, rowCount, actualTime, actualRows, loops))
	childKeys := []string{"materialized_from_subquery", "attached_subqueries", "nested_loop"}
	for _, key := range childKeys {
		if child, present := table[key]; present {
			node.Children = append(node.Children, mysqlJSONPlanNodes(child, id, sequence, rows)...)
		}
	}
	return node
}

func mysqlJSONCost(value map[string]any) float64 {
	if costInfo, ok := value["cost_info"].(map[string]any); ok {
		for _, key := range []string{"query_cost", "prefix_cost", "read_cost", "eval_cost"} {
			if item, present := costInfo[key]; present {
				return planFloat(item)
			}
		}
	}
	for _, key := range []string{"r_total_time_ms", "cost"} {
		if item, present := value[key]; present {
			return planFloat(item)
		}
	}
	return 0
}

func mysqlJSONRows(value map[string]any) int64 {
	for _, key := range []string{"r_rows", "rows_produced_per_join", "rows_examined_per_scan", "rows"} {
		if item, present := value[key]; present {
			return planInt(item)
		}
	}
	return 0
}

func mysqlJSONDetails(value map[string]any) map[string]any {
	details := make(map[string]any)
	for key, item := range value {
		switch key {
		case "table", "nested_loop", "query_block", "union_result", "duplicates_removal", "grouping_operation", "ordering_operation", "buffer_result", "windowing", "materialized_from_subquery", "attached_subqueries", "cost_info", "table_name", "access_type", "rows", "r_rows", "r_loops", "r_total_time_ms", "rows_produced_per_join", "rows_examined_per_scan":
			continue
		default:
			details[lowerCamelPlanKey(key)] = item
		}
	}
	return details
}

func explainMySQLTree(ctx context.Context, executor queryExecutor, sqlText string, mode dto.QueryExplainMode) (dto.QueryExplainPlan, error) {
	rows, err := executor.QueryContext(ctx, "EXPLAIN ANALYZE "+sqlText)
	if err != nil {
		return dto.QueryExplainPlan{}, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return dto.QueryExplainPlan{}, err
	}
	lines := make([]string, 0)
	for rows.Next() {
		values := make([]sql.RawBytes, len(columns))
		destinations := make([]any, len(columns))
		for index := range values {
			destinations[index] = &values[index]
		}
		if err := rows.Scan(destinations...); err != nil {
			return dto.QueryExplainPlan{}, err
		}
		for _, value := range values {
			for _, line := range strings.Split(string(value), "\n") {
				if strings.TrimSpace(line) != "" {
					lines = append(lines, line)
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return dto.QueryExplainPlan{}, err
	}
	tree, tableRows := parseMySQLTreeLines(lines)
	if len(tree) == 0 {
		return dto.QueryExplainPlan{}, apperror.NewInternal("MySQL query plan has no nodes", nil)
	}
	return dto.QueryExplainPlan{Mode: mode, Table: dto.QueryPlanTable{Columns: queryPlanColumns(), Rows: tableRows}, Tree: tree}, nil
}

type mysqlTemporaryPlanNode struct {
	indent   int
	node     dto.QueryPlanNode
	children []*mysqlTemporaryPlanNode
}

func parseMySQLTreeLines(lines []string) ([]dto.QueryPlanNode, [][]any) {
	sequence := 0
	stack := make([]*mysqlTemporaryPlanNode, 0)
	roots := make([]*mysqlTemporaryPlanNode, 0)
	rows := make([][]any, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "->"))
		if trimmed == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		operation := mysqlTreeOperation(trimmed)
		relation := ""
		if match := mysqlTreeRelationPattern.FindStringSubmatch(operation); len(match) > 1 {
			relation = match[1]
		}
		startupCost, totalCost, plannedRows := 0.0, 0.0, int64(0)
		if match := mysqlTreeCostPattern.FindStringSubmatch(trimmed); len(match) == 4 {
			startupCost, _ = strconv.ParseFloat(match[1], 64)
			totalCost, _ = strconv.ParseFloat(match[2], 64)
			plannedRows, _ = strconv.ParseInt(match[3], 10, 64)
		}
		actualTime, actualRows, loops := 0.0, int64(0), int64(0)
		if match := mysqlTreeActualPattern.FindStringSubmatch(trimmed); len(match) == 5 {
			actualTime, _ = strconv.ParseFloat(match[2], 64)
			actualRows, _ = strconv.ParseInt(match[3], 10, 64)
			loops, _ = strconv.ParseInt(match[4], 10, 64)
		}
		sequence++
		temporary := &mysqlTemporaryPlanNode{indent: indent, node: dto.QueryPlanNode{ID: "node-" + strconv.Itoa(sequence), Operation: operation, Relation: relation, Cost: totalCost, ActualTimeMS: actualTime, Rows: actualRows, Loops: loops, Details: map[string]any{"source": trimmed}, Children: []dto.QueryPlanNode{}}}
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			roots = append(roots, temporary)
		} else {
			parent := stack[len(stack)-1]
			temporary.node.ParentID = parent.node.ID
			parent.children = append(parent.children, temporary)
		}
		stack = append(stack, temporary)
		rows = append(rows, queryPlanRow(operation, relation, startupCost, totalCost, plannedRows, actualTime, actualRows, loops))
	}
	result := make([]dto.QueryPlanNode, len(roots))
	for index, root := range roots {
		result[index] = materializeMySQLPlanNode(root)
	}
	return result, rows
}

func materializeMySQLPlanNode(value *mysqlTemporaryPlanNode) dto.QueryPlanNode {
	value.node.Children = make([]dto.QueryPlanNode, len(value.children))
	for index, child := range value.children {
		value.node.Children[index] = materializeMySQLPlanNode(child)
	}
	return value.node
}

func mysqlTreeOperation(value string) string {
	if index := strings.Index(value, "  (cost="); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	if index := strings.Index(value, " (cost="); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	if index := strings.Index(value, "  (actual time="); index >= 0 {
		return strings.TrimSpace(value[:index])
	}
	return value
}

func nextPlanNodeID(sequence *int) string {
	(*sequence)++
	return "node-" + strconv.Itoa(*sequence)
}

func queryPlanRow(operation, relation string, startupCost, totalCost float64, plannedRows int64, actualTime float64, actualRows, loops int64) []any {
	return []any{operation, relation, startupCost, totalCost, strconv.FormatInt(plannedRows, 10), actualTime, strconv.FormatInt(actualRows, 10), strconv.FormatInt(loops, 10)}
}
