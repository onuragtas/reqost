import type { HeaderRow, Auth, BodyType } from './useTabs'

export type CodeLang =
  | 'curl' | 'python' | 'javascript' | 'go'
  | 'java' | 'csharp' | 'powershell' | 'http'

export const CODE_LANGS: { id: CodeLang; label: string }[] = [
  { id: 'curl',       label: 'cURL' },
  { id: 'python',     label: 'Python (requests)' },
  { id: 'javascript', label: 'JavaScript (fetch)' },
  { id: 'go',         label: 'Go (net/http)' },
  { id: 'java',       label: 'Java (OkHttp)' },
  { id: 'csharp',     label: 'C# (HttpClient)' },
  { id: 'powershell', label: 'PowerShell' },
  { id: 'http',       label: 'Raw HTTP' },
]

export interface CodeGenInput {
  method: string
  url: string
  headers: HeaderRow[]
  body: string
  bodyType: BodyType
  auth: Auth
}

function activeHeaders(headers: HeaderRow[]): HeaderRow[] {
  return headers.filter(h => h.enabled !== false && h.key.trim())
}

function authHeader(auth: Auth): { key: string; value: string } | null {
  if (auth.type === 'bearer') return { key: 'Authorization', value: `Bearer ${auth.token}` }
  if (auth.type === 'basic')  return { key: 'Authorization', value: `Basic ${btoa(`${auth.username}:${auth.password}`)}` }
  if (auth.type === 'apikey' && auth.key) return { key: auth.key, value: auth.value }
  return null
}

function allHeaders(input: CodeGenInput): { key: string; value: string }[] {
  const h = activeHeaders(input.headers).map(r => ({ key: r.key, value: r.value }))
  const a = authHeader(input.auth)
  if (a) h.push(a)
  return h
}

function hasBody(input: CodeGenInput) {
  return input.body && input.method !== 'GET' && input.method !== 'HEAD'
}

export function generatePython(input: CodeGenInput): string {
  const headers = allHeaders(input)
  const headerObj = headers.length
    ? '{\n' + headers.map(h => `        "${h.key}": "${h.value.replace(/"/g, '\\"')}"`).join(',\n') + '\n    }'
    : '{}'
  const bodyLine = hasBody(input)
    ? `,\n    ${input.bodyType === 'json' ? 'json=' : 'data='}${JSON.stringify(input.body)}`
    : ''
  return `import requests

response = requests.${input.method.toLowerCase()}(
    "${input.url}",
    headers=${headerObj}${bodyLine}
)

print(response.status_code)
print(response.text)
`
}

export function generateJS(input: CodeGenInput): string {
  const headers = allHeaders(input)
  const headerObj = headers.length
    ? '{\n' + headers.map(h => `    "${h.key}": "${h.value.replace(/"/g, '\\"')}"`).join(',\n') + '\n  }'
    : '{}'
  const bodyLine = hasBody(input) ? `\n  body: ${JSON.stringify(input.body)},` : ''
  return `const response = await fetch("${input.url}", {
  method: "${input.method}",
  headers: ${headerObj},${bodyLine}
});

const data = await response.text();
console.log(response.status, data);
`
}

// ── Java OkHttp ────────────────────────────────────────────────────────────
export function generateJava(input: CodeGenInput): string {
  const headers = allHeaders(input)
  const m = input.method.toUpperCase()
  const has = hasBody(input)
  const bodyLine = has
    ? `RequestBody body = RequestBody.create(${JSON.stringify(input.body)}, MediaType.parse("application/octet-stream"));\n`
    : ''
  const methodCall = m === 'GET'
    ? '.get()'
    : `.method("${m}", ${has ? 'body' : (m === 'POST' || m === 'PUT' || m === 'PATCH' ? 'RequestBody.create(new byte[0])' : 'null')})`
  const headerLines = headers.map(h => `  .addHeader(${JSON.stringify(h.key)}, ${JSON.stringify(h.value)})`).join('\n')
  return `OkHttpClient client = new OkHttpClient();

${bodyLine}Request req = new Request.Builder()
  .url(${JSON.stringify(input.url)})
${headerLines ? headerLines + '\n' : ''}  ${methodCall}
  .build();

try (Response resp = client.newCall(req).execute()) {
  System.out.println(resp.code());
  System.out.println(resp.body().string());
}
`
}

// ── C# HttpClient ──────────────────────────────────────────────────────────
export function generateCSharp(input: CodeGenInput): string {
  const headers = allHeaders(input)
  const m = input.method.toUpperCase()
  const has = hasBody(input)
  const lines = [
    `using var client = new HttpClient();`,
    `var req = new HttpRequestMessage(new HttpMethod(${JSON.stringify(m)}), ${JSON.stringify(input.url)});`,
  ]
  for (const h of headers) {
    lines.push(`req.Headers.TryAddWithoutValidation(${JSON.stringify(h.key)}, ${JSON.stringify(h.value)});`)
  }
  if (has) lines.push(`req.Content = new StringContent(${JSON.stringify(input.body)});`)
  lines.push(
    `var resp = await client.SendAsync(req);`,
    `Console.WriteLine((int)resp.StatusCode);`,
    `Console.WriteLine(await resp.Content.ReadAsStringAsync());`,
  )
  return lines.join('\n')
}

// ── PowerShell Invoke-RestMethod ───────────────────────────────────────────
export function generatePowerShell(input: CodeGenInput): string {
  const headers = allHeaders(input)
  const m = input.method.toUpperCase()
  const has = hasBody(input)
  const hb = headers.length
    ? `$headers = @{\n${headers.map(h => `  ${JSON.stringify(h.key)} = ${JSON.stringify(h.value)}`).join('\n')}\n}\n`
    : ''
  const args = [`-Method ${m}`, `-Uri ${JSON.stringify(input.url)}`]
  if (headers.length) args.push('-Headers $headers')
  if (has) args.push(`-Body ${JSON.stringify(input.body)}`)
  return `${hb}Invoke-RestMethod ${args.join(' ')}`
}

// ── Raw HTTP wire ──────────────────────────────────────────────────────────
export function generateHTTP(input: CodeGenInput): string {
  const headers = allHeaders(input)
  let pathQuery = input.url, host = ''
  try {
    const u = new URL(input.url.replace(/{{[^}]+}}/g, 'x'))
    pathQuery = u.pathname + u.search
    host = u.host
  } catch { /* keep raw url */ }
  const lines: string[] = [`${input.method.toUpperCase()} ${pathQuery} HTTP/1.1`]
  if (host) lines.push(`Host: ${host}`)
  for (const h of headers) lines.push(`${h.key}: ${h.value}`)
  lines.push('')
  if (input.body) lines.push(input.body)
  return lines.join('\n')
}

export function generateGo(input: CodeGenInput): string {
  const headers = allHeaders(input)
  const headerLines = headers.map(h => `\treq.Header.Set("${h.key}", "${h.value.replace(/"/g, '\\"')}")`).join('\n')
  const bodyDecl = hasBody(input)
    ? `body := strings.NewReader(${JSON.stringify(input.body)})\n\t`
    : ''
  const bodyArg = hasBody(input) ? 'body' : 'nil'
  const imports = ['fmt', 'io', 'net/http', ...(hasBody(input) ? ['strings'] : [])].map(p => `\t"${p}"`).join('\n')
  return `package main

import (
${imports}
)

func main() {
\t${bodyDecl}req, _ := http.NewRequest("${input.method}", "${input.url}", ${bodyArg})
${headerLines ? headerLines + '\n' : ''}\tclient := &http.Client{}
\tresp, _ := client.Do(req)
\tdefer resp.Body.Close()
\tbody2, _ := io.ReadAll(resp.Body)
\tfmt.Println(resp.Status)
\tfmt.Println(string(body2))
}
`
}
