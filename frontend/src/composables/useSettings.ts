import { reactive, watch } from 'vue'

// AppSettings holds global, app-wide defaults persisted to localStorage.
// Per-request fields (timeout, ...) can override these.
export interface ClientCert {
  hostPattern: string  // e.g. "api.example.com", "*.corp.local"
  certPath: string
  keyPath: string
}

export interface AppSettings {
  defaultTimeoutMs: number       // 0 = no timeout
  defaultFollowRedirects: boolean
  defaultVerifySSL: boolean
  defaultMaxRedirects: number    // 0 = library default
  proxyURL: string               // empty = use system proxy from env
  clientCerts: ClientCert[]      // mTLS — first matching pattern wins
  caFilePath: string             // additional PEM root bundle; empty = system roots only
}

const KEY = 'reqost:settings:v1'

const DEFAULTS: AppSettings = {
  defaultTimeoutMs: 0,
  defaultFollowRedirects: true,
  defaultVerifySSL: true,
  defaultMaxRedirects: 10,
  proxyURL: '',
  clientCerts: [],
  caFilePath: '',
}

function load(): AppSettings {
  try {
    const raw = localStorage.getItem(KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      return { ...DEFAULTS, ...parsed }
    }
  } catch { /* fall through */ }
  return { ...DEFAULTS }
}

const settings = reactive<AppSettings>(load())

watch(
  settings,
  v => {
    try { localStorage.setItem(KEY, JSON.stringify(v)) } catch { /* ignore */ }
  },
  { deep: true },
)

export function useSettings() {
  function reset() {
    Object.assign(settings, DEFAULTS)
  }
  return { settings, reset }
}
