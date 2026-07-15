import type { ApiCatalogTree, ApiTableMutationResult, ApiTableRowsResult, ApiTableSchema } from './contracts/catalog'
import type { ApiConnection, ApiConnectionStatus, ApiConnectionTestResult } from './contracts/connection'
import type { ApiOperationsDashboard, ApiOperationsLocks, ApiOperationsPerformance, ApiOperationsSessions } from './contracts/operations'
import type { ApiQueryExecutionResult, ApiQueryHistoryItem } from './contracts/query'
import type { ApiPublicSavedQuery, ApiSavedQuery, ApiSavedQueryFolder, ApiSavedQueryShare } from './contracts/saved-query'
import type { ApiSchemaApplyResult, ApiSchemaPreview } from './contracts/schema'
import type { ApiTransaction } from './contracts/transaction'
import type { ApiWorkspace } from './contracts/workspace'
import type { ConnectionInput } from '../entities/connection'
import type {
  CreateIndexInput,
  CreateTableInput,
  RowMutation,
  TableAlteration,
  TableRowsInput,
} from '../entities/database-object'
import type {
  CreateSavedQueryFolderInput,
  CreateSavedQueryInput,
  ExecuteQueryInput,
  ExplainQueryInput,
  TransactionAction,
  UpdateSavedQueryFolderInput,
  UpdateSavedQueryInput,
} from '../entities/query'
import type { SchemaAction, SchemaApplyInput, SchemaTarget } from '../entities/schema'
import type { CreateWorkspaceInput, UpdateWorkspaceInput } from '../entities/workspace'
import { API_ENDPOINTS, withSearch } from '../shared/config/api-endpoints'
import { APP_CONFIG } from '../shared/config/constants'
import { type DataDockGateway, GatewayError } from './gateway'
import { ApiClient } from './http-client'
import { mapCatalog } from './mappers/catalog'
import { mapConnection, mapConnectionStatus, mapConnectionTest, toConnectionRequest } from './mappers/connection'
import { mapDashboard, mapLocks, mapPerformance, mapSessions } from './mappers/operations'
import { mapQueryHistoryItem, mapQueryResult } from './mappers/query'
import { mapPublicSavedQuery, mapSavedQuery, mapSavedQueryFolder, mapSavedQueryShare } from './mappers/saved-query'
import { mapSchemaApplyResult, mapSchemaPreview } from './mappers/schema'
import { mapTableMutationResult, mapTableRows, mapTableSchema, toTableMutationsRequest, toTableRowsRequest } from './mappers/table'
import { mapTransaction } from './mappers/transaction'
import { mapWorkspace } from './mappers/workspace'

function json(body: unknown): RequestInit {
  return { body: JSON.stringify(body) }
}

function target(reference: string): SchemaTarget {
  const parts = reference.split('.')
  if (parts.length < 2) return { table: reference }
  return { schema: parts.slice(0, -1).join('.'), table: parts.at(-1) ?? reference }
}

export class HttpDataDockGateway implements DataDockGateway {
  readonly source = 'api' as const
  private readonly client = new ApiClient()
  private readonly transactionConnections = new Map<string, string>()
  private readonly catalogReferences = new Map<string, Map<string, string>>()

  async listWorkspaces(signal?: AbortSignal) {
    const values = await this.client.request<ApiWorkspace[]>(API_ENDPOINTS.workspaces, { signal })
    return values.map(mapWorkspace)
  }

  async createWorkspace(input: CreateWorkspaceInput, signal?: AbortSignal) {
    const value = await this.client.request<ApiWorkspace>(API_ENDPOINTS.workspaces, { method: 'POST', ...json(input), signal })
    return mapWorkspace(value)
  }

  async updateWorkspace(id: string, input: UpdateWorkspaceInput, signal?: AbortSignal) {
    const value = await this.client.request<ApiWorkspace>(API_ENDPOINTS.workspace(id), { method: 'PATCH', ...json(input), signal })
    return mapWorkspace(value)
  }

  deleteWorkspace(id: string, signal?: AbortSignal) {
    return this.client.request<void>(API_ENDPOINTS.workspace(id), { method: 'DELETE', signal })
  }

  reorderWorkspaces(ids: string[], signal?: AbortSignal) {
    return this.client.request<void>(API_ENDPOINTS.workspaceOrder, { method: 'PUT', ...json({ ids }), signal })
  }

  async listConnections(workspaceId?: string, signal?: AbortSignal) {
    const values = await this.client.request<ApiConnection[]>(withSearch(API_ENDPOINTS.connections, { workspaceId }), { signal })
    return values.map(mapConnection)
  }

  async getConnection(id: string, signal?: AbortSignal) {
    return mapConnection(await this.client.request<ApiConnection>(API_ENDPOINTS.connection(id), { signal }))
  }

  async createConnection(input: ConnectionInput, signal?: AbortSignal) {
    const value = await this.client.request<ApiConnection>(API_ENDPOINTS.connections, { method: 'POST', ...json(toConnectionRequest(input)), signal })
    return mapConnection(value)
  }

  async updateConnection(id: string, input: ConnectionInput, signal?: AbortSignal) {
    const value = await this.client.request<ApiConnection>(API_ENDPOINTS.connection(id), { method: 'PATCH', ...json(toConnectionRequest(input)), signal })
    return mapConnection(value)
  }

  deleteConnection(id: string, signal?: AbortSignal) {
    return this.client.request<void>(API_ENDPOINTS.connection(id), { method: 'DELETE', signal })
  }

  async duplicateConnection(id: string, signal?: AbortSignal) {
    const value = await this.client.request<ApiConnection>(API_ENDPOINTS.duplicateConnection(id), { method: 'POST', signal })
    return mapConnection(value)
  }

  async testConnectionDraft(input: ConnectionInput, signal?: AbortSignal) {
    const value = await this.client.request<ApiConnectionTestResult>(API_ENDPOINTS.testConnectionDraft, { method: 'POST', ...json(toConnectionRequest(input)), signal })
    return mapConnectionTest(value)
  }

  async testConnection(id: string, signal?: AbortSignal) {
    const value = await this.client.request<ApiConnectionTestResult>(API_ENDPOINTS.testConnection(id), { method: 'POST', signal })
    return mapConnectionTest(value)
  }

  async connectConnection(id: string, signal?: AbortSignal) {
    const value = await this.client.request<ApiConnection>(API_ENDPOINTS.connectConnection(id), { method: 'POST', signal })
    return mapConnection(value)
  }

  async disconnectConnection(id: string, signal?: AbortSignal) {
    const value = await this.client.request<ApiConnection>(API_ENDPOINTS.disconnectConnection(id), { method: 'POST', signal })
    return mapConnection(value)
  }

  async getConnectionStatus(id: string, signal?: AbortSignal) {
    const value = await this.client.request<ApiConnectionStatus>(API_ENDPOINTS.connectionStatus(id), { signal })
    return mapConnectionStatus(value)
  }

  async setConnectionFavorite(id: string, favorite: boolean, signal?: AbortSignal) {
    const value = await this.client.request<ApiConnection>(API_ENDPOINTS.connectionFavorite(id), { method: 'PATCH', ...json({ favorite }), signal })
    return mapConnection(value)
  }

  async listCatalog(connectionId: string, signal?: AbortSignal) {
    const endpoint = withSearch(API_ENDPOINTS.catalog(connectionId), { depth: 'all', limit: APP_CONFIG.catalog.pageSize })
    const catalog = mapCatalog(await this.client.request<ApiCatalogTree>(endpoint, { signal }))
    const references = new Map<string, string>()
    const visit = (objects: typeof catalog.databases) => objects.forEach((object) => {
      if (object.reference) {
        references.set(object.reference, object.reference)
        references.set(object.qualifiedName, object.reference)
      }
      if (object.children) visit(object.children)
    })
    visit(catalog.databases)
    this.catalogReferences.set(connectionId, references)
    return catalog
  }

  async getTableRows(connectionId: string, reference: string, input: TableRowsInput = {}, signal?: AbortSignal) {
    const pageSize = Math.min(Math.max(1, input.pageSize ?? APP_CONFIG.table.defaultPageSize), APP_CONFIG.table.maxPageSize)
    const page = Math.max(1, input.page ?? 1)
    const request = toTableRowsRequest(await this.resolveTableReference(connectionId, reference, signal), input, pageSize, page)
    const value = await this.client.request<ApiTableRowsResult>(API_ENDPOINTS.tableRows(connectionId), { method: 'POST', ...json(request), signal })
    return mapTableRows(value)
  }

  async getTableSchema(connectionId: string, reference: string, signal?: AbortSignal) {
    const endpoint = withSearch(API_ENDPOINTS.tableSchema(connectionId), { reference: await this.resolveTableReference(connectionId, reference, signal) })
    return mapTableSchema(await this.client.request<ApiTableSchema>(endpoint, { signal }))
  }

  async getTableDDL(connectionId: string, reference: string, signal?: AbortSignal) {
    const endpoint = withSearch(API_ENDPOINTS.tableDDL(connectionId), { reference: await this.resolveTableReference(connectionId, reference, signal) })
    return (await this.client.request<{ ddl: string }>(endpoint, { signal })).ddl
  }

  mutateTableRows(connectionId: string, reference: string, mutations: RowMutation[], signal?: AbortSignal) {
    return this.applyTableMutations(connectionId, reference, mutations, undefined, signal)
  }

  mutateTableRowsInTransaction(connectionId: string, reference: string, mutations: RowMutation[], transactionId: string, signal?: AbortSignal) {
    return this.applyTableMutations(connectionId, reference, mutations, transactionId, signal)
  }

  async createTable(connectionId: string, input: CreateTableInput, signal?: AbortSignal) {
    await this.previewAndApply(connectionId, [{
      kind: 'create_table',
      target: { schema: input.schema, table: input.name },
      columns: input.columns.map((column) => ({ ...column })),
    }], signal)
  }

  async alterTable(connectionId: string, reference: string, actions: TableAlteration[], signal?: AbortSignal) {
    const schemaTarget = target(reference)
    const schemaActions: SchemaAction[] = actions.map((action) => {
      if (action.kind === 'add_column') return { kind: 'add_column', target: schemaTarget, column: action.definition }
      if (action.kind === 'drop_column') return { kind: 'drop_column', target: schemaTarget, name: action.column, cascade: false }
      if (action.kind === 'rename_column') return { kind: 'rename_column', target: schemaTarget, name: action.column, newName: action.newName }
      if (action.kind === 'change_type') return { kind: 'alter_column_type', target: schemaTarget, name: action.column, dataType: action.definition?.dataType }
      if (action.kind === 'set_nullable') return { kind: 'set_column_nullable', target: schemaTarget, name: action.column, nullable: action.definition?.nullable }
      if (action.kind === 'set_default') return { kind: 'set_column_default', target: schemaTarget, name: action.column, defaultValue: action.definition?.defaultValue }
      return { kind: 'rename_table', target: schemaTarget, newName: action.newName }
    })
    await this.previewAndApply(connectionId, schemaActions, signal)
  }

  async createIndex(connectionId: string, reference: string, input: CreateIndexInput, signal?: AbortSignal) {
    await this.previewAndApply(connectionId, [{ kind: 'create_index', target: target(reference), index: { ...input } }], signal)
  }

  async dropIndex(connectionId: string, reference: string, index: string, signal?: AbortSignal) {
    await this.previewAndApply(connectionId, [{ kind: 'drop_index', target: target(reference), name: index }], signal)
  }

  async executeQuery(input: ExecuteQueryInput, signal?: AbortSignal) {
    const executionId = input.executionId ?? crypto.randomUUID()
    const value = await this.client.request<ApiQueryExecutionResult>(API_ENDPOINTS.executeQuery, {
      method: 'POST',
      ...json({ ...input, executionId }),
      signal,
    })
    return mapQueryResult(value)
  }

  async explainQuery(input: ExplainQueryInput, signal?: AbortSignal) {
    const executionId = input.executionId ?? crypto.randomUUID()
    const value = await this.client.request<ApiQueryExecutionResult>(API_ENDPOINTS.explainQuery, {
      method: 'POST',
      ...json({ ...input, executionId, analyze: input.analyze ?? false }),
      signal,
    })
    return mapQueryResult(value)
  }

  cancelQuery(executionId: string, signal?: AbortSignal) {
    return this.client.request<void>(API_ENDPOINTS.cancelQuery(executionId), { method: 'POST', signal })
  }

  async listQueryHistory(connectionId?: string, signal?: AbortSignal) {
    const values = await this.client.request<ApiQueryHistoryItem[]>(withSearch(API_ENDPOINTS.queryHistory, { connectionId }), { signal })
    return values.map(mapQueryHistoryItem)
  }

  clearQueryHistory(connectionId?: string, signal?: AbortSignal) {
    return this.client.request<void>(withSearch(API_ENDPOINTS.queryHistory, { connectionId }), { method: 'DELETE', signal })
  }

  deleteQueryHistory(ids: string[], signal?: AbortSignal) {
    return this.client.request<void>(API_ENDPOINTS.deleteQueryHistory, { method: 'POST', ...json({ ids }), signal })
  }

  async beginTransaction(connectionId: string, signal?: AbortSignal) {
    const value = await this.client.request<ApiTransaction>(API_ENDPOINTS.transactions(connectionId), { method: 'POST', signal })
    this.transactionConnections.set(value.id, connectionId)
    return mapTransaction(value)
  }

  async getTransaction(connectionId: string, id: string, signal?: AbortSignal) {
    const value = await this.client.request<ApiTransaction>(API_ENDPOINTS.transaction(connectionId, id), { signal })
    this.transactionConnections.set(value.id, connectionId)
    return mapTransaction(value)
  }

  async transactionAction(id: string, action: TransactionAction, name?: string, signal?: AbortSignal) {
    const connectionId = this.transactionConnections.get(id)
    if (!connectionId) throw new GatewayError('The transaction connection is unavailable', 'transaction_context_missing')
    let endpoint: string
    let method = 'POST'
    let body: RequestInit = {}
    if (action === 'commit' || action === 'rollback') endpoint = API_ENDPOINTS.transactionAction(connectionId, id, action)
    else if (action === 'savepoint') {
      endpoint = API_ENDPOINTS.transactionSavepoints(connectionId, id)
      body = json({ name })
    } else if (action === 'rollback_to') endpoint = API_ENDPOINTS.transactionSavepointRollback(connectionId, id, name ?? '')
    else {
      endpoint = API_ENDPOINTS.transactionSavepoint(connectionId, id, name ?? '')
      method = 'DELETE'
    }
    const value = await this.client.request<ApiTransaction>(endpoint, { method, ...body, signal })
    if (value.state !== 'active') this.transactionConnections.delete(id)
    return mapTransaction(value)
  }

  async previewSchema(connectionId: string, actions: SchemaAction[], signal?: AbortSignal) {
    const value = await this.client.request<ApiSchemaPreview>(API_ENDPOINTS.schemaPreview(connectionId), { method: 'POST', ...json({ actions }), signal })
    return mapSchemaPreview(value)
  }

  async applySchema(connectionId: string, input: SchemaApplyInput, signal?: AbortSignal) {
    const value = await this.client.request<ApiSchemaApplyResult>(API_ENDPOINTS.schemaApply(connectionId), { method: 'POST', ...json(input), signal })
    return mapSchemaApplyResult(value)
  }

  async getDashboard(connectionId: string, signal?: AbortSignal) {
    const endpoint = withSearch(API_ENDPOINTS.dashboard(connectionId), APP_CONFIG.operations.dashboardQuery)
    return mapDashboard(await this.client.request<ApiOperationsDashboard>(endpoint, { signal }))
  }

  async getSessions(connectionId: string, signal?: AbortSignal) {
    const endpoint = withSearch(API_ENDPOINTS.sessions(connectionId), APP_CONFIG.operations.sessionsQuery)
    return mapSessions(await this.client.request<ApiOperationsSessions>(endpoint, { signal }))
  }

  cancelSession(connectionId: string, sessionId: string, force = false, signal?: AbortSignal) {
    return this.client.request<void>(API_ENDPOINTS.controlSession(connectionId, sessionId), {
      method: 'POST',
      ...json({ action: force ? 'terminate' : 'cancel' }),
      signal,
    })
  }

  async getLocks(connectionId: string, signal?: AbortSignal) {
    const endpoint = withSearch(API_ENDPOINTS.locks(connectionId), APP_CONFIG.operations.locksQuery)
    return mapLocks(await this.client.request<ApiOperationsLocks>(endpoint, { signal }))
  }

  async getPerformance(connectionId: string, signal?: AbortSignal) {
    const endpoint = withSearch(API_ENDPOINTS.performance(connectionId), APP_CONFIG.operations.performanceQuery)
    return mapPerformance(await this.client.request<ApiOperationsPerformance>(endpoint, { signal }))
  }

  async listSavedQueries(signal?: AbortSignal) {
    const values = await this.client.request<ApiSavedQuery[]>(API_ENDPOINTS.savedQueries, { signal })
    return values.map(mapSavedQuery)
  }

  async getSavedQuery(id: string, signal?: AbortSignal) {
    return mapSavedQuery(await this.client.request<ApiSavedQuery>(API_ENDPOINTS.savedQuery(id), { signal }))
  }

  async createSavedQuery(input: CreateSavedQueryInput, signal?: AbortSignal) {
    const value = await this.client.request<ApiSavedQuery>(API_ENDPOINTS.savedQueries, { method: 'POST', ...json(input), signal })
    return mapSavedQuery(value)
  }

  async updateSavedQuery(id: string, input: UpdateSavedQueryInput, signal?: AbortSignal) {
    const value = await this.client.request<ApiSavedQuery>(API_ENDPOINTS.savedQuery(id), { method: 'PATCH', ...json(input), signal })
    return mapSavedQuery(value)
  }

  async duplicateSavedQuery(id: string, signal?: AbortSignal) {
    const value = await this.client.request<ApiSavedQuery>(API_ENDPOINTS.duplicateSavedQuery(id), { method: 'POST', signal })
    return mapSavedQuery(value)
  }

  async setSavedQueryFavorite(id: string, favorite: boolean, signal?: AbortSignal) {
    const value = await this.client.request<ApiSavedQuery>(API_ENDPOINTS.favoriteSavedQuery(id), { method: 'PATCH', ...json({ favorite }), signal })
    return mapSavedQuery(value)
  }

  deleteSavedQuery(id: string, signal?: AbortSignal) {
    return this.client.request<void>(API_ENDPOINTS.savedQuery(id), { method: 'DELETE', signal })
  }

  async getSavedQueryShare(id: string, signal?: AbortSignal) {
    try {
      const value = await this.client.request<ApiSavedQueryShare>(API_ENDPOINTS.savedQueryShare(id), { signal })
      return mapSavedQueryShare(value)
    } catch (error) {
      if (error instanceof GatewayError && error.status === 404) return null
      throw error
    }
  }

  async shareSavedQuery(id: string, expiresAt?: string, signal?: AbortSignal) {
    const value = await this.client.request<ApiSavedQueryShare>(API_ENDPOINTS.savedQueryShare(id), { method: 'POST', ...json({ expiresAt }), signal })
    return mapSavedQueryShare(value)
  }

  revokeSavedQueryShare(id: string, signal?: AbortSignal) {
    return this.client.request<void>(API_ENDPOINTS.savedQueryShare(id), { method: 'DELETE', signal })
  }

  async getSharedQuery(code: string, signal?: AbortSignal) {
    return mapPublicSavedQuery(await this.client.request<ApiPublicSavedQuery>(API_ENDPOINTS.sharedQuery(code), { signal }))
  }

  async listSavedQueryFolders(signal?: AbortSignal) {
    const values = await this.client.request<ApiSavedQueryFolder[]>(API_ENDPOINTS.savedQueryFolders, { signal })
    return values.map(mapSavedQueryFolder)
  }

  async createSavedQueryFolder(input: CreateSavedQueryFolderInput, signal?: AbortSignal) {
    const value = await this.client.request<ApiSavedQueryFolder>(API_ENDPOINTS.savedQueryFolders, { method: 'POST', ...json({ ...input, position: input.position ?? 0 }), signal })
    return mapSavedQueryFolder(value)
  }

  async updateSavedQueryFolder(id: string, input: UpdateSavedQueryFolderInput, signal?: AbortSignal) {
    const value = await this.client.request<ApiSavedQueryFolder>(API_ENDPOINTS.savedQueryFolder(id), { method: 'PATCH', ...json(input), signal })
    return mapSavedQueryFolder(value)
  }

  deleteSavedQueryFolder(id: string, policy: 'reject' | 'move' = 'reject', destination = '', signal?: AbortSignal) {
    return this.client.request<void>(API_ENDPOINTS.savedQueryFolder(id), { method: 'DELETE', ...json({ policy, destination }), signal })
  }

  private async applyTableMutations(connectionId: string, reference: string, mutations: RowMutation[], transactionId?: string, signal?: AbortSignal) {
    const resolvedReference = await this.resolveTableReference(connectionId, reference, signal)
    const request = toTableMutationsRequest(resolvedReference, mutations, transactionId)
    const value = await this.client.request<ApiTableMutationResult>(API_ENDPOINTS.tableMutations(connectionId), { method: 'POST', ...json(request), signal })
    return mapTableMutationResult(value)
  }

  private async previewAndApply(connectionId: string, actions: SchemaAction[], signal?: AbortSignal) {
    const preview = await this.previewSchema(connectionId, actions, signal)
    await this.applySchema(connectionId, { actions: preview.actions, previewHash: preview.hash, confirmDestructive: preview.destructive }, signal)
  }

  private async resolveTableReference(connectionId: string, value: string, signal?: AbortSignal) {
    const cached = this.catalogReferences.get(connectionId)?.get(value)
    if (cached) return cached
    await this.listCatalog(connectionId, signal)
    const resolved = this.catalogReferences.get(connectionId)?.get(value)
    if (!resolved) throw new GatewayError('The table is no longer present in the loaded catalog', 'catalog_reference_missing', 404)
    return resolved
  }
}
