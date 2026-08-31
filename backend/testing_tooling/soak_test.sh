#!/usr/bin/env bash
set -uo pipefail

# Simulates sustained, multi-user, time-varying traffic against Dewey
# (issue #311). Where load_test.sh fires one fixed-size burst from a single
# source and exits in seconds, this runs continuously across many simulated
# "users", each issuing requests on a randomized, diurnally-varying cadence
# (busy by day, mostly dead at night) — the kind of pattern the tick-scaling
# daemon's EMA smoothing and active-user backoff (computeTickInterval,
# backend/utils.go) actually needs to be exercised against.
#
# Multi-user simulation binds each simulated user's requests to its own
# loopback alias (127.0.0.2, 127.0.0.3, ...) via `curl --interface`. Every
# container in dewey-pod shares one network namespace, so spinning up more
# containers wouldn't produce distinct source IPs — loopback aliasing is
# what actually gets ActiveUserCount() (backend/prometheus.go) above 1,
# which also closes the gap #309 flags (load_test.sh structurally can't do
# this since every request comes from one source).
#
# Also logs a CSV time series of tick timing, cache-clean cadence, and
# resource usage (RAM/heap/goroutines/fds) sampled at a fixed real-time
# cadence, so a long run's slower-building trends are easy to graph
# afterward rather than only visible live in Grafana.
#
# Run from inside the backend's own container, same as load_test.sh — every
# request needs a source IP registered in known_machines, and the loopback
# aliasing trick above only works from inside the container being tested:
#   podman exec cross-doc-tool-dev ./testing_tooling/soak_test.sh
#
# Also requires FILES_PASSWORD and MACHINES_PASSWORD in the environment
# (issue #332) — both are already set on the backend container, so running
# via podman exec picks them up for free.

BASE="${1:-http://localhost:8080}"
BASE="${BASE%/}"
SIM_DAYS="${2:-30}"
USERS="${3:-5}"
# Real seconds per simulated day. 60 (default) compresses 30 simulated days
# into ~30 real minutes. Pass 86400 for true 1:1 real-time soak testing.
SECONDS_PER_SIM_DAY="${4:-60}"

TIME_SCALE=$(awk -v s="$SECONDS_PER_SIM_DAY" 'BEGIN { print 86400 / s }')
TOTAL_REAL_SECONDS=$(awk -v d="$SIM_DAYS" -v s="$SECONDS_PER_SIM_DAY" 'BEGIN { print d * s }')

# Target requests per simulated day per user at peak (multiplier 1.0) — kept
# as a requests/day rate rather than a fixed seconds-between-requests value
# because the latter doesn't scale with SECONDS_PER_SIM_DAY: at the default
# 60s/simday compression, a fixed "8 simulated seconds between requests"
# works out to ~0.006 real seconds, which the floor below always clamps —
# flattening the diurnal ramp entirely regardless of time of day. Deriving
# the interval from a requests/day target instead keeps it meaningful across
# any compression factor: real_interval = SECONDS_PER_SIM_DAY / (rate * multiplier).
PEAK_REQUESTS_PER_SIM_DAY=200
NIGHT_FLOOR=0.05

# Mirrors test_suite.sh's rand_date() — date_of_injury is required
# server-side (issue #365); soak_test.sh simulates real user traffic, so this
# generates a plausible value rather than a fixed constant.
rand_date() {
	date -d "2024-01-01 + $((RANDOM % 730)) days" +%Y-%m-%d 2>/dev/null || echo "2025-06-15"
}

METRICS_LOG_IP="127.0.0.99"
RUN_ID="$(date +%Y%m%d_%H%M%S)"
METRICS_LOG="/app/logs/soak_metrics_${RUN_ID}.csv"
METRICS_SAMPLE_REAL_SECONDS=10 # fixed cadence, independent of TIME_SCALE

TMP_DIR="$(mktemp -d)"
# Each simulate_user runs as its own backgrounded subshell, so a plain bash
# array won't accumulate across them — every upload appends its filename
# here instead. Single small `echo >>` writes are append-atomic (POSIX
# O_APPEND, well under PIPE_BUF), so concurrent users writing to the same
# file is safe without extra locking.
UPLOADED_FILES_LOG="$TMP_DIR/uploaded_files.txt"
START_REAL=$(date +%s)

echo "Soak test: $SIM_DAYS simulated days, $USERS users, ${SECONDS_PER_SIM_DAY}s/simday (${TIME_SCALE}x compression) against $BASE"
echo "Metrics log: $METRICS_LOG"

# --- machine registration, so every request has a source IP known_machines accepts ---

REGISTERED_IPS=()

register_machine() {
	local ip="$1" label="$2"
	curl -s -o /dev/null -X POST -H "Content-Type: application/json" -H "X-Dewey-Password: $MACHINES_PASSWORD" \
		-d "{\"ip\":\"${ip}\",\"label\":\"${label}\"}" "$BASE/core/machines"
	REGISTERED_IPS+=("$ip")
}

cleanup() {
	echo "Cleaning up: stopping simulated users and deregistering soak-test IPs..."
	jobs -p | xargs -r kill 2>/dev/null
	wait 2>/dev/null
	for ip in "${REGISTERED_IPS[@]:-}"; do
		[ -n "$ip" ] && curl -s -o /dev/null -X DELETE -H "X-Dewey-Password: $MACHINES_PASSWORD" "$BASE/core/machines/${ip}"
	done
	# Best-effort: delete every file this run uploaded into the live store —
	# same reasoning as test_suite.sh's cleanup (issue #347): a soak run can
	# upload thousands of files over a long simulated window, and nothing
	# else removes them afterward. deleteStoredFile (backend/filemanager.go)
	# removes the store file, cache copy, and DB row together, so one DELETE
	# per name is enough. Runs after the `wait` above, so no simulate_user is
	# still appending to the log while this reads it.
	if [ -s "$UPLOADED_FILES_LOG" ]; then
		echo "Cleaning up: deleting uploaded test files from the live store..."
		while IFS= read -r name; do
			[ -n "$name" ] && curl -s -o /dev/null -H "X-Dewey-Password: $FILES_PASSWORD" \
				-X DELETE "$BASE/core/files/$name" --max-time 10 2>/dev/null
		done <"$UPLOADED_FILES_LOG"
	fi
	rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

# --- diurnal traffic curve ---
# hour is simulated hour-of-day (0-23.99...). sin(pi*(hour-9)/12) is
# positive only for hour in (9,21) and peaks at hour=15 — that's the whole
# daytime bump. Only sin() is used since it's the one trig function every
# awk implementation (including busybox's) reliably provides.
rate_multiplier() {
	local hour="$1"
	awk -v h="$hour" -v floor="$NIGHT_FLOOR" 'BEGIN {
		pi = 3.14159265358979
		bump = sin(pi * (h - 9) / 12)
		if (bump < 0) bump = 0
		print floor + (1 - floor) * bump
	}'
}

sim_elapsed_seconds() {
	local real_elapsed=$(($(date +%s) - START_REAL))
	awk -v e="$real_elapsed" -v scale="$TIME_SCALE" 'BEGIN { print e * scale }'
}

sim_hour_of_day() {
	local sim_elapsed="$1"
	awk -v e="$sim_elapsed" 'BEGIN { h = (e / 3600) % 24; if (h < 0) h += 24; print h }'
}

# extract_metric pulls the value off a bare (unlabeled) Prometheus metric
# line — exposition format is "metric_name value" — out of a full /metrics
# scrape body. The trailing space in the pattern keeps e.g.
# app_cache_clean_cycles_total from matching a same-prefixed metric name.
extract_metric() {
	local name="$1" body="$2"
	echo "$body" | grep -E "^${name} " | awk '{print $2}' | head -1
}

# --- one simulated user: uploads/retrieves on a randomized, diurnally-scaled cadence ---
simulate_user() {
	local user_id="$1" ip="$2"
	local uploaded_file=""
	local user_dir="$TMP_DIR/user_${user_id}"
	local req_id=0
	mkdir -p "$user_dir"

	while true; do
		local remaining=$((TOTAL_REAL_SECONDS - ($(date +%s) - START_REAL)))
		if ((remaining <= 0)); then
			break
		fi

		local sim_elapsed hour multiplier sleep_s
		sim_elapsed=$(sim_elapsed_seconds)
		hour=$(sim_hour_of_day "$sim_elapsed")
		multiplier=$(rate_multiplier "$hour")
		# mean real-seconds-between-requests = SECONDS_PER_SIM_DAY / (peak
		# rate * multiplier) — see PEAK_REQUESTS_PER_SIM_DAY comment above
		# for why this has to be derived this way rather than a fixed
		# simulated-seconds constant. Jittered +/-50%, floored at 0.2s so an
		# extreme compression factor can't spin this into a tight loop, and
		# capped at the run's remaining real-time budget — at low (night)
		# rates the jittered interval can otherwise legitimately exceed
		# however much time is actually left, oversleeping straight past
		# the run's intended end since nothing wakes the loop early to
		# recheck (found by soak-testing this script itself at real-time
		# scale over a short window).
		sleep_s=$(awk -v spd="$SECONDS_PER_SIM_DAY" -v rate="$PEAK_REQUESTS_PER_SIM_DAY" -v m="$multiplier" -v r="$RANDOM" -v remaining="$remaining" 'BEGIN {
			jitter = 0.5 + (r % 1000) / 1000.0
			s = (spd / (rate * m)) * jitter
			if (s < 0.2) s = 0.2
			if (s > remaining) s = remaining
			print s
		}')
		sleep "$sleep_s"

		# 70/30 upload-vs-retrieve mix; always upload until this user has
		# something of its own to retrieve.
		if [ -z "$uploaded_file" ] || ((RANDOM % 10 < 7)); then
			# date +%s%N is meant to give nanosecond uniqueness, but the
			# backend container's BusyBox date silently drops %N and
			# returns plain whole seconds (issue #367) — two requests from
			# this same user landing in the same wall-clock second would
			# reuse the identical original filename. A per-user counter is
			# unique regardless of clock resolution.
			req_id=$((req_id + 1))
			local f="$user_dir/soak_${user_id}_${req_id}.txt"
			head -c 512 /dev/urandom | base64 >"$f"
			local resp name
			resp="$(curl -s --interface "$ip" -H "X-Dewey-Password: $FILES_PASSWORD" -F "file=@${f}" -F "date_of_injury=$(rand_date)" "$BASE/core/upload")"
			name=$(echo "$resp" | grep -o '"filename":"[^"]*"' | cut -d'"' -f4)
			if [ -n "$name" ]; then
				uploaded_file="$name"
				echo "$name" >>"$UPLOADED_FILES_LOG"
			fi
			rm -f "$f"
		else
			curl -s -o /dev/null --interface "$ip" -H "X-Dewey-Password: $FILES_PASSWORD" "$BASE/core/files/${uploaded_file}/false"
		fi
	done
}

# --- metrics logger: samples /metrics on a fixed real-time cadence, writes CSV ---
log_metrics() {
	echo "timestamp,sim_day,sim_hour,time_til_next_tick,cache_clean_cycles_total,ram_usage_mb,heap_usage_mb,goroutines,open_fds,connected_users,upload_rate" >"$METRICS_LOG"

	while true; do
		local remaining=$((TOTAL_REAL_SECONDS - ($(date +%s) - START_REAL)))
		if ((remaining <= 0)); then
			break
		fi

		local body sim_elapsed hour day
		body="$(curl -s --interface "$METRICS_LOG_IP" "$BASE/metrics")"
		sim_elapsed=$(sim_elapsed_seconds)
		hour=$(sim_hour_of_day "$sim_elapsed")
		day=$(awk -v e="$sim_elapsed" 'BEGIN { print int(e / 86400) }')

		echo "$(date +%s),${day},${hour},$(extract_metric app_time_til_next_tick "$body"),$(extract_metric app_cache_clean_cycles_total "$body"),$(extract_metric app_ram_usage "$body"),$(extract_metric app_heap_usage "$body"),$(extract_metric go_goroutines "$body"),$(extract_metric process_open_fds "$body"),$(extract_metric app_connected_users "$body"),$(extract_metric app_upload_rate "$body")" >>"$METRICS_LOG"

		# Capped at the remaining budget, same reasoning as simulate_user's
		# sleep above — a short total run shouldn't wait a full fixed
		# sample interval past its own intended end.
		local sleep_s=$METRICS_SAMPLE_REAL_SECONDS
		((sleep_s > remaining)) && sleep_s=$remaining
		sleep "$sleep_s"
	done
}

for i in $(seq 1 "$USERS"); do
	register_machine "127.0.0.$((i + 1))" "soak-user-${i}"
done
register_machine "$METRICS_LOG_IP" "soak-metrics-logger"

for i in $(seq 1 "$USERS"); do
	simulate_user "$i" "127.0.0.$((i + 1))" &
done
log_metrics &

wait
echo "Soak test complete. Metrics log: $METRICS_LOG"
