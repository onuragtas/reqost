<script setup lang="ts">
import { ref } from 'vue'
import { Call, ListMethods } from '../../bindings/reqost/grpcservice'
import { useEnv } from '../composables/useEnv'
import { interpolate } from '../composables/interpolate'
import type { ReqTab } from '../composables/useTabs'

const props = defineProps<{ tab: ReqTab }>()
const { activeVars } = useEnv()

const methods = ref<string[]>([])
const invoking = ref(false)
const listing = ref(false)
const response = ref('')
const error = ref('')

// tab.url holds the target. The scheme (grpc:// plaintext, grpcs:// TLS, or
// bare) is handled by the backend; we just interpolate {{vars}} and trim.
function target(): string {
  return interpolate(props.tab.url.trim(), activeVars.value)
}

async function list() {
  listing.value = true
  error.value = ''
  try {
    methods.value = (await ListMethods(target())) ?? []
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  } finally {
    listing.value = false
  }
}

async function invoke() {
  invoking.value = true
  error.value = ''
  response.value = ''
  try {
    const method = interpolate(props.tab.grpcMethod.trim(), activeVars.value)
    const body = interpolate(props.tab.body, activeVars.value)
    const res: any = await Call(target(), method, body)
    if (res?.error) error.value = res.error
    else response.value = res?.body ?? ''
  } catch (e: any) {
    error.value = e?.message ?? String(e)
  } finally {
    invoking.value = false
  }
}
</script>

<template>
  <div class="grpc">
    <div class="bar">
      <span class="proto">gRPC</span>
      <input v-model="tab.url" class="target" placeholder="host:port (plaintext)" />
      <button class="list" :disabled="listing" @click="list">{{ listing ? '…' : 'Reflect' }}</button>
      <button class="invoke" :disabled="invoking" @click="invoke">{{ invoking ? '…' : 'Invoke' }}</button>
    </div>

    <div class="method-row">
      <input v-model="tab.grpcMethod" class="method" list="grpc-methods" placeholder="package.Service/Method" />
      <datalist id="grpc-methods">
        <option v-for="m in methods" :key="m" :value="m" />
      </datalist>
    </div>

    <div class="panes">
      <div class="pane">
        <div class="pane-head">Request (JSON)</div>
        <textarea v-model="tab.body" class="editor selectable" spellcheck="false" placeholder="{ }" />
      </div>
      <div class="pane">
        <div class="pane-head">Response</div>
        <pre v-if="error" class="editor err selectable">{{ error }}</pre>
        <pre v-else class="editor selectable">{{ response || '—' }}</pre>
      </div>
    </div>
  </div>
</template>

<style scoped>
.grpc { flex: 1; display: flex; flex-direction: column; overflow: hidden; background: var(--bg); }
.bar { display: flex; gap: 8px; align-items: center; padding: 12px 16px; border-bottom: 1px solid var(--border); background: var(--bg-elevated); }
.proto { font: 700 11px monospace; color: var(--accent); background: color-mix(in srgb, var(--accent) 15%, transparent); padding: 4px 7px; border-radius: 4px; }
.target { flex: 1; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; font: 13px monospace; padding: 8px 10px; }
.list { background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; color: var(--text-dim); padding: 0 14px; }
.invoke { background: var(--accent); color: var(--accent-text); border-radius: 5px; font-weight: 700; padding: 0 18px; }
.target:focus, .method:focus { outline: none; border-color: var(--accent); }
.method-row { padding: 10px 16px 0; }
.method { width: 100%; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; font: 12px monospace; padding: 7px 10px; }

.panes { flex: 1; display: flex; gap: 12px; padding: 12px 16px; overflow: hidden; }
.pane { flex: 1; display: flex; flex-direction: column; gap: 6px; min-width: 0; }
.pane-head { font-size: 10px; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-faint); }
.editor { flex: 1; background: var(--bg-input); border: 1px solid var(--border); border-radius: 5px; color: var(--text); font: 12px/1.5 monospace; padding: 10px; resize: none; overflow: auto; white-space: pre-wrap; word-break: break-word; }
.editor:focus { outline: none; border-color: var(--accent); }
.editor.err { color: var(--danger); }
</style>
