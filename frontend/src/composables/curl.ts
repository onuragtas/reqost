import type { HeaderRow } from './useTabs'

// toCurl builds a copy-pasteable cURL command. Values are emitted as-is
// (including {{variables}}) — it mirrors what the user typed, like Postman's
// "Copy as cURL".
export function toCurl(method: string, url: string, headers: HeaderRow[], body: string): string {
  const q = (s: string) => `'${s.replace(/'/g, `'\\''`)}'`
  const parts = [`curl -X ${method || 'GET'} ${q(url)}`]
  for (const h of headers) {
    if (h.enabled === false || !h.key.trim()) continue
    parts.push(`-H ${q(`${h.key}: ${h.value}`)}`)
  }
  if (body && method !== 'GET' && method !== 'HEAD') {
    parts.push(`--data ${q(body)}`)
  }
  return parts.join(' \\\n  ')
}
