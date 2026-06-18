<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useEnv, type Environment } from '../composables/useEnv'

const { environments, activeId, modalOpen, closeModal, createEnv, deleteEnv, addVar, removeVar, setActive, touch } = useEnv()

const editId = ref('')
const editing = computed<Environment | null>(() => environments.value.find(e => e.id === editId.value) ?? null)

// When the modal opens, focus the active env (or the first one).
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
          <button class="add-env" @click="onCreate">+ New Environment</button>
        </div>

        <!-- var editor -->
        <div class="editor selectable">
          <template v-if="editing">
            <input v-model="editing.name" class="name-input" placeholder="Environment name" @input="touch" />
            <div class="vars-head"><span></span><span>Variable</span><span>Value</span><span></span></div>
            <div class="vars">
              <div v-for="(v, i) in editing.vars" :key="i" class="var-row">
                <input type="checkbox" v-model="v.enabled" @change="touch" />
                <input v-model="v.key" placeholder="key" class="v-key" @input="touch" />
                <input v-model="v.value" placeholder="value" class="v-val" @input="touch" />
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
.add-env { margin-top: 6px; border: 1px dashed var(--border-strong); border-radius: 6px; color: var(--text-dim); font-size: 12px; padding: 8px; }
.add-env:hover { color: var(--text); }

.editor { flex: 1; padding: 16px; overflow-y: auto; }
.name-input { width: 100%; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 6px; font-size: 14px; font-weight: 600; padding: 8px 10px; margin-bottom: 14px; }
.name-input:focus, .v-key:focus, .v-val:focus { outline: none; border-color: var(--accent); }
.vars-head { display: grid; grid-template-columns: 22px 1fr 1fr 22px; gap: 6px; font-size: 10px; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-faint); padding: 0 0 6px; }
.vars { display: flex; flex-direction: column; gap: 6px; }
.var-row { display: grid; grid-template-columns: 22px 1fr 1fr 22px; gap: 6px; align-items: center; }
.v-key, .v-val { background: var(--bg-input); border: 1px solid var(--border); border-radius: 4px; font: 12px monospace; padding: 6px 8px; }
.add-var { align-self: flex-start; margin-top: 8px; border: 1px dashed var(--border-strong); border-radius: 4px; color: var(--text-dim); font-size: 12px; padding: 6px 10px; }
.no-sel { display: flex; align-items: center; justify-content: center; height: 100%; color: var(--text-faint); font-size: 13px; }
</style>
