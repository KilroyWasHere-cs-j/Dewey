#!/usr/bin/env bash
set -euo pipefail

BASE="http://localhost:8080"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DOWNLOAD_DIR="$(mktemp -d)"
PASS=0
FAIL=0

cleanup() { rm -rf "$DOWNLOAD_DIR"; }
trap cleanup EXIT

green()  { printf "\033[32m%s\033[0m\n" "$*"; }
red()    { printf "\033[31m%s\033[0m\n" "$*"; }
yellow() { printf "\033[33m%s\033[0m\n" "$*"; }
bold()   { printf "\033[1m%s\033[0m\n" "$*"; }

FILES=("lenna.jpg" "lenna.png" "test.pdf" "bee_moive_script.txt")

bold "=== Roundtrip Upload/Retrieve Test ==="
echo "Download dir: $DOWNLOAD_DIR"
echo ""

for f in "${FILES[@]}"; do
    src="$SCRIPT_DIR/$f"
    if [ ! -f "$src" ]; then
        red "SKIP $f — source file not found"
        continue
    fi

    bold "--- $f ---"

    # Upload
    echo "  Uploading..."
    resp=$(curl -s -X POST "$BASE/upload" \
        -F "file=@$src" \
        -F "claim_number=CLM-99999" \
        -F "claimant_name=Test User" \
        -F "date_of_injury=2025-01-01" \
        -F "employer=TestCorp" \
        -F "adjuster=A. Smith" \
        -F "support=Full Support" \
        -F "claim_type=Workers Comp" \
        -F "jurisdiction=California" \
        -F "policy_number=POL-000001" \
        -F "acts_id=ACTS_TEST" \
        -F "data=roundtrip-test")

    server_file=$(echo "$resp" | grep -o '"filename":"[^"]*"' | cut -d'"' -f4)
    if [ -z "$server_file" ]; then
        red "  FAIL — upload failed: $resp"
        FAIL=$((FAIL + 1))
        continue
    fi
    green "  Uploaded as: $server_file"

    # Retrieve
    echo "  Retrieving..."
    dst="$DOWNLOAD_DIR/$server_file"
    http_code=$(curl -s -o "$dst" -w "%{http_code}" "$BASE/files/$server_file/false")

    if [ "$http_code" != "200" ]; then
        red "  FAIL — retrieve returned HTTP $http_code"
        FAIL=$((FAIL + 1))
        continue
    fi

    # Compare hashes
    hash_orig=$(sha256sum "$src" | cut -d' ' -f1)
    hash_down=$(sha256sum "$dst" | cut -d' ' -f1)

    if [ "$hash_orig" = "$hash_down" ]; then
        green "  PASS — SHA256 match: $hash_orig"
        PASS=$((PASS + 1))
    else
        red "  FAIL — SHA256 mismatch"
        red "    original:   $hash_orig"
        red "    downloaded: $hash_down"
        echo "  file command says: $(file "$dst")"
        FAIL=$((FAIL + 1))
    fi

    # Open visually for images and PDFs
    ext="${f##*.}"
    case "$ext" in
        jpg|jpeg|png|pdf)
            yellow "  Opening $f for visual inspection..."
            xdg-open "$dst" 2>/dev/null &
            ;;
    esac

    echo ""
    sleep 2
done

bold "=== Results: $PASS passed, $FAIL failed ==="
if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
