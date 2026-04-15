# Felicity Battery SOC in the Zsh Prompt

Shows the battery's State of Charge directly in the terminal prompt, updated automatically every 5 minutes.

## What was done

Two files were modified:

### `~/.p10k.zsh`

**1. Added `battery_soc` to the right-side prompt elements**, immediately before the `time` segment:

```zsh
typeset -g POWERLEVEL9K_RIGHT_PROMPT_ELEMENTS=(
  ...
  battery_soc   # felicity battery state of charge
  time          # current time
  ...
)
```

**2. Added two functions** inside the p10k config:

- `_felicity_fetch_soc` — calls `GET /api/status`, extracts `soc` from the JSON response, and writes it to `/tmp/felicity_soc_cache`.
- `prompt_battery_soc` — the segment function Powerlevel10k calls on every prompt render. It reads the cache file and, when the cache is missing or older than 5 minutes, fires `_felicity_fetch_soc` in the background so the prompt never blocks waiting for the network.

### `~/.zshrc`

Added the `FELICITY_API_URL` environment variable pointing to the API:

```zsh
export FELICITY_API_URL="http://localhost:8080"
```

Change this to your VPS address if the service is running remotely.

---

## Why each decision was made

### Non-blocking background fetch

Calling an HTTP API on every prompt render would make the shell feel laggy. Instead, `prompt_battery_soc` only reads from a local cache file — a near-instant operation. The actual network request is triggered asynchronously in a subshell (`(_felicity_fetch_soc &>/dev/null &)`) only when the cache is stale. The next prompt after the fetch completes will show the fresh value.

### 5-minute cache TTL

The Felicity battery pushes data to the cloud every 5 minutes. Refreshing more often would fetch the same value anyway, and refreshing less often would show stale data. The TTL matches the source's own update cadence, checked with `find -mmin -5`.

### Color thresholds

Colors follow the reference intervals documented in `felicity-battery/API.md`:

| SOC | Color | Meaning |
|-----|-------|---------|
| ≥ 50% | Green | Normal |
| 20–49% | Yellow | Attention |
| < 20% | Red | Critical |

### Placement next to `time`

The SOC is contextual, glanceable information — the same category as the current time. Placing them side by side groups them visually and keeps the left prompt clean for working context (directory, git status, etc.).

### `grep` instead of `jq` for JSON parsing

`jq` is available on this machine (it's in the zsh plugins list), but `grep -o` is simpler for extracting a single numeric field and avoids a dependency in a shell startup file. The pattern `"soc":[0-9.]*` is unambiguous given the API response structure.

---

## Result

After `source ~/.zshrc`, the prompt shows something like:

```
... 🔋 26%  09:51:02
```

The segment is invisible when the cache doesn't exist yet (first shell after boot) and reappears once the first background fetch completes (~3 seconds).

---

## Configuration

| Variable | Default | Purpose |
|----------|---------|---------|
| `FELICITY_API_URL` | `http://localhost:8080` | Base URL of the felicity-battery HTTP service |

Cache file: `/tmp/felicity_soc_cache` — cleared on reboot, rebuilt automatically on next prompt.
