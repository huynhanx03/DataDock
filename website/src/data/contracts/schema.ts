import type { AvailableDatabaseEngine } from '../../entities/connection'
import type { SchemaAction, SchemaActionKind } from '../../entities/schema'

export type ApiSchemaPreview = {
  connectionId: string
  engine: AvailableDatabaseEngine
  actions: SchemaAction[]
  steps: Array<{
    position: number
    actionId: string
    kind: SchemaActionKind
    sql: string
    destructive: boolean
  }>
  sql: string
  hash: string
  destructive: boolean
  generatedAt: string
}

export type ApiSchemaApplyResult = {
  previewHash: string
  appliedSteps: number
  appliedAt: string
}
