import { ENV } from './env'

const DEFAULT_DENSITY = 'compact' as const

const DENSITY_CONFIG = Object.freeze({
  compact: Object.freeze({
    dataRowHeight: 34,
    treeRowHeight: 28,
  }),
  comfortable: Object.freeze({
    dataRowHeight: 38,
    treeRowHeight: 30,
  }),
})

export const APP_CONFIG = Object.freeze({
  name: 'DataDock',
  api: Object.freeze({
    baseUrl: ENV.apiBaseUrl.replace(/\/$/, ''),
    prefix: '/api/v1',
    requestTimeoutMs: 75_000,
  }),
  dataSource: ENV.dataSource,
  databasePorts: Object.freeze({
    postgresql: 5432,
    mysql: 3306,
    mariadb: 3306,
    sqlite: 0,
    sqlserver: 1433,
    oracle: 1521,
    clickhouse: 8123,
    redis: 6379,
    mongodb: 27017,
  }),
  connection: Object.freeze({
    defaultHost: 'localhost',
    sshDefaultPort: 22,
    maxOpenConnections: 10,
    maxIdleConnections: 5,
    maxLifetimeSeconds: 300,
  }),
  layout: Object.freeze({
    inspectorInlineMinWidth: 960,
    inspectorWidth: 288,
  }),
  ui: Object.freeze({
    defaultDensity: DEFAULT_DENSITY,
    densities: DENSITY_CONFIG,
  }),
  table: Object.freeze({
    defaultPageSize: 50,
    pageSizes: Object.freeze([25, 50, 100, 200] as const),
    maxPageSize: 200,
    rowOverscan: 12,
    rowNumberWidth: 54,
    defaultColumnWidth: 176,
    minColumnWidth: 88,
    maxColumnWidth: 720,
    keyboardResizeStep: 16,
    autoSizeSampleRows: 100,
    preferencesDebounceMs: 180,
    columnPreferencesStoragePrefix: 'datadock:grid',
  }),
  explorer: Object.freeze({
    overscan: 10,
  }),
  catalog: Object.freeze({
    pageSize: 500,
  }),
  query: Object.freeze({
    fileName: 'query.sql',
    liveInitialSql: 'SELECT 1 AS connected;',
    mockInitialSql: `SELECT
  id,
  email,
  full_name,
  role,
  plan,
  mrr,
  created_at
FROM public.users
WHERE status = 'active'
ORDER BY created_at DESC
LIMIT 100;`,
    defaultTimeoutSeconds: 30,
    maxTimeoutSeconds: 60,
    historyLimit: 100,
    draftStorageKey: 'datadock:query:draft',
    draftSaveDebounceMs: 240,
  }),
  operations: Object.freeze({
    dashboardRefreshMs: 15_000,
    activityRefreshMs: 8_000,
    dashboardQuery: Object.freeze({ window: '15m', points: 30 }),
    sessionsQuery: Object.freeze({ limit: 200, state: 'all' }),
    locksQuery: Object.freeze({ limit: 200 }),
    performanceQuery: Object.freeze({ window: '1h', points: 60, limit: 100 }),
  }),
  cache: Object.freeze({
    staleTimeMs: 30_000,
    catalogStaleTimeMs: 60_000,
    gcTimeMs: 10 * 60_000,
  }),
  mock: Object.freeze({
    latencyMs: 180,
    mutationLatencyMs: 260,
  }),
  features: Object.freeze({
    commandPalette: true,
    chartResults: true,
    erDiagram: true,
    stagedEditing: true,
  }),
})
