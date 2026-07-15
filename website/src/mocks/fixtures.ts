import type { Connection } from '../entities/connection'
import type {
  CatalogTree,
  DataColumn,
  DatabaseDashboard,
  DatabaseLocks,
  DatabaseObject,
  DatabasePerformance,
  DatabaseSessions,
  TableRow,
  TableSchema,
} from '../entities/database-object'
import type { QueryHistoryItem, SavedQuery } from '../entities/query'
import type { Workspace } from '../entities/workspace'

const FIXTURE_TIME = '2026-07-11T04:30:00.000Z'

const sshTunnel = {
  enabled: false,
  host: '',
  port: 22,
  username: '',
  privateKeyPath: '',
  knownHostsPath: '',
  hasPassword: false,
}

export const mockWorkspaces: Workspace[] = [
  { id: 'ws-primary', name: 'DataDock Studio', icon: 'D', color: '#7168f5', position: 0, collapsed: false, createdAt: '2026-05-18T09:00:00.000Z', updatedAt: FIXTURE_TIME },
  { id: 'ws-labs', name: 'Product Labs', icon: 'P', color: '#14b8a6', position: 1, collapsed: false, createdAt: '2026-06-04T09:00:00.000Z', updatedAt: '2026-07-10T15:40:00.000Z' },
]

export const mockConnections: Connection[] = [
  {
    id: 'conn-primary', workspaceId: 'ws-primary', name: 'Production PostgreSQL', engine: 'postgresql', host: 'db.internal.acme.test', port: 5432, database: 'datadock', username: 'readonly_app', sslMode: 'verify-full', sslCaPath: '/certs/ca.pem', sslCertPath: '', sslKeyPath: '', proxyUrl: '', sshTunnel, readOnly: false, autoReconnect: true, maxOpenConns: 20, maxIdleConns: 10, connMaxLifetimeSeconds: 600, favorite: true, hasPassword: true, status: 'connected', latencyMs: 18, lastConnectedAt: FIXTURE_TIME, createdAt: '2026-05-18T09:05:00.000Z', updatedAt: FIXTURE_TIME,
  },
  {
    id: 'conn-analytics', workspaceId: 'ws-primary', name: 'Analytics MySQL', engine: 'mysql', host: 'analytics.internal.acme.test', port: 3306, database: 'analytics', username: 'analyst', sslMode: 'require', sslCaPath: '', sslCertPath: '', sslKeyPath: '', proxyUrl: '', sshTunnel, readOnly: true, autoReconnect: true, maxOpenConns: 12, maxIdleConns: 6, connMaxLifetimeSeconds: 300, favorite: true, hasPassword: true, status: 'disconnected', latencyMs: 42, lastConnectedAt: '2026-07-10T12:14:22.000Z', createdAt: '2026-05-22T11:30:00.000Z', updatedAt: '2026-07-10T12:14:22.000Z',
  },
  {
    id: 'conn-staging', workspaceId: 'ws-labs', name: 'Staging MariaDB', engine: 'mariadb', host: 'mariadb.staging.local', port: 3306, database: 'product_staging', username: 'developer', sslMode: 'disable', sslCaPath: '', sslCertPath: '', sslKeyPath: '', proxyUrl: 'socks5://proxy.local:1080', sshTunnel: { ...sshTunnel, enabled: true, host: 'bastion.staging.local', username: 'deploy', knownHostsPath: '/home/datadock/.ssh/known_hosts' }, readOnly: false, autoReconnect: false, maxOpenConns: 8, maxIdleConns: 4, connMaxLifetimeSeconds: 300, favorite: false, hasPassword: true, status: 'error', latencyMs: 86, lastConnectedAt: '2026-07-09T08:02:00.000Z', createdAt: '2026-06-04T09:10:00.000Z', updatedAt: '2026-07-09T08:02:00.000Z',
  },
]

type SchemaObjects = {
  tables: string[]
  views?: string[]
  materializedViews?: string[]
  functions?: string[]
  procedures?: string[]
  sequences?: string[]
}

function leaf(connectionId: string, parentId: string, schema: string, name: string, kind: DatabaseObject['kind']): DatabaseObject {
  return { id: `${connectionId}:${kind}:${schema}.${name}`, connectionId, parentId, name, qualifiedName: `${schema}.${name}`, kind, schema }
}

function objectGroup(connectionId: string, schemaId: string, schema: string, name: string, kind: DatabaseObject['kind'], items: string[]): DatabaseObject {
  const id = `${schemaId}:group:${name.toLowerCase().replaceAll(' ', '-')}`
  return { id, connectionId, parentId: schemaId, name, qualifiedName: `${schema}.${name}`, kind: 'group', schema, count: items.length, children: items.map((item) => leaf(connectionId, id, schema, item, kind)) }
}

function buildCatalog(connection: Connection, schemas: Record<string, SchemaObjects>): CatalogTree {
  const database = connection.database || connection.host
  const databaseId = `${connection.id}:database:${database}`
  const schemaNodes = Object.entries(schemas).map(([schema, objects]) => {
    const schemaId = `${connection.id}:schema:${schema}`
    const groups = [
      objectGroup(connection.id, schemaId, schema, 'Tables', 'table', objects.tables),
      objectGroup(connection.id, schemaId, schema, 'Views', 'view', objects.views ?? []),
      objectGroup(connection.id, schemaId, schema, 'Materialized Views', 'materialized-view', objects.materializedViews ?? []),
      objectGroup(connection.id, schemaId, schema, 'Functions', 'function', objects.functions ?? []),
      objectGroup(connection.id, schemaId, schema, 'Procedures', 'procedure', objects.procedures ?? []),
      objectGroup(connection.id, schemaId, schema, 'Sequences', 'sequence', objects.sequences ?? []),
    ]
    return { id: schemaId, connectionId: connection.id, parentId: databaseId, name: schema, qualifiedName: schema, kind: 'schema' as const, database, schema, children: groups }
  })
  return { connectionId: connection.id, engine: connection.engine, databases: [{ id: databaseId, connectionId: connection.id, name: database, qualifiedName: database, kind: 'database', database, children: schemaNodes }], loadedAt: FIXTURE_TIME }
}

export const mockCatalogs: Record<string, CatalogTree> = {
  'conn-primary': buildCatalog(mockConnections[0], {
    public: { tables: ['users', 'orders', 'organizations', 'products', 'subscriptions', 'audit_events'], views: ['active_subscriptions', 'monthly_revenue'], materializedViews: ['account_health'], functions: ['search_users', 'calculate_mrr'], sequences: ['order_number_seq'] },
    billing: { tables: ['invoices', 'payment_methods', 'refunds'], views: ['outstanding_invoices'], functions: ['next_invoice_number'] },
  }),
  'conn-analytics': buildCatalog(mockConnections[1], {
    analytics: { tables: ['events', 'sessions', 'daily_metrics', 'funnels'], views: ['retention_cohorts', 'revenue_attribution'], procedures: ['refresh_daily_metrics'] },
  }),
  'conn-staging': buildCatalog(mockConnections[2], {
    product_staging: { tables: ['users', 'projects', 'feature_flags', 'jobs'], views: ['active_jobs'], procedures: ['reset_demo_data'] },
  }),
}

const firstNames = ['Avery', 'Mia', 'Noah', 'Sofia', 'Ethan', 'Lina', 'Theo', 'Ivy', 'Mason', 'Aria', 'Leo', 'Nora']
const lastNames = ['Nguyen', 'Tran', 'Patel', 'Kim', 'Garcia', 'Martin', 'Brown', 'Wilson', 'Chen', 'Taylor', 'Anderson', 'Lee']
const roles = ['owner', 'admin', 'developer', 'analyst', 'viewer']
const plans = ['Enterprise', 'Scale', 'Pro', 'Starter']
const userStatuses = ['active', 'active', 'active', 'invited', 'suspended']

function generateUsers(count: number): TableRow[] {
  return Array.from({ length: count }, (_, index) => {
    const first = firstNames[index % firstNames.length]
    const last = lastNames[(index * 5) % lastNames.length]
    const created = new Date(Date.UTC(2025, 0, 3 + index, 9 + index % 8, index % 60))
    const lastSeen = new Date(Date.UTC(2026, 6, 11 - index % 21, 2 + index % 17, index * 7 % 60))
    return {
      id: `usr_${String(index + 1).padStart(5, '0')}`,
      email: `${first}.${last}${index + 1}@example.com`.toLowerCase(),
      full_name: `${first} ${last}`,
      role: roles[index % roles.length],
      status: userStatuses[index % userStatuses.length],
      plan: plans[index % plans.length],
      verified: index % 7 !== 0,
      mrr: Number((29 + (index % 9) * 37.5).toFixed(2)),
      created_at: created.toISOString(),
      last_seen_at: index % 11 === 0 ? null : lastSeen.toISOString(),
    }
  })
}

const orderStatuses = ['paid', 'paid', 'processing', 'refunded', 'failed']

function generateOrders(count: number, users: TableRow[]): TableRow[] {
  return Array.from({ length: count }, (_, index) => {
    const created = new Date(Date.UTC(2026, 4 + index % 3, 1 + index % 27, 7 + index % 12, index * 11 % 60))
    const subtotal = 24 + (index * 17) % 780
    const tax = Number((subtotal * 0.08).toFixed(2))
    return {
      id: `ord_${String(10482 + index).padStart(6, '0')}`,
      user_id: users[(index * 7) % users.length].id,
      status: orderStatuses[index % orderStatuses.length],
      subtotal: Number(subtotal.toFixed(2)),
      tax,
      total: Number((subtotal + tax).toFixed(2)),
      currency: index % 13 === 0 ? 'EUR' : 'USD',
      items: 1 + index % 6,
      channel: ['web', 'api', 'mobile'][index % 3],
      created_at: created.toISOString(),
    }
  })
}

const userRows = generateUsers(128)
const orderRows = generateOrders(220, userRows)

const userColumns: DataColumn[] = [
  { name: 'id', type: 'uuid', nullable: false }, { name: 'email', type: 'varchar', nullable: false }, { name: 'full_name', type: 'varchar', nullable: false }, { name: 'role', type: 'user_role', nullable: false, enumValues: ['owner', 'admin', 'developer', 'analyst', 'viewer'] }, { name: 'status', type: 'varchar', nullable: false, enumValues: ['active', 'invited', 'suspended'] }, { name: 'plan', type: 'varchar', nullable: false, enumValues: ['Starter', 'Pro', 'Scale', 'Enterprise'] }, { name: 'verified', type: 'boolean', nullable: false }, { name: 'mrr', type: 'numeric', nullable: false }, { name: 'created_at', type: 'timestamptz', nullable: false }, { name: 'last_seen_at', type: 'timestamptz', nullable: true },
]

const orderColumns: DataColumn[] = [
  { name: 'id', type: 'varchar', nullable: false }, { name: 'user_id', type: 'uuid', nullable: false }, { name: 'status', type: 'varchar', nullable: false }, { name: 'subtotal', type: 'numeric', nullable: false }, { name: 'tax', type: 'numeric', nullable: false }, { name: 'total', type: 'numeric', nullable: false }, { name: 'currency', type: 'char', nullable: false }, { name: 'items', type: 'integer', nullable: false }, { name: 'channel', type: 'varchar', nullable: false }, { name: 'created_at', type: 'timestamptz', nullable: false },
]

const userSchema: TableSchema = {
  columns: userColumns.map((column) => ({ name: column.name, dataType: column.type, nullable: Boolean(column.nullable), defaultValue: column.name === 'id' ? 'gen_random_uuid()' : column.name === 'created_at' ? 'now()' : null, comment: column.name === 'email' ? 'Primary account email' : '' })),
  indexes: [
    { name: 'users_pkey', unique: true, primary: true, type: 'btree', definition: 'CREATE UNIQUE INDEX users_pkey ON public.users USING btree (id)' },
    { name: 'users_email_key', unique: true, primary: false, type: 'btree', definition: 'CREATE UNIQUE INDEX users_email_key ON public.users USING btree (email)' },
    { name: 'idx_users_status_created', unique: false, primary: false, type: 'btree', definition: 'CREATE INDEX idx_users_status_created ON public.users USING btree (status, created_at DESC)' },
  ],
  constraints: [
    { name: 'users_pkey', type: 'PRIMARY KEY', columns: ['id'], definition: 'PRIMARY KEY (id)' },
    { name: 'users_email_key', type: 'UNIQUE', columns: ['email'], definition: 'UNIQUE (email)' },
    { name: 'users_mrr_check', type: 'CHECK', columns: ['mrr'], definition: 'CHECK (mrr >= 0)' },
  ],
}

const orderSchema: TableSchema = {
  columns: orderColumns.map((column) => ({ name: column.name, dataType: column.type, nullable: Boolean(column.nullable), defaultValue: column.name === 'created_at' ? 'now()' : null, comment: '' })),
  indexes: [
    { name: 'orders_pkey', unique: true, primary: true, type: 'btree', definition: 'CREATE UNIQUE INDEX orders_pkey ON public.orders USING btree (id)' },
    { name: 'idx_orders_user_id', unique: false, primary: false, type: 'btree', definition: 'CREATE INDEX idx_orders_user_id ON public.orders USING btree (user_id)' },
    { name: 'idx_orders_created_at', unique: false, primary: false, type: 'btree', definition: 'CREATE INDEX idx_orders_created_at ON public.orders USING btree (created_at DESC)' },
  ],
  constraints: [
    { name: 'orders_pkey', type: 'PRIMARY KEY', columns: ['id'], definition: 'PRIMARY KEY (id)' },
    { name: 'orders_user_id_fkey', type: 'FOREIGN KEY', columns: ['user_id'], definition: 'FOREIGN KEY (user_id) REFERENCES users(id)' },
    { name: 'orders_total_check', type: 'CHECK', columns: ['total'], definition: 'CHECK (total >= 0)' },
  ],
}

export type MockTableFixture = {
  columns: DataColumn[]
  rows: TableRow[]
  schema: TableSchema
  ddl: string
}

export const mockTables: Record<string, MockTableFixture> = {
  'conn-primary:public.users': {
    columns: userColumns,
    rows: userRows,
    schema: userSchema,
    ddl: `CREATE TABLE public.users (\n  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),\n  email varchar(255) NOT NULL UNIQUE,\n  full_name varchar(160) NOT NULL,\n  role user_role NOT NULL DEFAULT 'viewer',\n  status varchar(32) NOT NULL DEFAULT 'invited',\n  plan varchar(32) NOT NULL DEFAULT 'Starter',\n  verified boolean NOT NULL DEFAULT false,\n  mrr numeric(12,2) NOT NULL DEFAULT 0,\n  created_at timestamptz NOT NULL DEFAULT now(),\n  last_seen_at timestamptz\n);`,
  },
  'conn-primary:public.orders': {
    columns: orderColumns,
    rows: orderRows,
    schema: orderSchema,
    ddl: `CREATE TABLE public.orders (\n  id varchar(24) PRIMARY KEY,\n  user_id uuid NOT NULL REFERENCES public.users(id),\n  status varchar(32) NOT NULL,\n  subtotal numeric(12,2) NOT NULL,\n  tax numeric(12,2) NOT NULL,\n  total numeric(12,2) NOT NULL,\n  currency char(3) NOT NULL DEFAULT 'USD',\n  items integer NOT NULL DEFAULT 1,\n  channel varchar(24) NOT NULL,\n  created_at timestamptz NOT NULL DEFAULT now()\n);`,
  },
  'conn-staging:product_staging.users': {
    columns: userColumns,
    rows: userRows.slice(0, 34),
    schema: userSchema,
    ddl: 'CREATE TABLE product_staging.users (...)',
  },
}

function metricSeries(base: number, spread: number) {
  return Array.from({ length: 20 }, (_, index) => ({ timestamp: new Date(Date.UTC(2026, 6, 11, 3, 35 + index * 3)).toISOString(), value: Number((base + Math.sin(index * 0.72) * spread + index * spread * 0.05).toFixed(2)) }))
}

export const mockDashboards: Record<string, DatabaseDashboard> = {
  'conn-primary': { available: true, engine: 'postgresql', version: 'PostgreSQL 17.5', message: 'All systems operating normally', metrics: [
    { key: 'connections', label: 'Connections', value: 42, detail: '42 of 100', trend: 'neutral', series: metricSeries(38, 6) },
    { key: 'queries_per_second', label: 'Queries / sec', value: 1842, detail: '+12.4%', trend: 'up', series: metricSeries(1600, 220) },
    { key: 'cache_hit', label: 'Cache hit', value: 99.4, unit: '%', detail: '+0.6%', trend: 'up', series: metricSeries(98.8, 0.45) },
    { key: 'storage', label: 'Storage', value: 84.7, unit: 'GB', detail: '68% used', trend: 'neutral', series: metricSeries(84.1, 0.3) },
    { key: 'tables', label: 'Tables', value: 128, detail: '9 schemas', trend: 'neutral' },
    { key: 'replication_lag', label: 'Replica lag', value: 21, unit: 'ms', detail: '-8 ms', trend: 'up', series: metricSeries(28, 9) },
  ] },
  'conn-analytics': { available: true, engine: 'mysql', version: 'MySQL 8.4.5', metrics: [
    { key: 'connections', label: 'Connections', value: 18 }, { key: 'queries_per_second', label: 'Queries / sec', value: 674, series: metricSeries(590, 110) }, { key: 'cache_hit', label: 'Buffer pool hit', value: 98.1, unit: '%' }, { key: 'storage', label: 'Storage', value: 214, unit: 'GB' },
  ] },
  'conn-staging': { available: true, engine: 'mariadb', version: 'MariaDB 11.7', metrics: [
    { key: 'connections', label: 'Connections', value: 9 }, { key: 'queries_per_second', label: 'Queries / sec', value: 121, series: metricSeries(110, 25) }, { key: 'cache_hit', label: 'Buffer pool hit', value: 96.8, unit: '%' }, { key: 'storage', label: 'Storage', value: 18.2, unit: 'GB' },
  ] },
}

export const mockSessions: Record<string, DatabaseSessions> = {
  'conn-primary': { available: true, items: [
    { id: '18421', user: 'api_service', database: 'datadock', state: 'active', query: 'SELECT id, email, plan FROM users WHERE status = $1 ORDER BY created_at DESC', durationMs: 842, startedAt: '2026-07-11T04:29:59.158Z', client: '10.0.12.44', waitEvent: '' },
    { id: '18409', user: 'worker', database: 'datadock', state: 'active', query: 'UPDATE jobs SET status = $1, completed_at = now() WHERE id = $2', durationMs: 2412, startedAt: '2026-07-11T04:29:57.588Z', client: '10.0.8.19', waitEvent: 'Lock:transactionid' },
    { id: '18394', user: 'readonly_app', database: 'datadock', state: 'idle', query: 'SELECT * FROM monthly_revenue LIMIT 200', durationMs: 0, startedAt: '2026-07-11T04:28:34.000Z', client: '10.0.4.25' },
    { id: '18372', user: 'billing_service', database: 'datadock', state: 'idle in transaction', query: 'INSERT INTO invoices (...) VALUES (...)', durationMs: 18340, startedAt: '2026-07-11T04:29:41.660Z', client: '10.0.7.11', waitEvent: 'Client:ClientRead' },
    { id: '18311', user: 'migration', database: 'datadock', state: 'active', query: 'CREATE INDEX CONCURRENTLY idx_events_created_at ON audit_events (created_at)', durationMs: 48621, startedAt: '2026-07-11T04:29:11.379Z', client: '10.0.2.8' },
  ] },
  'conn-analytics': { available: true, items: [{ id: '9012', user: 'analyst', database: 'analytics', state: 'Query', query: 'SELECT campaign, sum(revenue) FROM daily_metrics GROUP BY campaign', durationMs: 1320, client: '10.0.4.82' }] },
  'conn-staging': { available: true, items: [{ id: '443', user: 'developer', database: 'product_staging', state: 'Sleep', durationMs: 0, client: '172.18.0.4' }] },
}

export const mockLocks: Record<string, DatabaseLocks> = {
  'conn-primary': { available: true, items: [
    { id: '18409:transactionid', type: 'transactionid', object: 'jobs', mode: 'ShareLock', granted: false, waitingPid: '18409', blockingPid: '18372', query: 'UPDATE jobs SET status = $1 WHERE id = $2' },
    { id: '18372:relation', type: 'relation', object: 'invoices', mode: 'RowExclusiveLock', granted: true, query: 'INSERT INTO invoices (...) VALUES (...)' },
    { id: '18311:relation', type: 'relation', object: 'audit_events', mode: 'ShareUpdateExclusiveLock', granted: true, query: 'CREATE INDEX CONCURRENTLY idx_events_created_at ON audit_events (created_at)' },
  ] },
  'conn-analytics': { available: true, items: [] },
  'conn-staging': { available: true, items: [] },
}

export const mockPerformance: Record<string, DatabasePerformance> = {
  'conn-primary': { available: true, slowQueries: [
    { query: 'SELECT o.*, u.email FROM orders o JOIN users u ON u.id = o.user_id WHERE o.created_at >= $1 ORDER BY o.created_at DESC', calls: 12844, totalMs: 84230.4, meanMs: 6.56, rows: 428100 },
    { query: 'SELECT date_trunc(\'day\', created_at), sum(total) FROM orders GROUP BY 1 ORDER BY 1 DESC', calls: 482, totalMs: 42980.2, meanMs: 89.17, rows: 14842 },
    { query: 'UPDATE jobs SET status = $1, locked_by = $2 WHERE id IN (SELECT id FROM jobs WHERE status = $3 LIMIT 10 FOR UPDATE SKIP LOCKED)', calls: 284201, totalMs: 39214.8, meanMs: 0.14, rows: 128440 },
    { query: 'SELECT * FROM audit_events WHERE payload @> $1 ORDER BY created_at DESC LIMIT 100', calls: 1298, totalMs: 32118.1, meanMs: 24.74, rows: 8921 },
  ] },
  'conn-analytics': { available: true, slowQueries: [{ query: 'SELECT session_id, count(*) FROM events GROUP BY session_id', calls: 821, totalMs: 98432, meanMs: 119.89, rows: 840120 }] },
  'conn-staging': { available: false, message: 'Performance Schema statement summaries are unavailable', slowQueries: [] },
}

export const mockQueryHistory: QueryHistoryItem[] = [
  { id: 'history-1', connectionId: 'conn-primary', sqlText: 'SELECT id, email, plan, status\nFROM public.users\nORDER BY created_at DESC\nLIMIT 100;', status: 'success', durationMs: 38, rowCount: 100, executedAt: '2026-07-11T04:24:12.000Z' },
  { id: 'history-2', connectionId: 'conn-primary', sqlText: "SELECT date_trunc('day', created_at) AS day, sum(total) AS revenue\nFROM public.orders\nWHERE status = 'paid'\nGROUP BY 1 ORDER BY 1;", status: 'success', durationMs: 124, rowCount: 31, executedAt: '2026-07-11T04:18:44.000Z' },
  { id: 'history-3', connectionId: 'conn-primary', sqlText: 'EXPLAIN ANALYZE SELECT * FROM public.orders WHERE user_id = $1;', status: 'success', durationMs: 9, rowCount: 8, executedAt: '2026-07-11T03:51:08.000Z' },
  { id: 'history-4', connectionId: 'conn-analytics', sqlText: 'SELECT campaign, sum(revenue) FROM daily_metrics GROUP BY campaign;', status: 'success', durationMs: 482, rowCount: 24, executedAt: '2026-07-10T15:40:31.000Z' },
  { id: 'history-5', connectionId: 'conn-primary', sqlText: 'SELECT missing_column FROM public.users;', status: 'error', durationMs: 12, rowCount: 0, error: 'column "missing_column" does not exist', executedAt: '2026-07-10T14:12:09.000Z' },
]

export const mockSavedQueries: SavedQuery[] = [
  { id: 'saved-1', connectionId: 'conn-primary', folder: 'Revenue', title: 'Daily revenue by channel', sql: "SELECT date_trunc('day', created_at) AS day, channel, sum(total) AS revenue\nFROM public.orders\nWHERE status = 'paid'\nGROUP BY 1, 2 ORDER BY 1 DESC;", tags: ['revenue', 'daily'], shareCode: 'rev-day-7K2P', createdAt: '2026-06-14T08:30:00.000Z', updatedAt: '2026-07-10T12:08:00.000Z' },
  { id: 'saved-2', connectionId: 'conn-primary', folder: 'Customers', title: 'At-risk enterprise accounts', sql: "SELECT id, email, plan, last_seen_at\nFROM public.users\nWHERE plan = 'Enterprise'\n  AND last_seen_at < now() - interval '14 days'\nORDER BY mrr DESC;", tags: ['customers', 'retention'], shareCode: 'risk-ent-4M9Q', createdAt: '2026-06-22T10:12:00.000Z', updatedAt: '2026-07-09T17:44:00.000Z' },
  { id: 'saved-3', connectionId: 'conn-primary', folder: 'Operations', title: 'Long-running sessions', sql: "SELECT pid, usename, state, query_start, query\nFROM pg_stat_activity\nWHERE state <> 'idle'\n  AND clock_timestamp() - query_start > interval '5 seconds'\nORDER BY query_start;", tags: ['postgres', 'operations'], createdAt: '2026-06-28T07:20:00.000Z', updatedAt: '2026-07-08T11:20:00.000Z' },
  { id: 'saved-4', connectionId: 'conn-analytics', folder: 'Product', title: 'Weekly activation funnel', sql: "SELECT week, step, count(DISTINCT user_id)\nFROM funnels\nWHERE funnel = 'activation'\nGROUP BY week, step ORDER BY week DESC, step;", tags: ['product', 'funnel'], createdAt: '2026-07-02T13:48:00.000Z', updatedAt: '2026-07-07T09:14:00.000Z' },
]
