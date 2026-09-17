#!/usr/bin/env python3
"""Fires a burst of small text uploads (plus a retrieval pass) at Dewey to
exercise the tick-scaling logic from issue #305: uploadCounter/
retrievalCounter's 30s sliding window, the EMA smoothing in startDaemon,
and computeTickInterval's effect on app_time_til_next_tick/app_upload_rate.

Ported from load_test.sh (issue #469). Also fixes a real bug found while
porting: the bash version sent the raw X-Dewey-Password header directly,
which stopped working once issue #409 replaced requirePassword with
requireSession on every files/machines route except /session — confirmed
by actually running the old script against a live backend first (0/N
uploads succeeded). This version exchanges for a session token like
test_suite.py already does.

Run this from somewhere with a registered IP (known_machines) — an
external host connecting through podman's rootless port-forwarding NATs
to an unregistered IP and gets rejected by the allowlist. From the repo
root: `podman exec cross-doc-tool-dev python3 testing_tooling/load_test.py`
runs it from inside the backend container itself, same as the BITs suite.

Also requires FILES_PASSWORD in the environment (issue #332) — set on the
backend container already, so running via podman exec picks it up for
free; running from outside the container needs it passed explicitly, e.g.
`FILES_PASSWORD=$(cat .files-password) ./load_test.py`.

Usage:
    python3 load_test.py [base_url] [count] [concurrency]
"""

from __future__ import annotations

import base64
import os
import shutil
import sys
import tempfile
from collections import Counter
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import requests

from dewey_test_utils import exchange_session_token


def upload_one(session: requests.Session, base: str, tmp_dir: Path, i: int) -> str:
    f = tmp_dir / f"loadtest_{i}.txt"
    # Small, random-ish content so it sniffs as text/plain and clears
    # matchesDeclaredType (filevalidator.go) without eating much disk —
    # ~700 bytes per file (512 random bytes, base64-encoded), so even a
    # few thousand of these stays trivial.
    f.write_bytes(base64.b64encode(os.urandom(512)))

    # date_of_injury is required server-side (issue #365) — any valid
    # date works here since this script only exercises upload
    # volume/timing, not metadata content.
    try:
        with open(f, "rb") as fh:
            resp = session.post(
                f"{base}/core/upload",
                files={"file": (f.name, fh)},
                data={"date_of_injury": "2025-01-01"},
                timeout=30,
            )
        return resp.json().get("filename", "")
    except (requests.exceptions.RequestException, ValueError):
        return ""


def retrieve_one(session: requests.Session, base: str, name: str) -> int:
    # meta is a query param, not a path segment (getFile, backend/routes.go)
    # — the bash original used /false as a path segment instead, which
    # matched the *filename wildcard route and always landed on the
    # "unknown meta flag" 400 case. Confirmed against the CLI's actual
    # working get_file implementation and concurrency_test.sh, both of
    # which already use ?meta=false correctly.
    try:
        resp = session.get(f"{base}/core/files/{name}?meta=false", timeout=10)
        return resp.status_code
    except requests.exceptions.RequestException:
        return 0


def main() -> int:
    base = (sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8080").rstrip("/")
    count = int(sys.argv[2]) if len(sys.argv) > 2 else 500
    concurrency = int(sys.argv[3]) if len(sys.argv) > 3 else 10

    session = requests.Session()
    token = exchange_session_token(base, os.environ.get("FILES_PASSWORD", ""), "files")
    session.headers.update({"X-Dewey-Session-Token": token})

    tmp_dir = Path(tempfile.mkdtemp())
    filenames: list[str] = []

    def cleanup():
        # Best-effort: delete every file this run uploaded into the live
        # store — same reasoning as test_suite.py's cleanup (issue #347):
        # this hits the real backend, and nothing else removes these
        # afterward. deleteStoredFile (backend/filemanager.go) removes the
        # store file, cache copy, and DB row together, so one DELETE per
        # name is enough.
        if filenames:
            print("Cleaning up: deleting uploaded test files from the live store...")
            for name in filenames:
                try:
                    session.delete(f"{base}/core/files/{name}", timeout=10)
                except requests.exceptions.RequestException:
                    pass
        shutil.rmtree(tmp_dir, ignore_errors=True)

    try:
        print(f"Uploading {count} files to {base} (concurrency: {concurrency})...")

        with ThreadPoolExecutor(max_workers=concurrency) as pool:
            filenames = list(pool.map(lambda i: upload_one(session, base, tmp_dir, i), range(1, count + 1)))
        filenames = [f for f in filenames if f]

        print(f"Uploaded {len(filenames)}/{count} files.")

        print("Retrieving them back to exercise retrievalCounter...")
        with ThreadPoolExecutor(max_workers=concurrency) as pool:
            codes = list(pool.map(lambda name: retrieve_one(session, base, name), filenames))

        for code, n in sorted(Counter(codes).items()):
            print(f"{n:6d} {code}")

        print("Done.")
    finally:
        cleanup()

    return 0


if __name__ == "__main__":
    sys.exit(main())
