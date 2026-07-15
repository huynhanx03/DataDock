package dto

type TableMutationKind string

const (
	TableMutationInsert TableMutationKind = "insert"
	TableMutationUpdate TableMutationKind = "update"
	TableMutationDelete TableMutationKind = "delete"
)

type TableMutationBatchStatus string

const (
	TableMutationBatchApplied  TableMutationBatchStatus = "applied"
	TableMutationBatchConflict TableMutationBatchStatus = "conflict"
)

type TableMutationConflictReason string

const (
	TableMutationConflictOptimistic TableMutationConflictReason = "optimistic_conflict"
	TableMutationConflictMissingRow TableMutationConflictReason = "row_not_found"
)

type TableMutation struct {
	Kind           TableMutationKind `json:"kind"`
	Values         map[string]any    `json:"values,omitempty"`
	Keys           map[string]any    `json:"keys,omitempty"`
	ExpectedValues map[string]any    `json:"expectedValues,omitempty"`
}

type TableMutationBatchInput struct {
	ConnectionID  string          `json:"connectionId"`
	Reference     string          `json:"reference"`
	TransactionID string          `json:"transactionId,omitempty"`
	Mutations     []TableMutation `json:"mutations"`
}

type TableMutationConflict struct {
	Index  int                         `json:"index"`
	Kind   TableMutationKind           `json:"kind"`
	Reason TableMutationConflictReason `json:"reason"`
}

type TableMutationBatchResult struct {
	Status    TableMutationBatchStatus `json:"status"`
	Atomic    bool                     `json:"atomic"`
	Applied   int                      `json:"applied"`
	Conflicts []TableMutationConflict  `json:"conflicts"`
}

type TableMutationColumn struct {
	Name      string `json:"name"`
	Writable  bool   `json:"writable"`
	Generated bool   `json:"generated"`
}

type TableMutationMetadata struct {
	Columns           []TableMutationColumn `json:"columns"`
	PrimaryKeyColumns []string              `json:"primaryKeyColumns"`
}
