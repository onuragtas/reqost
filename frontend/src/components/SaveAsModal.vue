<script setup lang="ts">
import { ref, watch } from 'vue'
import { useSaveAsDialog } from '../composables/useSaveAsDialog'
import { GetRootItems, GetChildren } from '../../bindings/reqost/collectionservice'

interface FolderRow { id: string; name: string; depth: number; hasChildren: boolean; expanded: boolean }

const { state, choose, cancel } = useSaveAsDialog()
const folders = ref<FolderRow[]>([])
const selectedId = ref('') // '' = collection root

function toRow(n: any, depth: number): FolderRow {
  return { id: n.id, name: n.name, depth, hasChildren: !!n.hasChildren, expanded: false }
}

async function loadRoot() {
  const nodes = await GetRootItems()
  folders.value = nodes.filter((n: any) => n.type === 'folder').map((n: any) => toRow(n, 0))
}

watch(() => state.open, (open) => {
  if (!open) return
  selectedId.value = ''
  void loadRoot()
})

async function toggle(node: FolderRow) {
  const idx = folders.value.findIndex(f => f.id === node.id)
  if (idx === -1) return
  if (node.expanded) {
    let end = idx + 1
    while (end < folders.value.length && folders.value[end].depth > node.depth) end++
    const list = folders.value.slice()
    list[idx] = { ...node, expanded: false }
    list.splice(idx + 1, end - idx - 1)
    folders.value = list
  } else {
    const children = await GetChildren(node.id)
    const childRows = children.filter((n: any) => n.type === 'folder').map((n: any) => toRow(n, node.depth + 1))
    const list = folders.value.slice()
    list[idx] = { ...node, expanded: true }
    list.splice(idx + 1, 0, ...childRows)
    folders.value = list
  }
}

function save() { choose(selectedId.value) }
</script>

<template>
  <div v-if="state.open" class="overlay" @click.self="cancel" @keydown.esc="cancel">
    <div class="dialog">
      <div class="title">{{ state.title }}</div>
      <div class="hint">Choose where to save this request in your collection.</div>
      <div class="tree">
        <div class="row" :class="{ selected: selectedId === '' }" @click="selectedId = ''">
          <span class="chevron-spacer" />
          <span class="name">/ (Collection root)</span>
        </div>
        <div
          v-for="f in folders" :key="f.id" class="row"
          :class="{ selected: selectedId === f.id }"
          :style="{ paddingLeft: (f.depth * 16) + 'px' }"
          @click="selectedId = f.id"
        >
          <span v-if="f.hasChildren" class="chevron" @click.stop="toggle(f)">{{ f.expanded ? '▾' : '▸' }}</span>
          <span v-else class="chevron-spacer" />
          <span class="name">📁 {{ f.name }}</span>
        </div>
      </div>
      <div class="actions">
        <button class="cancel" @click="cancel">Cancel</button>
        <button class="ok" @click="save">Save</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 300; }
.dialog { width: 420px; max-width: 90vw; background: var(--bg-elevated); border: 1px solid var(--border-strong); border-radius: 10px; padding: 18px; box-shadow: 0 20px 60px rgba(0,0,0,0.5); }
.title { font-size: 14px; color: var(--text); margin-bottom: 4px; }
.hint { font-size: 11px; color: var(--text-dim); margin-bottom: 12px; }
.tree {
  max-height: 260px; overflow-y: auto;
  background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 6px;
  padding: 4px; margin-bottom: 14px;
}
.row {
  display: flex; align-items: center; gap: 4px;
  padding: 5px 8px; border-radius: 5px; cursor: pointer;
  font-size: 12.5px; color: var(--text);
  white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
}
.row:hover { background: var(--bg-hover); }
.row.selected { background: var(--accent); color: var(--accent-text); }
.chevron { width: 14px; flex: none; text-align: center; color: var(--text-dim); font-size: 10px; }
.row.selected .chevron { color: var(--accent-text); }
.chevron-spacer { width: 14px; flex: none; }
.name { overflow: hidden; text-overflow: ellipsis; }
.actions { display: flex; justify-content: flex-end; gap: 8px; }
.cancel { color: var(--text-dim); font-size: 13px; padding: 7px 14px; border-radius: 6px; }
.cancel:hover { background: var(--bg-hover); }
.ok { background: var(--accent); color: var(--accent-text); font-weight: 600; font-size: 13px; padding: 7px 16px; border-radius: 6px; }
</style>
