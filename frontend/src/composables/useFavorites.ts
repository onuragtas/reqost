import { reactive, watch } from 'vue'

// Favorites = a flat Set of tree node ids the user has starred. Persisted in
// localStorage so it survives across sessions; **not** workspace-scoped right
// now (the tree id is unique across the active workspace's index anyway).

const KEY = 'reqost:favorites:v1'

function load(): Set<string> {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return new Set()
    const arr = JSON.parse(raw)
    return Array.isArray(arr) ? new Set(arr) : new Set()
  } catch { return new Set() }
}

// Vue's `reactive` doesn't track Set mutations directly — wrap inside an
// object and bump a tick counter on every mutation so consumers re-render.
const state = reactive({
  set: load(),
  tick: 0,
})

watch(() => state.tick, () => {
  try { localStorage.setItem(KEY, JSON.stringify(Array.from(state.set))) } catch { /* ignore */ }
})

export function useFavorites() {
  function isFav(id: string): boolean {
    void state.tick               // touch so isFav is reactive on tick change
    return state.set.has(id)
  }
  function toggle(id: string) {
    if (state.set.has(id)) state.set.delete(id)
    else state.set.add(id)
    state.tick++
  }
  function add(id: string)    { if (!state.set.has(id)) { state.set.add(id); state.tick++ } }
  function remove(id: string) { if (state.set.has(id))  { state.set.delete(id); state.tick++ } }
  function all(): string[]    { void state.tick; return Array.from(state.set) }
  return { isFav, toggle, add, remove, all }
}
