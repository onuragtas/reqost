<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import {
  Init as GitInit, Status as GitStatus,
  Export as GitExport, Commit as GitCommit,
  Branches as GitBranches, Checkout as GitCheckout,
  Push as GitPush, Pull as GitPull, Fetch as GitFetch, SetRemote as GitSetRemote,
} from '../../bindings/reqost/gitservice'
import { useGitBind } from '../composables/useGitBind'
import { useDialog } from '../composables/useDialog'

const props = defineProps<{ open: boolean; workspaceId: string; workspaceName: string }>()
const emit = defineEmits<{ close: [committed: boolean] }>()

// Track whether the user committed inside this modal session so the close
// handler can clear the workspace's "unsynced to git" flag.
const didCommit = ref(false)

const { get: getBind, set: setBind } = useGitBind()
const dialog = useDialog()

const path     = ref<string>('')
const status   = ref<any>({ running: false, branch: '', status: '', hasRepo: false })
const branches = ref<string[]>([])
const busy     = ref(false)
const message  = ref<string>('')

const dirty = computed(() => !!status.value?.status && status.value.status.trim().length > 0)

async function refresh() {
  if (!path.value) {
    status.value = { hasRepo: false, branch: '', status: '' }
    branches.value = []
    return
  }
  busy.value = true
  try {
    status.value  = (await GitStatus(path.value)) ?? { hasRepo: false }
    branches.value = status.value.hasRepo ? ((await GitBranches(path.value)) ?? []) : []
  } catch (e: any) {
    message.value = e?.message ?? String(e)
  } finally { busy.value = false }
}

// Auto-export the current workspace into the bound directory whenever the
// modal opens. Without this, in-app edits to the SQLite index don't appear in
// `git status` until the user manually triggers Snapshot — leading to the
// "I changed an endpoint but git looks clean" foot-gun.
async function autoExport() {
  if (!path.value || !status.value?.hasRepo) return
  try {
    await GitExport(path.value, props.workspaceName)
  } catch { /* surface via refresh() error path */ }
}

// Re-read state every time the modal is reopened or workspace changes.
watch(
  () => [props.open, props.workspaceId] as const,
  async ([open, ws]) => {
    if (!open) return
    didCommit.value = false
    path.value = getBind(ws as string) || ''
    message.value = ''
    await refresh()
    // Surface in-app edits as on-disk diff every time the user opens the
    // modal — without this they look "clean" until Snapshot runs.
    if (path.value && status.value?.hasRepo) {
      await autoExport()
      await refresh()
      if ((status.value?.status ?? '').trim()) {
        message.value = '↻ Exported current workspace — pending changes below reflect what would be committed.'
      }
    }
  },
  { immediate: true },
)

async function onBindOrChange() {
  const next = await dialog.prompt('Path to Git directory (absolute)', path.value || `${import.meta.env.HOME ?? '~'}/git/reqost-${props.workspaceName}`)
  if (!next?.trim()) return
  path.value = next.trim()
  setBind(props.workspaceId, path.value)
  await refresh()
}

async function initRepo() {
  if (!path.value) return
  busy.value = true; message.value = ''
  try {
    await GitInit(path.value)
    message.value = `Initialised repo at ${path.value}`
    await refresh()
  } catch (e: any) { message.value = e?.message ?? String(e) }
  finally { busy.value = false }
}

async function snapshotToGit() {
  if (!path.value) return
  busy.value = true; message.value = ''
  try {
    await GitExport(path.value, props.workspaceName)
    const msg = (await dialog.prompt('Commit message', `snapshot — ${new Date().toISOString().slice(0, 19).replace('T', ' ')}`)) ?? ''
    if (!msg.trim()) { busy.value = false; return }
    await GitCommit(path.value, msg.trim())
    message.value = '✓ Snapshot committed'
    didCommit.value = true
    await refresh()
  } catch (e: any) {
    message.value = e?.message ?? String(e)
  } finally { busy.value = false }
}

async function commitOnly() {
  if (!path.value) return
  const msg = (await dialog.prompt('Commit message', 'reqost: manual commit')) ?? ''
  if (!msg.trim()) return
  busy.value = true; message.value = ''
  try {
    await GitCommit(path.value, msg.trim())
    message.value = '✓ Committed'
    didCommit.value = true
    await refresh()
  } catch (e: any) { message.value = e?.message ?? String(e) }
  finally { busy.value = false }
}

async function push() {
  if (!path.value) return
  busy.value = true; message.value = ''
  try {
    await GitPush(path.value)
    message.value = '✓ Pushed'
    didCommit.value = true
    await refresh()
  } catch (e: any) { message.value = `push: ${e?.message ?? e}` }
  finally { busy.value = false }
}
async function pull() {
  if (!path.value) return
  busy.value = true; message.value = ''
  try {
    await GitPull(path.value)
    message.value = '✓ Pulled (fast-forward)'
    await refresh()
  } catch (e: any) { message.value = `pull: ${e?.message ?? e}` }
  finally { busy.value = false }
}
async function fetchRemote() {
  if (!path.value) return
  busy.value = true; message.value = ''
  try {
    await GitFetch(path.value)
    message.value = '✓ Fetched'
    await refresh()
  } catch (e: any) { message.value = `fetch: ${e?.message ?? e}` }
  finally { busy.value = false }
}
async function setRemote() {
  if (!path.value) return
  const url = await dialog.prompt('Remote URL (origin)', status.value?.remote ? '' : 'git@github.com:user/repo.git')
  if (!url?.trim()) return
  busy.value = true; message.value = ''
  try {
    await GitSetRemote(path.value, 'origin', url.trim())
    message.value = `✓ Remote origin → ${url.trim()}`
    await refresh()
  } catch (e: any) { message.value = `remote: ${e?.message ?? e}` }
  finally { busy.value = false }
}

async function switchBranch(name: string) {
  if (!path.value || !name) return
  busy.value = true; message.value = ''
  try {
    await GitCheckout(path.value, name)
    message.value = `Switched to ${name}`
    await refresh()
  } catch (e: any) { message.value = e?.message ?? String(e) }
  finally { busy.value = false }
}

async function newBranch() {
  const name = await dialog.prompt('New branch name', 'feature/')
  if (!name?.trim()) return
  await switchBranch(name.trim())
}
</script>

<template>
  <div v-if="open" class="overlay" @click.self="emit('close', didCommit)">
    <div class="modal">
      <header class="head">
        <span class="title">Git — {{ workspaceName }}</span>
        <button class="close" @click="emit('close', didCommit)">✕</button>
      </header>

      <section class="row">
        <label>Bound path</label>
        <div class="path-line">
          <code class="path">{{ path || '— not bound —' }}</code>
          <button class="btn" @click="onBindOrChange">{{ path ? 'Change…' : 'Bind directory…' }}</button>
        </div>
      </section>

      <section v-if="path" class="row">
        <label>Repository</label>
        <div v-if="status?.hasRepo" class="status">
          <span class="badge ok">repo</span>
          <span class="badge branch">{{ status.branch || '(detached)' }}</span>
          <span class="badge" :class="dirty ? 'warn' : 'clean'">{{ dirty ? `${(status.status.split('\n').filter(Boolean).length)} change(s)` : 'clean' }}</span>
        </div>
        <div v-else class="status">
          <span class="badge warn">no repo</span>
          <button class="btn" :disabled="busy" @click="initRepo">git init</button>
        </div>
      </section>

      <section v-if="path && status?.hasRepo" class="row">
        <label>Branch</label>
        <div class="branch-line">
          <select :value="status.branch" @change="switchBranch(($event.target as HTMLSelectElement).value)">
            <option v-for="b in branches" :key="b" :value="b">{{ b }}</option>
          </select>
          <button class="btn" @click="newBranch">+ New branch</button>
        </div>
      </section>

      <section v-if="path && status?.hasRepo" class="row">
        <label>Remote</label>
        <div class="branch-line">
          <code class="path">{{ status.remote || '(no remote)' }}</code>
          <button class="btn" @click="setRemote">{{ status.remote ? 'Change…' : 'Add origin…' }}</button>
        </div>
        <div v-if="status.upstream" class="tracking">
          <span class="badge branch">↑ {{ status.upstream }}</span>
          <span v-if="status.ahead > 0"  class="badge warn">↑ {{ status.ahead }} to push</span>
          <span v-if="status.behind > 0" class="badge warn">↓ {{ status.behind }} to pull</span>
          <span v-if="!status.ahead && !status.behind" class="badge clean">up to date</span>
        </div>
        <p v-else-if="status.remote" class="hint">
          Current branch has no upstream — first <strong>Push</strong> sets it to <code>origin/{{ status.branch }}</code>.
        </p>
      </section>

      <section v-if="path && status?.hasRepo && dirty" class="row diff">
        <label>Pending changes</label>
        <pre class="porcelain selectable">{{ status.status.trim() }}</pre>
      </section>

      <section v-if="path && status?.hasRepo" class="row actions">
        <button class="btn primary" :disabled="busy" @click="snapshotToGit">
          📸 Snapshot &amp; commit
        </button>
        <button class="btn" :disabled="busy" @click="commitOnly">
          Commit current tree
        </button>
        <span class="action-sep"></span>
        <button
          class="btn" :disabled="busy || !status.remote || dirty"
          :title="dirty ? 'Working tree is dirty — commit first' : !status.remote ? 'No remote configured' : 'git pull --ff-only'"
          @click="pull"
        >⇣ Pull</button>
        <button
          class="btn" :disabled="busy || !status.remote"
          :title="!status.remote ? 'No remote configured' : 'git push'"
          @click="push"
        >⇡ Push{{ status.ahead ? ` (${status.ahead})` : '' }}</button>
        <button
          class="btn" :disabled="busy || !status.remote"
          :title="'git fetch — refreshes ahead/behind counters without merging'"
          @click="fetchRemote"
        >⟳ Fetch</button>
        <span class="action-sep"></span>
        <button class="btn" :disabled="busy" @click="refresh">↻ Refresh</button>
      </section>

      <p v-if="message" class="msg">{{ message }}</p>
    </div>
  </div>
</template>

<style scoped>
.overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 280; }
.modal { width: 560px; max-width: 92vw; max-height: 80vh; background: var(--bg-elevated); border: 1px solid var(--border-strong); border-radius: 10px; box-shadow: 0 20px 60px rgba(0,0,0,0.45); display: flex; flex-direction: column; overflow: hidden; }
.head { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; border-bottom: 1px solid var(--border); }
.title { font-size: 13px; font-weight: 700; color: var(--text); }
.close { color: var(--text-faint); font-size: 14px; padding: 2px 6px; }
.close:hover { color: var(--text); }
.row { display: flex; flex-direction: column; gap: 6px; padding: 10px 18px; border-bottom: 1px dashed var(--border); }
.row.actions { flex-direction: row; flex-wrap: wrap; gap: 8px; border-bottom: 0; }
.row label { font-size: 10px; text-transform: uppercase; letter-spacing: 0.6px; color: var(--text-faint); }
.path-line { display: flex; gap: 8px; align-items: center; }
.path { flex: 1; background: var(--bg-input); padding: 5px 8px; border-radius: 4px; font: 11px monospace; color: var(--text); word-break: break-all; }
.btn { background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; color: var(--text-dim); font-size: 12px; padding: 5px 12px; }
.btn:hover:not(:disabled) { color: var(--text); border-color: var(--accent); }
.btn:disabled { opacity: 0.55; cursor: default; }
.btn.primary { background: var(--accent); color: var(--accent-text); border-color: transparent; font-weight: 600; }
.btn.primary:hover:not(:disabled) { filter: brightness(1.1); }
.status { display: flex; gap: 6px; align-items: center; }
.badge { font: 700 10px monospace; padding: 2px 8px; border-radius: 10px; }
.badge.ok    { background: color-mix(in srgb, var(--ok) 18%, transparent);     color: var(--ok); }
.badge.warn  { background: color-mix(in srgb, var(--warn-text) 18%, transparent); color: var(--warn-text); }
.badge.clean { background: color-mix(in srgb, var(--text-faint) 18%, transparent); color: var(--text-dim); }
.badge.branch{ background: color-mix(in srgb, var(--accent) 16%, transparent); color: var(--accent); }
.branch-line { display: flex; gap: 8px; align-items: center; }
.branch-line select { background: var(--bg-input); border: 1px solid var(--border-strong); color: var(--text); border-radius: 4px; padding: 5px 8px; font: 12px monospace; }
.branch-line select:focus { outline: none; border-color: var(--accent); }
.porcelain { background: var(--bg-input); border: 1px solid var(--border); border-radius: 4px; padding: 8px; max-height: 160px; overflow: auto; color: var(--text); font: 11px/1.5 monospace; white-space: pre; }
.msg { font-size: 11px; color: var(--text); padding: 6px 18px 14px; white-space: pre-wrap; word-break: break-word; }
.tracking { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; margin-top: 4px; }
.action-sep { width: 1px; height: 20px; background: var(--border); align-self: center; }
.hint { font-size: 11px; color: var(--text-faint); }
.hint code { background: var(--bg-input); padding: 0 4px; border-radius: 3px; font-size: 10.5px; }
</style>
