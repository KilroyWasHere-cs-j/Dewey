#!/usr/bin/env python3
"""Simulates sustained, multi-user, time-varying traffic against Dewey
(issue #311). Where load_test.py fires one fixed-size burst from a single
source and exits in seconds, this runs continuously across many simulated
"users", each issuing requests on a randomized, diurnally-varying cadence
(busy by day, mostly dead at night) — the kind of pattern the tick-scaling
daemon's EMA smoothing and active-user backoff (computeTickInterval,
backend/utils.go) actually needs to be exercised against.

Multi-user simulation binds each simulated user's requests to its own
loopback alias (127.0.0.2, 127.0.0.3, ...) — every container in dewey-pod
shares one network namespace, so spinning up more containers wouldn't
produce distinct source IPs; loopback aliasing is what actually gets
ActiveUserCount() (backend/prometheus.go) above 1.

Also logs a CSV time series of tick timing, cache-clean cadence, and
resource usage (RAM/heap/goroutines/fds) sampled at a fixed real-time
cadence, so a long run's slower-building trends are easy to graph
afterward rather than only visible live in Grafana.

Ported from soak_test.sh (issue #469) — also fixes the same two bugs
load_test.py's port found and fixed: the raw X-Dewey-Password header
(replaced by session tokens, issue #409) and a "/false" path segment
that should be a "?meta=false" query param.

Run from inside the backend's own container, same as load_test.py — every
request needs a source IP registered in known_machines, and the loopback
aliasing trick above only works from inside the container being tested:
    podman exec cross-doc-tool-dev python3 testing_tooling/soak_test.py

Also requires FILES_PASSWORD and MACHINES_PASSWORD in the environment
(issue #332) — both are already set on the backend container, so running
via podman exec picks them up for free.

Usage:
    python3 soak_test.py [base_url] [sim_days] [users] [seconds_per_sim_day]
"""

from __future__ import annotations

import base64
import math
import os
import random
import re
import shutil
import signal
import sys
import tempfile
import threading
import time
from pathlib import Path

import requests
from requests.adapters import HTTPAdapter

from dewey_test_utils import exchange_session_token, rand_date

# Target requests per simulated day per user at peak (multiplier 1.0) — kept
# as a requests/day rate rather than a fixed seconds-between-requests value
# because the latter doesn't scale with seconds_per_sim_day: at the default
# 60s/simday compression, a fixed "8 simulated seconds between requests"
# works out to ~0.006 real seconds, which the floor below always clamps —
# flattening the diurnal ramp entirely regardless of time of day. Deriving
# the interval from a requests/day target instead keeps it meaningful across
# any compression factor: real_interval = seconds_per_sim_day / (rate * multiplier).
PEAK_REQUESTS_PER_SIM_DAY = 200
NIGHT_FLOOR = 0.05
METRICS_SAMPLE_REAL_SECONDS = 10  # fixed cadence, independent of TIME_SCALE


class SourceIPAdapter(HTTPAdapter):
    """Binds outgoing connections to a specific local IP — see
    concurrency_test.py's copy of this for the full rationale. Duplicated
    rather than shared because each script's adapter is a small,
    self-contained implementation detail, not something that would
    benefit from another layer of indirection through dewey_test_utils.
    """

    def __init__(self, source_ip: str, **kwargs):
        self.source_ip = source_ip
        super().__init__(**kwargs)

    def init_poolmanager(self, *args, **kwargs):
        kwargs["source_address"] = (self.source_ip, 0)
        return super().init_poolmanager(*args, **kwargs)


def session_for_ip(token: str, ip: str) -> requests.Session:
    s = requests.Session()
    s.headers.update({"X-Dewey-Session-Token": token})
    s.mount("http://", SourceIPAdapter(ip))
    s.mount("https://", SourceIPAdapter(ip))
    return s


def rate_multiplier(hour: float) -> float:
    # hour is simulated hour-of-day (0-23.99...). sin(pi*(hour-9)/12) is
    # positive only for hour in (9,21) and peaks at hour=15 — that's the
    # whole daytime bump.
    bump = math.sin(math.pi * (hour - 9) / 12)
    bump = max(bump, 0)
    return NIGHT_FLOOR + (1 - NIGHT_FLOOR) * bump


def extract_metric(name: str, body: str) -> str:
    match = re.search(rf"^{re.escape(name)} (\S+)", body, re.MULTILINE)
    return match.group(1) if match else ""


class SoakTest:
    def __init__(self, base: str, sim_days: int, users: int, seconds_per_sim_day: int):
        self.base = base
        self.sim_days = sim_days
        self.users = users
        self.seconds_per_sim_day = seconds_per_sim_day
        self.time_scale = 86400 / seconds_per_sim_day
        self.total_real_seconds = sim_days * seconds_per_sim_day

        self.start_real = time.time()
        self.stop_event = threading.Event()

        self.tmp_dir = Path(tempfile.mkdtemp())
        self.uploaded_files_log = self.tmp_dir / "uploaded_files.txt"
        self.uploaded_files_lock = threading.Lock()

        run_id = time.strftime("%Y%m%d_%H%M%S")
        self.metrics_log = Path(f"/app/logs/soak_metrics_{run_id}.csv")

        self.registered_ips: list[str] = []
        self.machines_session: requests.Session | None = None
        self.files_token: str = ""

    def remaining(self) -> float:
        return self.total_real_seconds - (time.time() - self.start_real)

    def sim_elapsed_seconds(self) -> float:
        return (time.time() - self.start_real) * self.time_scale

    def sim_hour_of_day(self, sim_elapsed: float) -> float:
        h = (sim_elapsed / 3600) % 24
        return h + 24 if h < 0 else h

    def register_machine(self, ip: str, label: str):
        try:
            self.machines_session.post(f"{self.base}/core/machines", json={"ip": ip, "label": label}, timeout=10)
        except requests.exceptions.RequestException:
            pass
        self.registered_ips.append(ip)

    def log_uploaded_file(self, name: str):
        with self.uploaded_files_lock:
            with open(self.uploaded_files_log, "a") as f:
                f.write(name + "\n")

    def cleanup(self):
        print("Cleaning up: stopping simulated users and deregistering soak-test IPs...")
        self.stop_event.set()
        for ip in self.registered_ips:
            try:
                self.machines_session.delete(f"{self.base}/core/machines/{ip}", timeout=10)
            except requests.exceptions.RequestException:
                pass

        # Best-effort: delete every file this run uploaded into the live
        # store — same reasoning as test_suite.py's cleanup (issue #347):
        # a soak run can upload thousands of files over a long simulated
        # window, and nothing else removes them afterward.
        if self.uploaded_files_log.exists():
            print("Cleaning up: deleting uploaded test files from the live store...")
            files_session = requests.Session()
            files_session.headers.update({"X-Dewey-Session-Token": self.files_token})
            for name in self.uploaded_files_log.read_text().splitlines():
                if name:
                    try:
                        files_session.delete(f"{self.base}/core/files/{name}", timeout=10)
                    except requests.exceptions.RequestException:
                        pass
        shutil.rmtree(self.tmp_dir, ignore_errors=True)

    # --- one simulated user: uploads/retrieves on a randomized, diurnally-scaled cadence ---

    def simulate_user(self, user_id: int, ip: str):
        session = session_for_ip(self.files_token, ip)
        uploaded_file = ""
        user_dir = self.tmp_dir / f"user_{user_id}"
        user_dir.mkdir(parents=True, exist_ok=True)
        req_id = 0

        while not self.stop_event.is_set():
            remaining = self.remaining()
            if remaining <= 0:
                break

            sim_elapsed = self.sim_elapsed_seconds()
            hour = self.sim_hour_of_day(sim_elapsed)
            multiplier = rate_multiplier(hour)

            # mean real-seconds-between-requests = seconds_per_sim_day /
            # (peak rate * multiplier) — see PEAK_REQUESTS_PER_SIM_DAY
            # comment above for why this has to be derived this way rather
            # than a fixed simulated-seconds constant. Jittered +/-50%,
            # floored at 0.2s so an extreme compression factor can't spin
            # this into a tight loop, and capped at the run's remaining
            # real-time budget — at low (night) rates the jittered
            # interval can otherwise legitimately exceed however much time
            # is actually left, oversleeping straight past the run's
            # intended end since nothing wakes the loop early to recheck.
            jitter = 0.5 + random.random()
            sleep_s = (self.seconds_per_sim_day / (PEAK_REQUESTS_PER_SIM_DAY * multiplier)) * jitter
            sleep_s = max(sleep_s, 0.2)
            sleep_s = min(sleep_s, remaining)
            self.stop_event.wait(sleep_s)
            if self.stop_event.is_set():
                break

            # 70/30 upload-vs-retrieve mix; always upload until this user
            # has something of its own to retrieve.
            if not uploaded_file or random.random() < 0.7:
                req_id += 1
                f = user_dir / f"soak_{user_id}_{req_id}.txt"
                f.write_bytes(base64.b64encode(os.urandom(512)))
                try:
                    with open(f, "rb") as fh:
                        resp = session.post(
                            f"{self.base}/core/upload",
                            files={"file": (f.name, fh)},
                            data={"date_of_injury": rand_date()},
                            timeout=30,
                        )
                    name = resp.json().get("filename", "")
                except (requests.exceptions.RequestException, ValueError):
                    name = ""
                if name:
                    uploaded_file = name
                    self.log_uploaded_file(name)
                f.unlink(missing_ok=True)
            else:
                # meta is a query param, not a path segment (getFile,
                # backend/routes.go) — see load_test.py's retrieve_one for
                # the full explanation of this fix.
                try:
                    session.get(f"{self.base}/core/files/{uploaded_file}?meta=false", timeout=10)
                except requests.exceptions.RequestException:
                    pass

    # --- metrics logger: samples /metrics on a fixed real-time cadence, writes CSV ---

    def log_metrics(self, metrics_session: requests.Session):
        with open(self.metrics_log, "w") as f:
            f.write(
                "timestamp,sim_day,sim_hour,time_til_next_tick,cache_clean_cycles_total,"
                "ram_usage_mb,heap_usage_mb,goroutines,open_fds,connected_users,upload_rate\n"
            )

        while not self.stop_event.is_set():
            remaining = self.remaining()
            if remaining <= 0:
                break

            try:
                body = metrics_session.get(f"{self.base}/metrics", timeout=10).text
            except requests.exceptions.RequestException:
                body = ""
            sim_elapsed = self.sim_elapsed_seconds()
            hour = self.sim_hour_of_day(sim_elapsed)
            day = int(sim_elapsed / 86400)

            row = ",".join(
                [
                    str(int(time.time())),
                    str(day),
                    str(hour),
                    extract_metric("app_time_til_next_tick", body),
                    extract_metric("app_cache_clean_cycles_total", body),
                    extract_metric("app_ram_usage", body),
                    extract_metric("app_heap_usage", body),
                    extract_metric("go_goroutines", body),
                    extract_metric("process_open_fds", body),
                    extract_metric("app_connected_users", body),
                    extract_metric("app_upload_rate", body),
                ]
            )
            with open(self.metrics_log, "a") as f:
                f.write(row + "\n")

            # Capped at the remaining budget, same reasoning as
            # simulate_user's sleep above — a short total run shouldn't
            # wait a full fixed sample interval past its own intended end.
            sleep_s = min(METRICS_SAMPLE_REAL_SECONDS, remaining)
            self.stop_event.wait(sleep_s)

    def run(self):
        files_password = os.environ.get("FILES_PASSWORD", "")
        machines_password = os.environ.get("MACHINES_PASSWORD", "")

        self.files_token = exchange_session_token(self.base, files_password, "files")
        self.machines_session = requests.Session()
        self.machines_session.headers.update(
            {"X-Dewey-Session-Token": exchange_session_token(self.base, machines_password, "machines")}
        )

        print(
            f"Soak test: {self.sim_days} simulated days, {self.users} users, "
            f"{self.seconds_per_sim_day}s/simday ({self.time_scale:g}x compression) against {self.base}"
        )
        print(f"Metrics log: {self.metrics_log}")

        metrics_log_ip = "127.0.0.99"
        for i in range(1, self.users + 1):
            self.register_machine(f"127.0.0.{i + 1}", f"soak-user-{i}")
        self.register_machine(metrics_log_ip, "soak-metrics-logger")

        metrics_session = session_for_ip(self.files_token, metrics_log_ip)

        threads = [
            threading.Thread(target=self.simulate_user, args=(i, f"127.0.0.{i + 1}"), daemon=True)
            for i in range(1, self.users + 1)
        ]
        threads.append(threading.Thread(target=self.log_metrics, args=(metrics_session,), daemon=True))

        for t in threads:
            t.start()

        def handle_signal(signum, frame):
            self.stop_event.set()

        signal.signal(signal.SIGINT, handle_signal)
        signal.signal(signal.SIGTERM, handle_signal)

        for t in threads:
            t.join()

        print(f"Soak test complete. Metrics log: {self.metrics_log}")


def main() -> int:
    base = (sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8080").rstrip("/")
    sim_days = int(sys.argv[2]) if len(sys.argv) > 2 else 30
    users = int(sys.argv[3]) if len(sys.argv) > 3 else 5
    seconds_per_sim_day = int(sys.argv[4]) if len(sys.argv) > 4 else 60

    soak = SoakTest(base, sim_days, users, seconds_per_sim_day)
    try:
        soak.run()
    finally:
        soak.cleanup()
    return 0


if __name__ == "__main__":
    sys.exit(main())
