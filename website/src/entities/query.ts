import type { DataColumn, TableRow } from './database-object'

export type QueryStatus = 'success' | 'error' | 'running' | 'cancelled'

export type ExecuteQueryInput = {
  connectionId: string
  sql: string
  timeoutSeconds?: number
  transactionId?: string
}

export type QueryResult = {
  columns: DataColumn[]
  rows: TableRow[]
  rowsAffected: number
  durationMs: number
  message?: string
}

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

export type TransactionAction = 'commit' | 'rollback' | 'savepoint' | 'rollback_to'

export type TransactionState = {
  id: string
  connectionId: string
  state: 'active' | 'committed' | 'rolled-back'
  startedAt: string
  savepoints: string[]
}

export type SavedQuery = {
  id: string
  connectionId?: string
  folder: string
  title: string
  sql: string
  tags: string[]
  shareCode?: string
  createdAt: string
  updatedAt: string
}

export type CreateSavedQueryInput = Pick<SavedQuery, 'connectionId' | 'folder' | 'title' | 'sql' | 'tags'>
