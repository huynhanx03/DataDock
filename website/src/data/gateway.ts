import type { Connection, ConnectionRuntimeStatus, ConnectionTestResult, CreateConnectionInput, UpdateConnectionInput } from '../entities/connection'
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
import type { DataSource } from '../shared/config/env'

export class GatewayError extends Error {
  readonly code: string
  readonly status?: number
  readonly requestId?: string

  constructor(message: string, code = 'gateway_error', status?: number, requestId?: string) {
    super(message)
    this.name = 'GatewayError'
    this.code = code
    this.status = status
    this.requestId = requestId
  }
}

export interface DataDockGateway {
  readonly source: DataSource
  listWorkspaces(signal?: AbortSignal): Promise<Workspace[]>
  createWorkspace(input: CreateWorkspaceInput, signal?: AbortSignal): Promise<Workspace>
  updateWorkspace(id: string, input: UpdateWorkspaceInput, signal?: AbortSignal): Promise<Workspace>
  deleteWorkspace(id: string, signal?: AbortSignal): Promise<void>
  reorderWorkspaces(ids: string[], signal?: AbortSignal): Promise<void>
  listConnections(workspaceId?: string, signal?: AbortSignal): Promise<Connection[]>
  getConnection(id: string, signal?: AbortSignal): Promise<Connection>
  createConnection(input: CreateConnectionInput, signal?: AbortSignal): Promise<Connection>
  updateConnection(id: string, input: UpdateConnectionInput, signal?: AbortSignal): Promise<Connection>
  deleteConnection(id: string, signal?: AbortSignal): Promise<void>
  duplicateConnection(id: string, signal?: AbortSignal): Promise<Connection>
  testConnectionDraft(input: CreateConnectionInput, signal?: AbortSignal): Promise<ConnectionTestResult>
  testConnection(id: string, signal?: AbortSignal): Promise<ConnectionTestResult>
  connectConnection(id: string, signal?: AbortSignal): Promise<Connection>
  disconnectConnection(id: string, signal?: AbortSignal): Promise<Connection>
  getConnectionStatus(id: string, signal?: AbortSignal): Promise<ConnectionRuntimeStatus>
  setConnectionFavorite(id: string, favorite: boolean, signal?: AbortSignal): Promise<Connection>
  listCatalog(connectionId: string, signal?: AbortSignal): Promise<CatalogTree>
  getTableRows(connectionId: string, table: string, input?: TableRowsInput, signal?: AbortSignal): Promise<TableRowsResult>
  getTableSchema(connectionId: string, table: string, signal?: AbortSignal): Promise<TableSchema>
  getTableDDL(connectionId: string, table: string, signal?: AbortSignal): Promise<string>
  mutateTableRows(connectionId: string, table: string, mutations: RowMutation[], signal?: AbortSignal): Promise<TableMutateResult>
  mutateTableRowsInTransaction(connectionId: string, table: string, mutations: RowMutation[], transactionId: string, signal?: AbortSignal): Promise<TableMutateResult>
  createTable(connectionId: string, input: CreateTableInput, signal?: AbortSignal): Promise<void>
  alterTable(connectionId: string, table: string, actions: TableAlteration[], signal?: AbortSignal): Promise<void>
  createIndex(connectionId: string, table: string, input: CreateIndexInput, signal?: AbortSignal): Promise<void>
  dropIndex(connectionId: string, table: string, index: string, signal?: AbortSignal): Promise<void>
  executeQuery(input: ExecuteQueryInput, signal?: AbortSignal): Promise<QueryResult>
  explainQuery(input: ExplainQueryInput, signal?: AbortSignal): Promise<QueryResult>
  cancelQuery(executionId: string, signal?: AbortSignal): Promise<void>
  listQueryHistory(connectionId?: string, signal?: AbortSignal): Promise<QueryHistoryItem[]>
  clearQueryHistory(connectionId?: string, signal?: AbortSignal): Promise<void>
  deleteQueryHistory(ids: string[], signal?: AbortSignal): Promise<void>
  beginTransaction(connectionId: string, signal?: AbortSignal): Promise<TransactionState>
  getTransaction(connectionId: string, id: string, signal?: AbortSignal): Promise<TransactionState>
  transactionAction(id: string, action: TransactionAction, name?: string, signal?: AbortSignal): Promise<TransactionState>
  previewSchema(connectionId: string, actions: SchemaAction[], signal?: AbortSignal): Promise<SchemaPreview>
  applySchema(connectionId: string, input: SchemaApplyInput, signal?: AbortSignal): Promise<SchemaApplyResult>
  getDashboard(connectionId: string, signal?: AbortSignal): Promise<DatabaseDashboard>
  getSessions(connectionId: string, signal?: AbortSignal): Promise<DatabaseSessions>
  cancelSession(connectionId: string, sessionId: string, force?: boolean, signal?: AbortSignal): Promise<void>
  getLocks(connectionId: string, signal?: AbortSignal): Promise<DatabaseLocks>
  getPerformance(connectionId: string, signal?: AbortSignal): Promise<DatabasePerformance>
  listSavedQueries(signal?: AbortSignal): Promise<SavedQuery[]>
  getSavedQuery(id: string, signal?: AbortSignal): Promise<SavedQuery>
  createSavedQuery(input: CreateSavedQueryInput, signal?: AbortSignal): Promise<SavedQuery>
  updateSavedQuery(id: string, input: UpdateSavedQueryInput, signal?: AbortSignal): Promise<SavedQuery>
  duplicateSavedQuery(id: string, signal?: AbortSignal): Promise<SavedQuery>
  setSavedQueryFavorite(id: string, favorite: boolean, signal?: AbortSignal): Promise<SavedQuery>
  deleteSavedQuery(id: string, signal?: AbortSignal): Promise<void>
  getSavedQueryShare(id: string, signal?: AbortSignal): Promise<SavedQueryShare | null>
  shareSavedQuery(id: string, expiresAt?: string, signal?: AbortSignal): Promise<SavedQueryShare>
  revokeSavedQueryShare(id: string, signal?: AbortSignal): Promise<void>
  getSharedQuery(code: string, signal?: AbortSignal): Promise<PublicSavedQuery>
  listSavedQueryFolders(signal?: AbortSignal): Promise<SavedQueryFolder[]>
  createSavedQueryFolder(input: CreateSavedQueryFolderInput, signal?: AbortSignal): Promise<SavedQueryFolder>
  updateSavedQueryFolder(id: string, input: UpdateSavedQueryFolderInput, signal?: AbortSignal): Promise<SavedQueryFolder>
  deleteSavedQueryFolder(id: string, policy?: 'reject' | 'move', destination?: string, signal?: AbortSignal): Promise<void>
}
