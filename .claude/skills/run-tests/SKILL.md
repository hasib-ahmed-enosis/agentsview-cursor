---
name: run-tests
description: Use when you need to run agentsview's test suites — Go unit/integration tests, benchmarks, lint, or frontend unit/E2E tests — on Linux or Windows, including which ones need Docker.
---

# Run agentsview tests

Companion to [build-and-run](../build-and-run/SKILL.md). Covers *how to run*
each test suite; see that skill for CGO/toolchain setup if a build step fails
first.

## Go tests (backend)

Requires CGO enabled and the `fts5` build tag — always run through `make` or
pass those flags explicitly, never a bare `go test ./...`.

```bash
make test           # full suite: CGO_ENABLED=1 go test -tags fts5 ./... -v
make test-short     # fast subset: adds -short
make test-evalingest # quarantined evalingest-tagged tests (internal/server only)
```

Run a single package or test directly (same flags `make test` uses):

```bash
CGO_ENABLED=1 go test -tags fts5 ./internal/db/... -run TestName -v
```

`GO_TEST_P` controls parallel test-binary fan-out (default 4); override with
`GO_TEST_P=` for the Go default.

### Windows

No `make` on Windows. Set up the pinned MSYS2 UCRT64 toolchain once (see
[build-and-run](../build-and-run/SKILL.md#windows-native-no-wsl)), then run Go
tests with the same env `build-windows-msys.sh` uses:

```powershell
$env:PATH = "$PWD\.toolchain\ucrt64-gcc14\ucrt64\bin;C:\Program Files\Go\bin;$env:PATH"
$env:CC = "$PWD\.toolchain\ucrt64-gcc14\ucrt64\bin\gcc.exe"
$env:CGO_ENABLED = "1"
go test -tags fts5 ./... -v
```

Or just run tests from WSL/a Linux shell against the same checkout, using the
Linux commands above — simpler when you don't need a native Windows binary.

## Integration tests (Docker required)

Each backend has an `-up`/`-down` pair (starts/stops the service via
`docker-compose.test.yml`) and a `test-<backend>` target that starts the
service automatically if not already running:

```bash
make test-postgres    # PostgreSQL 16 (pgvector/pgvector:pg16), port 5433
make test-clickhouse  # ClickHouse 25.8, native 19000 / HTTP 18123
make test-ssh         # sshd container; copies a test key to testdata/ssh/test_key
make test-s3          # S3-compatible (rustfs) via testcontainers — only needs Docker running, no compose service
```

CI variants (`test-postgres-ci`, `test-clickhouse-ci`, `test-ssh-ci`) assume
the service is already up (started as a CI service container) and skip the
`-up`/wait step — don't use these locally unless you started the container
yourself.

Tear down explicitly when done: `make postgres-down`, `make clickhouse-down`,
`docker compose -f docker-compose.test.yml down sshd`.

These need a working Docker daemon on both Linux and Windows (Docker Desktop
with WSL2 backend on Windows); there is no native-Windows-without-Docker path
for them.

## Benchmarks

```bash
make bench-backends  # SQLite vs DuckDB vs PostgreSQL read-query comparison; needs Docker (testcontainers PG)
make bench-pg-usage  # opt-in PG usage benchmarks against the pinned PG16 service
make bench-gate      # hot-path benchmarks for before/after local comparison
make bench-gate-config # prints the sample/iteration config bench-gate used, to reuse for a baseline run
```

`bench-backends` fixture size: `BENCH_BACKENDS_SESSIONS` /
`BENCH_BACKENDS_MESSAGES_PER_SESSION` env vars.

## Lint / static analysis

```bash
make lint      # golangci-lint (auto-fix) + NilAway + config/sql/timing-budget checks
make lint-ci   # same, no auto-fix (what CI runs)
make vet       # go vet -tags fts5 ./...
```

`golangci-lint` and `prek` must be installed first (`make lint-tools`,
`make install-hooks`); `make nilaway` runs just the NilAway pass.

## Frontend tests (from `frontend/`)

```bash
npm ci                # once, or after package-lock.json changes
npm run check         # svelte-check (types)
npm run test          # vitest (vp test run)
npm run e2e           # Playwright E2E, run from frontend/
```

Playwright E2E can also be driven from the repo root against a real backend:

```bash
make e2e         # full Playwright suite
make e2e-duckdb  # focused smoke tests against a DuckDB-backed server, chromium only
```

Both `precheck` (`npm run precheck`, runs automatically via lint-staged/hooks)
and `frontend/AGENTS.md`'s checklist expect `npm run check` and `npm run test`
to pass before a frontend change is considered done.

## Quick picks

- Changed only Go code, no new backend/infra: `make test-short`
- About to open a PR: `make test && make lint` (backend) + `npm run check && npm run test` (frontend, from `frontend/`)
- Changed a storage backend (Postgres/ClickHouse/DuckDB/SQLite): also run the matching `make test-<backend>`
- Changed UI behavior: `make e2e` or `make e2e-duckdb` for a faster pass
