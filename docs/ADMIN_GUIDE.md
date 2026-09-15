# Dewey — Admin Guide

Day-to-day operation, file management, settings, and monitoring.

## Table of Contents
- [Overview](#overview)
- [Uploading Files](#uploading-files)
- [Retrieving Files](#retrieving-files)
- [Known Machines](#known-machines)
- [Analytics](#analytics)
- [Settings](#settings)
- [System Configuration](#system-configuration)
- [Troubleshooting](#troubleshooting)

## Overview

Dewey is a document management and file sorting system. When a file is submitted, Dewey validates it, scans it for barcodes, sorts it into the right location using configurable rules (plugins), and stores it alongside its metadata. Files can then be retrieved or deleted through the API.

This portal gives you visibility into the system's health, lets you tune its behaviour, and is the primary place to monitor activity.

| Service | Port | Purpose |
|---|---|---|
| Backend API | 8080 | Handles all file operations and system control. |
| Admin Portal | 3000 | This interface. Analytics, settings, and docs. |
| Prometheus | 9090 | Raw metrics if you need deeper inspection. |

## Uploading Files

Files are submitted via a `POST` to `http://<host>:8080/upload` as `multipart/form-data`. This is typically done by your document submission tool or integration, not manually.

### Allowed File Types

Only the following extensions are accepted. All others are rejected before any processing occurs.

`.pdf` `.txt` `.doc` `.docx` `.xls` `.xlsx` `.csv` `.ppt` `.png` `.jpg` `.jpeg`

### File Size Limit

The default maximum file size is **50 MB**. This is a backend default ([System Configuration](#system-configuration), below) — changing it means editing `backend/config.json` and redeploying, not a Settings-page toggle.

### Security Checks

Every file — regardless of extension — is inspected for Windows PE and Linux ELF executable signatures. Any file that appears to be an executable binary is rejected, even if it has a permitted extension like `.pdf`.

### What happens after upload

1. File is placed in the cache temporarily.
2. Any barcode or QR code in the file is decoded and recorded.
3. Sorting rules (plugins) run and determine the file's destination folder.
4. File is moved to permanent storage and a database record is created.

> **Note:** The cache is cleared periodically by the background daemon. Do not rely on files remaining in the cache — always retrieve from the permanent store via the filename.

## Retrieving Files

### List all files

Returns a JSON list of filenames currently in the cache.

```
GET http://<host>:8080/files
```

### Get a specific file

Returns the file as a stream. Use the timestamped filename returned at upload time.

```
GET http://<host>:8080/files/<filename>/false
```

### Delete a file

Removes the file from the cache. The database record is soft-deleted (not permanently removed).

```
DELETE http://<host>:8080/files/<filename>
```

> **Filenames:** Dewey prefixes every uploaded file with a Unix timestamp to prevent collisions, e.g. `1700000000_report.pdf`. The original filename is preserved after the prefix. The full timestamped name is returned in the upload response and should be stored by your integration.

## Known Machines

Dewey only accepts requests from machines whose IP address has been registered ahead of time. Anything else — including the file manager portal, if opened from an unregistered machine — gets rejected with `403 Forbidden` before it reaches any file operation.

Use the **Known Machines** page in the sidebar to see every registered machine, when it was added, and when it last connected — and to add or remove machines yourself.

> **Note:** you can only reach the Known Machines page from a machine that's already registered — a brand-new machine can't add itself. The very first entry has to be set up directly against the database by whoever deployed the system.

## Analytics

The Analytics page shows live metrics pulled from the backend. They refresh at the interval configured in Settings → Behavior → Metrics Poll Interval.

| Metric | Description |
|---|---|
| Files Uploaded | Total number of files successfully processed since the server started. |
| File Sorts | How many files have been moved from cache to permanent storage by the pipeline. |
| File Retrievals | Total number of times a file has been fetched via the API. |
| File Deletions | Total number of cache deletions triggered manually or by the API. |
| File Copies | Total copy operations from cache to store. Should equal File Sorts in normal operation. |
| Barcode Successes / Failures | Count of files where barcode scanning succeeded or failed. High failures may indicate image quality issues. |
| Plugin Runs / Errors | How many times plugins have executed, and how many errors were encountered. Plugin errors are non-fatal but the file may be sorted incorrectly. |
| DB Errors | Database write failures. Any non-zero value warrants investigation. |
| Upload Rejections | Files rejected before processing — broken down by reason (invalid extension, PE detected, ELF detected). |
| Upload Size Distribution | Histogram of uploaded file sizes. Useful for capacity planning. |

> All metrics shown here are **in-memory counters** that reset when the server restarts. For long-term historical data, use the Prometheus endpoint at port 9090.

## Settings

Settings are accessed via the sidebar. They are saved immediately to your browser's local storage — no save button needed, and changes apply instantly. They are per-browser and do not affect other users.

### Behavior

| Setting | Description |
|---|---|
| Metrics Poll Interval | How often the Analytics and Dashboard pages refresh. Lower values give more real-time data but increase backend traffic. Options: 1s, 5s, 15s, 30s. |
| Analytics History Window | How many data points the sparkline charts retain. More points = longer history visible, higher memory use in the browser. |
| Prometheus Alert Banner | When enabled, a red banner appears at the top of the page if the backend metrics endpoint cannot be reached. Disable if the Prometheus banner is distracting in stable environments. |
| Default Sidebar State | Whether the sidebar is open or collapsed when you first load the app on any page. |
| Layout Density | Comfortable adds more padding throughout the interface. Compact reduces spacing for smaller screens or denser information display. |

### Alerts

| Setting | Description |
|---|---|
| RAM Alert Threshold | Shows a warning on the dashboard when backend RAM usage exceeds this value in megabytes. Default is 200 MB. |
| Retry Alert Threshold | Shows a warning when file processing retries exceed this count. A non-zero retry count usually indicates a transient failure in the pipeline. |

### Customization

| Setting | Description |
|---|---|
| Dark Mode | Switches the entire portal to a dark colour scheme. Saved per browser. |
| Accent Color | The highlight colour used on cards, active sidebar links, and section headings. |
| Chart Color Theme | The colour palette used by all analytics charts. Default, Cool, Warm, and Mono are available. |

## System Configuration

These values aren't editable from the portal — they're backend defaults, read once from `backend/config.json` at server startup.

| Setting | Description |
|---|---|
| Daemon Tick Interval | How often (in hours) the background daemon runs. On each cycle it clears the file cache and creates a backup of permanent storage. Reducing this means more frequent backups and more aggressive cache clearing. Default: 1 hour. |
| Max Upload File Size | The largest file the server will accept in a single upload request, in megabytes. Files exceeding this are rejected before any processing. Default: 50 MB. |
| Max Open DB Connections | The maximum number of simultaneous open connections to the MySQL database. Increase if you see database timeout errors under heavy load. Default: 10. |
| Max Idle DB Connections | The number of connections kept ready in the pool when not in use. Higher values reduce connection setup latency at the cost of held resources. Default: 10. |
| DB Connection Timeout | How long (in minutes) a database connection is kept alive before being recycled. Recycling connections helps recover from silent disconnects. Default: 2 minutes. |

> **Note:** To change a value, edit `backend/config.json` and redeploy — there's no live API or Settings-page control for these.

## Troubleshooting

### Prometheus banner / metrics not loading

**Possible causes:**
- The backend server is not running.
- The server is running but the metrics endpoint (port 8080) is not reachable from the frontend container.
- The Prometheus scraper on port 9090 is down (this does not affect the API metrics endpoint).

**Fix:** Check that the backend container is running: `podman ps`. Verify the pod network is intact. Review the backend logs for `Fatal` entries.

### File upload rejected — "Invalid file type"

**Possible causes:**
- The file extension is not in the allowlist.
- The file has a permitted extension but its internal bytes look like an executable (PE or ELF header detected).

**Fix:** Verify the file extension. If the file is legitimate but being flagged as an executable, it may be corrupted — re-export it from the source application.

### File uploaded but not appearing in retrieval

**Possible causes:**
- The cache was cleared by the daemon between upload and retrieval (cache is ephemeral).
- The plugin chain set an unexpected destination path — the file is in permanent storage but under a different path than expected.

**Fix:** Use `GET /files` to list what is currently in cache. For permanent storage, check the database record directly — the `filepath` column holds the exact location within `./store`.

### High DB Errors count on the Analytics page

**Possible causes:**
- The MySQL server is unreachable or the connection pool is exhausted.
- The schema is out of date (migration did not run cleanly on the last start).

**Fix:** Check backend logs for `WARN` or `FATAL` lines containing "transaction" or "database". Verify the `DB_DSN` environment variable is correct and the database server is healthy.

### High Barcode Failures count

**Possible causes:**
- Source documents do not contain barcodes — this is expected for many document types.
- Barcode images are too small, low resolution, or distorted.

**Fix:** If barcode scanning is not required for your workflow, barcode failures are informational only and do not affect sorting or storage. If barcodes are expected, check the source document quality.

### 403 Forbidden — "unregistered machine"

**Possible causes:**
- The request came from an IP address that has not been added to the Known Machines list.
- The machine's IP changed (e.g. DHCP reassignment) since it was registered.
- On dual-stack systems, requests to `localhost` resolved to IPv6 `::1` when only IPv4 `127.0.0.1` was registered.
- Podman NAT translated the connection so it arrived under a virtual bridge or gateway IP.
- `deploy.sh` fell back to literal `"localhost"` because `hostname -I` returned nothing, creating a non-matching allowlist entry.

**Fix:** From an already-registered machine, open Known Machines in the sidebar and add the new IP with a label. If connecting via `localhost`, ensure both `127.0.0.1` and `::1` are registered. If Podman's NAT altered the source IP, check `podman logs cross-doc-tool-dev` for the "unregistered machine" line to see the actual address received. If this is the very first machine being set up, it must be registered directly against the database instead.

### Plugin Errors showing in Analytics

**Possible causes:**
- A Lua plugin script has a runtime error.
- A plugin's hook function (e.g. `OnFilter`, `OnUpload`) returned an unexpected type instead of the entry table.

**Fix:** Check the backend logs for `WARN` lines containing "plugin". The error message will include the plugin name and the Lua error. Fix the plugin script in `./plugins/` — it is reloaded on the next server start (or on the next daemon tick, or immediately via `GET /admin/reloadPlugins` — see the Developer Guide's Plugin System section).
