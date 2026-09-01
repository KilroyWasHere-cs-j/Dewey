package main

import (
	"fmt"
	"time"

	tui "github.com/grindlemire/go-tui"
)

// pollInterval mirrors the frontend analytics page's default pollIntervalMs
// (settings.svelte.ts DEFAULTS.pollIntervalMs = 5000).
const pollInterval = 5 * time.Second

// sparkHistoryLen caps how many samples the two sparkline slots keep —
// same rolling-window idea as MetricGraph.svelte's maxHistory, just a fixed
// value here since there's no settings store on the CLI side.
const sparkHistoryLen = 20

// metricsDashboard is the terminal metrics view for issue #348. It polls
// the real backend on pollInterval and renders it — RAM/heap gauges are
// scaled against fixed display caps (see pctOf in metrics_fetch.go), since
// the CLI has no equivalent to the frontend's user-configurable alert
// thresholds to read.
type metricsDashboard struct {
	host             string
	snapshot         *tui.State[metricsSnapshot]
	retrievalHistory *tui.State[[]int]
	rejectionHistory *tui.State[[]int]
	pollCh           chan metricsSnapshot
	scrollY          *tui.State[int]
	contentRef       *tui.Ref
}

func MetricsDashboard(host string) *metricsDashboard {
	pollCh := make(chan metricsSnapshot, 1)
	go pollMetrics(host, pollInterval, pollCh)

	return &metricsDashboard{
		host:             host,
		snapshot:         tui.NewState(metricsSnapshot{}),
		retrievalHistory: tui.NewState([]int{}),
		rejectionHistory: tui.NewState([]int{}),
		pollCh:           pollCh,
		scrollY:          tui.NewState(0),
		contentRef:       tui.NewRef(),
	}
}

// applySnapshot runs on the app's main event loop (via the Watch below),
// so it's safe to call .Set() here directly.
func (m *metricsDashboard) applySnapshot(snap metricsSnapshot) {
	m.snapshot.Set(snap)

	pushHistory := func(state *tui.State[[]int], v int) {
		hist := append(append([]int{}, state.Get()...), v)
		if len(hist) > sparkHistoryLen {
			hist = hist[len(hist)-sparkHistoryLen:]
		}
		state.Set(hist)
	}
	pushHistory(m.retrievalHistory, int(snap.scalars["app_file_retrievals"]))
	pushHistory(m.rejectionHistory, int(sumMap(snap.labeled["app_upload_rejections_total"])))
}

func (m *metricsDashboard) Watchers() []tui.Watcher {
	return []tui.Watcher{
		tui.Watch(m.pollCh, m.applySnapshot),
	}
}

func (m *metricsDashboard) scrollBy(delta int) {
	el := m.contentRef.El()
	if el == nil {
		return
	}
	_, maxY := el.MaxScroll()
	newY := m.scrollY.Get() + delta
	if newY < 0 {
		newY = 0
	} else if newY > maxY {
		newY = maxY
	}
	m.scrollY.Set(newY)
}

func (m *metricsDashboard) KeyMap() tui.KeyMap {
	return tui.KeyMap{
		tui.On(tui.KeyEscape, func(ke tui.KeyEvent) { ke.App().Stop() }),
		tui.On(tui.Rune('q'), func(ke tui.KeyEvent) { ke.App().Stop() }),
		tui.On(tui.Rune('j'), func(ke tui.KeyEvent) { m.scrollBy(1) }),
		tui.On(tui.Rune('k'), func(ke tui.KeyEvent) { m.scrollBy(-1) }),
		tui.On(tui.KeyDown, func(ke tui.KeyEvent) { m.scrollBy(1) }),
		tui.On(tui.KeyUp, func(ke tui.KeyEvent) { m.scrollBy(-1) }),
	}
}

func (m *metricsDashboard) HandleMouse(me tui.MouseEvent) bool {
	switch me.Button {
	case tui.MouseWheelUp:
		m.scrollBy(-1)
		return true
	case tui.MouseWheelDown:
		m.scrollBy(1)
		return true
	}
	return false
}

// gaugeColor mirrors the analytics page's alert-threshold coloring —
// green under normal load, yellow/red as the gauge fills toward its cap.
func gaugeColor(pct int) string {
	if pct >= 80 {
		return "text-red font-bold"
	}
	if pct >= 60 {
		return "text-yellow"
	}
	return "text-green"
}

// gaugeBar renders a fixed-width filled/empty block bar for a 0-100 value —
// same block-character approach as go-tui's own dashboard example.
func gaugeBar(pct int) string {
	width := 16
	filled := pct * width / 100
	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	return bar
}

// sparkline renders data as a block-character sparkline, auto-scaled to
// its own max — same shape as go-tui's own dashboard example's sparkline().
func sparkline(data []int) string {
	if len(data) == 0 {
		return "(no data yet)"
	}
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	maxVal := 1
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}
	s := ""
	for _, v := range data {
		idx := v * 7 / maxVal
		if idx > 7 {
			idx = 7
		}
		if idx < 0 {
			idx = 0
		}
		s += string(blocks[idx])
	}
	return s
}

// sumMap totals every value in a label->value map (e.g. every reason under
// app_upload_rejections_total) — same reduction the analytics page's
// uploadRejectionsTotal does client-side.
func sumMap(m map[string]float64) float64 {
	var total float64
	for _, v := range m {
		total += v
	}
	return total
}

// --- accessors into the current snapshot, with safe zero-value fallbacks
// (identical spirit to the frontend's `metrics.x ?? 0`) -------------------

func (m *metricsDashboard) scalar(name string) float64 {
	return m.snapshot.Get().scalars[name]
}

func (m *metricsDashboard) opBytes(op string) float64 {
	return aggregateByOp(m.snapshot.Get().labeled["file_io_bytes_total"])[op]
}

func (m *metricsDashboard) opCount(op string) float64 {
	return aggregateByOp(m.snapshot.Get().labeled["file_io_ops_total"])[op]
}

func (m *metricsDashboard) uploadsByType(fileType string) float64 {
	return m.snapshot.Get().labeled["app_uploads_by_type_total"][fileType]
}

templ (m *metricsDashboard) Render() {
	<div class="flex-col p-1 gap-1 h-full border-rounded border-cyan">
		<div class="flex justify-center shrink-0">
			<span class="text-gradient-cyan-magenta font-bold">Dewey — Live Metrics</span>
		</div>

		<div class="flex justify-center shrink-0">
			if m.snapshot.Get().err != nil {
				<span class="text-red font-bold">{fmt.Sprintf("unreachable: %v", m.snapshot.Get().err)}</span>
			} else {
				<span class="font-dim">{fmt.Sprintf("polling %s every %s", m.host, pollInterval)}</span>
			}
		</div>

		<div
			ref={m.contentRef}
			class="flex-col gap-1"
			flexGrow={1.0}
			scrollable={tui.ScrollVertical}
			scrollOffset={0, m.scrollY.Get()}
		>
			<div class="flex gap-1 shrink-0">
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="text-gradient-cyan-magenta font-bold">RAM Usage</span>
					<span class={gaugeColor(pctOf(m.scalar("app_ram_usage"), 500))}>{gaugeBar(pctOf(m.scalar("app_ram_usage"), 500))}</span>
					<span class={gaugeColor(pctOf(m.scalar("app_ram_usage"), 500)) + " font-bold"}>{fmt.Sprintf("%.0f MB", m.scalar("app_ram_usage"))}</span>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="text-gradient-cyan-magenta font-bold">Heap Usage</span>
					<span class={gaugeColor(pctOf(m.scalar("app_heap_usage"), 200))}>{gaugeBar(pctOf(m.scalar("app_heap_usage"), 200))}</span>
					<span class={gaugeColor(pctOf(m.scalar("app_heap_usage"), 200)) + " font-bold"}>{fmt.Sprintf("%.0f MB", m.scalar("app_heap_usage"))}</span>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="text-gradient-cyan-magenta font-bold">Goroutines</span>
					<span class={gaugeColor(pctOf(m.scalar("go_goroutines"), 200))}>{gaugeBar(pctOf(m.scalar("go_goroutines"), 200))}</span>
					<span class={gaugeColor(pctOf(m.scalar("go_goroutines"), 200)) + " font-bold"}>{fmt.Sprintf("%.0f", m.scalar("go_goroutines"))}</span>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="text-gradient-cyan-magenta font-bold">Open FDs</span>
					<span class={gaugeColor(pctOf(m.scalar("process_open_fds"), 100))}>{gaugeBar(pctOf(m.scalar("process_open_fds"), 100))}</span>
					<span class={gaugeColor(pctOf(m.scalar("process_open_fds"), 100)) + " font-bold"}>{fmt.Sprintf("%.0f", m.scalar("process_open_fds"))}</span>
				</div>
			</div>

			<div class="flex gap-1 shrink-0">
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="font-dim">Files in Store</span>
					<span class="text-cyan font-bold">{fmt.Sprintf("%.0f", m.scalar("app_files_in_store"))}</span>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="font-dim">Files in Backup</span>
					<span class="text-cyan font-bold">{fmt.Sprintf("%.0f", m.scalar("app_files_in_backup"))}</span>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="font-dim">Cache Size</span>
					<span class="text-cyan font-bold">{fmt.Sprintf("%.0f", m.scalar("app_cache_size"))}</span>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="font-dim">Dumping cache in</span>
					<span class="text-yellow font-bold">{fmtCountdown(m.scalar("app_time_til_next_tick"))}</span>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="font-dim">Uptime</span>
					<span class="text-cyan font-bold">{fmt.Sprintf("%.1f hr", m.scalar("app_uptime_seconds")/3600)}</span>
				</div>
			</div>

			<div class="flex gap-1 shrink-0">
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="text-gradient-cyan-magenta font-bold">I/O Bytes by Operation</span>
					<div class="flex gap-1">
						<span class="font-dim">read:</span>
						<span class="text-cyan font-bold">{humanizeBytes(m.opBytes("read"))}</span>
					</div>
					<div class="flex gap-1">
						<span class="font-dim">write:</span>
						<span class="text-magenta font-bold">{humanizeBytes(m.opBytes("write"))}</span>
					</div>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="text-gradient-cyan-magenta font-bold">I/O Operation Counts</span>
					<div class="flex gap-1">
						<span class="font-dim">read ops:</span>
						<span class="text-green font-bold">{fmt.Sprintf("%.0f", m.opCount("read"))}</span>
						<span class="font-dim">write ops:</span>
						<span class="text-green font-bold">{fmt.Sprintf("%.0f", m.opCount("write"))}</span>
					</div>
				</div>
			</div>

			<div class="flex gap-1 flex-grow">
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="text-gradient-cyan-magenta font-bold">Activity</span>
					<div class="flex gap-1">
						<span class="font-dim">File retrievals:</span>
						<span class="text-cyan">{sparkline(m.retrievalHistory.Get())}</span>
					</div>
					<div class="flex gap-1">
						<span class="font-dim">Upload rejections:</span>
						<span class="text-magenta">{sparkline(m.rejectionHistory.Get())}</span>
					</div>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="text-gradient-cyan-magenta font-bold">Upload Activity</span>
					<div class="flex gap-1">
						<span class="font-dim">Accepted (.jpg):</span>
						<span class="text-green font-bold">{fmt.Sprintf("%.0f", m.uploadsByType(".jpg"))}</span>
					</div>
					<div class="flex gap-1">
						<span class="font-dim">Accepted (.pdf):</span>
						<span class="text-green font-bold">{fmt.Sprintf("%.0f", m.uploadsByType(".pdf"))}</span>
					</div>
					<div class="flex gap-1">
						<span class="font-dim">Rejected:</span>
						<span class="text-red font-bold">{fmt.Sprintf("%.0f", sumMap(m.snapshot.Get().labeled["app_upload_rejections_total"]))}</span>
					</div>
				</div>
			</div>

			<div class="flex gap-1 shrink-0">
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="font-dim">Barcode OK</span>
					<span class="text-green font-bold">{fmt.Sprintf("%.0f", m.scalar("app_barcode_successes"))}</span>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="font-dim">Barcode Fail</span>
					<span class="text-red font-bold">{fmt.Sprintf("%.0f", m.scalar("app_barcode_failures"))}</span>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="font-dim">Plugin Runs</span>
					<span class="text-cyan font-bold">{fmt.Sprintf("%.0f", m.scalar("app_plugin_runs"))}</span>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="font-dim">Plugin Errors</span>
					<span class="text-red font-bold">{fmt.Sprintf("%.0f", m.scalar("app_plugin_errors"))}</span>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="font-dim">DB Errors</span>
					<span class="text-red font-bold">{fmt.Sprintf("%.0f", m.scalar("app_db_errors"))}</span>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="font-dim">Upload Rejections</span>
					<span class="text-yellow font-bold">{fmt.Sprintf("%.0f", sumMap(m.snapshot.Get().labeled["app_upload_rejections_total"]))}</span>
				</div>
			</div>

			<div class="flex gap-1 shrink-0">
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="text-gradient-cyan-magenta font-bold">Network Receive</span>
					<span class="text-cyan font-bold">{humanizeBytes(m.scalar("process_network_receive_bytes_total"))}</span>
				</div>
				<div class="flex-col border-rounded p-1 gap-1" flexGrow={1.0}>
					<span class="text-gradient-cyan-magenta font-bold">Network Transmit</span>
					<span class="text-magenta font-bold">{humanizeBytes(m.scalar("process_network_transmit_bytes_total"))}</span>
				</div>
			</div>
		</div>

		<div class="flex justify-center shrink-0">
			<span class="font-dim">↑/↓ or j/k or mouse wheel to scroll — q to quit</span>
		</div>
	</div>
}
