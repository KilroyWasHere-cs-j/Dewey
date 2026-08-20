# Dewey

A backend file sorting and storage service with a RESTful API, built for stable operation on modest hardware.

## Table of Contents
- [Overview](#overview)
- [Features](#features)
- [Architecture](#architecture)
  - [Deployment Topology](#deployment-topology)
  - [Upload / Ingest Pipeline](#upload--ingest-pipeline)
  - [File Retrieval](#file-retrieval)
  - [Backup & Maintenance Daemon](#backup--maintenance-daemon)
- [API Reference](#api-reference)
- [Plugin System](#plugin-system)
- [Technology Stack](#technology-stack)
- [External Libraries](#external-libraries)
- [Testing](#testing)
- [Deployment & Packaging](#deployment--packaging)
  - [Data Persistence](#data-persistence)
- [Roadmap](#roadmap)

## Overview

Dewey ingests uploaded files, validates and sorts them, and exposes them again through a RESTful API. Every upload gets a database record tracking file identity, hash, storage path, and case metadata (claim number, claimant, employer, etc.). Speed and low resource overhead are priorities — the goal is a service that stays stable on the smallest hardware it can get away with.

## Features

**Ingest & Sorting**
- Extension allowlist (`pdf`, `txt`, `doc`/`docx`, `xls`/`xlsx`, `csv`, `ppt`, `png`, `jpg`/`jpeg`)
- Content-sniffing so a file's actual bytes must match its declared extension
- Windows PE / Linux ELF binary rejection
- SHA-256 hashing of every upload
- Code128 barcode scanning on image uploads
- Lua-based plugin pipeline for custom sorting/filtering logic, executed in salience order

**Security**
- IP allowlist ("known machines") gating every route, with per-request access logging as an audit trail
- Hard upload size ceiling enforced at the body-read level, not just multipart parsing
- Global HTTP rate limiting (token bucket)
- Files are served with `nosniff` and forced-download headers so stored content can't be executed by a browser

**Storage & Reliability**
- Two-tier storage: a fast `cache/` directory for recent uploads and a permanent `store/` directory
- Hourly daemon that zips `store/` into a timestamped archive under `backup/`
- Hourly cache clear to keep the fast-path directory small

**Observability & Administration**
- Prometheus metrics: uptime, memory/GC stats, file counts, upload rejections by reason, upload size distribution, barcode success/failure, plugin run/error counts, DB errors, and more
- Svelte-based admin portal: file browser, machine allowlist management, analytics/graphing, and in-app docs for both developers and administrators
- Structured, rotating logger

**Extensibility**
- Lua plugin system with a dynamic hook registry (`OnUpload`, `OnFilter`, `OnDelete`, `OnInit`, `OnTick`) — a plugin attaches to a hook simply by defining a function with that name
- (Planned) plugin validation
- (Planned) self restart and cleanup

## Architecture

### Deployment Topology

Dewey runs as four containers inside a single Podman pod, sharing a network namespace so they can reach each other over `localhost`.

```mermaid
flowchart LR
    Client["Client / Browser"]

    subgraph Pod["dewey-pod (Podman)"]
        FE["Frontend<br/>Svelte admin portal<br/>:3000"]
        BE["Backend<br/>Go API<br/>:8080"]
        DB[("MySQL<br/>:3306")]
        PROM["Prometheus<br/>:9090"]
    end

    Vol[("Volumes<br/>store/ cache/ backup/ logs/")]

    Client --> FE
    Client --> BE
    FE --> BE
    BE --> DB
    BE -. scraped by .-> PROM
    BE --> Vol
```

### Upload / Ingest Pipeline

`POST /upload` validates and responds fast; hashing, barcode scanning, plugin sorting, and the permanent copy all happen afterward in the background so the client isn't kept waiting on them.

```mermaid
flowchart TD
    A["POST /upload"] --> B{"Extension allowed?"}
    B -- No --> R1["400 Invalid file type"]
    B -- Yes --> C{"PE / ELF binary?"}
    C -- Yes --> R2["400 Executable blocked"]
    C -- No --> D{"Content matches extension?"}
    D -- No --> R3["400 Content mismatch"]
    D -- Yes --> E["SHA-256 hash + save to cache/"]
    E --> F["200 OK to client"]
    E --> G["Async post-processing"]
    G --> H{"Image extension?"}
    H -- Yes --> I["Scan Code128 barcode"]
    H -- No --> J["Barcode = Nil"]
    I --> K["Run OnUpload plugins (tag-only, can't veto)"]
    J --> K
    K --> K2["Run OnFilter plugins in salience order (can rewrite Path)"]
    K2 --> L["Copy file: cache/ -> store/"]
    L --> M["Create files + meta DB records"]
```

### File Retrieval

`GET /files/:filename/:meta` checks the fast cache before falling back to the permanent store looked up through the database.

```mermaid
flowchart TD
    A["GET /files/:filename/:meta"] --> B{"meta == true?"}
    B -- Yes --> C["Fetch metadata row from DB"]
    C --> D["200 OK JSON"]
    B -- No --> E{"File in cache/?"}
    E -- Yes --> F["Stream from cache/"]
    E -- No --> G["Look up store/ path in DB"]
    G --> H{"Found on disk?"}
    H -- Yes --> I["Stream from store/"]
    H -- No --> J["404 Not Found"]
```

### Backup & Maintenance Daemon

A background ticker (default interval: hourly, see `daemonTickTime` in `consts.go`) handles cleanup and backups without blocking the request path.

```mermaid
flowchart LR
    T(("Hourly tick")) --> A["Clear cache/"]
    A --> B["Zip store/ into a timestamped file under backup/"]
    B --> C["Run OnTick plugins"]
```

## API Reference

All routes below sit behind the IP-allowlist middleware (`known_machines`), which also logs every authorized request.

| Method | Path | Description |
|---|---|---|
| GET | `/` | Health check; returns a Unix timestamp |
| GET | `/version` | Reports the running app version and the git branch it was built from |
| GET | `/admin` | Admin portal page |
| GET | `/settings` | Settings page |
| POST | `/upload` | Upload, validate, hash, and queue a file for sorting |
| GET | `/files/:filename/:meta` | Stream a file (`meta=false`) or return its metadata as JSON (`meta=true`) |
| GET | `/files` | List files currently sitting in `cache/` |
| DELETE | `/files/:filename` | Remove a file's `store/` copy and mark its DB record deleted |
| GET | `/admin/dumpCache` | Trigger an async clear of `cache/` |
| GET | `/machines` | List machines on the IP allowlist |
| POST | `/machines` | Add a machine to the IP allowlist |
| DELETE | `/machines/:ip` | Remove a machine from the IP allowlist |

## Plugin System

Sorting and filtering logic is written in Lua rather than hardcoded in Go, so it can change without a rebuild. Plugin files live in `backend/plugins/` and are loaded from `pluginDir` at startup, against a dynamic registry of hook names Go registers up front (`main.go`) — there's no fixed set of plugin "types" baked into the loader.

Every plugin declares a `WhoAmI()` function returning its salience only. Which hook(s) it attaches to comes from the global functions it defines: a file attaches to a hook by defining a function with that hook's exact name (e.g. `OnFilter(entry)`), and a single file can implement more than one hook. Each such function receives the file's DB entry as a Lua table and returns the (possibly modified) table back.

| Hook | Runs | Notes |
|---|---|---|
| `OnUpload` | Once per upload, during async post-processing, before `OnFilter` | Fires after the `200 OK` is already sent, so it can't veto the upload — tag-only (e.g. write into `Meta`) |
| `OnFilter` | Once per upload, during async post-processing | Can rewrite the file's destination `Path` |
| `OnDelete` | Once per delete request, synchronously, before anything is removed | Calling `error(...)` vetoes the deletion — the caller gets a `403` instead |
| `OnInit` | Once at server startup | Registered and invoked; no shipped example plugin |
| `OnTick` | Once per daemon tick | `OnTick.lua` — fetches NOAA's planetary K-index (`http.get`), caches it to the plugin scratch directory (`files.write`), reads the cache back (`files.read`), and beacons it out (`http.post`); see the Capability API section below |

Within a hook, plugins run in descending salience order (highest first), each in its own isolated Lua state (via a per-plugin `sync.Pool`) so one plugin's globals can't leak into another's. Example plugins for `OnFilter`, `OnUpload`, `OnDelete`, and `OnTick` ship in `backend/plugins/`.

### Sandboxing & Capability API

A plugin's Lua state only has `base`, `string`, and `table` opened — no `os`, `io`, or `math`, and the base-library globals that could reconstruct that access (`load`, `loadstring`, `dofile`, `loadfile`, `require`, `getfenv`, `setfenv`) are stripped out too (issue #284, superseding #199, which had found `lua.NewState()` running with the full stdlib open — full shell access from any `.lua` file dropped into `plugins/`).

In place of raw `os`/`io`, plugins get two narrow, Go-implemented capability APIs — the only way for one to reach the network or filesystem:

| Function | Does | Restriction |
|---|---|---|
| `http.get(url)` | GET request, returns the response body as a string | Refuses to dial loopback/private/link-local/unspecified IPs — checked against the actually-resolved IP at dial time, not just the URL's hostname, closing a DNS-rebinding gap. MySQL and Prometheus are deliberately unpublished from the LAN (#200, #204) with no auth of their own; this is what stops an unrestricted plugin HTTP client from being a direct SSRF path back into both. |
| `http.post(url, body)` | POST request with `body`, returns the response body as a string | Same restriction as `http.get`. |
| `files.read(name)` | Reads a file, returns its contents as a string | Confined to `pluginScratchDir` (`./plugin-scratch`, separate from `store`/`cache`/`backup`) via the same traversal check (`resolveStorePath`) that keeps uploads inside `fileSystemBaseDir` — a `../` escape is rejected before any file is touched. |
| `files.write(name, data)` | Writes `data` to a file | Same restriction as `files.read`. |

A Go-side error on any of these four raises a normal Lua error, catchable with `pcall` like any other plugin error. `OnTick.lua` uses all four together as one real workflow: fetch, cache, read the cache back, beacon out.

## Technology Stack

The backend is written entirely in Go, chosen for the balance it strikes between simplicity and performance: a shallow learning curve, fast compile times, and static-binary deployment, without giving up the control and speed of a compiled language.

Go's concurrency model (goroutines and channels) is a good match for a service that needs to handle multiple uploads at once — post-processing runs in the background per-request (see the ingest diagram above), and the maintenance daemon runs on its own ticker, both without blocking the HTTP server.

The frontend admin portal is built with Svelte, giving a reactive UI with HTML-like syntax and a fast dev experience. Keeping frontend and backend as separate deployable units keeps each side easier to maintain and scale independently.

## External Libraries

| Library | Purpose |
|---|---|
| [Gin](https://github.com/gin-gonic/gin) | Go web framework used for the REST API |
| [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) | MySQL driver — Dewey's actual database backend |
| [Gozxing](https://github.com/makiuchi-d/gozxing) | Barcode generation and decoding (Code128) |
| [GopherLua](https://github.com/yuin/gopher-lua) | Native Go Lua VM powering the plugin system |
| [prometheus/client_golang](https://github.com/prometheus/client_golang) + [go-gin-prometheus](https://github.com/zsais/go-gin-prometheus) | Metrics collection and exposition |
| [golang.org/x/time](https://pkg.go.dev/golang.org/x/time/rate) | Token-bucket rate limiting for the HTTP API |

## Testing

Two layers of tests cover the backend:

- **Go unit tests** (`go test ./...` from `backend/`) — cover the pure-logic pieces in isolation: `filevalidator_test.go` exercises PE/ELF binary sniffing and extension/content-type matching, and `plugins_test.go` exercises hook registration, plugin loading/attachment, salience ordering, `OnDelete`'s veto behavior, the runtime sandbox (dangerous globals and `os`/`io` absent), and the `http`/`files` capability API (successful round trips, plus the SSRF and path-traversal restrictions). These run in CI on every push/PR (`.github/workflows/go.yml`).
- **BITs** (Built-In Tests, `backend/testing_tooling/test_suite.sh`) — a black-box HTTP suite that runs against a live server, hitting real endpoints (uploads with randomized metadata, error cases, health checks) rather than calling Go functions directly. The server launches this suite automatically in the background on every startup (see the `BITs` section in `main.go`) so a bad deploy fails loudly instead of silently; it can also be run manually against any base URL: `./test_suite.sh http://localhost:8080`.

## Deployment & Packaging

The application and its supporting services are built into Podman containers for portable, controlled deployment.

- **`deploy.sh`** — creates `dewey-pod` and starts all four containers (MySQL, Prometheus, backend, frontend) with persistent named volumes for the database and for `store/`, `cache/`, `backup/`, and `logs/`.
- **`package.sh`** — builds the backend and frontend images against an already-running `dewey-pod`, then exports the whole pod (images + Kubernetes-style pod spec + a self-contained `run.sh`) as a `.tar.gz` bundle that can be moved to another host and deployed without needing a registry or a repo checkout — it also cross-compiles and bundles `dewey-cli`.
- **`dewey-cli`** — a standalone CLI client for the deployed API (`cli/main.go`), buildable manually (`cd cli && go build -o dewey-cli .`) or already sitting alongside `run.sh` in a `package.sh` bundle. List-shaped responses (`list_files`, `list_machines`) render as aligned tables; every other response prints as indented JSON, colored red on a non-2xx status. `dewey-cli` can also generate synthetic test files (`create_docx`, `create_exe`, `create_pdf`) for exercising upload validation.

### Data Persistence

Both `deploy.sh` and a bundle's `run.sh` recreate `dewey-pod` from scratch on every run (`podman pod rm -f dewey-pod`), so whatever survives that has to live in a named Podman volume rather than the pod itself. Two independent things can be wiped, normally controlled separately (`deploy.sh --clean-slate` ties them together — see below):

| Data | Volume(s) | Controlled by |
|---|---|---|
| MySQL database (file records, metadata, the `known_machines` allowlist) | `mysql-data` | `deploy.sh` only — `--reset-db` |
| Backend data (`store/`, `cache/`, `backup/`, `logs/`, `plugin-scratch/`) | `dewey-store`, `dewey-cache`, `dewey-backup`, `dewey-logs`, `dewey-plugin-scratch` | `deploy.sh` — `--keep-data` / `--wipe-data`, or the interactive prompt (`package.sh`'s bundled `run.sh` doesn't yet mount `dewey-plugin-scratch`) |

**`deploy.sh` flags:**
- `--reset-db` — wipes the `mysql-data` volume before starting MySQL. Not passing this is the default and keeps the database across redeploys. Since MySQL only honors `MYSQL_ROOT_PASSWORD` on first init of an empty data directory, the root password is persisted to a local, gitignored `.mysql-root-password` file and reused on every run that keeps `mysql-data` — it's only regenerated when `--reset-db` actually wipes the volume.
- `--keep-data` — preserves `dewey-store`/`dewey-cache`/`dewey-backup`/`dewey-logs`/`dewey-plugin-scratch` instead of wiping them.
- `--wipe-data` — explicitly wipes those same four volumes. Needed to wipe non-interactively (CI, cron, `ssh` without `-t`), since without a TTY to prompt on, the script defaults to `--keep-data` rather than silently wiping.
- No flags, run interactively: prompted `Wipe store/cache/backup/logs volumes before this deploy? [Y/n]` (default: wipe).
- `--clean-slate` — forces both a full `mysql-data` wipe and a `dewey-store`/`dewey-cache`/`dewey-backup`/`dewey-logs`/`dewey-plugin-scratch` wipe together, overriding any `--reset-db`/`--keep-data`/`--wipe-data` also passed. Exists because those two resets are otherwise independent: running one without the other leaves the DB pointing at files that no longer exist, or files on disk with no DB record — exactly the drift this flag is meant to rule out (issue #296). Requires an interactive TTY and typing `yes` at a dedicated confirmation prompt; refuses to run at all non-interactively, since this permanently destroys every stored file and its metadata in one shot.

**Bundled `run.sh` flags** (from a `package.sh` bundle): the same `--keep-data` / `--wipe-data` pair and interactive-prompt fallback for the four backend data volumes — but no `--reset-db` equivalent. `run.sh` never touches `mysql-data`, so the database always persists across a bundle's redeploys regardless of flags; wiping it requires `podman volume rm mysql-data` by hand.

**To guarantee nothing is lost across a redeploy:** don't pass `--reset-db`, and either pass `--keep-data` or answer `n` to the prompt (or pass `--keep-data` up front to skip the prompt entirely in a non-interactive context).

**To guarantee a fully clean, consistent state instead:** pass `--clean-slate` rather than combining `--reset-db` with a store/cache wipe by hand — it's the only path that keeps both sides in sync, since the individual flags don't warn you if they end up wiping just one.

## Repo Stats

As of 2026-08-12: 450 commits on `main` (1 contributor), 68 branches, 6 tags (latest `v0.2.0`), 165 issues filed (150 closed / 15 open), 136 PRs (131 merged / 5 closed). 98 tracked files, 10,221 lines of hand-written source — Go 4,828, Svelte 3,209, Shell 1,572, TypeScript 488, Lua 124.

## Roadmap

- Plugin validation
- Self restart and cleanup
