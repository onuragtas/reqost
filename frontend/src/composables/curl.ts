import type { HeaderRow, BodyType, FormRow } from './useTabs'

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

export interface ParsedCurl {
  method: string
  url: string
  headers: HeaderRow[]
  body: string
  bodyType: BodyType
  formFields: FormRow[]
}

// parseCurl parses a cURL command string into request fields.
// Returns null if the text is not a curl command.
export function parseCurl(text: string): ParsedCurl | null {
  const line = text.replace(/\\\n\s*/g, ' ').trim()
  if (!line.startsWith('curl ') && line !== 'curl') return null

  const tokens = shellTokenize(line)
  if (tokens.length < 2) return null

  let url = ''
  let method = ''
  const headers: HeaderRow[] = []
  let body = ''

  let i = 1
  while (i < tokens.length) {
    const t = tokens[i]
    if (t === '-X' || t === '--request') {
      method = tokens[++i] ?? ''
    } else if (t === '-H' || t === '--header') {
      const h = tokens[++i] ?? ''
      const colon = h.indexOf(':')
      if (colon > 0) {
        headers.push({ key: h.slice(0, colon).trim(), value: h.slice(colon + 1).trim(), enabled: true })
      }
    } else if (t === '-d' || t === '--data' || t === '--data-raw' || t === '--data-binary' || t === '--data-ascii') {
      body = tokens[++i] ?? ''
    } else if (t === '--url') {
      url = tokens[++i] ?? ''
    } else if (t === '-u' || t === '--user') {
      const creds = tokens[++i] ?? ''
      headers.push({ key: 'Authorization', value: `Basic ${btoa(creds)}`, enabled: true })
    } else if (!t.startsWith('-') && !url) {
      url = t
    }
    i++
  }

  if (!method) method = body ? 'POST' : 'GET'
  method = method.toUpperCase()

  let bodyType: BodyType = 'none'
  let formFields: FormRow[] = []

  if (body) {
    const ct = headers.find(h => h.key.toLowerCase() === 'content-type')?.value ?? ''
    if (ct.includes('application/x-www-form-urlencoded')) {
      bodyType = 'urlencoded'
      formFields = body.split('&').map(pair => {
        const eq = pair.indexOf('=')
        const k = eq >= 0 ? pair.slice(0, eq) : pair
        const v = eq >= 0 ? pair.slice(eq + 1) : ''
        return { key: decodeURIComponent(k), value: decodeURIComponent(v), type: 'text' as const, enabled: true }
      }).filter(f => f.key)
    } else if (ct.includes('application/json') || isJSON(body)) {
      bodyType = 'json'
    } else {
      bodyType = 'raw'
    }
  }

  return { method, url, headers, body, bodyType, formFields }
}

function isJSON(s: string): boolean {
  const t = s.trim()
  return (t.startsWith('{') && t.endsWith('}')) || (t.startsWith('[') && t.endsWith(']'))
}

function shellTokenize(input: string): string[] {
  const tokens: string[] = []
  let i = 0
  const len = input.length
  while (i < len) {
    while (i < len && /\s/.test(input[i])) i++
    if (i >= len) break
    let token = ''
    while (i < len && !/\s/.test(input[i])) {
      const c = input[i]
      if (c === "'") {
        i++
        while (i < len && input[i] !== "'") token += input[i++]
        if (i < len) i++
      } else if (c === '"') {
        i++
        while (i < len && input[i] !== '"') {
          if (input[i] === '\\' && i + 1 < len) { i++; token += input[i++] }
          else token += input[i++]
        }
        if (i < len) i++
      } else if (c === '\\' && i + 1 < len) {
        i++; token += input[i++]
      } else {
        token += input[i++]
      }
    }
    if (token) tokens.push(token)
  }
  return tokens
}
