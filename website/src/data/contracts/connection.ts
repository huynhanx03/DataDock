import type { AvailableDatabaseEngine, ConnectionStatus, SSLMode } from '../../entities/connection'

export type ApiSSHTunnel = {
  enabled: boolean
  host: string
  port: number
  username: string
  privateKeyPath: string
  knownHostsPath: string
  hasPassword: boolean
}

export type ApiConnection = {
  id: string
  workspaceId: string
  name: string
  engine: AvailableDatabaseEngine
  host: string
  port: number
  database: string
  username: string
  sslMode: SSLMode
  sslCaPath: string
  sslCertPath: string
  sslKeyPath: string
  proxyUrl: string
  sshTunnel: ApiSSHTunnel
  readOnly: boolean
  autoReconnect: boolean
  maxOpenConns: number
  maxIdleConns: number
  connMaxLifetimeSeconds: number
  connMaxIdleTimeSeconds: number
  favorite: boolean
  hasPassword: boolean
  hasProxyCredentials: boolean
  status?: ConnectionStatus
  latencyMs?: number
  lastConnectedAt?: string
  lastErrorCode?: string
  activeTransactions?: number
  createdAt: string
  updatedAt: string
}

export type ApiConnectionStatus = {
  status?: ConnectionStatus
  latencyMs?: number
  lastConnectedAt?: string
  lastErrorCode?: string
  activeTransactions?: number
}

export type ApiConnectionTestResult = {
  ok: boolean
  latencyMs: number
  message: string
}
