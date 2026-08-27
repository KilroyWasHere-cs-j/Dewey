#!/usr/bin/env bash
set -uo pipefail

# Verifies computeTickInterval's active-users term (issue #309) by forcing
# genuinely concurrent requests from distinct source IPs and scraping
# /metrics while they're still in flight, rather than soak_test.sh's
# independently-randomized per-user schedule — at typical request rates,
# brief non-overlapping requests almost never land inside the same 10s
# scrape window, so ActiveUserCount() never reads above 1 under that
# approach.
#
# A real backend request over a small file only takes microseconds, and
# curl's --limit-rate against a SMALL file doesn't help either: the whole
# payload fits inside the kernel's TCP send buffer, so the server's write()
# returns (and trackUser's defer fires) the instant it hands the bytes to
# the OS, regardless of how slowly the client is actually draining them —
# the throttling only affects the client's read rate, never the server's
# in-flight window. This uploads one large synthetic file first (default
# 8MB) specifically so it's bigger than typical send-buffer autotuning —
# once the client can't keep up, the server's write genuinely blocks on
# real TCP backpressure, and that blocked time is what keeps the request
# counted as in-flight for trackUser's whole duration.
#
# Also requires the /metrics-doesn't-self-count fix (logConnections'
# trackActivity flag, backend/main.go) — without it, every scrape of
# /metrics counts its own in-flight connection and the result here is
# meaningless (connected_users would always read at least 1, burst or not).
#
# Run from inside the backend's own container, same as load_test.sh/
# soak_test.sh — every request needs a source IP registered in
# known_machines, and the loopback aliasing trick below only works from
# inside the container being tested:
#   podman exec cross-doc-tool-dev ./testing_tooling/concurrency_test.sh
#
# Also requires FILES_PASSWORD and MACHINES_PASSWORD in the environment
# (issue #332) — both are already set on the backend container, so running
# via podman exec picks them up for free.

BASE="${1:-http://localhost:8080}"
BASE="${BASE%/}"
USERS="${2:-10}"
LIMIT_RATE="${3:-200k}" # per-download throttle; lower = longer in-flight window

: "${FILES_PASSWORD:?FILES_PASSWORD must be set}"
: "${MACHINES_PASSWORD:?MACHINES_PASSWORD must be set}"

REGISTERED_IPS=()
TEST_FILE=""
UPLOADED_FILENAME=""

register_machine() {
	local ip="$1" label="$2"
	curl -s -o /dev/null -X POST -H "Content-Type: application/json" -H "X-Dewey-Password: $MACHINES_PASSWORD" \
		-d "{\"ip\":\"${ip}\",\"label\":\"${label}\"}" "$BASE/core/machines"
	REGISTERED_IPS+=("$ip")
}

cleanup() {
	echo "Cleaning up..."
	if [ -n "$UPLOADED_FILENAME" ]; then
		curl -s -o /dev/null -X DELETE -H "X-Dewey-Password: $FILES_PASSWORD" "$BASE/core/files/${UPLOADED_FILENAME}"
	fi
	[ -n "$TEST_FILE" ] && rm -f "$TEST_FILE"
	for ip in "${REGISTERED_IPS[@]}"; do
		curl -s -o /dev/null -X DELETE -H "X-Dewey-Password: $MACHINES_PASSWORD" "$BASE/core/machines/${ip}"
	done
}
trap cleanup EXIT

echo "Registering $USERS loopback-alias users..."
for i in $(seq 1 "$USERS"); do
	register_machine "127.0.0.$((i + 1))" "concurrency-test-${i}"
done

echo "Generating and uploading an 8MB test file (large enough to outrun TCP send-buffer autotuning)..."
# .txt content must actually sniff as text/plain to pass the upload's
# extension/content validator (filevalidator.go) — base64-encoded random
# bytes gives printable text at the right size, not raw binary.
TEST_FILE="$(mktemp -d)/concurrency_test.txt"
dd if=/dev/urandom bs=1M count=6 status=none | base64 >"$TEST_FILE"

upload_resp="$(curl -s -H "X-Dewey-Password: $FILES_PASSWORD" -F "file=@${TEST_FILE}" "$BASE/core/upload")"
UPLOADED_FILENAME="$(echo "$upload_resp" | grep -o '"filename":"[^"]*"' | head -1 | cut -d'"' -f4)"
if [ -z "$UPLOADED_FILENAME" ]; then
	echo "FAIL — upload didn't return a filename: $upload_resp" >&2
	exit 1
fi
echo "Uploaded as $UPLOADED_FILENAME"

scrape_metric() {
	local name="$1" body
	body="$(curl -s "$BASE/metrics")"
	echo "$body" | grep "^${name} " | awk '{print $2}'
}

echo "Baseline (before burst):"
baseline_connected="$(scrape_metric app_connected_users)"
baseline_tick="$(scrape_metric app_time_til_next_tick)"
echo "  connected_users=${baseline_connected:-?} time_til_next_tick=${baseline_tick:-?}"

echo "Firing $USERS concurrent throttled downloads (--limit-rate $LIMIT_RATE), polling /metrics while they're in flight..."

for i in $(seq 1 "$USERS"); do
	ip="127.0.0.$((i + 1))"
	curl -s --interface "$ip" --limit-rate "$LIMIT_RATE" \
		-H "X-Dewey-Password: $FILES_PASSWORD" \
		"$BASE/core/files/${UPLOADED_FILENAME}?meta=false" -o /dev/null &
done

# 8MB at $LIMIT_RATE takes real seconds once backpressure kicks in, so a
# plain sleep-spaced poll loop is reliable here — no need to race process
# start-up jitter the way a sub-millisecond-duration request would require.
#
# Records time_til_next_tick alongside connected_users on every sample, not
# just the peak concurrency itself — computeTickInterval only runs once per
# daemon tick (not continuously), so whether alpha*activeUsers actually
# influenced anything is only visible at the tick that happens to land
# while the burst is still in flight, not at every sample.
max_connected=0
peak_tick=""
echo "elapsed_s,connected_users,time_til_next_tick"
for i in $(seq 1 20); do
	body="$(curl -s "$BASE/metrics")"
	connected="$(echo "$body" | grep '^app_connected_users' | awk '{print $2}')"
	tick="$(echo "$body" | grep '^app_time_til_next_tick' | awk '{print $2}')"
	echo "${i},${connected:-},${tick:-}"
	if [ -n "$connected" ] && awk "BEGIN { exit !($connected >= $max_connected) }"; then
		max_connected="$connected"
		peak_tick="$tick"
	fi
	sleep 1
done

wait # let every download finish before this script exits

after_tick="$(scrape_metric app_time_til_next_tick)"

echo ""
echo "baseline: connected_users=${baseline_connected:-?} time_til_next_tick=${baseline_tick:-?}"
echo "peak:     connected_users=$max_connected time_til_next_tick=${peak_tick:-?}"
echo "after:    time_til_next_tick=${after_tick:-?} (burst finished, downloads no longer in flight)"

if awk "BEGIN { exit !($max_connected > 1) }"; then
	echo "PASS — active-users term is exercised above 1 (max connected_users=$max_connected)"
	echo "Whether the formula reacted: compare the elapsed_s,connected_users,time_til_next_tick" \
		"table above — a fresh tick landing while connected_users was elevated should show a" \
		"higher reset value than the baseline/idle peaks typically seen; time_til_next_tick" \
		"counts down between ticks, so read local maxima (right after a reset), not raw samples."
	exit 0
else
	echo "FAIL — connected_users still <= 1; try a higher USERS count, a lower LIMIT_RATE, or investigate further"
	exit 1
fi
