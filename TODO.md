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
- [x] Drag-drop tree reorder/move (backend `MoveNode`/reorder + frontend dnd)
- [x] Duplicate request/folder
- [x] Settings sub-tab real settings — timeout, follow-redirects, SSL verify toggle (proxy hariç)
- [ ] Collection/folder-level variables + scripts (pre-request/test inheritance)
- [x] Response history per request — per-request localStorage (son 10), History subtabı
- [x] Keyboard shortcuts — Cmd+Enter send, Cmd+S save, Cmd+W close (dirty check)
- [x] cURL paste — URL alanına curl komutu yapıştırınca otomatik parse
- [x] Environment import/export — Postman env JSON import/export
- [x] Code generation — Python / JavaScript / Go (URL bar'da </> butonu)

## B — UX polish
- [ ] Resizable panels (request/response split + sidebar width)
- [ ] Response: syntax highlight, search-in-response, raw/preview/pretty, HTML/image preview, copy
- [x] Warn on close if tab is dirty
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
- [x] Module still named `changeme` (rename → regenerate bindings) — zaten `reqost`
- [ ] No CI; no frontend tests (Go tests only) — Go tarafına `go test`/`go vet` adımı CI'a eklenmedi
- [ ] Remove the debug `log.Printf("using index at…")` in internal/index/db.go
- [x] Document/automate the lightningcss symlink + `go run` wails3 workarounds — `task fix:lightningcss` + `WAILS_CLI` var
- [ ] WS tabs/messages don't persist across restart
- [x] Settings rail icon was a stub (SettingsPanel.vue in progress) — `SettingsPanel.vue` artık tam
- [x] Build & Release pipeline — `.github/workflows/build.yml`, 4 platform, auto-tag (conventional commits), `workflow_dispatch` `bump` input
- [x] Auto-update — GitHub Releases self-update (`minio/selfupdate`), title-bar update pill
- [x] README + LICENSE (MIT)
- [ ] CI'a `v*` manuel tag push'unda release açan ayrı job
- [ ] CI'a `go test` + `go vet` + `vue-tsc` adımları
- [ ] macOS notarize, Windows code sign (cert gerek)
- [ ] AppImage paketleme (Linux), Universal macOS binary (arm64+amd64 lipo)
- [ ] CONTRIBUTING.md + `.github/ISSUE_TEMPLATE/`

## E — Postman/Insomnia parity (2026-06-19 seansından)

### Tamamlandı (12 yeni özellik)
- [x] **Paste cURL** — Sidebar `+` menü → curl yapıştır → tab açar (`composables/curl.ts::parseCurl`, `useDialog::promptMultiline`)
- [x] **Bulk paste headers** — Headers tab Key-Value ↔ Bulk Edit toggle, `#` ile disable
- [x] **Dynamic variables** — `{{$timestamp}}` `{{$isoTimestamp}}` `{{$unixEpochMs}}` `{{$guid}}` `{{$randomUUID}}` `{{$randomInt}}` `{{$randomBoolean}}` `{{$randomEmail}}` `{{$randomFirstName}}` `{{$randomLastName}}` `{{$randomFullName}}` `{{$randomUserName}}` `{{$randomPassword}}` `{{$randomCity}}` `{{$randomCountry}}` `{{$randomCountryCode}}` `{{$randomPhoneNumber}}` `{{$randomUrl}}` `{{$randomIP}}` `{{$randomColor}}` `{{$randomCompanyName}}` `{{$randomLoremWord}}` `{{$randomLoremSentence}}` (`interpolate.go` + test)
- [x] **Timing waterfall** — Response bar'da DNS/Connect/TLS/Wait/Download SVG + hover tooltip
- [x] **HAR import** — Browser DevTools "Save all as HAR" yapıştır → `internal/har` parse + `AddItems` (test'i ile, pseudo-header filter)
- [x] **Code generation — 8 dil** — cURL, Python `requests`, Node `fetch`, Go `net/http`, Java OkHttp, C# HttpClient, PowerShell, Raw HTTP wire (`useCodeGen.ts`)
- [x] **JSON tree view + search** — Pretty/Raw/Tree toggle, collapsible JSON, key/value filter (`JsonTree.vue`, `JsonNode.vue`)
- [x] **Command Palette (Cmd+K) + Quick Switcher (Cmd+P)** — FTS5 fuzzy request search + global action registry (`useCommands.ts`, `CommandPalette.vue`)
- [x] **mTLS / client certificates** — Settings'te host pattern → cert/key path; wildcard + suffix match; per-request fresh TLS transport (`client.go::matchClientCert`)
- [x] **Vault — masked secrets** — Env var `secret` flag, `type="password"` + 👁 reveal (`envstore.Var.Secret`, `EnvironmentsModal.vue`)
- [x] **Proxy settings (global + per-request)** — Settings → Proxy URL, cache'li transport per-proxy (`client.go::transportFor`)
- [x] **gRPC streaming başlığı** (eski parite içinde, henüz değil — aşağıda)

### Devam edilecek

**Orta efor (4–8 saat her biri)**
- [ ] **Save as Example** — Response'u `detail.examples_json`'a snapshot. Workbench Examples sekmesi. Mock server için ön gereksinim.
- [ ] **Request chaining — response reference syntax** — Insomnia tarzı `{{Login.response.body.$.token}}`. Backend in-memory cache name-keyed. `interpolate` öncesi resolve.
- [ ] **SSE (Server-Sent Events) console** — `text/event-stream` Accept → `SseConsole.vue` (WsConsole pattern). Line-by-line scan → `sse:event` emit.
- [ ] **GraphQL schema introspection + autocomplete** — `__schema` POST cache. CodeMirror graphql language (CodeMirror upgrade sonrası).
- [ ] **gRPC streaming (server/client/bidi)** — `StreamCall` method, `grpc:event` emit, `GrpcConsole.vue` Send/Recv panel.
- [ ] **Newman-style CLI runner** — `reqost run <collection.json>` alt-komutu, JUnit/JSON output, exit code.
- [ ] **Mock server (Saved Examples)** — `reqost mock` veya in-app server. Saved Example'ları endpoint olarak serv et. **Save as Example sonrası.**

**Büyük (1+ gün her biri)**
- [ ] **Folder-level auth / headers / scripts inheritance** — `tree.folder` ek alanlar, send-time ancestor merge, child override.
- [ ] **OAuth 2.0 flows (Auth Code + PKCE, Client Credentials, Password)** — `internal/oauth2`, localhost listener redirect, `Browser.OpenURL`, token cache + auto-refresh. **En büyük tek gap.**
- [ ] **CodeMirror 6 editor upgrade** — body / scripts / response için. Tek `EditorPane.vue` wrapper, modlar: JSON/JS/XML/GraphQL/HTTP.
- [ ] **Variable highlighting + autocomplete** — `{{var}}` renkli, hover preview, eksik var kırmızı, `{{` tetikleyince dropdown. **CodeMirror sonrası.**

**Çok büyük (haftalar)**
- [ ] **Multiple workspaces** — Tek DB → birden çok, `cache/workspaces/<id>/index.db`, title bar switcher.
- [ ] **Git sync** — Workspace ↔ git repo, commit/branch/diff, `go-git`.
- [ ] **API design-first (OpenAPI editor + mock)** — Sol-rail `Design` modu, Insomnia eşdeğeri.
- [ ] **Plugin / extension sistemi** — `goja` sandbox, `preSend`/`postReceive`/`transformBody` hook'ları, custom auth providerlar.
