package dto

type ExecuteQueryInput struct {
	ConnectionID   string `json:"connectionId" binding:"required"`
	SQL            string `json:"sql" binding:"required,max=100000"`
	TimeoutSeconds int    `json:"timeoutSeconds" binding:"gte=0,lte=60"`
	TransactionID  string `json:"transactionId" binding:"omitempty,max=64"`
}

type TransactionInput struct {
	ConnectionID  string `json:"connectionId" binding:"required"`
	TransactionID string `json:"transactionId" binding:"omitempty,max=64"`
	Name          string `json:"name" binding:"omitempty,max=64"`
}
type TransactionState struct {
	ID           string   `json:"id"`
	ConnectionID string   `json:"connectionId"`
	State        string   `json:"state"`
	StartedAt    string   `json:"startedAt"`
	Savepoints   []string `json:"savepoints"`
}
type SessionActionInput struct {
	ConnectionID string `json:"connectionId" binding:"required"`
	SessionID    string `json:"sessionId" binding:"required,max=64"`
	Force        bool   `json:"force"`
}
type SavedQueryInput struct {
	ConnectionID *string  `json:"connectionId"`
	Folder       string   `json:"folder" binding:"max=128"`
	Title        string   `json:"title" binding:"required,max=200"`
	SQL          string   `json:"sql" binding:"required,max=100000"`
	Tags         []string `json:"tags" binding:"max=30,dive,max=64"`
}
type SavedQuery struct {
	ID           string   `json:"id"`
	ConnectionID *string  `json:"connectionId,omitempty"`
	Folder       string   `json:"folder"`
	Title        string   `json:"title"`
	SQL          string   `json:"sql"`
	Tags         []string `json:"tags"`
	ShareCode    string   `json:"shareCode,omitempty"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
}

type QueryResult struct {
	Columns      []string `json:"columns"`
	Rows         [][]any  `json:"rows"`
	RowsAffected int64    `json:"rowsAffected"`
	DurationMs   int64    `json:"durationMs"`
}

type Metric struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value any    `json:"value"`
	Unit  string `json:"unit,omitempty"`
}

type DatabaseDashboard struct {
	Available bool     `json:"available"`
	Message   string   `json:"message,omitempty"`
	Engine    string   `json:"engine"`
	Version   string   `json:"version,omitempty"`
	Metrics   []Metric `json:"metrics"`
}

type DatabaseSession struct {
	ID         string `json:"id"`
	User       string `json:"user"`
	Database   string `json:"database"`
	State      string `json:"state"`
	Query      string `json:"query,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	Client     string `json:"client,omitempty"`
	WaitEvent  string `json:"waitEvent,omitempty"`
}

type DatabaseSessions struct {
	Available bool              `json:"available"`
	Message   string            `json:"message,omitempty"`
	Items     []DatabaseSession `json:"items"`
}

type DatabaseLock struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Object      string `json:"object,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Granted     bool   `json:"granted"`
	WaitingPID  string `json:"waitingPid,omitempty"`
	BlockingPID string `json:"blockingPid,omitempty"`
	Query       string `json:"query,omitempty"`
}

type DatabaseLocks struct {
	Available bool           `json:"available"`
	Message   string         `json:"message,omitempty"`
	Items     []DatabaseLock `json:"items"`
}

type SlowQuery struct {
	Query   string  `json:"query"`
	Calls   int64   `json:"calls"`
	TotalMs float64 `json:"totalMs"`
	MeanMs  float64 `json:"meanMs"`
	Rows    int64   `json:"rows"`
}

type DatabasePerformance struct {
	Available   bool        `json:"available"`
	Message     string      `json:"message,omitempty"`
	SlowQueries []SlowQuery `json:"slowQueries"`
}

type TableRowsInput struct {
	ConnectionID string        `json:"connectionId,omitempty"`
	Reference    string        `json:"reference"`
	Table        string        `json:"table,omitempty"`
	Limit        int           `json:"limit"`
	Offset       int           `json:"offset"`
	Search       string        `json:"search,omitempty"`
	Columns      []string      `json:"columns,omitempty"`
	Sorts        []TableSort   `json:"sorts,omitempty"`
	Filters      []TableFilter `json:"filters,omitempty"`
	IncludeTotal bool          `json:"includeTotal"`
	Sort         string        `json:"sort,omitempty"`
	Order        string        `json:"order,omitempty"`
}

type TableRowsResult struct {
	Columns           []DataColumn `json:"columns"`
	Rows              [][]any      `json:"rows"`
	PrimaryKeyColumns []string     `json:"primaryKeyColumns"`
	Total             *int64       `json:"total,omitempty"`
	Limit             int          `json:"limit"`
	Offset            int          `json:"offset"`
	HasMore           bool         `json:"hasMore"`
	NextOffset        int          `json:"nextOffset,omitempty"`
	DurationMS        int64        `json:"durationMs"`
	Truncated         bool         `json:"truncated"`
}

type TableSort struct {
	Column    string `json:"column"`
	Direction string `json:"direction"`
}

type TableFilter struct {
	Column   string `json:"column"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

type LogicalType string

const (
	LogicalTypeString   LogicalType = "string"
	LogicalTypeBoolean  LogicalType = "boolean"
	LogicalTypeInteger  LogicalType = "integer"
	LogicalTypeBigInt   LogicalType = "bigint"
	LogicalTypeDecimal  LogicalType = "decimal"
	LogicalTypeFloat    LogicalType = "float"
	LogicalTypeDate     LogicalType = "date"
	LogicalTypeTime     LogicalType = "time"
	LogicalTypeDateTime LogicalType = "datetime"
	LogicalTypeJSON     LogicalType = "json"
	LogicalTypeBinary   LogicalType = "binary"
	LogicalTypeUUID     LogicalType = "uuid"
	LogicalTypeEnum     LogicalType = "enum"
	LogicalTypeUnknown  LogicalType = "unknown"
)

type ValueEncoding string

const (
	ValueEncodingNative   ValueEncoding = "native"
	ValueEncodingDecimal  ValueEncoding = "decimal-string"
	ValueEncodingJSON     ValueEncoding = "json-string"
	ValueEncodingBase64   ValueEncoding = "base64"
	ValueEncodingTemporal ValueEncoding = "temporal-string"
)

type DataColumn struct {
	Key           string        `json:"key"`
	Name          string        `json:"name"`
	Type          string        `json:"type"`
	DatabaseType  string        `json:"databaseType"`
	LogicalType   LogicalType   `json:"logicalType"`
	Nullable      bool          `json:"nullable"`
	DefaultValue  *string       `json:"defaultValue,omitempty"`
	Precision     *int64        `json:"precision,omitempty"`
	Scale         *int64        `json:"scale,omitempty"`
	Length        *int64        `json:"length,omitempty"`
	EnumValues    []string      `json:"enumValues,omitempty"`
	Identity      bool          `json:"identity"`
	Generated     bool          `json:"generated"`
	PrimaryKey    bool          `json:"primaryKey"`
	ValueEncoding ValueEncoding `json:"valueEncoding"`
}

type RowMutation struct {
	Kind   string         `json:"kind" binding:"required,oneof=insert update delete"`
	Values map[string]any `json:"values"`
	Keys   map[string]any `json:"keys"`
}

type TableMutateInput struct {
	Mutations []RowMutation `json:"mutations" binding:"required,min=1,max=100"`
}

type TableMutateResult struct {
	Applied int64 `json:"applied"`
}

type TableSchema struct {
	Columns     []TableColumn     `json:"columns"`
	Indexes     []TableIndex      `json:"indexes"`
	Constraints []TableConstraint `json:"constraints"`
}

type TableColumn struct {
	Name         string   `json:"name"`
	DataType     string   `json:"dataType"`
	DatabaseType string   `json:"databaseType"`
	Nullable     bool     `json:"nullable"`
	DefaultValue *string  `json:"defaultValue"`
	Comment      string   `json:"comment"`
	Precision    *int64   `json:"precision,omitempty"`
	Scale        *int64   `json:"scale,omitempty"`
	Length       *int64   `json:"length,omitempty"`
	EnumValues   []string `json:"enumValues,omitempty"`
	Identity     bool     `json:"identity"`
	Generated    bool     `json:"generated"`
	PrimaryKey   bool     `json:"primaryKey"`
}

type TableIndex struct {
	Name       string `json:"name"`
	Unique     bool   `json:"unique"`
	Primary    bool   `json:"primary"`
	Type       string `json:"type"`
	Definition string `json:"definition"`
}

type TableConstraint struct {
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Columns    []string `json:"columns"`
	Definition string   `json:"definition"`
}

type ColumnDefinition struct {
	Name         string  `json:"name" binding:"required,max=128"`
	DataType     string  `json:"dataType" binding:"required,max=128"`
	Nullable     bool    `json:"nullable"`
	DefaultValue *string `json:"defaultValue" binding:"omitempty,max=500"`
	Comment      string  `json:"comment" binding:"max=500"`
}

type CreateTableInput struct {
	Schema  string             `json:"schema" binding:"omitempty,max=128"`
	Name    string             `json:"name" binding:"required,max=128"`
	Columns []ColumnDefinition `json:"columns" binding:"required,min=1,max=200"`
}

type TableAlteration struct {
	Kind       string            `json:"kind" binding:"required,max=32"`
	Column     string            `json:"column" binding:"omitempty,max=128"`
	NewName    string            `json:"newName" binding:"omitempty,max=128"`
	Definition *ColumnDefinition `json:"definition"`
}

type AlterTableInput struct {
	Actions []TableAlteration `json:"actions" binding:"required,min=1,max=50"`
}

type CreateIndexInput struct {
	Name    string   `json:"name" binding:"required,max=128"`
	Columns []string `json:"columns" binding:"required,min=1,max=32,dive,max=128"`
	Unique  bool     `json:"unique"`
}
