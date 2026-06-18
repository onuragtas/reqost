# reqost

A high-performance desktop API client built for **very large Postman collections** (50k+ items). Native Go backend (Wails v3) with a Vue 3 + TypeScript frontend rendered in a system webview — no Electron, no browser CORS, no Postman account.

> Postman started feeling sluggish around 5k requests. reqost is what happens when you treat a collection as an indexed database instead of a giant JSON blob loaded into a JS app.

## Features

- **Lazy tree** — `collection.json` is parsed once into a SQLite index; children are fetched per-folder on demand. Heavy request content (body / headers / scripts) only loads when a request is opened.
- **Postman-compatible scripts** — pre-request + test scripts run in a goja sandbox with a pragmatic `pm.*` subset (`pm.test`, chai-style `pm.expect`, `pm.environment`, `pm.sendRequest`, `pm.response`, `console`).
- **Multi-protocol** — HTTP, **WebSocket** (live frame log), **gRPC** (server-reflection, no `.proto` needed), **GraphQL** body type.
- **Per-request settings** — timeout, follow-redirects, max-redirects, verify-SSL (tri-state Inherit / On / Off against global defaults).
- **Drag-and-drop** tree reorder & reparent.
- **OpenAPI / Swagger import** — JSON or YAML, merged under a folder.
- **Collection runner** — run a folder/collection sequentially, threading `pm.environment.set` across requests.
- **Native cookie jar** — `Set-Cookie` replayed on subsequent requests; no CORS.
- **Auto-update** — checks GitHub Releases on startup, installs in-place on confirm.
- **Server / Docker mode** — same backend can run headless (`task build:server`).

## Install

Grab the latest binary from [Releases](https://github.com/onuragtas/reqost/releases/latest):

| Platform | Asset |
|---|---|
| macOS (Apple Silicon) | `reqost-darwin-arm64.tar.gz` |
| macOS (Intel) | `reqost-darwin-amd64.tar.gz` |
| Linux (x86_64) | `reqost-linux-amd64.tar.gz` |
| Windows (x86_64) | `reqost-windows-amd64.zip` |

Each asset ships with a `.sha256` checksum.

### macOS

```bash
tar xzf reqost-darwin-arm64.tar.gz
# Gatekeeper: the binary is not notarized yet, strip the quarantine flag once.
xattr -dr com.apple.quarantine ./reqost
./reqost
```

### Linux

Requires GTK 4 + WebKitGTK 6 (Ubuntu 24.04+, Fedora 40+):

```bash
sudo apt install libgtk-4-1 libwebkitgtk-6.0-4    # debian/ubuntu
tar xzf reqost-linux-amd64.tar.gz
./reqost
```

### Windows

Extract the zip and run `reqost.exe`. SmartScreen will warn on first run (binary is not code-signed yet) — click *More info* → *Run anyway*.

### Auto-update

reqost checks GitHub Releases on startup and surfaces an update pill in the title bar when a newer version exists. Click **Install & relaunch** — the binary patches itself in place. Quit and reopen to pick up the new version.

If the binary lives in a system-owned path (e.g. `/usr/local/bin/`) the install will fail with a permission error; move it somewhere writable or re-download manually.

## Build from source

Prerequisites: Go 1.25, Node 20+, [Task](https://taskfile.dev).

```bash
git clone https://github.com/onuragtas/reqost
cd reqost

# Hot-reload dev (Wails CLI not required — Taskfile uses `go run`)
task dev

# Production build into bin/
task build

# Regenerate TS bindings after touching a Go service method signature
task bindings
```

Linux dev needs `libgtk-4-dev libwebkitgtk-6.0-dev pkg-config`. macOS needs Xcode CLT.

> First-time `npm run build` may fail with `Cannot find module '../lightningcss.<arch>.node'`. Run `task fix:lightningcss` (auto-detects the arch suffix) or `task frontend:build` which calls it for you.

## Architecture

See [`CLAUDE.md`](./CLAUDE.md) for a deep tour. TL;DR:

- **`internal/index`** — SQLite schema split: lightweight `tree` for the sidebar, heavy `detail` only on open, FTS5 for search. Re-imports merge incrementally so in-app edits survive a re-parse of the same collection.
- **`internal/httpclient`** — `net/http` + `httptrace` per-phase timing, shared cookie jar, two cached transports (secure / TLS-insecure), per-request timeout & redirect policy.
- **`internal/script`** — goja JS sandbox with the `pm.*` prelude.
- **`internal/update`** — GitHub Releases self-update via `minio/selfupdate`.

## Releasing

Push to `master` → CI auto-tags the next patch version, builds all 4 platforms, opens a GitHub Release with auto-generated notes + binaries + SHA256s. For a minor or major bump, push a tag manually (`git tag v1.1.0 && git push --tags`) or trigger the workflow manually with the `bump` input.

## License

MIT — see [LICENSE](./LICENSE).
