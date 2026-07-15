import { useState, type KeyboardEvent } from 'react'
import { CircleSlash2 } from 'lucide-react'
import type { DataColumn } from '@/entities/database-object'
import { editorInputMode, editorInputStep, editorInputType, editorInputValue, editorValueKind, parseEditorValue } from '@/features/data-editor/value-coercion'
import { cn } from '@/shared/lib/cn'
import { IconButton, Input, Select } from '@/shared/ui'

type TypedCellEditorProps = {
  column: DataColumn
  value: unknown
  onCommit: (value: unknown) => void
  onCancel: () => void
}

export function TypedCellEditor({ column, value, onCommit, onCancel }: TypedCellEditorProps) {
  const kind = editorValueKind(column.type)
  const [raw, setRaw] = useState(() => editorInputValue(value, kind))
  const [error, setError] = useState<string>()

  function commit() {
    const parsed = parseEditorValue(raw, kind, column.nullable)
    if (!parsed.ok) {
      setError(parsed.error)
      return
    }
    onCommit(parsed.value)
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement | HTMLSelectElement>) {
    if (event.key === 'Enter') {
      event.preventDefault()
      commit()
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      onCancel()
    }
  }

  if (column.enumValues?.length) {
    return (
      <Select
        autoFocus
        aria-label={`Edit ${column.name}`}
        className="h-7 rounded border-primary/45 bg-background py-0 font-mono text-[length:var(--font-size-data)]"
        value={raw || (column.nullable ? '__null__' : column.enumValues[0])}
        onChange={(event) => onCommit(event.target.value === '__null__' ? null : event.target.value)}
        onKeyDown={handleKeyDown}
      >
        {column.nullable ? <option value="__null__">NULL</option> : null}
        {column.enumValues.map((option) => <option key={option} value={option}>{option}</option>)}
      </Select>
    )
  }

  if (kind === 'boolean') {
    return (
      <Select
        autoFocus
        aria-label={`Edit ${column.name}`}
        className="h-7 rounded border-primary/45 bg-background py-0 font-mono text-[length:var(--font-size-data)]"
        value={raw || (column.nullable ? 'null' : 'false')}
        onChange={(event) => {
          const next = event.target.value
          setRaw(next)
          onCommit(next === 'null' ? null : next === 'true')
        }}
        onKeyDown={handleKeyDown}
      >
        <option value="true">true</option>
        <option value="false">false</option>
        {column.nullable ? <option value="null">NULL</option> : null}
      </Select>
    )
  }

  return (
    <div className="relative flex w-full min-w-0 items-center gap-1">
      <Input
        autoFocus
        aria-label={`Edit ${column.name}`}
        aria-invalid={Boolean(error)}
        title={error}
        type={editorInputType(kind)}
        inputMode={editorInputMode(kind)}
        step={editorInputStep(kind)}
        value={raw}
        onChange={(event) => {
          setRaw(event.target.value)
          setError(undefined)
        }}
        onBlur={commit}
        onKeyDown={handleKeyDown}
        className={cn('h-7 min-w-20 rounded border-primary/45 bg-background px-2 font-mono text-[length:var(--font-size-data)]', error && 'border-destructive ring-2 ring-destructive/20')}
      />
      {column.nullable ? (
        <IconButton
          label={`Set ${column.name} to NULL`}
          size="icon-xs"
          className="shrink-0"
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => onCommit(null)}
        >
          <CircleSlash2 />
        </IconButton>
      ) : null}
    </div>
  )
}
