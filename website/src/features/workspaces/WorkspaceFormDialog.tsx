import { useEffect, useId, useState, type FormEvent } from 'react'
import { Check } from 'lucide-react'
import type { CreateWorkspaceInput, Workspace } from '@/entities/workspace'
import { cn } from '@/shared/lib/cn'
import { Button, Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, Input } from '@/shared/ui'
import { WORKSPACE_COLORS, WORKSPACE_ICONS, WorkspaceMark } from './WorkspaceMark'

type WorkspaceFormDialogProps = {
  open: boolean
  mode: 'create' | 'edit'
  workspace?: Workspace
  defaultColor?: string
  onOpenChange: (open: boolean) => void
  onSubmit: (value: CreateWorkspaceInput) => Promise<void>
}

type FormErrors = Partial<Record<keyof CreateWorkspaceInput, string>>

function initialValue(workspace: Workspace | undefined, defaultColor: string): CreateWorkspaceInput {
  return workspace
    ? { name: workspace.name, icon: workspace.icon, color: workspace.color }
    : { name: 'New workspace', icon: 'database', color: defaultColor }
}

function validate(value: CreateWorkspaceInput): FormErrors {
  const errors: FormErrors = {}
  if (!value.name.trim()) errors.name = 'Enter a workspace name.'
  else if (value.name.trim().length > 80) errors.name = 'Use 80 characters or fewer.'
  if (!value.icon.trim()) errors.icon = 'Choose an icon.'
  if (!/^#[0-9a-f]{6}$/i.test(value.color.trim())) errors.color = 'Use a six-digit hex color.'
  return errors
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : 'The workspace could not be saved.'
}

export function WorkspaceFormDialog({ open, mode, workspace, defaultColor = WORKSPACE_COLORS[0], onOpenChange, onSubmit }: WorkspaceFormDialogProps) {
  const generatedId = useId().replaceAll(':', '')
  const [value, setValue] = useState<CreateWorkspaceInput>(() => initialValue(workspace, defaultColor))
  const [errors, setErrors] = useState<FormErrors>({})
  const [submitError, setSubmitError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    if (!open) return
    setValue(initialValue(workspace, defaultColor))
    setErrors({})
    setSubmitError('')
    setSubmitting(false)
  }, [defaultColor, open, workspace])

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const normalized = { name: value.name.trim(), icon: value.icon.trim(), color: value.color.trim() }
    const nextErrors = validate(normalized)
    setErrors(nextErrors)
    setSubmitError('')
    if (Object.keys(nextErrors).length) return
    setSubmitting(true)
    try {
      await onSubmit(normalized)
      onOpenChange(false)
    } catch (error) {
      setSubmitError(errorMessage(error))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!submitting) onOpenChange(next) }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{mode === 'create' ? 'Create workspace' : 'Edit workspace'}</DialogTitle>
          <DialogDescription>{mode === 'create' ? 'Group related database connections in a focused workspace.' : 'Update how this workspace appears throughout DataDock.'}</DialogDescription>
        </DialogHeader>
        <form className="grid gap-5" onSubmit={submit}>
          <div className="flex items-center gap-3 rounded-xl border border-border bg-muted/35 p-3">
            <WorkspaceMark workspace={value} className="size-10 rounded-xl" iconClassName="size-5" />
            <div className="min-w-0">
              <p className="truncate text-[length:var(--font-size-ui)] font-semibold text-foreground">{value.name.trim() || 'Workspace preview'}</p>
              <p className="mt-0.5 text-[length:var(--font-size-meta)] text-muted-foreground">Icon and color preview</p>
            </div>
          </div>

          <label htmlFor={`${generatedId}-name`} className="grid gap-1.5 text-[length:var(--font-size-ui)] font-medium text-foreground">
            <span>Name</span>
            <Input id={`${generatedId}-name`} autoFocus maxLength={80} value={value.name} invalid={Boolean(errors.name)} aria-describedby={errors.name ? `${generatedId}-name-error` : undefined} onChange={(event) => setValue((current) => ({ ...current, name: event.target.value }))} placeholder="Production databases" />
            {errors.name ? <span id={`${generatedId}-name-error`} role="alert" className="text-[length:var(--font-size-meta)] font-normal text-destructive">{errors.name}</span> : null}
          </label>

          <fieldset className="grid gap-2">
            <legend className="text-[length:var(--font-size-ui)] font-medium text-foreground">Icon</legend>
            <div className="grid grid-cols-8 gap-1.5">
              {WORKSPACE_ICONS.map((option) => {
                const Icon = option.icon
                const selected = value.icon === option.value
                return <button key={option.value} type="button" aria-label={option.label} aria-pressed={selected} onClick={() => setValue((current) => ({ ...current, icon: option.value }))} className={cn('grid aspect-square place-items-center rounded-lg border outline-none transition-colors focus-visible:ring-2 focus-visible:ring-ring/30', selected ? 'border-primary bg-accent text-accent-foreground' : 'border-border bg-surface text-muted-foreground hover:border-border-strong hover:bg-accent/45 hover:text-foreground')}><Icon className="size-4" /></button>
              })}
            </div>
            {errors.icon ? <span role="alert" className="text-[length:var(--font-size-meta)] text-destructive">{errors.icon}</span> : null}
          </fieldset>

          <fieldset className="grid gap-2">
            <legend className="text-[length:var(--font-size-ui)] font-medium text-foreground">Color</legend>
            <div className="flex flex-wrap gap-2">
              {WORKSPACE_COLORS.map((color) => <button key={color} type="button" aria-label={`Use ${color}`} aria-pressed={value.color.toLowerCase() === color.toLowerCase()} onClick={() => setValue((current) => ({ ...current, color }))} className="grid size-7 place-items-center rounded-full border border-white/10 outline-none ring-offset-2 ring-offset-popover transition-transform hover:scale-105 focus-visible:ring-2 focus-visible:ring-ring"><span className="grid size-6 place-items-center rounded-full" style={{ backgroundColor: color }}>{value.color.toLowerCase() === color.toLowerCase() ? <Check className="size-3.5 text-white drop-shadow" /> : null}</span></button>)}
            </div>
            <div className="grid grid-cols-[auto_1fr] items-center gap-2">
              <input aria-label="Choose custom workspace color" type="color" value={/^#[0-9a-f]{6}$/i.test(value.color) ? value.color : WORKSPACE_COLORS[0]} onChange={(event) => setValue((current) => ({ ...current, color: event.target.value }))} className="h-[var(--control-height)] w-11 cursor-pointer rounded-md border border-input bg-surface p-1 shadow-control" />
              <Input id={`${generatedId}-color`} aria-label="Workspace color hex value" maxLength={7} value={value.color} invalid={Boolean(errors.color)} aria-describedby={errors.color ? `${generatedId}-color-error` : undefined} onChange={(event) => setValue((current) => ({ ...current, color: event.target.value }))} placeholder="#7168f5" className="font-mono" />
            </div>
            {errors.color ? <span id={`${generatedId}-color-error`} role="alert" className="text-[length:var(--font-size-meta)] text-destructive">{errors.color}</span> : null}
          </fieldset>

          {submitError ? <div role="alert" className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-[length:var(--font-size-ui)] text-destructive">{submitError}</div> : null}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={submitting} onClick={() => onOpenChange(false)}>Cancel</Button>
            <Button type="submit" disabled={submitting}>{submitting ? 'Saving…' : mode === 'create' ? 'Create workspace' : 'Save changes'}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
