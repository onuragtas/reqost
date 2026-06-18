import { reactive, computed } from 'vue'
import { GetRequestDetail } from '../../bindings/reqost/collectionservice'
import type { FlatNode } from './useTree'
import { parseQuery } from './url'

export interface HeaderRow { key: string; value: string; enabled: boolean }
export interface FormRow { key: string; value: string; type: 'text' | 'file'; enabled: boolean }
export type BodyType = 'none' | 'raw' | 'json' | 'urlencoded' | 'formdata' | 'graphql'

// Per-request execution settings. undefined means "inherit the app default".
export interface RequestSettings {
  timeoutMs?: number
  followRedirects?: boolean
  maxRedirects?: number
  verifySSL?: boolean
}

export type ReqSubTab = 'params' | 'auth' | 'headers' | 'body' | 'prereq' | 'tests' | 'examples' | 'settings'
export type ResSubTab = 'body' | 'headers' | 'cookies' | 'testResults' | 'history'

// SavedExample — a frozen snapshot of a request + its response. Per-request,
// persisted in detail.examples_json. Powers Postman-style "Save as example"
// and the planned local mock server (see TODO.md).
export interface SavedExample {
  id: string
  name: string
  savedAt: number  // ms epoch
  request: {
    method: string
    url: string
    headers: HeaderRow[]
    body: string
    bodyType: BodyType
  }
  response: {
    status: number
    statusText: string
    headers: { key: string; value: string; enabled: boolean }[]
    body: string
    sizeBytes: number
  }
}

export interface TestRow { name: string; passed: boolean; error: string }

export type AuthType = 'none' | 'bearer' | 'basic' | 'apikey' | 'oauth2'

export type OAuthGrant = 'client_credentials' | 'password' | 'authorization_code'

export interface OAuth2Config {
  grant: OAuthGrant
  authUrl?: string
  tokenUrl: string
  clientId: string
  clientSecret?: string
  username?: string
  password?: string
  scope?: string
  audience?: string
  redirectUri?: string
  usePkce?: boolean
  clientAuthIn?: 'header' | 'body'
}

export interface Auth {
  type: AuthType
  token: string
  username: string
  password: string
  key: string
  value: string
  // OAuth 2.0 — only meaningful when type === 'oauth2'. The acquired access
  // token is mirrored into `token` so the transport layer treats it like a
  // Bearer for actual sending.
  oauth2?: OAuth2Config
}

function blankAuth(): Auth {
  return { type: 'none', token: '', username: '', password: '', key: '', value: '' }
}

// One open request tab. Holds its own editable request + last response so
// switching tabs preserves edits and results.
export interface ReqTab {
  id: string
  name: string
  method: string
  url: string
  params: HeaderRow[]
  headers: HeaderRow[]
  body: string
  bodyType: BodyType
  formFields: FormRow[]
  graphqlVars: string
  grpcMethod: string
  auth: Auth
  // Preserved across edits so Save doesn't wipe imported scripts/description.
  preScript: string
  postScript: string
  description: string
  settings: RequestSettings
  examples: SavedExample[]
  clean: string // snapshot() at last load/save; '' means not yet baselined
  reqSubTab: ReqSubTab
  resSubTab: ResSubTab
  loading: boolean
  sending: boolean
  sendError: string
  response: any | null
  tests: TestRow[]
  logs: string[]
}

export interface AdhocRequest {
  name: string
  method: string
  url: string
  headers?: HeaderRow[]
  body?: string
  bodyType?: BodyType
  formFields?: FormRow[]
  auth?: Auth
}

const state = reactive({
  tabs: [] as ReqTab[],
  activeId: '',
})

function blankTab(id: string, name: string, method: string): ReqTab {
  return {
    id, name, method: method || 'GET',
    url: '', params: [], headers: [], body: '', bodyType: 'none', formFields: [], graphqlVars: '', grpcMethod: '', auth: blankAuth(),
    preScript: '', postScript: '', description: '', settings: {}, examples: [], clean: '',
    reqSubTab: 'headers', resSubTab: 'body',
    loading: true, sending: false, sendError: '', response: null,
    tests: [], logs: [],
  }
}

function genId(): string {
  try { return crypto.randomUUID() } catch { return `tab-${Date.now()}-${Math.floor(Math.random() * 1e6)}` }
}

function parseHeaders(json: string): HeaderRow[] {
  try {
    const arr = JSON.parse(json || '[]')
    if (!Array.isArray(arr)) return []
    return arr.map((h: any) => ({ key: h.key ?? '', value: h.value ?? '', enabled: h.enabled !== false }))
  } catch {
    return []
  }
}

function parseForm(json: string): FormRow[] {
  try {
    const arr = JSON.parse(json || '[]')
    if (!Array.isArray(arr)) return []
    return arr.map((f: any) => ({ key: f.key ?? '', value: f.value ?? '', type: f.type === 'file' ? 'file' : 'text', enabled: f.enabled !== false }))
  } catch {
    return []
  }
}

function parseAuth(json: string): Auth {
  const a = blankAuth()
  try {
    const o = JSON.parse(json || '{}')
    if (o && typeof o === 'object') Object.assign(a, o)
  } catch { /* keep blank */ }
  return a
}

function parseExamples(json: string): SavedExample[] {
  try {
    const arr = JSON.parse(json || '[]')
    if (!Array.isArray(arr)) return []
    return arr.filter(e => e && typeof e === 'object' && e.id) as SavedExample[]
  } catch { return [] }
}

function parseSettings(json: string): RequestSettings {
  try {
    const o = JSON.parse(json || '{}')
    if (o && typeof o === 'object') {
      const out: RequestSettings = {}
      if (typeof o.timeoutMs === 'number') out.timeoutMs = o.timeoutMs
      if (typeof o.followRedirects === 'boolean') out.followRedirects = o.followRedirects
      if (typeof o.maxRedirects === 'number') out.maxRedirects = o.maxRedirects
      if (typeof o.verifySSL === 'boolean') out.verifySSL = o.verifySSL
      return out
    }
  } catch { /* keep empty */ }
  return {}
}

// load is keyed by id so it mutates the *reactive* tab from the store, not a
// raw object reference — otherwise Vue never sees loading flip to false and the
// pane stays stuck on "Loading…" until an unrelated re-render.
async function load(id: string) {
  const tab = state.tabs.find(t => t.id === id)
  if (!tab) return
  tab.loading = true
  try {
    const d: any = await GetRequestDetail(tab.id)
    if (!d) return
    tab.method = d.method || 'GET'
    tab.url = d.url || ''
    tab.params = parseQuery(tab.url)
    tab.body = d.body || ''
    tab.bodyType = (d.bodyType as BodyType) || (d.body ? 'raw' : 'none')
    tab.formFields = parseForm(d.formFields)
    tab.graphqlVars = d.graphqlVars || ''
    tab.grpcMethod = d.grpcMethod || ''
    tab.headers = parseHeaders(d.headers)
    tab.auth = parseAuth(d.auth)
    tab.preScript = d.preScript || ''
    tab.postScript = d.postScript || ''
    tab.description = d.description || ''
    tab.settings = parseSettings(d.settings)
    tab.examples = parseExamples(d.examples)
    tab.reqSubTab = d.body ? 'body' : 'headers'
    markClean(tab)
  } finally {
    tab.loading = false
  }
}

// snapshot is a stable serialization of a tab's saveable state, used to detect
// unsaved edits (dirty) by comparing against the value captured at load/save.
export function snapshot(tab: ReqTab): string {
  return JSON.stringify(toDetail(tab))
}
export function isDirty(tab: ReqTab): boolean {
  return tab.clean !== '' && snapshot(tab) !== tab.clean
}
export function markClean(tab: ReqTab) {
  tab.clean = snapshot(tab)
}

// toDetail serializes a tab back into the index RequestDetail shape for Save.
export function toDetail(tab: ReqTab) {
  return {
    id: tab.id,
    name: tab.name,
    method: tab.method,
    url: tab.url.trim(),
    headers: JSON.stringify(tab.headers),
    body: tab.body,
    preScript: tab.preScript,
    postScript: tab.postScript,
    description: tab.description,
    bodyType: tab.bodyType,
    formFields: JSON.stringify(tab.formFields),
    graphqlVars: tab.graphqlVars,
    grpcMethod: tab.grpcMethod,
    auth: JSON.stringify(tab.auth),
    settings: JSON.stringify(tab.settings),
    examples: JSON.stringify(tab.examples),
  }
}

export function useTabs() {
  const tabs = computed(() => state.tabs)
  const activeId = computed(() => state.activeId)
  const active = computed(() => state.tabs.find(t => t.id === state.activeId) ?? null)

  function openRequest(node: FlatNode) {
    const existing = state.tabs.find(t => t.id === node.id)
    if (existing) {
      state.activeId = existing.id
      return
    }
    const tab = blankTab(node.id, node.name, node.method)
    state.tabs.push(tab)
    state.activeId = tab.id
    void load(tab.id)
  }

  // openAdhoc opens a request that isn't backed by a collection node (e.g. from
  // History). Always a fresh tab with a synthetic id.
  function openAdhoc(req: AdhocRequest) {
    const tab = blankTab(genId(), req.name || req.url, req.method)
    tab.url = req.url
    tab.params = parseQuery(req.url)
    tab.headers = req.headers ? req.headers.map(h => ({ ...h })) : []
    tab.body = req.body ?? ''
    tab.bodyType = req.bodyType ?? (tab.body ? 'raw' : 'none')
    tab.formFields = req.formFields ? req.formFields.map(f => ({ ...f })) : []
    if (req.auth) tab.auth = { ...req.auth }
    tab.loading = false
    markClean(tab)
    state.tabs.push(tab)
    state.activeId = tab.id
  }

  function selectTab(id: string) {
    state.activeId = id
  }

  function closeTab(id: string) {
    const idx = state.tabs.findIndex(t => t.id === id)
    if (idx === -1) return
    state.tabs.splice(idx, 1)
    if (state.activeId === id) {
      const next = state.tabs[idx] ?? state.tabs[idx - 1] ?? null
      state.activeId = next ? next.id : ''
    }
  }

  return { tabs, activeId, active, openRequest, openAdhoc, selectTab, closeTab }
}
