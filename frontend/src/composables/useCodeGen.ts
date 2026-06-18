import type { HeaderRow, Auth, BodyType } from './useTabs'

export type CodeLang = 'curl' | 'python' | 'javascript' | 'go'

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
