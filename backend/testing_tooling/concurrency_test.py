#!/usr/bin/env python3
"""Verifies computeTickInterval's active-users term (issue #309) by forcing
genuinely concurrent requests from distinct source IPs and scraping
/metrics while they're still in flight, rather than soak_test.py's
independently-randomized per-user schedule — at typical request rates,
brief non-overlapping requests almost never land inside the same 10s
scrape window, so ActiveUserCount() never reads above 1 under that
approach.

A real backend request over a small file only takes microseconds, and
throttling a SMALL file's read rate doesn't help either: the whole
payload fits inside the kernel's TCP send buffer, so the server's write()
returns (and trackUser's defer fires) the instant it hands the bytes to
the OS, regardless of how slowly the client is actually draining them —
the throttling only affects the client's read rate, never the server's
in-flight window. This uploads one large synthetic file first (default
8MB) specifically so it's bigger than typical send-buffer autotuning —
once the client can't keep up, the server's write genuinely blocks on
real TCP backpressure, and that blocked time is what keeps the request
counted as in-flight for trackUser's whole duration.

Also requires the /metrics-doesn't-self-count fix (logConnections'
trackActivity flag, backend/main.go) — without it, every scrape of
/metrics counts its own in-flight connection and the result here is
meaningless (connected_users would always read at least 1, burst or not).

Ported from concurrency_test.sh (issue #469). Run from inside the
backend's own container, same as load_test.py/soak_test.py — every
request needs a source IP registered in known_machines, and the loopback
aliasing trick below only works from inside the container being tested:
    podman exec cross-doc-tool-dev python3 testing_tooling/concurrency_test.py

Also requires FILES_PASSWORD and MACHINES_PASSWORD in the environment
(issue #332) — both are already set on the backend container, so running
via podman exec picks them up for free.

Usage:
    python3 concurrency_test.py [base_url] [users] [limit_rate]
"""

from __future__ import annotations

import base64
import os
import re
import shutil
import sys
import tempfile
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import requests
from requests.adapters import HTTPAdapter

from dewey_test_utils import exchange_session_token


class SourceIPAdapter(HTTPAdapter):
    """Binds outgoing connections to a specific local IP — Python
    equivalent of curl's --interface, used so each simulated user's
    requests actually arrive at the backend from a distinct source
    address (each loopback alias must already be registered in
    known_machines).
    """

    def __init__(self, source_ip: str, **kwargs):
        self.source_ip = source_ip
        super().__init__(**kwargs)

    def init_poolmanager(self, *args, **kwargs):
        kwargs["source_address"] = (self.source_ip, 0)
        return super().init_poolmanager(*args, **kwargs)


def parse_rate(rate: str) -> int:
    """Parses a curl-style --limit-rate value ("200k", "1m", "500") into
    bytes/sec.
    """
    match = re.fullmatch(r"(\d+)([kKmMgG]?)", rate)
    if not match:
        raise ValueError(f"invalid rate: {rate!r}")
    n, suffix = match.groups()
    multiplier = {"": 1, "k": 1024, "m": 1024**2, "g": 1024**3}[suffix.lower()]
    return int(n) * multiplier


def throttled_download(session: requests.Session, url: str, rate_bytes_per_sec: int) -> int:
    """Downloads url via session, deliberately reading slowly enough to
    simulate curl's --limit-rate — this is what creates the real TCP
    backpressure the rest of the script depends on (see the module
    docstring). chunk_size is small enough that the sleep granularity
    stays fine relative to the target rate.
    """
    chunk_size = 8192
    with session.get(url, stream=True, timeout=120) as resp:
        for _ in resp.iter_content(chunk_size=chunk_size):
            time.sleep(chunk_size / rate_bytes_per_sec)
        return resp.status_code


def scrape_metric(base: str, name: str) -> str:
    try:
        body = requests.get(f"{base}/metrics", timeout=10).text
    except requests.exceptions.RequestException:
        return ""
    match = re.search(rf"^{re.escape(name)} (\S+)", body, re.MULTILINE)
    return match.group(1) if match else ""


def main() -> int:
    base = (sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8080").rstrip("/")
    users = int(sys.argv[2]) if len(sys.argv) > 2 else 10
    limit_rate = parse_rate(sys.argv[3] if len(sys.argv) > 3 else "200k")

    files_password = os.environ.get("FILES_PASSWORD")
    machines_password = os.environ.get("MACHINES_PASSWORD")
    if not files_password:
        print("FILES_PASSWORD must be set", file=sys.stderr)
        return 1
    if not machines_password:
        print("MACHINES_PASSWORD must be set", file=sys.stderr)
        return 1

    files_session = requests.Session()
    files_session.headers.update(
        {"X-Dewey-Session-Token": exchange_session_token(base, files_password, "files")}
    )
    machines_session = requests.Session()
    machines_session.headers.update(
        {"X-Dewey-Session-Token": exchange_session_token(base, machines_password, "machines")}
    )

    registered_ips: list[str] = []
    test_file: Path | None = None
    uploaded_filename = ""

    def register_machine(ip: str, label: str):
        try:
            machines_session.post(f"{base}/core/machines", json={"ip": ip, "label": label}, timeout=10)
        except requests.exceptions.RequestException:
            pass
        registered_ips.append(ip)

    def cleanup():
        print("Cleaning up...")
        if uploaded_filename:
            try:
                files_session.delete(f"{base}/core/files/{uploaded_filename}", timeout=10)
            except requests.exceptions.RequestException:
                pass
        if test_file is not None:
            shutil.rmtree(test_file.parent, ignore_errors=True)
        for ip in registered_ips:
            try:
                machines_session.delete(f"{base}/core/machines/{ip}", timeout=10)
            except requests.exceptions.RequestException:
                pass

    try:
        print(f"Registering {users} loopback-alias users...")
        for i in range(1, users + 1):
            register_machine(f"127.0.0.{i + 1}", f"concurrency-test-{i}")

        print("Generating and uploading an 8MB test file (large enough to outrun TCP send-buffer autotuning)...")
        # .txt content must actually sniff as text/plain to pass the
        # upload's extension/content validator (filevalidator.go) —
        # base64-encoded random bytes gives printable text at the right
        # size, not raw binary.
        tmp_dir = Path(tempfile.mkdtemp())
        test_file = tmp_dir / "concurrency_test.txt"
        test_file.write_bytes(base64.b64encode(os.urandom(6 * 1024 * 1024)))

        # date_of_injury is required server-side (issue #365); the value
        # doesn't matter for this script's purpose (forcing concurrent
        # in-flight requests).
        with open(test_file, "rb") as f:
            upload_resp = files_session.post(
                f"{base}/core/upload",
                files={"file": (test_file.name, f)},
                data={"date_of_injury": "2025-01-01"},
                timeout=60,
            )
        try:
            uploaded_filename = upload_resp.json().get("filename", "")
        except ValueError:
            uploaded_filename = ""
        if not uploaded_filename:
            print(f"FAIL — upload didn't return a filename: {upload_resp.text}", file=sys.stderr)
            return 1
        print(f"Uploaded as {uploaded_filename}")

        print("Baseline (before burst):")
        baseline_connected = scrape_metric(base, "app_connected_users")
        baseline_tick = scrape_metric(base, "app_time_til_next_tick")
        print(f"  connected_users={baseline_connected or '?'} time_til_next_tick={baseline_tick or '?'}")

        print(
            f"Firing {users} concurrent throttled downloads (limit-rate {sys.argv[3] if len(sys.argv) > 3 else '200k'}), "
            "polling /metrics while they're in flight..."
        )

        download_pool = ThreadPoolExecutor(max_workers=users)
        download_futures = []
        for i in range(1, users + 1):
            ip = f"127.0.0.{i + 1}"
            s = requests.Session()
            s.headers.update(files_session.headers)
            s.mount("http://", SourceIPAdapter(ip))
            s.mount("https://", SourceIPAdapter(ip))
            url = f"{base}/core/files/{uploaded_filename}?meta=false"
            download_futures.append(download_pool.submit(throttled_download, s, url, limit_rate))

        # 8MB at limit_rate takes real seconds once backpressure kicks in,
        # so a plain sleep-spaced poll loop is reliable here — no need to
        # race process start-up jitter the way a sub-millisecond-duration
        # request would require.
        #
        # Records time_til_next_tick alongside connected_users on every
        # sample, not just the peak concurrency itself — computeTickInterval
        # only runs once per daemon tick (not continuously), so whether
        # alpha*activeUsers actually influenced anything is only visible at
        # the tick that happens to land while the burst is still in flight,
        # not at every sample.
        max_connected = 0.0
        peak_tick = ""
        print("elapsed_s,connected_users,time_til_next_tick")
        for i in range(1, 21):
            connected = scrape_metric(base, "app_connected_users")
            tick = scrape_metric(base, "app_time_til_next_tick")
            print(f"{i},{connected},{tick}")
            if connected:
                try:
                    connected_f = float(connected)
                except ValueError:
                    connected_f = 0.0
                if connected_f >= max_connected:
                    max_connected = connected_f
                    peak_tick = tick
            time.sleep(1)

        for fut in download_futures:
            fut.result()  # let every download finish before this script exits
        download_pool.shutdown(wait=True)

        after_tick = scrape_metric(base, "app_time_til_next_tick")

        print()
        print(f"baseline: connected_users={baseline_connected or '?'} time_til_next_tick={baseline_tick or '?'}")
        print(f"peak:     connected_users={max_connected:g} time_til_next_tick={peak_tick or '?'}")
        print(f"after:    time_til_next_tick={after_tick or '?'} (burst finished, downloads no longer in flight)")

        if max_connected > 1:
            print(f"PASS — active-users term is exercised above 1 (max connected_users={max_connected:g})")
            print(
                "Whether the formula reacted: compare the elapsed_s,connected_users,time_til_next_tick "
                "table above — a fresh tick landing while connected_users was elevated should show a "
                "higher reset value than the baseline/idle peaks typically seen; time_til_next_tick "
                "counts down between ticks, so read local maxima (right after a reset), not raw samples."
            )
            return 0
        else:
            print(
                f"FAIL — connected_users still <= 1; try a higher users count, a lower limit_rate, "
                "or investigate further",
                file=sys.stderr,
            )
            return 1
    finally:
        cleanup()


if __name__ == "__main__":
    sys.exit(main())
