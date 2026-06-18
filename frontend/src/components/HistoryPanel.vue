<script setup lang="ts">
import { useHistory } from '../composables/useHistory'
import { useTabs } from '../composables/useTabs'

const { entries, clear } = useHistory()
const { openAdhoc } = useTabs()

const METHOD_COLORS: Record<string, string> = {
  GET: '#61affe', POST: '#49cc90', PUT: '#fca130', PATCH: '#50e3c2', DELETE: '#f93e3e',
}

function statusColor(s: number) {
  if (s >= 200 && s < 300) return 'var(--ok)'
  if (s >= 400) return 'var(--danger)'
  return 'var(--text-dim)'
}

function open(e: typeof entries.value[number]) {
  openAdhoc({ name: e.name, method: e.method, url: e.url, headers: e.headers, body: e.body, auth: e.auth })
}

function ago(ts: number): string {
  const s = Math.floor((Date.now() - ts) / 1000)
  if (s < 60) return `${s}s`
  if (s < 3600) return `${Math.floor(s / 60)}m`
  if (s < 86400) return `${Math.floor(s / 3600)}h`
  return `${Math.floor(s / 86400)}d`
}
</script>

<template>
  <div class="history">
    <div class="head">
      <span>History</span>
      <button v-if="entries.length" class="clear" @click="clear">Clear</button>
    </div>
    <div v-if="!entries.length" class="empty">No requests sent yet</div>
    <div v-else class="list">
      <div v-for="e in entries" :key="e.id" class="row" @click="open(e)">
        <span class="m" :style="{ color: METHOD_COLORS[e.method] ?? 'var(--text-dim)' }">{{ e.method }}</span>
        <span class="url">{{ e.url }}</span>
        <span class="st" :style="{ color: statusColor(e.status) }">{{ e.status || '—' }}</span>
        <span class="ago">{{ ago(e.ts) }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.history { display: flex; flex-direction: column; height: 100%; background: var(--bg-panel); border-right: 1px solid var(--border); overflow: hidden; }
.head { display: flex; align-items: center; justify-content: space-between; padding: 10px 12px; font-size: 13px; color: var(--text); flex-shrink: 0; }
.clear { font-size: 11px; color: var(--text-dim); padding: 3px 8px; border-radius: 4px; }
.clear:hover { background: var(--bg-hover); color: var(--danger); }
.empty { display: flex; align-items: center; justify-content: center; flex: 1; color: var(--text-faint); font-size: 12px; }
.list { flex: 1; overflow-y: auto; }
.row { display: flex; align-items: center; gap: 6px; padding: 6px 12px; cursor: pointer; color: var(--text-dim); border-bottom: 1px solid var(--border); }
.row:hover { background: var(--bg-hover); color: var(--text); }
.m { font: 700 9px monospace; width: 38px; flex-shrink: 0; }
.url { flex: 1; font: 11px monospace; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.st { font: 700 11px monospace; flex-shrink: 0; }
.ago { font-size: 10px; color: var(--text-faint); width: 26px; text-align: right; flex-shrink: 0; }
</style>
