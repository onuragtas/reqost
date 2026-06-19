import { reactive, watch } from 'vue'

// Map: workspaceId → bound git directory absolute path. Persisted in
// localStorage so the UI remembers the binding across launches. Backend
// stores nothing about the binding — the git directory itself is the source
// of truth for what's there.

const KEY = 'reqost:git-bind:v1'

function load(): Record<string, string> {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return {}
    const o = JSON.parse(raw)
    return (o && typeof o === 'object') ? o as Record<string, string> : {}
  } catch { return {} }
}

const state = reactive<{ binds: Record<string, string> }>({ binds: load() })

watch(() => state.binds, (b) => {
  try { localStorage.setItem(KEY, JSON.stringify(b)) } catch { /* ignore */ }
}, { deep: true })

export function useGitBind() {
  function get(workspaceId: string): string {
    return state.binds[workspaceId] ?? ''
  }
  function set(workspaceId: string, path: string) {
    if (!workspaceId) return
    if (path) state.binds[workspaceId] = path
    else delete state.binds[workspaceId]
  }
  return { get, set, all: state.binds }
}
