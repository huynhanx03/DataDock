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
  connectionId: string
  parentId?: string
  name: string
  qualifiedName: string
  kind: DatabaseObjectKind
  database?: string
  schema?: string
  dataType?: string
  count?: number
  children?: DatabaseObject[]
}

export type CatalogTree = {
  connectionId: string
  engine: DatabaseEngine
  databases: DatabaseObject[]
  loadedAt: string
}

export type DataColumn = {
  name: string
  key?: string
  type: string
  nullable?: boolean
}

export type TableRow = Record<string, unknown>

export type TableRowsInput = {
  page?: number
  pageSize?: number
  search?: string
  sortBy?: string
  sortDirection?: 'asc' | 'desc'
}

export type TableRowsResult = {
  columns: DataColumn[]
  rows: TableRow[]
  page: number
  pageSize: number
  total: number
  durationMs: number
}

export type RowMutation = {
  kind: 'insert' | 'update' | 'delete'
  values?: TableRow
  keys?: TableRow
}

export type TableMutateResult = {
  applied: number
}

export type TableColumn = {
  name: string
  dataType: string
  nullable: boolean
  defaultValue?: string | null
  comment: string
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
  metrics: DatabaseMetric[]
}

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
  items: DatabaseLock[]
}

export type SlowQuery = {
  query: string
  calls: number
  totalMs: number
  meanMs: number
  rows: number
}

export type DatabasePerformance = {
  available: boolean
  message?: string
  slowQueries: SlowQuery[]
}
