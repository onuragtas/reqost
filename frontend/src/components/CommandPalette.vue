<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useCommands, type Command } from '../composables/useCommands'
import { useTabs } from '../composables/useTabs'
import { Search } from '../../bindings/reqost/collectionservice'

const { state, close, commands } = useCommands()
const { openRequest } = useTabs()

const query = ref('')
const input = ref<HTMLInputElement | null>(null)
const cursor = ref(0)
const searchResults = ref<any[]>([])

watch(() => state.open, async (open) => {
  if (open) {
    query.value = ''
    cursor.value = 0
    searchResults.value = []
    await nextTick()
    input.value?.focus()
  }
})

// Fuzzy command match: every query char must appear in order (case-insensitive).
function fuzzyMatch(label: string, q: string): boolean {
  if (!q) return true
  const s = label.toLowerCase()
  const needle = q.toLowerCase()
  let j = 0
  for (let i = 0; i < s.length && j < needle.length; i++) {
    if (s[i] === needle[j]) j++
  }
  return j === needle.length
}

const filtered = computed<Command[]>(() => {
  if (state.mode !== 'commands') return []
  const q = query.value.trim()
  return commands().filter(c => fuzzyMatch(c.label + ' ' + (c.hint ?? ''), q))
})

// Debounced FTS5 search via the backend for quick switcher.
let searchTimer: ReturnType<typeof setTimeout> | undefined
watch([query, () => state.mode], () => {
  if (state.mode !== 'search') return
  clearTimeout(searchTimer)
  const q = query.value.trim()
  if (!q) { searchResults.value = []; return }
  searchTimer = setTimeout(async () => {
    try {
      const hits: any = await Search(q)
      searchResults.value = (hits ?? []).filter((n: any) => n.type === 'request').slice(0, 50)
    } catch { searchResults.value = [] }
  }, 120)
})

const items = computed(() => state.mode === 'commands' ? filtered.value : searchResults.value)

watch(items, () => { cursor.value = 0 })

function runItem(it: any) {
  if (state.mode === 'commands') {
    (it as Command).run()
  } else {
    openRequest({ ...it, depth: 0, isExpanded: false })
  }
  close()
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') { close(); return }
  if (e.key === 'ArrowDown') { e.preventDefault(); cursor.value = Math.min(cursor.value + 1, items.value.length - 1); return }
  if (e.key === 'ArrowUp')   { e.preventDefault(); cursor.value = Math.max(cursor.value - 1, 0); return }
  if (e.key === 'Enter')     {
    e.preventDefault()
    const sel = items.value[cursor.value]
    if (sel) runItem(sel)
    return
  }
  // Swap modes from inside the palette without closing it.
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'p') { e.preventDefault(); state.mode = 'search'; return }
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); state.mode = 'commands'; return }
}

const placeholder = computed(() =>
  state.mode === 'commands'
    ? 'Type a command…  (Cmd+P to switch to request search)'
    : 'Search requests…  (Cmd+K to switch to commands)',
)
</script>

<template>
  <div v-if="state.open" class="overlay" @click.self="close">
    <div class="palette" @keydown="onKey">
      <input
        ref="input"
        v-model="query"
        class="input"
        :placeholder="placeholder"
        spellcheck="false"
      />
      <div class="list">
        <div v-if="!items.length" class="empty">
          {{ state.mode === 'commands' ? 'No matching commands' : 'No matching requests' }}
        </div>
        <button
          v-for="(it, i) in items"
          :key="state.mode === 'commands' ? (it as Command).id : (it as any).id"
          class="row"
          :class="{ active: cursor === i }"
          @mousemove="cursor = i"
          @click="runItem(it)"
        >
          <template v-if="state.mode === 'commands'">
            <span class="label">{{ (it as Command).label }}</span>
            <span class="group">{{ (it as Command).group ?? '' }}</span>
          </template>
          <template v-else>
            <span class="method" :style="{ color: methodColor((it as any).method) }">{{ (it as any).method }}</span>
            <span class="label">{{ (it as any).name }}</span>
          </template>
        </button>
      </div>
      <div class="footer">
        <span>↑↓ navigate</span><span>↵ run</span><span>esc close</span>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
function methodColor(m: string): string {
  return ({
    GET: '#61affe', POST: '#49cc90', PUT: '#fca130', PATCH: '#50e3c2', DELETE: '#f93e3e',
  } as Record<string, string>)[m] ?? 'var(--text-dim)'
}
</script>

<style scoped>
.overlay {
  position: fixed; inset: 0; background: rgba(0,0,0,0.45);
  display: flex; justify-content: center; padding-top: 80px; z-index: 320;
}
.palette {
  width: 560px; max-width: 92vw; max-height: 70vh;
  display: flex; flex-direction: column;
  background: var(--bg-elevated); border: 1px solid var(--border-strong);
  border-radius: 10px; box-shadow: 0 20px 60px rgba(0,0,0,0.45); overflow: hidden;
}
.input {
  background: transparent; border: 0; padding: 14px 18px;
  color: var(--text); font: 14px monospace; border-bottom: 1px solid var(--border);
}
.input:focus { outline: none; }
.list { flex: 1; overflow: auto; padding: 4px; }
.empty { padding: 20px; color: var(--text-faint); text-align: center; font-size: 12px; }
.row {
  width: 100%; display: flex; align-items: center; gap: 10px;
  padding: 8px 12px; text-align: left; color: var(--text-dim);
  font-size: 13px; border-radius: 6px;
}
.row.active { background: var(--bg-hover); color: var(--text); }
.row .label { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.row .group { color: var(--text-faint); font-size: 11px; }
.row .method { font: 700 10px monospace; min-width: 50px; }
.footer { display: flex; gap: 14px; padding: 6px 14px; border-top: 1px solid var(--border); font-size: 10px; color: var(--text-faint); background: var(--bg-panel); }
</style>
