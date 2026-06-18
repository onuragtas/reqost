# reqost — TODO / Backlog

Persistent backlog (the chat doesn't survive new sessions; this file does).
Check items off as they land. See CLAUDE.md for architecture.

## ✅ Done
REST / GraphQL / WebSocket / gRPC · environments + `{{vars}}` · auth (bearer/basic/apikey)
· params · history · cookies panel · Save / Create / Rename / Delete · Export (Postman v2.1)
· Copy as cURL · body types (raw/json/urlencoded/formdata/file) · `pm.*` script engine
(+ `pm.sendRequest`) · Collection Runner · OpenAPI/Swagger import · light/dark theme
· dirty indicator · editable name/description · native file dialogs · import auth/body-mode parse.
Fixed: webview prompt/confirm, loading reactivity, delete SQLITE_BUSY, delete FTS perf.

## A — High-value features
- [ ] Drag-drop tree reorder/move (backend `MoveNode`/reorder + frontend dnd)
- [ ] Duplicate request/folder
- [ ] Settings sub-tab real settings — timeout, follow-redirects, SSL verify toggle, proxy
- [ ] Collection/folder-level variables + scripts (pre-request/test inheritance)
- [ ] Response history per request
- [ ] Keyboard shortcuts — Cmd+Enter send, Cmd+S save, Cmd+W close
- [ ] Environment import/export (Postman env files)
- [ ] Code generation beyond cURL (Python/JS/Go…)

## B — UX polish
- [ ] Resizable panels (request/response split + sidebar width)
- [ ] Response: syntax highlight, search-in-response, raw/preview/pretty, HTML/image preview, copy
- [ ] Warn on close if tab is dirty
- [ ] Native file picker for form-data file fields (currently manual path)
- [ ] Inline `{{var}}` peek (resolved value on hover)
- [ ] Sidebar multi-select / breadcrumbs

## C — Close known limitations
- [ ] Auth depth — query API key, OAuth2, AWS sig, digest (header-only today)
- [ ] OpenAPI deepening — path param `{id}` → `{{id}}`, non-JSON content types, `allOf/oneOf`
- [ ] gRPC streaming (client/server/bidi) + metadata + mTLS/custom CA
- [ ] WebSocket — subprotocols, binary frames, ping/pong, auto-reconnect
- [ ] Move WS/gRPC `{{var}}` interpolation into the Go engine (frontend-only now)
- [ ] Runner — data-file iteration, delay, stop button
- [ ] Surface a notice when a response is truncated at the 50 MiB cap (silent today)
- [ ] pm.* parity — `pm.cookies`, `pm.iterationData`, async timers, chai `.deep`

## D — Tech debt / infra
- [ ] Module still named `changeme` (rename → regenerate bindings)
- [ ] No CI; no frontend tests (Go tests only)
- [ ] Remove the debug `log.Printf("using index at…")` in internal/index/db.go
- [ ] Document/automate the lightningcss symlink + `go run` wails3 workarounds
- [ ] WS tabs/messages don't persist across restart
- [ ] Settings rail icon was a stub (SettingsPanel.vue in progress)
