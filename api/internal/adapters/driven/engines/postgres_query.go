package engines

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
)

func explainPostgres(ctx context.Context, executor queryExecutor, sqlText string, mode dto.QueryExplainMode) (dto.QueryExplainPlan, error) {
	options := "FORMAT JSON, VERBOSE TRUE, COSTS TRUE"
	if mode == dto.QueryExplainAnalyze {
		options += ", ANALYZE TRUE, BUFFERS TRUE, TIMING TRUE, SUMMARY TRUE"
	}
	var raw []byte
	if err := executor.QueryRowContext(ctx, "EXPLAIN ("+options+") "+sqlText).Scan(&raw); err != nil {
		return dto.QueryExplainPlan{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var payload []map[string]any
	if err := decoder.Decode(&payload); err != nil || len(payload) == 0 {
		return dto.QueryExplainPlan{}, apperror.NewInternal("PostgreSQL returned an invalid query plan", err)
	}
	root, ok := payload[0]["Plan"].(map[string]any)
	if !ok {
		return dto.QueryExplainPlan{}, apperror.NewInternal("PostgreSQL query plan has no root node", nil)
	}
	sequence := 0
	rows := make([][]any, 0)
	tree := []dto.QueryPlanNode{postgresPlanNode(root, "", &sequence, &rows)}
	return dto.QueryExplainPlan{Mode: mode, Table: dto.QueryPlanTable{Columns: queryPlanColumns(), Rows: rows}, Tree: tree}, nil
}

func postgresPlanNode(value map[string]any, parentID string, sequence *int, rows *[][]any) dto.QueryPlanNode {
	(*sequence)++
	id := "node-" + strconv.Itoa(*sequence)
	operation := planString(value["Node Type"])
	relation := planString(value["Relation Name"])
	if schema := planString(value["Schema"]); schema != "" && relation != "" {
		relation = schema + "." + relation
	}
	totalCost := planFloat(value["Total Cost"])
	actualTime := planFloat(value["Actual Total Time"])
	planRows := planInt(value["Plan Rows"])
	actualRows := planInt(value["Actual Rows"])
	loops := planInt(value["Actual Loops"])
	rowsValue := planRows
	if _, present := value["Actual Rows"]; present {
		rowsValue = actualRows
	}
	details := make(map[string]any)
	for key, item := range value {
		if key == "Plans" || key == "Node Type" || key == "Relation Name" || key == "Schema" || key == "Total Cost" || key == "Actual Total Time" || key == "Plan Rows" || key == "Actual Rows" || key == "Actual Loops" {
			continue
		}
		details[lowerCamelPlanKey(key)] = item
	}
	node := dto.QueryPlanNode{ID: id, ParentID: parentID, Operation: operation, Relation: relation, Cost: totalCost, ActualTimeMS: actualTime, Rows: rowsValue, Loops: loops, Details: details, Children: []dto.QueryPlanNode{}}
	*rows = append(*rows, queryPlanRow(operation, relation, planFloat(value["Startup Cost"]), totalCost, planRows, actualTime, actualRows, loops))
	if children, ok := value["Plans"].([]any); ok {
		for _, child := range children {
			if childMap, ok := child.(map[string]any); ok {
				node.Children = append(node.Children, postgresPlanNode(childMap, id, sequence, rows))
			}
		}
	}
	return node
}

func queryPlanColumns() []dto.DataColumn {
	return []dto.DataColumn{
		queryPlanColumn("operation", "Operation", dto.LogicalTypeString),
		queryPlanColumn("relation", "Relation", dto.LogicalTypeString),
		queryPlanColumn("startupCost", "Startup cost", dto.LogicalTypeFloat),
		queryPlanColumn("totalCost", "Total cost", dto.LogicalTypeFloat),
		queryPlanColumn("plannedRows", "Planned rows", dto.LogicalTypeBigInt),
		queryPlanColumn("actualTimeMs", "Actual time", dto.LogicalTypeFloat),
		queryPlanColumn("actualRows", "Actual rows", dto.LogicalTypeBigInt),
		queryPlanColumn("loops", "Loops", dto.LogicalTypeBigInt),
	}
}

func queryPlanColumn(key, name string, logicalType dto.LogicalType) dto.DataColumn {
	return dto.DataColumn{Key: key, Name: name, Type: string(logicalType), DatabaseType: string(logicalType), LogicalType: logicalType, Nullable: true, ValueEncoding: valueEncodingForLogicalType(logicalType)}
}

func planString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func planFloat(value any) float64 {
	switch typed := value.(type) {
	case json.Number:
		result, _ := typed.Float64()
		return result
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int64:
		return float64(typed)
	case int:
		return float64(typed)
	default:
		result, _ := strconv.ParseFloat(fmt.Sprint(value), 64)
		return result
	}
}

func planInt(value any) int64 {
	switch typed := value.(type) {
	case json.Number:
		if result, err := typed.Int64(); err == nil {
			return result
		}
		result, _ := typed.Float64()
		return int64(result)
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		result, _ := strconv.ParseInt(fmt.Sprint(value), 10, 64)
		return result
	}
}

func lowerCamelPlanKey(value string) string {
	words := strings.Fields(strings.ToLower(value))
	if len(words) == 0 {
		return "detail"
	}
	result := words[0]
	for _, word := range words[1:] {
		if word != "" {
			result += strings.ToUpper(word[:1]) + word[1:]
		}
	}
	return result
}
