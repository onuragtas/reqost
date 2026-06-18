<script setup lang="ts">
import { ref, computed } from 'vue'
import { useWs } from '../composables/useWs'
import { useEnv } from '../composables/useEnv'
import { interpolate } from '../composables/interpolate'
import type { ReqTab } from '../composables/useTabs'

const props = defineProps<{ tab: ReqTab }>()
const { state, connect, send, disconnect, clearMessages } = useWs()
const { activeVars } = useEnv()

const draft = ref('')
const st = computed(() => state(props.tab.id))

function onConnect() {
  const headers = props.tab.headers.map(h => ({
    ...h, key: interpolate(h.key, activeVars.value), value: interpolate(h.value, activeVars.value),
  }))
  connect(props.tab.id, interpolate(props.tab.url.trim(), activeVars.value), headers)
}
function onSend() {
  send(props.tab.id, draft.value)
  draft.value = ''
}

function fmtTime(ts: number) {
  const d = new Date(ts)
  return d.toLocaleTimeString()
}
</script>

<template>
  <div class="ws">
    <div class="ws-bar">
      <span class="proto">WS</span>
      <input v-model="tab.url" class="url" placeholder="wss://echo.websocket.org" @keyup.enter="onConnect" />
      <button v-if="!st.connected" class="connect" @click="onConnect">Connect</button>
      <button v-else class="disconnect" @click="disconnect(tab.id)">Disconnect</button>
    </div>

    <div class="msgs selectable">
      <div v-if="!st.messages.length" class="empty">Connect, then send messages</div>
      <div v-for="(m, i) in st.messages" :key="i" class="msg" :class="m.dir">
        <span class="arrow">{{ m.dir === 'in' ? '↓' : m.dir === 'out' ? '↑' : '•' }}</span>
        <span class="data">{{ m.data }}</span>
        <span class="time">{{ fmtTime(m.ts) }}</span>
      </div>
    </div>

    <div class="composer">
      <textarea v-model="draft" class="draft" :disabled="!st.connected" placeholder="Message…" @keydown.enter.exact.prevent="onSend" />
      <div class="composer-actions">
        <button class="clear" @click="clearMessages(tab.id)">Clear</button>
        <button class="send" :disabled="!st.connected" @click="onSend">Send</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ws { flex: 1; display: flex; flex-direction: column; overflow: hidden; background: var(--bg); }
.ws-bar { display: flex; gap: 8px; align-items: center; padding: 12px 16px; border-bottom: 1px solid var(--border); background: var(--bg-elevated); }
.proto { font: 700 11px monospace; color: var(--accent); background: color-mix(in srgb, var(--accent) 15%, transparent); padding: 4px 7px; border-radius: 4px; }
.url { flex: 1; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; font: 13px monospace; padding: 8px 10px; }
.url:focus { outline: none; border-color: var(--accent); }
.connect { background: var(--accent); color: var(--accent-text); border-radius: 5px; font-weight: 700; padding: 0 18px; }
.disconnect { background: var(--danger); color: #fff; border-radius: 5px; font-weight: 700; padding: 0 16px; }

.msgs { flex: 1; overflow-y: auto; padding: 8px 16px; }
.empty { display: flex; align-items: center; justify-content: center; height: 100%; color: var(--text-faint); font-size: 13px; }
.msg { display: flex; gap: 8px; padding: 5px 0; border-bottom: 1px solid var(--border); font: 12px monospace; }
.msg .arrow { flex-shrink: 0; width: 12px; }
.msg.in .arrow { color: #61affe; }
.msg.out .arrow { color: var(--ok); }
.msg.sys { color: var(--text-faint); font-style: italic; }
.msg .data { flex: 1; white-space: pre-wrap; word-break: break-word; }
.msg .time { color: var(--text-faint); font-size: 10px; flex-shrink: 0; }

.composer { border-top: 1px solid var(--border); padding: 10px 16px; display: flex; flex-direction: column; gap: 8px; background: var(--bg-elevated); }
.draft { width: 100%; min-height: 56px; background: var(--bg-input); border: 1px solid var(--border); border-radius: 5px; color: var(--text); font: 12px/1.4 monospace; padding: 8px; resize: vertical; }
.draft:focus { outline: none; border-color: var(--accent); }
.composer-actions { display: flex; justify-content: flex-end; gap: 8px; }
.clear { color: var(--text-dim); font-size: 12px; padding: 6px 12px; border-radius: 5px; }
.clear:hover { background: var(--bg-hover); }
.send { background: var(--accent); color: var(--accent-text); border-radius: 5px; font-weight: 700; padding: 6px 18px; }
.send:disabled { opacity: 0.5; }
</style>
