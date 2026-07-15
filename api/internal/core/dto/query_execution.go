package dto

import "time"

type QueryExecutionStatus string

const (
	QueryExecutionSucceeded QueryExecutionStatus = "success"
	QueryExecutionFailed    QueryExecutionStatus = "error"
	QueryExecutionTimedOut  QueryExecutionStatus = "timeout"
	QueryExecutionCancelled QueryExecutionStatus = "cancelled"
)

type QueryStatementType string

const (
	QueryStatementUnknown     QueryStatementType = "unknown"
	QueryStatementSelect      QueryStatementType = "select"
	QueryStatementInsert      QueryStatementType = "insert"
	QueryStatementUpdate      QueryStatementType = "update"
	QueryStatementDelete      QueryStatementType = "delete"
	QueryStatementMerge       QueryStatementType = "merge"
	QueryStatementDDL         QueryStatementType = "ddl"
	QueryStatementTransaction QueryStatementType = "transaction"
	QueryStatementUtility     QueryStatementType = "utility"
)

type QueryExplainMode string

const (
	QueryExplainOnly    QueryExplainMode = "explain"
	QueryExplainAnalyze QueryExplainMode = "explain-analyze"
)

type QueryExecutionInput struct {
	ExecutionID    string `json:"executionId" binding:"omitempty,max=128"`
	ConnectionID   string `json:"connectionId" binding:"required,max=128"`
	SQL            string `json:"sql" binding:"required,max=100000"`
	TimeoutSeconds int    `json:"timeoutSeconds" binding:"gte=0"`
	TransactionID  string `json:"transactionId" binding:"omitempty,max=128"`
}

type QueryExplainInput struct {
	ExecutionID    string `json:"executionId" binding:"omitempty,max=128"`
	ConnectionID   string `json:"connectionId" binding:"required,max=128"`
	SQL            string `json:"sql" binding:"required,max=100000"`
	TimeoutSeconds int    `json:"timeoutSeconds" binding:"gte=0"`
	TransactionID  string `json:"transactionId" binding:"omitempty,max=128"`
	Analyze        bool   `json:"analyze"`
}

type QueryExecutionLimits struct {
	MaxRows   int   `json:"maxRows"`
	MaxBytes  int64 `json:"maxBytes"`
	RowsRead  int   `json:"rowsRead"`
	BytesRead int64 `json:"bytesRead"`
}

type QueryNotice struct {
	Severity string `json:"severity"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
	Detail   string `json:"detail,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

type QueryPlanTable struct {
	Columns []DataColumn `json:"columns"`
	Rows    [][]any      `json:"rows"`
}

type QueryPlanNode struct {
	ID           string          `json:"id"`
	ParentID     string          `json:"parentId,omitempty"`
	Operation    string          `json:"operation"`
	Relation     string          `json:"relation,omitempty"`
	Cost         float64         `json:"cost,omitempty"`
	ActualTimeMS float64         `json:"actualTimeMs,omitempty"`
	Rows         int64           `json:"rows,omitempty"`
	Loops        int64           `json:"loops,omitempty"`
	Details      map[string]any  `json:"details,omitempty"`
	Children     []QueryPlanNode `json:"children,omitempty"`
}

type QueryExplainPlan struct {
	Mode  QueryExplainMode `json:"mode"`
	Table QueryPlanTable   `json:"table"`
	Tree  []QueryPlanNode  `json:"tree"`
}

type QueryExecutionResult struct {
	ExecutionID   string               `json:"executionId"`
	StatementType QueryStatementType   `json:"statementType"`
	Columns       []DataColumn         `json:"columns"`
	Rows          [][]any              `json:"rows"`
	DurationMS    int64                `json:"durationMs"`
	RowsAffected  int64                `json:"rowsAffected"`
	Truncated     bool                 `json:"truncated"`
	Limits        QueryExecutionLimits `json:"limits"`
	Notices       []QueryNotice        `json:"notices"`
	Plan          *QueryExplainPlan    `json:"plan,omitempty"`
}

type QueryHistoryItem struct {
	ID           string               `json:"id"`
	ConnectionID string               `json:"connectionId"`
	SQL          string               `json:"sql"`
	Status       QueryExecutionStatus `json:"status"`
	DurationMS   int64                `json:"durationMs"`
	RowCount     int64                `json:"rowCount"`
	Error        string               `json:"error,omitempty"`
	ExecutedAt   time.Time            `json:"executedAt"`
}

type QueryHistoryDeleteInput struct {
	IDs []string `json:"ids" binding:"required,min=1,max=100"`
}
