package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dewey-httpclient"
)

// metricsSnapshot holds one parsed /metrics scrape: scalar metrics by name,
// and a small set of labeled metrics collapsed into label->value maps.
// Mirrors the frontend's parsePrometheusMetrics
// (frontend/doctooladmin/src/lib/server/metricsParser.ts) so the two stay
// readable side by side, though this only keeps what the dashboard displays.
type metricsSnapshot struct {
	scalars map[string]float64
	labeled map[string]map[string]float64
	err     error
}

// labeledMetrics lists which metric names get collapsed into a
// label->value map instead of being skipped as unrecognized.
var labeledMetrics = map[string]bool{
	"file_io_ops_total":           true,
	"file_io_bytes_total":         true,
	"app_upload_rejections_total": true,
	"app_uploads_by_type_total":   true,
}

var labelPairRe = regexp.MustCompile(`(\w+)="([^"]*)"`)

// buildLabelKey turns a Prometheus label string like `op="read",app="dewey"`
// into a compact key: the bare value if there's exactly one label pair
// (e.g. "read"), or "k=v,k=v" joined if there's more than one.
func buildLabelKey(labelStr string) string {
	matches := labelPairRe.FindAllStringSubmatch(labelStr, -1)
	if len(matches) == 1 {
		return matches[0][2]
	}
	parts := make([]string, len(matches))
	for i, m := range matches {
		parts[i] = m[1] + "=" + m[2]
	}
	return strings.Join(parts, ",")
}

// parsePrometheusMetrics parses Prometheus text-exposition format the same
// way the frontend's metricsParser.ts does: the "myapp_" namespace prefix
// (backend/prometheus.go's file_io_* metrics) is stripped, scalar metrics
// are kept as-is, and metrics listed in labeledMetrics are collapsed into a
// label->value map via buildLabelKey. Labeled metrics not in that set are
// silently skipped, same as the frontend.
func parsePrometheusMetrics(text string) metricsSnapshot {
	snap := metricsSnapshot{
		scalars: make(map[string]float64),
		labeled: make(map[string]map[string]float64),
	}

	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		spaceIdx := strings.LastIndex(line, " ")
		if spaceIdx == -1 {
			continue
		}
		keyPart := strings.TrimSpace(line[:spaceIdx])
		value, err := strconv.ParseFloat(strings.TrimSpace(line[spaceIdx+1:]), 64)
		if err != nil {
			continue
		}

		metricName := keyPart
		var labelStr string
		hasLabels := false
		if braceIdx := strings.Index(keyPart, "{"); braceIdx > 0 && strings.HasSuffix(keyPart, "}") {
			metricName = keyPart[:braceIdx]
			labelStr = keyPart[braceIdx+1 : len(keyPart)-1]
			hasLabels = true
		}
		cleanName := strings.TrimPrefix(metricName, "myapp_")

		if hasLabels {
			if !labeledMetrics[cleanName] {
				continue
			}
			labelKey := buildLabelKey(labelStr)
			if snap.labeled[cleanName] == nil {
				snap.labeled[cleanName] = make(map[string]float64)
			}
			snap.labeled[cleanName][labelKey] += value
		} else {
			snap.scalars[cleanName] = value
		}
	}

	return snap
}

// aggregateByOp collapses a labeled metric's keys down to just the "op="
// component, summing across any other labels present (result, file_group,
// the ConstLabels "app", etc.) — same reduction the analytics page's
// ioOpsByType does client-side, applied here to both file_io_ops_total and
// file_io_bytes_total so their bar/gauge display isn't a raw multi-label key.
func aggregateByOp(byLabel map[string]float64) map[string]float64 {
	out := make(map[string]float64)
	for key, val := range byLabel {
		op := key
		for _, part := range strings.Split(key, ",") {
			if strings.HasPrefix(part, "op=") {
				op = strings.TrimPrefix(part, "op=")
				break
			}
		}
		out[op] += val
	}
	return out
}

// fetchMetrics gets a single /metrics scrape and parses it. /metrics sits
// behind the known_machines IP allowlist but not requirePassword (issue
// #348's "Where" section), so no X-Dewey-Password header is needed here —
// same as the health/version commands.
func fetchMetrics(host string) metricsSnapshot {
	client := http.Client{
		Timeout: httpclient.RequestTimeout,
	}
	resp, err := client.Get(host + "/metrics")
	if err != nil {
		return metricsSnapshot{err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return metricsSnapshot{err: fmt.Errorf("HTTP %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return metricsSnapshot{err: err}
	}

	return parsePrometheusMetrics(string(body))
}

// pollMetrics fetches host's /metrics on a fixed interval and pushes each
// result to ch. Deliberately kept off the TUI's own goroutine: State.Set
// must only be called from the app's main event loop (go-tui's own
// documented rule), and an HTTP round trip can block for the timeout
// duration on a stalled connection — a Watch on this channel lets the
// event loop pick up each result when it's ready instead of blocking on it.
func pollMetrics(host string, interval time.Duration, ch chan<- metricsSnapshot) {
	ch <- fetchMetrics(host) // first fetch immediately, don't wait a full interval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		ch <- fetchMetrics(host)
	}
}

// pctOf scales value into a 0-100 display percentage against cap, clamped
// at both ends. These caps are display-only sizing for the gauge bar —
// unlike the frontend's user-configurable ramAlertThresholdMb, there's no
// equivalent server-side limit exposed over the API for the CLI to read,
// so these are just reasonable fixed reference points for the bar to fill
// against, not real alert thresholds.
func pctOf(value, maxRef float64) int {
	if maxRef <= 0 {
		return 0
	}
	pct := int(value / maxRef * 100)
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

// fmtCountdown mirrors the frontend's StatTile countdown formatter
// (fmtCountdown in StatTile.svelte): mm:ss, or hh:mm:ss past an hour.
func fmtCountdown(totalSeconds float64) string {
	s := int(totalSeconds)
	if s < 0 {
		s = 0
	}
	hh, mm, ss := s/3600, (s%3600)/60, s%60
	if hh > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hh, mm, ss)
	}
	return fmt.Sprintf("%02d:%02d", mm, ss)
}

// humanizeBytes renders a raw byte count as the largest unit that keeps it
// readable, matching the general shape of the frontend's MB conversions.
func humanizeBytes(b float64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", b/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", b/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", b/(1<<10))
	default:
		return fmt.Sprintf("%.0f B", b)
	}
}
