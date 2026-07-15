import type { DatabaseEngine } from './connection'

export type DatabaseObjectKind =
  | 'database'
  | 'schema'
  | 'group'
  | 'table'
  | 'view'
  | 'materialized-view'
  | 'function'
  | 'procedure'
  | 'sequence'
  | 'extension'
  | 'trigger'
  | 'column'

export type DatabaseObject = {
  id: string
  reference?: string
  connectionId: string
  parentId?: string
  name: string
  qualifiedName: string
  kind: DatabaseObjectKind
  database?: string
  schema?: string
  dataType?: string
  count?: number
  childrenState?: 'unloaded' | 'loaded' | 'empty' | 'unsupported'
  capabilities?: string[]
  children?: DatabaseObject[]
}

export type CatalogTree = {
  connectionId: string
  engine: DatabaseEngine
  capabilities?: string[]
  databases: DatabaseObject[]
  nextCursor?: string
  loadedAt: string
}

export type DataColumn = {
  name: string
  key?: string
  type: string
  databaseType?: string
  logicalType?: 'string' | 'boolean' | 'integer' | 'bigint' | 'decimal' | 'float' | 'date' | 'time' | 'datetime' | 'json' | 'binary' | 'uuid' | 'enum' | 'unknown'
  nullable?: boolean
  defaultValue?: string | null
  precision?: number
  scale?: number
  length?: number
  enumValues?: string[]
  identity?: boolean
  generated?: boolean
  primaryKey?: boolean
  valueEncoding?: 'native' | 'decimal-string' | 'json-string' | 'base64' | 'temporal-string'
}

export type TableRow = Record<string, unknown>

export type TableRowsInput = {
  page?: number
  pageSize?: number
  search?: string
  sortBy?: string
  sortDirection?: 'asc' | 'desc'
  columns?: string[]
  filters?: TableFilter[]
  includeTotal?: boolean
}

export type TableFilter = {
  column: string
  operator: 'eq' | 'ne' | 'neq' | 'gt' | 'gte' | 'lt' | 'lte' | 'contains' | 'starts_with' | 'ends_with' | 'is_null' | 'is_not_null' | 'in'
  value?: unknown
}

export type TableRowsResult = {
  columns: DataColumn[]
  rows: TableRow[]
  page: number
  pageSize: number
  total: number
  durationMs: number
  primaryKeyColumns?: string[]
  hasMore?: boolean
  nextPage?: number
  truncated?: boolean
}

export type RowMutation = {
  kind: 'insert' | 'update' | 'delete'
  values?: TableRow
  keys?: TableRow
  expectedValues?: TableRow
}

export type TableMutateResult = {
  applied: number
  status?: 'applied' | 'conflict'
  atomic?: boolean
  conflicts?: Array<{
    index: number
    kind: RowMutation['kind']
    reason: 'optimistic_conflict' | 'row_not_found'
  }>
}

export type TableColumn = {
  name: string
  dataType: string
  nullable: boolean
  defaultValue?: string | null
  comment: string
  databaseType?: string
  precision?: number
  scale?: number
  length?: number
  enumValues?: string[]
  identity?: boolean
  generated?: boolean
  primaryKey?: boolean
}

export type TableIndex = {
  name: string
  unique: boolean
  primary: boolean
  type: string
  definition: string
}

export type TableConstraint = {
  name: string
  type: string
  columns: string[]
  definition: string
}

export type TableSchema = {
  columns: TableColumn[]
  indexes: TableIndex[]
  constraints: TableConstraint[]
}

export type ColumnDefinition = {
  name: string
  dataType: string
  nullable: boolean
  defaultValue?: string | null
  comment: string
}

export type TableAlteration = {
  kind: 'add_column' | 'drop_column' | 'rename_column' | 'change_type' | 'set_nullable' | 'set_default' | 'rename_table'
  column?: string
  newName?: string
  definition?: ColumnDefinition
}

export type CreateTableInput = {
  schema?: string
  name: string
  columns: ColumnDefinition[]
}

export type CreateIndexInput = {
  name: string
  columns: string[]
  unique: boolean
}

export type DatabaseMetric = {
  key: string
  label: string
  value: string | number
  unit?: string
  detail?: string
  trend?: 'up' | 'down' | 'neutral'
  series?: Array<{ timestamp: string; value: number }>
}

export type DatabaseDashboard = {
  available: boolean
  message?: string
  engine: DatabaseEngine
  version?: string
  connectionId?: string
  window?: OperationsWindow
  collectedAt?: string
  metrics: DatabaseMetric[]
}

export type OperationsWindow = '5m' | '15m' | '1h' | '6h' | '24h'

export type DatabaseSession = {
  id: string
  user: string
  database: string
  state: string
  query?: string
  durationMs?: number
  startedAt?: string
  client?: string
  waitEvent?: string
}

export type DatabaseSessions = {
  available: boolean
  message?: string
  connectionId?: string
  engine?: DatabaseEngine
  collectedAt?: string
  items: DatabaseSession[]
}

export type DatabaseLock = {
  id: string
  type: string
  object?: string
  mode?: string
  granted: boolean
  waitingPid?: string
  blockingPid?: string
  query?: string
}

export type DatabaseLocks = {
  available: boolean
  message?: string
  connectionId?: string
  engine?: DatabaseEngine
  collectedAt?: string
  items: DatabaseLock[]
  blockingChains?: Array<{ waitingSessionId: string; blockingSessionIds: string[] }>
}

export type SlowQuery = {
  fingerprint?: string
  query: string
  calls: number
  totalMs: number
  meanMs: number
  rows: number
}

export type DatabasePerformance = {
  available: boolean
  message?: string
  connectionId?: string
  engine?: DatabaseEngine
  window?: OperationsWindow
  collectedAt?: string
  metrics?: DatabaseMetric[]
  slowQueries: SlowQuery[]
}
