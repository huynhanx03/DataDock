import type { Connection, ConnectionInput, ConnectionTestResult } from '../entities/connection'
import type {
  CatalogTree,
  CreateIndexInput,
  CreateTableInput,
  DataColumn,
  DatabaseDashboard,
  DatabaseLocks,
  DatabaseObject,
  DatabasePerformance,
  DatabaseSessions,
  RowMutation,
  TableAlteration,
  TableMutateResult,
  TableRowsInput,
  TableRowsResult,
  TableSchema,
  TableRow,
} from '../entities/database-object'
import type {
  CreateSavedQueryInput,
  ExecuteQueryInput,
  QueryHistoryItem,
  QueryResult,
  SavedQuery,
  TransactionAction,
  TransactionState,
} from '../entities/query'
import type { CreateWorkspaceInput, UpdateWorkspaceInput, Workspace } from '../entities/workspace'
import { API_ENDPOINTS, buildApiUrl } from '../shared/config/api-endpoints'
import { APP_CONFIG } from '../shared/config/constants'
import { GatewayError, type DataDockGateway } from './gateway'

type ApiEnvelope<T> = { data: T }
type ApiErrorEnvelope = { error?: { code?: string; message?: string } }
type RawRows = { columns?: string[]; rows?: unknown[][]; total?: number; limit?: number; offset?: number }
type RawQueryResult = { columns?: string[]; rows?: unknown[][]; rowsAffected?: number; durationMs?: number; message?: string }

function normalizeConnection(connection: Omit<Connection, 'status'> & Partial<Pick<Connection, 'status'>>) : Connection {
  return { ...connection, status: connection.status ?? 'disconnected' }
}

function inferType(values: unknown[]) {
  const value = values.find((item) => item !== null && item !== undefined)
  if (typeof value === 'number') return Number.isInteger(value) ? 'integer' : 'numeric'
  if (typeof value === 'boolean') return 'boolean'
  if (value instanceof Date) return 'timestamp'
  if (typeof value === 'object') return 'json'
  return 'text'
}

function normalizeColumns(names: string[], rows: unknown[][]): DataColumn[] {
  const usedKeys = new Set<string>()
  return names.map((name, index) => {
    let key = name
    let suffix = 2
    while (usedKeys.has(key)) {
      key = `${name}__${suffix}`
      suffix += 1
    }
    usedKeys.add(key)
    return { name, key, type: inferType(rows.map((row) => row[index])) }
  })
}

function normalizeRows(columns: DataColumn[], rows: unknown[][]): TableRow[] {
  return rows.map((row) => Object.fromEntries(columns.map((column, index) => [column.key ?? column.name, row[index]])))
}

function tableNode(connectionId: string, schema: string, name: string, parentId: string): DatabaseObject {
  return {
    id: `${connectionId}:table:${schema}.${name}`,
    connectionId,
    parentId,
    name,
    qualifiedName: `${schema}.${name}`,
    kind: 'table',
    schema,
  }
}

function catalogFromTables(connection: Connection, tables: string[]): CatalogTree {
  const defaultSchema = connection.engine === 'postgresql' ? 'public' : connection.database || 'default'
  const grouped = new Map<string, string[]>()
  tables.forEach((reference) => {
    const split = reference.split('.')
    const schema = split.length > 1 ? split[0] : defaultSchema
    const name = split.length > 1 ? split.slice(1).join('.') : reference
    grouped.set(schema, [...(grouped.get(schema) ?? []), name])
  })
  const databaseName = connection.database || connection.host
  const databaseId = `${connection.id}:database:${databaseName}`
  const schemas: DatabaseObject[] = [...grouped.entries()].map(([schema, names]) => {
    const schemaId = `${connection.id}:schema:${schema}`
    const tablesId = `${schemaId}:group:tables`
    const groups: DatabaseObject[] = [
      { id: tablesId, connectionId: connection.id, parentId: schemaId, name: 'Tables', qualifiedName: `${schema}.tables`, kind: 'group', schema, count: names.length, children: names.map((name) => tableNode(connection.id, schema, name, tablesId)) },
      { id: `${schemaId}:group:views`, connectionId: connection.id, parentId: schemaId, name: 'Views', qualifiedName: `${schema}.views`, kind: 'group', schema, count: 0, children: [] },
      { id: `${schemaId}:group:functions`, connectionId: connection.id, parentId: schemaId, name: 'Functions', qualifiedName: `${schema}.functions`, kind: 'group', schema, count: 0, children: [] },
      { id: `${schemaId}:group:procedures`, connectionId: connection.id, parentId: schemaId, name: 'Procedures', qualifiedName: `${schema}.procedures`, kind: 'group', schema, count: 0, children: [] },
    ]
    return { id: schemaId, connectionId: connection.id, parentId: databaseId, name: schema, qualifiedName: schema, kind: 'schema', database: databaseName, schema, children: groups }
  })
  return {
    connectionId: connection.id,
    engine: connection.engine,
    databases: [{ id: databaseId, connectionId: connection.id, name: databaseName, qualifiedName: databaseName, kind: 'database', database: databaseName, children: schemas }],
    loadedAt: new Date().toISOString(),
  }
}

export class HttpDataDockGateway implements DataDockGateway {
  readonly source = 'api' as const
  private readonly queryHistory: QueryHistoryItem[] = []
  private historySequence = 0

  private async request<T>(path: string, init: RequestInit = {}, signal?: AbortSignal): Promise<T> {
    const controller = new AbortController()
    const abort = () => controller.abort(signal?.reason)
    signal?.addEventListener('abort', abort, { once: true })
    const timeout = window.setTimeout(() => controller.abort(), APP_CONFIG.api.requestTimeoutMs)
    const headers = new Headers(init.headers)
    if (init.body && !headers.has('Content-Type')) headers.set('Content-Type', 'application/json')
    try {
      const response = await fetch(buildApiUrl(path), { ...init, headers, signal: controller.signal })
      const text = await response.text()
      const body = text ? JSON.parse(text) as ApiEnvelope<T> | ApiErrorEnvelope : undefined
      if (!response.ok) {
        const error = body as ApiErrorEnvelope | undefined
        throw new GatewayError(error?.error?.message || `Request failed (${response.status})`, error?.error?.code || 'http_error', response.status)
      }
      return body && 'data' in body ? body.data : undefined as T
    } catch (error) {
      if (error instanceof GatewayError) throw error
      if (controller.signal.aborted) throw new GatewayError('Request was cancelled or timed out', 'request_aborted')
      throw new GatewayError(error instanceof Error ? error.message : 'Network request failed', 'network_error')
    } finally {
      window.clearTimeout(timeout)
      signal?.removeEventListener('abort', abort)
    }
  }

  listWorkspaces(signal?: AbortSignal) {
    return this.request<Workspace[]>(API_ENDPOINTS.workspaces, {}, signal)
  }

  createWorkspace(input: CreateWorkspaceInput, signal?: AbortSignal) {
    return this.request<Workspace>(API_ENDPOINTS.workspaces, { method: 'POST', body: JSON.stringify(input) }, signal)
  }

  updateWorkspace(id: string, input: UpdateWorkspaceInput, signal?: AbortSignal) {
    return this.request<Workspace>(API_ENDPOINTS.workspace(id), { method: 'PATCH', body: JSON.stringify(input) }, signal)
  }

  deleteWorkspace(id: string, signal?: AbortSignal) {
    return this.request<void>(API_ENDPOINTS.workspace(id), { method: 'DELETE' }, signal)
  }

  reorderWorkspaces(ids: string[], signal?: AbortSignal) {
    return this.request<void>(API_ENDPOINTS.workspaceOrder, { method: 'PUT', body: JSON.stringify({ ids }) }, signal)
  }

  async listConnections(workspaceId?: string, signal?: AbortSignal) {
    const query = workspaceId ? `?${new URLSearchParams({ workspaceId })}` : ''
    const connections = await this.request<Array<Omit<Connection, 'status'>>>(`${API_ENDPOINTS.connections}${query}`, {}, signal)
    return connections.map(normalizeConnection)
  }

  async getConnection(id: string, signal?: AbortSignal) {
    return normalizeConnection(await this.request<Omit<Connection, 'status'>>(API_ENDPOINTS.connection(id), {}, signal))
  }

  async createConnection(input: ConnectionInput, signal?: AbortSignal) {
    return normalizeConnection(await this.request<Omit<Connection, 'status'>>(API_ENDPOINTS.connections, { method: 'POST', body: JSON.stringify(input) }, signal))
  }

  async updateConnection(id: string, input: ConnectionInput, signal?: AbortSignal) {
    return normalizeConnection(await this.request<Omit<Connection, 'status'>>(API_ENDPOINTS.connection(id), { method: 'PATCH', body: JSON.stringify(input) }, signal))
  }

  deleteConnection(id: string, signal?: AbortSignal) {
    return this.request<void>(API_ENDPOINTS.connection(id), { method: 'DELETE' }, signal)
  }

  async duplicateConnection(id: string, signal?: AbortSignal) {
    return normalizeConnection(await this.request<Omit<Connection, 'status'>>(API_ENDPOINTS.duplicateConnection(id), { method: 'POST' }, signal))
  }

  async testConnection(id: string, signal?: AbortSignal): Promise<ConnectionTestResult> {
    const startedAt = performance.now()
    const result = await this.request<{ ok: boolean }>(API_ENDPOINTS.testConnection(id), { method: 'POST' }, signal)
    return { ok: result.ok, latencyMs: Math.max(1, Math.round(performance.now() - startedAt)), message: result.ok ? 'Connection successful' : 'Connection failed' }
  }

  async listCatalog(connectionId: string, signal?: AbortSignal) {
    const [connection, tables] = await Promise.all([
      this.getConnection(connectionId, signal),
      this.request<string[]>(API_ENDPOINTS.tables(connectionId), {}, signal),
    ])
    return catalogFromTables(connection, tables)
  }

  async getTableRows(connectionId: string, table: string, input: TableRowsInput = {}, signal?: AbortSignal): Promise<TableRowsResult> {
    const pageSize = Math.min(input.pageSize ?? APP_CONFIG.table.defaultPageSize, APP_CONFIG.table.maxPageSize)
    const page = Math.max(1, input.page ?? 1)
    const parameters = new URLSearchParams({ limit: String(pageSize), offset: String((page - 1) * pageSize) })
    if (input.search) parameters.set('search', input.search)
    if (input.sortBy) parameters.set('sort', input.sortBy)
    if (input.sortDirection) parameters.set('order', input.sortDirection)
    const startedAt = performance.now()
    const result = await this.request<RawRows>(`${API_ENDPOINTS.tableRows(connectionId, table)}?${parameters}`, {}, signal)
    const rawRows = result.rows ?? []
    const columns = normalizeColumns(result.columns ?? [], rawRows)
    return {
      columns,
      rows: normalizeRows(columns, rawRows),
      page: Math.floor((result.offset ?? 0) / (result.limit || pageSize)) + 1,
      pageSize: result.limit || pageSize,
      total: result.total ?? rawRows.length,
      durationMs: Math.max(1, Math.round(performance.now() - startedAt)),
    }
  }

  getTableSchema(connectionId: string, table: string, signal?: AbortSignal) {
    return this.request<TableSchema>(API_ENDPOINTS.tableSchema(connectionId, table), {}, signal)
  }

  async getTableDDL(connectionId: string, table: string, signal?: AbortSignal) {
    return (await this.request<{ ddl: string }>(API_ENDPOINTS.tableDDL(connectionId, table), {}, signal)).ddl
  }

  mutateTableRows(connectionId: string, table: string, mutations: RowMutation[], signal?: AbortSignal) {
    return this.request<TableMutateResult>(API_ENDPOINTS.tableMutations(connectionId, table), { method: 'POST', body: JSON.stringify({ mutations }) }, signal)
  }

  createTable(connectionId: string, input: CreateTableInput, signal?: AbortSignal) {
    return this.request<void>(API_ENDPOINTS.schemaTables(connectionId), { method: 'POST', body: JSON.stringify(input) }, signal)
  }

  alterTable(connectionId: string, table: string, actions: TableAlteration[], signal?: AbortSignal) {
    return this.request<void>(API_ENDPOINTS.tableSchema(connectionId, table), { method: 'PATCH', body: JSON.stringify({ actions }) }, signal)
  }

  createIndex(connectionId: string, table: string, input: CreateIndexInput, signal?: AbortSignal) {
    return this.request<void>(API_ENDPOINTS.tableIndexes(connectionId, table), { method: 'POST', body: JSON.stringify(input) }, signal)
  }

  dropIndex(connectionId: string, table: string, index: string, signal?: AbortSignal) {
    return this.request<void>(API_ENDPOINTS.tableIndex(connectionId, table, index), { method: 'DELETE' }, signal)
  }

  async executeQuery(input: ExecuteQueryInput, signal?: AbortSignal): Promise<QueryResult> {
    const executedAt = new Date().toISOString()
    try {
      const result = await this.request<RawQueryResult>(API_ENDPOINTS.executeQuery, { method: 'POST', body: JSON.stringify(input) }, signal)
      const rawRows = result.rows ?? []
      const columns = normalizeColumns(result.columns ?? [], rawRows)
      const normalized = { columns, rows: normalizeRows(columns, rawRows), rowsAffected: result.rowsAffected ?? 0, durationMs: result.durationMs ?? 0, message: result.message }
      this.queryHistory.unshift({ id: `http-history-${++this.historySequence}`, connectionId: input.connectionId, sqlText: input.sql, status: 'success', durationMs: normalized.durationMs, rowCount: normalized.rows.length || normalized.rowsAffected, executedAt })
      this.queryHistory.splice(APP_CONFIG.query.historyLimit)
      return normalized
    } catch (error) {
      this.queryHistory.unshift({ id: `http-history-${++this.historySequence}`, connectionId: input.connectionId, sqlText: input.sql, status: 'error', durationMs: 0, rowCount: 0, error: error instanceof Error ? error.message : 'Query failed', executedAt })
      throw error
    }
  }

  async listQueryHistory(connectionId?: string, signal?: AbortSignal) {
    if (signal?.aborted) throw new GatewayError('Request was cancelled', 'request_aborted')
    return this.queryHistory.filter((item) => !connectionId || item.connectionId === connectionId).map((item) => ({ ...item }))
  }

  async clearQueryHistory(connectionId?: string, signal?: AbortSignal) {
    if (signal?.aborted) throw new GatewayError('Request was cancelled', 'request_aborted')
    if (!connectionId) {
      this.queryHistory.length = 0
      return
    }
    for (let index = this.queryHistory.length - 1; index >= 0; index -= 1) {
      if (this.queryHistory[index].connectionId === connectionId) this.queryHistory.splice(index, 1)
    }
  }

  async beginTransaction(connectionId: string, signal?: AbortSignal) {
    return this.normalizeTransaction(await this.request<TransactionState>(API_ENDPOINTS.beginTransaction, { method: 'POST', body: JSON.stringify({ connectionId }) }, signal))
  }

  async transactionAction(id: string, action: TransactionAction, name?: string, signal?: AbortSignal) {
    return this.normalizeTransaction(await this.request<TransactionState>(API_ENDPOINTS.transactionAction(id, action), { method: 'POST', body: JSON.stringify({ name }) }, signal))
  }

  getDashboard(connectionId: string, signal?: AbortSignal) {
    return this.request<DatabaseDashboard>(API_ENDPOINTS.dashboard(connectionId), {}, signal)
  }

  getSessions(connectionId: string, signal?: AbortSignal) {
    return this.request<DatabaseSessions>(API_ENDPOINTS.sessions(connectionId), {}, signal)
  }

  cancelSession(connectionId: string, sessionId: string, force = false, signal?: AbortSignal) {
    return this.request<void>(API_ENDPOINTS.cancelSession(connectionId, sessionId), { method: 'POST', body: JSON.stringify({ force }) }, signal)
  }

  getLocks(connectionId: string, signal?: AbortSignal) {
    return this.request<DatabaseLocks>(API_ENDPOINTS.locks(connectionId), {}, signal)
  }

  getPerformance(connectionId: string, signal?: AbortSignal) {
    return this.request<DatabasePerformance>(API_ENDPOINTS.performance(connectionId), {}, signal)
  }

  listSavedQueries(signal?: AbortSignal) {
    return this.request<SavedQuery[]>(API_ENDPOINTS.savedQueries, {}, signal)
  }

  createSavedQuery(input: CreateSavedQueryInput, signal?: AbortSignal) {
    return this.request<SavedQuery>(API_ENDPOINTS.savedQueries, { method: 'POST', body: JSON.stringify(input) }, signal)
  }

  deleteSavedQuery(id: string, signal?: AbortSignal) {
    return this.request<void>(API_ENDPOINTS.savedQuery(id), { method: 'DELETE' }, signal)
  }

  private normalizeTransaction(transaction: TransactionState): TransactionState {
    return transaction
  }
}
