import type { ApiOperationsDashboard, ApiOperationsLocks, ApiOperationsMetric, ApiOperationsPerformance, ApiOperationsSessions } from '../contracts/operations'
import type { DatabaseDashboard, DatabaseLocks, DatabaseMetric, DatabasePerformance, DatabaseSessions } from '../../entities/database-object'

function mapMetric(value: ApiOperationsMetric): DatabaseMetric {
  return {
    key: value.key,
    label: value.label,
    value: value.value,
    unit: value.unit,
    series: value.series?.map((point) => ({ ...point })),
  }
}

export function mapDashboard(value: ApiOperationsDashboard): DatabaseDashboard {
  return {
    available: value.available,
    message: value.message,
    connectionId: value.connectionId,
    engine: value.engine,
    version: value.version,
    window: value.window,
    collectedAt: value.collectedAt,
    metrics: value.metrics.map(mapMetric),
  }
}

export function mapSessions(value: ApiOperationsSessions): DatabaseSessions {
  return {
    available: value.available,
    message: value.message,
    connectionId: value.connectionId,
    engine: value.engine,
    collectedAt: value.collectedAt,
    items: value.items.map((item) => ({ ...item })),
  }
}

export function mapLocks(value: ApiOperationsLocks): DatabaseLocks {
  return {
    available: value.available,
    message: value.message,
    connectionId: value.connectionId,
    engine: value.engine,
    collectedAt: value.collectedAt,
    items: value.items.map((item) => ({
      id: item.id,
      type: item.type,
      object: item.object,
      mode: item.mode,
      granted: item.granted,
      waitingPid: item.waitingSessionId,
      blockingPid: item.blockingSessionId,
      query: item.query,
    })),
    blockingChains: value.blockingChains.map((chain) => ({ ...chain, blockingSessionIds: [...chain.blockingSessionIds] })),
  }
}

export function mapPerformance(value: ApiOperationsPerformance): DatabasePerformance {
  return {
    available: value.available,
    message: value.message,
    connectionId: value.connectionId,
    engine: value.engine,
    window: value.window,
    collectedAt: value.collectedAt,
    metrics: value.metrics.map(mapMetric),
    slowQueries: value.slowQueries.map((query) => ({ ...query })),
  }
}
