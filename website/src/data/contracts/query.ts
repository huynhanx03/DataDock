import type { ApiDataColumn } from './catalog'

export type ApiQueryExecutionResult = {
  executionId: string
  statementType: 'unknown' | 'select' | 'insert' | 'update' | 'delete' | 'merge' | 'ddl' | 'transaction' | 'utility'
  columns: ApiDataColumn[]
  rows: unknown[][]
  durationMs: number
  rowsAffected: number
  truncated: boolean
  limits: {
    maxRows: number
    maxBytes: number
    rowsRead: number
    bytesRead: number
  }
  notices: Array<{
    severity: string
    code?: string
    message: string
    detail?: string
    hint?: string
  }>
  plan?: {
    mode: 'explain' | 'explain-analyze'
    table: {
      columns: ApiDataColumn[]
      rows: unknown[][]
    }
    tree: ApiQueryPlanNode[]
  }
}

export type ApiQueryPlanNode = {
  id: string
  parentId?: string
  operation: string
  relation?: string
  cost?: number
  actualTimeMs?: number
  rows?: number
  loops?: number
  details?: Record<string, unknown>
  children?: ApiQueryPlanNode[]
}

export type ApiQueryHistoryItem = {
  id: string
  connectionId: string
  sql: string
  status: 'success' | 'error' | 'timeout' | 'cancelled'
  durationMs: number
  rowCount: number
  error?: string
  executedAt: string
}
