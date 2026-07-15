import type { ApiQueryExecutionResult, ApiQueryHistoryItem, ApiQueryPlanNode } from '../contracts/query'
import type { QueryHistoryItem, QueryPlanNode, QueryResult } from '../../entities/query'
import { mapDataColumn, mapRow } from './table'

function mapPlanNode(value: ApiQueryPlanNode): QueryPlanNode {
  return {
    id: value.id,
    parentId: value.parentId,
    operation: value.operation,
    relation: value.relation,
    cost: value.cost,
    actualTimeMs: value.actualTimeMs,
    rows: value.rows,
    loops: value.loops,
    details: value.details ? { ...value.details } : undefined,
    children: value.children?.map(mapPlanNode),
  }
}

export function mapQueryResult(value: ApiQueryExecutionResult): QueryResult {
  const sourceColumns = value.plan && value.columns.length === 0 ? value.plan.table.columns : value.columns
  const sourceRows = value.plan && value.rows.length === 0 ? value.plan.table.rows : value.rows
  const columns = sourceColumns.map(mapDataColumn)
  const planColumns = value.plan?.table.columns.map(mapDataColumn) ?? []
  return {
    executionId: value.executionId,
    statementType: value.statementType,
    columns,
    rows: sourceRows.map((row) => mapRow(columns, row)),
    rowsAffected: value.rowsAffected,
    durationMs: value.durationMs,
    truncated: value.truncated,
    limits: { ...value.limits },
    notices: value.notices.map((notice) => ({ ...notice })),
    plan: value.plan ? {
      mode: value.plan.mode,
      columns: planColumns,
      rows: value.plan.table.rows.map((row) => mapRow(planColumns, row)),
      tree: value.plan.tree.map(mapPlanNode),
    } : undefined,
  }
}

export function mapQueryHistoryItem(value: ApiQueryHistoryItem): QueryHistoryItem {
  return {
    id: value.id,
    connectionId: value.connectionId,
    sqlText: value.sql,
    status: value.status,
    durationMs: value.durationMs,
    rowCount: value.rowCount,
    error: value.error,
    executedAt: value.executedAt,
  }
}
