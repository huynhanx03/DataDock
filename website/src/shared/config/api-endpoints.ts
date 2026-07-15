import { APP_CONFIG } from './constants'

const segment = (value: string) => encodeURIComponent(value)
const endpoint = (path: string) => `${APP_CONFIG.api.prefix}${path}`

export const API_ENDPOINTS = Object.freeze({
  workspaces: endpoint('/workspaces'),
  workspace: (id: string) => endpoint(`/workspaces/${segment(id)}`),
  workspaceOrder: endpoint('/workspaces/order'),
  connections: endpoint('/connections'),
  testConnectionDraft: endpoint('/connections/test'),
  connection: (id: string) => endpoint(`/connections/${segment(id)}`),
  duplicateConnection: (id: string) => endpoint(`/connections/${segment(id)}/duplicate`),
  testConnection: (id: string) => endpoint(`/connections/${segment(id)}/test`),
  connectConnection: (id: string) => endpoint(`/connections/${segment(id)}/connect`),
  disconnectConnection: (id: string) => endpoint(`/connections/${segment(id)}/disconnect`),
  connectionStatus: (id: string) => endpoint(`/connections/${segment(id)}/status`),
  connectionFavorite: (id: string) => endpoint(`/connections/${segment(id)}/favorite`),
  catalog: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/catalog`),
  tableRows: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/table/rows`),
  tableMutations: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/table/mutations`),
  tableSchema: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/table/schema`),
  tableDDL: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/table/ddl`),
  schemaPreview: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/schema/preview`),
  schemaApply: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/schema/apply`),
  executeQuery: endpoint('/queries/execute'),
  explainQuery: endpoint('/queries/explain'),
  cancelQuery: (executionId: string) => endpoint(`/queries/${segment(executionId)}/cancel`),
  queryHistory: endpoint('/query-history'),
  deleteQueryHistory: endpoint('/query-history/delete'),
  transactions: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/transactions`),
  transaction: (connectionId: string, transactionId: string) => endpoint(`/connections/${segment(connectionId)}/transactions/${segment(transactionId)}`),
  transactionAction: (connectionId: string, transactionId: string, action: 'commit' | 'rollback') => endpoint(`/connections/${segment(connectionId)}/transactions/${segment(transactionId)}/${action}`),
  transactionSavepoints: (connectionId: string, transactionId: string) => endpoint(`/connections/${segment(connectionId)}/transactions/${segment(transactionId)}/savepoints`),
  transactionSavepointRollback: (connectionId: string, transactionId: string, name: string) => endpoint(`/connections/${segment(connectionId)}/transactions/${segment(transactionId)}/savepoints/${segment(name)}/rollback`),
  transactionSavepoint: (connectionId: string, transactionId: string, name: string) => endpoint(`/connections/${segment(connectionId)}/transactions/${segment(transactionId)}/savepoints/${segment(name)}`),
  dashboard: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/operations/dashboard`),
  sessions: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/operations/sessions`),
  controlSession: (connectionId: string, sessionId: string) => endpoint(`/connections/${segment(connectionId)}/operations/sessions/${segment(sessionId)}/control`),
  locks: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/operations/locks`),
  performance: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/operations/performance`),
  savedQueries: endpoint('/saved-queries'),
  savedQuery: (id: string) => endpoint(`/saved-queries/${segment(id)}`),
  duplicateSavedQuery: (id: string) => endpoint(`/saved-queries/${segment(id)}/duplicate`),
  favoriteSavedQuery: (id: string) => endpoint(`/saved-queries/${segment(id)}/favorite`),
  savedQueryShare: (id: string) => endpoint(`/saved-queries/${segment(id)}/share`),
  sharedQuery: (code: string) => endpoint(`/shared-queries/${segment(code)}`),
  savedQueryFolders: endpoint('/saved-query-folders'),
  savedQueryFolder: (id: string) => endpoint(`/saved-query-folders/${segment(id)}`),
})

export function withSearch(path: string, values: Record<string, string | number | boolean | undefined>) {
  const parameters = new URLSearchParams()
  Object.entries(values).forEach(([key, value]) => {
    if (value !== undefined && value !== '') parameters.set(key, String(value))
  })
  const query = parameters.toString()
  return query ? `${path}?${query}` : path
}

export function buildApiUrl(path: string) {
  return `${APP_CONFIG.api.baseUrl}${path}`
}
