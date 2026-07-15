import type { ApiSchemaApplyResult, ApiSchemaPreview } from '../contracts/schema'
import type { SchemaApplyResult, SchemaPreview } from '../../entities/schema'

export function mapSchemaPreview(value: ApiSchemaPreview): SchemaPreview {
  return {
    connectionId: value.connectionId,
    engine: value.engine,
    actions: value.actions.map((action) => ({ ...action })),
    steps: value.steps.map((step) => ({ ...step })),
    sql: value.sql,
    hash: value.hash,
    destructive: value.destructive,
    generatedAt: value.generatedAt,
  }
}

export function mapSchemaApplyResult(value: ApiSchemaApplyResult): SchemaApplyResult {
  return { ...value }
}
