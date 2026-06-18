import { ref } from 'vue'
import type { HeaderRow, Auth } from './useTabs'

export interface HistoryEntry {
  id: string
  ts: number
  name: string
  method: string
  url: string
  headers: HeaderRow[]
  body: string
  auth: Auth
  status: number
  ms: number
  ok: boolean
}

const STORAGE_KEY = 'reqost:history'
const CAP = 200

function load(): HistoryEntry[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? JSON.parse(raw) : []
  } catch {
    return []
  }
}

// Module-level shared history.
const entries = ref<HistoryEntry[]>(load())

function persist() {
  try { localStorage.setItem(STORAGE_KEY, JSON.stringify(entries.value)) } catch { /* quota */ }
}

function genId(): string {
  try { return crypto.randomUUID() } catch { return `h-${Date.now()}-${Math.floor(Math.random() * 1e6)}` }
}

export function useHistory() {
  function record(e: Omit<HistoryEntry, 'id' | 'ts'>) {
    entries.value.unshift({ ...e, id: genId(), ts: Date.now() })
    if (entries.value.length > CAP) entries.value.length = CAP
    persist()
  }
  function clear() {
    entries.value = []
    persist()
  }
  return { entries, record, clear }
}
