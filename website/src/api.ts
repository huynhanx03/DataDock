export type DatabaseEngine = 'postgresql' | 'mysql' | 'mariadb'

export type Workspace = { id: string; name: string; icon: string; color: string; position: number; collapsed: boolean; createdAt: string; updatedAt: string }
export type SSHTunnel = { enabled: boolean; host: string; port: number; username: string; privateKeyPath: string; knownHostsPath: string; hasPassword?: boolean }
export type Connection = { id: string; workspaceId: string; name: string; engine: DatabaseEngine; host: string; port: number; database: string; username: string; sslMode: 'disable' | 'require' | 'verify-ca' | 'verify-full'; sslCaPath: string; sslCertPath: string; sslKeyPath: string; proxyUrl: string; sshTunnel: SSHTunnel; readOnly: boolean; autoReconnect: boolean; maxOpenConns: number; maxIdleConns: number; connMaxLifetimeSeconds: number; favorite: boolean; hasPassword: boolean; createdAt: string; updatedAt: string }
export type CreateConnectionInput = Omit<Connection, 'id' | 'favorite' | 'hasPassword' | 'createdAt' | 'updatedAt' | 'sshTunnel'> & { password: string; sshTunnel: SSHTunnel & { password: string } }
export type QueryColumn = { name: string; type?: string }
export type QueryResult = { columns: QueryColumn[]; rows: unknown[][]; rowsAffected?: number; durationMs?: number; message?: string }
export type QueryHistoryItem = { id: string; sqlText: string; status: 'success' | 'error' | 'running'; durationMs: number; rowCount: number; executedAt: string }
export type TableRowsResult = { columns: QueryColumn[]; rows: unknown[][]; page?: number; pageSize?: number; limit?: number; offset?: number; total: number }
export type TableRowsInput = { page?: number; pageSize?: number; search?: string; sortBy?: string; sortDirection?: 'asc' | 'desc' }
export type RowMutation = { kind: 'insert' | 'update' | 'delete'; values?: Record<string, unknown>; keys?: Record<string, unknown> }
export type TableMutateResult = { applied: number }
export type TableColumn = { name: string; dataType: string; nullable: boolean; defaultValue?: string | null; comment: string }
export type TableIndex = { name: string; unique: boolean; primary: boolean; type: string; definition: string }
export type TableConstraint = { name: string; type: string; columns: string[]; definition: string }
export type TableSchema = { columns: TableColumn[]; indexes: TableIndex[]; constraints: TableConstraint[] }
export type ColumnDefinition = { name: string; dataType: string; nullable: boolean; defaultValue?: string | null; comment: string }
export type TableAlteration = { kind: 'add_column' | 'drop_column' | 'rename_column' | 'change_type' | 'set_nullable' | 'set_default' | 'rename_table'; column?: string; newName?: string; definition?: ColumnDefinition }
export type OperationsMetric = { label: string; value: string | number; detail?: string; trend?: 'up' | 'down' | 'neutral' }
export type OperationsSnapshot = { metrics?: OperationsMetric[]; data?: Record<string, unknown>; message?: string }
export type TransactionState = { id: string; connectionId: string; state: string; startedAt: string; savepoints: string[] }
export type SavedQuery = { id:string; connectionId?:string; folder:string; title:string; sql:string; tags:string[]; createdAt:string; updatedAt:string }

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`/api/v1${path}`, { ...init, headers: { 'Content-Type': 'application/json', ...init?.headers } })
  if (!response.ok) {
    const body = await response.json().catch(() => ({})) as { error?: { message?: string } }
    throw new Error(body.error?.message || `Request failed (${response.status})`)
  }
  if (response.status === 204) return undefined as T
  return (await response.json() as { data: T }).data
}

export const datadockApi = {
  listWorkspaces: () => request<Workspace[]>('/workspaces'),
  createWorkspace: (input: Pick<Workspace, 'name' | 'icon' | 'color'>) => request<Workspace>('/workspaces', { method: 'POST', body: JSON.stringify(input) }),
  listConnections: (workspaceId: string) => request<Connection[]>(`/connections?workspaceId=${encodeURIComponent(workspaceId)}`),
  createConnection: (input: CreateConnectionInput) => request<Connection>('/connections', { method: 'POST', body: JSON.stringify(input) }),
  getTableRows: (connectionId: string, table: string, input: TableRowsInput = {}) => {
    const params = new URLSearchParams()
    const limit = input.pageSize || 50
    params.set('limit', String(limit))
    params.set('offset', String(Math.max(0, ((input.page || 1) - 1) * limit)))
    if (input.search) params.set('search', input.search)
    if (input.sortBy) params.set('sort', input.sortBy)
    if (input.sortDirection) params.set('order', input.sortDirection)
    const query = params.toString()
    return request<TableRowsResult>(`/connections/${encodeURIComponent(connectionId)}/tables/${encodeURIComponent(table)}/rows${query ? `?${query}` : ''}`)
  },
  getTableSchema: (connectionId: string, table: string) => request<TableSchema>(`/connections/${encodeURIComponent(connectionId)}/tables/${encodeURIComponent(table)}/schema`),
  getTableDDL: (connectionId: string, table: string) => request<{ ddl: string }>(`/connections/${encodeURIComponent(connectionId)}/tables/${encodeURIComponent(table)}/ddl`),
  mutateTableRows: (connectionId: string, table: string, mutations: RowMutation[]) => request<TableMutateResult>(`/connections/${encodeURIComponent(connectionId)}/tables/${encodeURIComponent(table)}/rows/mutate`, { method: 'POST', body: JSON.stringify({ mutations }) }),
  createTable: (connectionId: string, input: { schema?: string; name: string; columns: ColumnDefinition[] }) => request<void>(`/connections/${encodeURIComponent(connectionId)}/schema/tables`, { method: 'POST', body: JSON.stringify(input) }),
  alterTable: (connectionId: string, table: string, actions: TableAlteration[]) => request<void>(`/connections/${encodeURIComponent(connectionId)}/tables/${encodeURIComponent(table)}/schema`, { method: 'PATCH', body: JSON.stringify({ actions }) }),
  createIndex: (connectionId: string, table: string, input: { name: string; columns: string[]; unique: boolean }) => request<void>(`/connections/${encodeURIComponent(connectionId)}/tables/${encodeURIComponent(table)}/indexes`, { method: 'POST', body: JSON.stringify(input) }),
  dropIndex: (connectionId: string, table: string, index: string) => request<void>(`/connections/${encodeURIComponent(connectionId)}/tables/${encodeURIComponent(table)}/indexes/${encodeURIComponent(index)}`, { method: 'DELETE' }),
  executeQuery: (input: { connectionId: string; sql: string; timeoutSeconds?: number }) => request<QueryResult>('/queries/execute', { method: 'POST', body: JSON.stringify(input) }),
  beginTransaction: (connectionId:string) => request<TransactionState>('/transactions/begin',{method:'POST',body:JSON.stringify({connectionId})}),
  transactionAction: (id:string, action:'commit'|'rollback'|'savepoint'|'rollback_to', name?:string) => request<TransactionState>(`/transactions/${encodeURIComponent(id)}/${action}`,{method:'POST',body:JSON.stringify({name})}),
  cancelSession: (connectionId:string, sessionId:string, force=false) => request<void>(`/connections/${encodeURIComponent(connectionId)}/sessions/${encodeURIComponent(sessionId)}/cancel`,{method:'POST',body:JSON.stringify({force})}),
  listSavedQueries: () => request<SavedQuery[]>('/saved-queries'),
  createSavedQuery: (input: {connectionId?:string;folder:string;title:string;sql:string;tags:string[]}) => request<SavedQuery>('/saved-queries',{method:'POST',body:JSON.stringify(input)}),
  deleteSavedQuery: (id:string) => request<void>(`/saved-queries/${encodeURIComponent(id)}`,{method:'DELETE'}),
  getDashboard: (connectionId: string) => request<OperationsSnapshot>(`/connections/${encodeURIComponent(connectionId)}/dashboard`),
  getSessions: (connectionId: string) => request<OperationsSnapshot>(`/connections/${encodeURIComponent(connectionId)}/sessions`),
  getLocks: (connectionId: string) => request<OperationsSnapshot>(`/connections/${encodeURIComponent(connectionId)}/locks`),
  getPerformance: (connectionId: string) => request<OperationsSnapshot>(`/connections/${encodeURIComponent(connectionId)}/performance`)
}
