import { reactive } from 'vue'
import { Events } from '@wailsio/runtime'
import { Connect, Send, Close } from '../../bindings/reqost/tcpservice'

export type SendMode = 'text' | 'line' | 'crlf' | 'hex'

export interface TcpMessage {
  dir: 'in' | 'out' | 'sys'
  text: string  // UTF-8 rendering (data frames) or status text (sys frames)
  hex: string   // hex rendering (data frames only)
  bytes: number // payload length (data frames)
  ts: number
}
interface TcpState { connected: boolean; messages: TcpMessage[] }

// One state per connId (we key by tab id).
const conns = reactive<Record<string, TcpState>>({})

function ensure(id: string): TcpState {
  if (!conns[id]) conns[id] = { connected: false, messages: [] }
  return conns[id]
}

function b64ToBytes(b64: string): Uint8Array {
  const bin = atob(b64)
  const out = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
  return out
}
function bytesToText(bytes: Uint8Array): string {
  try { return new TextDecoder().decode(bytes) } catch { return '' }
}
function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join(' ')
}

let wired = false
function wire() {
  if (wired) return
  wired = true
  Events.On('tcp:event', (ev: any) => {
    const p = ev?.data ?? ev
    const st = ensure(p.connId)
    switch (p.type) {
      case 'open':
        st.connected = true
        st.messages.push({ dir: 'sys', text: p.data, hex: '', bytes: 0, ts: p.ts })
        break
      case 'data': {
        const bytes = b64ToBytes(p.data)
        st.messages.push({ dir: p.dir, text: bytesToText(bytes), hex: bytesToHex(bytes), bytes: bytes.length, ts: p.ts })
        break
      }
      case 'close':
        st.connected = false
        st.messages.push({ dir: 'sys', text: `Closed: ${p.data}`, hex: '', bytes: 0, ts: p.ts })
        break
      case 'error':
        st.messages.push({ dir: 'sys', text: `Error: ${p.data}`, hex: '', bytes: 0, ts: p.ts })
        break
    }
  })
}

export function useTcp() {
  wire()
  function state(id: string) { return ensure(id) }
  async function connect(id: string, url: string) {
    ensure(id).messages.push({ dir: 'sys', text: `Connecting to ${url}…`, hex: '', bytes: 0, ts: Date.now() })
    await Connect(id, url)
  }
  function send(id: string, data: string, mode: SendMode) {
    if (!data && mode !== 'line' && mode !== 'crlf') return
    Send(id, data, mode)
  }
  function disconnect(id: string) { Close(id) }
  function clearMessages(id: string) { ensure(id).messages = [] }

  return { state, connect, send, disconnect, clearMessages }
}
