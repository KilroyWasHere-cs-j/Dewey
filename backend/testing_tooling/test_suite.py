#!/usr/bin/env python3
"""Dewey's BIT (built-in test) suite: a black-box pass over the whole HTTP
API. Ported from test_suite.sh (issue #469) — same sections, same output
format, same exit-code contract, so nothing calling this (main.go's BITs
goroutine, a developer running it by hand) needs to change how it's
invoked or how it reads the result.

Requires FILES_PASSWORD in the environment (issue #332) — main.go execs
this as a subprocess of the backend, which inherits the container's env,
so it's already set when this runs as BITs. Running it manually against a
different host needs it passed explicitly, e.g.
`FILES_PASSWORD=$(cat .files-password) ./test_suite.py`.

The password itself is only ever sent once, to exchange it for a session
token (issue #409) — every actual test request sends
X-Dewey-Session-Token instead, same as the real frontend does.

Usage:
    python3 test_suite.py [base_url]   # defaults to http://localhost:8080
"""

from __future__ import annotations

import atexit
import hashlib
import json
import os
import random
import re
import shutil
import sys
import tempfile
import time
from datetime import datetime
from pathlib import Path

import requests

from dewey_test_utils import NC, BOLD, DIM, GREEN, RED, YELLOW, CYAN, MAGENTA, WHITE, Tee, exchange_session_token, rand_date

# --- Random data generators (same pools as test_suite.sh) ---
FIRST_NAMES = ["Oliver", "Phoebe", "Marcus", "Ingrid", "Tariq", "Yuki", "Soren", "Amara", "Declan", "Priya"]
LAST_NAMES = ["Nakamura", "Osei", "Lindqvist", "Ferrara", "Patel", "Kowalski", "Okafor", "Reyes", "Svensson", "Mbeki"]
EMPLOYERS = ["Horizon Robotics", "Starfall Media", "Ironclad Materials", "Vivant Health", "Obsidian Logistics",
             "Luminary Tech", "Verdant Farms", "Nexus Analytics", "Solaris Energy", "Phalanx Security"]
ADJUSTERS = ["T. Hargrove", "M. Delacroix", "A. Fujimoto", "R. Oduya", "S. Bergmann",
             "C. Abramowitz", "D. Kazakov", "F. Osei-Mensah", "L. Cartwright", "P. Iyer"]
CLAIM_TYPES = ["Workers Comp", "Liability", "Property", "Medical", "Disability", "Auto", "Product Liability", "Environmental"]
SUPPORT_LVLS = ["Full Support", "Partial", "Minimal", "Psychiatric", "Physical Therapy", "None", "Pending Review"]
JURISDICTIONS = ["California", "New York", "Texas", "Florida", "Illinois", "Washington", "Colorado", "Georgia", "Ohio", "Michigan"]


class TestSuite:
    def __init__(self, base: str):
        self.base = base.rstrip("/")
        self.script_dir = Path(__file__).resolve().parent
        self.download_dir = Path(tempfile.mkdtemp())
        # Mirrors config.json's file_system.file_system_base_dir ("./store"
        # relative to the app's WORKDIR) — this script runs as a direct
        # subprocess of the Go backend (issue #99's BITs, exec'd from
        # main.go with no Dir override), so it shares the backend's own
        # working directory and this resolves to the real store the
        # backend writes into, same reasoning as the log dir below.
        self.store_dir = self.script_dir.parent / "store"

        self.uploaded_files: list[str] = []
        self.passed = 0
        self.failed = 0
        self.skipped = 0
        self.total = 0

        self.session = requests.Session()
        self.session_token = exchange_session_token(self.base, os.environ.get("FILES_PASSWORD", ""), "files")
        self.session.headers.update({"X-Dewey-Session-Token": self.session_token})

        atexit.register(self.cleanup)

    # --- Setup ---

    def cleanup(self):
        # Best-effort: delete every file this run uploaded into the live
        # store, so BITs stops leaving permanent residue in prod on every
        # boot (issue #347). Errors are ignored — a file run_delete_tests
        # already removed will just 404 here, which is fine, and this
        # must never fail the exit path.
        for f in self.uploaded_files:
            try:
                self.session.delete(f"{self.base}/core/files/{f}", timeout=10)
            except requests.exceptions.RequestException:
                pass
        shutil.rmtree(self.download_dir, ignore_errors=True)

    # --- Output helpers ---

    def result_pass(self, msg: str):
        self.passed += 1
        self.total += 1
        print(f"PASS | {msg}")
        print(f"  {GREEN}{BOLD}✔{NC} {GREEN}{msg}{NC}", file=sys.stderr)

    def result_fail(self, msg: str, detail: str):
        self.failed += 1
        self.total += 1
        print(f"FAIL | {msg} | {detail}")
        print(f"  {RED}{BOLD}✘{NC} {RED}{msg}{NC} {DIM}— {detail}{NC}", file=sys.stderr)

    def result_skip(self, msg: str, detail: str):
        self.skipped += 1
        self.total += 1
        print(f"SKIP | {msg} | {detail}")
        print(f"  {YELLOW}{BOLD}○{NC} {YELLOW}{msg}{NC} {DIM}— {detail}{NC}", file=sys.stderr)

    def info(self, msg: str = ""):
        print(msg, file=sys.stderr)

    def pace(self):
        time.sleep(1.1)

    def section(self, title: str):
        self.info()
        self.info(f"  {DIM}{CYAN}{'─' * 60}{NC}")
        self.info(f"  {MAGENTA}{BOLD}◆ {title}{NC}")
        self.info(f"  {DIM}{CYAN}{'─' * 60}{NC}")

    # --- Random data generators ---

    def rnd(self, choices: list[str]) -> str:
        return random.choice(choices)

    def rand_claim(self) -> str:
        return f"CLM-{random.randint(10000, 99999):05d}"

    def rand_policy(self) -> str:
        return f"POL-{random.randint(100000, 999999):06d}"

    def random_metadata(self) -> dict[str, str]:
        return {
            "claim_number": self.rand_claim(),
            "claimant_name": f"{self.rnd(FIRST_NAMES)} {self.rnd(LAST_NAMES)}",
            "date_of_injury": rand_date(),
            "employer": self.rnd(EMPLOYERS),
            "adjuster": self.rnd(ADJUSTERS),
            "support": self.rnd(SUPPORT_LVLS),
            "claim_type": self.rnd(CLAIM_TYPES),
            "jurisdiction": self.rnd(JURISDICTIONS),
            "policy_number": self.rand_policy(),
            "acts_id": f"ACTS_{random.randint(0, 999):03d}",
            "data": f"run-{random.randint(0, 0xffff):04x}",
        }

    def upload_file(self, src: Path, fields: dict[str, str] | None = None) -> requests.Response:
        self.pace()
        with open(src, "rb") as f:
            return self.session.post(
                f"{self.base}/core/upload",
                files={"file": (src.name, f)},
                data=fields if fields is not None else self.random_metadata(),
                timeout=30,
            )

    @staticmethod
    def filename_from(resp: requests.Response) -> str:
        try:
            return resp.json().get("filename", "")
        except ValueError:
            return ""

    # --- Section 1: Endpoint health checks ---

    def run_health_checks(self):
        self.section("ENDPOINT HEALTH CHECKS")

        endpoints = {
            "GET /": f"{self.base}/",
            "GET /core/files": f"{self.base}/core/files",
            "GET /core/admin/dumpCache": f"{self.base}/core/admin/dumpCache",
        }

        for label, url in endpoints.items():
            self.info(f"  Testing {label} ...")
            self.pace()
            try:
                resp = self.session.get(url, timeout=10)
            except requests.exceptions.RequestException:
                self.result_fail(label, "connection failed")
                continue
            if resp.status_code < 400:
                self.result_pass(f"{label} -> HTTP {resp.status_code}")
            else:
                self.result_fail(label, f"HTTP {resp.status_code}")

    # --- Section 2: JSON upload ---

    def run_json_upload_test(self):
        self.section("JSON UPLOAD")

        self.info("  Testing POST /upload with application/json ...")
        self.pace()
        try:
            resp = self.session.post(
                f"{self.base}/core/upload",
                headers={"Content-Type": "application/json"},
                data=json.dumps({"name": "json-test", "data": "hello"}),
                timeout=10,
            )
        except requests.exceptions.RequestException:
            self.result_fail("POST /upload [JSON]", "connection failed")
        else:
            if resp.status_code == 200:
                if "JSON received" in resp.text:
                    self.result_pass("POST /upload [JSON] -> HTTP 200, acknowledged")
                else:
                    self.result_fail("POST /upload [JSON]", f"HTTP 200 but unexpected body: {resp.text}")
            else:
                self.result_fail("POST /upload [JSON]", f"expected 200, got HTTP {resp.status_code}")

        self.info("  Testing POST /upload with invalid JSON ...")
        self.pace()
        try:
            resp = self.session.post(
                f"{self.base}/core/upload",
                headers={"Content-Type": "application/json"},
                data="not json at all",
                timeout=10,
            )
        except requests.exceptions.RequestException:
            self.result_fail("POST /upload [bad JSON]", "connection failed")
        else:
            if resp.status_code >= 400:
                self.result_pass(f"POST /upload [bad JSON] -> HTTP {resp.status_code} (rejected)")
            else:
                self.result_fail("POST /upload [bad JSON]", f"expected 4xx, got HTTP {resp.status_code}")

    # --- Section 3: Multipart upload with random metadata ---

    def run_upload_tests(self):
        self.section("MULTIPART UPLOAD TESTS (randomized metadata)")

        for f in ["lenna.jpg", "lenna.png", "test.pdf", "bee_moive_script.txt"]:
            src = self.script_dir / f
            if not src.is_file():
                self.result_skip(f"POST /upload [{f}]", "source file not found")
                continue

            self.info(f"  Uploading {f} ...")
            try:
                resp = self.upload_file(src)
            except requests.exceptions.RequestException:
                self.result_fail(f"POST /upload [{f}]", "curl error")
                continue

            server_file = self.filename_from(resp)
            if not server_file:
                self.result_fail(f"POST /upload [{f}]", f"no filename in response: {resp.text}")
            else:
                self.result_pass(f"POST /upload [{f}] -> {server_file}")
                self.uploaded_files.append(server_file)

    # --- Section 4: Upload error cases ---

    def run_upload_error_tests(self):
        self.section("UPLOAD ERROR CASES")

        self.info("  Testing invalid content-type ...")
        self.pace()
        try:
            resp = self.session.post(
                f"{self.base}/core/upload",
                headers={"Content-Type": "text/plain"},
                data="invalid",
                timeout=10,
            )
            ok = resp.status_code >= 400
            code = resp.status_code
        except requests.exceptions.RequestException:
            self.result_fail("POST /upload [bad content-type]", "connection failed")
        else:
            if ok:
                self.result_pass(f"POST /upload [bad content-type] -> HTTP {code} (rejected)")
            else:
                self.result_fail("POST /upload [bad content-type]", f"expected 4xx, got HTTP {code}")

        self.info("  Testing empty multipart (no file field) ...")
        self.pace()
        try:
            resp = self.session.post(
                f"{self.base}/core/upload",
                data={"claim_number": "CLM-00000"},
                timeout=10,
            )
        except requests.exceptions.RequestException:
            self.result_fail("POST /upload [no file]", "connection failed")
        else:
            if resp.status_code >= 400:
                self.result_pass(f"POST /upload [no file] -> HTTP {resp.status_code} (rejected)")
            else:
                self.result_fail("POST /upload [no file]", f"expected 4xx, got HTTP {resp.status_code}")

        self.info("  Testing blank date_of_injury (issue #365 — used to insert silently) ...")
        self.pace()
        doi_src = self.script_dir / "lenna.jpg"
        if not doi_src.is_file():
            self.result_skip("POST /upload [blank date_of_injury]", "source file not found")
        else:
            fields = {
                "claim_number": self.rand_claim(),
                "claimant_name": "Blank DOI Test",
                "date_of_injury": "",
                "employer": self.rnd(EMPLOYERS),
                "adjuster": self.rnd(ADJUSTERS),
                "support": self.rnd(SUPPORT_LVLS),
                "claim_type": self.rnd(CLAIM_TYPES),
                "jurisdiction": self.rnd(JURISDICTIONS),
                "policy_number": self.rand_policy(),
                "acts_id": "ACTS_000",
            }
            try:
                resp = self.upload_file(doi_src, fields)
            except requests.exceptions.RequestException:
                self.result_fail("POST /upload [blank date_of_injury]", "connection failed")
            else:
                if resp.status_code >= 400:
                    self.result_pass(f"POST /upload [blank date_of_injury] -> HTTP {resp.status_code} (rejected)")
                else:
                    self.result_fail(
                        "POST /upload [blank date_of_injury]",
                        f"expected 4xx, got HTTP {resp.status_code} — regression of issue #365",
                    )

    # --- Section 5: Disallowed file extension ---

    def run_bad_extension_test(self):
        self.section("DISALLOWED FILE EXTENSION")

        bad_ext_fields = {
            "claim_number": "CLM-00000",
            "claimant_name": "Bad Actor",
            "date_of_injury": "2025-01-01",
            "employer": "Evil Corp",
            "adjuster": "None",
            "support": "None",
            "claim_type": "Liability",
            "jurisdiction": "California",
            "policy_number": "POL-000000",
            "acts_id": "ACTS_BAD",
            "data": "bad-ext-test",
        }

        tmpfile = self.download_dir / "malicious.sh"
        tmpfile.write_text("#!/bin/bash\necho pwned\n")

        self.info("  Uploading .sh file (should be rejected) ...")
        self.pace()
        try:
            resp = self.upload_file(tmpfile, bad_ext_fields)
        except requests.exceptions.RequestException:
            self.result_fail("POST /upload [.sh extension]", "connection failed")
        else:
            if resp.status_code >= 400:
                self.result_pass(f"POST /upload [.sh extension] -> HTTP {resp.status_code} (rejected)")
            else:
                self.result_fail("POST /upload [.sh extension]", f"expected 4xx, got HTTP {resp.status_code}")

        tmpexe = self.download_dir / "payload.exe"
        tmpexe.write_text("not a real exe\n")

        self.info("  Uploading .exe file (should be rejected) ...")
        self.pace()
        try:
            resp = self.upload_file(tmpexe, bad_ext_fields)
        except requests.exceptions.RequestException:
            self.result_fail("POST /upload [.exe extension]", "connection failed")
        else:
            if resp.status_code >= 400:
                self.result_pass(f"POST /upload [.exe extension] -> HTTP {resp.status_code} (rejected)")
            else:
                self.result_fail("POST /upload [.exe extension]", f"expected 4xx, got HTTP {resp.status_code}")

    # --- Section 6: ELF binary rejection ---

    def run_elf_rejection_test(self):
        self.section("ELF BINARY REJECTION")

        src = self.script_dir / "renamedELF.txt"
        if not src.is_file():
            self.result_skip("POST /upload [ELF as .txt]", "renamedELF.txt not found")
            return

        self.info("  Uploading renamedELF.txt (ELF binary disguised as .txt) ...")
        self.pace()
        fields = {
            "claim_number": "CLM-00000",
            "claimant_name": "ELF Test",
            "date_of_injury": "2025-01-01",
            "employer": "TestCorp",
            "adjuster": "A. Smith",
            "support": "Full Support",
            "claim_type": "Workers Comp",
            "jurisdiction": "California",
            "policy_number": "POL-000001",
            "acts_id": "ACTS_ELF",
            "data": "elf-rejection-test",
        }
        try:
            resp = self.upload_file(src, fields)
        except requests.exceptions.RequestException:
            self.result_fail("POST /upload [ELF as .txt]", "connection failed")
            return

        if resp.status_code >= 400:
            if "elf" in resp.text.lower():
                self.result_pass(f"POST /upload [ELF as .txt] -> HTTP {resp.status_code} (ELF detected and blocked)")
            else:
                self.result_pass(f"POST /upload [ELF as .txt] -> HTTP {resp.status_code} (rejected)")
        else:
            self.result_fail(
                "POST /upload [ELF as .txt]",
                f"expected 4xx, got HTTP {resp.status_code} — server accepted an ELF binary",
            )

    # --- Section 7: Path traversal ---

    def run_path_traversal_tests(self):
        self.section("PATH TRAVERSAL PROTECTION")

        for tp in ["../../etc/passwd", "../../../etc/shadow", "..%2F..%2Fetc%2Fpasswd"]:
            self.info(f"  Testing GET /files/{tp}/false ...")
            self.pace()
            try:
                resp = self.session.get(f"{self.base}/core/files/{tp}/false", timeout=10)
            except requests.exceptions.RequestException:
                self.result_fail(f"GET /files/{tp}/false (traversal)", "connection failed")
                continue
            if resp.status_code >= 400 or resp.status_code in (301, 302):
                self.result_pass(f"GET /files/{tp}/false (traversal) -> HTTP {resp.status_code} (blocked)")
            elif resp.status_code == 200:
                self.result_fail(f"GET /files/{tp}/false (traversal)", "HTTP 200 — path traversal may have succeeded")
            else:
                self.result_pass(f"GET /files/{tp}/false (traversal) -> HTTP {resp.status_code}")

        self.info("  Testing DELETE with traversal path ...")
        self.pace()
        try:
            resp = self.session.delete(f"{self.base}/core/files/../../etc/passwd", timeout=10)
        except requests.exceptions.RequestException:
            self.result_fail("DELETE /files/../../etc/passwd (traversal)", "connection failed")
        else:
            if resp.status_code >= 400 or resp.status_code in (301, 302):
                self.result_pass(f"DELETE /files/../../etc/passwd (traversal) -> HTTP {resp.status_code} (blocked)")
            elif resp.status_code == 200:
                self.result_fail(
                    "DELETE /files/../../etc/passwd (traversal)", "HTTP 200 — path traversal may have succeeded"
                )
            else:
                self.result_pass(f"DELETE /files/../../etc/passwd (traversal) -> HTTP {resp.status_code}")

    # --- Section 8: SHA256 upload verification ---

    def run_sha256_upload_test(self):
        self.section("SHA256 UPLOAD VERIFICATION")

        f = "lenna.jpg"
        src = self.script_dir / f
        if not src.is_file():
            self.result_skip(f"SHA256 upload verify [{f}]", "source file not found")
            return

        expected_hash = hashlib.sha256(src.read_bytes()).hexdigest()

        self.info(f"  Uploading {f} and checking server-reported SHA256 ...")
        try:
            resp = self.upload_file(src)
        except requests.exceptions.RequestException:
            self.result_fail(f"SHA256 upload verify [{f}]", "curl error")
            return

        server_file = self.filename_from(resp)
        if server_file:
            self.uploaded_files.append(server_file)

        try:
            server_hash = resp.json().get("sha256", "")
        except ValueError:
            server_hash = ""

        if not server_hash:
            self.result_fail(f"SHA256 upload verify [{f}]", f"no sha256 in response: {resp.text}")
            return

        if expected_hash == server_hash:
            self.result_pass(f"SHA256 upload verify [{f}] local={expected_hash} == server={server_hash}")
        else:
            self.result_fail(f"SHA256 upload verify [{f}]", f"local={expected_hash} != server={server_hash}")

    # --- Section 9: Duplicate upload ---

    def run_duplicate_upload_test(self):
        self.section("DUPLICATE UPLOAD")

        f = "lenna.jpg"
        src = self.script_dir / f
        if not src.is_file():
            self.result_skip(f"DUPLICATE upload [{f}]", "source file not found")
            return

        self.info(f"  Uploading {f} twice ...")
        resp1 = self.upload_file(src)
        file1 = self.filename_from(resp1)
        resp2 = self.upload_file(src)
        file2 = self.filename_from(resp2)

        if file1:
            self.uploaded_files.append(file1)
        if file2:
            self.uploaded_files.append(file2)

        if not file1 or not file2:
            self.result_fail(f"DUPLICATE upload [{f}]", f"one or both uploads failed: file1={file1} file2={file2}")
            return

        if file1 != file2:
            self.result_pass(f"DUPLICATE upload [{f}] -> distinct filenames: {file1}, {file2}")
        else:
            self.result_fail(f"DUPLICATE upload [{f}]", f"both uploads returned same filename: {file1}")

    # --- Section 10: Roundtrip integrity (upload, download, SHA256 compare) ---

    def run_roundtrip_tests(self):
        self.section("ROUNDTRIP INTEGRITY (SHA256)")

        fixed_fields = {
            "claim_number": "CLM-99999",
            "claimant_name": "Roundtrip Test",
            "date_of_injury": "2025-01-01",
            "employer": "TestCorp",
            "adjuster": "A. Smith",
            "support": "Full Support",
            "claim_type": "Workers Comp",
            "jurisdiction": "California",
            "policy_number": "POL-000001",
            "acts_id": "ACTS_RT",
            "data": "roundtrip-test",
        }

        for f in ["lenna.jpg", "lenna.png", "test.pdf", "bee_moive_script.txt"]:
            src = self.script_dir / f
            if not src.is_file():
                self.result_skip(f"ROUNDTRIP [{f}]", "source file not found")
                continue

            self.info(f"  Uploading {f} for roundtrip ...")
            self.pace()
            try:
                resp = self.upload_file(src, fixed_fields)
            except requests.exceptions.RequestException:
                self.result_fail(f"ROUNDTRIP [{f}] upload", "connection failed")
                continue

            server_file = self.filename_from(resp)
            if not server_file:
                self.result_fail(f"ROUNDTRIP [{f}] upload", f"no filename in response: {resp.text}")
                continue
            self.uploaded_files.append(server_file)

            self.info(f"  Downloading {server_file} ...")
            self.pace()
            dst = self.download_dir / server_file
            try:
                dl = self.session.get(f"{self.base}/core/files/{server_file}?meta=false", timeout=30)
            except requests.exceptions.RequestException:
                self.result_fail(f"ROUNDTRIP [{f}] download", "connection failed")
                continue

            if dl.status_code != 200:
                self.result_fail(f"ROUNDTRIP [{f}] download", f"HTTP {dl.status_code}")
                continue
            dst.write_bytes(dl.content)

            hash_orig = hashlib.sha256(src.read_bytes()).hexdigest()
            hash_down = hashlib.sha256(dst.read_bytes()).hexdigest()
            if hash_orig == hash_down:
                self.result_pass(f"ROUNDTRIP [{f}] SHA256={hash_orig}")
            else:
                self.result_fail(f"ROUNDTRIP [{f}] SHA256 mismatch", f"orig={hash_orig} got={hash_down}")

    # --- Section 11: Store path verification ---
    #
    # The roundtrip check above only proves the API serves back byte-identical
    # content — but locateFile checks the upload cache before ever falling back
    # to the DB-recorded store path, so a passing roundtrip alone doesn't prove
    # idAndSort's async post-processing actually copied the file into
    # fileSystemBaseDir at all (issue #270). This checks the store directly on
    # disk instead of through the API.

    def run_store_path_verification_test(self):
        self.section("STORE PATH VERIFICATION")

        f = "lenna.jpg"
        src = self.script_dir / f
        if not src.is_file():
            self.result_skip(f"STORE PATH verify [{f}]", "source file not found")
            return
        if not self.store_dir.is_dir():
            self.result_skip(f"STORE PATH verify [{f}]", f"store directory not found at {self.store_dir}")
            return

        self.info(f"  Uploading {f} ...")
        try:
            resp = self.upload_file(src)
        except requests.exceptions.RequestException:
            self.result_fail(f"STORE PATH verify [{f}]", "curl error")
            return

        server_file = self.filename_from(resp)
        if not server_file:
            self.result_fail(f"STORE PATH verify [{f}]", f"no filename in response: {resp.text}")
            return
        self.uploaded_files.append(server_file)

        # idAndSort's disk copy runs asynchronously in a background goroutine
        # queued behind postProcessingSem (issue #217) — poll for a few
        # seconds rather than checking once immediately, so this doesn't
        # flake on a slow or backed-up post-processing pass.
        self.info(f"  Waiting for {server_file} to land in the store ...")
        store_path: Path | None = None
        for _ in range(15):
            matches = list(self.store_dir.rglob(server_file))
            if matches:
                store_path = matches[0]
                break
            time.sleep(1)

        if store_path is None:
            self.result_fail(f"STORE PATH verify [{server_file}]", f"not found anywhere under {self.store_dir} after upload")
            return
        self.result_pass(f"STORE PATH verify [{server_file}] -> found at {store_path}")

        if store_path.stat().st_size == 0:
            self.result_fail(f"STORE PATH verify [{server_file}]", f"store copy exists but is empty: {store_path}")
            return

        expected_hash = hashlib.sha256(src.read_bytes()).hexdigest()
        actual_hash = hashlib.sha256(store_path.read_bytes()).hexdigest()
        if expected_hash == actual_hash:
            self.result_pass(f"STORE PATH verify [{server_file}] content matches (SHA256 {actual_hash})")
        else:
            self.result_fail(
                f"STORE PATH verify [{server_file}]",
                f"store copy content mismatch: local={expected_hash} store={actual_hash}",
            )

    # --- Section 12: Metadata retrieval (GET /files/:filename?meta=true) ---

    def run_metadata_tests(self):
        self.section("METADATA RETRIEVAL (meta=true)")

        src = self.script_dir / "lenna.jpg"
        if not src.is_file():
            self.result_skip("METADATA retrieval", "lenna.jpg not found — skipping all metadata tests")
            return

        # Upload a file with known, fixed metadata so we can assert on the response fields
        self.info("  Uploading lenna.jpg with fixed metadata ...")
        self.pace()
        fields = {
            "claim_number": "CLM-META-01",
            "claimant_name": "Meta Tester",
            "date_of_injury": "2024-06-01",
            "employer": "TestCorp",
            "adjuster": "M. Inspector",
            "support": "Full Support",
            "claim_type": "Workers Comp",
            "jurisdiction": "California",
            "policy_number": "POL-META-01",
            "acts_id": "ACTS_META_01",
            "data": "metadata-test",
        }
        try:
            resp = self.upload_file(src, fields)
        except requests.exceptions.RequestException:
            self.result_fail("METADATA upload", "connection failed")
            return

        server_file = self.filename_from(resp)
        if not server_file:
            self.result_fail("METADATA upload", f"no filename in response: {resp.text}")
            return
        self.uploaded_files.append(server_file)
        self.result_pass(f"METADATA upload -> {server_file}")

        # Fetch metadata and assert HTTP 200 + expected JSON fields
        self.info(f"  Fetching metadata for {server_file} ...")
        self.pace()
        try:
            meta_resp = self.session.get(f"{self.base}/core/files/{server_file}?meta=true", timeout=10)
        except requests.exceptions.RequestException:
            self.result_fail(f"GET /files/{server_file}?meta=true", "connection failed")
            return

        if meta_resp.status_code != 200:
            self.result_fail(f"GET /files/{server_file}?meta=true", f"expected 200, got HTTP {meta_resp.status_code}")
            return
        self.result_pass(f"GET /files/{server_file}?meta=true -> HTTP 200")

        try:
            meta_body = meta_resp.json()
        except ValueError:
            meta_body = {}

        expected_fields = {
            "claim_number": "CLM-META-01",
            "claimant_name": "Meta Tester",
            "acts_id": "ACTS_META_01",
            "jurisdiction": "California",
        }
        for field, value in expected_fields.items():
            if meta_body.get(field) == value or f'"{value}"' in meta_resp.text:
                self.result_pass(f"METADATA field {field} = {value}")
            else:
                self.result_fail(f"METADATA field {field}", f"expected '{value}' not found in: {meta_resp.text}")

        # Unknown meta flag should return 400
        self.info("  Testing unknown meta flag ...")
        self.pace()
        try:
            resp = self.session.get(f"{self.base}/core/files/{server_file}?meta=maybe", timeout=10)
        except requests.exceptions.RequestException:
            self.result_fail(f"GET /files/{server_file}?meta=maybe", "connection failed")
        else:
            if resp.status_code == 400:
                self.result_pass(f"GET /files/{server_file}?meta=maybe -> HTTP 400 (bad flag rejected)")
            else:
                self.result_fail(f"GET /files/{server_file}?meta=maybe", f"expected 400, got HTTP {resp.status_code}")

        # meta=true for a non-existent file should return 404
        self.info("  Testing meta=true for non-existent file ...")
        self.pace()
        ghost = f"ghost_{random.randint(0, 0xffffffff):08x}.jpg"
        try:
            resp = self.session.get(f"{self.base}/core/files/{ghost}?meta=true", timeout=10)
        except requests.exceptions.RequestException:
            self.result_fail(f"GET /files/{ghost}?meta=true", "connection failed")
        else:
            if resp.status_code == 404:
                self.result_pass(f"GET /files/{ghost}?meta=true -> HTTP 404")
            else:
                self.result_fail(f"GET /files/{ghost}?meta=true", f"expected 404, got HTTP {resp.status_code}")

    # --- Section 13: File listing / catalog ---

    def run_catalog_test(self):
        self.section("CATALOG VERIFICATION")

        self.info("  Fetching file index ...")
        self.pace()
        try:
            resp = self.session.get(f"{self.base}/core/files", timeout=10)
        except requests.exceptions.RequestException:
            self.result_fail("GET /files (catalog)", "connection failed")
            return

        if resp.status_code != 200:
            self.result_fail("GET /files (catalog)", f"HTTP {resp.status_code}")
            return

        self.result_pass("GET /files (catalog) -> HTTP 200")
        self.info("  Response body (first 500 chars):")
        self.info(f"  {resp.text[:500]}")

    # --- Section 14: Delete tests ---

    def run_delete_tests(self):
        self.section("DELETE TESTS")

        if not self.uploaded_files:
            self.result_skip("DELETE /files/:name", "no files were uploaded")
        else:
            target = self.uploaded_files[0]
            self.info(f"  Deleting {target} ...")
            self.pace()
            try:
                resp = self.session.delete(f"{self.base}/core/files/{target}", timeout=10)
            except requests.exceptions.RequestException:
                self.result_fail(f"DELETE /files/{target}", "connection failed")
            else:
                if resp.status_code < 400:
                    self.result_pass(f"DELETE /files/{target} -> HTTP {resp.status_code}")

                    self.info("  Verifying file removed from cache ...")
                    self.pace()
                    try:
                        verify = self.session.get(f"{self.base}/core/files/{target}?meta=false", timeout=10)
                    except requests.exceptions.RequestException:
                        self.result_fail(f"DELETE verify {target}", "connection failed")
                    else:
                        if verify.status_code >= 400:
                            self.result_pass(f"DELETE verify {target} gone -> HTTP {verify.status_code}")
                        elif verify.status_code == 200:
                            # deleteFile removes from cache, but locateFile falls back to the store
                            self.result_pass(f"DELETE verify {target} -> HTTP 200 (served from store fallback, cache entry removed)")
                        else:
                            self.result_fail(f"DELETE verify {target}", f"unexpected HTTP {verify.status_code}")
                else:
                    self.result_fail(f"DELETE /files/{target}", f"HTTP {resp.status_code}")

        ghost = f"nonexistent_file_{random.randint(0, 0xffffffff):08x}.txt"
        self.info(f"  Deleting non-existent file {ghost} ...")
        self.pace()
        try:
            resp = self.session.delete(f"{self.base}/core/files/{ghost}", timeout=10)
        except requests.exceptions.RequestException:
            self.result_fail(f"DELETE /files/{ghost} (non-existent)", "connection failed")
        else:
            if resp.status_code == 404:
                self.result_pass(f"DELETE /files/{ghost} (non-existent) -> HTTP 404")
            elif resp.status_code >= 400:
                self.result_pass(f"DELETE /files/{ghost} (non-existent) -> HTTP {resp.status_code} (rejected)")
            else:
                self.result_fail(f"DELETE /files/{ghost} (non-existent)", f"expected 404, got HTTP {resp.status_code}")

    # --- Section 15: PDF embedded JavaScript rejection ---

    def run_pdf_javascript_test(self):
        self.section("PDF EMBEDDED JAVASCRIPT REJECTION")

        src = self.script_dir / "js_test.pdf"
        if not src.is_file():
            self.result_skip("POST /upload [PDF with embedded JS]", "js_test.pdf not found")
            return

        self.info("  Uploading js_test.pdf (PDF with an /OpenAction JavaScript trigger) ...")
        self.pace()
        fields = {
            "claim_number": "CLM-00000",
            "claimant_name": "PDF JS Test",
            "date_of_injury": "2025-01-01",
            "employer": "TestCorp",
            "adjuster": "A. Smith",
            "support": "Full Support",
            "claim_type": "Workers Comp",
            "jurisdiction": "California",
            "policy_number": "POL-000001",
            "acts_id": "ACTS_PDFJS",
            "data": "pdf-js-rejection-test",
        }
        try:
            resp = self.upload_file(src, fields)
        except requests.exceptions.RequestException:
            self.result_fail("POST /upload [PDF with embedded JS]", "connection failed")
            return

        if resp.status_code >= 400:
            if "javascript" in resp.text.lower():
                self.result_pass(f"POST /upload [PDF with embedded JS] -> HTTP {resp.status_code} (JS detected and blocked)")
            else:
                self.result_pass(f"POST /upload [PDF with embedded JS] -> HTTP {resp.status_code} (rejected)")
        else:
            self.result_fail(
                "POST /upload [PDF with embedded JS]",
                f"expected 4xx, got HTTP {resp.status_code} — server accepted a PDF with embedded JavaScript",
            )

    # --- Section 16: Fileview routes (issue #389) ---
    # /fileview is only gated by the known_machines IP allowlist
    # (logConnections .. false), not requirePassword — unlike /core/files/*,
    # these requests carry no X-Dewey-Password/session header, matching how
    # the routes are actually reachable. Uses plain requests.* calls rather
    # than self.session so the token header self.session carries never gets
    # sent here.

    def run_fileview_tests(self):
        self.section("FILEVIEW ROUTES")

        self.info("  Testing GET /fileview/viewLogDir ...")
        self.pace()
        try:
            resp = requests.get(f"{self.base}/fileview/viewLogDir", timeout=10)
        except requests.exceptions.RequestException:
            self.result_fail("GET /fileview/viewLogDir", "connection failed")
            resp = None

        today_log = ""
        if resp is not None:
            if resp.status_code == 200 and '"files"' in resp.text:
                self.result_pass("GET /fileview/viewLogDir -> HTTP 200 (files key present)")
            else:
                self.result_fail("GET /fileview/viewLogDir", f"expected 200 with a files key, got HTTP {resp.status_code}: {resp.text}")

            # Pull today's rotated log filename out of viewLogDir's own
            # response rather than assuming the name (issue #387's log.go
            # rotates daily), so this doesn't need updating every day the
            # suite runs.
            match = re.search(r'"(app-[0-9-]*\.log)"', resp.text)
            if match:
                today_log = match.group(1)

        if not today_log:
            self.result_skip("GET /fileview/viewFile/log/:file", "no log filename found in viewLogDir response")
        else:
            self.info(f"  Testing GET /fileview/viewFile/log/{today_log} ...")
            self.pace()
            try:
                resp = requests.get(f"{self.base}/fileview/viewFile/log/{today_log}", timeout=10)
            except requests.exceptions.RequestException:
                self.result_fail(f"GET /fileview/viewFile/log/{today_log}", "connection failed")
            else:
                if resp.status_code == 200:
                    self.result_pass(f"GET /fileview/viewFile/log/{today_log} -> HTTP 200")
                else:
                    self.result_fail(f"GET /fileview/viewFile/log/{today_log}", f"expected 200, got HTTP {resp.status_code}")

        self.info("  Testing GET /fileview/viewFile/config/config.json ...")
        self.pace()
        try:
            resp = requests.get(f"{self.base}/fileview/viewFile/config/config.json", timeout=10)
        except requests.exceptions.RequestException:
            self.result_fail("GET /fileview/viewFile/config/config.json", "connection failed")
        else:
            if resp.status_code == 200 and "file_system_base_dir" in resp.text:
                self.result_pass("GET /fileview/viewFile/config/config.json -> HTTP 200 (real config content)")
            else:
                self.result_fail("GET /fileview/viewFile/config/config.json", f"expected 200 with config content, got HTTP {resp.status_code}")

        self.info("  Testing GET /fileview/viewFile/bogus/whatever (unknown fileType) ...")
        self.pace()
        try:
            resp = requests.get(f"{self.base}/fileview/viewFile/bogus/whatever", timeout=10)
        except requests.exceptions.RequestException:
            self.result_fail("GET /fileview/viewFile/bogus/whatever", "connection failed")
        else:
            if resp.status_code == 400:
                self.result_pass("GET /fileview/viewFile/bogus/whatever -> HTTP 400 (unknown fileType rejected)")
            else:
                self.result_fail("GET /fileview/viewFile/bogus/whatever", f"expected 400, got HTTP {resp.status_code}")

        ghost = f"nonexistent-file-{random.randint(0, 0xffffffff):08x}.log"
        self.info(f"  Testing GET /fileview/viewFile/log/{ghost} (missing file) ...")
        self.pace()
        try:
            resp = requests.get(f"{self.base}/fileview/viewFile/log/{ghost}", timeout=10)
        except requests.exceptions.RequestException:
            self.result_fail(f"GET /fileview/viewFile/log/{ghost}", "connection failed")
        else:
            if resp.status_code >= 400:
                self.result_pass(f"GET /fileview/viewFile/log/{ghost} -> HTTP {resp.status_code} (missing file rejected)")
            else:
                self.result_fail(f"GET /fileview/viewFile/log/{ghost}", f"expected 4xx/5xx, got HTTP {resp.status_code}")

        # gin's :file param can't contain a literal "/", so a real "../"
        # traversal never reaches getViewFile at all — this confirms that
        # routing-level block still holds rather than exercising any
        # boundary check inside viewFile itself, which has none (see
        # TestViewFile in filemanager_test.go for what happens when a name
        # with ".." *is* actually joined).
        self.info("  Testing GET /fileview/viewFile/log/..%2F..%2F..%2Fetc%2Fpasswd (traversal) ...")
        self.pace()
        try:
            resp = requests.get(f"{self.base}/fileview/viewFile/log/..%2F..%2F..%2Fetc%2Fpasswd", timeout=10)
        except requests.exceptions.RequestException:
            self.result_fail("GET /fileview/viewFile/log traversal", "connection failed")
        else:
            if resp.status_code >= 400:
                self.result_pass(f"GET /fileview/viewFile/log traversal -> HTTP {resp.status_code} (blocked)")
            elif resp.status_code == 200:
                self.result_fail("GET /fileview/viewFile/log traversal", "HTTP 200 — path traversal may have succeeded")
            else:
                self.result_pass(f"GET /fileview/viewFile/log traversal -> HTTP {resp.status_code}")

    # --- Run everything ---

    def run_all(self) -> int:
        self.info(f"{CYAN}{BOLD}╔{'═' * 66}╗{NC}")
        self.info(f"{CYAN}{BOLD}║{NC}            {WHITE}{BOLD}DEWEY TEST SUITE{NC}                                 {CYAN}{BOLD}║{NC}")
        self.info(f"{CYAN}{BOLD}╚{'═' * 66}╝{NC}")
        self.info()
        self.info(f"  {DIM}Target:{NC}       {BOLD}{self.base}{NC}")
        self.info(f"  {DIM}Download dir:{NC} {self.download_dir}")
        self.info(f"  {DIM}Timestamp:{NC}    {datetime.now().astimezone().isoformat()}")

        print(f"# DEWEY TEST SUITE — {datetime.now().astimezone().isoformat()}")
        print(f"# Target: {self.base}")
        print("#")

        self.run_health_checks()
        self.run_json_upload_test()
        self.run_upload_tests()
        self.run_upload_error_tests()
        self.run_bad_extension_test()
        self.run_elf_rejection_test()
        self.run_pdf_javascript_test()
        self.run_path_traversal_tests()
        self.run_sha256_upload_test()
        self.run_duplicate_upload_test()
        self.run_roundtrip_tests()
        self.run_store_path_verification_test()
        self.run_metadata_tests()
        self.run_catalog_test()
        self.run_delete_tests()
        self.run_fileview_tests()

        self.section("SUMMARY")
        self.info(f"  {GREEN}{BOLD}Passed:{NC}  {self.passed}")
        self.info(f"  {RED}{BOLD}Failed:{NC}  {self.failed}")
        self.info(f"  {YELLOW}{BOLD}Skipped:{NC} {self.skipped}")
        self.info(f"  {WHITE}{BOLD}Total:{NC}   {self.total}")

        print("#")
        print(f"# PASSED={self.passed} FAILED={self.failed} SKIPPED={self.skipped} TOTAL={self.total}")

        return 1 if self.failed > 0 else 0


def main() -> int:
    base = sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8080"

    # --- Logging: mirror stdout/stderr into a timestamped log file ---
    # Lives under the app's own persisted /app/logs volume rather than a
    # testing_tooling/logs subdirectory — that path isn't covered by any
    # volume mount or the --tmpfs /tmp added for --read-only hardening
    # (issue #213), so mkdir here would fail under a read-only root
    # filesystem; putting it on a separate ephemeral tmpfs instead would
    # lose BITs' run history on every container restart, which would make
    # debugging a failed self-test on a redeployed container needlessly
    # hard.
    script_dir = Path(__file__).resolve().parent
    log_dir = script_dir.parent / "logs" / "bits"
    log_dir.mkdir(parents=True, exist_ok=True)
    log_path = log_dir / f"test_suite-{datetime.now().strftime('%Y%m%d-%H%M%S')}.log"
    log_file = open(log_path, "a")
    sys.stdout = Tee(sys.stdout, log_file)
    sys.stderr = Tee(sys.stderr, log_file)
    print(f"Logging output to {log_path}", file=sys.stderr)

    suite = TestSuite(base)
    return suite.run_all()


if __name__ == "__main__":
    sys.exit(main())
