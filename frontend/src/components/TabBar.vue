<script setup lang="ts">
import { useTabs, isDirty } from '../composables/useTabs'
import { useEnv } from '../composables/useEnv'
import { useDialog } from '../composables/useDialog'

const { tabs, activeId, selectTab, closeTab } = useTabs()
const { environments, activeId: envActiveId, setActive, openModal } = useEnv()
const dialog = useDialog()

async function maybeClose(id: string) {
  const t = tabs.value.find(tab => tab.id === id)
  if (t && isDirty(t)) {
    const ok = await dialog.confirm(`Close "${t.name}"? Unsaved changes will be lost.`)
    if (!ok) return
  }
  closeTab(id)
}

const METHOD_COLORS: Record<string, string> = {
  GET: '#61affe', POST: '#49cc90', PUT: '#fca130', PATCH: '#50e3c2', DELETE: '#f93e3e',
}
</script>

<template>
  <div class="tabbar">
    <div
      v-for="t in tabs"
      :key="t.id"
      class="tab"
      :class="{ active: t.id === activeId }"
      @click="selectTab(t.id)"
      @mousedown.middle.prevent="maybeClose(t.id)"
    >
      <span class="m" :style="{ color: METHOD_COLORS[t.method] ?? 'var(--text-dim)' }">{{ t.method }}</span>
      <span class="name">{{ t.name }}</span>
      <span v-if="isDirty(t)" class="dirty" title="Unsaved changes"></span>
      <button class="close" title="Close" @click.stop="maybeClose(t.id)">✕</button>
    </div>

    <div class="env">
      <select
        class="env-select"
        :value="envActiveId"
        @change="setActive(($event.target as HTMLSelectElement).value)"
      >
        <option value="">No Environment</option>
        <option v-for="e in environments" :key="e.id" :value="e.id">{{ e.name || 'Untitled' }}</option>
      </select>
      <button class="env-gear" title="Manage environments" @click="openModal">
        <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M19.1 4.9L17 7M7 17l-2.1 2.1"/></svg>
      </button>
    </div>
  </div>
</template>

<style scoped>
.tabbar {
  display: flex;
  align-items: stretch;
  height: 36px;
  background: var(--bg-panel);
  border-bottom: 1px solid var(--border);
  overflow-x: auto;
  flex-shrink: 0;
}
.tabbar::-webkit-scrollbar { height: 0; }

.tab {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 10px;
  max-width: 200px;
  border-right: 1px solid var(--border);
  color: var(--text-dim);
  cursor: pointer;
  flex-shrink: 0;
}
.tab:hover { background: var(--bg-hover); }
.tab.active {
  background: var(--bg);
  color: var(--text);
  box-shadow: inset 0 2px 0 var(--accent);
}

.m { font: 700 9px monospace; flex-shrink: 0; }
.name {
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dirty { width: 7px; height: 7px; border-radius: 50%; background: var(--accent); flex-shrink: 0; }
.tab:hover .dirty { display: none; }
.close {
  color: var(--text-faint);
  font-size: 11px;
  width: 16px;
  height: 16px;
  border-radius: 4px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}
.close:hover { background: var(--border-strong); color: var(--text); }

.env { display: flex; align-items: center; gap: 4px; margin-left: auto; padding: 0 8px; flex-shrink: 0; }
.env-select {
  background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px;
  color: var(--text-dim); font-size: 12px; padding: 4px 6px; max-width: 160px;
}
.env-select:focus { outline: none; border-color: var(--accent); }
.env-gear { width: 26px; height: 26px; border-radius: 5px; display: flex; align-items: center; justify-content: center; color: var(--text-dim); }
.env-gear:hover { background: var(--bg-hover); color: var(--text); }
.env-gear svg { width: 15px; height: 15px; fill: none; stroke: currentColor; stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
</style>
