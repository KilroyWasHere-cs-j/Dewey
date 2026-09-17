# Dewey — Feature List

A complete inventory of what Dewey does, organized by category. See [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) for implementation detail and [ADMIN_GUIDE.md](ADMIN_GUIDE.md) for operational detail.

## Ingest & Sorting

- Extension allowlist (`pdf`, `txt`, `doc`/`docx`, `xls`/`xlsx`, `csv`, `ppt`, `png`, `jpg`/`jpeg`) — anything else is rejected before processing.
- Content-sniffing so a file's actual bytes must match its declared extension.
- Windows PE / Linux ELF binary rejection, checked regardless of extension.
- PDF `/OpenAction` (embedded JavaScript trigger) rejection.
- SHA-256 hashing of every upload, verified round-trip on retrieval.
- Code128 barcode scanning on image uploads (via gozxing).
- Lua-based plugin pipeline for custom sorting/filtering logic, executed in salience order.
- Two-stage async pipeline: the client gets a fast `200 OK` right after validation + caching, while hashing, barcode scanning, plugin execution, and the permanent copy all happen in the background.
- Case metadata tracked per file: claim number, claimant name, date of injury, employer, adjuster, support contact, claim type, jurisdiction, policy number, and an ACTs system ID used as the linking key.

## Security

- IP allowlist ("known machines") gating every route, resolved via Go's `net/netip` (not gin's `ClientIP()`, which silently drops zone-qualified addresses from Podman's `pasta` networking).
- Link-local IPv6 addresses treated as host-equivalent and exempted from the allowlist check.
- Trusted proxies disabled (`SetTrustedProxies(nil)`) so `X-Forwarded-For`/`X-Real-IP` can't be spoofed to bypass the allowlist.
- Per-request access logging as an audit trail (`last_seen_at` bumped on every authorized request).
- Hard upload size ceiling enforced at the body-read level, not just multipart parsing.
- Global HTTP rate limiting (token bucket: refill rate + burst allowance).
- Files served with `nosniff` and forced-download headers so stored content can't be executed by a browser.
- Path-traversal protection on file retrieval/deletion routes.
- Lua plugin sandbox: only `base`, `string`, and `table` libraries opened; `os`, `io`, `math`, and stdlib-reconstruction globals (`load`, `loadstring`, `dofile`, `loadfile`, `require`, `getfenv`, `setfenv`) stripped out.
- Narrow, Go-implemented capability API (`http.get`, `http.post`, `files.read`, `files.write`) as the only way a plugin reaches the network or filesystem — network calls blocked from dialing loopback/private/link-local/unspecified IPs (checked at actual dial time, closing a DNS-rebinding gap); file calls confined to an isolated plugin scratch directory.
- MCP server path-traversal protection (`resolveSafePath`) mirroring the backend's own guard.

## Storage & Reliability

- Two-tier storage: a fast `cache/` directory for recent uploads and a permanent `store/` directory.
- Hourly daemon that zips `store/` into a timestamped archive under `backup/`.
- Hourly cache clear to keep the fast-path directory small.
- Soft deletes — files are flagged `is_deleted` in the database rather than the record being removed.
- Adaptive daemon tick interval — scales with active-user count and smoothed upload/retrieval rate (idle stays quiet, busy periods tick faster), clamped to a min/max range.
- Plugin hot-reload: the daemon hashes every plugin file each tick and only reloads if something actually changed, avoiding unnecessary sandbox rebuilds.
- Manual plugin reload on demand via `GET /admin/reloadPlugins`, without restarting the server.
- Data persistence via named Podman volumes (database, store, cache, backup, logs, plugin scratch) that survive pod recreation.
- Separately controllable data resets: `--reset-db`, `--keep-data`/`--wipe-data`, and `--clean-slate` for a guaranteed-consistent full wipe.
- `--app-only` deploy mode to redeploy just the backend/frontend without restarting MySQL or Prometheus (and without resetting their data/metrics history).

## Observability & Administration

- Prometheus metrics: uptime, memory/GC stats, file counts, upload rejections by reason, upload size distribution, barcode success/failure, plugin run/error counts, DB errors, cache-clean cycle count, and more.
- Structured, rotating (daily) logger with distinct levels (`Debug`, `Info`, `Warn`, `Fatal`) plus styled startup output (`Ok`, `Section`, `Banner`).
- Svelte-based admin portal:
  - File browser
  - Machine (IP allowlist) management — list, add, remove
  - Analytics/graphing dashboard with configurable poll interval and history window
  - Configurable alert thresholds (RAM usage, retry count) and a Prometheus-unreachable banner
  - Dark mode, accent color, and chart color theme customization, saved per-browser
  - In-app documentation for both developers and administrators
- CLI (`dewey-cli`) live terminal metrics dashboard, polling every 5 seconds.
- CLI terminal markdown doc viewer (`dewey-cli docs readme|admin`), baked into the binary at build time.

## Extensibility — Plugin System

- Lua plugin system with a dynamic hook registry — a plugin attaches to a hook simply by defining a function with that hook's exact name.
- Five hooks: `OnUpload` (tag-only, can't veto), `OnFilter` (can rewrite the destination path), `OnDelete` (can veto the deletion via `error()`), `OnInit` (startup), `OnTick` (once per daemon tick).
- Salience-based ordering within a hook (higher runs first); each plugin runs in its own isolated Lua state so globals can't leak between plugins.
- A single plugin file can implement more than one hook.
- Shipped example plugins for `OnFilter`, `OnUpload`, `OnDelete`, and `OnTick`.
- (Planned) plugin validation.
- (Planned) self restart and cleanup.

## API

- RESTful HTTP API on port 8080 (Gin), covering health/version, upload, file listing/retrieval/deletion/move, cache admin, plugin reload, and machine allowlist management.
- JSON metadata retrieval alongside raw file streaming from the same retrieval route.

## CLI Tool (`dewey-cli`)

- Standalone Go client wrapping every backend route for scripting and development without hand-built `curl` requests.
- Commands for health, version, cache dump, machine list/add/delete, file list/get/get-meta/delete, upload (with arbitrary metadata fields), and a "what's my outbound IP" helper (`self_ip`).
- List-shaped responses render as aligned tables; everything else prints as indented JSON, colored red on a non-2xx status.
- Synthetic test-file generation (`create_docx`, `create_exe`, `create_pdf`) for exercising upload validation.
- Soak-test launcher (`soak_test`) that runs the load-simulation script inside the backend container.

## MCP Server (`dewey-mcp`)

- Standalone Go MCP (Model Context Protocol) server exposing Dewey and its Podman pod to AI model clients (Claude Desktop, Claude Code, etc.) over stdio.
- Read-only tools: `is_up`, `version`, `get_podman_health`, `get_podman_containers`, `get_podman_container_logs`, `read_file`.
- Destructive tools: `restart_podman_container`, `create_file`, `move_file`, `delete_file`.
- Path-traversal-safe file operations, mirroring the backend's own guard.
- Bundled and cross-compiled by `package.sh` alongside `dewey-cli`.

## Testing

- **Go unit tests** (`go test ./...`) covering file validation (PE/ELF sniffing, extension/content-type matching) and the plugin system (hook registration, loading, salience ordering, `OnDelete` veto, sandbox restrictions, capability API round trips and restrictions). Run in CI on every push/PR.
- **BITs (Built-In Tests)** — a black-box HTTP suite (`test_suite.py`) run automatically on every server startup (non-blocking, non-fatal on failure) and runnable manually against any base URL. 15 checks covering endpoint health, upload validation (JSON and multipart, error cases, disallowed extensions, ELF/PDF-JS rejection, path traversal), hash integrity, duplicate handling, on-disk store verification, metadata retrieval, cataloging, and deletion.
- **load_test.sh** — a quick fixed-size burst of uploads plus a retrieval pass, for sanity-checking a deploy.
- **soak_test.sh** — sustained, multi-user, diurnally-shaped traffic simulation over a configurable duration, with time compression for fast iteration and a CSV metrics log for long-run analysis. Simulates distinct users via loopback IP aliasing so allowlist/active-user logic is exercised realistically.

## Deployment & Packaging

- `deploy.sh` — creates the Podman pod and starts all four containers (MySQL, Prometheus, backend, frontend) with persistent named volumes.
- `package.sh` — builds backend/frontend images against a running pod and exports the whole thing (images + Kubernetes-style pod spec + a self-contained `run.sh`) as a portable `.tar.gz` bundle, no registry required. Also cross-compiles and bundles `dewey-cli` and `dewey-mcp`.
- Configurable pod resource limits (`DEWEY_POD_CPUS`, `DEWEY_POD_MEMORY`).
- Independent, composable data-reset controls, plus a single `--clean-slate` flag that keeps both sides (database and files) in sync when a full wipe is actually wanted.
- Non-interactive-safe defaults — a wipe never happens silently without a TTY unless explicitly flagged.

## Access Control Model

- Closed-network, IP-allowlist-based access — no per-user login or credential.
- Self-service management of the allowlist through the API or admin portal, once at least one machine is registered.
- Deliberate scope limitation: stops outside machines cold, but does not substitute for real authentication if the network trust boundary ever changes — documented as a known trade-off, not an oversight.

## Roadmap (Not Yet Implemented)

- Plugin validation
- Self restart and cleanup
