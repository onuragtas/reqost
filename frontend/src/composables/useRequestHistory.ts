const ENTRY_CAP = 10
const BODY_CAP = 50_000

export interface ReqHistoryEntry {
  ts: number
  status: number
  ms: number
  body: string
  headers: { key: string; value: string }[]
}

function storageKey(id: string) { return `reqost:rh:${id}` }

function load(id: string): ReqHistoryEntry[] {
  try { return JSON.parse(localStorage.getItem(storageKey(id)) ?? 'null') ?? [] } catch { return [] }
}

export function recordReqHistory(id: string, status: number, ms: number, body: string, headers: any[]) {
  if (!id) return
  const entries = load(id)
  const entry: ReqHistoryEntry = {
    ts: Date.now(), status, ms,
    body: body.length > BODY_CAP ? body.slice(0, BODY_CAP) + '\n…[truncated]' : body,
    headers,
  }
  try {
    localStorage.setItem(storageKey(id), JSON.stringify([entry, ...entries].slice(0, ENTRY_CAP)))
  } catch { /* storage quota */ }
}

export function loadReqHistory(id: string): ReqHistoryEntry[] {
  return id ? load(id) : []
}
