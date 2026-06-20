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

## F — Küçük UX gap'leri (Postman/Insomnia paritesinde sıkça farkedilenler)

Çekirdek parite kapandı, ama gündelik kullanımda gözüne batan ufak "yok"lar. Birikti — sıraladım, kolay→orta efor sırasıyla.

### Search / Navigation
- [x] **Cmd+F response body içinde arama** — Pretty/Raw artık `EditorPane` (CodeMirror) → built-in search keymap çalışır. JSON tree mode'da zaten filter input vardı.
- [ ] **Cmd+F response headers içinde arama** — headers şu an plain `<div>`. Headers'i de küçük bir filter input + match highlight ile sar.
- [ ] **Cmd+F request body / scripts içinde arama** — EditorPane geçince geldi ✓ (CodeMirror built-in). Settings'te shortcut listesinde belirt.
- [ ] **Sidebar tree içinde fuzzy filter (filter expansion preserve)** — şu anki search FTS5 ile yapıyor ama tree expansion'ı bozuyor; "filter" modu eklemek (expand'leri koru, sadece eşleşmeyenleri gizle).

### Tabs
- [ ] **Tab reorder via drag** — TabBar drag-drop ile tab sırasını değiştir.
- [ ] **Tab pin** (sağ-tık → Pin). Pinned tab'lar dirty-check'siz korunur.
- [ ] **Right-click tab → Close All / Close Others / Close to the Right**.
- [ ] **Tab tooltip → full URL + method** (uzun adlarda hangi request olduğunu görmek).
- [ ] **Drag URL onto tab bar / workbench → openAdhoc** (browser address bar pattern).

### Response panel
- [ ] **Copy response body** (button, tek tık → clipboard).
- [ ] **Download response body** (`Save…` button, response'u dosyaya yaz; binary için kritik).
- [ ] **Response image preview** — `Content-Type: image/*` ise base64 inline preview.
- [ ] **Response HTML preview** — sandboxed iframe (no JS).
- [ ] **Response size warning** — >10 MiB altında küçük bir "truncated at 50 MiB" badge'i şu an silent.
- [ ] **JSON path picker** — Tree view'da bir node'a tıkla → JSONPath (`$.user.items[0].id`) kopyala. Reqost'taki request chaining syntax'ı ile uyumlu olacak şekilde.
- [ ] **Test result expand** — bir test'i tıklayınca `actual` vs `expected` diff göster.
- [ ] **Console clear / filter (errors only / search)** — şu an her şey akıyor.

### Request panel
- [ ] **Send & Save** (Cmd+Shift+Enter), **Send & Download** (response → file).
- [ ] **Pre-request "Try" / Test script "Try"** — gerçek request yollamadan sadece scripti çalıştır (hızlı debug).
- [ ] **Path variables editor** (Postman pattern): URL `/users/:id` veya `/users/{id}` yazınca otomatik bir Path Variables alt-sekmesi.
- [ ] **Per-tab variable override** — bir request için sadece o tab içinde aktif environment override.
- [ ] **Body line-wrap toggle** — CodeMirror default wrap'i kapatma seçeneği uzun JSON için.
- [ ] **Recent URLs autocomplete** — URL bar'a yazarken history'den fuzzy öner.
- [ ] **Request → "Reset to last save"** — dirty edit'i geri al.

### Sidebar / collection
- [ ] **Item "Copy ID"** — sağ-tık menüsüne ekle (chaining ref yazarken faydalı).
- [ ] **"Move to workspace…"** — sağ-tık menüsünde target workspace seç.
- [ ] **Sidebar collapse to icon-only** — daha geniş workbench için.

### Settings / theme
- [ ] **System theme follow** — şu an light/dark var, OS preference takip yok.
- [ ] **Font size + family setting** — accessibility + retina dışı ekranlar.
- [ ] **Keyboard shortcuts cheat sheet** — Settings'te modal: "Cmd+K palette, Cmd+P quick switch, Cmd+Enter send, …".
- [ ] **Workspace export/import** — tüm workspace'i .zip'e (collection.json + environments.json + plugins/) + geri yükleme.

### Network / protocol
- [ ] **SOCKS5 proxy** — şu an sadece HTTP/HTTPS. `http.Transport.DialContext` ile ekle.
- [x] **Custom CA trust** — Settings'te "Custom CA bundle (PEM)" path; sistem rootlarına eklenir, `x509.SystemCertPool() + AppendCertsFromPEM`, sonuç path-keyli cache'lenir.
- [ ] **Request retry button** (failed response'tan sonra direkt yeniden gönder).
- [ ] **Send timing history graph per request** — küçük sparkline son 10 send'in `totalMs`'i.
- [ ] **Response truncation banner** — 50 MiB cap'e çarpınca silent → görünür mesaj.

### Editor / coding ergonomics
- [ ] **JSON inline validation** — CodeMirror linter ile error squiggle (lang-json zaten parse ediyor, sadece lint extension ekle).
- [ ] **Body "Format JSON" / "Minify" button**.
- [ ] **XML/HTML pretty button** for response.
- [ ] **Snippets / templates** — kullanıcının kayıtlı snippet'leri body'e dropdown ile.

### Plugin ecosystem
- [ ] **Plugin marketplace stub** — Settings'te "Discover plugins" listesi (placeholder, GitHub topic ile).
- [ ] **Plugin `console.log` → in-app console** — şu an goja'nın stdout'una gidiyor, görünmez.
- [ ] **Plugin permission model** — `manifest.json` ile network/fs/timer izinleri.

### Mock / Design
- [ ] **Mock server log panel** — gelen request'leri DesignPanel altında listele.
- [ ] **Mock server CORS headers default-on** — frontend dev için ön gereksinim.
- [ ] **OpenAPI spec validation** — kaydetmeden önce sözdizimi hataları işaretle.

### CI / build / release
- [ ] **CI'a `go test` + `go vet` + `vue-tsc` adımları**.
- [ ] **`v*` manuel tag push'unda release açan ayrı job**.
- [ ] **Universal macOS binary** (arm64+amd64 lipo).
- [ ] **AppImage (Linux), .pkg (macOS), MSI (Windows)** native installer'lar.
- [ ] **macOS notarize, Windows code sign**.

## G — Postman + Insomnia detaylı parite (sıralı yapılacak)

Çekirdek + UX gap'lerinden sonra **bu listedekiler** ürünü "günlük kullanılan herhangi bir Postman/Insomnia ekranıyla aynı" hissettirir. Numaralandırma uygulama sırasıdır — bağımlılıklar takip edilebilsin.

### G1 · Workbench layout (çoğu Pri-1)
- [x] **Request/response drag-resize + collapse** — split bar, 50/50/req-only/res-only toggle, localStorage persist (bu seansta yapıldı).
- [ ] **Horizontal split toggle** — top-bottom yerine yan yana (yaygın ultra-wide kullanıcısı tercihi).
- [ ] **Sidebar collapse to icon-only** — workbench için daha geniş alan.
- [ ] **"Open response in new window"** — ayrı Wails window, çok protokol takibi için.
- [ ] **"Pop request out"** — request'i ayrı pencerede açma.
- [ ] **Distraction-free / Zen mode** — sadece URL bar + body.

### G2 · Request body & advanced fields
- [x] **Raw body sub-type dropdown** — JSON/XML/HTML/JavaScript/Text + auto Content-Type.
- [x] **multipart/form-data per-part Content-Type** — her form satırında "part C-T" alanı; backend `mw.CreatePart` ile MIME header.
- [x] **Binary body type** — `application/octet-stream`, file path (Go side `os.Open` streamed).
- [ ] **MessagePack body** (modern ML API'leri).
- [ ] **"Sign body" hook** — body hash'i header'a otomatik (HMAC için).
- [x] **Path variables editor** — `:id` / `{id}` algılanır, Params alt-sekmesinde ayrı bölüm.
- [ ] **Form file content-type override** per-field.
- [ ] **Body gzip / deflate / br compress before send** opsiyonu.

### G3 · Auth genişletme
- [ ] **AWS Signature v4** — access key/secret, region, service.
- [x] **Digest Auth** — MD5 + SHA-256, qop=auth, transparent 401 retry.
- [ ] **OAuth 1.0a** (legacy ama Twitter v1.1, Trello vs hala kullanır).
- [x] **JWT Bearer** — HS256/384/512 WebCrypto, claim editor, auto-stamp `iat`.
- [ ] **Hawk** (legacy).
- [ ] **NTLM / Kerberos** (kurumsal).
- [ ] **Akamai EdgeGrid**.
- [ ] **Bearer "Add to" toggle** — Header vs Query Param vs Cookie (Postman pattern).
- [ ] **API Key "Add to" Header / Query Param**.
- [ ] **OAuth 2.0 token cache UI** — geçerli token'ı göster, expire'a kalan süre, manuel sil/refresh.

### G4 · Pre-request / Test scripts
- [x] **"Try" düğmesi (gerçek request olmadan run)** — ExecService.TryPreScript/TryTestScript.
- [x] **Console.log → Test Results console paneli** — Try sonrası logs UI'da görünür.
- [x] **Test snippet dropdown** — 11 hazır snippet (status, jsonBody, response time, save token, basic auth, etc).
- [ ] **Visual test results bar/chart** — pass/fail oran, response-time histogram.
- [ ] **Workflow scripts** — folder-level OR collection-level pre/post (zaten tree.context_json var, scripts kısmı eklenmesi gerek).
- [ ] **pm.cookies bridge** — gerçek cookie jar'a okuma/yazma (şu an stub).
- [ ] **pm.iterationData** — runner data file ile beraber.
- [ ] **Sandbox `require()`** sınırlı modüller (`crypto`, `uuid`, `lodash` whitelist).
- [ ] **chai extra** — `.deep`, `.respondTo`, `.throw`.

### G5 · Variables / Environments
- [ ] **Initial value vs Current value** (Postman pattern: initial committed, current local-only — vault ile uyumlu).
- [ ] **5 scope katmanı**: Global, Collection, Folder, Request, Environment — explicit precedence dropdown'u.
- [x] **Variable inspector** — env source + active/inactive + found/missing + dynamic helpers + response refs.
- [ ] **"Find usage"** — bir variable'ın hangi request'lerde kullanıldığını listele.
- [ ] **Quick switcher (Cmd+Shift+E)** — env hızlı değiştirme.
- [x] **Per-tab variable override** — Settings subtab'inde key/value editor; activeVars üstüne shadow.
- [ ] **Variable history** — son N değer (debug için).
- [ ] **Environment template sharing** — JSON export'ta gizli alanları opsiyonel maskele.

### G6 · Sidebar / collection ergonomics
- [ ] **Multi-select** (Shift-click + checkbox) → toplu sil / taşı / export.
- [ ] **Tag / label** per item (color chip) + sidebar filter.
- [x] **Star / favorite** — localStorage'da Set; filter-bar'da ★ toggle + tree row badge.
- [ ] **Folder color** (görsel ayırt etmek için).
- [ ] **Custom icon per folder** (emoji veya SVG dropdown).
- [x] **Filter by method** — sidebar üstünde renkli method chip'leri.
- [ ] **Recently used pseudo-folder** (top 10).
- [x] **"Copy ID" / "Copy reference path"** — sağ-tık menüsünde ikisi de var.
- [ ] **"Move to workspace…"** — sağ-tık → target workspace.
- [ ] **Bulk rename** (regex find/replace).
- [ ] **Sort options** — alphabetical, last-used, manual order.

### G7 · Tabs
- [x] **Tab drag-reorder** — pin grupları içinde, pin sınırını koruyor.
- [x] **Tab pin** — Postman tarzı kalıcı tab, dirty-bypass + sol grup.
- [x] **Right-click → Pin / Close Others / Right / All** (pinleri korur).
- [x] **Full URL + method tooltip** uzun adlarda.
- [x] **Tab restore on launch** — açık tabs + active + pin localStorage.
- [x] **Drag URL → tab bar** — `text/uri-list` / `text/plain` URL'leri openAdhoc.
- [x] **Cmd+1..9 tab switch shortcut**.

### G8 · Response panel
- [x] **Copy response body** button (one-click clipboard).
- [x] **Save response to file** (binary için kritik).
- [ ] **Response visualizer** — Postman'in `pm.visualizer.set(template, data)` ile custom HTML render.
- [x] **Image preview** (`image/*` content-type → data URL).
- [x] **HTML preview** (sandbox iframe, JS off, srcdoc).
- [x] **PDF preview** (`<object type="application/pdf">`).
- [ ] **Diff with previous response** (response history'den seç → side-by-side).
- [x] **JSON path picker** — tree node tıkla → `$.foo.bar[0]` clipboard, toast feedback.
- [x] **Search in response headers** — filter input.
- [x] **50 MiB truncation banner** — Body sekmesinde uyarı.
- [ ] **Response time sparkline** son N send.
- [ ] **Response size warning** (5 MB üstünde "büyük response" badge).
- [x] **Status code description** — full HTTP phrase table + class hint tooltip.
- [x] **Decode base64 body** quick action — Body view bar'da 🔓 Base64 toggle.

### G9 · Send actions
- [x] **Send & Save** (Cmd+Shift+Enter).
- [x] **Send & Download** (response → file).
- [x] **Send N times** — Send dropdown: 5/10/N… avg+ok/fail logs.
- [ ] **Send All in folder (parallel)** — şu an seq runner var.
- [x] **Send button dropdown** — Send & Save / Send N times / Save response.
- [x] **Retry button** (response bar'da ↻).
- [ ] **"Send to background"** (uzun response'lar için).
- [ ] **Schedule send** (cron veya delay).

### G10 · Cookies tab
- [x] **Manual add / edit / delete cookie**.
- [ ] **Domain-aware cookie list** (sadece bu URL'in göndereceği değil tüm jar).
- [ ] **Cookie import** Netscape format (cURL `-b cookies.txt`).
- [ ] **Cookie export** clipboard / file.

### G11 · Runner (Newman parite)
- [x] **Iterations (`-n 5`)** — CLI runner.
- [ ] **Data file (`-d data.csv` / `.json`)** — iteration başına bir row → variables.
- [x] **Delay between requests** (`--delay 500`).
- [x] **Bail on first failure** (`--bail`).
- [x] **Folder filter** (`--folder Auth`).
- [ ] **Reporters: html, allure** (junit + json + text var).
- [x] **`--insecure` flag** (verify SSL off CLI'da).
- [ ] **Runner progress UI** — şu an basit; per-iteration log + ortalama / p95 metrik.

### G12 · Mock server iyileştirmeleri
- [ ] **Request log panel** — gelen request'leri DesignPanel altında listele (canlı).
- [ ] **CORS headers default-on** (Access-Control-Allow-Origin: *).
- [ ] **Latency simulation** — `--delay 200ms` veya range.
- [ ] **Conditional response routing** (header/path/query match → farklı example).
- [ ] **Stateful mode** — last request memory (örn. POST sonrası GET dolar).
- [ ] **Multiple examples per endpoint, picker UI**.
- [ ] **Mock server save URL clipboard**.

### G13 · API Design
- [ ] **OpenAPI sözdizimi validation** — kaydetmeden lint.
- [ ] **Schema preview panel** (left=editor, right=rendered docs).
- [ ] **"Send request from spec"** — operation'a tıkla, sağdaki Workbench'te taze tab.
- [ ] **Import to Collection from spec** (zaten internal/openapi var; UI'dan tek-tık).
- [ ] **OpenAPI versioning** (v1, v2 dosyaları).
- [ ] **AsyncAPI desteği** — WebSocket/SSE event spec.

### G14 · Plugin sistemi iyileştirmeleri
- [ ] **Manifest.json + permission model** — `network`, `fs`, `timer` izinleri.
- [x] **Plugin console** — Settings → Plugins → "Console" toggle; `plugin:console` Wails event'i her plugin'in `console.{log,info,warn,error}` çağrısını level-renkli olarak akıtır.
- [x] **Plugin reload düğmesi** — Refresh + Reload binding; disk'e yeni `.js` koyup tek tık.
- [ ] **"Discover plugins" listesi** — GitHub topic `reqost-plugin` ile.
- [ ] **Plugin context API** — `pm.environment`, `pm.cookies`, `pm.request`, `pm.response` plugin'lere expose.
- [ ] **Custom auth provider** API — plugin yeni bir AuthType register edebilsin.
- [ ] **Plugin per-workspace toggle**.

### G15 · Workspaces / collaboration
- [x] **Workspace export/import** — `.zip` (collection.json + environments.json + manifest.json); sidebar header menüsünden tetiklenir.
- [ ] **Workspace activity log** — son N create/delete/move/save (kim, ne, ne zaman; tek kullanıcı için bile undo'ya temel).
- [ ] **Workspace settings panel** — default request settings (timeout, redirect vs.) per workspace.
- [ ] **Workspace switcher shortcut** (Cmd+Shift+W).
- [ ] **Cloud sync hook** (opsiyonel, future: S3/Git backed).
- [ ] **Workspace-level secrets store** (vault, OS keychain backed).

### G16 · Git sync UI
- [x] **"Bind to Git directory…"** workspace dropdown'unda — her workspace satırında ⎇ butonu.
- [x] **Status badge** — title bar'da `branch · N change(s)` (clean: yeşil, dirty: turuncu).
- [x] **Branches modal** — switch dropdown'u + "+ New branch".
- [x] **Snapshot & commit** — Export + Commit tek tıkla; ayrı "Commit current tree".
- [x] **Pull / Push / Fetch UI** — modal'da düğmeler + title bar badge'inde `↑N ↓N` sayaçları; pull dirty tree'de blocklu, push upstream yoksa otomatik `-u origin <branch>`.
- [x] **Set remote (origin)** — "Add origin…" / "Change…" prompt → `git remote add/set-url`.
- [ ] **Conflict UI** (3-way merge collection.json üzerinde).

### G17 · Network detay
- [x] **SOCKS5 proxy** — `socks5://[user:pass@]host:port` via `x/net/proxy`.
- [ ] **Custom CA trust store** path veya inline PEM.
- [ ] **Network throttling** (slow 3G / fast 3G simulation).
- [ ] **DNS over HTTPS** opsiyonu.
- [ ] **HTTP/2 + HTTP/3 toggle** (default auto).
- [ ] **WebSocket subprotocols** (Sec-WebSocket-Protocol header).
- [ ] **WebSocket binary frame** view (hex + decode dropdown).
- [ ] **WebSocket ping/pong** otomatik + manuel.
- [ ] **WebSocket auto-reconnect** + backoff.
- [ ] **WebSocket subscribe replay** — connect'te kuyrukta bekleyen mesajları gönder.

### G18 · gRPC iyileştirmeleri
- [ ] **gRPC `.proto` import** (reflection olmayan sunucular için).
- [ ] **gRPC metadata editor** (headers eşdeğeri).
- [ ] **gRPC TLS + mTLS client cert** (zaten httpclient'ta var, gRPC'ye taşı).
- [ ] **gRPC stream replay** (önceki streaming response'u kaydedip mock'ta kullan).

### G19 · UI / theme / a11y
- [x] **System theme follow** — `light` / `dark` / `system` segmented; `prefers-color-scheme` listener.
- [x] **Font size setting** — Settings slider 10–20 px (CSS var `--app-font-size`).
- [x] **Keyboard shortcuts cheat sheet** — Settings → modal + Cmd+/ jumps to Settings.
- [ ] **Full keyboard navigation** (focus ring, no mouse).
- [ ] **High contrast theme**.
- [x] **Reduced motion** — `@media (prefers-reduced-motion)` global guard.
- [ ] **Localization** — TR + EN.
- [ ] **Date/time format** — locale-aware (RFC, ISO, relative).
- [x] **Status bar** — workspace · env · proxy · plugin count · tab count.

### G20 · Reliability / quality of life
- [ ] **Crash report opt-in** (telemetri yok varsayılan).
- [ ] **Auto-save dirty tabs** (debounced).
- [ ] **Unsaved changes warning on quit**.
- [ ] **Disk corruption recovery** (SQLite WAL replay + DB integrity check).
- [ ] **Offline mode banner** (network yok → auto-update / OAuth disable).
- [ ] **Settings backup/restore**.

### G21 · Documentation / sharing
- [ ] **Auto-doc from collection** (Markdown + HTML).
- [ ] **Public share link** (read-only static page export).
- [ ] **Embed widget** (web sayfasına iframe).

### G22 · Performance test mode (Postman v11 yok ama Insomnia + Hoppscotch peek)
- [ ] **Iteration concurrency** (`-c 50` parallel sessions).
- [ ] **Ramp-up profile** (1 → 100 user 30s).
- [ ] **p50/p95/p99 metrik UI**.
- [ ] **k6 script export**.

### G23 · Integrations
- [ ] **Slack notification** — collection runner sonucu webhook.
- [ ] **Webhook test** (gelen tarafı dinleme, ngrok-style tunnel'a hook).
- [ ] **Browser extension** — sayfada cURL kopyaladığında otomatik tab açma (uzak ihtimal).
- [ ] **VSCode extension** stub.

### G24 · Power user
- [ ] **Command palette parametre prompt** (cmd → "Set env: ___").
- [ ] **Macros / kayıtlı aksiyon sequence**.
- [ ] **Workspace template** (boş workspace + default folder/scripts).
- [ ] **Settings → JSON import/export** (ayar paylaşımı).

### G25 · Niş ama büyük etki yaratan ufak şeyler
- [ ] **Request "description" markdown render** subtabe.
- [ ] **TODO/note comment per-line in scripts** (CodeMirror linter integration).
- [ ] **Request copy → paste yapıldığında format korunur** (Postman v2.1 JSON clipboard).
- [ ] **URL bar'da `?key=value&...` highlight + paste'te otomatik params'a parse**.
- [ ] **Drag environment from sidebar onto URL bar** → o env'in `{{baseUrl}}` insert.
- [ ] **"Use this response as input for next request"** (chaining wizard).
- [ ] **Quick variable extract** — response body'de bir değeri seç → "Save as variable" pop-up.
