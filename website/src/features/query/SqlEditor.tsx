import { useEffect, useRef, useState } from 'react'
import MonacoEditor, { loader, type BeforeMount, type OnMount } from '@monaco-editor/react'
import type { editor as MonacoEditorApi, Position } from 'monaco-editor'
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api2.js'
import 'monaco-editor/esm/vs/editor/contrib/bracketMatching/browser/bracketMatching.js'
import 'monaco-editor/esm/vs/editor/contrib/clipboard/browser/clipboard.js'
import 'monaco-editor/esm/vs/editor/contrib/contextmenu/browser/contextmenu.js'
import 'monaco-editor/esm/vs/editor/contrib/find/browser/findController.js'
import 'monaco-editor/esm/vs/editor/contrib/folding/browser/folding.js'
import 'monaco-editor/esm/vs/editor/contrib/hover/browser/hoverContribution.js'
import 'monaco-editor/esm/vs/editor/contrib/snippet/browser/snippetController2.js'
import 'monaco-editor/esm/vs/editor/contrib/suggest/browser/suggestController.js'
import 'monaco-editor/esm/vs/basic-languages/sql/sql.contribution.js'
import EditorWorker from 'monaco-editor/esm/vs/editor/editor.worker.js?worker'
import { Braces, Code2, Play, Search, WandSparkles } from 'lucide-react'
import { Badge, Button, IconButton, Skeleton, Tooltip, TooltipContent, TooltipTrigger } from '@/shared/ui'

type MonacoHost = typeof globalThis & {
  MonacoEnvironment?: {
    getWorker: () => Worker
  }
}

;(globalThis as MonacoHost).MonacoEnvironment = { getWorker: () => new EditorWorker() }
loader.config({ monaco })

export type SqlCompletionItem = {
  label: string
  detail: string
  insertText?: string
  kind?: 'database' | 'schema' | 'table' | 'view' | 'column' | 'function' | 'keyword'
}

type SqlEditorProps = {
  value: string
  dialect: string
  completionItems: SqlCompletionItem[]
  onChange: (value: string) => void
  onRun: () => void
  onRunSelection: (value: string) => void
  onFormat: () => void
  onSelectionChange?: (value: string) => void
}

function editorLoading() {
  return <div className="grid h-full grid-cols-[2.75rem_1fr] gap-3 bg-[#070a11] px-4 py-4"><div className="space-y-2">{Array.from({ length: 8 }, (_, index) => <Skeleton key={index} className="h-4 w-5" />)}</div><div className="space-y-2">{[58, 44, 72, 63, 38, 67, 51, 30].map((width, index) => <Skeleton key={index} className="h-4" style={{ width: `${width}%` }} />)}</div></div>
}

export function SqlEditor({ value, dialect, completionItems, onChange, onRun, onRunSelection, onFormat, onSelectionChange }: SqlEditorProps) {
  const [selection, setSelection] = useState('')
  const editorRef = useRef<Parameters<OnMount>[0] | null>(null)
  const completionRef = useRef(completionItems)
  const callbacksRef = useRef({ onRun, onRunSelection, onFormat, onSelectionChange })
  const completionDisposable = useRef<{ dispose: () => void } | null>(null)

  useEffect(() => { completionRef.current = completionItems }, [completionItems])
  useEffect(() => { callbacksRef.current = { onRun, onRunSelection, onFormat, onSelectionChange } }, [onFormat, onRun, onRunSelection, onSelectionChange])
  useEffect(() => () => completionDisposable.current?.dispose(), [])

  const beforeMount: BeforeMount = (monaco) => {
    monaco.editor.defineTheme('datadock-sql', {
      base: 'vs-dark',
      inherit: true,
      rules: [
        { token: 'keyword.sql', foreground: '8BA8FF', fontStyle: 'bold' },
        { token: 'string.sql', foreground: '70D6A6' },
        { token: 'number.sql', foreground: 'D6A7FF' },
        { token: 'comment.sql', foreground: '687389', fontStyle: 'italic' },
      ],
      colors: {
        'editor.background': '#070A11',
        'editor.foreground': '#D9DFEA',
        'editorLineNumber.foreground': '#465066',
        'editorLineNumber.activeForeground': '#9AA7BD',
        'editorCursor.foreground': '#7692FF',
        'editor.selectionBackground': '#334B9B66',
        'editor.inactiveSelectionBackground': '#27396F55',
        'editor.lineHighlightBackground': '#101624',
        'editorIndentGuide.background1': '#1B2231',
        'editorSuggestWidget.background': '#111722',
        'editorSuggestWidget.border': '#2A3447',
        'editorSuggestWidget.selectedBackground': '#283861',
      },
    })
  }

  const handleMount: OnMount = (editor, monaco) => {
    editorRef.current = editor
    completionDisposable.current?.dispose()
    completionDisposable.current = monaco.languages.registerCompletionItemProvider('sql', {
      triggerCharacters: ['.', ' '],
      provideCompletionItems: (model: MonacoEditorApi.ITextModel, position: Position) => {
        const word = model.getWordUntilPosition(position)
        const needle = word.word.toLowerCase()
        const range = { startLineNumber: position.lineNumber, endLineNumber: position.lineNumber, startColumn: word.startColumn, endColumn: word.endColumn }
        const kinds = monaco.languages.CompletionItemKind
        const mapKind = (kind: SqlCompletionItem['kind']) => {
          if (kind === 'table' || kind === 'view') return kinds.Class
          if (kind === 'column') return kinds.Field
          if (kind === 'function') return kinds.Function
          if (kind === 'database' || kind === 'schema') return kinds.Module
          return kinds.Keyword
        }
        return {
          suggestions: completionRef.current
            .filter((item) => !needle || item.label.toLowerCase().includes(needle))
            .slice(0, 160)
            .map((item) => ({
            label: item.label,
            detail: item.detail,
            insertText: item.insertText ?? item.label,
            kind: mapKind(item.kind),
            range,
            })),
        }
      },
    })
    editor.addAction({
      id: 'datadock.run-query',
      label: 'Run query or selection',
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter],
      run: () => {
        const activeSelection = editor.getSelection()
        const selected = activeSelection ? editor.getModel()?.getValueInRange(activeSelection).trim() ?? '' : ''
        if (selected) callbacksRef.current.onRunSelection(selected)
        else callbacksRef.current.onRun()
      },
    })
    editor.addAction({
      id: 'datadock.format-query',
      label: 'Format SQL',
      keybindings: [monaco.KeyMod.Shift | monaco.KeyMod.Alt | monaco.KeyCode.KeyF],
      run: () => callbacksRef.current.onFormat(),
    })
    editor.onDidChangeCursorSelection((event) => {
      const selected = editor.getModel()?.getValueInRange(event.selection).trim() ?? ''
      setSelection(selected)
      callbacksRef.current.onSelectionChange?.(selected)
    })
  }

  function openFind() {
    editorRef.current?.trigger('datadock-toolbar', 'actions.find', null)
    editorRef.current?.focus()
  }

  return <div className="flex h-full min-h-0 flex-col overflow-hidden bg-[#070a11]">
    <div className="flex h-[var(--toolbar-height)] shrink-0 items-center gap-2 border-b border-border bg-surface/45 px-3 text-[length:var(--font-size-meta)] text-muted-foreground">
      <Code2 className="size-3.5 text-primary" />
      <span className="font-semibold text-foreground">SQL</span>
      <Badge variant="outline" className="h-5 font-mono text-[10px]">{dialect}</Badge>
      <span className="h-4 w-px bg-border" />
      <Tooltip><TooltipTrigger asChild><IconButton label="Find in query" size="icon-xs" onClick={openFind}><Search /></IconButton></TooltipTrigger><TooltipContent>Find <kbd className="ml-1 font-mono">⌘F</kbd></TooltipContent></Tooltip>
      <Tooltip><TooltipTrigger asChild><IconButton label="Format SQL" size="icon-xs" onClick={onFormat}><WandSparkles /></IconButton></TooltipTrigger><TooltipContent>Format SQL <kbd className="ml-1 font-mono">⇧⌥F</kbd></TooltipContent></Tooltip>
      <div className="ml-auto flex items-center gap-2">
        {selection ? <span className="hidden max-w-56 truncate font-mono text-[10px] text-primary lg:inline">{selection.split('\n').length} selected {selection.split('\n').length === 1 ? 'line' : 'lines'}</span> : <span className="hidden items-center gap-1.5 text-[10px] xl:flex"><Braces className="size-3" />Catalog completion enabled</span>}
        {selection ? <Button size="xs" variant="subtle" onClick={() => onRunSelection(selection)}><Play />Run selection</Button> : null}
      </div>
    </div>
    <div className="min-h-0 flex-1">
      <MonacoEditor
          beforeMount={beforeMount}
          onMount={handleMount}
          language="sql"
          theme="datadock-sql"
          value={value}
          onChange={(next) => onChange(next ?? '')}
          loading={editorLoading()}
          options={{
            automaticLayout: true,
            fontFamily: 'JetBrains Mono Variable, JetBrains Mono, monospace',
            fontSize: 12.5,
            fontLigatures: true,
            lineHeight: 21,
            minimap: { enabled: false },
            padding: { top: 14, bottom: 18 },
            lineNumbersMinChars: 3,
            glyphMargin: false,
            folding: true,
            smoothScrolling: true,
            scrollBeyondLastLine: false,
            quickSuggestions: { other: true, comments: false, strings: false },
            suggestOnTriggerCharacters: true,
            wordBasedSuggestions: 'off',
            renderLineHighlight: 'line',
            overviewRulerLanes: 0,
            hideCursorInOverviewRuler: true,
            scrollbar: { verticalScrollbarSize: 9, horizontalScrollbarSize: 9, useShadows: false },
            find: { addExtraSpaceOnTop: false, autoFindInSelection: 'never' },
            bracketPairColorization: { enabled: true },
            guides: { bracketPairs: true, indentation: false },
            fixedOverflowWidgets: true,
          }}
      />
    </div>
  </div>
}
