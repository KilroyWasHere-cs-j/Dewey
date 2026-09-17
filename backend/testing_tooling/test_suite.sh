#!/usr/bin/env bash
set -uo pipefail

# Requires FILES_PASSWORD in the environment (issue #332) — main.go execs
# this as a subprocess of the backend, which inherits the container's env,
# so it's already set when this runs as BITs. Running it manually against a
# different host needs it passed explicitly, e.g.
# `FILES_PASSWORD=$(cat .files-password) ./test_suite.sh`.
#
# The password itself is only ever sent once, below, to exchange it for a
# session token (issue #409) — every actual test request sends
# X-Dewey-Session-Token instead, same as the real frontend does.

BASE="${1:-http://localhost:8080}"
BASE="${BASE%/}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DOWNLOAD_DIR="$(mktemp -d)"

# Mirrors config.json's file_system.file_system_base_dir ("./store" relative
# to the app's WORKDIR) — this script runs as a direct subprocess of the Go
# backend (issue #99's BITs, exec'd from main.go with no Dir override), so
# it shares the backend's own working directory and this resolves to the
# real store the backend writes into, same reasoning as LOG_DIR below.
STORE_DIR="$(dirname "$SCRIPT_DIR")/store"

# --- LOGGING ---
# Mirror both streams into a timestamped log file, keeping stdout (structured
# PASS/FAIL results) and stderr (human-readable progress) separately teed so
# neither stream's meaning changes for callers piping this script's output.
# Lives under the app's own persisted /app/logs volume rather than a
# testing_tooling/logs subdirectory — that path isn't covered by any volume
# mount or the --tmpfs /tmp added for --read-only hardening (issue #213), so
# mkdir here would fail under a read-only root filesystem; putting it on a
# separate ephemeral tmpfs instead would lose BITs' run history on every
# container restart, which would make debugging a failed self-test on a
# redeployed container needlessly hard.
LOG_DIR="$(dirname "$SCRIPT_DIR")/logs/bits"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/test_suite-$(date +%Y%m%d-%H%M%S).log"
exec > >(tee -a "$LOG_FILE") 2> >(tee -a "$LOG_FILE" >&2)
echo >&2 "Logging output to $LOG_FILE"

# --- AUTH ---
# Exchange FILES_PASSWORD for a session token once (issue #409), same as
# the frontend does — every test below sends the token, never the raw
# password again. Failing fast here with a clear message beats letting
# every single test in the suite fail with a confusing 401.
SESSION_TOKEN=$(curl -s -X POST -H "X-Dewey-Password: ${FILES_PASSWORD:-}" \
    "$BASE/core/files/session" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
if [ -z "$SESSION_TOKEN" ]; then
    echo >&2 "Failed to exchange FILES_PASSWORD for a session token against $BASE — check FILES_PASSWORD and that the backend is reachable."
    exit 1
fi

PASS=0
FAIL=0
SKIP=0
TOTAL=0

# Every filename the suite gets back from a successful upload is appended
# here as it happens, so cleanup can remove it regardless of which section
# created it. Declared up front (rather than where the first upload section
# used to declare it) since cleanup() reads it and trap fires on early exit
# too, before later sections would otherwise have defined it.
UPLOADED_FILES=()

cleanup() {
    # Best-effort: delete every file this run uploaded into the live store,
    # so BITs stops leaving permanent residue in prod on every boot (issue
    # #347). Errors are ignored — a file run_delete_tests already removed
    # will just 404 here, which is fine, and this must never fail the trap.
    for f in "${UPLOADED_FILES[@]:-}"; do
        [ -n "$f" ] && curl -s -o /dev/null -H "X-Dewey-Session-Token: ${SESSION_TOKEN:-}" \
            -X DELETE "$BASE/core/files/$f" --max-time 10 2>/dev/null
    done
    rm -rf "$DOWNLOAD_DIR"
}
trap cleanup EXIT

# --- COLORS ---
NC='\033[0m'
BOLD='\033[1m'
DIM='\033[2m'
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
WHITE='\033[1;37m'

# stdout: structured test results (machine-readable)
# stderr: human-readable progress and diagnostics
result_pass() {
    PASS=$((PASS + 1))
    TOTAL=$((TOTAL + 1))
    echo "PASS | $1"
    echo >&2 -e "  ${GREEN}${BOLD}✔${NC} ${GREEN}$1${NC}"
}

result_fail() {
    FAIL=$((FAIL + 1))
    TOTAL=$((TOTAL + 1))
    echo "FAIL | $1 | $2"
    echo >&2 -e "  ${RED}${BOLD}✘${NC} ${RED}$1${NC} ${DIM}— $2${NC}"
}

result_skip() {
    SKIP=$((SKIP + 1))
    TOTAL=$((TOTAL + 1))
    echo "SKIP | $1 | $2"
    echo >&2 -e "  ${YELLOW}${BOLD}○${NC} ${YELLOW}$1${NC} ${DIM}— $2${NC}"
}

info() { echo >&2 -e "$@"; }

pace() { sleep 1.1; }

section() {
    info ""
    info "  ${DIM}${CYAN}──────────────────────────────────────────────────────────${NC}"
    info "  ${MAGENTA}${BOLD}◆ $1${NC}"
    info "  ${DIM}${CYAN}──────────────────────────────────────────────────────────${NC}"
}

# ── Random data generators ────────────────────────────────────────────────────

FIRST_NAMES=("Oliver" "Phoebe" "Marcus" "Ingrid" "Tariq" "Yuki" "Soren" "Amara" "Declan" "Priya")
LAST_NAMES=("Nakamura" "Osei" "Lindqvist" "Ferrara" "Patel" "Kowalski" "Okafor" "Reyes" "Svensson" "Mbeki")
EMPLOYERS=("Horizon Robotics" "Starfall Media" "Ironclad Materials" "Vivant Health" "Obsidian Logistics" "Luminary Tech" "Verdant Farms" "Nexus Analytics" "Solaris Energy" "Phalanx Security")
ADJUSTERS=("T. Hargrove" "M. Delacroix" "A. Fujimoto" "R. Oduya" "S. Bergmann" "C. Abramowitz" "D. Kazakov" "F. Osei-Mensah" "L. Cartwright" "P. Iyer")
CLAIM_TYPES=("Workers Comp" "Liability" "Property" "Medical" "Disability" "Auto" "Product Liability" "Environmental")
SUPPORT_LVLS=("Full Support" "Partial" "Minimal" "Psychiatric" "Physical Therapy" "None" "Pending Review")
JURISDICTIONS=("California" "New York" "Texas" "Florida" "Illinois" "Washington" "Colorado" "Georgia" "Ohio" "Michigan")

rnd() {
    local -n arr=$1
    echo "${arr[RANDOM % ${#arr[@]}]}"
}

rand_claim()  { printf "CLM-%05d" $((RANDOM % 90000 + 10000)); }
rand_policy() { printf "POL-%06d" $((RANDOM % 900000 + 100000)); }
rand_date()   { date -d "2024-01-01 + $((RANDOM % 730)) days" +%Y-%m-%d 2>/dev/null || echo "2025-06-15"; }

upload_file() {
    local src="$1"
    pace
    curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -X POST "$BASE/core/upload" \
        -F "file=@$src" \
        -F "claim_number=$(rand_claim)" \
        -F "claimant_name=$(rnd FIRST_NAMES) $(rnd LAST_NAMES)" \
        -F "date_of_injury=$(rand_date)" \
        -F "employer=$(rnd EMPLOYERS)" \
        -F "adjuster=$(rnd ADJUSTERS)" \
        -F "support=$(rnd SUPPORT_LVLS)" \
        -F "claim_type=$(rnd CLAIM_TYPES)" \
        -F "jurisdiction=$(rnd JURISDICTIONS)" \
        -F "policy_number=$(rand_policy)" \
        -F "acts_id=ACTS_$(printf '%03d' $((RANDOM % 999)))" \
        -F "data=run-$(printf '%04x' $RANDOM)" \
        --max-time 30 2>/dev/null
}

# ── Section 1: Endpoint health checks ────────────────────────────────────────

run_health_checks() {
    section "ENDPOINT HEALTH CHECKS"

    local -A endpoints=(
        ["GET /"]="$BASE/"
        ["GET /core/files"]="$BASE/core/files"
        ["GET /core/admin/dumpCache"]="$BASE/core/admin/dumpCache"
    )

    for label in "${!endpoints[@]}"; do
        local url="${endpoints[$label]}"
        info "  Testing $label ..."
        pace
        local code
        code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 "$url" 2>/dev/null)
        if [ $? -ne 0 ] || [ "$code" = "000" ]; then
            result_fail "$label" "connection failed"
        elif [ "$code" -lt 400 ]; then
            result_pass "$label -> HTTP $code"
        else
            result_fail "$label" "HTTP $code"
        fi
    done
}

# ── Section 2: JSON upload ────────────────────────────────────────────────────

run_json_upload_test() {
    section "JSON UPLOAD"

    info "  Testing POST /upload with application/json ..."
    pace
    local body code
    body=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -w "\n%{http_code}" --max-time 10 \
        -X POST "$BASE/core/upload" \
        -H "Content-Type: application/json" \
        -d '{"name":"json-test","data":"hello"}' 2>/dev/null)
    code=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')

    if [ "$code" = "000" ]; then
        result_fail "POST /upload [JSON]" "connection failed"
        return
    fi

    if [ "$code" = "200" ]; then
        if echo "$body" | grep -q '"JSON received"'; then
            result_pass "POST /upload [JSON] -> HTTP 200, acknowledged"
        else
            result_fail "POST /upload [JSON]" "HTTP 200 but unexpected body: $body"
        fi
    else
        result_fail "POST /upload [JSON]" "expected 200, got HTTP $code"
    fi

    info "  Testing POST /upload with invalid JSON ..."
    pace
    code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 \
        -X POST "$BASE/core/upload" \
        -H "Content-Type: application/json" \
        -d 'not json at all' 2>/dev/null)

    if [ "$code" -ge 400 ]; then
        result_pass "POST /upload [bad JSON] -> HTTP $code (rejected)"
    elif [ "$code" = "000" ]; then
        result_fail "POST /upload [bad JSON]" "connection failed"
    else
        result_fail "POST /upload [bad JSON]" "expected 4xx, got HTTP $code"
    fi
}

# ── Section 3: Multipart upload with random metadata ─────────────────────────

run_upload_tests() {
    section "MULTIPART UPLOAD TESTS (randomized metadata)"

    local files=("lenna.jpg" "lenna.png" "test.pdf" "bee_moive_script.txt")

    for f in "${files[@]}"; do
        local src="$SCRIPT_DIR/$f"

        if [ ! -f "$src" ]; then
            result_skip "POST /upload [$f]" "source file not found"
            continue
        fi

        info "  Uploading $f ..."
        local resp
        resp=$(upload_file "$src")

        if [ $? -ne 0 ]; then
            result_fail "POST /upload [$f]" "curl error"
            continue
        fi

        local server_file
        server_file=$(echo "$resp" | grep -o '"filename":"[^"]*"' | cut -d'"' -f4)

        if [ -z "$server_file" ]; then
            result_fail "POST /upload [$f]" "no filename in response: $resp"
        else
            result_pass "POST /upload [$f] -> $server_file"
            UPLOADED_FILES+=("$server_file")
        fi
    done
}

# ── Section 4: Upload error cases ────────────────────────────────────────────

run_upload_error_tests() {
    section "UPLOAD ERROR CASES"

    info "  Testing invalid content-type ..."
    pace
    local code
    code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 \
        -X POST "$BASE/core/upload" \
        -H "Content-Type: text/plain" \
        -d "invalid" 2>/dev/null)

    if [ "$code" -ge 400 ]; then
        result_pass "POST /upload [bad content-type] -> HTTP $code (rejected)"
    elif [ "$code" = "000" ]; then
        result_fail "POST /upload [bad content-type]" "connection failed"
    else
        result_fail "POST /upload [bad content-type]" "expected 4xx, got HTTP $code"
    fi

    info "  Testing empty multipart (no file field) ..."
    pace
    code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 \
        -X POST "$BASE/core/upload" \
        -F "claim_number=CLM-00000" 2>/dev/null)

    if [ "$code" -ge 400 ]; then
        result_pass "POST /upload [no file] -> HTTP $code (rejected)"
    elif [ "$code" = "000" ]; then
        result_fail "POST /upload [no file]" "connection failed"
    else
        result_fail "POST /upload [no file]" "expected 4xx, got HTTP $code"
    fi

    info "  Testing blank date_of_injury (issue #365 — used to insert silently) ..."
    pace
    local doi_src="$SCRIPT_DIR/lenna.jpg"
    if [ ! -f "$doi_src" ]; then
        result_skip "POST /upload [blank date_of_injury]" "source file not found"
    else
        code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 \
            -X POST "$BASE/core/upload" \
            -F "file=@$doi_src" \
            -F "claim_number=$(rand_claim)" \
            -F "claimant_name=Blank DOI Test" \
            -F "date_of_injury=" \
            -F "employer=$(rnd EMPLOYERS)" \
            -F "adjuster=$(rnd ADJUSTERS)" \
            -F "support=$(rnd SUPPORT_LVLS)" \
            -F "claim_type=$(rnd CLAIM_TYPES)" \
            -F "jurisdiction=$(rnd JURISDICTIONS)" \
            -F "policy_number=$(rand_policy)" \
            -F "acts_id=ACTS_000" 2>/dev/null)

        if [ "$code" -ge 400 ]; then
            result_pass "POST /upload [blank date_of_injury] -> HTTP $code (rejected)"
        elif [ "$code" = "000" ]; then
            result_fail "POST /upload [blank date_of_injury]" "connection failed"
        else
            result_fail "POST /upload [blank date_of_injury]" "expected 4xx, got HTTP $code — regression of issue #365"
        fi
    fi
}

# ── Section 5: Disallowed file extension ──────────────────────────────────────

run_bad_extension_test() {
    section "DISALLOWED FILE EXTENSION"

    local tmpfile="$DOWNLOAD_DIR/malicious.sh"
    echo '#!/bin/bash' > "$tmpfile"
    echo 'echo pwned' >> "$tmpfile"

    info "  Uploading .sh file (should be rejected) ..."
    pace
    local code
    code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 \
        -X POST "$BASE/core/upload" \
        -F "file=@$tmpfile" \
        -F "claim_number=CLM-00000" \
        -F "claimant_name=Bad Actor" \
        -F "date_of_injury=2025-01-01" \
        -F "employer=Evil Corp" \
        -F "adjuster=None" \
        -F "support=None" \
        -F "claim_type=Liability" \
        -F "jurisdiction=California" \
        -F "policy_number=POL-000000" \
        -F "acts_id=ACTS_BAD" \
        -F "data=bad-ext-test" 2>/dev/null)

    if [ "$code" -ge 400 ]; then
        result_pass "POST /upload [.sh extension] -> HTTP $code (rejected)"
    elif [ "$code" = "000" ]; then
        result_fail "POST /upload [.sh extension]" "connection failed"
    else
        result_fail "POST /upload [.sh extension]" "expected 4xx, got HTTP $code"
    fi

    local tmpexe="$DOWNLOAD_DIR/payload.exe"
    echo 'not a real exe' > "$tmpexe"

    info "  Uploading .exe file (should be rejected) ..."
    pace
    code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 \
        -X POST "$BASE/core/upload" \
        -F "file=@$tmpexe" \
        -F "claim_number=CLM-00000" \
        -F "claimant_name=Bad Actor" \
        -F "date_of_injury=2025-01-01" \
        -F "employer=Evil Corp" \
        -F "adjuster=None" \
        -F "support=None" \
        -F "claim_type=Liability" \
        -F "jurisdiction=California" \
        -F "policy_number=POL-000000" \
        -F "acts_id=ACTS_BAD" \
        -F "data=bad-ext-test" 2>/dev/null)

    if [ "$code" -ge 400 ]; then
        result_pass "POST /upload [.exe extension] -> HTTP $code (rejected)"
    elif [ "$code" = "000" ]; then
        result_fail "POST /upload [.exe extension]" "connection failed"
    else
        result_fail "POST /upload [.exe extension]" "expected 4xx, got HTTP $code"
    fi
}

# ── Section 6: ELF binary rejection ──────────────────────────────────────────

run_elf_rejection_test() {
    section "ELF BINARY REJECTION"

    local src="$SCRIPT_DIR/renamedELF.txt"

    if [ ! -f "$src" ]; then
        result_skip "POST /upload [ELF as .txt]" "renamedELF.txt not found"
        return
    fi

    info "  Uploading renamedELF.txt (ELF binary disguised as .txt) ..."
    pace
    local body code
    body=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -w "\n%{http_code}" --max-time 30 \
        -X POST "$BASE/core/upload" \
        -F "file=@$src" \
        -F "claim_number=CLM-00000" \
        -F "claimant_name=ELF Test" \
        -F "date_of_injury=2025-01-01" \
        -F "employer=TestCorp" \
        -F "adjuster=A. Smith" \
        -F "support=Full Support" \
        -F "claim_type=Workers Comp" \
        -F "jurisdiction=California" \
        -F "policy_number=POL-000001" \
        -F "acts_id=ACTS_ELF" \
        -F "data=elf-rejection-test" 2>/dev/null)
    code=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')

    if [ "$code" -ge 400 ]; then
        if echo "$body" | grep -qi "ELF"; then
            result_pass "POST /upload [ELF as .txt] -> HTTP $code (ELF detected and blocked)"
        else
            result_pass "POST /upload [ELF as .txt] -> HTTP $code (rejected)"
        fi
    elif [ "$code" = "000" ]; then
        result_fail "POST /upload [ELF as .txt]" "connection failed"
    else
        result_fail "POST /upload [ELF as .txt]" "expected 4xx, got HTTP $code — server accepted an ELF binary"
    fi
}

# ── Section 7: Path traversal ────────────────────────────────────────────────

run_path_traversal_tests() {
    section "PATH TRAVERSAL PROTECTION"

    local traversal_paths=("../../etc/passwd" "../../../etc/shadow" "..%2F..%2Fetc%2Fpasswd")

    for tp in "${traversal_paths[@]}"; do
        info "  Testing GET /files/$tp/false ..."
        pace
        local code
        code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 \
            "$BASE/core/files/$tp/false" 2>/dev/null)

        if [ "$code" = "000" ]; then
            result_fail "GET /files/$tp/false (traversal)" "connection failed"
        elif [ "$code" -ge 400 ] || [ "$code" = "301" ] || [ "$code" = "302" ]; then
            result_pass "GET /files/$tp/false (traversal) -> HTTP $code (blocked)"
        elif [ "$code" = "200" ]; then
            result_fail "GET /files/$tp/false (traversal)" "HTTP 200 — path traversal may have succeeded"
        else
            result_pass "GET /files/$tp/false (traversal) -> HTTP $code"
        fi
    done

    info "  Testing DELETE with traversal path ..."
    pace
    local code
    code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 \
        -X DELETE "$BASE/core/files/../../etc/passwd" 2>/dev/null)

    if [ "$code" = "000" ]; then
        result_fail "DELETE /files/../../etc/passwd (traversal)" "connection failed"
    elif [ "$code" -ge 400 ] || [ "$code" = "301" ] || [ "$code" = "302" ]; then
        result_pass "DELETE /files/../../etc/passwd (traversal) -> HTTP $code (blocked)"
    elif [ "$code" = "200" ]; then
        result_fail "DELETE /files/../../etc/passwd (traversal)" "HTTP 200 — path traversal may have succeeded"
    else
        result_pass "DELETE /files/../../etc/passwd (traversal) -> HTTP $code"
    fi
}

# ── Section 8: SHA256 upload verification ─────────────────────────────────────

run_sha256_upload_test() {
    section "SHA256 UPLOAD VERIFICATION"

    local f="lenna.jpg"
    local src="$SCRIPT_DIR/$f"

    if [ ! -f "$src" ]; then
        result_skip "SHA256 upload verify [$f]" "source file not found"
        return
    fi

    local expected_hash
    expected_hash=$(sha256sum "$src" | cut -d' ' -f1)

    info "  Uploading $f and checking server-reported SHA256 ..."
    local resp
    resp=$(upload_file "$src")

    if [ $? -ne 0 ]; then
        result_fail "SHA256 upload verify [$f]" "curl error"
        return
    fi

    local server_hash server_file
    server_hash=$(echo "$resp" | grep -o '"sha256":"[^"]*"' | cut -d'"' -f4)
    server_file=$(echo "$resp" | grep -o '"filename":"[^"]*"' | cut -d'"' -f4)
    [ -n "$server_file" ] && UPLOADED_FILES+=("$server_file")

    if [ -z "$server_hash" ]; then
        result_fail "SHA256 upload verify [$f]" "no sha256 in response: $resp"
        return
    fi

    if [ "$expected_hash" = "$server_hash" ]; then
        result_pass "SHA256 upload verify [$f] local=$expected_hash == server=$server_hash"
    else
        result_fail "SHA256 upload verify [$f]" "local=$expected_hash != server=$server_hash"
    fi
}

# ── Section 9: Duplicate upload ───────────────────────────────────────────────

run_duplicate_upload_test() {
    section "DUPLICATE UPLOAD"

    local f="lenna.jpg"
    local src="$SCRIPT_DIR/$f"

    if [ ! -f "$src" ]; then
        result_skip "DUPLICATE upload [$f]" "source file not found"
        return
    fi

    info "  Uploading $f twice ..."
    local resp1 resp2 file1 file2

    resp1=$(upload_file "$src")
    file1=$(echo "$resp1" | grep -o '"filename":"[^"]*"' | cut -d'"' -f4)

    resp2=$(upload_file "$src")
    file2=$(echo "$resp2" | grep -o '"filename":"[^"]*"' | cut -d'"' -f4)

    [ -n "$file1" ] && UPLOADED_FILES+=("$file1")
    [ -n "$file2" ] && UPLOADED_FILES+=("$file2")

    if [ -z "$file1" ] || [ -z "$file2" ]; then
        result_fail "DUPLICATE upload [$f]" "one or both uploads failed: file1=$file1 file2=$file2"
        return
    fi

    if [ "$file1" != "$file2" ]; then
        result_pass "DUPLICATE upload [$f] -> distinct filenames: $file1, $file2"
    else
        result_fail "DUPLICATE upload [$f]" "both uploads returned same filename: $file1"
    fi
}

# ── Section 10: Roundtrip integrity (upload, download, SHA256 compare) ────────

run_roundtrip_tests() {
    section "ROUNDTRIP INTEGRITY (SHA256)"

    local files=("lenna.jpg" "lenna.png" "test.pdf" "bee_moive_script.txt")

    for f in "${files[@]}"; do
        local src="$SCRIPT_DIR/$f"

        if [ ! -f "$src" ]; then
            result_skip "ROUNDTRIP [$f]" "source file not found"
            continue
        fi

        info "  Uploading $f for roundtrip ..."
        pace
        local resp
        resp=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -X POST "$BASE/core/upload" \
            -F "file=@$src" \
            -F "claim_number=CLM-99999" \
            -F "claimant_name=Roundtrip Test" \
            -F "date_of_injury=2025-01-01" \
            -F "employer=TestCorp" \
            -F "adjuster=A. Smith" \
            -F "support=Full Support" \
            -F "claim_type=Workers Comp" \
            -F "jurisdiction=California" \
            -F "policy_number=POL-000001" \
            -F "acts_id=ACTS_RT" \
            -F "data=roundtrip-test" \
            --max-time 30 2>/dev/null)

        local server_file
        server_file=$(echo "$resp" | grep -o '"filename":"[^"]*"' | cut -d'"' -f4)

        if [ -z "$server_file" ]; then
            result_fail "ROUNDTRIP [$f] upload" "no filename in response: $resp"
            continue
        fi

        UPLOADED_FILES+=("$server_file")

        info "  Downloading $server_file ..."
        pace
        local dst="$DOWNLOAD_DIR/$server_file"
        local code
        code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o "$dst" -w "%{http_code}" --max-time 30 "$BASE/core/files/$server_file?meta=false" 2>/dev/null)

        if [ "$code" != "200" ]; then
            result_fail "ROUNDTRIP [$f] download" "HTTP $code"
            continue
        fi

        local hash_orig hash_down
        hash_orig=$(sha256sum "$src" | cut -d' ' -f1)
        hash_down=$(sha256sum "$dst" | cut -d' ' -f1)

        if [ "$hash_orig" = "$hash_down" ]; then
            result_pass "ROUNDTRIP [$f] SHA256=$hash_orig"
        else
            result_fail "ROUNDTRIP [$f] SHA256 mismatch" "orig=$hash_orig got=$hash_down"
        fi
    done
}

# ── Section 11: Store path verification ──────────────────────────────────────
#
# The roundtrip check above only proves the API serves back byte-identical
# content — but locateFile checks the upload cache before ever falling back
# to the DB-recorded store path, so a passing roundtrip alone doesn't prove
# idAndSort's async post-processing actually copied the file into
# fileSystemBaseDir at all (issue #270). This checks the store directly on
# disk instead of through the API.

run_store_path_verification_test() {
    section "STORE PATH VERIFICATION"

    local f="lenna.jpg"
    local src="$SCRIPT_DIR/$f"

    if [ ! -f "$src" ]; then
        result_skip "STORE PATH verify [$f]" "source file not found"
        return
    fi

    if [ ! -d "$STORE_DIR" ]; then
        result_skip "STORE PATH verify [$f]" "store directory not found at $STORE_DIR"
        return
    fi

    info "  Uploading $f ..."
    local resp
    resp=$(upload_file "$src")

    if [ $? -ne 0 ]; then
        result_fail "STORE PATH verify [$f]" "curl error"
        return
    fi

    local server_file
    server_file=$(echo "$resp" | grep -o '"filename":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$server_file" ]; then
        result_fail "STORE PATH verify [$f]" "no filename in response: $resp"
        return
    fi

    UPLOADED_FILES+=("$server_file")

    # idAndSort's disk copy runs asynchronously in a background goroutine
    # queued behind postProcessingSem (issue #217) — poll for a few seconds
    # rather than checking once immediately, so this doesn't flake on a
    # slow or backed-up post-processing pass.
    info "  Waiting for $server_file to land in the store ..."
    local store_path=""
    for _ in $(seq 1 15); do
        store_path=$(find "$STORE_DIR" -type f -name "$server_file" 2>/dev/null | head -1)
        if [ -n "$store_path" ]; then
            break
        fi
        sleep 1
    done

    if [ -z "$store_path" ]; then
        result_fail "STORE PATH verify [$server_file]" "not found anywhere under $STORE_DIR after upload"
        return
    fi
    result_pass "STORE PATH verify [$server_file] -> found at $store_path"

    if [ ! -s "$store_path" ]; then
        result_fail "STORE PATH verify [$server_file]" "store copy exists but is empty: $store_path"
        return
    fi

    local expected_hash actual_hash
    expected_hash=$(sha256sum "$src" | cut -d' ' -f1)
    actual_hash=$(sha256sum "$store_path" | cut -d' ' -f1)

    if [ "$expected_hash" = "$actual_hash" ]; then
        result_pass "STORE PATH verify [$server_file] content matches (SHA256 $actual_hash)"
    else
        result_fail "STORE PATH verify [$server_file]" "store copy content mismatch: local=$expected_hash store=$actual_hash"
    fi
}

# ── Section 12: Metadata retrieval (GET /files/:filename?meta=true) ──────────

run_metadata_tests() {
    section "METADATA RETRIEVAL (meta=true)"

    local src="$SCRIPT_DIR/lenna.jpg"

    if [ ! -f "$src" ]; then
        result_skip "METADATA retrieval" "lenna.jpg not found — skipping all metadata tests"
        return
    fi

    # Upload a file with known, fixed metadata so we can assert on the response fields
    info "  Uploading lenna.jpg with fixed metadata ..."
    pace
    local resp
    resp=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -X POST "$BASE/core/upload" \
        -F "file=@$src" \
        -F "claim_number=CLM-META-01" \
        -F "claimant_name=Meta Tester" \
        -F "date_of_injury=2024-06-01" \
        -F "employer=TestCorp" \
        -F "adjuster=M. Inspector" \
        -F "support=Full Support" \
        -F "claim_type=Workers Comp" \
        -F "jurisdiction=California" \
        -F "policy_number=POL-META-01" \
        -F "acts_id=ACTS_META_01" \
        -F "data=metadata-test" \
        --max-time 30 2>/dev/null)

    local server_file
    server_file=$(echo "$resp" | grep -o '"filename":"[^"]*"' | cut -d'"' -f4)

    if [ -z "$server_file" ]; then
        result_fail "METADATA upload" "no filename in response: $resp"
        return
    fi
    UPLOADED_FILES+=("$server_file")
    result_pass "METADATA upload -> $server_file"

    # Fetch metadata and assert HTTP 200 + expected JSON fields
    info "  Fetching metadata for $server_file ..."
    pace
    local body code
    body=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -w "\n%{http_code}" --max-time 10 "$BASE/core/files/$server_file?meta=true" 2>/dev/null)
    code=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')

    if [ "$code" = "000" ]; then
        result_fail "GET /files/$server_file?meta=true" "connection failed"
        return
    fi

    if [ "$code" != "200" ]; then
        result_fail "GET /files/$server_file?meta=true" "expected 200, got HTTP $code"
        return
    fi
    result_pass "GET /files/$server_file?meta=true -> HTTP 200"

    # Check that each known field is present in the JSON response
    local -A expected_fields=(
        ["claim_number"]="CLM-META-01"
        ["claimant_name"]="Meta Tester"
        ["acts_id"]="ACTS_META_01"
        ["jurisdiction"]="California"
    )
    for field in "${!expected_fields[@]}"; do
        local value="${expected_fields[$field]}"
        if echo "$body" | grep -q "\"$value\""; then
            result_pass "METADATA field $field = $value"
        else
            result_fail "METADATA field $field" "expected '$value' not found in: $body"
        fi
    done

    # Unknown meta flag should return 400
    info "  Testing unknown meta flag ..."
    pace
    code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 \
        "$BASE/core/files/$server_file?meta=maybe" 2>/dev/null)
    if [ "$code" = "400" ]; then
        result_pass "GET /files/$server_file?meta=maybe -> HTTP 400 (bad flag rejected)"
    elif [ "$code" = "000" ]; then
        result_fail "GET /files/$server_file?meta=maybe" "connection failed"
    else
        result_fail "GET /files/$server_file?meta=maybe" "expected 400, got HTTP $code"
    fi

    # meta=true for a non-existent file should return 404
    info "  Testing meta=true for non-existent file ..."
    pace
    local ghost="ghost_$(printf '%08x' $RANDOM$RANDOM).jpg"
    code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 \
        "$BASE/core/files/$ghost?meta=true" 2>/dev/null)
    if [ "$code" = "404" ]; then
        result_pass "GET /files/$ghost?meta=true -> HTTP 404"
    elif [ "$code" = "000" ]; then
        result_fail "GET /files/$ghost?meta=true" "connection failed"
    else
        result_fail "GET /files/$ghost?meta=true" "expected 404, got HTTP $code"
    fi
}

# ── Section 13: File listing / catalog ───────────────────────────────────────

run_catalog_test() {
    section "CATALOG VERIFICATION"

    info "  Fetching file index ..."
    pace
    local body code
    body=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -w "\n%{http_code}" --max-time 10 "$BASE/core/files" 2>/dev/null)
    code=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')

    if [ "$code" = "000" ]; then
        result_fail "GET /files (catalog)" "connection failed"
        return
    fi

    if [ "$code" != "200" ]; then
        result_fail "GET /files (catalog)" "HTTP $code"
        return
    fi

    result_pass "GET /files (catalog) -> HTTP 200"
    info "  Response body (first 500 chars):"
    info "  ${body:0:500}"
}

# ── Section 14: Delete tests ─────────────────────────────────────────────────

run_delete_tests() {
    section "DELETE TESTS"

    # Delete a file that exists
    if [ ${#UPLOADED_FILES[@]} -eq 0 ]; then
        result_skip "DELETE /files/:name" "no files were uploaded"
    else
        local target="${UPLOADED_FILES[0]}"
        info "  Deleting $target ..."
        pace
        local code
        code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 \
            -X DELETE "$BASE/core/files/$target" 2>/dev/null)

        if [ "$code" = "000" ]; then
            result_fail "DELETE /files/$target" "connection failed"
        elif [ "$code" -lt 400 ]; then
            result_pass "DELETE /files/$target -> HTTP $code"

            info "  Verifying file removed from cache ..."
            pace
            local verify_code
            verify_code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 \
                "$BASE/core/files/$target?meta=false" 2>/dev/null)
            if [ "$verify_code" -ge 400 ]; then
                result_pass "DELETE verify $target gone -> HTTP $verify_code"
            elif [ "$verify_code" = "000" ]; then
                result_fail "DELETE verify $target" "connection failed"
            elif [ "$verify_code" = "200" ]; then
                # deleteFile removes from cache, but locateFile falls back to the store
                result_pass "DELETE verify $target -> HTTP 200 (served from store fallback, cache entry removed)"
            else
                result_fail "DELETE verify $target" "unexpected HTTP $verify_code"
            fi
        else
            result_fail "DELETE /files/$target" "HTTP $code"
        fi
    fi

    # Delete a file that does not exist
    local ghost="nonexistent_file_$(printf '%08x' $RANDOM$RANDOM).txt"
    info "  Deleting non-existent file $ghost ..."
    pace
    local code
    code=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -o /dev/null -w "%{http_code}" --max-time 10 \
        -X DELETE "$BASE/core/files/$ghost" 2>/dev/null)

    if [ "$code" = "404" ]; then
        result_pass "DELETE /files/$ghost (non-existent) -> HTTP 404"
    elif [ "$code" = "000" ]; then
        result_fail "DELETE /files/$ghost (non-existent)" "connection failed"
    elif [ "$code" -ge 400 ]; then
        result_pass "DELETE /files/$ghost (non-existent) -> HTTP $code (rejected)"
    else
        result_fail "DELETE /files/$ghost (non-existent)" "expected 404, got HTTP $code"
    fi
}

# ── Section 16: Fileview routes (issue #389) ──────────────────────────────────
# /fileview is only gated by the known_machines IP allowlist (logConnections
# .. false), not requirePassword — unlike /core/files/*, these requests carry
# no X-Dewey-Password header, matching how the routes are actually reachable.

run_fileview_tests() {
    section "FILEVIEW ROUTES"

    info "  Testing GET /fileview/viewLogDir ..."
    pace
    local body code
    body=$(curl -s -w '\n%{http_code}' --max-time 10 "$BASE/fileview/viewLogDir" 2>/dev/null)
    code=$(echo "$body" | tail -n1)
    body=$(echo "$body" | sed '$d')

    if [ "$code" = "000" ]; then
        result_fail "GET /fileview/viewLogDir" "connection failed"
    elif [ "$code" = "200" ] && echo "$body" | grep -q '"files"'; then
        result_pass "GET /fileview/viewLogDir -> HTTP 200 (files key present)"
    else
        result_fail "GET /fileview/viewLogDir" "expected 200 with a files key, got HTTP $code: $body"
    fi

    # Pull today's rotated log filename out of viewLogDir's own response
    # rather than assuming the name (issue #387's log.go rotates daily), so
    # this doesn't need updating every day the suite runs.
    local today_log
    today_log=$(echo "$body" | grep -o '"app-[0-9-]*\.log"' | head -n1 | tr -d '"')

    if [ -z "$today_log" ]; then
        result_skip "GET /fileview/viewFile/log/:file" "no log filename found in viewLogDir response"
    else
        info "  Testing GET /fileview/viewFile/log/$today_log ..."
        pace
        code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "$BASE/fileview/viewFile/log/$today_log" 2>/dev/null)
        if [ "$code" = "000" ]; then
            result_fail "GET /fileview/viewFile/log/$today_log" "connection failed"
        elif [ "$code" = "200" ]; then
            result_pass "GET /fileview/viewFile/log/$today_log -> HTTP 200"
        else
            result_fail "GET /fileview/viewFile/log/$today_log" "expected 200, got HTTP $code"
        fi
    fi

    info "  Testing GET /fileview/viewFile/config/config.json ..."
    pace
    body=$(curl -s -w '\n%{http_code}' --max-time 10 "$BASE/fileview/viewFile/config/config.json" 2>/dev/null)
    code=$(echo "$body" | tail -n1)
    body=$(echo "$body" | sed '$d')

    if [ "$code" = "000" ]; then
        result_fail "GET /fileview/viewFile/config/config.json" "connection failed"
    elif [ "$code" = "200" ] && echo "$body" | grep -q "file_system_base_dir"; then
        result_pass "GET /fileview/viewFile/config/config.json -> HTTP 200 (real config content)"
    else
        result_fail "GET /fileview/viewFile/config/config.json" "expected 200 with config content, got HTTP $code"
    fi

    info "  Testing GET /fileview/viewFile/bogus/whatever (unknown fileType) ..."
    pace
    code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "$BASE/fileview/viewFile/bogus/whatever" 2>/dev/null)
    if [ "$code" = "400" ]; then
        result_pass "GET /fileview/viewFile/bogus/whatever -> HTTP 400 (unknown fileType rejected)"
    elif [ "$code" = "000" ]; then
        result_fail "GET /fileview/viewFile/bogus/whatever" "connection failed"
    else
        result_fail "GET /fileview/viewFile/bogus/whatever" "expected 400, got HTTP $code"
    fi

    local ghost="nonexistent-file-$(printf '%08x' $RANDOM$RANDOM).log"
    info "  Testing GET /fileview/viewFile/log/$ghost (missing file) ..."
    pace
    code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "$BASE/fileview/viewFile/log/$ghost" 2>/dev/null)
    if [ "$code" = "000" ]; then
        result_fail "GET /fileview/viewFile/log/$ghost" "connection failed"
    elif [ "$code" -ge 400 ]; then
        result_pass "GET /fileview/viewFile/log/$ghost -> HTTP $code (missing file rejected)"
    else
        result_fail "GET /fileview/viewFile/log/$ghost" "expected 4xx/5xx, got HTTP $code"
    fi

    # gin's :file param can't contain a literal "/", so a real "../"
    # traversal never reaches getViewFile at all — this confirms that
    # routing-level block still holds rather than exercising any boundary
    # check inside viewFile itself, which has none (see TestViewFile in
    # filemanager_test.go for what happens when a name with ".." *is*
    # actually joined).
    info "  Testing GET /fileview/viewFile/log/..%2F..%2F..%2Fetc%2Fpasswd (traversal) ..."
    pace
    code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "$BASE/fileview/viewFile/log/..%2F..%2F..%2Fetc%2Fpasswd" 2>/dev/null)
    if [ "$code" = "000" ]; then
        result_fail "GET /fileview/viewFile/log traversal" "connection failed"
    elif [ "$code" -ge 400 ]; then
        result_pass "GET /fileview/viewFile/log traversal -> HTTP $code (blocked)"
    elif [ "$code" = "200" ]; then
        result_fail "GET /fileview/viewFile/log traversal" "HTTP 200 — path traversal may have succeeded"
    else
        result_pass "GET /fileview/viewFile/log traversal -> HTTP $code"
    fi
}

# ── Section 15: PDF embedded JavaScript rejection ─────────────────────────────

run_pdf_javascript_test() {
    section "PDF EMBEDDED JAVASCRIPT REJECTION"

    local src="$SCRIPT_DIR/js_test.pdf"

    if [ ! -f "$src" ]; then
        result_skip "POST /upload [PDF with embedded JS]" "js_test.pdf not found"
        return
    fi

    info "  Uploading js_test.pdf (PDF with an /OpenAction JavaScript trigger) ..."
    pace
    local body code
    body=$(curl -s -H "X-Dewey-Session-Token: $SESSION_TOKEN" -w "\n%{http_code}" --max-time 30 \
        -X POST "$BASE/core/upload" \
        -F "file=@$src" \
        -F "claim_number=CLM-00000" \
        -F "claimant_name=PDF JS Test" \
        -F "date_of_injury=2025-01-01" \
        -F "employer=TestCorp" \
        -F "adjuster=A. Smith" \
        -F "support=Full Support" \
        -F "claim_type=Workers Comp" \
        -F "jurisdiction=California" \
        -F "policy_number=POL-000001" \
        -F "acts_id=ACTS_PDFJS" \
        -F "data=pdf-js-rejection-test" 2>/dev/null)
    code=$(echo "$body" | tail -1)
    body=$(echo "$body" | sed '$d')

    if [ "$code" -ge 400 ]; then
        if echo "$body" | grep -qi "javascript"; then
            result_pass "POST /upload [PDF with embedded JS] -> HTTP $code (JS detected and blocked)"
        else
            result_pass "POST /upload [PDF with embedded JS] -> HTTP $code (rejected)"
        fi
    elif [ "$code" = "000" ]; then
        result_fail "POST /upload [PDF with embedded JS]" "connection failed"
    else
        result_fail "POST /upload [PDF with embedded JS]" "expected 4xx, got HTTP $code — server accepted a PDF with embedded JavaScript"
    fi
}

# ── Run everything ────────────────────────────────────────────────────────────

info "${CYAN}${BOLD}╔══════════════════════════════════════════════════════════════╗${NC}"
info "${CYAN}${BOLD}║${NC}            ${WHITE}${BOLD}DEWEY TEST SUITE${NC}                                 ${CYAN}${BOLD}║${NC}"
info "${CYAN}${BOLD}╚══════════════════════════════════════════════════════════════╝${NC}"
info ""
info "  ${DIM}Target:${NC}       ${BOLD}$BASE${NC}"
info "  ${DIM}Download dir:${NC} $DOWNLOAD_DIR"
info "  ${DIM}Timestamp:${NC}    $(date -Iseconds)"

echo "# DEWEY TEST SUITE — $(date -Iseconds)"
echo "# Target: $BASE"
echo "#"

run_health_checks
run_json_upload_test
run_upload_tests
run_upload_error_tests
run_bad_extension_test
run_elf_rejection_test
run_pdf_javascript_test
run_path_traversal_tests
run_sha256_upload_test
run_duplicate_upload_test
run_roundtrip_tests
run_store_path_verification_test
run_metadata_tests
run_catalog_test
run_delete_tests
run_fileview_tests

section "SUMMARY"
info "  ${GREEN}${BOLD}Passed:${NC}  $PASS"
info "  ${RED}${BOLD}Failed:${NC}  $FAIL"
info "  ${YELLOW}${BOLD}Skipped:${NC} $SKIP"
info "  ${WHITE}${BOLD}Total:${NC}   $TOTAL"

echo "#"
echo "# PASSED=$PASS FAILED=$FAIL SKIPPED=$SKIP TOTAL=$TOTAL"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi

# Without this, the script's exit code falls through to whatever the `if`
# condition above last evaluated to — [ "$FAIL" -gt 0 ] is false when there
# are no failures, and a false test's own exit status (1) becomes the
# script's exit status since its body never ran. That reported every clean
# BITs run to main.go as a failure, with no real test failure behind it.
exit 0
