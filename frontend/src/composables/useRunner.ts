import { reactive } from 'vue'
import { ListRequestsUnder, GetRequestDetail } from '../../bindings/reqost/collectionservice'
import { SendRequest } from '../../bindings/reqost/execservice'
import { useEnv } from './useEnv'

export interface RunRow {
  id: string
  name: string
  method: string
  url: string
  status: number
  ok: boolean
  ms: number
  passed: number
  total: number
  error: string
}

const state = reactive({
  open: false,
  running: false,
  done: 0,
  total: 0,
  rows: [] as RunRow[],
})

function parseHeaders(json: string) {
  try {
    const a = JSON.parse(json || '[]')
    return Array.isArray(a) ? a.filter((h: any) => h.key) : []
  } catch {
    return []
  }
}

export function useRunner() {
  const { activeVars, applyVars } = useEnv()

  function close() { state.open = false }

  async function run(rootId: string) {
    state.open = true
    state.running = true
    state.rows = []
    state.done = 0

    const list: any[] = (await ListRequestsUnder(rootId)) ?? []
    state.total = list.length

    // Running variable map threads pm.environment.set across requests.
    const vars: Record<string, string> = { ...activeVars.value }

    for (const node of list) {
      const d: any = await GetRequestDetail(node.id)
      const row: RunRow = {
        id: node.id, name: node.name, method: d?.method || node.method || 'GET',
        url: d?.url || '', status: 0, ok: false, ms: 0, passed: 0, total: 0, error: '',
      }
      try {
        const res: any = await SendRequest(node.id, {
          protocol: 'http',
          method: row.method,
          url: row.url,
          headers: parseHeaders(d?.headers),
          body: d?.body || '',
          bodyType: d?.body ? 'raw' : 'none',
          formFields: [],
          auth: null,
          variables: vars,
        }, d?.preScript || '', d?.postScript || '')

        const resp = res?.response
        row.status = resp?.status ?? 0
        row.ms = resp?.timing?.totalMs ?? 0
        row.ok = row.status >= 200 && row.status < 400
        const tests = res?.tests ?? []
        row.total = tests.length
        row.passed = tests.filter((t: any) => t.passed).length
        if (res?.vars) Object.assign(vars, res.vars)
        if (res?.scriptError) row.error = res.scriptError
      } catch (e: any) {
        row.error = e?.message ?? String(e)
      }
      state.rows.push(row)
      state.done++
    }

    applyVars(vars)
    state.running = false
  }

  return { state, run, close }
}
