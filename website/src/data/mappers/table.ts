import type { ApiDataColumn, ApiTableMutationResult, ApiTableRowsResult, ApiTableSchema } from '../contracts/catalog'
import type { DataColumn, RowMutation, TableMutateResult, TableRow, TableRowsInput, TableRowsResult, TableSchema } from '../../entities/database-object'

export function mapDataColumn(value: ApiDataColumn): DataColumn {
  return {
    key: value.key,
    name: value.name,
    type: value.databaseType || value.type,
    databaseType: value.databaseType,
    logicalType: value.logicalType,
    nullable: value.nullable,
    defaultValue: value.defaultValue,
    precision: value.precision,
    scale: value.scale,
    length: value.length,
    enumValues: [...(value.enumValues ?? [])],
    identity: value.identity,
    generated: value.generated,
    primaryKey: value.primaryKey,
    valueEncoding: value.valueEncoding,
  }
}

export function mapRow(columns: DataColumn[], values: unknown[]): TableRow {
  return Object.fromEntries(columns.map((column, index) => [column.key ?? column.name, values[index]]))
}

export function toTableRowsRequest(reference: string, input: TableRowsInput, pageSize: number, page: number) {
  const sorts = input.sortBy ? [{ column: input.sortBy, direction: input.sortDirection ?? 'asc' }] : []
  return {
    reference,
    limit: pageSize,
    offset: (page - 1) * pageSize,
    search: input.search?.trim() ?? '',
    columns: input.columns ?? [],
    sorts,
    filters: (input.filters ?? []).map((filter) => ({ ...filter, operator: filter.operator === 'neq' ? 'ne' : filter.operator })),
    includeTotal: input.includeTotal ?? true,
  }
}

export function mapTableRows(value: ApiTableRowsResult): TableRowsResult {
  const columns = value.columns.map(mapDataColumn)
  const pageSize = Math.max(1, value.limit)
  return {
    columns,
    rows: value.rows.map((row) => mapRow(columns, row)),
    page: Math.floor(value.offset / pageSize) + 1,
    pageSize,
    total: value.total ?? value.offset + value.rows.length,
    durationMs: value.durationMs,
    primaryKeyColumns: [...(value.primaryKeyColumns ?? [])],
    hasMore: value.hasMore,
    nextPage: value.hasMore ? Math.floor((value.nextOffset ?? value.offset + pageSize) / pageSize) + 1 : undefined,
    truncated: value.truncated,
  }
}

export function mapTableSchema(value: ApiTableSchema): TableSchema {
  return {
    columns: value.columns.map((column) => ({
      name: column.name,
      dataType: column.dataType,
      databaseType: column.databaseType,
      nullable: column.nullable,
      defaultValue: column.defaultValue,
      comment: column.comment,
      precision: column.precision,
      scale: column.scale,
      length: column.length,
      enumValues: [...(column.enumValues ?? [])],
      identity: column.identity,
      generated: column.generated,
      primaryKey: column.primaryKey,
    })),
    indexes: value.indexes.map((index) => ({ ...index })),
    constraints: value.constraints.map((constraint) => ({ ...constraint, columns: [...constraint.columns] })),
  }
}

export function toTableMutationsRequest(reference: string, mutations: RowMutation[], transactionId?: string) {
  return {
    reference,
    transactionId,
    mutations: mutations.map((mutation) => ({
      kind: mutation.kind,
      values: mutation.values,
      keys: mutation.keys,
      expectedValues: mutation.expectedValues,
    })),
  }
}

export function mapTableMutationResult(value: ApiTableMutationResult): TableMutateResult {
  return {
    status: value.status,
    atomic: value.atomic,
    applied: value.applied,
    conflicts: value.conflicts.map((conflict) => ({ ...conflict })),
  }
}
