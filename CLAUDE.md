# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build -o felicity-battery .        # build binary
go fmt ./...                          # format (always run before committing)
go vet ./...                          # lint
CGO_ENABLED=0 go build -o /dev/null ./...  # CI-style compile check
```

No test suite exists yet. CI runs `go vet` + `go build` only.

**Run modes:**
```bash
# One-shot CLI
./felicity-battery -user EMAIL -pass PASS

# Watch mode (polls every 5 min, ANSI terminal dashboard + appends JSONL)
./felicity-battery -user EMAIL -pass PASS -watch

# HTTP API server (background poller + REST on :8080)
./felicity-battery -user EMAIL -pass PASS -serve :8080

# Optional flags: -device SN, -history FILE, -load WATTS (projection)
# Credentials also via: FELICITY_USER / FELICITY_PASS env vars
```

**Docker:**
```bash
docker compose build
FELICITY_USER=x FELICITY_PASS=x docker compose up
```

## Architecture

Pure stdlib Go, no external dependencies, no `go.sum`.

### Data flow

```
Felicity cloud REST API (shine-api.felicitysolar.com)
  └── api.go  →  client.getSnapshot()  →  BatterySnapshot   (raw *string fields)
        └── history.go  →  snapshotToRecord()  →  HistoryRecord  (typed SI units)
              ├── display.go  →  printBattery()   (CLI/watch mode only)
              ├── history.go  →  AppendHistory()  →  data/battery.jsonl  (JSONL, one line/record)
              └── server.go   →  serverState      →  HTTP handlers
```

### Key types

| Type | File | Purpose |
|------|------|---------|
| `BatterySnapshot` | `api.go` | Raw API response; all numeric fields are `*string` (many nullable) |
| `HistoryRecord` | `history.go` | Decoded, typed struct; SI units (V, A, W, %, °C, mV); written to JSONL and served by API |
| `serverState` | `server.go` | `sync.RWMutex`-guarded shared state between poller goroutine and HTTP handlers |
| `client` | `api.go` | HTTP client with token cache; re-logins after 25 days |

### Concurrency invariant

Only `runPoller` (in `server.go`) ever calls `client.getSnapshot()`. HTTP handlers only read `serverState` via `RLock` and read the JSONL file. No mutex needed on `client`.

### JSONL history (`data/battery.jsonl`)

One `HistoryRecord` JSON object per line, oldest-first. `readHistory()` reverses to newest-first before applying `offset`/`limit`. Malformed lines are silently skipped. Cell voltage sentinel `"32767"` means an unpopulated slot — `filterActive()` strips these before writing.

### HTTP API (server mode)

Routes registered in `StartServer()` (`server.go`):

| Route | Handler |
|-------|---------|
| `GET /` | `handleDashboard()` — serves embedded `dashboard.html` from `static.go` |
| `GET /api/status` | latest `HistoryRecord` as JSON; 503 until first poll succeeds |
| `GET /api/history` | JSONL history with `from`/`to`/`limit`/`offset` query params; newest-first |
| `GET /api/health` | poll health: `ok`, `last_poll`, `last_error`, `data_stale` |

CORS (`*`) applied via `corsMiddleware` wrapper. Full API reference in `API.md`.

### `static.go`

Embeds `dashboard.html` as a Go string literal (generated manually, not via `//go:embed`). Update both files together when changing the dashboard.

### Upstream API quirks

- TLS cert on `shine-api.felicitysolar.com` is self-signed → `InsecureSkipVerify: true` (intentional, documented with `//nolint`).
- Auth header format is `Bearer_<token>` (underscore, not space).
- `deviceSn` can be left empty — the API returns the first device on the account.
