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
#
# Also requires FILES_PASSWORD in the environment (issue #332) — set on the
# backend container already, so running via podman exec picks it up for
# free; running from outside the container needs it passed explicitly, e.g.
# `FILES_PASSWORD=$(cat .files-password) ./load_test.sh`.

BASE="${1:-http://localhost:8080}"
BASE="${BASE%/}"
COUNT="${2:-500}"
CONCURRENCY="${3:-10}"

TMP_DIR="$(mktemp -d)"
# Declared up front (rather than where it's first written below) since
# cleanup() reads it, and the trap can fire before that point is reached.
FILENAMES="$TMP_DIR/filenames.txt"

cleanup() {
	# Best-effort: delete every file this run uploaded into the live store —
	# same reasoning as test_suite.sh's cleanup (issue #347): this hits the
	# real backend, and nothing else removes these afterward. deleteStoredFile
	# (backend/filemanager.go) removes the store file, cache copy, and DB row
	# together, so one DELETE per name is enough.
	if [ -s "$FILENAMES" ]; then
		echo "Cleaning up: deleting uploaded test files from the live store..."
		while IFS= read -r name; do
			[ -n "$name" ] && curl -s -o /dev/null -H "X-Dewey-Password: $FILES_PASSWORD" \
				-X DELETE "$BASE/core/files/$name" --max-time 10 2>/dev/null
		done <"$FILENAMES"
	fi
	rm -rf "$TMP_DIR"
}
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
	# date_of_injury is required server-side (issue #365) — any valid date
	# works here since this script only exercises upload volume/timing, not
	# metadata content.
	resp="$(curl -s -H "X-Dewey-Password: $FILES_PASSWORD" -F "file=@${f}" -F "date_of_injury=2025-01-01" "$BASE/core/upload")"
	echo "$resp" | grep -o '"filename":"[^"]*"' | cut -d'"' -f4
}
export -f upload_one
export TMP_DIR BASE FILES_PASSWORD

seq 1 "$COUNT" | xargs -P "$CONCURRENCY" -I{} bash -c 'upload_one "$@"' _ {} >"$FILENAMES"

UPLOADED=$(wc -l <"$FILENAMES")
echo "Uploaded $UPLOADED/$COUNT files."

echo "Retrieving them back to exercise retrievalCounter..."
retrieve_one() {
	local name="$1"
	curl -s -o /dev/null -w "%{http_code}\n" -H "X-Dewey-Password: $FILES_PASSWORD" "$BASE/core/files/${name}/false"
}
export -f retrieve_one
export BASE FILES_PASSWORD

xargs -P "$CONCURRENCY" -I{} bash -c 'retrieve_one "$@"' _ {} <"$FILENAMES" | sort | uniq -c

echo "Done."
