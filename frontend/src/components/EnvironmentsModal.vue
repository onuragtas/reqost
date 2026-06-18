<script setup lang="ts">
import { ref, computed, watch, reactive } from 'vue'
import { useEnv, type Environment } from '../composables/useEnv'

// Per-row reveal state for secret variables. Keyed by `${envId}:${idx}`; lost
// on modal close (no persistence — clipboard-leak guard).
const revealed = reactive<Record<string, boolean>>({})
function revealKey(envId: string, i: number) { return `${envId}:${i}` }
function isRevealed(envId: string, i: number) { return !!revealed[revealKey(envId, i)] }
function toggleReveal(envId: string, i: number) {
  const k = revealKey(envId, i)
  revealed[k] = !revealed[k]
}

const { environments, activeId, modalOpen, closeModal, createEnv, deleteEnv, addVar, removeVar, setActive, touch } = useEnv()

const editId = ref('')
const editing = computed<Environment | null>(() => environments.value.find(e => e.id === editId.value) ?? null)
const fileInput = ref<HTMLInputElement | null>(null)

watch(modalOpen, (open) => {
  if (open && !editing.value) editId.value = activeId.value || environments.value[0]?.id || ''
})

function onCreate() {
  const env = createEnv()
  editId.value = env.id
}
function onDelete(id: string) {
  deleteEnv(id)
  if (editId.value === id) editId.value = environments.value[0]?.id ?? ''
}

function onExport(env: Environment) {
  const data = {
    id: env.id,
    name: env.name,
    values: env.vars.map(v => ({ key: v.key, value: v.value, enabled: v.enabled, type: 'default' })),
    _postman_variable_scope: 'environment',
  }
  const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${env.name || 'environment'}.postman_environment.json`
  a.click()
  URL.revokeObjectURL(url)
}

function onImportClick() { fileInput.value?.click() }

function onFileChange(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = (ev) => {
    try {
      const data = JSON.parse(ev.target?.result as string)
      const name = data.name || file.name.replace(/\.json$/, '')
      const vars = (data.values || data.vars || []).map((v: any) => ({
        key: String(v.key ?? ''),
        value: String(v.value ?? ''),
        enabled: v.enabled !== false,
      })).filter((v: any) => v.key)
      const env = createEnv(name)
      env.vars = vars
      touch()
      editId.value = env.id
    } catch { /* invalid file */ }
    if (fileInput.value) fileInput.value.value = ''
  }
  reader.readAsText(file)
}
</script>

<template>
  <div v-if="modalOpen" class="overlay" @click.self="closeModal">
    <div class="modal">
      <header>
        <h3>Environments</h3>
        <button class="x" @click="closeModal">✕</button>
      </header>

      <div class="cols">
        <!-- env list -->
        <div class="list">
          <div
            v-for="e in environments" :key="e.id"
            class="env-item" :class="{ sel: e.id === editId }"
            @click="editId = e.id"
          >
            <button
              class="active-dot" :class="{ on: e.id === activeId }"
              :title="e.id === activeId ? 'Active' : 'Set active'"
              @click.stop="setActive(e.id)"
            ></button>
            <span class="env-name">{{ e.name || 'Untitled' }}</span>
            <button class="del" @click.stop="onDelete(e.id)">✕</button>
          </div>
          <div class="env-actions">
            <button class="add-env" @click="onCreate">+ New</button>
            <button class="add-env" @click="onImportClick">↑ Import</button>
          </div>
          <input ref="fileInput" type="file" accept=".json" style="display:none" @change="onFileChange" />
        </div>

        <!-- var editor -->
        <div class="editor selectable">
          <template v-if="editing">
            <div class="name-row">
              <input v-model="editing.name" class="name-input" placeholder="Environment name" @input="touch" />
              <button class="export-btn" title="Export as Postman JSON" @click="onExport(editing)">↓ Export</button>
            </div>
            <div class="vars-head"><span></span><span>Variable</span><span>Value</span><span title="Secret">🔒</span><span></span></div>
            <div class="vars">
              <div v-for="(v, i) in editing.vars" :key="i" class="var-row">
                <input type="checkbox" v-model="v.enabled" @change="touch" />
                <input v-model="v.key" placeholder="key" class="v-key" @input="touch" />
                <input
                  v-model="v.value"
                  :type="v.secret && !isRevealed(editing.id, i) ? 'password' : 'text'"
                  :placeholder="v.secret ? '••••••••' : 'value'"
                  class="v-val"
                  :class="{ secret: v.secret }"
                  @input="touch"
                />
                <button
                  class="secret-btn" :class="{ on: v.secret }"
                  :title="v.secret ? 'Unmark as secret' : 'Mark as secret (masked)'"
                  @click="v.secret = !v.secret; touch()"
                >🔒</button>
                <button v-if="v.secret" class="reveal-btn" :title="isRevealed(editing.id, i) ? 'Hide' : 'Reveal'" @click="toggleReveal(editing.id, i)">{{ isRevealed(editing.id, i) ? '🙈' : '👁' }}</button>
                <button v-else class="reveal-btn placeholder" disabled></button>
                <button class="del" @click="removeVar(editing, i)">✕</button>
              </div>
            </div>
            <button class="add-var" @click="addVar(editing)">+ Add variable</button>
          </template>
          <div v-else class="no-sel">Create or select an environment</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 100; }
.modal { width: 760px; max-width: 92vw; height: 70vh; background: var(--bg-elevated); border: 1px solid var(--border-strong); border-radius: 10px; display: flex; flex-direction: column; overflow: hidden; box-shadow: 0 20px 60px rgba(0,0,0,0.5); }
header { display: flex; align-items: center; justify-content: space-between; padding: 14px 18px; border-bottom: 1px solid var(--border); }
h3 { font-size: 15px; font-weight: 600; }
.x { color: var(--text-dim); font-size: 14px; width: 24px; height: 24px; border-radius: 5px; }
.x:hover { background: var(--bg-hover); color: var(--text); }

.cols { flex: 1; display: flex; overflow: hidden; }
.list { width: 240px; flex-shrink: 0; border-right: 1px solid var(--border); padding: 8px; overflow-y: auto; display: flex; flex-direction: column; gap: 2px; }
.env-item { display: flex; align-items: center; gap: 8px; padding: 7px 8px; border-radius: 6px; cursor: pointer; color: var(--text-dim); }
.env-item:hover { background: var(--bg-hover); }
.env-item.sel { background: var(--bg-hover); color: var(--text); }
.env-name { flex: 1; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.active-dot { width: 12px; height: 12px; border-radius: 50%; border: 2px solid var(--border-strong); flex-shrink: 0; }
.active-dot.on { background: var(--accent); border-color: var(--accent); }
.del { color: var(--text-faint); font-size: 11px; width: 18px; height: 18px; border-radius: 4px; flex-shrink: 0; }
.del:hover { background: var(--border-strong); color: var(--danger); }
.env-actions { display: flex; gap: 4px; margin-top: 6px; }
.add-env { flex: 1; border: 1px dashed var(--border-strong); border-radius: 6px; color: var(--text-dim); font-size: 12px; padding: 8px; }
.add-env:hover { color: var(--text); }
.name-row { display: flex; gap: 8px; align-items: center; margin-bottom: 14px; }
.name-row .name-input { flex: 1; margin-bottom: 0; }
.export-btn { flex-shrink: 0; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 6px; color: var(--text-dim); font-size: 12px; padding: 7px 10px; }
.export-btn:hover { color: var(--text); }

.editor { flex: 1; padding: 16px; overflow-y: auto; }
.name-input { width: 100%; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 6px; font-size: 14px; font-weight: 600; padding: 8px 10px; margin-bottom: 14px; }
.name-input:focus, .v-key:focus, .v-val:focus { outline: none; border-color: var(--accent); }
.vars-head { display: grid; grid-template-columns: 22px 1fr 1fr 22px 22px 22px; gap: 6px; font-size: 10px; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-faint); padding: 0 0 6px; }
.vars { display: flex; flex-direction: column; gap: 6px; }
.var-row { display: grid; grid-template-columns: 22px 1fr 1fr 22px 22px 22px; gap: 6px; align-items: center; }
.v-key, .v-val { background: var(--bg-input); border: 1px solid var(--border); border-radius: 4px; font: 12px monospace; padding: 6px 8px; }
.v-val.secret { color: var(--warn-text); font-family: monospace; letter-spacing: 0.04em; }
.secret-btn, .reveal-btn { font-size: 11px; padding: 2px 0; border-radius: 4px; color: var(--text-faint); opacity: 0.5; }
.secret-btn:hover, .reveal-btn:hover { background: var(--bg-hover); opacity: 1; }
.secret-btn.on { opacity: 1; color: var(--warn-text); }
.reveal-btn.placeholder { background: transparent; cursor: default; opacity: 0; }
.reveal-btn:disabled { background: transparent; }
.add-var { align-self: flex-start; margin-top: 8px; border: 1px dashed var(--border-strong); border-radius: 4px; color: var(--text-dim); font-size: 12px; padding: 6px 10px; }
.no-sel { display: flex; align-items: center; justify-content: center; height: 100%; color: var(--text-faint); font-size: 13px; }
</style>
