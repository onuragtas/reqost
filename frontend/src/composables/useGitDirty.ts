import { reactive, watch } from 'vue'
import { Events } from '@wailsio/runtime'

// Tracks whether the active workspace has in-app edits that haven't been
// exported to its bound Git directory yet. Backend emits `collection:edited`
// after every Save/Create/Delete/Rename/Move; we flip the per-workspace flag
// to `true` and let the title bar + GitModal surface that to the user.
//
// Cleared after a successful Snapshot via `clear(wsId)`.

const KEY = 'reqost:git-dirty:v1'

function load(): Record<string, boolean> {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return {}
    const o = JSON.parse(raw)
    return (o && typeof o === 'object') ? o as Record<string, boolean> : {}
  } catch { return {} }
}

const state = reactive<{ dirty: Record<string, boolean>; activeWs: string }>({
  dirty: load(),
  activeWs: '',
})

watch(() => state.dirty, (d) => {
  try { localStorage.setItem(KEY, JSON.stringify(d)) } catch { /* ignore */ }
}, { deep: true })

let listenerAttached = false
function attach() {
  if (listenerAttached) return
  listenerAttached = true
  Events.On('collection:edited', () => {
    if (state.activeWs) state.dirty[state.activeWs] = true
  })
  // ImportCollection wipes most of the index; treat it as edited too.
  Events.On('collection:ready', (ev: any) => {
    const reason = ev?.data ?? ev
    // Workspace switches set state via a separate code path; only flip dirty
    // for real imports / OpenAPI / HAR pastes.
    if (reason && reason !== 'workspace-switch' && state.activeWs) {
      state.dirty[state.activeWs] = true
    }
  })
}

export function useGitDirty() {
  attach()
  function setActive(wsId: string) { state.activeWs = wsId }
  function isDirty(wsId: string): boolean { return !!state.dirty[wsId] }
  function clear(wsId: string)    { if (state.dirty[wsId]) state.dirty[wsId] = false }
  function markDirty(wsId: string) { if (wsId) state.dirty[wsId] = true }
  return { setActive, isDirty, clear, markDirty }
}
