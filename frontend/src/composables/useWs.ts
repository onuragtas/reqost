import { reactive } from 'vue'
import { Events } from '@wailsio/runtime'
import { Connect, Send, Close } from '../../bindings/reqost/wsservice'
import type { HeaderRow } from './useTabs'

export interface WsMessage { dir: 'in' | 'out' | 'sys'; data: string; ts: number }
interface WsState { connected: boolean; messages: WsMessage[] }

// One state per connId (we key by tab id).
const conns = reactive<Record<string, WsState>>({})

function ensure(id: string): WsState {
  if (!conns[id]) conns[id] = { connected: false, messages: [] }
  return conns[id]
}

// Single global listener fans events out by connId.
let wired = false
function wire() {
  if (wired) return
  wired = true
  Events.On('ws:event', (ev: any) => {
    const p = ev?.data ?? ev
    const st = ensure(p.connId)
    switch (p.type) {
      case 'open':
        st.connected = true
        st.messages.push({ dir: 'sys', data: `Connected to ${p.data}`, ts: p.ts })
        break
      case 'message':
        st.messages.push({ dir: p.dir, data: p.data, ts: p.ts })
        break
      case 'close':
        st.connected = false
        st.messages.push({ dir: 'sys', data: `Closed: ${p.data}`, ts: p.ts })
        break
      case 'error':
        st.messages.push({ dir: 'sys', data: `Error: ${p.data}`, ts: p.ts })
        break
    }
  })
}

export function useWs() {
  wire()
  function state(id: string) { return ensure(id) }
  async function connect(id: string, url: string, headers: HeaderRow[]) {
    ensure(id).messages.push({ dir: 'sys', data: `Connecting to ${url}…`, ts: Date.now() })
    await Connect(id, url, headers.filter(h => h.enabled !== false && h.key.trim()) as any)
  }
  function send(id: string, text: string) {
    if (!text) return
    Send(id, text)
  }
  function disconnect(id: string) { Close(id) }
  function clearMessages(id: string) { ensure(id).messages = [] }

  return { state, connect, send, disconnect, clearMessages }
}
