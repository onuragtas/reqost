<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { SendRequest, Cancel, GetCookies, ClearCookies } from '../../bindings/reqost/execservice'
import { SaveRequest } from '../../bindings/reqost/collectionservice'
import { useTabs, toDetail, markClean, isDirty, type ReqSubTab, type ResSubTab, type AuthType, type BodyType } from '../composables/useTabs'
import { useEnv } from '../composables/useEnv'
import { useHistory } from '../composables/useHistory'
import { useTree } from '../composables/useTree'
import { useSettings } from '../composables/useSettings'
import { resolveAncestorContext } from '../composables/useFolderContext'
import { useDialog } from '../composables/useDialog'
import { parseQuery, buildUrl, baseOf } from '../composables/url'
import { parseCurl, toCurl } from '../composables/curl'
import { recordReqHistory, loadReqHistory, type ReqHistoryEntry } from '../composables/useRequestHistory'
import {
  generatePython, generateJS, generateGo, generateJava, generateCSharp, generatePowerShell, generateHTTP,
  CODE_LANGS, type CodeLang,
} from '../composables/useCodeGen'
import WsConsole from './WsConsole.vue'
import GrpcConsole from './GrpcConsole.vue'
import SseConsole from './SseConsole.vue'
import JsonTree from './JsonTree.vue'
import { useVarHint } from '../composables/useVarHint'

const { active, closeTab } = useTabs()
const dialog = useDialog()
const { activeVars, applyVars } = useEnv()
const { varHint, showVarHint, hideVarHint } = useVarHint()
const varFmt = (n: string) => '{{' + n + '}}'
const { record } = useHistory()
const { refreshNode } = useTree()
const { settings: appSettings } = useSettings()

// Switch protocol UI by URL scheme: ws/wss → WebSocket, grpc → gRPC, else HTTP.
const mode = computed<'http' | 'ws' | 'grpc' | 'sse'>(() => {
  const u = active.value?.url?.trim().toLowerCase() ?? ''
  if (u.startsWith('ws://') || u.startsWith('wss://')) return 'ws'
  if (u.startsWith('grpc://') || u.startsWith('grpcs://')) return 'grpc'
  if (u.startsWith('sse://') || u.startsWith('sses://')) return 'sse'
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

// ── cURL paste ────────────────────────────────────────────────────────────────
function onUrlPaste(e: ClipboardEvent) {
  const text = e.clipboardData?.getData('text') ?? ''
  if (!text.trim().startsWith('curl ')) return
  const parsed = parseCurl(text)
  if (!parsed || !active.value) return
  e.preventDefault()
  const t = active.value
  t.method = parsed.method
  t.url = parsed.url
  t.params = parseQuery(parsed.url)
  t.headers = parsed.headers
  t.body = parsed.body
  t.bodyType = parsed.bodyType
  if (parsed.formFields.length) t.formFields = parsed.formFields
  // Switch to Body tab so user sees the pasted data
  if (parsed.body) t.reqSubTab = 'body'
}

// ── Keyboard shortcuts ─────────────────────────────────────────────────────
async function maybeCloseActive() {
  const t = active.value
  if (!t) return
  if (isDirty(t)) {
    const ok = await dialog.confirm(`Close "${t.name}"? Unsaved changes will be lost.`)
    if (!ok) return
  }
  closeTab(t.id)
}
function onKeyDown(e: KeyboardEvent) {
  if (!e.metaKey && !e.ctrlKey) return
  if (e.key === 'Enter') { e.preventDefault(); send() }
  else if (e.key === 's') { e.preventDefault(); save() }
  else if (e.key === 'w') { e.preventDefault(); maybeCloseActive() }
}
onMounted(() => window.addEventListener('keydown', onKeyDown))
onUnmounted(() => window.removeEventListener('keydown', onKeyDown))

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
  { id: 'oauth2', label: 'OAuth 2.0' },
]

const REQ_TABS: { id: ReqSubTab; label: string; soon?: boolean }[] = [
  { id: 'params', label: 'Params' },
  { id: 'auth', label: 'Auth' },
  { id: 'headers', label: 'Headers' },
  { id: 'body', label: 'Body' },
  { id: 'prereq', label: 'Pre-req' },
  { id: 'tests', label: 'Tests' },
  { id: 'examples', label: 'Examples' },
  { id: 'settings', label: 'Settings' },
]
const RES_TABS: { id: ResSubTab; label: string; soon?: boolean }[] = [
  { id: 'body', label: 'Body' },
  { id: 'headers', label: 'Headers' },
  { id: 'cookies', label: 'Cookies' },
  { id: 'testResults', label: 'Test Results' },
  { id: 'history', label: 'History' },
]

// ── Per-request response history ──────────────────────────────────────────
const reqHistory = ref<ReqHistoryEntry[]>([])
const selHistIdx = ref(-1)
watch(() => active.value?.id, (id) => {
  reqHistory.value = id ? loadReqHistory(id) : []
  selHistIdx.value = -1
}, { immediate: true })
const prettyHistBody = computed(() => {
  const e = reqHistory.value[selHistIdx.value]
  if (!e) return ''
  try { return JSON.stringify(JSON.parse(e.body), null, 2) } catch { return e.body }
})
function histStatusColor(s: number) {
  if (s >= 200 && s < 300) return 'var(--ok)'
  if (s >= 300 && s < 400) return 'var(--warn-text)'
  if (s >= 400) return 'var(--danger)'
  return 'var(--text-dim)'
}
function fmtTs(ts: number) {
  const d = new Date(ts)
  return `${d.getHours().toString().padStart(2,'0')}:${d.getMinutes().toString().padStart(2,'0')}:${d.getSeconds().toString().padStart(2,'0')}`
}

// ── Code generation ────────────────────────────────────────────────────────
const showCode = ref(false)
const codeLang = ref<CodeLang>('python')
const generatedCode = computed(() => {
  const t = active.value
  if (!t) return ''
  const input = { method: t.method, url: t.url, headers: t.headers, body: t.body, bodyType: t.bodyType, auth: t.auth }
  switch (codeLang.value) {
    case 'curl':       return toCurl(t.method, t.url, t.headers, t.body)
    case 'python':     return generatePython(input)
    case 'javascript': return generateJS(input)
    case 'go':         return generateGo(input)
    case 'java':       return generateJava(input)
    case 'csharp':     return generateCSharp(input)
    case 'powershell': return generatePowerShell(input)
    case 'http':       return generateHTTP(input)
  }
  return ''
})
async function copyCode() {
  try { await navigator.clipboard.writeText(generatedCode.value) } catch { /* ignore */ }
}

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

// ── Bulk headers edit (Postman-style) ─────────────────────────────────────
const bulkHeaders = ref(false)
const bulkHeadersText = ref('')

function headersToBulk(): string {
  return (active.value?.headers ?? [])
    .filter(h => h.key.trim())
    .map(h => `${h.enabled ? '' : '#'}${h.key}: ${h.value}`)
    .join('\n')
}
function bulkToHeaders(text: string): { key: string; value: string; enabled: boolean }[] {
  const out: { key: string; value: string; enabled: boolean }[] = []
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trimEnd()
    if (!line.trim()) continue
    let enabled = true
    let s = line
    if (s.startsWith('#')) { enabled = false; s = s.slice(1) }
    const colon = s.indexOf(':')
    if (colon < 1) continue
    out.push({ key: s.slice(0, colon).trim(), value: s.slice(colon + 1).trim(), enabled })
  }
  return out
}
function setBulkHeaders(on: boolean) {
  if (on && !bulkHeaders.value) {
    bulkHeadersText.value = headersToBulk()
  } else if (!on && bulkHeaders.value) {
    commitBulkHeaders()
  }
  bulkHeaders.value = on
}
function commitBulkHeaders() {
  if (!active.value) return
  active.value.headers = bulkToHeaders(bulkHeadersText.value)
}

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

  // Merge folder-level inherited headers + auth. Child overrides: the
  // request's own headers come AFTER inherited ones, so duplicates favour
  // the request; auth falls back to inherited only when the request has none.
  const ancestor = await resolveAncestorContext(t.id)
  const inheritedHeaders = ancestor.headers.filter(h => h.enabled !== false && h.key.trim())
  const ownHeaderKeys = new Set(t.headers.filter(h => h.key.trim()).map(h => h.key.toLowerCase()))
  const mergedHeaders = [
    ...inheritedHeaders.filter(h => !ownHeaderKeys.has(h.key.toLowerCase())),
    ...t.headers.filter(h => h.key.trim()),
  ]
  // OAuth 2 was already resolved to a bearer access token in the Auth tab —
  // the transport layer only understands bearer/basic/apikey, so map it.
  const ownAuth = t.auth.type === 'oauth2' && t.auth.token
    ? { ...t.auth, type: 'bearer' as const }
    : (t.auth.type === 'none' ? null : { ...t.auth })
  const mergedAuth = (!ownAuth && ancestor.auth) ? ancestor.auth : ownAuth

  // Resolve per-request settings → falling back to app-wide defaults.
  const s = t.settings
  const timeoutMs        = s.timeoutMs        ?? appSettings.defaultTimeoutMs
  const followRedirects  = s.followRedirects  ?? appSettings.defaultFollowRedirects
  const maxRedirects     = s.maxRedirects     ?? appSettings.defaultMaxRedirects
  const verifySSL        = s.verifySSL        ?? appSettings.defaultVerifySSL

  try {
    const res: any = await SendRequest(t.id, t.name, {
      protocol: 'http',
      method: t.method,
      url: t.url.trim(),
      headers: mergedHeaders,
      body,
      bodyType,
      formFields: t.formFields.filter(f => f.key.trim()),
      auth: mergedAuth,
      variables: activeVars.value,
      timeoutMs,
      disableRedirect: !followRedirects,
      maxRedirects,
      insecureSkipVerify: !verifySSL,
      proxyURL: appSettings.proxyURL,
      clientCerts: appSettings.clientCerts.filter(c => c.hostPattern && c.certPath && c.keyPath),
    }, t.preScript, t.postScript)

    const resp = res?.response
    t.response = resp
    t.tests = res?.tests ?? []
    t.logs = res?.logs ?? []
    if (res?.scriptError) t.logs = [...t.logs, `⚠ ${res.scriptError}`]
    if (res?.vars) applyVars(res.vars)
    t.resSubTab = t.tests.length ? 'testResults' : 'body'
    if (resp && t.id) {
      recordReqHistory(t.id, resp.status, resp.timing?.totalMs ?? 0, resp.body ?? '', resp.headers ?? [])
      reqHistory.value = loadReqHistory(t.id)
      selHistIdx.value = -1
    }
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

// ── Response body view mode (Pretty / Raw / Tree) ──────────────────────────
type BodyView = 'pretty' | 'raw' | 'tree'
const bodyView = ref<BodyView>('pretty')
const bodyJSON = computed(() => {
  const b = active.value?.response?.body ?? ''
  try { return JSON.parse(b) } catch { return null }
})
const canTree = computed(() => bodyJSON.value !== null)

function fmtSize(n: number) {
  if (n < 1024) return `${n} B`
  if (n < 1048576) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1048576).toFixed(2)} MB`
}
function fmtMs(n: number) { return n >= 1000 ? `${(n / 1000).toFixed(2)} s` : `${Math.round(n)} ms` }

// ── OAuth 2.0 ─────────────────────────────────────────────────────────────
const oauthGetting = ref(false)
const oauthError = ref<string>('')

const oauth2Cfg = computed(() => {
  const t = active.value
  if (!t) return { grant: 'authorization_code' as const, tokenUrl: '', clientId: '' }
  if (!t.auth.oauth2) {
    t.auth.oauth2 = { grant: 'authorization_code', tokenUrl: '', clientId: '', scope: '', usePkce: true }
  }
  return t.auth.oauth2
})

async function getOAuthToken() {
  const t = active.value
  if (!t || !t.auth.oauth2) return
  oauthGetting.value = true
  oauthError.value = ''
  try {
    const { GetToken } = await import('../../bindings/reqost/oauthservice')
    const tok: any = await GetToken(t.auth.oauth2 as any)
    if (tok?.accessToken) {
      t.auth.token = tok.accessToken
    } else {
      oauthError.value = 'No access_token in response'
    }
  } catch (e: any) {
    oauthError.value = e?.message ?? String(e)
  } finally {
    oauthGetting.value = false
  }
}

// ── GraphQL schema introspection ──────────────────────────────────────────
const gqlSchema = ref<{ queryType?: string; types: any[] } | null>(null)
const gqlLoadingSchema = ref(false)
const gqlSchemaError = ref<string>('')

async function loadGqlSchema() {
  const t = active.value
  if (!t || !t.url.trim()) return
  gqlLoadingSchema.value = true
  gqlSchemaError.value = ''
  const introspection = `query IntrospectionQuery {
    __schema {
      queryType { name }
      mutationType { name }
      types {
        kind
        name
        fields { name args { name } type { name kind } }
      }
    }
  }`
  try {
    const res: any = await SendRequest(t.id, t.name + ' (schema)', {
      protocol: 'http', method: 'POST', url: t.url.trim(),
      headers: [{ key: 'Content-Type', value: 'application/json', enabled: true }],
      body: JSON.stringify({ query: introspection }),
      bodyType: 'json', formFields: [], auth: t.auth.type === 'none' ? null : { ...t.auth },
      variables: activeVars.value,
      timeoutMs: 30000, disableRedirect: false, maxRedirects: 10, insecureSkipVerify: false,
      proxyURL: appSettings.proxyURL, clientCerts: [],
    }, '', '')
    const body = res?.response?.body
    const data = JSON.parse(body || '{}')?.data?.__schema
    if (!data) {
      gqlSchemaError.value = 'No __schema in response'
    } else {
      gqlSchema.value = {
        queryType: data.queryType?.name,
        types: (data.types ?? []).filter((t: any) => !t.name?.startsWith('__')),
      }
    }
  } catch (e: any) {
    gqlSchemaError.value = e?.message ?? String(e)
  } finally {
    gqlLoadingSchema.value = false
  }
}

// ── Saved Examples ────────────────────────────────────────────────────────
async function saveAsExample() {
  const t = active.value
  if (!t || !t.response) return
  const defaultName = `${t.response.status} ${t.response.statusText || 'response'} — ${new Date().toLocaleString()}`
  const name = await dialog.prompt('Save as example — name?', defaultName)
  if (!name?.trim()) return
  const id = (typeof crypto !== 'undefined' && crypto.randomUUID) ? crypto.randomUUID() : `ex-${Date.now()}`
  t.examples.push({
    id,
    name: name.trim(),
    savedAt: Date.now(),
    request: {
      method: t.method,
      url: t.url,
      headers: t.headers.map(h => ({ ...h })),
      body: t.body,
      bodyType: t.bodyType,
    },
    response: {
      status: t.response.status,
      statusText: t.response.statusText,
      headers: (t.response.headers ?? []).map((h: any) => ({ key: h.key, value: h.value, enabled: true })),
      body: t.response.body ?? '',
      sizeBytes: t.response.sizeBytes ?? 0,
    },
  })
  try {
    await SaveRequest(toDetail(t) as any)
    markClean(t)
  } catch (e) { /* keep in memory; user can hit Save again */ }
}

function deleteExample(id: string) {
  const t = active.value
  if (!t) return
  t.examples = t.examples.filter(e => e.id !== id)
  // Persist on next Save — keeps the action async-light.
}

function loadExample(id: string) {
  const t = active.value
  if (!t) return
  const ex = t.examples.find(e => e.id === id)
  if (!ex) return
  t.method     = ex.request.method
  t.url        = ex.request.url
  t.params     = parseQuery(t.url)
  t.headers    = ex.request.headers.map(h => ({ ...h }))
  t.body       = ex.request.body
  t.bodyType   = ex.request.bodyType
  t.response   = {
    status: ex.response.status,
    statusText: ex.response.statusText,
    headers: ex.response.headers,
    body: ex.response.body,
    sizeBytes: ex.response.sizeBytes,
    timing: { dnsMs: 0, connectMs: 0, tlsMs: 0, ttfbMs: 0, totalMs: 0 },
  }
  t.resSubTab  = 'body'
}

function fmtExampleTime(ms: number): string {
  return new Date(ms).toLocaleString()
}

// ── Timing waterfall (DNS / Connect / TLS / TTFB / Download) ──────────────
interface Timing {
  dnsMs: number; connectMs: number; tlsMs: number; ttfbMs: number; totalMs: number
}
interface WaterSegment { label: string; ms: number; x: number; w: number; color: string }

function waterfallSegments(t: Timing): WaterSegment[] {
  if (!t || !t.totalMs) return []
  // TTFB already includes DNS + Connect + TLS in httpclient's accounting (it
  // is measured from start → first byte). So "wait" = ttfb - dns - connect - tls.
  const dns = Math.max(0, t.dnsMs)
  const conn = Math.max(0, t.connectMs)
  const tls = Math.max(0, t.tlsMs)
  const wait = Math.max(0, t.ttfbMs - dns - conn - tls)
  const dl = Math.max(0, t.totalMs - t.ttfbMs)
  const total = dns + conn + tls + wait + dl || t.totalMs
  const segs: { label: string; ms: number; color: string }[] = [
    { label: 'DNS',      ms: dns,  color: '#61affe' },
    { label: 'Connect',  ms: conn, color: '#49cc90' },
    { label: 'TLS',      ms: tls,  color: '#fca130' },
    { label: 'Wait',     ms: wait, color: '#50e3c2' },
    { label: 'Download', ms: dl,   color: '#a78bfa' },
  ]
  let x = 0
  const out: WaterSegment[] = []
  for (const s of segs) {
    const w = (s.ms / total) * 100
    if (w <= 0) continue
    out.push({ ...s, x, w })
    x += w
  }
  return out
}

function timingTooltip(t: Timing): string {
  return [
    `DNS:      ${fmtMs(t.dnsMs)}`,
    `Connect:  ${fmtMs(t.connectMs)}`,
    `TLS:      ${fmtMs(t.tlsMs)}`,
    `TTFB:     ${fmtMs(t.ttfbMs)}`,
    `Total:    ${fmtMs(t.totalMs)}`,
  ].join('\n')
}

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
    <SseConsole v-else-if="mode === 'sse'" :tab="active" />

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
        <input v-model="active.url" class="url" :placeholder="URL_PLACEHOLDER" @input="onUrlInput" @keyup.enter="send" @paste="onUrlPaste" @mouseenter="showVarHint($event, active.url)" @mouseleave="hideVarHint" />
        <button v-if="!active.sending" class="send" @click="send">Send</button>
        <button v-else class="cancel" @click="cancel">Cancel</button>
        <button class="save" :disabled="saving" @click="save">{{ savedFlash ? 'Saved ✓' : 'Save' }}</button>
        <button class="code-btn" :class="{ active: showCode }" title="Generate code" @click="showCode = !showCode">&lt;/&gt;</button>
      </div>

      <!-- Code generation panel -->
      <div v-if="showCode" class="code-panel">
        <div class="code-header">
          <select v-model="codeLang" class="lang-select">
            <option v-for="l in CODE_LANGS" :key="l.id" :value="l.id">{{ l.label }}</option>
          </select>
          <button class="copy-code" @click="copyCode">Copy</button>
          <button class="code-close" @click="showCode = false">✕</button>
        </div>
        <pre class="code-body selectable">{{ generatedCode }}</pre>
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
                <input v-model="p.key" placeholder="Key" class="kv-key" @input="syncUrl" @mouseenter="showVarHint($event, p.key)" @mouseleave="hideVarHint" />
                <input v-model="p.value" placeholder="Value" class="kv-val" @input="syncUrl" @mouseenter="showVarHint($event, p.value)" @mouseleave="hideVarHint" />
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
                <input v-model="active.auth.token" class="auth-in" placeholder="Token" @mouseenter="showVarHint($event, active.auth.token)" @mouseleave="hideVarHint" />
              </template>
              <template v-else-if="active.auth.type === 'basic'">
                <input v-model="active.auth.username" class="auth-in" placeholder="Username" @mouseenter="showVarHint($event, active.auth.username)" @mouseleave="hideVarHint" />
                <input v-model="active.auth.password" class="auth-in" placeholder="Password" @mouseenter="showVarHint($event, active.auth.password)" @mouseleave="hideVarHint" />
              </template>
              <template v-else-if="active.auth.type === 'apikey'">
                <input v-model="active.auth.key" class="auth-in" placeholder="Header name (e.g. X-API-Key)" @mouseenter="showVarHint($event, active.auth.key)" @mouseleave="hideVarHint" />
                <input v-model="active.auth.value" class="auth-in" placeholder="Value" @mouseenter="showVarHint($event, active.auth.value)" @mouseleave="hideVarHint" />
              </template>
              <template v-else-if="active.auth.type === 'oauth2'">
                <div class="oauth-grid">
                  <label>Grant</label>
                  <select v-model="oauth2Cfg.grant" class="auth-in">
                    <option value="authorization_code">Authorization Code + PKCE</option>
                    <option value="client_credentials">Client Credentials</option>
                    <option value="password">Password (legacy)</option>
                  </select>

                  <label v-if="oauth2Cfg.grant === 'authorization_code'">Authorize URL</label>
                  <input v-if="oauth2Cfg.grant === 'authorization_code'" v-model="oauth2Cfg.authUrl" class="auth-in" placeholder="https://issuer/authorize" />

                  <label>Token URL</label>
                  <input v-model="oauth2Cfg.tokenUrl" class="auth-in" placeholder="https://issuer/oauth/token" />

                  <label>Client ID</label>
                  <input v-model="oauth2Cfg.clientId" class="auth-in" placeholder="client id" />

                  <label>Client secret</label>
                  <input v-model="oauth2Cfg.clientSecret" type="password" class="auth-in" placeholder="(optional for public clients)" />

                  <template v-if="oauth2Cfg.grant === 'password'">
                    <label>Username</label>
                    <input v-model="oauth2Cfg.username" class="auth-in" />
                    <label>Password</label>
                    <input v-model="oauth2Cfg.password" type="password" class="auth-in" />
                  </template>

                  <label>Scope</label>
                  <input v-model="oauth2Cfg.scope" class="auth-in" placeholder="openid profile email …" />

                  <label>Audience</label>
                  <input v-model="oauth2Cfg.audience" class="auth-in" placeholder="(Auth0-style; optional)" />
                </div>
                <div class="oauth-actions">
                  <button class="oauth-get" :disabled="oauthGetting" @click="getOAuthToken">
                    {{ oauthGetting ? 'Getting…' : 'Get token' }}
                  </button>
                  <span v-if="active.auth.token" class="oauth-have">✓ Token cached</span>
                  <span v-if="oauthError" class="err">⚠ {{ oauthError }}</span>
                </div>
              </template>
              <p v-if="active.auth.type !== 'none' && active.auth.type !== 'oauth2'" class="hint">{{ VAR_HINT }}</p>
            </div>

            <!-- Headers -->
            <div v-else-if="active.reqSubTab === 'headers'" class="kv">
              <div class="kv-bar">
                <button
                  class="kv-mode" :class="{ active: !bulkHeaders }"
                  @click="setBulkHeaders(false)"
                >Key-Value</button>
                <button
                  class="kv-mode" :class="{ active: bulkHeaders }"
                  @click="setBulkHeaders(true)"
                >Bulk Edit</button>
              </div>

              <template v-if="!bulkHeaders">
                <div v-for="(h, i) in active.headers" :key="i" class="kv-row">
                  <input type="checkbox" v-model="h.enabled" />
                  <input v-model="h.key" placeholder="Key" class="kv-key" @mouseenter="showVarHint($event, h.key)" @mouseleave="hideVarHint" />
                  <input v-model="h.value" placeholder="Value" class="kv-val" @mouseenter="showVarHint($event, h.value)" @mouseleave="hideVarHint" />
                  <button class="kv-del" @click="removeHeader(i)">✕</button>
                </div>
                <button class="add" @click="addHeader">+ Add header</button>
              </template>

              <textarea
                v-else
                v-model="bulkHeadersText"
                class="body-area"
                placeholder="Authorization: Bearer abc&#10;Content-Type: application/json&#10;X-Disabled: paste; lines starting with # are disabled"
                spellcheck="false"
                @blur="commitBulkHeaders"
              />
              <p v-if="bulkHeaders" class="hint">One header per line: <code>Key: Value</code>. Leading <code>#</code> disables the row. Click Key-Value to apply.</p>
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
                @mouseenter="showVarHint($event, active.body)" @mouseleave="hideVarHint"
              />
              <div v-else-if="active.bodyType === 'graphql'" class="gql">
                <div class="gql-toolbar">
                  <span class="gql-label">Query</span>
                  <button class="gql-schema" :disabled="gqlLoadingSchema" @click="loadGqlSchema">
                    {{ gqlLoadingSchema ? 'Loading…' : (gqlSchema ? 'Reload schema' : 'Load schema') }}
                  </button>
                </div>
                <textarea v-model="active.body" class="body-area script" spellcheck="false" placeholder="query { ... }" />
                <div class="gql-label">Variables (JSON)</div>
                <textarea v-model="active.graphqlVars" class="body-area script gql-vars" spellcheck="false" placeholder="{ }" />
                <div v-if="gqlSchemaError" class="gql-err">⚠ {{ gqlSchemaError }}</div>
                <div v-if="gqlSchema" class="gql-types selectable">
                  <div class="gql-label">Types ({{ gqlSchema.types.length }})</div>
                  <div v-for="t in gqlSchema.types" :key="t.name" class="gql-type">
                    <span class="gql-kind">{{ t.kind.toLowerCase() }}</span>
                    <span class="gql-name">{{ t.name }}</span>
                    <span v-if="t.fields?.length" class="gql-fields">
                      {{ t.fields.slice(0, 8).map((f: any) => f.name).join(', ') }}{{ t.fields.length > 8 ? `, …+${t.fields.length - 8}` : '' }}
                    </span>
                  </div>
                </div>
              </div>
              <div v-else class="kv">
                <div v-for="(f, i) in active.formFields" :key="i" class="kv-row">
                  <input type="checkbox" v-model="f.enabled" />
                  <input v-model="f.key" placeholder="Key" class="kv-key" @mouseenter="showVarHint($event, f.key)" @mouseleave="hideVarHint" />
                  <select v-if="active.bodyType === 'formdata'" v-model="f.type" class="f-type">
                    <option value="text">Text</option>
                    <option value="file">File</option>
                  </select>
                  <input v-model="f.value" :placeholder="f.type === 'file' ? '/path/to/file' : 'Value'" class="kv-val" @mouseenter="showVarHint($event, f.value)" @mouseleave="hideVarHint" />
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

            <!-- Examples -->
            <div v-else-if="active.reqSubTab === 'examples'" class="examples-pane">
              <div v-if="!active.examples.length" class="soon">
                <span>No saved examples. After a Send, click <strong>Save as example</strong> on the response.</span>
              </div>
              <div v-else class="ex-list">
                <div v-for="e in [...active.examples].reverse()" :key="e.id" class="ex-row">
                  <button class="ex-load" :title="`Load — ${e.name}`" @click="loadExample(e.id)">
                    <span class="ex-method" :style="{ color: METHOD_COLORS[e.request.method] ?? 'var(--text-dim)' }">{{ e.request.method }}</span>
                    <span class="ex-name">{{ e.name }}</span>
                    <span class="ex-status" :class="{
                      ok: e.response.status >= 200 && e.response.status < 300,
                      warn: e.response.status >= 300 && e.response.status < 400,
                      err: e.response.status >= 400,
                    }">{{ e.response.status }}</span>
                  </button>
                  <span class="ex-time">{{ fmtExampleTime(e.savedAt) }}</span>
                  <button class="ex-del" title="Delete" @click="deleteExample(e.id)">✕</button>
                </div>
              </div>
            </div>

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
              <span class="meta timing-meta" :title="timingTooltip(active.response.timing)">
                {{ fmtMs(active.response.timing.totalMs) }}
                <svg class="waterfall" viewBox="0 0 100 8" preserveAspectRatio="none" aria-hidden="true">
                  <rect
                    v-for="seg in waterfallSegments(active.response.timing)"
                    :key="seg.label"
                    :x="seg.x" :y="0" :width="seg.w" :height="8"
                    :fill="seg.color"
                  />
                </svg>
              </span>
              <span class="meta">{{ fmtSize(active.response.sizeBytes) }}</span>
              <button class="save-ex" title="Save this response as an example" @click="saveAsExample">
                ★ Save as example
              </button>
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

            <template v-if="active.resSubTab === 'body'">
              <div class="body-view-bar">
                <button
                  class="bv-btn" :class="{ active: bodyView === 'pretty' }"
                  @click="bodyView = 'pretty'"
                >Pretty</button>
                <button
                  class="bv-btn" :class="{ active: bodyView === 'raw' }"
                  @click="bodyView = 'raw'"
                >Raw</button>
                <button
                  class="bv-btn" :class="{ active: bodyView === 'tree' }"
                  :disabled="!canTree"
                  :title="!canTree ? 'Tree view requires a JSON body' : ''"
                  @click="bodyView = 'tree'"
                >Tree</button>
              </div>
              <JsonTree v-if="bodyView === 'tree' && canTree" :value="bodyJSON" />
              <pre v-else-if="bodyView === 'pretty'" class="res-body selectable">{{ prettyBody }}</pre>
              <pre v-else class="res-body selectable">{{ active.response.body }}</pre>
            </template>
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
            <div v-else-if="active.resSubTab === 'history'" class="hist selectable">
              <div v-if="!reqHistory.length" class="soon"><span>No history yet — send a request to record it</span></div>
              <template v-else>
                <div class="hist-list">
                  <div
                    v-for="(e, i) in reqHistory" :key="e.ts"
                    class="hist-row" :class="{ sel: selHistIdx === i }"
                    @click="selHistIdx = i"
                  >
                    <span class="hist-badge" :style="{ color: histStatusColor(e.status) }">{{ e.status }}</span>
                    <span class="hist-time">{{ fmtTs(e.ts) }}</span>
                    <span class="hist-dur">{{ fmtMs(e.ms) }}</span>
                  </div>
                </div>
                <pre v-if="selHistIdx >= 0" class="hist-body">{{ prettyHistBody }}</pre>
              </template>
            </div>
            <div v-else class="soon"><span>{{ RES_TABS.find(t => t.id === active!.resSubTab)?.label }}</span></div>
          </template>

          <div v-else class="res-msg muted">Send the request to see the response</div>
        </section>
      </div>
    </template>
  </div>

  <Teleport to="body">
    <div v-if="varHint.visible" class="var-hint" :style="{ left: varHint.x + 'px', top: varHint.y + 'px' }">
      <div v-for="p in varHint.pairs" :key="p.name" class="var-hint-row">
        <span class="var-hint-name">{{ varFmt(p.name) }}</span>
        <span class="var-hint-sep">→</span>
        <span :class="p.found ? 'var-hint-val' : 'var-hint-unset'">{{ p.found ? (p.value || '(empty)') : 'not set' }}</span>
      </div>
    </div>
  </Teleport>
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
.code-btn { background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; color: var(--text-dim); padding: 0 10px; font-size: 13px; font-weight: 600; }
.code-btn:hover, .code-btn.active { color: var(--accent); border-color: var(--accent); }

.code-panel { background: var(--bg-panel); border-bottom: 1px solid var(--border); flex-shrink: 0; display: flex; flex-direction: column; max-height: 240px; }
.code-header { display: flex; gap: 8px; align-items: center; padding: 8px 12px; border-bottom: 1px solid var(--border); }
.lang-select { background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; color: var(--text); font-size: 12px; padding: 4px 8px; }
.lang-select:focus { outline: none; border-color: var(--accent); }
.copy-code { background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 5px; color: var(--text-dim); font-size: 12px; padding: 4px 12px; }
.copy-code:hover { color: var(--text); }
.code-close { margin-left: auto; color: var(--text-faint); font-size: 12px; }
.code-close:hover { color: var(--text); }
.code-body { flex: 1; overflow: auto; margin: 0; padding: 10px 14px; font: 12px/1.6 monospace; color: var(--text); background: transparent; white-space: pre; }

.hist { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.hist-list { flex-shrink: 0; max-height: 120px; overflow-y: auto; border-bottom: 1px solid var(--border); }
.hist-row { display: flex; gap: 12px; align-items: center; padding: 6px 14px; cursor: pointer; font-size: 12px; color: var(--text-dim); }
.hist-row:hover { background: var(--bg-hover); }
.hist-row.sel { background: color-mix(in srgb, var(--accent) 10%, transparent); }
.hist-badge { font-weight: 700; font-size: 11px; width: 36px; text-align: right; }
.hist-time { font-family: monospace; }
.hist-dur { color: var(--text-faint); margin-left: auto; }
.hist-body { flex: 1; overflow: auto; margin: 0; padding: 10px 14px; font: 12px/1.6 monospace; color: var(--text); }

.split { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.req { display: flex; flex-direction: column; flex-shrink: 0; max-height: 44%; }
.res { flex: 1; display: flex; flex-direction: column; overflow: hidden; border-top: 1px solid var(--border); }

.subtabs, .res-subtabs { display: flex; gap: 2px; }
.save-ex {
  background: var(--bg-input); border: 1px solid var(--border-strong);
  border-radius: 4px; color: var(--text-dim); font-size: 11px;
  padding: 3px 8px; margin-left: 4px;
}
.save-ex:hover { color: var(--accent); border-color: var(--accent); }
.examples-pane { flex: 1; display: flex; flex-direction: column; min-height: 100px; }
.ex-list { display: flex; flex-direction: column; gap: 4px; }
.ex-row {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 10px; background: var(--bg-input);
  border: 1px solid var(--border); border-radius: 4px;
}
.ex-row:hover { border-color: var(--border-strong); }
.ex-load {
  flex: 1; display: flex; align-items: center; gap: 10px;
  background: transparent; text-align: left;
}
.ex-method { font: 700 10px monospace; min-width: 50px; }
.ex-name { flex: 1; font-size: 12px; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ex-status { font: 700 11px monospace; padding: 1px 6px; border-radius: 3px; }
.ex-status.ok    { color: var(--ok); background: color-mix(in srgb, var(--ok) 12%, transparent); }
.ex-status.warn  { color: var(--warn-text); background: color-mix(in srgb, var(--warn-text) 12%, transparent); }
.ex-status.err   { color: var(--danger); background: color-mix(in srgb, var(--danger) 12%, transparent); }
.ex-time { color: var(--text-faint); font-size: 10px; min-width: 130px; text-align: right; }
.ex-del { color: var(--text-faint); font-size: 11px; padding: 2px 6px; border-radius: 3px; }
.ex-del:hover { color: var(--danger); background: var(--bg-hover); }
.subtabs { padding: 6px 12px 0; border-bottom: 1px solid var(--border); overflow-x: auto; flex-shrink: 0; }
.subtabs button, .res-subtabs button { color: var(--text-dim); font-size: 12px; padding: 6px 10px; border-radius: 5px 5px 0 0; white-space: nowrap; display: flex; align-items: center; gap: 4px; }
.subtabs button.active, .res-subtabs button.active { color: var(--text); border-bottom: 2px solid var(--accent); }
.count { background: var(--border-strong); border-radius: 8px; color: var(--text); font-size: 10px; padding: 0 5px; }
.dot-on { width: 6px; height: 6px; border-radius: 50%; background: var(--ok); }

.subpanel { padding: 12px 16px; overflow-y: auto; flex: 1; }
.kv { display: flex; flex-direction: column; gap: 6px; }
.kv-bar { display: flex; gap: 2px; margin-bottom: 2px; }
.kv-mode { font-size: 10px; padding: 3px 8px; color: var(--text-faint); border-radius: 4px; }
.kv-mode:hover { color: var(--text-dim); }
.kv-mode.active { color: var(--text); background: var(--bg-input); }
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
.oauth-grid { display: grid; grid-template-columns: 110px 1fr; gap: 6px 10px; align-items: center; }
.oauth-grid label { font-size: 11px; color: var(--text-faint); }
.oauth-actions { display: flex; align-items: center; gap: 10px; margin-top: 10px; }
.oauth-get { background: var(--accent); color: var(--accent-text); border-radius: 5px; font-weight: 600; padding: 6px 14px; }
.oauth-get:disabled { opacity: 0.6; cursor: default; }
.oauth-have { color: var(--ok); font-size: 11px; }
.auth-in { background: var(--bg-input); border: 1px solid var(--border); border-radius: 4px; font: 12px monospace; padding: 7px 9px; }
.hint { font-size: 11px; color: var(--text-faint); }

.soon { display: flex; align-items: center; justify-content: center; height: 100%; min-height: 80px; color: var(--text-faint); font-size: 12px; }

.res-bar { display: flex; align-items: center; gap: 14px; padding: 9px 16px; background: var(--bg-elevated); border-bottom: 1px solid var(--border); flex-shrink: 0; }
.timing-meta { display: flex; align-items: center; gap: 6px; cursor: help; }
.waterfall { width: 60px; height: 8px; border-radius: 2px; overflow: hidden; background: var(--bg-input); }
.status { display: flex; align-items: center; gap: 7px; font: 700 13px monospace; }
.dot { width: 7px; height: 7px; border-radius: 50%; }
.meta { color: var(--text-dim); font-size: 12px; }
.res-subtabs { margin-left: auto; }
.res-body { flex: 1; overflow: auto; padding: 12px 16px; color: var(--text); font: 12px/1.5 monospace; white-space: pre-wrap; word-break: break-word; }
.body-view-bar { display: flex; gap: 4px; padding: 4px 16px; background: var(--bg-panel); border-bottom: 1px solid var(--border); flex-shrink: 0; }
.bv-btn { font-size: 10px; padding: 3px 8px; color: var(--text-faint); border-radius: 3px; }
.bv-btn:hover:not(:disabled) { color: var(--text-dim); }
.bv-btn.active { color: var(--text); background: var(--bg-input); }
.bv-btn:disabled { opacity: 0.4; cursor: default; }
.res-headers { flex: 1; overflow: auto; padding: 8px 16px; }
.rh { display: flex; gap: 12px; padding: 4px 0; border-bottom: 1px solid var(--border); font: 12px monospace; }
.rh-k { color: #61affe; min-width: 220px; word-break: break-all; }
.rh-v { color: var(--text); word-break: break-all; }
.script { font-size: 12px; }
.gql { display: flex; flex-direction: column; gap: 6px; height: 100%; }
.gql-label { font-size: 10px; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-faint); }
.gql-vars { min-height: 60px; max-height: 100px; }
.gql-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.gql-schema { font-size: 11px; padding: 3px 10px; background: var(--bg-input); border: 1px solid var(--border-strong); border-radius: 4px; color: var(--text-dim); }
.gql-schema:hover:not(:disabled) { color: var(--accent); border-color: var(--accent); }
.gql-schema:disabled { opacity: 0.6; cursor: default; }
.gql-err { font-size: 11px; color: var(--danger); padding-top: 4px; }
.gql-types { max-height: 200px; overflow: auto; padding: 6px 0; display: flex; flex-direction: column; gap: 3px; }
.gql-type { display: flex; gap: 8px; align-items: baseline; font: 11px monospace; padding: 2px 6px; border-radius: 3px; }
.gql-type:hover { background: var(--bg-hover); }
.gql-kind { color: var(--text-faint); min-width: 60px; }
.gql-name { color: var(--accent); }
.gql-fields { color: var(--text-dim); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
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

<style>
.var-hint {
  position: fixed;
  z-index: 9999;
  background: var(--bg-elevated);
  border: 1px solid var(--border-strong);
  border-radius: 6px;
  padding: 6px 10px;
  box-shadow: 0 6px 20px rgba(0,0,0,0.45);
  pointer-events: none;
  max-width: 400px;
  font-size: 11px;
  font-family: monospace;
}
.var-hint-row { display: flex; gap: 6px; align-items: baseline; padding: 2px 0; white-space: pre; }
.var-hint-name { color: var(--accent); }
.var-hint-sep { color: var(--text-faint); }
.var-hint-val { color: var(--ok); word-break: break-all; white-space: normal; }
.var-hint-unset { color: var(--text-faint); font-style: italic; }
</style>
