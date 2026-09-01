package main

import "testing"

// sampleMetrics mirrors a real scrape's shape closely enough to catch
// parsing regressions: myapp_ namespace + ConstLabels on the file_io_*
// counters (backend/prometheus.go), single-label reason/ext on the upload
// counters, and the standard go_/process_ collector names the frontend
// already relies on.
const sampleMetrics = `# HELP app_ram_usage Total memory obtained from the OS by the application
# TYPE app_ram_usage gauge
app_ram_usage 82
# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines 17
# HELP process_open_fds Number of open file descriptors.
# TYPE process_open_fds gauge
process_open_fds 14
app_files_in_store 128
app_time_til_next_tick 252
app_uptime_seconds 12240
# HELP myapp_file_io_bytes_total Total bytes read/written.
# TYPE myapp_file_io_bytes_total counter
myapp_file_io_bytes_total{app="dewey",op="read"} 1.288490188e+09
myapp_file_io_bytes_total{app="dewey",op="write"} 5.36870912e+08
# HELP myapp_file_io_ops_total Counts of file IO operations.
# TYPE myapp_file_io_ops_total counter
myapp_file_io_ops_total{app="dewey",file_group="default",op="read",result="ok"} 500
myapp_file_io_ops_total{app="dewey",file_group="default",op="write",result="ok"} 200
# HELP app_upload_rejections_total Number of upload rejections, labelled by reason
# TYPE app_upload_rejections_total counter
app_upload_rejections_total{reason="invalid_ext"} 5
app_upload_rejections_total{reason="pe_blocked"} 4
# HELP app_uploads_by_type_total Number of accepted uploads, labelled by file extension
# TYPE app_uploads_by_type_total counter
app_uploads_by_type_total{ext=".jpg"} 312
app_uploads_by_type_total{ext=".pdf"} 204
process_network_receive_bytes_total 19292160
process_network_transmit_bytes_total 10171187
`

func TestParsePrometheusMetrics_Scalars(t *testing.T) {
	snap := parsePrometheusMetrics(sampleMetrics)

	cases := map[string]float64{
		"app_ram_usage":                        82,
		"go_goroutines":                        17,
		"process_open_fds":                     14,
		"app_files_in_store":                   128,
		"app_time_til_next_tick":                252,
		"app_uptime_seconds":                   12240,
		"process_network_receive_bytes_total":  19292160,
		"process_network_transmit_bytes_total": 10171187,
	}
	for name, want := range cases {
		if got := snap.scalars[name]; got != want {
			t.Errorf("scalars[%q] = %v, want %v", name, got, want)
		}
	}
}

func TestParsePrometheusMetrics_NamespaceStripped(t *testing.T) {
	snap := parsePrometheusMetrics(sampleMetrics)

	// myapp_ prefix must be stripped, same as the frontend's parser, and
	// the ConstLabels "app" pair must not prevent the "op" pair from being
	// found once aggregateByOp reduces the multi-label key.
	if _, ok := snap.labeled["file_io_bytes_total"]; !ok {
		t.Fatalf("expected file_io_bytes_total under the stripped name, got keys: %v", keysOf(snap.labeled))
	}
	byOp := aggregateByOp(snap.labeled["file_io_bytes_total"])
	if byOp["read"] != 1288490188 {
		t.Errorf("file_io_bytes_total read = %v, want 1288490188", byOp["read"])
	}
	if byOp["write"] != 536870912 {
		t.Errorf("file_io_bytes_total write = %v, want 536870912", byOp["write"])
	}
}

func TestParsePrometheusMetrics_MultiLabelOpsAggregation(t *testing.T) {
	snap := parsePrometheusMetrics(sampleMetrics)

	// file_io_ops_total has 4 labels (app, file_group, op, result) — the
	// per-metric key won't be a bare "read"/"write" like the single-label
	// case, but aggregateByOp must still pull the right op out and sum
	// across the other labels correctly.
	byOp := aggregateByOp(snap.labeled["file_io_ops_total"])
	if byOp["read"] != 500 {
		t.Errorf("file_io_ops_total read = %v, want 500", byOp["read"])
	}
	if byOp["write"] != 200 {
		t.Errorf("file_io_ops_total write = %v, want 200", byOp["write"])
	}
}

func TestParsePrometheusMetrics_SingleLabelCollapsesToValue(t *testing.T) {
	snap := parsePrometheusMetrics(sampleMetrics)

	// app_uploads_by_type_total only has one label (ext), so buildLabelKey
	// should collapse straight to the bare value (".jpg"), not "ext=.jpg".
	if got := snap.labeled["app_uploads_by_type_total"][".jpg"]; got != 312 {
		t.Errorf(`uploads_by_type[".jpg"] = %v, want 312`, got)
	}
	if got := snap.labeled["app_uploads_by_type_total"][".pdf"]; got != 204 {
		t.Errorf(`uploads_by_type[".pdf"] = %v, want 204`, got)
	}
}

func TestParsePrometheusMetrics_SumMap(t *testing.T) {
	snap := parsePrometheusMetrics(sampleMetrics)

	total := sumMap(snap.labeled["app_upload_rejections_total"])
	if total != 9 {
		t.Errorf("sumMap(app_upload_rejections_total) = %v, want 9 (5+4)", total)
	}
}

func TestParsePrometheusMetrics_IgnoresCommentsAndBlankLines(t *testing.T) {
	snap := parsePrometheusMetrics("# just a comment\n\n   \napp_db_errors 0\n")
	if _, ok := snap.scalars["app_db_errors"]; !ok {
		t.Fatal("expected app_db_errors to be parsed despite surrounding comments/blank lines")
	}
}

func TestParsePrometheusMetrics_UnrecognizedLabeledMetricSkipped(t *testing.T) {
	// A labeled metric not in labeledMetrics (e.g. gin's own request
	// histogram) should be silently skipped, same as the frontend.
	snap := parsePrometheusMetrics(`gin_request_size_bytes_bucket{le="1024"} 5` + "\n")
	if len(snap.labeled) != 0 {
		t.Errorf("expected no labeled metrics captured, got: %v", snap.labeled)
	}
	if len(snap.scalars) != 0 {
		t.Errorf("expected no scalars captured, got: %v", snap.scalars)
	}
}

func TestPctOf(t *testing.T) {
	cases := []struct {
		value, maxRef float64
		want          int
	}{
		{50, 100, 50},
		{150, 100, 100}, // clamped at 100
		{-10, 100, 0},   // clamped at 0
		{50, 0, 0},      // avoid divide-by-zero
	}
	for _, c := range cases {
		if got := pctOf(c.value, c.maxRef); got != c.want {
			t.Errorf("pctOf(%v, %v) = %v, want %v", c.value, c.maxRef, got, c.want)
		}
	}
}

func TestFmtCountdown(t *testing.T) {
	cases := map[float64]string{
		0:    "00:00",
		65:   "01:05",
		3665: "01:01:05",
	}
	for in, want := range cases {
		if got := fmtCountdown(in); got != want {
			t.Errorf("fmtCountdown(%v) = %q, want %q", in, got, want)
		}
	}
}

func keysOf(m map[string]map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
