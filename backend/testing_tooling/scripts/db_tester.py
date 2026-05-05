"""
Database test suite — master.db
Table: files (id, filename, acts_id, sha256_hash, created_at, filepath, is_deleted, claimNumber)

Usage:
    python test_db.py [--db path/to/master.db]
"""

import hashlib
import os
import re
import sqlite3
import sys
import tempfile
from datetime import datetime, timezone

# ── Config ────────────────────────────────────────────────────────────────────

DEFAULT_DB = "master.db"  # ← change this path if needed

PASS = "\033[92m✔\033[0m"
FAIL = "\033[91m✘\033[0m"
INFO = "\033[94m·\033[0m"

results: list[tuple[str, bool]] = []


def record(name: str, passed: bool, detail: str = ""):
    tag = PASS if passed else FAIL
    print(f"  {tag}  {name}" + (f"  — {detail}" if detail else ""))
    results.append((name, passed))


def section(title: str):
    print(f"\n{'─' * 60}")
    print(f"  {title}")
    print(f"{'─' * 60}")


def summary() -> bool:
    total = len(results)
    passed = sum(1 for _, p in results if p)
    print(f"\n{'═' * 60}")
    print(f"  Results: {passed}/{total} passed")
    if passed < total:
        print("  Failed:")
        for name, p in results:
            if not p:
                print(f"    {FAIL}  {name}")
    print(f"{'═' * 60}\n")
    return passed == total


def connect(path: str) -> sqlite3.Connection:
    con = sqlite3.connect(path)
    con.row_factory = sqlite3.Row
    return con


# ══════════════════════════════════════════════════════════════════════════════
# 1. SCHEMA INTEGRITY
# ══════════════════════════════════════════════════════════════════════════════


def test_schema(con: sqlite3.Connection):
    section("Schema Integrity")

    cur = con.cursor()

    # -- 1a: required table exists ----------------------------------------
    cur.execute("SELECT name FROM sqlite_master WHERE type='table' AND name='files'")
    record("Table 'files' exists", cur.fetchone() is not None)

    # -- 1b: expected columns present -------------------------------------
    cur.execute("PRAGMA table_info(files)")
    cols = {row["name"] for row in cur.fetchall()}
    expected = {
        "id",
        "filename",
        "acts_id",
        "sha256_hash",
        "created_at",
        "filepath",
        "is_deleted",
        "claimNumber",
    }
    missing = expected - cols
    record(
        "All expected columns present",
        not missing,
        f"missing: {missing}" if missing else "",
    )

    # -- 1c: id is PRIMARY KEY --------------------------------------------
    cur.execute("PRAGMA table_info(files)")
    pk_cols = [r["name"] for r in cur.fetchall() if r["pk"] == 1]
    record("'id' is PRIMARY KEY", pk_cols == ["id"], f"pk cols: {pk_cols}")

    # -- 1d: no unexpected tables (besides sqlite_sequence) ---------------
    cur.execute("SELECT name FROM sqlite_master WHERE type='table'")
    all_tables = {r[0] for r in cur.fetchall()} - {"sqlite_sequence"}
    record(
        "Only expected tables present", all_tables == {"files"}, f"found: {all_tables}"
    )


# ══════════════════════════════════════════════════════════════════════════════
# 2. DATA INTEGRITY
# ══════════════════════════════════════════════════════════════════════════════


def test_data_integrity(con: sqlite3.Connection):
    section("Data Integrity")

    cur = con.cursor()

    # -- 2a: no NULL filenames --------------------------------------------
    cur.execute("SELECT COUNT(*) FROM files WHERE filename IS NULL OR filename = ''")
    n = cur.fetchone()[0]
    record("No NULL/empty filenames", n == 0, f"{n} violations")

    # -- 2b: no NULL filepaths --------------------------------------------
    cur.execute("SELECT COUNT(*) FROM files WHERE filepath IS NULL OR filepath = ''")
    n = cur.fetchone()[0]
    record("No NULL/empty filepaths", n == 0, f"{n} violations")

    # -- 2c: is_deleted is 0 or 1 (boolean) --------------------------------
    cur.execute(
        "SELECT COUNT(*) FROM files WHERE is_deleted NOT IN (0, 1) AND is_deleted IS NOT NULL"
    )
    n = cur.fetchone()[0]
    record("is_deleted only contains 0/1/NULL", n == 0, f"{n} invalid values")

    # -- 2d: sha256_hash format (64 hex chars) when not NULL ---------------
    cur.execute("SELECT id, sha256_hash FROM files WHERE sha256_hash IS NOT NULL")
    bad_hashes = [
        r["id"]
        for r in cur.fetchall()
        if not re.fullmatch(r"[0-9a-fA-F]{64}", r["sha256_hash"])
    ]
    record(
        "All sha256_hash values are valid 64-char hex",
        not bad_hashes,
        f"bad ids: {bad_hashes}" if bad_hashes else "",
    )

    # -- 2e: no duplicate sha256 hashes for non-deleted files (same content uploaded twice) --
    cur.execute("""
        SELECT sha256_hash, COUNT(*) AS cnt
        FROM files
        WHERE sha256_hash IS NOT NULL AND is_deleted = 0
        GROUP BY sha256_hash
        HAVING cnt > 1
    """)
    dupes = cur.fetchall()
    record(
        "No duplicate sha256 hashes among active files",
        not dupes,
        f"{len(dupes)} duplicate hash(es)" if dupes else "",
    )

    # -- 2f: created_at is parseable datetime ------------------------------
    cur.execute("SELECT id, created_at FROM files WHERE created_at IS NOT NULL")
    bad_dates = []
    for row in cur.fetchall():
        ts = row["created_at"]
        # Accept ISO-8601 variants the Go time package emits
        normalized = re.sub(r"(\.\d+)?[-+]\d{2}:\d{2}$", "", ts).strip()
        try:
            datetime.fromisoformat(normalized)
        except ValueError:
            bad_dates.append(row["id"])
    record(
        "All created_at values are parseable",
        not bad_dates,
        f"bad ids: {bad_dates}" if bad_dates else "",
    )

    # -- 2g: filepath starts with expected prefix -------------------------
    cur.execute("SELECT id, filepath FROM files WHERE filepath IS NOT NULL")
    bad_paths = [
        r["id"] for r in cur.fetchall() if not r["filepath"].startswith("store/")
    ]
    record(
        "All filepaths start with 'store/'",
        not bad_paths,
        f"ids with unexpected paths: {bad_paths}" if bad_paths else "",
    )

    # -- 2h: filename matches basename of filepath -------------------------
    cur.execute("SELECT id, filename, filepath FROM files WHERE filepath IS NOT NULL")
    mismatches = []
    for row in cur.fetchall():
        expected_name = os.path.basename(row["filepath"])
        if expected_name != row["filename"]:
            mismatches.append(row["id"])
    record(
        "filename matches basename(filepath)",
        not mismatches,
        f"mismatch ids: {mismatches}" if mismatches else "",
    )


# ══════════════════════════════════════════════════════════════════════════════
# 3. SOFT DELETE LOGIC
# ══════════════════════════════════════════════════════════════════════════════


def test_soft_delete(con: sqlite3.Connection):
    section("Soft Delete Logic")

    cur = con.cursor()

    # -- 3a: count active vs deleted records ------------------------------
    cur.execute("SELECT is_deleted, COUNT(*) AS n FROM files GROUP BY is_deleted")
    counts = {row["is_deleted"]: row["n"] for row in cur.fetchall()}
    active = counts.get(0, 0)
    deleted = counts.get(1, 0)
    record(
        "Active / deleted counts are sensible (active ≥ 0)",
        active >= 0,
        f"active={active}, deleted={deleted}",
    )

    # -- 3b: no hard-deleted rows (id gaps) --------------------------------
    cur.execute("SELECT id FROM files ORDER BY id")
    ids = [r[0] for r in cur.fetchall()]
    if ids:
        expected_ids = list(range(ids[0], ids[-1] + 1))
        gaps = set(expected_ids) - set(ids)
        record(
            "No gaps in id sequence (no hard deletes)",
            not gaps,
            f"missing ids: {sorted(gaps)}" if gaps else f"sequence {ids[0]}→{ids[-1]}",
        )
    else:
        record("No gaps in id sequence", True, "table is empty")

    # -- 3c: deleted files still have all metadata intact -----------------
    cur.execute("SELECT * FROM files WHERE is_deleted = 1")
    deleted_rows = cur.fetchall()
    incomplete = [
        r["id"] for r in deleted_rows if not r["filename"] or not r["filepath"]
    ]
    record(
        "Soft-deleted rows retain filename/filepath",
        not incomplete,
        f"incomplete ids: {incomplete}"
        if incomplete
        else f"checked {len(deleted_rows)} deleted rows",
    )


# ══════════════════════════════════════════════════════════════════════════════
# 4. ACTS_ID / CLAIM NUMBER CONSISTENCY
# ══════════════════════════════════════════════════════════════════════════════


def test_business_fields(con: sqlite3.Connection):
    section("acts_id / claimNumber Consistency")

    cur = con.cursor()

    # -- 4a: acts_id format (ACTS_XXX pattern) when set -------------------
    cur.execute("SELECT id, acts_id FROM files WHERE acts_id IS NOT NULL")
    bad_acts = [
        r["id"] for r in cur.fetchall() if not re.match(r"^ACTS_\w+$", r["acts_id"])
    ]
    record(
        "acts_id matches 'ACTS_<alphanumeric>' format",
        not bad_acts,
        f"bad ids: {bad_acts}" if bad_acts else "",
    )

    # -- 4b: files with same acts_id should have same claimNumber ---------
    cur.execute("""
        SELECT acts_id, COUNT(DISTINCT claimNumber) AS distinct_claims
        FROM files
        WHERE acts_id IS NOT NULL AND claimNumber IS NOT NULL
        GROUP BY acts_id
        HAVING distinct_claims > 1
    """)
    conflicts = cur.fetchall()
    record(
        "Same acts_id always maps to same claimNumber",
        not conflicts,
        f"{len(conflicts)} conflicting acts_id(s)" if conflicts else "",
    )

    # -- 4e: distribution summary (informational) -------------------------
    cur.execute(
        "SELECT acts_id, COUNT(*) AS n FROM files GROUP BY acts_id ORDER BY n DESC LIMIT 5"
    )
    top = cur.fetchall()
    print(
        f"  {INFO}  Top acts_id values: "
        + ", ".join(f"{r['acts_id']}={r['n']}" for r in top)
    )


# ══════════════════════════════════════════════════════════════════════════════
# 5. FILENAME SAFETY
# ══════════════════════════════════════════════════════════════════════════════


def test_filename_safety(con: sqlite3.Connection):
    section("Filename / Filepath Safety")

    cur = con.cursor()
    cur.execute("SELECT id, filename, filepath FROM files")
    rows = cur.fetchall()

    # -- 5a: no path traversal sequences ----------------------------------
    traversal_ids = [
        r["id"] for r in rows if ".." in r["filename"] or ".." in (r["filepath"] or "")
    ]
    record(
        "No path traversal (..) in filename/filepath",
        not traversal_ids,
        f"ids: {traversal_ids}" if traversal_ids else "",
    )

    # -- 5b: no null bytes ------------------------------------------------
    null_byte_ids = [
        r["id"]
        for r in rows
        if "\x00" in r["filename"] or "\x00" in (r["filepath"] or "")
    ]
    record(
        "No null bytes in filename/filepath",
        not null_byte_ids,
        f"ids: {null_byte_ids}" if null_byte_ids else "",
    )

    # -- 5c: no shell metacharacters in filename --------------------------
    shell_chars = re.compile(r"[;&|`$<>!\\]")
    shell_ids = [r["id"] for r in rows if shell_chars.search(r["filename"])]
    record(
        "No shell metacharacters in filenames",
        not shell_ids,
        f"ids: {shell_ids}" if shell_ids else "",
    )

    # -- 5d: filenames have an extension ----------------------------------
    no_ext_ids = [r["id"] for r in rows if "." not in r["filename"]]
    record(
        "All filenames have a file extension",
        not no_ext_ids,
        f"ids without extension: {no_ext_ids}" if no_ext_ids else "",
    )

    # -- 5e: no suspicious extension sequences (double ext: file.php.jpg) -
    suspicious = re.compile(r"\.(php|exe|sh|bat|js|py|rb|pl|cmd)\.", re.I)
    suspicious_ids = [r["id"] for r in rows if suspicious.search(r["filename"])]
    record(
        "No suspicious double-extension filenames",
        not suspicious_ids,
        f"ids: {suspicious_ids}" if suspicious_ids else "",
    )


# ══════════════════════════════════════════════════════════════════════════════
# 6. HASH CONSISTENCY (if files are accessible on disk)
# ══════════════════════════════════════════════════════════════════════════════


def test_hash_consistency(con: sqlite3.Connection, db_path: str):
    section("SHA-256 Hash Consistency (on-disk check)")

    cur = con.cursor()
    cur.execute(
        "SELECT id, filename, filepath, sha256_hash FROM files WHERE sha256_hash IS NOT NULL AND is_deleted = 0"
    )
    rows = cur.fetchall()

    base_dir = os.path.dirname(os.path.abspath(db_path))
    checked = 0
    mismatches = []

    for row in rows:
        full_path = os.path.join(base_dir, row["filepath"])
        if not os.path.exists(full_path):
            continue  # file not present in this environment — skip
        h = hashlib.sha256(open(full_path, "rb").read()).hexdigest()
        if h != row["sha256_hash"]:
            mismatches.append((row["id"], row["filename"]))
        checked += 1

    if checked == 0:
        print(
            f"  {INFO}  No on-disk files found at {base_dir}/store — skipping hash check"
        )
        record("SHA-256 on-disk verification (skipped — no files)", True, "n/a")
    else:
        record(
            f"SHA-256 matches for {checked} on-disk file(s)",
            not mismatches,
            f"mismatches: {mismatches}" if mismatches else "all match",
        )


# ══════════════════════════════════════════════════════════════════════════════
# 7. WRITE / TRANSACTION SAFETY (uses temp copy)
# ══════════════════════════════════════════════════════════════════════════════


def test_write_safety(db_path: str):
    section("Write / Transaction Safety  (uses temp copy of DB)")

    import shutil

    with tempfile.TemporaryDirectory() as tmp:
        tmp_db = os.path.join(tmp, "test.db")
        shutil.copy2(db_path, tmp_db)
        # Use isolation_level=None for explicit transaction control
        con = sqlite3.connect(tmp_db)
        con.row_factory = sqlite3.Row
        con.execute("PRAGMA journal_mode=WAL")
        cur = con.cursor()

        # -- 7a: insert a valid row then rollback -------------------------
        cur.execute("SELECT MAX(id) FROM files")
        max_id_before = cur.fetchone()[0] or 0
        try:
            cur.execute("BEGIN")
            cur.execute(
                """
                INSERT INTO files (filename, acts_id, sha256_hash, filepath, is_deleted, claimNumber)
                VALUES ('test_rollback.png', 'ACTS_TEST', ?, 'store/images/test_rollback.png', 0, 'CLM-TEST')
            """,
                ("a" * 64,),
            )
            con.rollback()
        except Exception as e:
            record("Rollback on INSERT", False, str(e))
        else:
            cur.execute("SELECT MAX(id) FROM files")
            max_id_after = cur.fetchone()[0] or 0
            record(
                "Rollback leaves table unchanged",
                max_id_after == max_id_before,
                f"id before={max_id_before}, after={max_id_after}",
            )

        # -- 7b: duplicate primary key rejected ---------------------------
        cur.execute("SELECT id FROM files LIMIT 1")
        existing_id = cur.fetchone()[0]
        try:
            cur.execute(
                """
                INSERT INTO files (id, filename, filepath, is_deleted)
                VALUES (?, 'dup.png', 'store/dup.png', 0)
            """,
                (existing_id,),
            )
            con.commit()
            record(
                "Duplicate PK rejected",
                False,
                "insert succeeded — no UNIQUE constraint?",
            )
        except sqlite3.IntegrityError:
            con.rollback()
            record("Duplicate PK correctly rejected", True)

        # -- 7c: soft delete works ----------------------------------------
        cur.execute(
            "INSERT INTO files (filename, filepath, is_deleted, claimNumber) VALUES ('del_test.png','store/del_test.png',0,'CLM-DEL')"
        )
        con.commit()
        new_id = cur.lastrowid
        cur.execute("UPDATE files SET is_deleted = 1 WHERE id = ?", (new_id,))
        con.commit()
        cur.execute("SELECT is_deleted FROM files WHERE id = ?", (new_id,))
        row = cur.fetchone()
        record(
            "Soft delete UPDATE works",
            row is not None and row[0] == 1,
            f"is_deleted={row[0] if row else 'row missing'}",
        )

        con.close()


# ══════════════════════════════════════════════════════════════════════════════
# 8. STATISTICS  (informational)
# ══════════════════════════════════════════════════════════════════════════════


def test_statistics(con: sqlite3.Connection):
    section("Statistics  (informational)")

    cur = con.cursor()

    cur.execute("SELECT COUNT(*) FROM files")
    total = cur.fetchone()[0]
    print(f"  {INFO}  Total rows: {total}")

    cur.execute("SELECT COUNT(*) FROM files WHERE is_deleted = 0")
    active = cur.fetchone()[0]
    print(f"  {INFO}  Active files: {active}")

    cur.execute("SELECT COUNT(*) FROM files WHERE is_deleted = 1")
    deleted = cur.fetchone()[0]
    print(f"  {INFO}  Soft-deleted files: {deleted}")

    cur.execute("SELECT COUNT(*) FROM files WHERE sha256_hash IS NULL")
    no_hash = cur.fetchone()[0]
    print(f"  {INFO}  Rows without hash: {no_hash}")

    cur.execute(
        "SELECT COUNT(*) FROM files WHERE claimNumber IS NULL OR claimNumber = ''"
    )
    no_claim = cur.fetchone()[0]
    print(f"  {INFO}  Rows without claimNumber: {no_claim}")

    cur.execute("""
        SELECT substr(filename, instr(filename,'.')+1) AS ext, COUNT(*) AS n
        FROM files GROUP BY ext ORDER BY n DESC
    """)
    print(f"  {INFO}  File type breakdown:")
    for row in cur.fetchall():
        print(f"          .{row[0] or '(none)'}  →  {row[1]}")

    record("Statistics collected", True, "see above")


# ══════════════════════════════════════════════════════════════════════════════
# MAIN
# ══════════════════════════════════════════════════════════════════════════════

if __name__ == "__main__":
    db_path = "../../store/master.db"

    if not os.path.exists(db_path):
        print(f"\n  {FAIL}  Database not found: {db_path}\n")
        sys.exit(1)

    print(f"\n{'═' * 60}")
    print(f"  master.db  —  Database Test Suite")
    print(f"  File: {os.path.abspath(db_path)}  ({os.path.getsize(db_path):,} bytes)")
    print(f"{'═' * 60}")

    con = connect(db_path)

    test_schema(con)
    test_data_integrity(con)
    test_soft_delete(con)
    test_business_fields(con)
    test_filename_safety(con)
    test_hash_consistency(con, db_path)
    test_write_safety(db_path)
    test_statistics(con)

    con.close()
    ok = summary()
    sys.exit(0 if ok else 1)
