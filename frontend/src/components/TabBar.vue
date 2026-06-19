<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useTabs, isDirty } from '../composables/useTabs'
import { useEnv } from '../composables/useEnv'
import { useDialog } from '../composables/useDialog'

const { tabs, activeId, selectTab, closeTab, moveTab, pinTab, openAdhoc } = useTabs()
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

// ── Right-click context menu (G7) ──
interface MenuItem { label: string; run: () => void; danger?: boolean }
const menu = ref<{ x: number; y: number; items: MenuItem[] } | null>(null)
function closeMenu() { menu.value = null }

function openTabMenu(e: MouseEvent, id: string) {
  const idx = tabs.value.findIndex(t => t.id === id)
  const t = tabs.value[idx]
  const items: MenuItem[] = [
    { label: t?.pinned ? 'Unpin'        : 'Pin tab',   run: () => pinTab(id) },
    { label: 'Close',                   run: () => maybeClose(id) },
    { label: 'Close Others',            run: () => closeOthers(id) },
    { label: 'Close to the Right',      run: () => closeToTheRight(idx) },
    { label: 'Close All',               run: () => closeAll(),        danger: true },
  ]
  menu.value = { x: e.clientX, y: e.clientY, items }
}

// Pinned tabs are kept by Close Others / Close All — that's the contract
// every modern editor has trained users on.
async function closeOthers(keepId: string) {
  for (const t of [...tabs.value]) {
    if (t.id !== keepId && !t.pinned) await maybeClose(t.id)
  }
}
async function closeToTheRight(fromIdx: number) {
  const ids = tabs.value.slice(fromIdx + 1).filter(t => !t.pinned).map(t => t.id)
  for (const id of ids) await maybeClose(id)
}
async function closeAll() {
  for (const t of [...tabs.value]) {
    if (!t.pinned) await maybeClose(t.id)
  }
}

// ── Drag-reorder ──
const dragFromIdx = ref<number | null>(null)
const dropIdx     = ref<number | null>(null)

function onDragStart(e: DragEvent, idx: number) {
  dragFromIdx.value = idx
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(idx))
  }
}
function onDragOver(e: DragEvent, idx: number) {
  if (dragFromIdx.value === null || dragFromIdx.value === idx) return
  // Pin groups don't mix — refuse drop if the two sides of the line differ.
  const from = tabs.value[dragFromIdx.value]
  const to   = tabs.value[idx]
  if (!from || !to || from.pinned !== to.pinned) return
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
  dropIdx.value = idx
}
function onDrop(idx: number) {
  if (dragFromIdx.value !== null) moveTab(dragFromIdx.value, idx)
  dragFromIdx.value = null
  dropIdx.value = null
}
function onDragEnd() {
  dragFromIdx.value = null
  dropIdx.value = null
}

// Drop a URL (browser address bar, link in another app) onto empty tab-bar
// space → open an adhoc tab. We only react if the drop carries `text/uri-list`
// or a plain URL string — won't fight the tab-reorder DnD.
function onBarDragOver(e: DragEvent) {
  if (dragFromIdx.value !== null) return // tab reorder in progress
  if (!e.dataTransfer) return
  if (e.dataTransfer.types.includes('text/uri-list') || e.dataTransfer.types.includes('text/plain')) {
    e.preventDefault()
    e.dataTransfer.dropEffect = 'copy'
  }
}
function onBarDrop(e: DragEvent) {
  if (dragFromIdx.value !== null) return
  const url = (e.dataTransfer?.getData('text/uri-list') || e.dataTransfer?.getData('text/plain') || '').trim()
  if (!url) return
  if (!/^https?:\/\/|^wss?:\/\/|^grpc:\/\/|^sse:\/\//.test(url)) return
  let name = url
  try { name = new URL(url).host } catch { /* keep raw */ }
  openAdhoc({ name, method: 'GET', url })
}

// Cmd+1 .. Cmd+9 → switch to nth tab (Postman/Insomnia parity).
function onKey(e: KeyboardEvent) {
  if (!(e.metaKey || e.ctrlKey) || e.shiftKey || e.altKey) return
  if (e.key < '1' || e.key > '9') return
  const idx = Number(e.key) - 1
  const t = tabs.value[idx]
  if (t) { e.preventDefault(); selectTab(t.id) }
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))

const METHOD_COLORS: Record<string, string> = {
  GET: '#61affe', POST: '#49cc90', PUT: '#fca130', PATCH: '#50e3c2', DELETE: '#f93e3e',
}
</script>

<template>
  <div class="tabbar" @dragover="onBarDragOver" @drop.prevent="onBarDrop">
    <div
      v-for="(t, i) in tabs"
      :key="t.id"
      class="tab"
      :class="{ active: t.id === activeId, pinned: t.pinned, 'drop-target': dropIdx === i }"
      :title="`${t.method} ${t.url || t.name}`"
      :draggable="true"
      @click="selectTab(t.id)"
      @mousedown.middle.prevent="maybeClose(t.id)"
      @contextmenu.prevent="openTabMenu($event, t.id)"
      @dragstart="onDragStart($event, i)"
      @dragover="onDragOver($event, i)"
      @dragleave="dropIdx = null"
      @drop.prevent="onDrop(i)"
      @dragend="onDragEnd"
    >
      <span v-if="t.pinned" class="pin-mark" title="Pinned">📌</span>
      <span class="m" :style="{ color: METHOD_COLORS[t.method] ?? 'var(--text-dim)' }">{{ t.method }}</span>
      <span class="name">{{ t.name }}</span>
      <span v-if="isDirty(t)" class="dirty" title="Unsaved changes"></span>
      <button class="close" title="Close" @click.stop="maybeClose(t.id)">✕</button>
    </div>

    <!-- context menu -->
    <template v-if="menu">
      <div class="ctx-overlay" @click="closeMenu" @contextmenu.prevent="closeMenu" />
      <div class="ctx-menu" :style="{ left: `${menu.x}px`, top: `${menu.y}px` }">
        <button
          v-for="(it, i) in menu.items" :key="i"
          class="ctx-item" :class="{ danger: it.danger }"
          @click="it.run(); closeMenu()"
        >{{ it.label }}</button>
      </div>
    </template>

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
.tab.pinned { background: color-mix(in srgb, var(--accent) 8%, transparent); }
.tab.drop-target { box-shadow: inset 2px 0 0 var(--accent); }
.pin-mark { font-size: 9px; flex-shrink: 0; opacity: 0.75; }

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

.ctx-overlay { position: fixed; inset: 0; z-index: 200; }
.ctx-menu {
  position: fixed; z-index: 201; min-width: 170px;
  background: var(--bg-elevated); border: 1px solid var(--border-strong);
  border-radius: 8px; padding: 4px;
  box-shadow: 0 10px 30px rgba(0,0,0,0.4);
  display: flex; flex-direction: column;
}
.ctx-item { text-align: left; font-size: 12px; color: var(--text); padding: 7px 10px; border-radius: 5px; }
.ctx-item:hover { background: var(--bg-hover); }
.ctx-item.danger { color: var(--danger); }
</style>
