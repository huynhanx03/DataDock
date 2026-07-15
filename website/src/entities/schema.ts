import type { DatabaseEngine } from './connection'

export type SchemaActionKind =
  | 'create_table'
  | 'rename_table'
  | 'drop_table'
  | 'add_column'
  | 'rename_column'
  | 'alter_column_type'
  | 'set_column_nullable'
  | 'set_column_default'
  | 'set_column_comment'
  | 'drop_column'
  | 'add_constraint'
  | 'drop_constraint'
  | 'create_index'
  | 'drop_index'
  | 'rebuild_index'
  | 'analyze_index'

export type SchemaTarget = {
  schema?: string
  table: string
}

export type SchemaColumnDefinition = {
  name: string
  dataType: string
  nullable: boolean
  defaultValue?: string | null
  comment?: string
  identity?: boolean
  generatedExpression?: string | null
}

export type SchemaConstraintDefinition = {
  name: string
  type: 'primary_key' | 'foreign_key' | 'unique' | 'check'
  columns?: string[]
  referencedTarget?: SchemaTarget
  referencedColumns?: string[]
  expression?: string
  onUpdate?: string
  onDelete?: string
}

export type SchemaIndexDefinition = {
  name: string
  columns: string[]
  unique: boolean
  method?: string
  predicate?: string
}

export type SchemaAction = {
  id?: string
  kind: SchemaActionKind
  target: SchemaTarget
  name?: string
  newName?: string
  columns?: SchemaColumnDefinition[]
  column?: SchemaColumnDefinition
  constraint?: SchemaConstraintDefinition
  index?: SchemaIndexDefinition
  dataType?: string
  nullable?: boolean
  defaultValue?: string | null
  comment?: string
  cascade?: boolean
}

export type SchemaDraftAction = Omit<SchemaAction, 'id' | 'target'>

export type SchemaDraftChange = {
  label: string
  description: string
  actions: SchemaDraftAction[]
}

export type SchemaStep = {
  position: number
  actionId: string
  kind: SchemaActionKind
  sql: string
  destructive: boolean
}

export type SchemaPreview = {
  connectionId: string
  engine: DatabaseEngine
  actions: SchemaAction[]
  steps: SchemaStep[]
  sql: string
  hash: string
  destructive: boolean
  generatedAt: string
}

export type SchemaApplyInput = {
  actions: SchemaAction[]
  previewHash: string
  confirmDestructive: boolean
}

export type SchemaApplyResult = {
  previewHash: string
  appliedSteps: number
  appliedAt: string
}
