import type { AvailableDatabaseEngine } from '../../entities/connection'

export type ApiCatalogObject = {
  id: string
  reference: string
  connectionId: string
  parentId?: string
  name: string
  qualifiedName: string
  kind: 'database' | 'schema' | 'group' | 'table' | 'view' | 'materialized-view' | 'function' | 'procedure' | 'sequence' | 'extension' | 'trigger' | 'column'
  database?: string
  schema?: string
  dataType?: string
  count?: number
  childrenState: 'unloaded' | 'loaded' | 'empty' | 'unsupported'
  capabilities?: string[]
  children?: ApiCatalogObject[]
}

export type ApiCatalogTree = {
  connectionId: string
  engine: AvailableDatabaseEngine
  capabilities: string[]
  databases: ApiCatalogObject[]
  nextCursor?: string
  loadedAt: string
}

export type ApiDataColumn = {
  key: string
  name: string
  type: string
  databaseType: string
  logicalType: 'string' | 'boolean' | 'integer' | 'bigint' | 'decimal' | 'float' | 'date' | 'time' | 'datetime' | 'json' | 'binary' | 'uuid' | 'enum' | 'unknown'
  nullable: boolean
  defaultValue?: string | null
  precision?: number
  scale?: number
  length?: number
  enumValues?: string[]
  identity: boolean
  generated: boolean
  primaryKey: boolean
  valueEncoding: 'native' | 'decimal-string' | 'json-string' | 'base64' | 'temporal-string'
}

export type ApiTableRowsResult = {
  columns: ApiDataColumn[]
  rows: unknown[][]
  primaryKeyColumns: string[]
  total?: number
  limit: number
  offset: number
  hasMore: boolean
  nextOffset?: number
  durationMs: number
  truncated: boolean
}

export type ApiTableSchema = {
  columns: Array<{
    name: string
    dataType: string
    databaseType: string
    nullable: boolean
    defaultValue?: string | null
    comment: string
    precision?: number
    scale?: number
    length?: number
    enumValues?: string[]
    identity: boolean
    generated: boolean
    primaryKey: boolean
  }>
  indexes: Array<{
    name: string
    unique: boolean
    primary: boolean
    type: string
    definition: string
  }>
  constraints: Array<{
    name: string
    type: string
    columns: string[]
    definition: string
  }>
}

export type ApiTableMutationResult = {
  status: 'applied' | 'conflict'
  atomic: boolean
  applied: number
  conflicts: Array<{
    index: number
    kind: 'insert' | 'update' | 'delete'
    reason: 'optimistic_conflict' | 'row_not_found'
  }>
}
