import type { InputHTMLAttributes } from 'react'
import type { DataColumn } from '@/entities/database-object'

export type EditorValueKind = 'boolean' | 'integer' | 'exact-integer' | 'exact-decimal' | 'float' | 'date' | 'datetime-local' | 'json' | 'text'

type ParsedEditorValue = { ok: true; value: unknown } | { ok: false; error: string }

const integerPattern = /^[+-]?\d+$/
const decimalPattern = /^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/

export function editorValueKind(type: string): EditorValueKind {
  const normalized = type.toLowerCase()
  if (normalized.includes('bool')) return 'boolean'
  if (normalized.includes('bigint') || normalized.includes('int8') || normalized.includes('bigserial')) return 'exact-integer'
  if (normalized.includes('numeric') || normalized.includes('decimal') || /(^|\W)number(\W|$)/.test(normalized)) return 'exact-decimal'
  if (normalized.includes('int') || normalized.includes('serial')) return 'integer'
  if (normalized.includes('float') || normalized.includes('double') || normalized.includes('real')) return 'float'
  if (normalized.trim() === 'date') return 'date'
  if (normalized.includes('timestamp') || normalized.includes('datetime')) return 'datetime-local'
  if (normalized.includes('json')) return 'json'
  return 'text'
}

function pad(value: number, length = 2) {
  return String(value).padStart(length, '0')
}

function localDateTimeValue(value: unknown) {
  const date = new Date(String(value))
  if (Number.isNaN(date.getTime())) return String(value)
  const base = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
  return date.getMilliseconds() ? `${base}.${pad(date.getMilliseconds(), 3)}` : base
}

export function editorInputValue(value: unknown, kind: EditorValueKind) {
  if (value === null || value === undefined) return ''
  if (kind === 'json') return typeof value === 'string' ? value : JSON.stringify(value)
  if (kind === 'datetime-local') return localDateTimeValue(value)
  return String(value)
}

export function editorInputType(kind: EditorValueKind): 'text' | 'number' | 'date' | 'datetime-local' {
  if (kind === 'integer' || kind === 'float') return 'number'
  if (kind === 'date') return 'date'
  if (kind === 'datetime-local') return 'datetime-local'
  return 'text'
}

export function editorInputMode(kind: EditorValueKind): InputHTMLAttributes<HTMLInputElement>['inputMode'] {
  if (kind === 'exact-integer' || kind === 'integer') return 'numeric'
  if (kind === 'exact-decimal' || kind === 'float') return 'decimal'
  return undefined
}

export function editorInputStep(kind: EditorValueKind) {
  if (kind === 'integer') return '1'
  if (kind === 'float') return 'any'
  if (kind === 'datetime-local') return '0.001'
  return undefined
}

export function parseEditorValue(raw: string, kind: EditorValueKind, nullable?: boolean): ParsedEditorValue {
  const trimmed = raw.trim()
  if (!trimmed) return nullable ? { ok: true, value: null } : { ok: false, error: 'A value is required' }
  if (kind === 'exact-integer') return integerPattern.test(trimmed) ? { ok: true, value: trimmed } : { ok: false, error: 'Enter a valid whole number' }
  if (kind === 'exact-decimal') return decimalPattern.test(trimmed) ? { ok: true, value: trimmed } : { ok: false, error: 'Enter a valid decimal number' }
  if (kind === 'integer') {
    if (!integerPattern.test(trimmed)) return { ok: false, error: 'Enter a valid whole number' }
    const value = Number(trimmed)
    return Number.isSafeInteger(value) ? { ok: true, value } : { ok: false, error: 'The integer is outside the safe range' }
  }
  if (kind === 'float') {
    const value = Number(trimmed)
    return Number.isFinite(value) ? { ok: true, value } : { ok: false, error: 'Enter a valid number' }
  }
  if (kind === 'boolean') {
    if (trimmed !== 'true' && trimmed !== 'false') return { ok: false, error: 'Choose true or false' }
    return { ok: true, value: trimmed === 'true' }
  }
  if (kind === 'json') {
    try {
      return { ok: true, value: JSON.parse(raw) }
    } catch {
      return { ok: false, error: 'Enter valid JSON' }
    }
  }
  if (kind === 'datetime-local') {
    const date = new Date(raw)
    return Number.isNaN(date.getTime()) ? { ok: false, error: 'Enter a valid date and time' } : { ok: true, value: date.toISOString() }
  }
  return { ok: true, value: raw }
}

export function validateEditorValue(column: DataColumn, value: unknown) {
  if ((value === null || value === undefined || value === '') && column.nullable) return undefined
  if (value === null || value === undefined || value === '') return `${column.name} is required`
  const kind = editorValueKind(column.type)
  if (kind === 'json' && typeof value !== 'string') return undefined
  if ((kind === 'exact-integer' || kind === 'exact-decimal') && typeof value === 'number') {
    if (!Number.isFinite(value) || kind === 'exact-integer' && !Number.isSafeInteger(value)) return `${column.name} must contain a valid ${kind === 'exact-integer' ? 'whole' : 'decimal'} number`
    return undefined
  }
  const parsed = parseEditorValue(String(value), kind, column.nullable)
  return parsed.ok ? undefined : `${column.name}: ${parsed.error}`
}
