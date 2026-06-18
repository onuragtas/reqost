import { reactive } from 'vue'
import { Events } from '@wailsio/runtime'
import { Connect, Close } from '../../bindings/reqost/sseservice'
import type { HeaderRow } from './useTabs'

// SSE frame state per tab id. Module-level so messages survive subtab switch.

export interface SseFrame { ts: number; type: string; data: string }
export interface SseState {
  state: 'idle' | 'connecting' | 'open' | 'closed' | 'error'
  frames: SseFrame[]
  error: string
}

const map = reactive<Record<string, SseState>>({})

function ensure(id: string): SseState {
  if (!map[id]) map[id] = { state: 'idle', frames: [], error: '' }
  return map[id]
}

let listenerAttached = false
function attachListener() {
  if (listenerAttached) return
  listenerAttached = true
  Events.On('sse:event', (ev: any) => {
    const d = ev?.data ?? ev
    const id = d?.connId
    if (!id || !map[id]) return
    const s = map[id]
    const f: SseFrame = { ts: d.ts ?? Date.now(), type: d.type, data: d.data ?? '' }
    if (d.type === 'open')       { s.state = 'open' }
    else if (d.type === 'close') { s.state = 'closed' }
    else if (d.type === 'error') { s.state = 'error'; s.error = d.data ?? '' }
    s.frames.push(f)
    if (s.frames.length > 1000) s.frames.splice(0, s.frames.length - 1000)
  })
}

export function useSse() {
  attachListener()
  function get(id: string) { return ensure(id) }

  async function connect(id: string, url: string, headers: HeaderRow[]) {
    const s = ensure(id)
    s.state = 'connecting'
    s.error = ''
    s.frames = []
    try {
      await Connect(id, url, headers.filter(h => h.enabled !== false && h.key.trim()))
    } catch (e: any) {
      s.state = 'error'
      s.error = e?.message ?? String(e)
    }
  }
  async function disconnect(id: string) {
    try { await Close(id) } catch { /* ignore */ }
    const s = ensure(id)
    s.state = 'closed'
  }
  function clear(id: string) {
    const s = ensure(id)
    s.frames = []
    s.error = ''
  }

  return { get, connect, disconnect, clear }
}
