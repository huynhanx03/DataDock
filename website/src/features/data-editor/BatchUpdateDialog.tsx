import { useEffect, useMemo, useState } from 'react'
import { PencilLine } from 'lucide-react'
import type { DataColumn } from '@/entities/database-object'
import { editorInputMode, editorInputStep, editorInputType, editorValueKind, parseEditorValue } from '@/features/data-editor/value-coercion'
import { Button, Checkbox, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, Input, PropertyField, Select } from '@/shared/ui'

type BatchUpdateDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  columns: DataColumn[]
  keyColumns: string[]
  selectedCount: number
  onApply: (column: string, value: unknown) => void
}

export function BatchUpdateDialog({ open, onOpenChange, columns, keyColumns, selectedCount, onApply }: BatchUpdateDialogProps) {
  const editableColumns = useMemo(() => columns.filter((column) => !column.generated && !column.identity && !keyColumns.includes(column.key ?? column.name)), [columns, keyColumns])
  const [columnName, setColumnName] = useState('')
  const [rawValue, setRawValue] = useState('')
  const [setNull, setSetNull] = useState(false)
  const [error, setError] = useState('')
  const column = editableColumns.find((item) => (item.key ?? item.name) === columnName) ?? editableColumns[0]
  const kind = editorValueKind(column?.type ?? 'text')
  const effectiveRawValue = kind === 'boolean' && !rawValue ? 'false' : rawValue

  useEffect(() => {
    if (!open) return
    setColumnName(editableColumns[0] ? editableColumns[0].key ?? editableColumns[0].name : '')
    setRawValue('')
    setSetNull(false)
    setError('')
  }, [editableColumns, open])

  function apply() {
    if (!column) return
    if (setNull) {
      if (!column.nullable) {
        setError(`${column.name} does not allow NULL`)
        return
      }
      onApply(column.key ?? column.name, null)
      onOpenChange(false)
      return
    }
    const parsed = parseEditorValue(effectiveRawValue, kind, column.nullable)
    if (!parsed.ok) {
      setError(parsed.error)
      return
    }
    onApply(column.key ?? column.name, parsed.value)
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Batch update {selectedCount} rows</DialogTitle>
          <DialogDescription>Set one typed value across the current selection. The batch stays staged and can be undone in one step.</DialogDescription>
        </DialogHeader>
        <PropertyField label="Column" htmlFor="batch-update-column">
          <Select id="batch-update-column" value={columnName} onChange={(event) => { setColumnName(event.target.value); setRawValue(''); setSetNull(false); setError('') }}>
            {editableColumns.map((item) => <option key={item.key ?? item.name} value={item.key ?? item.name}>{item.name} · {item.type}</option>)}
          </Select>
        </PropertyField>
        <PropertyField label="New value" htmlFor="batch-update-value" error={error}>
          {column?.enumValues?.length ? (
            <Select id="batch-update-value" value={rawValue} disabled={setNull} onChange={(event) => { setRawValue(event.target.value); setError('') }}>
              <option value="" disabled>Select a value…</option>
              {column.enumValues.map((option) => <option key={option} value={option}>{option}</option>)}
            </Select>
          ) : kind === 'boolean' ? (
            <Select id="batch-update-value" value={rawValue || 'false'} disabled={setNull} onChange={(event) => { setRawValue(event.target.value); setError('') }}><option value="true">true</option><option value="false">false</option></Select>
          ) : (
            <Input id="batch-update-value" autoFocus type={editorInputType(kind)} inputMode={editorInputMode(kind)} step={editorInputStep(kind)} value={rawValue} disabled={setNull} onChange={(event) => { setRawValue(event.target.value); setError('') }} placeholder={kind === 'json' ? '{"key":"value"}' : 'Value…'} />
          )}
        </PropertyField>
        {column?.nullable ? <Checkbox label="Set selected cells to NULL" checked={setNull} onChange={(event) => { setSetNull(event.target.checked); setError('') }} /> : null}
        <div className="rounded-lg border border-border bg-surface/60 p-3 text-[length:var(--font-size-meta)] leading-5 text-muted-foreground">Primary-key, identity, and generated columns are excluded to keep row identity and server-managed values stable. Review the staged batch before applying it.</div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button disabled={!column || (!setNull && effectiveRawValue === '' && (Boolean(column.enumValues?.length) || !column.nullable))} onClick={apply}><PencilLine />Stage batch update</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
