export const DATABASE_ENGINES = ['postgresql', 'mysql', 'mariadb', 'sqlite', 'sqlserver', 'oracle', 'clickhouse', 'redis', 'mongodb'] as const
export type DatabaseEngine = (typeof DATABASE_ENGINES)[number]
export type AvailableDatabaseEngine = Extract<DatabaseEngine, 'postgresql' | 'mysql' | 'mariadb'>
export type ConnectionStatus = 'connected' | 'connecting' | 'disconnected' | 'error'
export type SSLMode = 'disable' | 'require' | 'verify-ca' | 'verify-full'

export type SSHTunnel = {
  enabled: boolean
  host: string
  port: number
  username: string
  privateKeyPath: string
  knownHostsPath: string
  hasPassword: boolean
}

export type Connection = {
  id: string
  workspaceId: string
  name: string
  engine: DatabaseEngine
  host: string
  port: number
  database: string
  username: string
  sslMode: SSLMode
  sslCaPath: string
  sslCertPath: string
  sslKeyPath: string
  proxyUrl: string
  sshTunnel: SSHTunnel
  readOnly: boolean
  autoReconnect: boolean
  maxOpenConns: number
  maxIdleConns: number
  connMaxLifetimeSeconds: number
  favorite: boolean
  hasPassword: boolean
  status: ConnectionStatus
  latencyMs?: number
  lastConnectedAt?: string
  createdAt: string
  updatedAt: string
}

export type ConnectionInput = {
  workspaceId: string
  name: string
  engine: AvailableDatabaseEngine
  host: string
  port: number
  database: string
  username: string
  password: string
  sslMode: SSLMode
  sslCaPath: string
  sslCertPath: string
  sslKeyPath: string
  proxyUrl: string
  sshTunnel: Omit<SSHTunnel, 'hasPassword'> & { password: string }
  readOnly: boolean
  autoReconnect: boolean
  maxOpenConns: number
  maxIdleConns: number
  connMaxLifetimeSeconds: number
}

export type CreateConnectionInput = ConnectionInput
export type UpdateConnectionInput = ConnectionInput

export type ConnectionTestResult = {
  ok: boolean
  latencyMs: number
  message: string
}

export type EngineCapability = {
  engine: DatabaseEngine
  label: string
  available: boolean
  capabilities: string[]
}
