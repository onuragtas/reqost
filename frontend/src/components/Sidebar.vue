<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { RecycleScroller } from 'vue-virtual-scroller'
import { Events } from '@wailsio/runtime'
import {
  PickImport, PickImportOpenAPI, PickExport, CreateRequest, CreateFolder, RenameNode, DeleteNode,
  GetRequestDetail, MoveNode, DuplicateNode, ImportFromURL, ImportAllFromPostman, ClearAll,
  ImportHARBytes, GetFolderContext, SetFolderContext,
  PickExportWorkspaceZip, PickImportWorkspaceZip,
} from '../../bindings/reqost/collectionservice'
import { PickImportEnv } from '../../bindings/reqost/envservice'
import { useTree, type FlatNode } from '../composables/useTree'
import { useTabs } from '../composables/useTabs'
import { useRunner } from '../composables/useRunner'
import { useDialog } from '../composables/useDialog'
import { useEnv } from '../composables/useEnv'
import { useFavorites } from '../composables/useFavorites'
import { toCurl, parseCurl } from '../composables/curl'

const { flatList, loadRoot, toggleNode, searchNodes, refreshNode, removeNode, reloadChildren } = useTree()
const { isFav, toggle: toggleFav } = useFavorites()

// ── Filter state ────────────────────────────────────────────────────────
// methodFilter '' = no filter; otherwise method literal (GET, POST, …).
// showOnlyFavorites overlays a star filter (request-only).
const METHOD_FILTERS = ['', 'GET', 'POST', 'PUT', 'PATCH', 'DELETE']
const methodFilter = ref<string>('')
const showOnlyFavorites = ref(false)

const visibleList = computed(() => {
  if (!methodFilter.value && !showOnlyFavorites.value) return flatList.value
  return flatList.value.filter(n => {
    if (n.type === 'folder') return true   // folders always shown so structure stays
    if (methodFilter.value && n.method !== methodFilter.value) return false
    if (showOnlyFavorites.value && !isFav(n.id)) return false
    return true
  })
})

// ── Drag-and-drop reorder/move ─────────────────────────────────────────────
type DropZone = 'before' | 'after' | 'into'
const draggedId = ref<string>('')
const dropTarget = ref<{ id: string; zone: DropZone } | null>(null)

function onDragStart(e: DragEvent, node: FlatNode) {
  if (searchQuery.value) { e.preventDefault(); return } // no DnD while searching
  draggedId.value = node.id
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', node.id)
  }
}
function onDragEnd() {
  draggedId.value = ''
  dropTarget.value = null
}
function onDragOver(e: DragEvent, node: FlatNode) {
  if (!draggedId.value || draggedId.value === node.id) return
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'

  const t = e.currentTarget as HTMLElement
  const r = t.getBoundingClientRect()
  const y = e.clientY - r.top
  const h = r.height
  // Folder rows have a middle "into" zone (33–66%); requests are just before/after.
  let zone: DropZone
  if (node.type === 'folder' && y > h * 0.33 && y < h * 0.67) zone = 'into'
  else if (y < h / 2) zone = 'before'
  else zone = 'after'
  if (dropTarget.value?.id !== node.id || dropTarget.value?.zone !== zone) {
    dropTarget.value = { id: node.id, zone }
  }
}
function onDragLeaveRow(node: FlatNode) {
  if (dropTarget.value?.id === node.id) dropTarget.value = null
}

// computeNewIndex finds where draggedId should land among target's siblings
// (excluding draggedId itself), based on the chosen drop zone.
function computeNewIndex(target: FlatNode, zone: DropZone, draggedNodeId: string): number {
  if (zone === 'into') return 1_000_000 // backend clamps to len(siblings)
  let idx = 0
  for (const n of flatList.value) {
    if (n.parentId !== target.parentId) continue
    if (n.id === draggedNodeId) continue
    if (n.id === target.id) return zone === 'before' ? idx : idx + 1
    idx++
  }
  return idx
}

async function onDrop(target: FlatNode) {
  const did = draggedId.value
  const zone = dropTarget.value?.zone
  draggedId.value = ''
  dropTarget.value = null
  if (!did || !zone || did === target.id) return

  // Resolve destination parent + index.
  const newParentId = zone === 'into' ? target.id : target.parentId
  const dragged = flatList.value.find(n => n.id === did)
  const oldParentId = dragged?.parentId ?? ''
  const newIndex = computeNewIndex(target, zone, did)

  try {
    await MoveNode(did, newParentId, newIndex)
  } catch (e) {
    flashError('Move failed', e)
    return
  }

  // Refresh affected parents. "" maps to loadRoot which collapses everything —
  // acceptable cost; reorders inside a folder keep their expansion.
  if (oldParentId === '' || newParentId === '') {
    await loadRoot()
  } else {
    await reloadChildren(oldParentId)
    if (newParentId !== oldParentId) await reloadChildren(newParentId)
  }
}

const { openRequest, openAdhoc, closeTab } = useTabs()
const { run: runColl } = useRunner()
const dialog = useDialog()
const { syncEnvironments } = useEnv()

// ── Context / header menu ──────────────────────────────────────────────────
interface MenuItem { label: string; danger?: boolean; run: () => void }
const menu = ref<{ x: number; y: number; items: MenuItem[] } | null>(null)
function closeMenu() { menu.value = null }

function openNodeMenu(e: MouseEvent, node: FlatNode) {
  const items: MenuItem[] = []
  if (node.type === 'folder') {
    items.push(
      { label: 'Run Folder', run: () => runColl(node.id) },
      { label: 'New Request', run: () => createUnder(node.id, 'request') },
      { label: 'New Folder', run: () => createUnder(node.id, 'folder') },
      { label: 'Folder context (shared headers / auth)…', run: () => editFolderContext(node) },
    )
  } else {
    items.push({ label: 'Copy as cURL', run: () => copyCurl(node) })
    items.push({
      label: isFav(node.id) ? '★ Remove from favorites' : '☆ Add to favorites',
      run: () => toggleFav(node.id),
    })
    items.push({ label: `Copy reference: {{${node.name}.response.body.…}}`, run: () => copyText(`{{${node.name}.response.body.}}`) })
  }
  items.push(
    { label: 'Copy ID', run: () => copyText(node.id) },
    { label: 'Duplicate', run: () => duplicate(node) },
    { label: 'Rename', run: () => rename(node) },
    { label: 'Delete', danger: true, run: () => remove(node) },
  )
  menu.value = { x: e.clientX, y: e.clientY, items }
}
function openHeaderMenu(e: MouseEvent) {
  menu.value = {
    x: e.clientX, y: e.clientY,
    items: [
      { label: 'New Request', run: () => createUnder('', 'request') },
      { label: 'New Folder', run: () => createUnder('', 'folder') },
      { label: 'New WebSocket', run: () => openAdhoc({ name: 'WebSocket', method: 'GET', url: 'wss://' }) },
      { label: 'New gRPC Request', run: () => openAdhoc({ name: 'gRPC', method: 'POST', url: 'grpc://localhost:50051', body: '{}' }) },
      { label: 'New SSE Stream', run: () => openAdhoc({ name: 'SSE', method: 'GET', url: 'sses://api.example.com/stream' }) },
      { label: 'Paste cURL…', run: onPasteCurl },
      { label: 'Run Collection', run: () => runColl('') },
      { label: 'Import all from Postman…', run: onImportAllFromPostman },
      { label: 'Import Collection…', run: onImport },
      { label: 'Import Environment…', run: onImportEnv },
      { label: 'Import OpenAPI…', run: onImportOpenAPI },
      { label: 'Import HAR (paste)…', run: onImportHAR },
      { label: 'Import from URL…', run: onImportFromURL },
      { label: 'Export Collection…', run: onExport },
      { label: 'Export Workspace (.zip)…', run: onExportWorkspaceZip },
      { label: 'Import Workspace (.zip)…', run: onImportWorkspaceZip },
      { label: 'Delete All', run: onClearAll, danger: true },
    ],
  }
}

// flashError surfaces backend failures instead of letting them vanish — a
// swallowed SQLITE_BUSY here is exactly what made deletes look like no-ops.
function flashError(prefix: string, e: any) {
  statusMsg.value = `${prefix}: ${e?.message ?? e}`
  setTimeout(() => { if (statusMsg.value.startsWith(prefix)) statusMsg.value = '' }, 4000)
}

async function createUnder(parentId: string, type: 'request' | 'folder') {
  const name = await dialog.prompt(`New ${type} name`, type === 'request' ? 'New Request' : 'New Folder')
  if (!name?.trim()) return
  try {
    const node: any = type === 'request'
      ? await CreateRequest(parentId, name.trim(), 'GET')
      : await CreateFolder(parentId, name.trim())
    await reloadChildren(parentId)
    if (type === 'request' && node) openRequest({ ...node, depth: 0, isExpanded: false })
  } catch (e) {
    flashError('Create failed', e)
  }
}

async function rename(node: FlatNode) {
  const name = await dialog.prompt('Rename to', node.name)
  if (!name?.trim() || name === node.name) return
  try {
    await RenameNode(node.id, name.trim())
    refreshNode(node.id, { name: name.trim() })
  } catch (e) {
    flashError('Rename failed', e)
  }
}

async function duplicate(node: FlatNode) {
  try {
    await DuplicateNode(node.id)
    await reloadChildren(node.parentId)
  } catch (e) {
    flashError('Duplicate failed', e)
  }
}

async function remove(node: FlatNode) {
  const ok = await dialog.confirm(`Delete "${node.name}"${node.type === 'folder' ? ' and all its contents' : ''}?`)
  if (!ok) return
  try {
    await DeleteNode(node.id)
    removeNode(node.id)
    closeTab(node.id)
  } catch (e) {
    flashError('Delete failed', e)
  }
}

async function copyText(s: string) {
  try {
    await navigator.clipboard.writeText(s)
    statusMsg.value = 'Copied'
    setTimeout(() => { if (statusMsg.value === 'Copied') statusMsg.value = '' }, 1200)
  } catch { /* WKWebView occasionally blocks — silently ignore */ }
}

async function copyCurl(node: FlatNode) {
  const d: any = await GetRequestDetail(node.id)
  if (!d) return
  let headers: any[] = []
  try { headers = JSON.parse(d.headers || '[]') } catch { /* ignore */ }
  const curl = toCurl(d.method, d.url, headers, d.body)
  try {
    await navigator.clipboard.writeText(curl)
    statusMsg.value = 'cURL copied'
    setTimeout(() => { if (statusMsg.value === 'cURL copied') statusMsg.value = '' }, 1500)
  } catch {
    // Clipboard can be blocked in the webview — show it so the user can copy.
    await dialog.prompt('Copy this cURL command', curl)
  }
}

async function onExportWorkspaceZip() {
  try {
    const path = await PickExportWorkspaceZip()
    if (path) {
      statusMsg.value = `Workspace exported → ${path}`
      setTimeout(() => { if (statusMsg.value.startsWith('Workspace exported')) statusMsg.value = '' }, 2500)
    }
  } catch (e) { flashError('Workspace export', e) }
}
async function onImportWorkspaceZip() {
  try {
    const path = await PickImportWorkspaceZip()
    if (path) {
      statusMsg.value = 'Workspace imported'
      setTimeout(() => { if (statusMsg.value === 'Workspace imported') statusMsg.value = '' }, 2000)
    }
  } catch (e) { flashError('Workspace import', e) }
}

async function onExport() {
  const path = await PickExport('reqost export') // native save-file dialog
  if (!path) return
  statusMsg.value = 'Exported ✓'
  setTimeout(() => { if (statusMsg.value === 'Exported ✓') statusMsg.value = '' }, 1500)
}
const searchQuery = ref('')
const statusMsg = ref('')

onMounted(() => {
  loadRoot()

  Events.On('collection:importing', (ev: any) => {
    const msg = ev?.data ?? ev
    statusMsg.value = (typeof msg === 'string' && msg !== 'postman') ? msg : 'Indexing...'
  })
  Events.On('collection:ready', (ev: any) => {
    const msg = ev?.data ?? ev
    if (typeof msg === 'string' && msg.startsWith('Imported ')) {
      statusMsg.value = msg
      setTimeout(() => { if (statusMsg.value === msg) statusMsg.value = '' }, 3000)
    } else {
      statusMsg.value = ''
    }
    loadRoot()
  })
  Events.On('collection:error', (ev: any) => {
    statusMsg.value = `Error: ${ev?.data ?? ev}`
  })
})

let searchTimer: ReturnType<typeof setTimeout>
function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => searchNodes(searchQuery.value), 200)
}

async function onImport() {
  await PickImport() // native open-file dialog; import events update the tree
}

async function editFolderContext(node: FlatNode) {
  let current = '{}'
  try { current = await GetFolderContext(node.id) } catch { current = '{}' }
  // Pretty-print so the textarea is editable.
  try { current = JSON.stringify(JSON.parse(current), null, 2) } catch { /* keep raw */ }
  const next = await dialog.promptMultiline(
    `Folder "${node.name}" — shared context`,
    current,
    `{
  "sharedHeaders": [
    { "key": "Authorization", "value": "Bearer {{token}}", "enabled": true }
  ],
  "auth": { "type": "bearer", "token": "{{token}}" }
}`,
  )
  if (next === null) return
  let toSave = next.trim() || '{}'
  try { JSON.parse(toSave) } catch (e: any) {
    flashError('Folder context (invalid JSON)', e)
    return
  }
  try {
    await SetFolderContext(node.id, toSave)
    statusMsg.value = `Folder context saved`
    setTimeout(() => { if (statusMsg.value === 'Folder context saved') statusMsg.value = '' }, 1500)
  } catch (e) {
    flashError('Save folder context', e)
  }
}

async function onPasteCurl() {
  const text = await dialog.promptMultiline(
    'Paste cURL command',
    '',
    "curl 'https://api.example.com/v1/users' -H 'Authorization: Bearer …' --data '{}'",
  )
  if (!text?.trim()) return
  const parsed = parseCurl(text)
  if (!parsed) {
    flashError('Paste cURL', new Error("doesn't look like a curl command"))
    return
  }
  // Heuristic name: host + last path segment.
  let name = parsed.url
  try {
    const u = new URL(parsed.url.replace(/{{[^}]+}}/g, 'x'))
    const seg = u.pathname.split('/').filter(Boolean).pop() || u.host
    name = `${parsed.method} ${seg}`
  } catch { /* leave name as full URL */ }
  openAdhoc({
    name,
    method: parsed.method,
    url: parsed.url,
    headers: parsed.headers,
    body: parsed.body,
    bodyType: parsed.bodyType,
    formFields: parsed.formFields,
  })
}

async function onImportHAR() {
  const text = await dialog.promptMultiline(
    'Paste HAR JSON',
    '',
    'Browser DevTools → Network → "Save all as HAR with content" → paste here',
  )
  if (!text?.trim()) return
  try {
    const n = await ImportHARBytes(text)
    statusMsg.value = `Imported ${n} request(s) from HAR`
    setTimeout(() => { if (statusMsg.value.startsWith('Imported')) statusMsg.value = '' }, 2500)
  } catch (e) {
    flashError('HAR import failed', e)
  }
}

async function onImportOpenAPI() {
  try {
    await PickImportOpenAPI() // merges the spec under a new folder; emits collection:ready
  } catch (e) {
    flashError('Import failed', e)
  }
}

async function onImportFromURL() {
  const url = await dialog.prompt(
    'Import from URL',
    '',
    'https://  (Postman share link, OpenAPI spec, raw JSON…)'
  )
  if (!url?.trim()) return
  try {
    await ImportFromURL(url.trim())
  } catch (e) {
    flashError('Import failed', e)
  }
}

async function onImportEnv() {
  try {
    const name = await PickImportEnv()
    if (name) {
      await syncEnvironments()
      statusMsg.value = `Environment "${name}" imported`
      setTimeout(() => { if (statusMsg.value.startsWith('Environment')) statusMsg.value = '' }, 2500)
    }
  } catch (e) {
    flashError('Env import failed', e)
  }
}

async function onImportAllFromPostman() {
  const apiKey = await dialog.prompt(
    'Import all from Postman',
    '',
    'Postman API key',
    { url: 'https://go.postman.co/settings/me/api-keys', label: 'Get API key at postman.co' }
  )
  if (!apiKey?.trim()) return
  try {
    await ImportAllFromPostman(apiKey.trim())
    await syncEnvironments()
  } catch (e) {
    flashError('Postman import failed', e)
  }
}

async function onClearAll() {
  const ok = await dialog.confirm('Delete ALL items from the collection? This cannot be undone.', 'Delete All')
  if (!ok) return
  try {
    await ClearAll()
    await loadRoot()
  } catch (e) {
    flashError('Delete failed', e)
  }
}

function onNodeClick(node: FlatNode) {
  if (node.type === 'folder') {
    toggleNode(node)
  } else {
    openRequest(node)
  }
}

const METHOD_COLORS: Record<string, string> = {
  GET:    '#61affe',
  POST:   '#49cc90',
  PUT:    '#fca130',
  PATCH:  '#50e3c2',
  DELETE: '#f93e3e',
}
</script>

<template>
  <div class="sidebar">
    <div class="header">
      <input
        v-model="searchQuery"
        class="search"
        placeholder="Search requests..."
        @input="onSearchInput"
      />
      <button class="import-btn" title="New / Import / Export" @click="openHeaderMenu">+</button>
    </div>

    <div class="filter-bar">
      <button
        v-for="m in METHOD_FILTERS" :key="m"
        class="m-chip" :class="{ active: methodFilter === m, [m.toLowerCase()]: true }"
        :title="m === '' ? 'All methods' : `Only ${m}`"
        @click="methodFilter = (methodFilter === m ? '' : m)"
      >{{ m || 'All' }}</button>
      <button
        class="m-chip fav" :class="{ active: showOnlyFavorites }"
        title="Only favorites"
        @click="showOnlyFavorites = !showOnlyFavorites"
      >★</button>
    </div>

    <div v-if="statusMsg" class="status">{{ statusMsg }}</div>

    <div v-if="!flatList.length && !searchQuery" class="hint">
      <p>No requests yet</p>
      <button @click="onImport">Import a collection</button>
      <button class="ghost" @click="createUnder('', 'request')">New request</button>
    </div>

    <RecycleScroller
      v-show="visibleList.length"
      class="scroller"
      :items="visibleList"
      :item-size="28"
      key-field="id"
      v-slot="{ item }"
    >
      <div
        class="row"
        :class="{
          'drag-src': draggedId === item.id,
          'drop-before': dropTarget?.id === item.id && dropTarget?.zone === 'before',
          'drop-after':  dropTarget?.id === item.id && dropTarget?.zone === 'after',
          'drop-into':   dropTarget?.id === item.id && dropTarget?.zone === 'into',
        }"
        :style="{ paddingLeft: `${8 + item.depth * 14}px` }"
        draggable="true"
        @dragstart="onDragStart($event, item)"
        @dragend="onDragEnd"
        @dragover="onDragOver($event, item)"
        @dragleave="onDragLeaveRow(item)"
        @drop.prevent="onDrop(item)"
        @click="onNodeClick(item)"
        @contextmenu.prevent="openNodeMenu($event, item)"
      >
        <span class="icon">
          <template v-if="item.type === 'folder'">
            <svg
              class="chev"
              :class="{ open: item.isExpanded }"
              viewBox="0 0 10 10"
              aria-hidden="true"
            >
              <path d="M3 2.5 L7 5 L3 7.5 Z" />
            </svg>
            <svg class="folder-ico" viewBox="0 0 16 14" aria-hidden="true">
              <path d="M1.5 3.5 L1.5 11.5 A1 1 0 0 0 2.5 12.5 L13.5 12.5 A1 1 0 0 0 14.5 11.5 L14.5 5.5 A1 1 0 0 0 13.5 4.5 L7.5 4.5 L6 3 A1 1 0 0 0 5.3 2.7 L2.5 2.7 A1 1 0 0 0 1.5 3.5 Z"/>
            </svg>
          </template>
          <span
            v-else
            class="badge"
            :style="{ color: METHOD_COLORS[item.method] ?? '#888' }"
          >{{ item.method }}</span>
        </span>
        <span class="name">{{ item.name }}</span>
        <span
          v-if="item.type === 'request' && isFav(item.id)"
          class="fav-mark"
          title="Favorite"
        >★</span>
      </div>
    </RecycleScroller>

    <!-- context / header menu -->
    <template v-if="menu">
      <div class="menu-overlay" @click="closeMenu" @contextmenu.prevent="closeMenu"></div>
      <div class="menu" :style="{ left: `${menu.x}px`, top: `${menu.y}px` }">
        <button
          v-for="(it, i) in menu.items" :key="i"
          class="menu-item" :class="{ danger: it.danger }"
          @click="it.run(); closeMenu()"
        >{{ it.label }}</button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.sidebar {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--bg-panel);
  border-right: 1px solid var(--border);
  overflow: hidden;
}

.header {
  display: flex;
  gap: 6px;
  padding: 8px;
  flex-shrink: 0;
}

.search {
  flex: 1;
  background: var(--bg-input);
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  font-size: 12px;
  padding: 5px 8px;
  outline: none;
}
.search:focus { border-color: var(--accent); }

.import-btn {
  background: var(--bg-input);
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  color: var(--text-dim);
  font-size: 18px;
  line-height: 1;
  padding: 2px 9px;
}
.import-btn:hover { background: var(--bg-hover); color: var(--text); }

.filter-bar {
  display: flex; gap: 4px; padding: 0 8px 6px; flex-wrap: wrap;
  flex-shrink: 0;
}
.m-chip {
  font: 700 9px monospace; padding: 2px 7px; border-radius: 10px;
  background: var(--bg-input); border: 1px solid var(--border);
  color: var(--text-faint);
}
.m-chip:hover { color: var(--text); border-color: var(--border-strong); }
.m-chip.active { color: var(--text); background: var(--bg-elevated); border-color: var(--accent); }
.m-chip.get    { --c: #61affe; }
.m-chip.post   { --c: #49cc90; }
.m-chip.put    { --c: #fca130; }
.m-chip.patch  { --c: #50e3c2; }
.m-chip.delete { --c: #f93e3e; }
.m-chip.active.get,
.m-chip.active.post,
.m-chip.active.put,
.m-chip.active.patch,
.m-chip.active.delete {
  color: var(--c, var(--accent)); border-color: var(--c, var(--accent));
}
.m-chip.fav { color: var(--text-faint); font-size: 11px; padding: 1px 8px; }
.m-chip.fav.active { color: var(--accent); border-color: var(--accent); }

.fav-mark { font-size: 10px; color: var(--accent); margin-left: 4px; flex-shrink: 0; }

.status {
  font-size: 11px;
  padding: 3px 10px;
  flex-shrink: 0;
  background: var(--warn-bg);
  color: var(--warn-text);
}

.scroller {
  flex: 1;
  min-height: 0;
}

.row {
  display: flex;
  align-items: center;
  height: 28px;
  cursor: pointer;
  gap: 4px;
  border-radius: 3px;
  margin: 0 3px;
  color: var(--text-dim);
}
.row:hover { background: var(--bg-hover); color: var(--text); }
.row.drag-src { opacity: 0.4; }
.row.drop-before {
  box-shadow: inset 0 2px 0 0 var(--accent);
}
.row.drop-after {
  box-shadow: inset 0 -2px 0 0 var(--accent);
}
.row.drop-into {
  background: color-mix(in srgb, var(--accent) 18%, transparent);
  outline: 1px dashed var(--accent);
  outline-offset: -2px;
}

.icon {
  width: 42px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
  color: var(--text-dim);
}

.chev {
  width: 9px; height: 9px;
  flex-shrink: 0;
  color: var(--text-faint);
  transition: transform 0.12s ease, color 0.12s ease;
}
.chev path { fill: currentColor; }
.chev.open { transform: rotate(90deg); color: var(--accent); }
.row:hover .chev { color: var(--text); }
.row:hover .chev.open { color: var(--accent); }

.folder-ico {
  width: 14px; height: 12px;
  flex-shrink: 0;
  color: var(--accent);
  opacity: 0.85;
}
.folder-ico path {
  fill: color-mix(in srgb, currentColor 18%, transparent);
  stroke: currentColor;
  stroke-width: 1;
  stroke-linejoin: round;
}
.row:hover .folder-ico { opacity: 1; }

.badge {
  font-family: monospace;
  font-size: 9px;
  font-weight: 700;
}

.name {
  flex: 1;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.hint { display: flex; flex-direction: column; gap: 8px; align-items: center; padding: 30px 16px; }
.hint p { color: var(--text-faint); font-size: 12px; }
.hint button { background: var(--accent); color: var(--accent-text); font-weight: 600; font-size: 12px; padding: 7px 14px; border-radius: 6px; width: 100%; }
.hint button.ghost { background: var(--bg-input); border: 1px solid var(--border-strong); color: var(--text-dim); }

.menu-overlay { position: fixed; inset: 0; z-index: 200; }
.menu {
  position: fixed; z-index: 201; min-width: 170px;
  background: var(--bg-elevated); border: 1px solid var(--border-strong);
  border-radius: 8px; padding: 4px; box-shadow: 0 10px 30px var(--shadow, rgba(0,0,0,0.4));
  display: flex; flex-direction: column;
}
.menu-item { text-align: left; font-size: 12px; color: var(--text); padding: 7px 10px; border-radius: 5px; }
.menu-item:hover { background: var(--bg-hover); }
.menu-item.danger { color: var(--danger); }
</style>
