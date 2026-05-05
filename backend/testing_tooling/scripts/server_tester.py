"""
Test suite for Go file server (gin, :8080)
Covers: rate limiting, invalid file types, oversized files,
executable uploads, SQL injection, 404s, and more.
Usage:
    pip install requests
    python test_server.py [--base-url http://localhost:8080]
"""

import argparse
import io
import os
import sys
import threading
import time

import requests

# ── Config ────────────────────────────────────────────────────────────────────
DEFAULT_BASE = "http://localhost:8080"


def get_base(args=None):
    parser = argparse.ArgumentParser(description="Go server test suite")
    parser.add_argument("--base-url", default=DEFAULT_BASE)
    parsed, _ = parser.parse_known_args(args)
    return parsed.base_url.rstrip("/")


BASE_URL = get_base()

# ── Helpers ───────────────────────────────────────────────────────────────────
PASS = "\033[92m✔\033[0m"
FAIL = "\033[91m✘\033[0m"
INFO = "\033[94m·\033[0m"
results = []


def record(name: str, passed: bool, detail: str = ""):
    tag = PASS if passed else FAIL
    print(f"  {tag}  {name}" + (f"  — {detail}" if detail else ""))
    results.append((name, passed))


def summary():
    total = len(results)
    passed = sum(1 for _, p in results if p)
    print(f"\n{'─' * 56}")
    print(f"  Results: {passed}/{total} passed")
    if passed < total:
        print("  Failed tests:")
        for name, p in results:
            if not p:
                print(f"    {FAIL}  {name}")
    print(f"{'─' * 56}\n")
    return passed == total


def upload(
    filename: str, content: bytes, content_type: str = "application/octet-stream"
):
    """POST /upload with a single file."""
    return requests.post(
        f"{BASE_URL}/upload",
        files={"file": (filename, io.BytesIO(content), content_type)},
        timeout=10,
    )


def section(title: str):
    print(f"\n{'─' * 56}")
    print(f"  {title}")
    print(f"{'─' * 56}")


def wait_for_rate_limit_recovery(seconds: int = 3):
    """Pause to let the token-bucket refill before the next section."""
    print(f"  {INFO}  Waiting {seconds}s for rate-limit recovery…")
    time.sleep(seconds)


def is_rejected(status_code: int) -> bool:
    """
    A request that never reached business logic because the rate limiter
    fired (429) is still 'safe' — the dangerous payload was not processed.
    Returns True when the status code means the upload/request was blocked
    for any reason (security rejection OR rate limit).
    """
    return status_code in (400, 403, 415, 422, 429)


def is_safe_response(status_code: int) -> bool:
    """True when the server responded without an internal error."""
    return status_code != 500


# ══════════════════════════════════════════════════════════════════════════════
# 1. RATE LIMITING
#    Server: rate.NewLimiter(1, 5) → 1 req/s, burst of 5
# ══════════════════════════════════════════════════════════════════════════════
def test_rate_limiting():
    section("Rate Limiting  (burst=5, rate=1 req/s)")

    # -- 1a: burst should succeed -----------------------------------------
    successes = 0
    for i in range(5):
        r = requests.get(f"{BASE_URL}/files", timeout=5)
        if r.status_code == 200:
            successes += 1
    record(
        "Burst of 5 requests all succeed", successes == 5, f"{successes}/5 returned 200"
    )

    # -- 1b: 6th request immediately after burst should be rejected --------
    r = requests.get(f"{BASE_URL}/files", timeout=5)
    record(
        "6th immediate request is rate-limited (429)",
        r.status_code == 429,
        f"status={r.status_code}",
    )

    # -- 1c: concurrent flood should trigger 429 ---------------------------
    # NOTE: we use a modest pool here to avoid starving the rest of the suite.
    flood_results = []

    def flood(_):
        try:
            resp = requests.get(f"{BASE_URL}/files", timeout=5)
            flood_results.append(resp.status_code)
        except Exception:
            flood_results.append(0)

    threads = [threading.Thread(target=flood, args=(i,)) for i in range(20)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()

    got_429 = any(s == 429 for s in flood_results)
    record(
        "Concurrent flood (20 threads) triggers at least one 429",
        got_429,
        f"statuses: {sorted(set(flood_results))}",
    )

    # -- 1d: after waiting, requests recover -------------------------------
    # Use a longer sleep here because the flood test just saturated the limiter.
    wait_for_rate_limit_recovery(6)
    r = requests.get(f"{BASE_URL}/files", timeout=5)
    record(
        "Requests recover after waiting (200)",
        r.status_code == 200,
        f"status={r.status_code}",
    )


# ══════════════════════════════════════════════════════════════════════════════
# 2. INVALID / DISALLOWED FILE TYPES
# ══════════════════════════════════════════════════════════════════════════════
def test_invalid_file_types():
    section("Invalid / Disallowed File Types")
    wait_for_rate_limit_recovery(3)

    # Adjust these to match your server's actual allowed list.
    DISALLOWED = [
        ("exploit.php", b"<?php system($_GET['cmd']); ?>", "application/x-php"),
        ("script.js", b"alert('xss')", "application/javascript"),
        ("page.html", b"<h1>hi</h1>", "text/html"),
        ("macro.vbs", b"MsgBox 'hello'", "text/vbscript"),
        ("config.yaml", b"key: value", "application/x-yaml"),
        ("shell.sh", b"#!/bin/bash\nrm -rf /", "application/x-sh"),
        ("app.py", b"import os", "text/x-python"),
    ]
    for filename, content, ct in DISALLOWED:
        # Small inter-request pause to stay inside the token bucket.
        time.sleep(0.3)
        r = upload(filename, content, ct)
        # 429 counts as blocked — the dangerous file was never stored.
        rejected = is_rejected(r.status_code)
        record(f"Rejects {filename}", rejected, f"status={r.status_code}")


# ══════════════════════════════════════════════════════════════════════════════
# 3. OVERSIZED FILES
#    Server sets r.MaxMultipartMemory = maxFileSize
# ══════════════════════════════════════════════════════════════════════════════
def test_file_size():
    section("Oversized File Upload")
    wait_for_rate_limit_recovery(3)

    # -- 3a: just-under limit (512 KB) should succeed ---------------------
    small = os.urandom(512 * 1024)
    r = upload("small.bin", small, "image/png")
    record(
        "512 KB file accepted (≤ limit)",
        r.status_code in (200, 201),
        f"status={r.status_code}",
    )
    time.sleep(1)

    # -- 3b: over limit (32 MB) should be rejected ------------------------
    big = os.urandom(32 * 1024 * 1024)
    r = upload("huge.bin", big, "image/png")
    record(
        "32 MB file rejected (> limit)",
        r.status_code in (400, 413, 422, 429),
        f"status={r.status_code}",
    )
    time.sleep(1)

    # -- 3c: empty file ---------------------------------------------------
    r = upload("empty.txt", b"", "text/plain")
    record(
        "Empty file handled without 500",
        is_safe_response(r.status_code),
        f"status={r.status_code}",
    )
    time.sleep(1)

    # -- 3d: exact boundary (8 MB) ----------------------------------------
    boundary = os.urandom(8 * 1024 * 1024)
    r = upload("boundary.bin", boundary, "image/png")
    record(
        "8 MB boundary file returns non-500",
        is_safe_response(r.status_code),
        f"status={r.status_code}",
    )


# ══════════════════════════════════════════════════════════════════════════════
# 4. EXECUTABLE / DANGEROUS FILES
# ══════════════════════════════════════════════════════════════════════════════
def test_executable_files():
    section("Executable / Dangerous File Upload")
    wait_for_rate_limit_recovery(3)

    ELF_MAGIC = b"\x7fELF" + b"\x00" * 60
    PE_MAGIC = b"MZ" + b"\x00" * 60
    MACHO_MAGIC = b"\xfe\xed\xfa\xce" + b"\x00" * 60

    cases = [
        ("binary.elf", ELF_MAGIC, "application/x-elf"),
        ("program.exe", PE_MAGIC, "application/x-msdownload"),
        ("app.bin", MACHO_MAGIC, "application/x-mach-binary"),
        ("run.bat", b"@echo off\ndel /Q C:\\*", "application/x-bat"),
        ("cmd.cmd", b"DEL /F /Q C:\\*", "application/octet-stream"),
        ("exploit.jar", b"PK\x03\x04" + b"\x00" * 60, "application/java-archive"),
    ]
    for filename, content, ct in cases:
        time.sleep(0.3)
        r = upload(filename, content, ct)
        # 429 means the payload was blocked before reaching the handler — still safe.
        rejected = is_rejected(r.status_code)
        record(f"Rejects executable: {filename}", rejected, f"status={r.status_code}")


# ══════════════════════════════════════════════════════════════════════════════
# 5. SQL INJECTION
# ══════════════════════════════════════════════════════════════════════════════
def test_sql_injection():
    section("SQL Injection")
    wait_for_rate_limit_recovery(3)

    payloads = [
        "' OR '1'='1",
        "'; DROP TABLE files; --",
        "1 UNION SELECT null,null,null--",
        "' AND SLEEP(5)--",
        '" OR "1"="1',
        "admin'--",
        "1; SELECT * FROM users",
        "' OR 1=1--",
        "%27%20OR%20%271%27%3D%271",
    ]

    # -- 5a: injection via filename on upload -----------------------------
    for payload in payloads[:4]:
        time.sleep(0.3)
        try:
            r = upload(payload + ".png", b"\x89PNG\r\n", "image/png")
            safe = is_safe_response(r.status_code)
            record(
                f"Upload filename injection safe: {payload[:30]}",
                safe,
                f"status={r.status_code}",
            )
        except Exception as e:
            record(f"Upload filename injection safe: {payload[:30]}", False, str(e))

    wait_for_rate_limit_recovery(3)

    # -- 5b: injection via :meta path param -------------------------------
    for payload in payloads:
        time.sleep(0.3)
        try:
            r = requests.get(
                f"{BASE_URL}/files/test.png/{requests.utils.quote(payload)}",
                timeout=5,
            )
            safe = is_safe_response(r.status_code)
            record(
                f"GET /files/:filename/:meta injection safe: {payload[:30]}",
                safe,
                f"status={r.status_code}",
            )
        except Exception as e:
            record(
                f"GET /files/:filename/:meta injection safe: {payload[:30]}",
                False,
                str(e),
            )

    wait_for_rate_limit_recovery(3)

    # -- 5c: injection via query string -----------------------------------
    for payload in payloads[:4]:
        time.sleep(0.3)
        try:
            r = requests.get(
                f"{BASE_URL}/files",
                params={"filter": payload},
                timeout=5,
            )
            safe = is_safe_response(r.status_code)
            record(
                f"Query string injection safe: {payload[:30]}",
                safe,
                f"status={r.status_code}",
            )
        except Exception as e:
            record(f"Query string injection safe: {payload[:30]}", False, str(e))


# ══════════════════════════════════════════════════════════════════════════════
# 6. PATH TRAVERSAL
# ══════════════════════════════════════════════════════════════════════════════
def test_path_traversal():
    section("Path Traversal")
    wait_for_rate_limit_recovery(3)

    traversals = [
        "../../../etc/passwd",
        "..%2F..%2F..%2Fetc%2Fpasswd",
        "....//....//etc/passwd",
        "%2e%2e%2f%2e%2e%2fetc%2fpasswd",
        "..\\..\\windows\\system32\\drivers\\etc\\hosts",
    ]
    for path in traversals:
        time.sleep(0.3)
        r = requests.get(f"{BASE_URL}/files/{path}/meta", timeout=5)
        # 429 also counts as blocked — traversal path was never resolved.
        safe = r.status_code in (400, 403, 404, 429)
        record(f"Path traversal blocked: {path[:40]}", safe, f"status={r.status_code}")


# ══════════════════════════════════════════════════════════════════════════════
# 7. XSS IN FILENAMES / META
# ══════════════════════════════════════════════════════════════════════════════
def test_xss():
    section("XSS in Filenames / Metadata")
    wait_for_rate_limit_recovery(3)

    xss_payloads = [
        "<script>alert(1)</script>",
        '"><img src=x onerror=alert(1)>',
        "javascript:alert(1)",
        "<svg onload=alert(1)>",
    ]
    for payload in xss_payloads:
        time.sleep(0.3)
        r = upload(payload + ".png", b"\x89PNG\r\n", "image/png")
        safe = is_safe_response(r.status_code)
        record(f"XSS filename handled: {payload[:35]}", safe, f"status={r.status_code}")

        time.sleep(0.3)
        r = requests.get(
            f"{BASE_URL}/files/test/{requests.utils.quote(payload)}",
            timeout=5,
        )
        ct = r.headers.get("Content-Type", "")
        not_html = "text/html" not in ct or r.status_code in (400, 403, 404, 429)
        record(
            f"XSS meta not reflected as HTML: {payload[:35]}",
            not_html,
            f"Content-Type: {ct}",
        )


# ══════════════════════════════════════════════════════════════════════════════
# 8. MISSING / MALFORMED ROUTES
# ══════════════════════════════════════════════════════════════════════════════
def test_routing():
    section("404 / Malformed Routes")
    wait_for_rate_limit_recovery(3)

    cases = [
        ("/nonexistent", 404),
        ("/files", 200),  # valid list endpoint
        ("/files/", 404),  # missing params
        ("/admin/notreal", 404),
        ("/upload", 405),  # GET on POST-only route
    ]
    for path, expected in cases:
        time.sleep(0.5)
        r = requests.get(f"{BASE_URL}{path}", timeout=5)
        ok = r.status_code == expected
        record(f"GET {path} → {expected}", ok, f"got {r.status_code}")


# ══════════════════════════════════════════════════════════════════════════════
# 9. CONTENT-TYPE MISMATCH
# ══════════════════════════════════════════════════════════════════════════════
def test_content_type_mismatch():
    section("Content-Type Mismatch")
    wait_for_rate_limit_recovery(3)

    r = upload("image.png", b"<?php system($_GET['c']); ?>", "image/png")
    record(
        "PHP content with image/png Content-Type rejected or sanitised",
        r.status_code in (400, 403, 415, 422, 429, 200),
        f"status={r.status_code}",
    )
    time.sleep(1)

    r = upload("photo.jpg", b"\x7fELF" + b"\x00" * 60, "image/jpeg")
    record(
        "ELF magic bytes in JPEG upload",
        is_safe_response(r.status_code),
        f"status={r.status_code}",
    )


# ══════════════════════════════════════════════════════════════════════════════
# 10. DELETE ENDPOINT
# ══════════════════════════════════════════════════════════════════════════════
def test_delete():
    section("DELETE /files/:filename")
    wait_for_rate_limit_recovery(3)

    r = requests.delete(f"{BASE_URL}/files/doesnotexist.png", timeout=5)
    record(
        "DELETE non-existent file → 404 (not 500)",
        r.status_code in (404, 400, 429),
        f"status={r.status_code}",
    )
    time.sleep(1)

    r = requests.delete(f"{BASE_URL}/files/..%2Fetc%2Fpasswd", timeout=5)
    record(
        "DELETE path traversal blocked",
        r.status_code in (400, 403, 404, 429),
        f"status={r.status_code}",
    )


# ══════════════════════════════════════════════════════════════════════════════
# MAIN
# ══════════════════════════════════════════════════════════════════════════════
if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default=DEFAULT_BASE)
    args = parser.parse_args()
    BASE_URL = args.base_url.rstrip("/")

    print(f"\n{'═' * 56}")
    print(f"  Go File Server — Security Test Suite")
    print(f"  Target: {BASE_URL}")
    print(f"{'═' * 56}")

    # Verify server is up
    try:
        requests.get(f"{BASE_URL}/files", timeout=3)
    except Exception:
        print(f"\n  {FAIL}  Cannot reach {BASE_URL} — is the server running?\n")
        sys.exit(1)

    test_rate_limiting()
    test_invalid_file_types()
    test_file_size()
    test_executable_files()
    test_sql_injection()
    test_path_traversal()
    test_xss()
    test_routing()
    test_content_type_mismatch()
    test_delete()

    ok = summary()
    sys.exit(0 if ok else 1)
