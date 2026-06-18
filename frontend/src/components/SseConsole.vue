<script setup lang="ts">
import { computed } from 'vue'
import type { ReqTab } from '../composables/useTabs'
import { useSse } from '../composables/useSse'

const props = defineProps<{ tab: ReqTab }>()
const { get, connect, disconnect, clear } = useSse()

const state = computed(() => get(props.tab.id))

function realUrl(): string {
  const u = props.tab.url.trim()
  if (u.startsWith('sses://')) return 'https://' + u.slice(7)
  if (u.startsWith('sse://'))  return 'http://'  + u.slice(6)
  return u
}
async function onConnect()    { await connect(props.tab.id, realUrl(), props.tab.headers) }
async function onDisconnect() { await disconnect(props.tab.id) }
function onClear()            { clear(props.tab.id) }

function fmtTime(ms: number): string {
  const d = new Date(ms)
  return d.toLocaleTimeString() + '.' + String(d.getMilliseconds()).padStart(3, '0')
}
</script>

<template>
  <div class="sse">
    <div class="bar">
      <select v-model="tab.method" class="method" disabled>
        <option value="GET">GET</option>
      </select>
      <input v-model="tab.url" class="url" placeholder="https://api.example.com/stream  (text/event-stream)" />
      <button
        v-if="state.state !== 'open' && state.state !== 'connecting'"
        class="connect" @click="onConnect"
      >Connect</button>
      <button v-else class="disconnect" @click="onDisconnect">
        {{ state.state === 'connecting' ? 'Connecting…' : 'Disconnect' }}
      </button>
    </div>

    <div class="status">
      <span class="dot" :class="state.state"></span>
      <span class="stat-text">{{ state.state }}</span>
      <span v-if="state.error" class="err">— {{ state.error }}</span>
      <button class="clear" @click="onClear">Clear</button>
    </div>

    <div class="frames selectable">
      <div v-if="!state.frames.length" class="empty">No events received yet.</div>
      <div v-for="(f, i) in state.frames" :key="i" class="frame" :class="f.type">
        <span class="t">{{ fmtTime(f.ts) }}</span>
        <span class="typ">{{ f.type }}</span>
        <pre class="data">{{ f.data }}</pre>
      </div>
    </div>
  </div>
</template>

<style scoped>
.sse { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.bar { display: flex; gap: 8px; padding: 10px 16px; background: var(--bg-elevated); border-bottom: 1px solid var(--border); flex-shrink: 0; }
.method { background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; font: 700 12px monospace; padding: 0 8px; opacity: 0.6; }
.url { flex: 1; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; font: 13px monospace; padding: 8px 10px; color: var(--text); }
.url:focus { outline: none; border-color: var(--accent); }
.connect { background: var(--accent); color: var(--accent-text); border-radius: 5px; font-weight: 700; padding: 0 22px; }
.disconnect { background: var(--danger); color: #fff; border-radius: 5px; font-weight: 700; padding: 0 18px; }
.status { display: flex; align-items: center; gap: 10px; padding: 6px 16px; font-size: 11px; color: var(--text-dim); background: var(--bg-panel); border-bottom: 1px solid var(--border); }
.dot { width: 8px; height: 8px; border-radius: 50%; background: var(--text-faint); }
.dot.open       { background: var(--ok); }
.dot.connecting { background: var(--warn-text); animation: pulse 1.2s ease-in-out infinite; }
.dot.closed     { background: var(--text-faint); }
.dot.error      { background: var(--danger); }
@keyframes pulse { 0%,100% { opacity: 1 } 50% { opacity: 0.3 } }
.err { color: var(--danger); }
.clear { margin-left: auto; font-size: 11px; color: var(--text-dim); padding: 2px 8px; border-radius: 4px; }
.clear:hover { background: var(--bg-hover); color: var(--text); }
.frames { flex: 1; overflow: auto; padding: 8px 14px; font: 12px/1.5 monospace; display: flex; flex-direction: column; gap: 6px; }
.empty { color: var(--text-faint); text-align: center; padding: 30px; font-size: 12px; }
.frame { display: grid; grid-template-columns: 95px 80px 1fr; gap: 8px; padding: 4px 0; border-bottom: 1px dashed var(--border); }
.frame .t { color: var(--text-faint); font-size: 11px; }
.frame .typ { color: var(--accent); font-size: 11px; text-transform: uppercase; }
.frame.error .typ { color: var(--danger); }
.frame.close .typ { color: var(--text-faint); }
.frame .data { margin: 0; color: var(--text); white-space: pre-wrap; word-break: break-word; }
</style>
