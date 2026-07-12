import type { Connection, ConnectionTestResult, CreateConnectionInput, UpdateConnectionInput } from '../entities/connection'
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
  ExecuteQueryInput,
  QueryHistoryItem,
  QueryResult,
  SavedQuery,
  TransactionAction,
  TransactionState,
} from '../entities/query'
import type { CreateWorkspaceInput, UpdateWorkspaceInput, Workspace } from '../entities/workspace'
import type { DataSource } from '../shared/config/env'

export class GatewayError extends Error {
  readonly code: string
  readonly status?: number

  constructor(message: string, code = 'gateway_error', status?: number) {
    super(message)
    this.name = 'GatewayError'
    this.code = code
    this.status = status
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
  testConnection(id: string, signal?: AbortSignal): Promise<ConnectionTestResult>
  listCatalog(connectionId: string, signal?: AbortSignal): Promise<CatalogTree>
  getTableRows(connectionId: string, table: string, input?: TableRowsInput, signal?: AbortSignal): Promise<TableRowsResult>
  getTableSchema(connectionId: string, table: string, signal?: AbortSignal): Promise<TableSchema>
  getTableDDL(connectionId: string, table: string, signal?: AbortSignal): Promise<string>
  mutateTableRows(connectionId: string, table: string, mutations: RowMutation[], signal?: AbortSignal): Promise<TableMutateResult>
  createTable(connectionId: string, input: CreateTableInput, signal?: AbortSignal): Promise<void>
  alterTable(connectionId: string, table: string, actions: TableAlteration[], signal?: AbortSignal): Promise<void>
  createIndex(connectionId: string, table: string, input: CreateIndexInput, signal?: AbortSignal): Promise<void>
  dropIndex(connectionId: string, table: string, index: string, signal?: AbortSignal): Promise<void>
  executeQuery(input: ExecuteQueryInput, signal?: AbortSignal): Promise<QueryResult>
  listQueryHistory(connectionId?: string, signal?: AbortSignal): Promise<QueryHistoryItem[]>
  clearQueryHistory(connectionId?: string, signal?: AbortSignal): Promise<void>
  beginTransaction(connectionId: string, signal?: AbortSignal): Promise<TransactionState>
  transactionAction(id: string, action: TransactionAction, name?: string, signal?: AbortSignal): Promise<TransactionState>
  getDashboard(connectionId: string, signal?: AbortSignal): Promise<DatabaseDashboard>
  getSessions(connectionId: string, signal?: AbortSignal): Promise<DatabaseSessions>
  cancelSession(connectionId: string, sessionId: string, force?: boolean, signal?: AbortSignal): Promise<void>
  getLocks(connectionId: string, signal?: AbortSignal): Promise<DatabaseLocks>
  getPerformance(connectionId: string, signal?: AbortSignal): Promise<DatabasePerformance>
  listSavedQueries(signal?: AbortSignal): Promise<SavedQuery[]>
  createSavedQuery(input: CreateSavedQueryInput, signal?: AbortSignal): Promise<SavedQuery>
  deleteSavedQuery(id: string, signal?: AbortSignal): Promise<void>
}
