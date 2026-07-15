import type { ApiConnection, ApiConnectionStatus, ApiConnectionTestResult } from '../contracts/connection'
import type { Connection, ConnectionInput, ConnectionRuntimeStatus, ConnectionTestResult } from '../../entities/connection'

export function mapConnection(value: ApiConnection): Connection {
  return {
    id: value.id,
    workspaceId: value.workspaceId,
    name: value.name,
    engine: value.engine,
    host: value.host,
    port: value.port,
    database: value.database,
    username: value.username,
    sslMode: value.sslMode,
    sslCaPath: value.sslCaPath,
    sslCertPath: value.sslCertPath,
    sslKeyPath: value.sslKeyPath,
    proxyUrl: value.proxyUrl,
    sshTunnel: {
      enabled: value.sshTunnel.enabled,
      host: value.sshTunnel.host,
      port: value.sshTunnel.port,
      username: value.sshTunnel.username,
      privateKeyPath: value.sshTunnel.privateKeyPath,
      knownHostsPath: value.sshTunnel.knownHostsPath,
      hasPassword: value.sshTunnel.hasPassword,
    },
    readOnly: value.readOnly,
    autoReconnect: value.autoReconnect,
    maxOpenConns: value.maxOpenConns,
    maxIdleConns: value.maxIdleConns,
    connMaxLifetimeSeconds: value.connMaxLifetimeSeconds,
    connMaxIdleTimeSeconds: value.connMaxIdleTimeSeconds,
    favorite: value.favorite,
    hasPassword: value.hasPassword,
    hasProxyCredentials: value.hasProxyCredentials,
    status: value.status ?? 'disconnected',
    latencyMs: value.latencyMs,
    lastConnectedAt: value.lastConnectedAt,
    lastErrorCode: value.lastErrorCode,
    activeTransactions: value.activeTransactions,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
  }
}

export function toConnectionRequest(value: ConnectionInput) {
  return {
    workspaceId: value.workspaceId,
    name: value.name,
    engine: value.engine,
    host: value.host,
    port: value.port,
    database: value.database,
    username: value.username,
    password: value.password,
    clearPassword: value.clearPassword ?? false,
    sslMode: value.sslMode,
    sslCaPath: value.sslCaPath,
    sslCertPath: value.sslCertPath,
    sslKeyPath: value.sslKeyPath,
    proxyUrl: value.proxyUrl,
    clearProxyCredentials: value.clearProxyCredentials ?? false,
    sshTunnel: {
      enabled: value.sshTunnel.enabled,
      host: value.sshTunnel.host,
      port: value.sshTunnel.port,
      username: value.sshTunnel.username,
      password: value.sshTunnel.password,
      clearPassword: value.sshTunnel.clearPassword ?? false,
      privateKeyPath: value.sshTunnel.privateKeyPath,
      knownHostsPath: value.sshTunnel.knownHostsPath,
    },
    readOnly: value.readOnly,
    autoReconnect: value.autoReconnect,
    maxOpenConns: value.maxOpenConns,
    maxIdleConns: value.maxIdleConns,
    connMaxLifetimeSeconds: value.connMaxLifetimeSeconds,
    connMaxIdleTimeSeconds: value.connMaxIdleTimeSeconds ?? 0,
  }
}

export function mapConnectionStatus(value: ApiConnectionStatus): ConnectionRuntimeStatus {
  return {
    status: value.status ?? 'disconnected',
    latencyMs: value.latencyMs,
    lastConnectedAt: value.lastConnectedAt,
    lastErrorCode: value.lastErrorCode,
    activeTransactions: value.activeTransactions,
  }
}

export function mapConnectionTest(value: ApiConnectionTestResult): ConnectionTestResult {
  return {
    ok: value.ok,
    latencyMs: value.latencyMs,
    message: value.message,
  }
}
