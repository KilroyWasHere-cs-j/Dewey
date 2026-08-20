<script lang="ts">
	import { onMount } from 'svelte';
	import AppShell from '$lib/components/AppShell.svelte';
	import MetricGraph from '$lib/components/MetricGraph.svelte';
	import MetricBarChart from '$lib/components/MetricBarChart.svelte';
	import StatTile from '$lib/components/StatTile.svelte';
	import { settings, CHART_THEMES } from '$lib/stores/settings.svelte';
	import type { AppMetrics } from '$lib/types';

	let metrics = $state<Partial<AppMetrics>>({});
	let prometheusDown = $state(false);

	async function refreshMetrics() {
		try {
			const res = await fetch('/api/metrics');
			if (res.ok) {
				metrics = await res.json();
				prometheusDown = false;
			} else {
				prometheusDown = true;
			}
		} catch {
			prometheusDown = true;
		}
	}

	// Initial fetch on mount
	onMount(refreshMetrics);

	// Recreate the polling interval whenever pollIntervalMs changes in settings.
	// The effect cleanup (return fn) clears the old interval before starting a new one.
	$effect(() => {
		const id = setInterval(refreshMetrics, settings.value.pollIntervalMs);
		return () => clearInterval(id);
	});

	// Sum app_upload_rejections_total across every reason label (pe_blocked,
	// elf_blocked, content_mismatch, pdf_js_blocked, etc.) into one running
	// count for the sparkline — the per-reason breakdown isn't shown here.
	let uploadRejectionsTotal = $derived(
		Object.values((metrics.app_upload_rejections_total as Record<string, number>) ?? {}).reduce(
			(sum, v) => sum + v,
			0
		)
	);

	// Aggregate file_io_ops_total (3 labels: op, result, file_group) by op for a clean bar chart
	let ioOpsByType = $derived(
		Object.entries((metrics.file_io_ops_total as Record<string, number>) ?? {}).reduce(
			(acc, [key, val]) => {
				const op =
					key
						.split(',')
						.find((p) => p.startsWith('op='))
						?.split('=')[1] ?? key;
				acc[op] = (acc[op] ?? 0) + val;
				return acc;
			},
			{} as Record<string, number>
		)
	);

	// Convenience shortcuts into the current theme's color array
	let colors = $derived(CHART_THEMES[settings.value.chartColorTheme]);

	// Alert conditions derived from live metrics + user thresholds
	let ramAlert = $derived((metrics.app_ram_usage ?? 0) > settings.value.ramAlertThresholdMb);
	let retryAlert = $derived((metrics.app_file_retries ?? 0) > settings.value.retryAlertThreshold);

	function round2(v: number) {
		return Math.round(v * 100) / 100;
	}
</script>

<!-- Alert banners — stacked via flex column + gap so the browser handles spacing
     instead of hand-computed margin offsets (issue #242). -->
<div class="fixed top-4 left-1/2 z-50 flex -translate-x-1/2 flex-col items-center gap-2">
	<!-- Prometheus unreachable banner (toggled in settings) -->
	{#if prometheusDown && settings.value.showPrometheusAlert}
		<div
			role="alert"
			class="flex items-center gap-3 rounded-lg border border-red-500/40 bg-red-950/90 px-5 py-3 text-sm text-red-300 shadow-lg backdrop-blur"
		>
			<svg
				aria-hidden="true"
				class="h-4 w-4 shrink-0 text-red-400"
				fill="none"
				viewBox="0 0 24 24"
				stroke="currentColor"
				stroke-width="2"
			>
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					d="M12 9v3m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"
				/>
			</svg>
			<span>Prometheus is unreachable — metrics may be stale or unavailable</span>
		</div>
	{/if}

	<!-- RAM alert -->
	{#if ramAlert}
		<div
			role="alert"
			class="flex items-center gap-3 rounded-lg border border-amber-500/40 bg-amber-950/90 px-5 py-3 text-sm text-amber-300 shadow-lg backdrop-blur"
		>
			<span
				>⚠ RAM usage ({metrics.app_ram_usage} MB) exceeds threshold ({settings.value
					.ramAlertThresholdMb} MB)</span
			>
		</div>
	{/if}

	<!-- Retry alert -->
	{#if retryAlert}
		<div
			role="alert"
			class="flex items-center gap-3 rounded-lg border border-red-500/40 bg-red-950/90 px-5 py-3 text-sm text-red-300 shadow-lg backdrop-blur"
		>
			<span
				>⚠ File retries ({metrics.app_file_retries}) exceeds threshold ({settings.value
					.retryAlertThreshold})</span
			>
		</div>
	{/if}
</div>

<svelte:head>
	<title>Analytics — Dewey</title>
</svelte:head>

<AppShell spacious="space-y-8 p-6">
	{#snippet children({ headingClass })}
		<!-- System Resources -->
		<section>
			<h2 class="mb-3 text-xs font-semibold tracking-wider uppercase {headingClass}">
				System Resources
			</h2>
			<div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
				<MetricGraph
					label="RAM Usage"
					value={metrics.app_ram_usage ?? 0}
					unit="MB"
					color={colors[0]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="Heap Usage"
					value={metrics.app_heap_usage ?? 0}
					unit="MB"
					color={colors[1]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="Goroutines"
					value={metrics.go_goroutines ?? 0}
					color={colors[2]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="Open File Descriptors"
					value={metrics.process_open_fds ?? 0}
					color={colors[3]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
			</div>
		</section>

		<!-- Application State -->
		<section>
			<h2 class="mb-3 text-xs font-semibold tracking-wider uppercase {headingClass}">
				Application
			</h2>
			<div class="grid grid-cols-2 gap-4 lg:grid-cols-4 xl:grid-cols-5">
				<MetricGraph
					label="Files in Store"
					value={metrics.app_files_in_store ?? 0}
					color={colors[4]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="Files in Backup"
					value={metrics.app_files_in_backup ?? 0}
					color={colors[5]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="Cache Size"
					value={metrics.app_cache_size ?? 0}
					color={colors[6]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<!-- Plain stat tiles, not sparkline graphs — a countdown and an
				     ever-increasing uptime counter don't have a meaningful trend
				     to plot (issue #279). -->
				<StatTile
					label="Next Tick"
					value={round2((metrics.app_time_til_next_tick ?? 0) / 60)}
					unit="min"
				/>
				<StatTile
					label="Uptime"
					value={round2((metrics.app_uptime_seconds ?? 0) / 3600)}
					unit="hr"
				/>
			</div>
		</section>

		<!-- File I/O -->
		<section>
			<h2 class="mb-3 text-xs font-semibold tracking-wider uppercase {headingClass}">File I/O</h2>
			<div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
				<!-- file_io_bytes_total has a single "op" label, so keys are just "read" / "write" -->
				<MetricBarChart
					label="I/O Bytes by Operation"
					data={(metrics.file_io_bytes_total as Record<string, number>) ?? {}}
					unit="bytes"
					colors={colors.slice(0, 4)}
				/>
				<!-- Aggregated from the 3-label ops metric by summing across result + file_group -->
				<MetricBarChart
					label="I/O Operations by Type"
					data={ioOpsByType}
					colors={colors.slice(4, 8)}
				/>
			</div>
		</section>

		<!-- Activity Counters — includes new deletions and filter loads from #104 -->
		<section>
			<h2 class="mb-3 text-xs font-semibold tracking-wider uppercase {headingClass}">Activity</h2>
			<div class="grid grid-cols-2 gap-4 lg:grid-cols-3 xl:grid-cols-6">
				<MetricGraph
					label="File Copies"
					value={metrics.app_file_copys ?? 0}
					color={colors[8]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="File Retries"
					value={metrics.app_file_retries ?? 0}
					color={colors[9]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="File Sorts"
					value={metrics.app_file_sorts ?? 0}
					color={colors[10]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="File Deletions"
					value={metrics.app_file_deletions ?? 0}
					color="#ef4444"
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="Filter Loads"
					value={metrics.app_filters_loadings ?? 0}
					color={colors[11]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="GC Cycles"
					value={metrics.app_gc_cycles ?? 0}
					color={colors[0]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
			</div>
		</section>

		<!-- Upload Activity — new in #104 -->
		<section>
			<h2 class="mb-3 text-xs font-semibold tracking-wider uppercase {headingClass}">
				Upload Activity
			</h2>
			<div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
				<MetricBarChart
					label="Rejections by Reason"
					data={(metrics.app_upload_rejections_total as Record<string, number>) ?? {}}
					colors={['#f97316', '#ef4444', '#dc2626']}
				/>
				<MetricBarChart
					label="Accepted Uploads by File Type"
					data={(metrics.app_uploads_by_type_total as Record<string, number>) ?? {}}
					colors={colors.slice(0, 8)}
				/>
			</div>
		</section>

		<!-- Processing Pipeline — new in #104 -->
		<section>
			<h2 class="mb-3 text-xs font-semibold tracking-wider uppercase {headingClass}">
				Processing Pipeline
			</h2>
			<div class="grid grid-cols-2 gap-4 lg:grid-cols-5 xl:grid-cols-6">
				<MetricGraph
					label="Barcode Successes"
					value={metrics.app_barcode_successes ?? 0}
					color="#22c55e"
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="Barcode Failures"
					value={metrics.app_barcode_failures ?? 0}
					color="#ef4444"
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="Plugin Runs"
					value={metrics.app_plugin_runs ?? 0}
					color={colors[2]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="Plugin Errors"
					value={metrics.app_plugin_errors ?? 0}
					color="#f97316"
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="DB Errors"
					value={metrics.app_db_errors ?? 0}
					color="#dc2626"
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="Upload Rejections"
					value={uploadRejectionsTotal}
					color="#a855f7"
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
			</div>
		</section>

		<!-- Network -->
		<section>
			<h2 class="mb-3 text-xs font-semibold tracking-wider uppercase {headingClass}">Network</h2>
			<div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
				<MetricGraph
					label="Network Receive"
					value={round2((metrics.process_network_receive_bytes_total ?? 0) / 1024 / 1024)}
					unit="MB"
					color={colors[12]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
				<MetricGraph
					label="Network Transmit"
					value={round2((metrics.process_network_transmit_bytes_total ?? 0) / 1024 / 1024)}
					unit="MB"
					color={colors[0]}
					maxHistory={settings.value.analyticsHistoryWindow}
				/>
			</div>
		</section>
	{/snippet}
</AppShell>
