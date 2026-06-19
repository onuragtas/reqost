import { reactive, computed } from 'vue'
import { useEnv } from './useEnv'

const VAR_RE = /\{\{([^}]+)\}\}/g

interface Pair {
  name: string
  value: string
  found: boolean
  // true = from active env (will be substituted); false = from another env (won't be substituted)
  active: boolean
  // name of the env it comes from (for display)
  envName: string
}
interface State { visible: boolean; pairs: Pair[]; x: number; y: number }

const state: State = reactive({ visible: false, pairs: [], x: 0, y: 0 })

export function useVarHint() {
  const { environments, activeId, activeVars } = useEnv()

  // Map every var to its source env name (active env takes precedence on duplicate keys).
  const varSourceMap = computed<Record<string, { value: string; envName: string; active: boolean }>>(() => {
    const out: Record<string, { value: string; envName: string; active: boolean }> = {}
    // Non-active envs first (lower priority)
    for (const env of environments.value) {
      if (env.id === activeId.value) continue
      for (const v of env.vars) {
        if (v.enabled && v.key.trim()) {
          out[v.key] = { value: v.value, envName: env.name || 'Unnamed', active: false }
        }
      }
    }
    // Active env overrides — these will actually be substituted
    for (const [k, v] of Object.entries(activeVars.value)) {
      const envName = environments.value.find(e => e.id === activeId.value)?.name || 'Active'
      out[k] = { value: v, envName, active: true }
    }
    return out
  })

  function showVarHint(e: MouseEvent, text: string) {
    const seen = new Set<string>()
    const pairs: Pair[] = []
    VAR_RE.lastIndex = 0
    let m: RegExpExecArray | null
    while ((m = VAR_RE.exec(text)) !== null) {
      const name = m[1].trim()
      if (!name || seen.has(name)) continue
      seen.add(name)
      const src = varSourceMap.value[name]
      pairs.push({
        name,
        value: src?.value ?? '',
        found: src !== undefined,
        active: src?.active ?? false,
        envName: src?.envName ?? '',
      })
    }
    if (pairs.length === 0) { state.visible = false; return }
    const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
    state.pairs = pairs
    state.x = rect.left
    state.y = rect.bottom + 6
    state.visible = true
  }

  function hideVarHint() { state.visible = false }

  return { varHint: state, showVarHint, hideVarHint }
}
