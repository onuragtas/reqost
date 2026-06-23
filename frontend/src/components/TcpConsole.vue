<script setup lang="ts">
import { ref, computed } from 'vue'
import { useTcp, type SendMode } from '../composables/useTcp'
import { useEnv } from '../composables/useEnv'
import { interpolate } from '../composables/interpolate'
import type { ReqTab } from '../composables/useTabs'

const props = defineProps<{ tab: ReqTab }>()
const { state, connect, send, disconnect, clearMessages } = useTcp()
const { activeVars } = useEnv()

const draft = ref('')
const sendMode = ref<SendMode>('crlf')
const hexView = ref(false)   // render received/sent payloads as hex
const st = computed(() => state(props.tab.id))

const proto = computed(() => {
  const u = props.tab.url.trim().toLowerCase()
  if (u.startsWith('tls://')) return 'TLS'
  if (u.startsWith('udp://')) return 'UDP'
  return 'TCP'
})

function onConnect() {
  connect(props.tab.id, interpolate(props.tab.url.trim(), activeVars.value))
}
function onSend() {
  // Text-ish modes interpolate {{vars}}; hex is taken literally.
  const data = sendMode.value === 'hex' ? draft.value : interpolate(draft.value, activeVars.value)
  send(props.tab.id, data, sendMode.value)
  draft.value = ''
}

function body(m: { text: string; hex: string; dir: string }) {
  if (m.dir === 'sys') return m.text
  return hexView.value ? m.hex : m.text
}
function fmtTime(ts: number) {
  return new Date(ts).toLocaleTimeString()
}

const MODES: { id: SendMode; label: string }[] = [
  { id: 'text', label: 'Text' },
  { id: 'line', label: 'Text + LF' },
  { id: 'crlf', label: 'Text + CRLF' },
  { id: 'hex', label: 'Hex' },
]
</script>

<template>
  <div class="tcp">
    <div class="tcp-bar">
      <span class="proto">{{ proto }}</span>
      <input v-model="tab.url" class="url" placeholder="tcp://host:port  ·  tls://host:port  ·  udp://host:port" @keyup.enter="onConnect" />
      <button v-if="!st.connected" class="connect" @click="onConnect">Connect</button>
      <button v-else class="disconnect" @click="disconnect(tab.id)">Disconnect</button>
    </div>

    <div class="msgs-head">
      <span class="hint">Raw socket — {{ st.connected ? 'connected' : 'not connected' }}</span>
      <label class="hexv"><input type="checkbox" v-model="hexView" /> Hex view</label>
    </div>

    <div class="msgs selectable">
      <div v-if="!st.messages.length" class="empty">Connect, then send bytes</div>
      <div v-for="(m, i) in st.messages" :key="i" class="msg" :class="m.dir">
        <span class="arrow">{{ m.dir === 'in' ? '↓' : m.dir === 'out' ? '↑' : '•' }}</span>
        <span class="data">{{ body(m) }}</span>
        <span v-if="m.dir !== 'sys'" class="len">{{ m.bytes }}B</span>
        <span class="time">{{ fmtTime(m.ts) }}</span>
      </div>
    </div>

    <div class="composer">
      <textarea
        v-model="draft" class="draft" :disabled="!st.connected"
        :placeholder="sendMode === 'hex' ? '48 65 6c 6c 6f  (hex bytes)' : 'Bytes to send…'"
        @keydown.enter.exact.prevent="onSend"
      />
      <div class="composer-actions">
        <select v-model="sendMode" class="mode" title="How the input is encoded before sending">
          <option v-for="m in MODES" :key="m.id" :value="m.id">{{ m.label }}</option>
        </select>
        <button class="clear" @click="clearMessages(tab.id)">Clear</button>
        <button class="send" :disabled="!st.connected" @click="onSend">Send</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tcp { flex: 1; display: flex; flex-direction: column; overflow: hidden; background: var(--bg); }
.tcp-bar { display: flex; gap: 8px; align-items: center; padding: 12px 16px; border-bottom: 1px solid var(--border); background: var(--bg-elevated); }
.proto { font: 700 11px monospace; color: var(--accent); background: color-mix(in srgb, var(--accent) 15%, transparent); padding: 4px 7px; border-radius: 4px; min-width: 34px; text-align: center; }
.url { flex: 1; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; font: 13px monospace; padding: 8px 10px; }
.url:focus { outline: none; border-color: var(--accent); }
.connect { background: var(--accent); color: var(--accent-text); border-radius: 5px; font-weight: 700; padding: 0 18px; }
.disconnect { background: var(--danger); color: #fff; border-radius: 5px; font-weight: 700; padding: 0 16px; }

.msgs-head { display: flex; align-items: center; justify-content: space-between; padding: 6px 16px; border-bottom: 1px solid var(--border); }
.msgs-head .hint { font-size: 11px; color: var(--text-faint); }
.hexv { font-size: 11px; color: var(--text-dim); display: flex; align-items: center; gap: 5px; cursor: pointer; }

.msgs { flex: 1; overflow-y: auto; padding: 8px 16px; }
.empty { display: flex; align-items: center; justify-content: center; height: 100%; color: var(--text-faint); font-size: 13px; }
.msg { display: flex; gap: 8px; padding: 5px 0; border-bottom: 1px solid var(--border); font: 12px monospace; }
.msg .arrow { flex-shrink: 0; width: 12px; }
.msg.in .arrow { color: #61affe; }
.msg.out .arrow { color: var(--ok); }
.msg.sys { color: var(--text-faint); font-style: italic; }
.msg .data { flex: 1; white-space: pre-wrap; word-break: break-word; }
.msg .len { color: var(--text-faint); font-size: 10px; flex-shrink: 0; }
.msg .time { color: var(--text-faint); font-size: 10px; flex-shrink: 0; }

.composer { border-top: 1px solid var(--border); padding: 10px 16px; display: flex; flex-direction: column; gap: 8px; background: var(--bg-elevated); }
.draft { width: 100%; min-height: 56px; background: var(--bg-input); border: 1px solid var(--border); border-radius: 5px; color: var(--text); font: 12px/1.4 monospace; padding: 8px; resize: vertical; }
.draft:focus { outline: none; border-color: var(--accent); }
.composer-actions { display: flex; justify-content: flex-end; align-items: center; gap: 8px; }
.mode { margin-right: auto; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; color: var(--text-dim); font-size: 12px; padding: 5px 7px; }
.clear { color: var(--text-dim); font-size: 12px; padding: 6px 12px; border-radius: 5px; }
.clear:hover { background: var(--bg-hover); }
.send { background: var(--accent); color: var(--accent-text); border-radius: 5px; font-weight: 700; padding: 6px 18px; }
.send:disabled { opacity: 0.5; }
</style>
