#!/usr/bin/env bash
set -uo pipefail

# Fires a burst of small text uploads (plus a retrieval pass) at Dewey to
# exercise the tick-scaling logic from issue #305: uploadCounter /
# retrievalCounter's 30s sliding window, the EMA smoothing in startDaemon,
# and computeTickInterval's effect on app_time_til_next_tick / app_upload_rate.
#
# Run this from somewhere with a registered IP (known_machines) — an
# external host connecting through podman's rootless port-forwarding NATs
# to an unregistered IP and gets rejected by the allowlist. From the repo
# root: `podman exec cross-doc-tool-dev ./testing_tooling/load_test.sh`
# runs it from inside the backend container itself, same as the BITs suite.

BASE="${1:-http://localhost:8080}"
BASE="${BASE%/}"
COUNT="${2:-500}"
CONCURRENCY="${3:-10}"

TMP_DIR="$(mktemp -d)"
cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

echo "Uploading $COUNT files to $BASE (concurrency: $CONCURRENCY)..."

upload_one() {
	local i="$1"
	local f="$TMP_DIR/loadtest_${i}.txt"
	# Small, random-ish content so it sniffs as text/plain and clears
	# MatchesDeclaredType (filevalidator.go:229) without eating much disk —
	# ~1KB per file, so even a few thousand of these stays trivial.
	head -c 512 /dev/urandom | base64 >"$f"
	local resp
	resp="$(curl -s -F "file=@${f}" "$BASE/upload")"
	echo "$resp" | grep -o '"filename":"[^"]*"' | cut -d'"' -f4
}
export -f upload_one
export TMP_DIR BASE

FILENAMES="$TMP_DIR/filenames.txt"
seq 1 "$COUNT" | xargs -P "$CONCURRENCY" -I{} bash -c 'upload_one "$@"' _ {} >"$FILENAMES"

UPLOADED=$(wc -l <"$FILENAMES")
echo "Uploaded $UPLOADED/$COUNT files."

echo "Retrieving them back to exercise retrievalCounter..."
retrieve_one() {
	local name="$1"
	curl -s -o /dev/null -w "%{http_code}\n" "$BASE/files/${name}/false"
}
export -f retrieve_one
export BASE

xargs -P "$CONCURRENCY" -I{} bash -c 'retrieve_one "$@"' _ {} <"$FILENAMES" | sort | uniq -c

echo "Done."
