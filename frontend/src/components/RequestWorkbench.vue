<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { SendRequest, Cancel, GetCookies, ClearCookies } from '../../bindings/reqost/execservice'
import { SaveRequest } from '../../bindings/reqost/collectionservice'
import { useTabs, toDetail, markClean, type ReqSubTab, type ResSubTab, type AuthType, type BodyType } from '../composables/useTabs'
import { useEnv } from '../composables/useEnv'
import { useHistory } from '../composables/useHistory'
import { useTree } from '../composables/useTree'
import { useSettings } from '../composables/useSettings'
import { parseQuery, buildUrl, baseOf } from '../composables/url'
import WsConsole from './WsConsole.vue'
import GrpcConsole from './GrpcConsole.vue'

const { active } = useTabs()
const { activeVars, applyVars } = useEnv()
const { record } = useHistory()
const { refreshNode } = useTree()
const { settings: appSettings } = useSettings()

// Switch protocol UI by URL scheme: ws/wss → WebSocket, grpc → gRPC, else HTTP.
const mode = computed<'http' | 'ws' | 'grpc'>(() => {
  const u = active.value?.url?.trim().toLowerCase() ?? ''
  if (u.startsWith('ws://') || u.startsWith('wss://')) return 'ws'
  if (u.startsWith('grpc://')) return 'grpc'
  return 'http'
})

const BODY_TYPES: { id: BodyType; label: string }[] = [
  { id: 'none', label: 'None' },
  { id: 'raw', label: 'Raw' },
  { id: 'json', label: 'JSON' },
  { id: 'urlencoded', label: 'x-www-form-urlencoded' },
  { id: 'formdata', label: 'form-data' },
  { id: 'graphql', label: 'GraphQL' },
]

const saving = ref(false)
const savedFlash = ref(false)

async function save() {
  const t = active.value
  if (!t) return
  saving.value = true
  try {
    await SaveRequest(toDetail(t) as any)
    markClean(t)
    refreshNode(t.id, { name: t.name, method: t.method })
    savedFlash.value = true
    setTimeout(() => (savedFlash.value = false), 1200)
  } finally {
    saving.value = false
  }
}

function addForm() { active.value?.formFields.push({ key: '', value: '', type: 'text', enabled: true }) }
function removeForm(i: number) { active.value?.formFields.splice(i, 1) }

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']
const METHOD_COLORS: Record<string, string> = {
  GET: '#61affe', POST: '#49cc90', PUT: '#fca130', PATCH: '#50e3c2', DELETE: '#f93e3e',
}
const AUTH_TYPES: { id: AuthType; label: string }[] = [
  { id: 'none', label: 'No Auth' },
  { id: 'bearer', label: 'Bearer Token' },
  { id: 'basic', label: 'Basic Auth' },
  { id: 'apikey', label: 'API Key' },
]

const REQ_TABS: { id: ReqSubTab; label: string; soon?: boolean }[] = [
  { id: 'params', label: 'Params' },
  { id: 'auth', label: 'Auth' },
  { id: 'headers', label: 'Headers' },
  { id: 'body', label: 'Body' },
  { id: 'prereq', label: 'Pre-req' },
  { id: 'tests', label: 'Tests' },
  { id: 'settings', label: 'Settings' },
]
const RES_TABS: { id: ResSubTab; label: string; soon?: boolean }[] = [
  { id: 'body', label: 'Body' },
  { id: 'headers', label: 'Headers' },
  { id: 'cookies', label: 'Cookies' },
  { id: 'testResults', label: 'Test Results' },
]

// Literal braces can't appear inside Vue template interpolation, so keep these
// as plain script constants.
const URL_PLACEHOLDER = 'https://{{baseUrl}}/path'
const VAR_HINT = 'Values support {{variables}}.'

const headerCount = computed(() => active.value?.headers.filter(h => h.key.trim()).length ?? 0)
const paramCount = computed(() => active.value?.params.filter(p => p.key.trim()).length ?? 0)
const authOn = computed(() => active.value && active.value.auth.type !== 'none')

// ── URL <-> Params two-way sync (event-driven, no watchers to avoid loops) ──
function onUrlInput() {
  if (active.value) active.value.params = parseQuery(active.value.url)
}
function syncUrl() {
  const t = active.value
  if (t) t.url = buildUrl(baseOf(t.url), t.params)
}
function addParam() { active.value?.params.push({ key: '', value: '', enabled: true }) }
function removeParam(i: number) { active.value?.params.splice(i, 1); syncUrl() }

function addHeader() { active.value?.headers.push({ key: '', value: '', enabled: true }) }
function removeHeader(i: number) { active.value?.headers.splice(i, 1) }

async function send() {
  const t = active.value
  if (!t || !t.url.trim()) return
  t.sending = true
  t.sendError = ''
  t.response = null
  // GraphQL is sent as a JSON {query, variables} POST body.
  let body = t.body
  let bodyType: string = t.bodyType
  if (t.bodyType === 'graphql') {
    let vars: any = {}
    try { vars = t.graphqlVars.trim() ? JSON.parse(t.graphqlVars) : {} } catch { /* send empty vars */ }
    body = JSON.stringify({ query: t.body, variables: vars })
    bodyType = 'json'
  }

  // Resolve per-request settings → falling back to app-wide defaults.
  const s = t.settings
  const timeoutMs        = s.timeoutMs        ?? appSettings.defaultTimeoutMs
  const followRedirects  = s.followRedirects  ?? appSettings.defaultFollowRedirects
  const maxRedirects     = s.maxRedirects     ?? appSettings.defaultMaxRedirects
  const verifySSL        = s.verifySSL        ?? appSettings.defaultVerifySSL

  try {
    const res: any = await SendRequest(t.id, {
      protocol: 'http',
      method: t.method,
      url: t.url.trim(),
      headers: t.headers.filter(h => h.key.trim()),
      body,
      bodyType,
      formFields: t.formFields.filter(f => f.key.trim()),
      auth: t.auth.type === 'none' ? null : { ...t.auth },
      variables: activeVars.value,
      timeoutMs,
      disableRedirect: !followRedirects,
      maxRedirects,
      insecureSkipVerify: !verifySSL,
    }, t.preScript, t.postScript)

    const resp = res?.response
    t.response = resp
    t.tests = res?.tests ?? []
    t.logs = res?.logs ?? []
    if (res?.scriptError) t.logs = [...t.logs, `⚠ ${res.scriptError}`]
    if (res?.vars) applyVars(res.vars)
    t.resSubTab = t.tests.length ? 'testResults' : 'body'
    record({
      name: t.name, method: t.method, url: t.url.trim(),
      headers: t.headers.map(h => ({ ...h })), body: t.body, auth: { ...t.auth },
      status: resp?.status ?? 0, ms: resp?.timing?.totalMs ?? 0, ok: (resp?.status ?? 0) >= 200 && (resp?.status ?? 0) < 400,
    })
  } catch (e: any) {
    t.sendError = e?.message ?? String(e)
  } finally {
    t.sending = false
  }
}

const passCount = computed(() => active.value?.tests.filter(t => t.passed).length ?? 0)

// Cookies tab: cookies the session jar would send to this request's URL.
const cookies = ref<any[]>([])
async function loadCookies() {
  const t = active.value
  if (!t?.url) { cookies.value = []; return }
  cookies.value = (await GetCookies(t.url.trim())) ?? []
}
async function clearCookies() {
  await ClearCookies()
  await loadCookies()
}
watch(() => active.value?.resSubTab, (tab) => { if (tab === 'cookies') loadCookies() })
function cancel() {
  if (active.value) Cancel(active.value.id)
}

const statusColor = computed(() => {
  const s = active.value?.response?.status ?? 0
  if (s >= 200 && s < 300) return 'var(--ok)'
  if (s >= 300 && s < 400) return 'var(--warn-text)'
  if (s >= 400) return 'var(--danger)'
  return 'var(--text-dim)'
})
const prettyBody = computed(() => {
  const b = active.value?.response?.body ?? ''
  try { return JSON.stringify(JSON.parse(b), null, 2) } catch { return b }
})

function fmtSize(n: number) {
  if (n < 1024) return `${n} B`
  if (n < 1048576) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1048576).toFixed(2)} MB`
}
function fmtMs(n: number) { return n >= 1000 ? `${(n / 1000).toFixed(2)} s` : `${Math.round(n)} ms` }

// Tri-state binding helpers for the Settings subtab: undefined ⇒ 'inherit'.
function boolToTri(v: boolean | undefined): 'inherit' | 'true' | 'false' {
  if (v === undefined) return 'inherit'
  return v ? 'true' : 'false'
}
function triToBool(s: string): boolean | undefined {
  if (s === 'inherit') return undefined
  return s === 'true'
}
function onSetTimeout(v: string) {
  if (!active.value) return
  active.value.settings.timeoutMs = v === '' ? undefined : Math.max(0, Number(v))
}
function onSetMaxRedirects(v: string) {
  if (!active.value) return
  active.value.settings.maxRedirects = v === '' ? undefined : Math.max(0, Number(v))
}
function onSetFollow(s: string) {
  if (!active.value) return
  active.value.settings.followRedirects = triToBool(s)
}
function onSetVerifySSL(s: string) {
  if (!active.value) return
  active.value.settings.verifySSL = triToBool(s)
}
</script>

<template>
  <div class="workbench">
    <div v-if="!active" class="empty">
      <div class="empty-art">⌘</div>
      <p>Select a request from the sidebar to get started</p>
    </div>

    <div v-else-if="active.loading" class="empty">Loading…</div>

    <WsConsole v-else-if="mode === 'ws'" :tab="active" />
    <GrpcConsole v-else-if="mode === 'grpc'" :tab="active" />

    <template v-else>
      <!-- request name -->
      <div class="title-row">
        <input v-model="active.name" class="title-input" placeholder="Request name" spellcheck="false" />
      </div>
      <!-- URL bar -->
      <div class="url-bar">
        <select v-model="active.method" class="method" :style="{ color: METHOD_COLORS[active.method] ?? 'var(--text-dim)' }">
          <option v-for="m in METHODS" :key="m" :value="m">{{ m }}</option>
        </select>
        <input v-model="active.url" class="url" :placeholder="URL_PLACEHOLDER" @input="onUrlInput" @keyup.enter="send" />
        <button v-if="!active.sending" class="send" @click="send">Send</button>
        <button v-else class="cancel" @click="cancel">Cancel</button>
        <button class="save" :disabled="saving" @click="save">{{ savedFlash ? 'Saved ✓' : 'Save' }}</button>
      </div>

      <div class="split">
        <!-- request -->
        <section class="req">
          <div class="subtabs">
            <button
              v-for="rt in REQ_TABS" :key="rt.id"
              :class="{ active: active.reqSubTab === rt.id }"
              @click="active.reqSubTab = rt.id"
            >
              {{ rt.label }}
              <span v-if="rt.id === 'headers' && headerCount" class="count">{{ headerCount }}</span>
              <span v-else-if="rt.id === 'params' && paramCount" class="count">{{ paramCount }}</span>
              <span v-else-if="rt.id === 'auth' && authOn" class="dot-on"></span>
            </button>
          </div>

          <div class="subpanel selectable">
            <!-- Params -->
            <div v-if="active.reqSubTab === 'params'" class="kv">
              <div v-for="(p, i) in active.params" :key="i" class="kv-row">
                <input type="checkbox" v-model="p.enabled" @change="syncUrl" />
                <input v-model="p.key" placeholder="Key" class="kv-key" @input="syncUrl" />
                <input v-model="p.value" placeholder="Value" class="kv-val" @input="syncUrl" />
                <button class="kv-del" @click="removeParam(i)">✕</button>
              </div>
              <button class="add" @click="addParam">+ Add query param</button>
            </div>

            <!-- Auth -->
            <div v-else-if="active.reqSubTab === 'auth'" class="auth">
              <label class="auth-type">
                <span>Type</span>
                <select v-model="active.auth.type">
                  <option v-for="a in AUTH_TYPES" :key="a.id" :value="a.id">{{ a.label }}</option>
                </select>
              </label>
              <template v-if="active.auth.type === 'bearer'">
                <input v-model="active.auth.token" class="auth-in" placeholder="Token" />
              </template>
              <template v-else-if="active.auth.type === 'basic'">
                <input v-model="active.auth.username" class="auth-in" placeholder="Username" />
                <input v-model="active.auth.password" class="auth-in" placeholder="Password" />
              </template>
              <template v-else-if="active.auth.type === 'apikey'">
                <input v-model="active.auth.key" class="auth-in" placeholder="Header name (e.g. X-API-Key)" />
                <input v-model="active.auth.value" class="auth-in" placeholder="Value" />
              </template>
              <p v-if="active.auth.type !== 'none'" class="hint">{{ VAR_HINT }}</p>
            </div>

            <!-- Headers -->
            <div v-else-if="active.reqSubTab === 'headers'" class="kv">
              <div v-for="(h, i) in active.headers" :key="i" class="kv-row">
                <input type="checkbox" v-model="h.enabled" />
                <input v-model="h.key" placeholder="Key" class="kv-key" />
                <input v-model="h.value" placeholder="Value" class="kv-val" />
                <button class="kv-del" @click="removeHeader(i)">✕</button>
              </div>
              <button class="add" @click="addHeader">+ Add header</button>
            </div>

            <!-- Body -->
            <div v-else-if="active.reqSubTab === 'body'" class="body">
              <div class="body-type">
                <select v-model="active.bodyType">
                  <option v-for="bt in BODY_TYPES" :key="bt.id" :value="bt.id">{{ bt.label }}</option>
                </select>
              </div>
              <div v-if="active.bodyType === 'none'" class="soon"><span>This request has no body</span></div>
              <textarea
                v-else-if="active.bodyType === 'raw' || active.bodyType === 'json'"
                v-model="active.body" class="body-area" spellcheck="false" placeholder="Request body…"
              />
              <div v-else-if="active.bodyType === 'graphql'" class="gql">
                <div class="gql-label">Query</div>
                <textarea v-model="active.body" class="body-area script" spellcheck="false" placeholder="query { ... }" />
                <div class="gql-label">Variables (JSON)</div>
                <textarea v-model="active.graphqlVars" class="body-area script gql-vars" spellcheck="false" placeholder="{ }" />
              </div>
              <div v-else class="kv">
                <div v-for="(f, i) in active.formFields" :key="i" class="kv-row">
                  <input type="checkbox" v-model="f.enabled" />
                  <input v-model="f.key" placeholder="Key" class="kv-key" />
                  <select v-if="active.bodyType === 'formdata'" v-model="f.type" class="f-type">
                    <option value="text">Text</option>
                    <option value="file">File</option>
                  </select>
                  <input v-model="f.value" :placeholder="f.type === 'file' ? '/path/to/file' : 'Value'" class="kv-val" />
                  <button class="kv-del" @click="removeForm(i)">✕</button>
                </div>
                <button class="add" @click="addForm">+ Add field</button>
              </div>
            </div>

            <!-- Pre-request script -->
            <textarea
              v-else-if="active.reqSubTab === 'prereq'"
              v-model="active.preScript" class="body-area script" spellcheck="false"
              placeholder="// Pre-request script (JavaScript)&#10;// pm.environment.set('ts', Date.now())"
            />
            <!-- Test script -->
            <textarea
              v-else-if="active.reqSubTab === 'tests'"
              v-model="active.postScript" class="body-area script" spellcheck="false"
              placeholder="// Tests (JavaScript)&#10;// pm.test('status 200', () => pm.response.to.have.status(200))"
            />

            <!-- Settings: per-request execution options + description -->
            <div v-else-if="active.reqSubTab === 'settings'" class="settings">
              <div class="set-grid">
                <label>Timeout (ms)</label>
                <input
                  type="number" min="0" step="500"
                  :placeholder="`Inherit (${appSettings.defaultTimeoutMs})`"
                  :value="active.settings.timeoutMs ?? ''"
                  @input="onSetTimeout(($event.target as HTMLInputElement).value)"
                />

                <label>Follow redirects</label>
                <select
                  :value="boolToTri(active.settings.followRedirects)"
                  @change="onSetFollow(($event.target as HTMLSelectElement).value)"
                >
                  <option value="inherit">Inherit ({{ appSettings.defaultFollowRedirects ? 'On' : 'Off' }})</option>
                  <option value="true">On</option>
                  <option value="false">Off</option>
                </select>

                <label>Max redirects</label>
                <input
                  type="number" min="0" max="50"
                  :placeholder="`Inherit (${appSettings.defaultMaxRedirects})`"
                  :value="active.settings.maxRedirects ?? ''"
                  @input="onSetMaxRedirects(($event.target as HTMLInputElement).value)"
                />

                <label>Verify SSL</label>
                <select
                  :value="boolToTri(active.settings.verifySSL)"
                  @change="onSetVerifySSL(($event.target as HTMLSelectElement).value)"
                >
                  <option value="inherit">Inherit ({{ appSettings.defaultVerifySSL ? 'On' : 'Off' }})</option>
                  <option value="true">On</option>
                  <option value="false">Off</option>
                </select>
              </div>
              <p class="hint">Per-request values override the global defaults from the Settings sidebar.</p>

              <div class="gql-label" style="margin-top: 16px">Description</div>
              <textarea v-model="active.description" class="body-area" spellcheck="false" placeholder="Notes / documentation for this request…" />
            </div>

            <div v-else class="soon"><span>{{ REQ_TABS.find(t => t.id === active!.reqSubTab)?.label }} — coming in a later phase</span></div>
          </div>
        </section>

        <!-- response -->
        <section class="res">
          <div v-if="active.sendError" class="res-msg err selectable">{{ active.sendError }}</div>

          <template v-else-if="active.response">
            <div class="res-bar">
              <span class="status" :style="{ color: statusColor }">
                <span class="dot" :style="{ background: statusColor }"></span>
                {{ active.response.status }} {{ active.response.statusText }}
              </span>
              <span class="meta">{{ fmtMs(active.response.timing.totalMs) }}</span>
              <span class="meta">{{ fmtSize(active.response.sizeBytes) }}</span>
              <div class="res-subtabs">
                <button
                  v-for="st in RES_TABS" :key="st.id"
                  :class="{ active: active.resSubTab === st.id }"
                  @click="active.resSubTab = st.id"
                >
                  {{ st.label }}
                  <span v-if="st.id === 'testResults' && active.tests.length" class="count" :class="{ allpass: passCount === active.tests.length }">
                    {{ passCount }}/{{ active.tests.length }}
                  </span>
                </button>
              </div>
            </div>

            <pre v-if="active.resSubTab === 'body'" class="res-body selectable">{{ prettyBody }}</pre>
            <div v-else-if="active.resSubTab === 'headers'" class="res-headers selectable">
              <div v-for="(h, i) in active.response.headers" :key="i" class="rh">
                <span class="rh-k">{{ h.key }}</span><span class="rh-v">{{ h.value }}</span>
              </div>
            </div>
            <div v-else-if="active.resSubTab === 'cookies'" class="cookies selectable">
              <div class="cookies-head">
                <span>{{ cookies.length }} cookie(s) for this host</span>
                <button v-if="cookies.length" class="clear-ck" @click="clearCookies">Clear all</button>
              </div>
              <div v-if="!cookies.length" class="soon"><span>No cookies stored for this URL</span></div>
              <div v-for="(c, i) in cookies" :key="i" class="rh">
                <span class="rh-k">{{ c.name }}</span><span class="rh-v">{{ c.value }}</span>
              </div>
            </div>
            <div v-else-if="active.resSubTab === 'testResults'" class="tests selectable">
              <div v-if="!active.tests.length" class="soon"><span>No tests — add assertions in the Tests tab</span></div>
              <template v-else>
                <div v-for="(t, i) in active.tests" :key="i" class="test-row" :class="{ fail: !t.passed }">
                  <span class="test-badge">{{ t.passed ? 'PASS' : 'FAIL' }}</span>
                  <span class="test-name">{{ t.name }}<span v-if="t.error" class="test-err"> — {{ t.error }}</span></span>
                </div>
                <div v-if="active.logs.length" class="logs">
                  <div class="logs-head">Console</div>
                  <div v-for="(l, i) in active.logs" :key="i" class="log-line">{{ l }}</div>
                </div>
              </template>
            </div>
            <div v-else class="soon"><span>{{ RES_TABS.find(t => t.id === active!.resSubTab)?.label }} — coming in a later phase</span></div>
          </template>

          <div v-else class="res-msg muted">Send the request to see the response</div>
        </section>
      </div>
    </template>
  </div>
</template>

<style scoped>
.workbench { flex: 1; display: flex; flex-direction: column; overflow: hidden; background: var(--bg); min-width: 0; }
.empty { display: flex; flex-direction: column; gap: 10px; align-items: center; justify-content: center; height: 100%; color: var(--text-faint); }
.empty-art { font-size: 40px; opacity: 0.4; }

.title-row { padding: 10px 16px 0; background: var(--bg-elevated); }
.title-input { width: 100%; background: transparent; border: 1px solid transparent; border-radius: 5px; color: var(--text); font-size: 14px; font-weight: 600; padding: 4px 6px; }
.title-input:hover { border-color: var(--border); }
.title-input:focus { outline: none; border-color: var(--accent); background: var(--bg-input); }
.settings { display: flex; flex-direction: column; gap: 6px; height: 100%; }
.set-grid {
  display: grid;
  grid-template-columns: 160px 1fr;
  gap: 8px 12px;
  align-items: center;
  max-width: 460px;
}
.set-grid label { font-size: 12px; color: var(--text-dim); }
.set-grid input, .set-grid select {
  background: var(--bg-input);
  border: 1px solid var(--border);
  border-radius: 4px;
  color: var(--text);
  font: 12px monospace;
  padding: 5px 8px;
}
.set-grid input:focus, .set-grid select:focus { outline: none; border-color: var(--accent); }
.url-bar { display: flex; gap: 8px; padding: 10px 16px 12px; border-bottom: 1px solid var(--border); background: var(--bg-elevated); flex-shrink: 0; }
.method { background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; font: 700 12px monospace; padding: 0 8px; }
.url { flex: 1; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; font: 13px monospace; padding: 8px 10px; }
.url:focus, .method:focus { outline: none; border-color: var(--accent); }
.send { background: var(--accent); color: var(--accent-text); border-radius: 5px; font-weight: 700; padding: 0 22px; }
.cancel { background: var(--danger); color: #fff; border-radius: 5px; font-weight: 700; padding: 0 18px; }
.save { background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; color: var(--text-dim); padding: 0 14px; }
.save:disabled { opacity: 0.6; cursor: default; }

.split { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.req { display: flex; flex-direction: column; flex-shrink: 0; max-height: 44%; }
.res { flex: 1; display: flex; flex-direction: column; overflow: hidden; border-top: 1px solid var(--border); }

.subtabs, .res-subtabs { display: flex; gap: 2px; }
.subtabs { padding: 6px 12px 0; border-bottom: 1px solid var(--border); overflow-x: auto; flex-shrink: 0; }
.subtabs button, .res-subtabs button { color: var(--text-dim); font-size: 12px; padding: 6px 10px; border-radius: 5px 5px 0 0; white-space: nowrap; display: flex; align-items: center; gap: 4px; }
.subtabs button.active, .res-subtabs button.active { color: var(--text); border-bottom: 2px solid var(--accent); }
.count { background: var(--border-strong); border-radius: 8px; color: var(--text); font-size: 10px; padding: 0 5px; }
.dot-on { width: 6px; height: 6px; border-radius: 50%; background: var(--ok); }

.subpanel { padding: 12px 16px; overflow-y: auto; flex: 1; }
.kv { display: flex; flex-direction: column; gap: 6px; }
.kv-row { display: flex; align-items: center; gap: 6px; }
.kv-key, .kv-val { background: var(--bg-input); border: 1px solid var(--border); border-radius: 4px; font: 12px monospace; padding: 5px 8px; }
.kv-key { width: 34%; } .kv-val { flex: 1; }
.kv-key:focus, .kv-val:focus, .body-area:focus, .auth-in:focus { outline: none; border-color: var(--accent); }
.kv-del { color: var(--danger); font-size: 11px; padding: 0 4px; }
.add { align-self: flex-start; border: 1px dashed var(--border-strong); border-radius: 4px; color: var(--text-dim); font-size: 12px; padding: 5px 10px; margin-top: 4px; }
.body-area { width: 100%; min-height: 120px; background: var(--bg-input); border: 1px solid var(--border); border-radius: 4px; color: var(--text); font: 12px/1.5 monospace; padding: 10px; resize: vertical; }
.body { display: flex; flex-direction: column; gap: 10px; height: 100%; }
.body-type select { background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; font-size: 12px; padding: 5px 8px; }
.f-type { background: var(--bg-input); border: 1px solid var(--border); border-radius: 4px; font-size: 11px; padding: 5px; }

.auth { display: flex; flex-direction: column; gap: 10px; max-width: 460px; }
.auth-type { display: flex; align-items: center; gap: 10px; font-size: 12px; color: var(--text-dim); }
.auth-type select { flex: 1; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; padding: 6px 8px; }
.auth-in { background: var(--bg-input); border: 1px solid var(--border); border-radius: 4px; font: 12px monospace; padding: 7px 9px; }
.hint { font-size: 11px; color: var(--text-faint); }

.soon { display: flex; align-items: center; justify-content: center; height: 100%; min-height: 80px; color: var(--text-faint); font-size: 12px; }

.res-bar { display: flex; align-items: center; gap: 14px; padding: 9px 16px; background: var(--bg-elevated); border-bottom: 1px solid var(--border); flex-shrink: 0; }
.status { display: flex; align-items: center; gap: 7px; font: 700 13px monospace; }
.dot { width: 7px; height: 7px; border-radius: 50%; }
.meta { color: var(--text-dim); font-size: 12px; }
.res-subtabs { margin-left: auto; }
.res-body { flex: 1; overflow: auto; padding: 12px 16px; color: var(--text); font: 12px/1.5 monospace; white-space: pre-wrap; word-break: break-word; }
.res-headers { flex: 1; overflow: auto; padding: 8px 16px; }
.rh { display: flex; gap: 12px; padding: 4px 0; border-bottom: 1px solid var(--border); font: 12px monospace; }
.rh-k { color: #61affe; min-width: 220px; word-break: break-all; }
.rh-v { color: var(--text); word-break: break-all; }
.script { font-size: 12px; }
.gql { display: flex; flex-direction: column; gap: 6px; height: 100%; }
.gql-label { font-size: 10px; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-faint); }
.gql-vars { min-height: 60px; max-height: 100px; }
.count.allpass { background: var(--ok); color: #06140d; }
.cookies { flex: 1; overflow: auto; padding: 8px 16px; }
.cookies-head { display: flex; align-items: center; justify-content: space-between; font-size: 11px; color: var(--text-faint); padding-bottom: 6px; }
.clear-ck { color: var(--danger); font-size: 11px; padding: 3px 8px; border-radius: 4px; }
.clear-ck:hover { background: var(--bg-hover); }
.tests { flex: 1; overflow: auto; padding: 10px 16px; }
.test-row { display: flex; align-items: center; gap: 10px; padding: 6px 0; border-bottom: 1px solid var(--border); font-size: 12px; }
.test-badge { font: 700 9px monospace; padding: 2px 6px; border-radius: 4px; background: var(--ok); color: #06140d; flex-shrink: 0; }
.test-row.fail .test-badge { background: var(--danger); color: #fff; }
.test-name { color: var(--text); }
.test-err { color: var(--danger); }
.logs { margin-top: 12px; }
.logs-head { font-size: 10px; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-faint); margin-bottom: 4px; }
.log-line { font: 11px/1.5 monospace; color: var(--text-dim); white-space: pre-wrap; word-break: break-word; }

.res-msg { display: flex; align-items: center; justify-content: center; flex: 1; font-size: 13px; padding: 16px; text-align: center; }
.res-msg.muted { color: var(--text-faint); }
.res-msg.err { color: var(--danger); }
</style>
