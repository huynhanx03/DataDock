import type { Connection, ConnectionInput, ConnectionRuntimeStatus, ConnectionTestResult } from '../entities/connection'
import type {
  CatalogTree,
  CreateIndexInput,
  CreateTableInput,
  DatabaseDashboard,
  DatabaseLocks,
  DatabasePerformance,
  DatabaseSessions,
  RowMutation,
  TableAlteration,
  TableMutateResult,
  TableRowsInput,
  TableRowsResult,
  TableSchema,
} from '../entities/database-object'
import type {
  CreateSavedQueryInput,
  CreateSavedQueryFolderInput,
  ExecuteQueryInput,
  ExplainQueryInput,
  PublicSavedQuery,
  QueryHistoryItem,
  QueryResult,
  SavedQuery,
  SavedQueryFolder,
  SavedQueryShare,
  TransactionAction,
  TransactionState,
  UpdateSavedQueryFolderInput,
  UpdateSavedQueryInput,
} from '../entities/query'
import type { SchemaAction, SchemaApplyInput, SchemaApplyResult, SchemaPreview } from '../entities/schema'
import type { CreateWorkspaceInput, UpdateWorkspaceInput, Workspace } from '../entities/workspace'
import { APP_CONFIG } from '../shared/config/constants'
import { GatewayError, type DataDockGateway } from '../data/gateway'
import {
  mockCatalogs,
  mockConnections,
  mockDashboards,
  mockLocks,
  mockPerformance,
  mockQueryHistory,
  mockSavedQueries,
  mockSessions,
  mockTables,
  mockWorkspaces,
  type MockTableFixture,
} from './fixtures'

const copy = <T>(value: T): T => structuredClone(value)

function matchesKeys(row: Record<string, unknown>, keys: Record<string, unknown>) {
  return Object.entries(keys).every(([key, value]) => Object.is(row[key], value))
}

function tableIdentity(connectionId: string, table: string) {
  return `${connectionId}:${table.replaceAll('`', '').replaceAll('"', '')}`
}

export class MockDataDockGateway implements DataDockGateway {
  readonly source = 'mock' as const
  private workspaces = copy(mockWorkspaces)
  private connections = copy(mockConnections)
  private catalogs = copy(mockCatalogs)
  private tables = copy(mockTables)
  private dashboards = copy(mockDashboards)
  private sessions = copy(mockSessions)
  private locks = copy(mockLocks)
  private performance = copy(mockPerformance)
  private history = copy(mockQueryHistory)
  private savedQueries: SavedQuery[] = copy(mockSavedQueries).map((item, index) => ({ ...item, favorite: item.favorite ?? index < 2 }))
  private transactions = new Map<string, TransactionState>()
  private savedQueryFolders: SavedQueryFolder[] = []
  private sequence = 1000

  constructor() {
    const now = new Date().toISOString()
    this.savedQueryFolders = [...new Set(this.savedQueries.map((query) => query.folder))].map((name, position) => ({
      id: `mock-folder-${position + 1}`,
      name,
      position,
      queryCount: this.savedQueries.filter((query) => query.folder === name).length,
      createdAt: now,
      updatedAt: now,
    }))
  }

  private async wait(signal?: AbortSignal, mutation = false) {
    const latency = mutation ? APP_CONFIG.mock.mutationLatencyMs : APP_CONFIG.mock.latencyMs
    await new Promise<void>((resolve, reject) => {
      if (signal?.aborted) {
        reject(new GatewayError('Request was cancelled', 'request_aborted'))
        return
      }
      const timer = window.setTimeout(resolve, latency)
      const abort = () => {
        window.clearTimeout(timer)
        reject(new GatewayError('Request was cancelled', 'request_aborted'))
      }
      signal?.addEventListener('abort', abort, { once: true })
    })
  }

  private id(prefix: string) {
    this.sequence += 1
    return `${prefix}-${this.sequence}`
  }

  private connection(id: string) {
    const connection = this.connections.find((item) => item.id === id)
    if (!connection) throw new GatewayError('Connection was not found', 'not_found', 404)
    return connection
  }

  private table(connectionId: string, reference: string) {
    const connection = this.connection(connectionId)
    const qualified = reference.includes('.') ? reference : `${connection.engine === 'postgresql' ? 'public' : connection.database}.${reference}`
    const key = tableIdentity(connectionId, qualified)
    const table = this.tables[key]
    if (!table) throw new GatewayError(`Table ${qualified} was not found`, 'not_found', 404)
    return { key, qualified, table }
  }

  async listWorkspaces(signal?: AbortSignal) {
    await this.wait(signal)
    return copy([...this.workspaces].sort((a, b) => a.position - b.position))
  }

  async createWorkspace(input: CreateWorkspaceInput, signal?: AbortSignal) {
    await this.wait(signal, true)
    const now = new Date().toISOString()
    const workspace: Workspace = { id: this.id('workspace'), ...input, position: this.workspaces.length, collapsed: false, createdAt: now, updatedAt: now }
    this.workspaces.push(workspace)
    return copy(workspace)
  }

  async updateWorkspace(id: string, input: UpdateWorkspaceInput, signal?: AbortSignal) {
    await this.wait(signal, true)
    const index = this.workspaces.findIndex((item) => item.id === id)
    if (index < 0) throw new GatewayError('Workspace was not found', 'not_found', 404)
    this.workspaces[index] = { ...this.workspaces[index], ...input, updatedAt: new Date().toISOString() }
    return copy(this.workspaces[index])
  }

  async deleteWorkspace(id: string, signal?: AbortSignal) {
    await this.wait(signal, true)
    if (!this.workspaces.some((item) => item.id === id)) throw new GatewayError('Workspace was not found', 'not_found', 404)
    const connectionIds = this.connections.filter((item) => item.workspaceId === id).map((item) => item.id)
    this.workspaces = this.workspaces.filter((item) => item.id !== id).map((item, position) => ({ ...item, position }))
    this.connections = this.connections.filter((item) => item.workspaceId !== id)
    connectionIds.forEach((connectionId) => delete this.catalogs[connectionId])
  }

  async reorderWorkspaces(ids: string[], signal?: AbortSignal) {
    await this.wait(signal, true)
    if (ids.length !== this.workspaces.length || ids.some((id) => !this.workspaces.some((workspace) => workspace.id === id))) throw new GatewayError('Workspace order is invalid', 'validation_error', 400)
    this.workspaces = ids.map((id, position) => ({ ...this.workspaces.find((item) => item.id === id)!, position }))
  }

  async listConnections(workspaceId?: string, signal?: AbortSignal) {
    await this.wait(signal)
    return copy(this.connections.filter((item) => !workspaceId || item.workspaceId === workspaceId).sort((a, b) => Number(b.favorite) - Number(a.favorite) || a.name.localeCompare(b.name)))
  }

  async getConnection(id: string, signal?: AbortSignal) {
    await this.wait(signal)
    return copy(this.connection(id))
  }

  async createConnection(input: ConnectionInput, signal?: AbortSignal) {
    await this.wait(signal, true)
    if (!this.workspaces.some((item) => item.id === input.workspaceId)) throw new GatewayError('Workspace was not found', 'not_found', 404)
    const now = new Date().toISOString()
    const connection: Connection = {
      ...input,
      id: this.id('connection'),
      sshTunnel: { ...input.sshTunnel, hasPassword: Boolean(input.sshTunnel.password) },
      favorite: false,
      hasPassword: Boolean(input.password),
      status: 'disconnected',
      createdAt: now,
      updatedAt: now,
    }
    this.connections.push(connection)
    const database = connection.database || connection.host
    const schema = connection.engine === 'postgresql' ? 'public' : database
    const databaseId = `${connection.id}:database:${database}`
    const schemaId = `${connection.id}:schema:${schema}`
    this.catalogs[connection.id] = {
      connectionId: connection.id,
      engine: connection.engine,
      loadedAt: now,
      databases: [{ id: databaseId, connectionId: connection.id, name: database, qualifiedName: database, kind: 'database', database, children: [{ id: schemaId, connectionId: connection.id, parentId: databaseId, name: schema, qualifiedName: schema, kind: 'schema', database, schema, children: ['Tables', 'Views', 'Functions', 'Procedures'].map((name) => ({ id: `${schemaId}:group:${name.toLowerCase()}`, connectionId: connection.id, parentId: schemaId, name, qualifiedName: `${schema}.${name.toLowerCase()}`, kind: 'group', schema, count: 0, children: [] })) }] }],
    }
    return copy(connection)
  }

  async updateConnection(id: string, input: ConnectionInput, signal?: AbortSignal) {
    await this.wait(signal, true)
    const index = this.connections.findIndex((item) => item.id === id)
    if (index < 0) throw new GatewayError('Connection was not found', 'not_found', 404)
    const previous = this.connections[index]
    this.connections[index] = {
      ...previous,
      ...input,
      id,
      sshTunnel: { ...input.sshTunnel, hasPassword: Boolean(input.sshTunnel.password) || previous.sshTunnel.hasPassword },
      hasPassword: Boolean(input.password) || previous.hasPassword,
      status: 'disconnected',
      updatedAt: new Date().toISOString(),
    }
    return copy(this.connections[index])
  }

  async deleteConnection(id: string, signal?: AbortSignal) {
    await this.wait(signal, true)
    if (!this.connections.some((item) => item.id === id)) throw new GatewayError('Connection was not found', 'not_found', 404)
    this.connections = this.connections.filter((item) => item.id !== id)
    delete this.catalogs[id]
    Object.keys(this.tables).filter((key) => key.startsWith(`${id}:`)).forEach((key) => delete this.tables[key])
  }

  async duplicateConnection(id: string, signal?: AbortSignal) {
    await this.wait(signal, true)
    const source = this.connection(id)
    const now = new Date().toISOString()
    const duplicate: Connection = { ...copy(source), id: this.id('connection'), name: `${source.name} copy`, favorite: false, status: 'disconnected', createdAt: now, updatedAt: now }
    this.connections.push(duplicate)
    const catalog = copy(this.catalogs[id])
    if (catalog) this.catalogs[duplicate.id] = { ...catalog, connectionId: duplicate.id, loadedAt: now }
    return copy(duplicate)
  }

  async testConnection(id: string, signal?: AbortSignal): Promise<ConnectionTestResult> {
    await this.wait(signal)
    const connection = this.connection(id)
    const latencyMs = connection.engine === 'postgresql' ? 18 : connection.engine === 'mysql' ? 42 : 76
    connection.status = 'connected'
    connection.latencyMs = latencyMs
    connection.lastConnectedAt = new Date().toISOString()
    return { ok: true, latencyMs, message: `Connected to ${connection.name}` }
  }

  async testConnectionDraft(input: ConnectionInput, signal?: AbortSignal): Promise<ConnectionTestResult> {
    await this.wait(signal)
    const failed = ['invalid', 'offline', 'unreachable', 'refused'].some((marker) => `${input.host} ${input.proxyUrl} ${input.sshTunnel.host}`.toLowerCase().includes(marker))
    if (failed) throw new GatewayError(`Could not reach ${input.host}:${input.port}`, 'connection_failed', 502)
    return { ok: true, latencyMs: input.engine === 'postgresql' ? 18 : 42, message: `Connected to ${input.host}:${input.port}` }
  }

  async connectConnection(id: string, signal?: AbortSignal): Promise<Connection> {
    await this.testConnection(id, signal)
    return copy(this.connection(id))
  }

  async disconnectConnection(id: string, signal?: AbortSignal): Promise<Connection> {
    await this.wait(signal, true)
    const connection = this.connection(id)
    connection.status = 'disconnected'
    return copy(connection)
  }

  async getConnectionStatus(id: string, signal?: AbortSignal): Promise<ConnectionRuntimeStatus> {
    await this.wait(signal)
    const connection = this.connection(id)
    return { status: connection.status, latencyMs: connection.latencyMs, lastConnectedAt: connection.lastConnectedAt }
  }

  async setConnectionFavorite(id: string, favorite: boolean, signal?: AbortSignal): Promise<Connection> {
    await this.wait(signal, true)
    const connection = this.connection(id)
    connection.favorite = favorite
    connection.updatedAt = new Date().toISOString()
    return copy(connection)
  }

  async listCatalog(connectionId: string, signal?: AbortSignal): Promise<CatalogTree> {
    await this.wait(signal)
    this.connection(connectionId)
    const catalog = this.catalogs[connectionId]
    if (!catalog) throw new GatewayError('Catalog is unavailable', 'not_found', 404)
    return copy(catalog)
  }

  async getTableRows(connectionId: string, reference: string, input: TableRowsInput = {}, signal?: AbortSignal): Promise<TableRowsResult> {
    await this.wait(signal)
    const { table } = this.table(connectionId, reference)
    const page = Math.max(1, input.page ?? 1)
    const pageSize = Math.min(input.pageSize ?? APP_CONFIG.table.defaultPageSize, APP_CONFIG.table.maxPageSize)
    const search = input.search?.trim().toLowerCase()
    let rows = table.rows.filter((row) => !search || Object.values(row).some((value) => String(value ?? '').toLowerCase().includes(search)))
    if (input.sortBy) {
      const direction = input.sortDirection === 'desc' ? -1 : 1
      const column = input.sortBy
      rows = [...rows].sort((left, right) => String(left[column] ?? '').localeCompare(String(right[column] ?? ''), undefined, { numeric: true }) * direction)
    }
    return { columns: copy(table.columns), rows: copy(rows.slice((page - 1) * pageSize, page * pageSize)), page, pageSize, total: rows.length, durationMs: 12 }
  }

  async getTableSchema(connectionId: string, reference: string, signal?: AbortSignal): Promise<TableSchema> {
    await this.wait(signal)
    return copy(this.table(connectionId, reference).table.schema)
  }

  async getTableDDL(connectionId: string, reference: string, signal?: AbortSignal) {
    await this.wait(signal)
    return this.table(connectionId, reference).table.ddl
  }

  async mutateTableRows(connectionId: string, reference: string, mutations: RowMutation[], signal?: AbortSignal): Promise<TableMutateResult> {
    await this.wait(signal, true)
    const { table } = this.table(connectionId, reference)
    mutations.forEach((mutation) => {
      if (mutation.kind === 'insert' && mutation.values) table.rows.push(copy(mutation.values))
      if (mutation.kind === 'update' && mutation.keys && mutation.values) {
        const row = table.rows.find((item) => matchesKeys(item, mutation.keys!))
        if (row) Object.assign(row, copy(mutation.values))
      }
      if (mutation.kind === 'delete' && mutation.keys) table.rows = table.rows.filter((item) => !matchesKeys(item, mutation.keys!))
    })
    return { applied: mutations.length }
  }

  mutateTableRowsInTransaction(connectionId: string, reference: string, mutations: RowMutation[], transactionId: string, signal?: AbortSignal) {
    if (!this.transactions.has(transactionId)) return Promise.reject(new GatewayError('Transaction was not found or has expired', 'not_found', 404))
    return this.mutateTableRows(connectionId, reference, mutations, signal)
  }

  async createTable(connectionId: string, input: CreateTableInput, signal?: AbortSignal) {
    await this.wait(signal, true)
    const connection = this.connection(connectionId)
    const schema = input.schema || (connection.engine === 'postgresql' ? 'public' : connection.database)
    const reference = `${schema}.${input.name}`
    const key = tableIdentity(connectionId, reference)
    if (this.tables[key]) throw new GatewayError('Table already exists', 'validation_error', 400)
    const schemaDefinition: TableSchema = { columns: copy(input.columns), indexes: [], constraints: [] }
    this.tables[key] = { columns: input.columns.map((column) => ({ name: column.name, type: column.dataType, nullable: column.nullable })), rows: [], schema: schemaDefinition, ddl: `CREATE TABLE ${reference} (\n${input.columns.map((column) => `  ${column.name} ${column.dataType}${column.nullable ? '' : ' NOT NULL'}`).join(',\n')}\n);` }
    const schemaNode = this.catalogs[connectionId]?.databases.flatMap((database) => database.children ?? []).find((item) => item.kind === 'schema' && item.name === schema)
    const group = schemaNode?.children?.find((item) => item.kind === 'group' && item.name === 'Tables')
    if (group) {
      group.children = [...(group.children ?? []), { id: `${connectionId}:table:${reference}`, connectionId, parentId: group.id, name: input.name, qualifiedName: reference, kind: 'table', schema }]
      group.count = group.children.length
    }
  }

  async alterTable(connectionId: string, reference: string, actions: TableAlteration[], signal?: AbortSignal) {
    await this.wait(signal, true)
    const target = this.table(connectionId, reference)
    actions.forEach((action) => {
      if (action.kind === 'add_column' && action.definition) {
        target.table.schema.columns.push(copy(action.definition))
        target.table.columns.push({ name: action.definition.name, type: action.definition.dataType, nullable: action.definition.nullable })
        target.table.rows.forEach((row) => { row[action.definition!.name] = null })
      }
      if (action.kind === 'drop_column' && action.column) {
        target.table.schema.columns = target.table.schema.columns.filter((column) => column.name !== action.column)
        target.table.columns = target.table.columns.filter((column) => column.name !== action.column)
        target.table.rows.forEach((row) => { delete row[action.column!] })
      }
      if (action.kind === 'rename_column' && action.column && action.newName) {
        const column = target.table.schema.columns.find((item) => item.name === action.column)
        const dataColumn = target.table.columns.find((item) => item.name === action.column)
        if (column) column.name = action.newName
        if (dataColumn) dataColumn.name = action.newName
        target.table.rows.forEach((row) => { row[action.newName!] = row[action.column!]; delete row[action.column!] })
      }
      if (action.kind === 'change_type' && action.column && action.definition) {
        const column = target.table.schema.columns.find((item) => item.name === action.column)
        const dataColumn = target.table.columns.find((item) => item.name === action.column)
        if (column) column.dataType = action.definition.dataType
        if (dataColumn) dataColumn.type = action.definition.dataType
      }
      if (action.kind === 'set_nullable' && action.column && action.definition) {
        const column = target.table.schema.columns.find((item) => item.name === action.column)
        if (column) column.nullable = action.definition.nullable
      }
      if (action.kind === 'set_default' && action.column && action.definition) {
        const column = target.table.schema.columns.find((item) => item.name === action.column)
        if (column) column.defaultValue = action.definition.defaultValue
      }
    })
  }

  async createIndex(connectionId: string, reference: string, input: CreateIndexInput, signal?: AbortSignal) {
    await this.wait(signal, true)
    const { table } = this.table(connectionId, reference)
    table.schema.indexes.push({ name: input.name, unique: input.unique, primary: false, type: 'btree', definition: `CREATE ${input.unique ? 'UNIQUE ' : ''}INDEX ${input.name} ON ${reference} (${input.columns.join(', ')})` })
  }

  async dropIndex(connectionId: string, reference: string, index: string, signal?: AbortSignal) {
    await this.wait(signal, true)
    const { table } = this.table(connectionId, reference)
    table.schema.indexes = table.schema.indexes.filter((item) => item.name !== index || item.primary)
  }

  async executeQuery(input: ExecuteQueryInput, signal?: AbortSignal): Promise<QueryResult> {
    const startedAt = new Date().toISOString()
    const sql = input.sql.trim()
    const executionId = input.executionId ?? this.id('execution')
    try {
      await this.wait(signal, true)
      if (!sql) throw new GatewayError('Query is required', 'validation_error', 400)
      if (/missing_column|syntax_error/i.test(sql)) throw new GatewayError('column "missing_column" does not exist', 'query_error', 400)
      let result: QueryResult
      if (/^explain/i.test(sql)) {
        result = { columns: [{ name: 'QUERY PLAN', type: 'text' }], rows: [{ 'QUERY PLAN': 'Index Scan using idx_users_status_created on users  (cost=0.29..42.81 rows=100 width=184)' }, { 'QUERY PLAN': '  Index Cond: (status = \'active\'::text)' }, { 'QUERY PLAN': 'Planning Time: 0.214 ms' }, { 'QUERY PLAN': 'Execution Time: 1.842 ms' }], rowsAffected: 0, durationMs: 3 }
      } else if (/^select|^with/i.test(sql)) {
        const match = sql.match(/\bfrom\s+([`"\w.]+)/i)
        const reference = match?.[1]?.replaceAll('`', '').replaceAll('"', '') || 'public.users'
        const { table } = this.table(input.connectionId, reference)
        if (/count\s*\(/i.test(sql)) result = { columns: [{ name: 'count', type: 'bigint' }], rows: [{ count: table.rows.length }], rowsAffected: 0, durationMs: 7 }
        else {
          const limit = Math.min(Number(sql.match(/\blimit\s+(\d+)/i)?.[1] || 100), 500)
          result = { columns: copy(table.columns), rows: copy(table.rows.slice(0, limit)), rowsAffected: 0, durationMs: reference.includes('orders') ? 42 : 18 }
        }
      } else result = { columns: [], rows: [], rowsAffected: 1, durationMs: 12, message: 'Query executed successfully' }
      result.executionId = executionId
      this.history.unshift({ id: executionId, connectionId: input.connectionId, sqlText: sql, status: 'success', durationMs: result.durationMs, rowCount: result.rows.length || result.rowsAffected, executedAt: startedAt })
      this.history.splice(APP_CONFIG.query.historyLimit)
      return result
    } catch (error) {
      const abortReason = signal?.aborted ? String(signal.reason ?? '') : ''
      const cancelled = error instanceof GatewayError && error.code === 'request_aborted' && abortReason !== 'timeout'
      const message = abortReason === 'timeout' ? 'Query timed out' : error instanceof Error ? error.message : 'Query failed'
      this.history.unshift({ id: executionId, connectionId: input.connectionId, sqlText: sql, status: cancelled ? 'cancelled' : 'error', durationMs: 8, rowCount: 0, error: cancelled ? undefined : message, executedAt: startedAt })
      throw error
    }
  }

  explainQuery(input: ExplainQueryInput, signal?: AbortSignal) {
    const prefix = input.analyze ? 'EXPLAIN ANALYZE ' : 'EXPLAIN '
    return this.executeQuery({ ...input, sql: `${prefix}${input.sql}` }, signal)
  }

  async cancelQuery(_executionId: string, signal?: AbortSignal) {
    await this.wait(signal, true)
  }

  async listQueryHistory(connectionId?: string, signal?: AbortSignal): Promise<QueryHistoryItem[]> {
    await this.wait(signal)
    return copy(this.history.filter((item) => !connectionId || item.connectionId === connectionId))
  }

  async clearQueryHistory(connectionId?: string, signal?: AbortSignal) {
    await this.wait(signal, true)
    this.history = connectionId ? this.history.filter((item) => item.connectionId !== connectionId) : []
  }

  async deleteQueryHistory(ids: string[], signal?: AbortSignal) {
    await this.wait(signal, true)
    const selected = new Set(ids)
    this.history = this.history.filter((item) => !selected.has(item.id))
  }

  async beginTransaction(connectionId: string, signal?: AbortSignal): Promise<TransactionState> {
    await this.wait(signal, true)
    this.connection(connectionId)
    const transaction: TransactionState = { id: this.id('transaction'), connectionId, state: 'active', startedAt: new Date().toISOString(), savepoints: [] }
    this.transactions.set(transaction.id, transaction)
    return copy(transaction)
  }

  async getTransaction(connectionId: string, id: string, signal?: AbortSignal): Promise<TransactionState> {
    await this.wait(signal)
    const transaction = this.transactions.get(id)
    if (!transaction || transaction.connectionId !== connectionId) throw new GatewayError('Transaction was not found or has expired', 'not_found', 404)
    return copy(transaction)
  }

  async transactionAction(id: string, action: TransactionAction, name?: string, signal?: AbortSignal): Promise<TransactionState> {
    await this.wait(signal, true)
    const transaction = this.transactions.get(id)
    if (!transaction) throw new GatewayError('Transaction was not found or has expired', 'not_found', 404)
    if (action === 'commit') transaction.state = 'committed'
    if (action === 'rollback') transaction.state = 'rolled-back'
    if (action === 'savepoint' && name && !transaction.savepoints.includes(name)) transaction.savepoints.push(name)
    if (action === 'rollback_to' && name && !transaction.savepoints.includes(name)) throw new GatewayError('Savepoint was not found', 'not_found', 404)
    if (action === 'release' && name) transaction.savepoints = transaction.savepoints.filter((savepoint) => savepoint !== name)
    if (transaction.state !== 'active') this.transactions.delete(id)
    return copy(transaction)
  }

  async previewSchema(connectionId: string, actions: SchemaAction[], signal?: AbortSignal): Promise<SchemaPreview> {
    await this.wait(signal)
    const connection = this.connection(connectionId)
    const destructiveKinds = new Set(['drop_table', 'drop_column', 'drop_constraint', 'drop_index', 'alter_column_type'])
    const steps = actions.map((action, position) => ({
      position: position + 1,
      actionId: action.id ?? `mock-schema-${position + 1}`,
      kind: action.kind,
      sql: `${action.kind.toUpperCase()} ${action.target.schema ? `${action.target.schema}.` : ''}${action.target.table}`,
      destructive: destructiveKinds.has(action.kind),
    }))
    return {
      connectionId,
      engine: connection.engine,
      actions: copy(actions),
      steps,
      sql: steps.map((step) => `${step.sql};`).join('\n'),
      hash: `mock-${this.id('schema')}`,
      destructive: steps.some((step) => step.destructive),
      generatedAt: new Date().toISOString(),
    }
  }

  async applySchema(_connectionId: string, input: SchemaApplyInput, signal?: AbortSignal): Promise<SchemaApplyResult> {
    await this.wait(signal, true)
    return { previewHash: input.previewHash, appliedSteps: input.actions.length, appliedAt: new Date().toISOString() }
  }

  async getDashboard(connectionId: string, signal?: AbortSignal): Promise<DatabaseDashboard> {
    await this.wait(signal)
    this.connection(connectionId)
    return copy(this.dashboards[connectionId] ?? { available: false, engine: this.connection(connectionId).engine, message: 'Dashboard is unavailable', metrics: [] })
  }

  async getSessions(connectionId: string, signal?: AbortSignal): Promise<DatabaseSessions> {
    await this.wait(signal)
    this.connection(connectionId)
    return copy(this.sessions[connectionId] ?? { available: false, message: 'Session inspection is unavailable', items: [] })
  }

  async cancelSession(connectionId: string, sessionId: string, force = false, signal?: AbortSignal) {
    await this.wait(signal, true)
    this.connection(connectionId)
    const sessions = this.sessions[connectionId]
    if (!sessions?.items.some((item) => item.id === sessionId)) throw new GatewayError('Session was not found', 'not_found', 404)
    sessions.items = force ? sessions.items.filter((item) => item.id !== sessionId) : sessions.items.map((item) => item.id === sessionId ? { ...item, state: 'cancelled', query: '' } : item)
  }

  async getLocks(connectionId: string, signal?: AbortSignal): Promise<DatabaseLocks> {
    await this.wait(signal)
    this.connection(connectionId)
    return copy(this.locks[connectionId] ?? { available: false, message: 'Lock inspection is unavailable', items: [] })
  }

  async getPerformance(connectionId: string, signal?: AbortSignal): Promise<DatabasePerformance> {
    await this.wait(signal)
    this.connection(connectionId)
    return copy(this.performance[connectionId] ?? { available: false, message: 'Performance statistics are unavailable', slowQueries: [] })
  }

  async listSavedQueries(signal?: AbortSignal): Promise<SavedQuery[]> {
    await this.wait(signal)
    return copy([...this.savedQueries].sort((a, b) => b.updatedAt.localeCompare(a.updatedAt)))
  }

  async getSavedQuery(id: string, signal?: AbortSignal): Promise<SavedQuery> {
    await this.wait(signal)
    const query = this.savedQueries.find((item) => item.id === id)
    if (!query) throw new GatewayError('Saved query was not found', 'not_found', 404)
    return copy(query)
  }

  async createSavedQuery(input: CreateSavedQueryInput, signal?: AbortSignal): Promise<SavedQuery> {
    await this.wait(signal, true)
    const now = new Date().toISOString()
    const query: SavedQuery = { ...input, id: this.id('saved'), folder: input.folder || 'General', favorite: false, shareCode: this.id('share').toUpperCase(), createdAt: now, updatedAt: now }
    this.savedQueries.unshift(query)
    this.ensureFolder(query.folder)
    return copy(query)
  }

  async updateSavedQuery(id: string, input: UpdateSavedQueryInput, signal?: AbortSignal): Promise<SavedQuery> {
    await this.wait(signal, true)
    const index = this.savedQueries.findIndex((item) => item.id === id)
    if (index < 0) throw new GatewayError('Saved query was not found', 'not_found', 404)
    const current = this.savedQueries[index]
    this.savedQueries[index] = {
      ...current,
      ...input,
      connectionId: input.clearConnection ? undefined : input.connectionId ?? current.connectionId,
      tags: input.tags ? [...input.tags] : current.tags,
      updatedAt: new Date().toISOString(),
    }
    this.ensureFolder(this.savedQueries[index].folder)
    return copy(this.savedQueries[index])
  }

  async duplicateSavedQuery(id: string, signal?: AbortSignal): Promise<SavedQuery> {
    const source = await this.getSavedQuery(id, signal)
    return this.createSavedQuery({ connectionId: source.connectionId, folder: source.folder, title: `${source.title} copy`, sql: source.sql, tags: source.tags }, signal)
  }

  async setSavedQueryFavorite(id: string, favorite: boolean, signal?: AbortSignal): Promise<SavedQuery> {
    await this.wait(signal, true)
    const query = this.savedQueries.find((item) => item.id === id)
    if (!query) throw new GatewayError('Saved query was not found', 'not_found', 404)
    query.favorite = favorite
    query.updatedAt = new Date().toISOString()
    return copy(query)
  }

  async deleteSavedQuery(id: string, signal?: AbortSignal) {
    await this.wait(signal, true)
    if (!this.savedQueries.some((item) => item.id === id)) throw new GatewayError('Saved query was not found', 'not_found', 404)
    this.savedQueries = this.savedQueries.filter((item) => item.id !== id)
  }

  async shareSavedQuery(id: string, expiresAt?: string, signal?: AbortSignal): Promise<SavedQueryShare> {
    await this.wait(signal, true)
    const query = this.savedQueries.find((item) => item.id === id)
    if (!query) throw new GatewayError('Saved query was not found', 'not_found', 404)
    query.shareCode = query.shareCode ?? this.id('share').toUpperCase()
    return { savedQueryId: id, shareCode: query.shareCode, createdAt: new Date().toISOString(), expiresAt }
  }

  async getSavedQueryShare(id: string, signal?: AbortSignal): Promise<SavedQueryShare | null> {
    await this.wait(signal)
    const query = this.savedQueries.find((item) => item.id === id)
    if (!query) throw new GatewayError('Saved query was not found', 'not_found', 404)
    if (!query.shareCode) return null
    return { savedQueryId: id, shareCode: query.shareCode, createdAt: query.updatedAt }
  }

  async revokeSavedQueryShare(id: string, signal?: AbortSignal) {
    await this.wait(signal, true)
    const query = this.savedQueries.find((item) => item.id === id)
    if (!query) throw new GatewayError('Saved query was not found', 'not_found', 404)
    query.shareCode = undefined
  }

  async getSharedQuery(code: string, signal?: AbortSignal): Promise<PublicSavedQuery> {
    await this.wait(signal)
    const query = this.savedQueries.find((item) => item.shareCode === code)
    if (!query) throw new GatewayError('Shared query was not found', 'not_found', 404)
    return { title: query.title, sql: query.sql, tags: [...query.tags], createdAt: query.createdAt, sharedAt: query.updatedAt }
  }

  async listSavedQueryFolders(signal?: AbortSignal): Promise<SavedQueryFolder[]> {
    await this.wait(signal)
    return copy(this.savedQueryFolders.map((folder) => ({ ...folder, queryCount: this.savedQueries.filter((query) => query.folder === folder.name).length })).sort((left, right) => left.position - right.position))
  }

  async createSavedQueryFolder(input: CreateSavedQueryFolderInput, signal?: AbortSignal): Promise<SavedQueryFolder> {
    await this.wait(signal, true)
    if (this.savedQueryFolders.some((folder) => folder.name === input.name)) throw new GatewayError('Saved query folder already exists', 'conflict', 409)
    const now = new Date().toISOString()
    const folder = { id: this.id('folder'), name: input.name, position: input.position ?? this.savedQueryFolders.length, queryCount: 0, createdAt: now, updatedAt: now }
    this.savedQueryFolders.push(folder)
    return copy(folder)
  }

  async updateSavedQueryFolder(id: string, input: UpdateSavedQueryFolderInput, signal?: AbortSignal): Promise<SavedQueryFolder> {
    await this.wait(signal, true)
    const folder = this.savedQueryFolders.find((item) => item.id === id)
    if (!folder) throw new GatewayError('Saved query folder was not found', 'not_found', 404)
    const previousName = folder.name
    if (input.name) folder.name = input.name
    if (input.position !== undefined) folder.position = input.position
    folder.updatedAt = new Date().toISOString()
    if (folder.name !== previousName) this.savedQueries.forEach((query) => { if (query.folder === previousName) query.folder = folder.name })
    folder.queryCount = this.savedQueries.filter((query) => query.folder === folder.name).length
    return copy(folder)
  }

  async deleteSavedQueryFolder(id: string, policy: 'reject' | 'move' = 'reject', destination = '', signal?: AbortSignal) {
    await this.wait(signal, true)
    const folder = this.savedQueryFolders.find((item) => item.id === id)
    if (!folder) throw new GatewayError('Saved query folder was not found', 'not_found', 404)
    const queries = this.savedQueries.filter((query) => query.folder === folder.name)
    if (queries.length && policy === 'reject') throw new GatewayError('Saved query folder is not empty', 'conflict', 409)
    if (queries.length) queries.forEach((query) => { query.folder = destination || 'General' })
    this.savedQueryFolders = this.savedQueryFolders.filter((item) => item.id !== id)
  }

  private ensureFolder(name: string) {
    if (this.savedQueryFolders.some((folder) => folder.name === name)) return
    const now = new Date().toISOString()
    this.savedQueryFolders.push({ id: this.id('folder'), name, position: this.savedQueryFolders.length, queryCount: 0, createdAt: now, updatedAt: now })
  }
}

export function cloneMockTableFixtures(): Record<string, MockTableFixture> {
  return copy(mockTables)
}
