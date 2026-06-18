<script setup lang="ts">
import { ref, onMounted } from 'vue'
import EditorPane from './EditorPane.vue'
import {
  LoadSpec, SaveSpec, StartMock, StopMock, MockStatus,
} from '../../bindings/reqost/designservice'

// DesignPanel — minimal Insomnia-Design-style mode. The user edits the spec
// in a CodeMirror buffer; Save persists to the cache dir, Start runs an
// in-app HTTP mock server that replies from the spec's response examples.

const spec = ref('')
const saving = ref(false)
const port = ref(8095)
const status = ref<{ running: boolean; port: number; error: string }>({ running: false, port: 0, error: '' })

async function refreshStatus() {
  try {
    const s: any = await MockStatus()
    status.value = { running: !!s?.running, port: s?.port ?? 0, error: s?.error ?? '' }
  } catch { /* ignore */ }
}
async function load() {
  try { spec.value = await LoadSpec() ?? '' } catch { spec.value = '' }
}
async function save() {
  saving.value = true
  try { await SaveSpec(spec.value) } finally { saving.value = false }
}
async function start() {
  await save()
  try { await StartMock(port.value) } catch { /* ignore */ }
  await refreshStatus()
}
async function stop() {
  try { await StopMock() } catch { /* ignore */ }
  await refreshStatus()
}

onMounted(async () => { await load(); await refreshStatus() })
</script>

<template>
  <aside class="design selectable">
    <header class="head">
      <span class="title">Design — OpenAPI</span>
      <span class="hint">Edit YAML/JSON. Mock server replies with response examples.</span>
    </header>

    <div class="bar">
      <button class="btn" :disabled="saving" @click="save">{{ saving ? 'Saving…' : 'Save' }}</button>
      <span class="sep"></span>
      <label class="port">
        Port
        <input type="number" min="1" max="65535" v-model.number="port" />
      </label>
      <button v-if="!status.running" class="btn primary" @click="start">▶ Start mock</button>
      <button v-else class="btn danger" @click="stop">■ Stop ({{ status.port }})</button>
      <span v-if="status.running" class="dot ok"></span>
      <span v-else-if="status.error" class="err">⚠ {{ status.error }}</span>
    </div>

    <div class="editor-wrap">
      <EditorPane v-model="spec" language="javascript" min-height="100%" />
    </div>
  </aside>
</template>

<style scoped>
.design {
  display: flex; flex-direction: column; flex: 1; min-width: 0;
  background: var(--bg-panel); border-right: 1px solid var(--border);
  overflow: hidden;
}
.head { display: flex; flex-direction: column; gap: 2px; padding: 10px 14px; border-bottom: 1px solid var(--border); }
.title { font-size: 13px; font-weight: 700; color: var(--text); }
.hint { font-size: 11px; color: var(--text-faint); }
.bar { display: flex; gap: 8px; align-items: center; padding: 8px 14px; background: var(--bg-elevated); border-bottom: 1px solid var(--border); }
.btn { background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; color: var(--text-dim); font-size: 12px; padding: 5px 12px; }
.btn:hover:not(:disabled) { color: var(--text); }
.btn:disabled { opacity: 0.6; cursor: default; }
.btn.primary { background: var(--accent); color: var(--accent-text); border-color: transparent; font-weight: 600; }
.btn.danger { background: var(--danger); color: #fff; border-color: transparent; font-weight: 600; }
.sep { width: 1px; height: 18px; background: var(--border); }
.port { display: flex; align-items: center; gap: 6px; font-size: 11px; color: var(--text-dim); }
.port input { width: 70px; background: var(--bg-input); border: 1px solid var(--border); border-radius: 4px; color: var(--text); font: 12px monospace; padding: 3px 6px; text-align: right; }
.port input:focus { outline: none; border-color: var(--accent); }
.dot { width: 9px; height: 9px; border-radius: 50%; background: var(--text-faint); margin-left: auto; }
.dot.ok { background: var(--ok); }
.err { color: var(--danger); font-size: 11px; margin-left: auto; }
.editor-wrap { flex: 1; padding: 8px 12px; overflow: hidden; display: flex; flex-direction: column; }
.editor-wrap > * { flex: 1; }
</style>
