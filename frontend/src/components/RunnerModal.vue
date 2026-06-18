<script setup lang="ts">
import { computed } from 'vue'
import { useRunner } from '../composables/useRunner'

const { state, close } = useRunner()

const METHOD_COLORS: Record<string, string> = {
  GET: '#61affe', POST: '#49cc90', PUT: '#fca130', PATCH: '#50e3c2', DELETE: '#f93e3e',
}
const totalPassed = computed(() => state.rows.reduce((n, r) => n + r.passed, 0))
const totalTests = computed(() => state.rows.reduce((n, r) => n + r.total, 0))
const failed = computed(() => state.rows.filter(r => !r.ok || (r.total > 0 && r.passed < r.total)).length)

function statusColor(s: number) {
  if (s >= 200 && s < 300) return 'var(--ok)'
  if (s >= 400 || s === 0) return 'var(--danger)'
  return 'var(--warn-text)'
}
</script>

<template>
  <div v-if="state.open" class="overlay" @click.self="close">
    <div class="modal">
      <header>
        <h3>Collection Runner</h3>
        <button class="x" @click="close">✕</button>
      </header>

      <div class="summary">
        <span class="prog">{{ state.done }}/{{ state.total }}</span>
        <span v-if="state.running" class="spin">running…</span>
        <template v-else>
          <span class="ok">{{ totalPassed }}/{{ totalTests }} tests passed</span>
          <span v-if="failed" class="bad">{{ failed }} failed</span>
        </template>
      </div>

      <div class="rows selectable">
        <div v-for="r in state.rows" :key="r.id" class="row">
          <span class="m" :style="{ color: METHOD_COLORS[r.method] ?? 'var(--text-dim)' }">{{ r.method }}</span>
          <span class="name">{{ r.name }}</span>
          <span class="tests" v-if="r.total">{{ r.passed }}/{{ r.total }}</span>
          <span class="ms">{{ Math.round(r.ms) }}ms</span>
          <span class="st" :style="{ color: statusColor(r.status) }">{{ r.status || '✕' }}</span>
        </div>
        <div v-if="!state.rows.length && !state.running" class="empty">No requests to run</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { width: 640px; max-width: 92vw; height: 66vh; background: var(--bg-elevated); border: 1px solid var(--border-strong); border-radius: 10px; display: flex; flex-direction: column; overflow: hidden; box-shadow: 0 20px 60px rgba(0,0,0,0.5); }
header { display: flex; align-items: center; justify-content: space-between; padding: 14px 18px; border-bottom: 1px solid var(--border); }
h3 { font-size: 15px; font-weight: 600; }
.x { color: var(--text-dim); width: 24px; height: 24px; border-radius: 5px; }
.x:hover { background: var(--bg-hover); color: var(--text); }
.summary { display: flex; align-items: center; gap: 14px; padding: 10px 18px; border-bottom: 1px solid var(--border); font-size: 12px; }
.prog { font: 700 13px monospace; }
.spin { color: var(--accent); }
.ok { color: var(--ok); }
.bad { color: var(--danger); }
.rows { flex: 1; overflow-y: auto; padding: 6px 0; }
.row { display: flex; align-items: center; gap: 10px; padding: 7px 18px; border-bottom: 1px solid var(--border); font-size: 12px; }
.m { font: 700 9px monospace; width: 42px; flex-shrink: 0; }
.name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.tests { color: var(--text-dim); font: 11px monospace; }
.ms { color: var(--text-faint); font: 11px monospace; width: 54px; text-align: right; }
.st { font: 700 12px monospace; width: 36px; text-align: right; flex-shrink: 0; }
.empty { display: flex; align-items: center; justify-content: center; height: 100%; color: var(--text-faint); }
</style>
