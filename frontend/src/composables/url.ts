import type { HeaderRow } from './useTabs'

// Lightweight query <-> rows helpers. We do NOT use the URL/URLSearchParams
// API because request URLs routinely contain {{variables}} that aren't valid
// URLs yet, and we must preserve key order and the disabled state of params.

export function parseQuery(url: string): HeaderRow[] {
  const q = url.indexOf('?')
  if (q === -1) return []
  const query = url.slice(q + 1)
  if (!query) return []
  return query.split('&').map((pair) => {
    const eq = pair.indexOf('=')
    const key = eq === -1 ? pair : pair.slice(0, eq)
    const value = eq === -1 ? '' : pair.slice(eq + 1)
    return { key: safeDecode(key), value: safeDecode(value), enabled: true }
  })
}

export function baseOf(url: string): string {
  const q = url.indexOf('?')
  return q === -1 ? url : url.slice(0, q)
}

export function buildUrl(base: string, params: HeaderRow[]): string {
  const active = params.filter((p) => p.enabled && (p.key.trim() || p.value.trim()))
  if (!active.length) return base
  const query = active
    .map((p) => `${safeEncode(p.key)}=${safeEncode(p.value)}`)
    .join('&')
  return `${base}?${query}`
}

// Encode but leave {{ }} placeholders intact and human-readable.
function safeEncode(s: string): string {
  return encodeURIComponent(s).replace(/%7B%7B/g, '{{').replace(/%7D%7D/g, '}}')
}
function safeDecode(s: string): string {
  try { return decodeURIComponent(s) } catch { return s }
}
