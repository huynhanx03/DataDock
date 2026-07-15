import type { DataColumn, TableRow } from './database-object'

export type QueryStatus = 'success' | 'error' | 'running' | 'cancelled' | 'timeout'

export type ExecuteQueryInput = {
  executionId?: string
  connectionId: string
  sql: string
  timeoutSeconds?: number
  transactionId?: string
}

export type ExplainQueryInput = ExecuteQueryInput & {
  analyze?: boolean
}

export type QueryExecutionLimits = {
  maxRows: number
  maxBytes: number
  rowsRead: number
  bytesRead: number
}

export type QueryNotice = {
  severity: string
  code?: string
  message: string
  detail?: string
  hint?: string
}

export type QueryPlanNode = {
  id: string
  parentId?: string
  operation: string
  relation?: string
  cost?: number
  actualTimeMs?: number
  rows?: number
  loops?: number
  details?: Record<string, unknown>
  children?: QueryPlanNode[]
}

export type QueryExplainPlan = {
  mode: 'explain' | 'explain-analyze'
  columns: DataColumn[]
  rows: TableRow[]
  tree: QueryPlanNode[]
}

export type QueryResult = {
  executionId?: string
  statementType?: 'unknown' | 'select' | 'insert' | 'update' | 'delete' | 'merge' | 'ddl' | 'transaction' | 'utility'
  columns: DataColumn[]
  rows: TableRow[]
  rowsAffected: number
  durationMs: number
  message?: string
  truncated?: boolean
  limits?: QueryExecutionLimits
  notices?: QueryNotice[]
  plan?: QueryExplainPlan
}

export type QueryResultMode = 'table' | 'json' | 'tree' | 'chart' | 'plan'

export type QueryExecutionKind = 'query' | 'selection' | 'explain' | 'explain-analyze'

export type QueryHistoryItem = {
  id: string
  connectionId: string
  sqlText: string
  status: QueryStatus
  durationMs: number
  rowCount: number
  error?: string
  executedAt: string
}

export type TransactionAction = 'commit' | 'rollback' | 'savepoint' | 'rollback_to' | 'release'

export type TransactionState = {
  id: string
  connectionId: string
  state: 'active' | 'committed' | 'rolled-back' | 'expired'
  startedAt: string
  savepoints: string[]
  lastActivityAt?: string
  expiresAt?: string
  completedAt?: string
}

export type SavedQuery = {
  id: string
  connectionId?: string
  folder: string
  title: string
  sql: string
  tags: string[]
  favorite?: boolean
  shareCode?: string
  createdAt: string
  updatedAt: string
}

export type CreateSavedQueryInput = Pick<SavedQuery, 'connectionId' | 'folder' | 'title' | 'sql' | 'tags'>

export type UpdateSavedQueryInput = Partial<Pick<SavedQuery, 'connectionId' | 'folder' | 'title' | 'sql' | 'tags'>> & {
  clearConnection?: boolean
}

export type SavedQueryFolder = {
  id: string
  name: string
  position: number
  queryCount: number
  createdAt: string
  updatedAt: string
}

export type CreateSavedQueryFolderInput = Pick<SavedQueryFolder, 'name'> & Partial<Pick<SavedQueryFolder, 'position'>>
export type UpdateSavedQueryFolderInput = Partial<Pick<SavedQueryFolder, 'name' | 'position'>>

export type SavedQueryShare = {
  savedQueryId: string
  shareCode: string
  createdAt: string
  expiresAt?: string
}

export type PublicSavedQuery = {
  title: string
  sql: string
  tags: string[]
  createdAt: string
  sharedAt: string
  expiresAt?: string
}
