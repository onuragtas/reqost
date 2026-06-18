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
- [x] **Save as Example** — `detail.examples_json` migrate, Examples sub-tab + response panelinde "★ Save as example" düğmesi, load/delete/save (`useTabs.ts::SavedExample`, `RequestWorkbench.vue::saveAsExample`)
- [x] **Request chaining — response refs** — `{{Login.response.body.user.id}}`, `{{Login.response.headers.X-Auth}}`, `{{Login.response.status}}`. ExecService in-memory cache name→last response; `httpclient.ResolveResponseRefs` send öncesi inject (`internal/httpclient/refs.go` + test, `exec_service.go`, frontend `SendRequest(reqId, reqName, ...)`)
- [x] **CI Linux apt-cache** — `awalsh128/cache-apt-pkgs-action` `libgtk-4-dev libwebkitgtk-6.0-dev pkg-config` .deb arşivlerini cache'ler; ikinci build'den itibaren apt indirme atlanır
- [x] **gRPC streaming başlığı** (eski parite içinde, henüz değil — aşağıda)

### Devam edilecek

**2026-06-19 ikinci turda biten 8**
- [x] **SSE (Server-Sent Events) console** — `sse://`/`sses://` scheme → `SseConsole.vue` (WsConsole pattern). Backend `internal/sse` benzeri `SSEService` line-by-line parse → `sse:event` emit. Open/event/close/error/id/retry frames.
- [x] **GraphQL schema introspection** — Body type GraphQL: "Load schema" düğmesi → `__schema` introspection POST, kind/name/fields listesi expand'lenebilir. (CodeMirror tabanlı autocomplete bir sonraki adım.)
- [x] **gRPC streaming (server/client/bidi)** — `GRPCService.StreamCall/StreamSend/StreamCloseSend/StreamCancel`, `grpc:event` emit. Tüm üç streaming modu protoreflect dynamic ile çalışıyor.
- [x] **Newman-style CLI runner** — `reqost run <coll.json>` alt-komutu (cli.go), `-e env.json` Postman env, `--format junit|json|text`, `--out path`, `-v` verbose. JUnit XML default. Exit code = fail var/yok. `reqost version`, `reqost help` de eklendi.
- [x] **Mock server (`reqost mock`)** — `reqost mock <coll.json> --port 8090` MVP: koleksiyondaki her request URL path'ini endpoint olarak serve eder. (Saved Example payload entegrasyonu sonraki iterasyona kaldı — `detail.examples_json` parser tarafından henüz çıkarılmıyor.)
- [x] **Folder-level inheritance (shared headers + auth)** — `tree.context_json` migrate; `GetFolderContext/SetFolderContext/AncestorContexts` Wails methodları. Sidebar folder right-click → "Folder context (shared headers / auth)…" → JSON editor. Send-time `resolveAncestorContext` ile root→parent zincirinden merge, child overrides parent. Scripts inheritance scope dışı bırakıldı (security/eval karmaşası).
- [x] **OAuth 2.0 (Auth Code + PKCE, Client Credentials, Password)** — `internal/oauth2`: state + PKCE S256, transient localhost callback listener, `Browser.OpenURL`. `OAuthService` token cache + 30s-buffer otomatik refresh. AuthType `oauth2` + workbench Auth tab'ında grant/scope/tokenUrl/clientId/secret/audience formu + "Get token" düğmesi. Token cache anahtarı = grant|tokenUrl|clientId|scope|audience|username.
- [x] **Multiple workspaces** — `internal/workspaces` Store (`workspaces.json` + `workspaces/<id>/index.db`). İlk açılışta default workspace + legacy `index.db` migrate. `CollectionService.{List,Create,Rename,Delete,Switch}Workspace`. Title bar'da workspace pill + dropdown (rename/delete/new), switch sonrası `collection:ready` event ile tree reload.
- [x] **Git sync** — `git_service.go` child-process git wrapper: `Init/Status/Export/Commit/Branches/Checkout`. `go-git` yerine PATH'teki `git`'i kullanıyor (zero new dep). Export = workspace → Postman v2.1 JSON `<dir>/collection.json`. (UI'a entegrasyon — Settings veya workspace menüsünde "Bind to Git…" — bir sonraki UI iterasyona kaldı; backend hazır.)

**2026-06-19 üçüncü turda biten 4 — tüm parite kapandı**
- [x] **CodeMirror 6 upgrade** — `EditorPane.vue` tek wrapper, body/scripts (pre+post)/graphql query+vars için. JSON/JavaScript/XML language. Line numbers, fold gutter, bracket match, syntax highlight, search, history, autocomplete, indent-with-tab. (`@codemirror/state`, `view`, `language`, `commands`, `search`, `autocomplete`, `lang-json`, `lang-javascript`, `lang-xml`)
- [x] **Variable highlighting + autocomplete** — EditorPane'e `vars` prop. `{{name}}` her oluşumu accent rengiyle vurgulanır; tanımlı değilse kırmızı dalga underline. `{{` tetiklendiğinde aktif env keylerinden + dynamic helpers (`$timestamp/$guid/$randomInt/...`) dropdown. Hover'da resolved value preview.
- [x] **API design-first (OpenAPI editor + mock)** — Sol-rail `Design` modu (yeni icon). `DesignPanel.vue` CodeMirror'da spec edit eder. `internal/openapi` reuse'lu YAML/JSON parse. Backend `DesignService.StartMock(port)` in-app HTTP server: spec'in `paths` map'inden response examples'i serve eder (2xx tercihli, `example`/`examples.*.value`).
- [x] **Plugin / extension sistemi** — `internal/plugins`: cache dir'deki `.js` dosyaları, goja sandbox, hook'lar `onPreSend(req)` / `onPostReceive(req, resp)` / `onTransformBody(req)`. `PluginService.{Dir,List,SetEnabled}`. `ExecService` send öncesi pre-send, sonrası post-receive çağırır. 2s watchdog her hook için. Enable/disable persistence `plugins.json`. Settings paneline plugin list + checkbox.

**Git sync UI (opsiyonel ileri iş)**
- [ ] Workspace dropdown'unda "Bind to Git directory…" — `Init+Export+Commit` tek tıkla
- [ ] Status badge (uncommitted değişiklik var mı)
- [ ] Branches modal — switch / new branch
