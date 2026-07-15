import { useEffect, useMemo } from 'react'
import {
  Activity,
  Braces,
  Cable,
  Database,
  FolderHeart,
  History,
  Moon,
  Plus,
  Search,
  Settings,
  Sun,
  Table2,
} from 'lucide-react'
import type { Connection } from '@/entities/connection'
import type { CatalogTree, DatabaseObject } from '@/entities/database-object'
import { APP_ROUTES } from '@/shared/config/routes'
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
  CommandShortcut,
} from '@/shared/ui'
import type { ActivityId } from '@/widgets/app-shell/shell-types'

type CommandPaletteProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  connections: Connection[]
  catalog?: CatalogTree
  theme: 'dark' | 'light'
  onThemeToggle: () => void
  onActivitySelect: (activity: ActivityId, route: string) => void
  onConnectionSelect: (connection: Connection) => void
  onObjectSelect: (object: DatabaseObject) => void
  onNewConnection: () => void
  onNewQuery: () => void
}

function collectObjects(nodes: DatabaseObject[], result: DatabaseObject[] = []) {
  for (const node of nodes) {
    if (node.kind !== 'group' && node.kind !== 'database' && node.kind !== 'schema') result.push(node)
    if (node.children?.length) collectObjects(node.children, result)
  }
  return result
}

export function CommandPalette({
  open,
  onOpenChange,
  connections,
  catalog,
  theme,
  onThemeToggle,
  onActivitySelect,
  onConnectionSelect,
  onObjectSelect,
  onNewConnection,
  onNewQuery,
}: CommandPaletteProps) {
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        onOpenChange(!open)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [onOpenChange, open])

  const objects = useMemo(() => collectObjects(catalog?.databases ?? []).slice(0, 60), [catalog])
  const run = (action: () => void) => {
    action()
    onOpenChange(false)
  }

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange} title="DataDock command palette" contentProps={{ className: 'top-[18%] translate-y-0' }}>
      <CommandInput placeholder="Search commands, connections, and database objects…" autoFocus />
      <CommandList>
        <CommandEmpty>
          <Search className="mx-auto mb-3 size-5 text-muted-foreground" />
          No matching command or object.
        </CommandEmpty>
        <CommandGroup heading="Quick actions">
          <CommandItem onSelect={() => run(onNewConnection)}>
            <Plus />
            <span>New database connection</span>
            <CommandShortcut>⌘N</CommandShortcut>
          </CommandItem>
          <CommandItem onSelect={() => run(onNewQuery)}>
            <Braces />
            <span>Open a new SQL query</span>
            <CommandShortcut>⌘T</CommandShortcut>
          </CommandItem>
          <CommandItem onSelect={() => run(onThemeToggle)}>
            {theme === 'dark' ? <Sun /> : <Moon />}
            <span>Switch to {theme === 'dark' ? 'light' : 'dark'} theme</span>
          </CommandItem>
        </CommandGroup>
        <CommandSeparator />
        <CommandGroup heading="Navigate">
          <CommandItem onSelect={() => run(() => onActivitySelect('explorer', APP_ROUTES.home))}>
            <Database />
            Database explorer
          </CommandItem>
          <CommandItem onSelect={() => run(() => onActivitySelect('connections', APP_ROUTES.connections))}>
            <Cable />
            Connection manager
          </CommandItem>
          <CommandItem onSelect={() => run(() => onActivitySelect('query', APP_ROUTES.query))}>
            <Braces />
            SQL workspace
          </CommandItem>
          <CommandItem onSelect={() => run(() => onActivitySelect('history', APP_ROUTES.history))}>
            <History />
            Query history
          </CommandItem>
          <CommandItem onSelect={() => run(() => onActivitySelect('saved', APP_ROUTES.savedQueries))}>
            <FolderHeart />
            Saved queries
          </CommandItem>
          <CommandItem onSelect={() => run(() => onActivitySelect('operations', APP_ROUTES.operations))}>
            <Activity />
            Database operations
          </CommandItem>
          <CommandItem onSelect={() => run(() => onActivitySelect('settings', APP_ROUTES.settings))}>
            <Settings />
            Settings
          </CommandItem>
        </CommandGroup>
        {connections.length ? (
          <>
            <CommandSeparator />
            <CommandGroup heading="Connections">
              {connections.map((connection) => (
                <CommandItem
                  key={connection.id}
                  value={`connection ${connection.name} ${connection.engine} ${connection.database}`}
                  onSelect={() => run(() => onConnectionSelect(connection))}
                >
                  <Database />
                  <span className="min-w-0 flex-1 truncate">{connection.name}</span>
                  <span className="font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{connection.engine}</span>
                  <span className={connection.status === 'connected' ? 'size-1.5 rounded-full bg-emerald-400' : 'size-1.5 rounded-full bg-muted-foreground'} />
                </CommandItem>
              ))}
            </CommandGroup>
          </>
        ) : null}
        {objects.length ? (
          <>
            <CommandSeparator />
            <CommandGroup heading="Database objects">
              {objects.map((object) => (
                <CommandItem
                  key={object.id}
                  value={`object ${object.kind} ${object.qualifiedName}`}
                  onSelect={() => run(() => onObjectSelect(object))}
                >
                  <Table2 />
                  <span className="min-w-0 flex-1 truncate">{object.name}</span>
                  <span className="truncate font-mono text-[length:var(--font-size-meta)] text-muted-foreground">{object.schema || object.kind}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </>
        ) : null}
      </CommandList>
      <div className="flex h-10 items-center gap-4 border-t border-border px-3.5 font-mono text-[length:var(--font-size-meta)] text-muted-foreground">
        <span><kbd>↑↓</kbd> Navigate</span>
        <span><kbd>↵</kbd> Open</span>
        <span><kbd>esc</kbd> Close</span>
        <span className="ml-auto">DataDock Command Center</span>
      </div>
    </CommandDialog>
  )
}
