import { AncestorContexts } from '../../bindings/reqost/collectionservice'
import type { HeaderRow, Auth } from './useTabs'

// Folder-level inheritance: each folder can carry shared headers + an
// override auth, merged into descendant requests at send time. Stored as a
// single JSON blob on tree.context_json so the schema doesn't grow per-field.

export interface FolderContext {
  sharedHeaders?: HeaderRow[]
  auth?: Auth
}

export interface MergedContext {
  headers: HeaderRow[]
  auth: Auth | null
}

export function parseContext(json: string): FolderContext {
  try {
    const o = JSON.parse(json || '{}')
    if (o && typeof o === 'object') return o as FolderContext
  } catch { /* keep empty */ }
  return {}
}

// resolveAncestorContext walks the ancestor chain via the backend and merges:
//   headers: ancestor-first concat (root first, immediate parent last) — the
//            request itself can still override by key, but folder-level
//            duplicates are kept since headers ARE allowed multi-valued.
//   auth: nearest non-empty wins (so a folder auth shadows a higher folder
//         auth; the request's own auth field still wins downstream).
export async function resolveAncestorContext(requestId: string): Promise<MergedContext> {
  if (!requestId) return { headers: [], auth: null }
  let chain: string[] = []
  try {
    chain = (await AncestorContexts(requestId)) ?? []
  } catch { return { headers: [], auth: null } }

  const headers: HeaderRow[] = []
  let auth: Auth | null = null

  for (const json of chain) {
    const ctx = parseContext(json)
    if (ctx.sharedHeaders) {
      for (const h of ctx.sharedHeaders) {
        if (h?.key) headers.push({ ...h })
      }
    }
    if (ctx.auth && ctx.auth.type && ctx.auth.type !== 'none') {
      auth = { ...ctx.auth }
    }
  }
  return { headers, auth }
}
