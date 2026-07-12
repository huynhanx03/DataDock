import { APP_CONFIG } from './constants'

const segment = (value: string) => encodeURIComponent(value)
const endpoint = (path: string) => `${APP_CONFIG.api.prefix}${path}`

export const API_ENDPOINTS = Object.freeze({
  workspaces: endpoint('/workspaces'),
  workspace: (id: string) => endpoint(`/workspaces/${segment(id)}`),
  workspaceOrder: endpoint('/workspaces/order'),
  connections: endpoint('/connections'),
  connection: (id: string) => endpoint(`/connections/${segment(id)}`),
  duplicateConnection: (id: string) => endpoint(`/connections/${segment(id)}/duplicate`),
  testConnection: (id: string) => endpoint(`/connections/${segment(id)}/test`),
  tables: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/tables`),
  tableRows: (connectionId: string, table: string) => endpoint(`/connections/${segment(connectionId)}/tables/${segment(table)}/rows`),
  tableMutations: (connectionId: string, table: string) => endpoint(`/connections/${segment(connectionId)}/tables/${segment(table)}/rows/mutate`),
  tableSchema: (connectionId: string, table: string) => endpoint(`/connections/${segment(connectionId)}/tables/${segment(table)}/schema`),
  tableDDL: (connectionId: string, table: string) => endpoint(`/connections/${segment(connectionId)}/tables/${segment(table)}/ddl`),
  schemaTables: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/schema/tables`),
  tableIndexes: (connectionId: string, table: string) => endpoint(`/connections/${segment(connectionId)}/tables/${segment(table)}/indexes`),
  tableIndex: (connectionId: string, table: string, index: string) => endpoint(`/connections/${segment(connectionId)}/tables/${segment(table)}/indexes/${segment(index)}`),
  executeQuery: endpoint('/queries/execute'),
  beginTransaction: endpoint('/transactions/begin'),
  transactionAction: (transactionId: string, action: string) => endpoint(`/transactions/${segment(transactionId)}/${segment(action)}`),
  dashboard: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/dashboard`),
  sessions: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/sessions`),
  cancelSession: (connectionId: string, sessionId: string) => endpoint(`/connections/${segment(connectionId)}/sessions/${segment(sessionId)}/cancel`),
  locks: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/locks`),
  performance: (connectionId: string) => endpoint(`/connections/${segment(connectionId)}/performance`),
  savedQueries: endpoint('/saved-queries'),
  savedQuery: (id: string) => endpoint(`/saved-queries/${segment(id)}`),
})

export function buildApiUrl(path: string) {
  return `${APP_CONFIG.api.baseUrl}${path}`
}
