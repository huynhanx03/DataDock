import { Filter, Search, X } from 'lucide-react'
import { IconButton, Input } from '@/shared/ui'

type ExplorerSearchProps = {
  value: string
  onChange: (value: string) => void
}

export function ExplorerSearch({ value, onChange }: ExplorerSearchProps) {
  return (
    <div className="flex h-[var(--toolbar-height)] shrink-0 items-center gap-1.5 border-b border-border bg-surface/60 px-3">
      <div className="relative min-w-0 flex-1">
        <Search className="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={value}
          onChange={(event) => onChange(event.target.value)}
          placeholder="Search databases and objects"
          aria-label="Search database objects"
          className="h-[var(--control-height-sm)] bg-background/55 pr-8 pl-8 text-[length:var(--font-size-ui)] shadow-none"
        />
        {value ? (
          <button
            type="button"
            aria-label="Clear object search"
            onClick={() => onChange('')}
            className="absolute top-1/2 right-1.5 grid size-6 -translate-y-1/2 place-items-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
          >
            <X className="size-3.5" />
          </button>
        ) : null}
      </div>
      <IconButton label="Filter database objects" size="icon-sm" variant="ghost">
        <Filter />
      </IconButton>
    </div>
  )
}
