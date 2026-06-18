<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useUpdate } from '../composables/useUpdate'
import {
  ListWorkspaces, ActiveWorkspaceID, SwitchWorkspace, CreateWorkspace, RenameWorkspace, DeleteWorkspace,
} from '../../bindings/reqost/collectionservice'
import { useDialog } from '../composables/useDialog'

const { version, updateInfo, applying, applied, checkError, autoCheck, install } = useUpdate()
const dialog = useDialog()

const showPopover = ref(false)
const showWs = ref(false)
const workspaces = ref<any[]>([])
const activeWs = ref<string>('')

async function loadWorkspaces() {
  try {
    workspaces.value = await ListWorkspaces() ?? []
    activeWs.value = await ActiveWorkspaceID() ?? ''
  } catch { /* keep last */ }
}
async function pickWs(id: string) {
  if (id === activeWs.value) { showWs.value = false; return }
  try { await SwitchWorkspace(id) ; activeWs.value = id } catch { /* ignore */ }
  showWs.value = false
}
async function newWs() {
  const name = await dialog.prompt('New workspace name', 'New workspace')
  if (!name?.trim()) return
  try {
    const w: any = await CreateWorkspace(name.trim())
    if (w?.id) { await loadWorkspaces(); await pickWs(w.id) }
  } catch { /* ignore */ }
}
async function renameWs(id: string, oldName: string) {
  const name = await dialog.prompt('Rename workspace', oldName)
  if (!name?.trim()) return
  try { await RenameWorkspace(id, name.trim()); await loadWorkspaces() } catch { /* ignore */ }
}
async function delWs(id: string) {
  const ok = await dialog.confirm('Delete this workspace AND its index file?')
  if (!ok) return
  try { await DeleteWorkspace(id); await loadWorkspaces() } catch { /* ignore */ }
}
function activeWsName(): string {
  return workspaces.value.find(w => w.id === activeWs.value)?.name ?? 'Workspace'
}

onMounted(() => { autoCheck(); loadWorkspaces() })

async function onInstall() {
  await install()
  if (applied.value) showPopover.value = false
}
</script>

<template>
  <div class="titlebar">
    <span class="app-name">ReQost</span>

    <div class="ws-wrap">
      <button class="ws-pill" @click.stop="showWs = !showWs">
        ⌘ {{ activeWsName() }} ▾
      </button>
      <div v-if="showWs" class="ws-menu" @click.stop>
        <div class="ws-head">Workspaces</div>
        <button
          v-for="w in workspaces" :key="w.id"
          class="ws-item" :class="{ active: w.id === activeWs }"
          @click="pickWs(w.id)"
        >
          <span class="ws-name">{{ w.name }}</span>
          <button class="ws-act" title="Rename" @click.stop="renameWs(w.id, w.name)">✎</button>
          <button v-if="workspaces.length > 1" class="ws-act danger" title="Delete" @click.stop="delWs(w.id)">✕</button>
        </button>
        <button class="ws-new" @click="newWs">+ New workspace</button>
      </div>
    </div>

    <div class="right">
      <span class="ver-badge">{{ version }}</span>

      <!-- update available pill -->
      <div v-if="updateInfo" class="upd-wrap">
        <button class="upd-pill" @click.stop="showPopover = !showPopover">
          ↑ {{ updateInfo.latest }}
        </button>

        <div v-if="showPopover" class="upd-pop" @click.stop>
          <div class="pop-title">Update available</div>
          <div class="pop-meta">{{ version }} → {{ updateInfo.latest }}</div>
          <p v-if="checkError" class="pop-err">{{ checkError }}</p>
          <div class="pop-actions">
            <button class="pop-btn primary" :disabled="applying" @click="onInstall">
              {{ applying ? 'Installing…' : 'Install & relaunch' }}
            </button>
            <button class="pop-btn" @click="showPopover = false">Later</button>
          </div>
          <p class="pop-hint">Quit and reopen to apply after install.</p>
        </div>
      </div>
    </div>

    <!-- click-outside dismiss -->
    <div v-if="showPopover || showWs" class="backdrop" @click="showPopover = false; showWs = false" />
  </div>
</template>

<style scoped>
.titlebar {
  height: 50px;
  flex-shrink: 0;
  background: var(--rail-bg);
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  padding-left: 88px;
  padding-right: 12px;
  -webkit-app-region: drag;
  user-select: none;
  -webkit-user-select: none;
}

.app-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-faint);
  letter-spacing: 0.08em;
  text-transform: uppercase;
  -webkit-app-region: no-drag;
}

.right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 8px;
  -webkit-app-region: no-drag;
}

.ver-badge {
  font: 10px monospace;
  color: var(--text-faint);
  opacity: 0.6;
}

/* ── update pill ── */
.upd-wrap {
  position: relative;
}

.upd-pill {
  background: color-mix(in srgb, var(--ok) 18%, transparent);
  border: 1px solid color-mix(in srgb, var(--ok) 50%, transparent);
  border-radius: 12px;
  color: var(--ok);
  font-size: 11px;
  font-weight: 600;
  padding: 3px 10px;
  cursor: pointer;
  animation: pulse 2.4s ease-in-out infinite;
}
.upd-pill:hover { filter: brightness(1.15); }

@keyframes pulse {
  0%, 100% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--ok) 35%, transparent); }
  50%       { box-shadow: 0 0 0 4px color-mix(in srgb, var(--ok) 0%, transparent); }
}

/* ── popover ── */
.upd-pop {
  position: absolute;
  top: calc(100% + 10px);
  right: 0;
  width: 260px;
  background: var(--bg-panel);
  border: 1px solid var(--border-strong);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0,0,0,.35);
  padding: 14px 16px;
  z-index: 200;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.pop-title { font-size: 13px; font-weight: 700; color: var(--text); }
.pop-meta  { font: 11px monospace; color: var(--text-dim); }
.pop-hint  { font-size: 10px; color: var(--text-faint); margin-top: 2px; }
.pop-err   { font-size: 11px; color: var(--danger); }

.pop-actions { display: flex; gap: 6px; margin-top: 4px; }

.pop-btn {
  flex: 1;
  background: var(--bg-input);
  border: 1px solid var(--border-strong);
  border-radius: 6px;
  color: var(--text-dim);
  font-size: 12px;
  padding: 6px 0;
  cursor: pointer;
}
.pop-btn:hover:not(:disabled) { color: var(--text); background: var(--bg-hover); }
.pop-btn.primary {
  background: var(--ok);
  color: #06140d;
  border-color: transparent;
  font-weight: 600;
}
.pop-btn.primary:hover:not(:disabled) { filter: brightness(1.1); }
.pop-btn:disabled { opacity: 0.5; cursor: default; }

.backdrop {
  position: fixed;
  inset: 0;
  z-index: 199;
}

.ws-wrap { position: relative; margin-left: 14px; -webkit-app-region: no-drag; }
.ws-pill {
  background: var(--bg-input); border: 1px solid var(--border-strong);
  border-radius: 12px; color: var(--text-dim); font-size: 11px; padding: 3px 10px;
}
.ws-pill:hover { color: var(--text); }
.ws-menu {
  position: absolute; top: calc(100% + 6px); left: 0; min-width: 220px;
  background: var(--bg-panel); border: 1px solid var(--border-strong);
  border-radius: 8px; box-shadow: 0 8px 24px rgba(0,0,0,.35);
  padding: 4px; z-index: 220; display: flex; flex-direction: column;
}
.ws-head { font-size: 10px; text-transform: uppercase; letter-spacing: 0.6px; color: var(--text-faint); padding: 6px 10px; }
.ws-item {
  display: flex; align-items: center; gap: 8px; padding: 6px 10px;
  border-radius: 5px; color: var(--text-dim); font-size: 12px;
}
.ws-item:hover { background: var(--bg-hover); color: var(--text); }
.ws-item.active { color: var(--accent); }
.ws-name { flex: 1; text-align: left; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ws-act { padding: 0 4px; font-size: 11px; color: var(--text-faint); }
.ws-act:hover { color: var(--text); }
.ws-act.danger:hover { color: var(--danger); }
.ws-new { padding: 7px 10px; color: var(--accent); font-size: 12px; border-radius: 5px; text-align: left; }
.ws-new:hover { background: var(--bg-hover); }
</style>
