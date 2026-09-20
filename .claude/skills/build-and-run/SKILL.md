---
name: build-and-run
description: Use when you need to build, run, or hot-reload the agentsview backend (Go) and/or frontend (Svelte/Vite) on Linux or Windows — first-time setup, dev loop, tests, and CGO/toolchain requirements.
---

# Build and run agentsview

Full source is Go backend (`cmd/agentsview`, `internal/`) + Svelte frontend
(`frontend/`), optionally wrapped by a Tauri desktop app (`desktop/`). SQLite
FTS5 requires **CGO**, so a C compiler is mandatory on every platform.

Canonical reference: `README.md` → `## Development`. This skill is the
condensed, task-oriented version of that section — re-read the README if a
step here fails or looks stale.

## Prerequisites (all platforms)

- Go 1.27+, CGO enabled
- Node.js 24.11+ and npm (`frontend/`)
- C compiler on PATH (gcc on Linux, MSYS2 UCRT64 gcc on Windows, Xcode CLT on macOS)
- [air](https://github.com/air-verse/air) for backend live reload — `make air-install`

## Linux (and macOS/WSL via `make`)

First-time build:

```bash
git clone https://github.com/kenn-io/agentsview.git
cd agentsview
make build          # npm ci + frontend build + embed + compile ./agentsview
```

Full-stack dev loop (two terminals):

```bash
make dev            # Terminal 1: Go API on :8080, rebuilds on .go edits (air)
make frontend-dev   # Terminal 2: Vite dev server (usually :5173), proxies /api
```

Backend-only, no hot reload: `make build && ./agentsview serve`, open
`http://127.0.0.1:8080`.

Use an isolated data dir while hacking (default is `~/.agentsview/`):

```bash
mkdir -p tmp/dev-data
AGENTSVIEW_DATA_DIR="$PWD/tmp/dev-data" make dev
```

Desktop wrapper (Tauri): `cd desktop && npm ci && npm run tauri:dev` (or
`make desktop-dev`). Needs Rust (`rustc`, `cargo`) in addition to the above.

Tests and lint:

```bash
make test           # Go tests (CGO_ENABLED=1 -tags "fts5")
make lint           # golangci-lint + NilAway
make e2e            # Playwright E2E (frontend)
```

Frontend-only checks (from `frontend/`): `npm ci`, then `npm run check` and
`npm run test`.

Other useful `make` targets: `make frontend` (build SPA only), `make clean`,
`make install` (installs to `~/.local/bin`), `make help` (full list).

## Windows (native, no WSL)

`make` is not required or used. Requires [MSYS2](https://www.msys2.org/)
installed at `C:\msys64` (the build scripts hardcode that path).

Usual build-and-run loop, from the repo root in PowerShell:

```powershell
.\scripts\build-windows.ps1 -Restart
```

This script:
1. Builds the frontend (`npm run build` → `internal/web/dist`) unless `-BackendOnly`.
2. Bootstraps the pinned MSYS2 UCRT64 gcc toolchain into `.toolchain/ucrt64-gcc14`
   via `scripts/setup-windows-toolchain.ps1` (only runs once; safe to re-run).
3. Cross-invokes `scripts/build-windows-msys.sh` under MSYS2 bash to compile
   `agentsview.exe` with `CGO_ENABLED=1 -tags fts5`.
4. Stops any running server and starts `.\agentsview.exe serve --no-browser`.

Open `http://127.0.0.1:8080`.

Flags:
- `-BackendOnly` — skip the frontend build (fast loop for Go-only changes)
- `-FreshNpm` — run `npm ci` before the frontend build (after `package-lock.json` changes)

`-Restart` targets the repo-root binary only; an `agentsview` elsewhere on
PATH (e.g. `~/.agentsview/bin`) is not auto-updated.

Isolated data dir:

```powershell
$env:AGENTSVIEW_DATA_DIR = "$PWD\tmp\dev-data"
New-Item -ItemType Directory -Force -Path $env:AGENTSVIEW_DATA_DIR | Out-Null
.\scripts\build-windows.ps1 -Restart
```

Frontend hot reload on Windows: run `cd frontend; npm run dev` in a second
terminal while `agentsview.exe serve` listens on `127.0.0.1:8080` (or just use
WSL + `make frontend-dev` for the usual Vite dev-proxy flow).

Lower-level scripts (only touch directly when debugging the toolchain itself):
- `scripts/setup-windows-toolchain.ps1` — installs pinned MSYS2 UCRT64 gcc 14 into `.toolchain/`
- `scripts/build-windows-msys.sh` — the actual `go build` invocation with `CC`/`CXX` pointed at that toolchain

Desktop wrapper (Tauri) on Windows also needs Rust; see `desktop/README.md`
(`npm run tauri:dev`, `npm run tauri:build:windows` for the NSIS installer).

## Gotchas

- Forgetting `-tags fts5` (or CGO) produces a binary that fails at runtime on
  SQLite FTS queries, not at compile time in all cases — always build through
  `make build`/`make dev`/the Windows scripts rather than a bare `go build`.
- `make dev` and `.\scripts\build-windows.ps1` both bind port 8080 by default;
  stop one before starting the other, or set `AGENTSVIEW_DATA_DIR` +
  a different port to run side by side.
- More build tags, CI, and dependency rules: `docs/agents/build.md`.
