import type { AvailableDatabaseEngine } from '../../entities/connection'
import type { OperationsWindow } from '../../entities/database-object'

export type ApiOperationsMetric = {
  key: string
  label: string
  value: number
  unit?: string
  series?: Array<{ timestamp: string; value: number }>
}

export type ApiOperationsDashboard = {
  available: boolean
  message?: string
  connectionId: string
  engine: AvailableDatabaseEngine
  version?: string
  window: OperationsWindow
  collectedAt: string
  metrics: ApiOperationsMetric[]
}

export type ApiOperationsSessions = {
  available: boolean
  message?: string
  connectionId: string
  engine: AvailableDatabaseEngine
  collectedAt: string
  items: Array<{
    id: string
    user: string
    database: string
    state: string
    query?: string
    durationMs?: number
    startedAt?: string
    client?: string
    waitEvent?: string
  }>
}

export type ApiOperationsLocks = {
  available: boolean
  message?: string
  connectionId: string
  engine: AvailableDatabaseEngine
  collectedAt: string
  items: Array<{
    id: string
    type: string
    object?: string
    mode?: string
    granted: boolean
    waitingSessionId?: string
    blockingSessionId?: string
    query?: string
  }>
  blockingChains: Array<{
    waitingSessionId: string
    blockingSessionIds: string[]
  }>
}

export type ApiOperationsPerformance = {
  available: boolean
  message?: string
  connectionId: string
  engine: AvailableDatabaseEngine
  window: OperationsWindow
  collectedAt: string
  metrics: ApiOperationsMetric[]
  slowQueries: Array<{
    fingerprint: string
    query: string
    calls: number
    totalMs: number
    meanMs: number
    rows: number
  }>
}

export type ApiSessionControlResult = {
  sessionId: string
  action: 'cancel' | 'terminate'
  acceptedAt: string
}
