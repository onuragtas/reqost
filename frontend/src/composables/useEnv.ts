import { reactive, computed } from 'vue'
import { LoadEnvironments, SaveEnvironments } from '../../bindings/reqost/envservice'

export interface EnvVar { key: string; value: string; enabled: boolean; secret?: boolean }
export interface Environment { id: string; name: string; vars: EnvVar[] }

const state = reactive({
  activeId: '',
  environments: [] as Environment[],
  loaded: false,
  modalOpen: false,
})

function genId(): string {
  try { return crypto.randomUUID() } catch { return `env-${Date.now()}-${Math.floor(Math.random() * 1e6)}` }
}

let saveTimer: ReturnType<typeof setTimeout>
function persist() {
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    void SaveEnvironments({ activeId: state.activeId, environments: state.environments as any })
  }, 300)
}

export function useEnv() {
  async function loadEnvironments() {
    if (state.loaded) return
    const s: any = await LoadEnvironments()
    state.activeId = s?.activeId ?? ''
    state.environments = (s?.environments ?? []).map((e: any): Environment => ({
      id: e.id, name: e.name,
      vars: (e.vars ?? []).map((v: any): EnvVar => ({ key: v.key ?? '', value: v.value ?? '', enabled: v.enabled !== false })),
    }))
    state.loaded = true
  }

  const environments = computed(() => state.environments)
  const activeId = computed(() => state.activeId)
  const active = computed(() => state.environments.find(e => e.id === state.activeId) ?? null)

  function setActive(id: string) { state.activeId = id; persist() }

  // Resolved enabled vars of the active environment, ready for {{interpolation}}.
  const activeVars = computed<Record<string, string>>(() => {
    const out: Record<string, string> = {}
    const env = state.environments.find(e => e.id === state.activeId)
    if (env) for (const v of env.vars) if (v.enabled && v.key.trim()) out[v.key] = v.value
    return out
  })

  function createEnv(name = 'New Environment'): Environment {
    const env: Environment = { id: genId(), name, vars: [] }
    state.environments.push(env)
    state.activeId = env.id
    persist()
    return env
  }
  function deleteEnv(id: string) {
    const i = state.environments.findIndex(e => e.id === id)
    if (i === -1) return
    state.environments.splice(i, 1)
    if (state.activeId === id) state.activeId = state.environments[0]?.id ?? ''
    persist()
  }
  // applyVars merges variable changes a script made (pm.environment.set) back
  // into the active environment and persists. New keys are appended.
  function applyVars(vars: Record<string, string>) {
    const env = state.environments.find(e => e.id === state.activeId)
    if (!env) return
    let changed = false
    for (const [k, v] of Object.entries(vars)) {
      const existing = env.vars.find(x => x.key === k)
      if (existing) {
        if (existing.value !== v) { existing.value = v; changed = true }
      } else {
        env.vars.push({ key: k, value: v, enabled: true }); changed = true
      }
    }
    if (changed) persist()
  }

  function addVar(env: Environment) { env.vars.push({ key: '', value: '', enabled: true }); persist() }
  function removeVar(env: Environment, i: number) { env.vars.splice(i, 1); persist() }
  function touch() { persist() }

  const modalOpen = computed(() => state.modalOpen)
  function openModal() { state.modalOpen = true }
  function closeModal() { state.modalOpen = false }

  return {
    environments, activeId, active, activeVars,
    loadEnvironments, setActive, createEnv, deleteEnv, addVar, removeVar, touch, applyVars,
    modalOpen, openModal, closeModal,
  }
}
