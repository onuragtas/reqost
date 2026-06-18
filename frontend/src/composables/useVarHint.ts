import { reactive } from 'vue'
import { useEnv } from './useEnv'

const VAR_RE = /\{\{([^}]+)\}\}/g

interface Pair { name: string; value: string; found: boolean }
interface State { visible: boolean; pairs: Pair[]; x: number; y: number }

const state: State = reactive({ visible: false, pairs: [], x: 0, y: 0 })

export function useVarHint() {
  const { activeVars } = useEnv()

  function showVarHint(e: MouseEvent, text: string) {
    const seen = new Set<string>()
    const pairs: Pair[] = []
    VAR_RE.lastIndex = 0
    let m: RegExpExecArray | null
    while ((m = VAR_RE.exec(text)) !== null) {
      const name = m[1].trim()
      if (!name || seen.has(name)) continue
      seen.add(name)
      const val = activeVars.value[name]
      pairs.push({ name, value: val ?? '', found: val !== undefined })
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
