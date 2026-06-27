<script lang="ts">
	import { onMount } from 'svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Topbar from '$lib/components/Topbar.svelte';
	import MetricGraph from '$lib/components/MetricGraph.svelte';
	import MetricBarChart from '$lib/components/MetricBarChart.svelte';
	import { settings, CHART_THEMES, ACCENT } from '$lib/stores/settings.svelte';
	import type { AppMetrics } from '../api/metrics/+server';

	// Read sidebar default from settings on first load
	let sidebarOpen = $state(settings.value.defaultSidebarOpen);
	const toggleSidebar = () => {
		sidebarOpen = !sidebarOpen;
	};

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
	let ramAlert = $derived(
		(metrics.app_ram_usage ?? 0) > settings.value.ramAlertThresholdMb
	);
	let retryAlert = $derived(
		(metrics.app_file_retries ?? 0) > settings.value.retryAlertThreshold
	);

	// Padding and gap change with layout density
	let mainClass = $derived(
		settings.value.layoutDensity === 'compact' ? 'space-y-4 p-4' : 'space-y-8 p-6'
	);

	// Section heading color tracks the accent setting
	let headingClass = $derived(ACCENT[settings.value.accentColor].text);

	function round2(v: number) {
		return Math.round(v * 100) / 100;
	}
</script>

<!-- Prometheus unreachable banner (toggled in settings) -->
{#if prometheusDown && settings.value.showPrometheusAlert}
	<div
		class="fixed top-4 left-1/2 z-50 flex -translate-x-1/2 items-center gap-3 rounded-lg border border-red-500/40 bg-red-950/90 px-5 py-3 text-sm text-red-300 shadow-lg backdrop-blur"
	>
		<svg
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
		class="fixed top-4 left-1/2 z-50 flex -translate-x-1/2 items-center gap-3 rounded-lg border border-amber-500/40 bg-amber-950/90 px-5 py-3 text-sm text-amber-300 shadow-lg backdrop-blur"
		style="margin-top: {prometheusDown && settings.value.showPrometheusAlert ? '3.5rem' : '0'}"
	>
		<span>⚠ RAM usage ({metrics.app_ram_usage} MB) exceeds threshold ({settings.value.ramAlertThresholdMb} MB)</span>
	</div>
{/if}

<!-- Retry alert -->
{#if retryAlert}
	<div
		class="fixed top-4 left-1/2 z-50 flex -translate-x-1/2 items-center gap-3 rounded-lg border border-red-500/40 bg-red-950/90 px-5 py-3 text-sm text-red-300 shadow-lg backdrop-blur"
		style="margin-top: {(prometheusDown && settings.value.showPrometheusAlert ? 3.5 : 0) + (ramAlert ? 3.5 : 0)}rem"
	>
		<span>⚠ File retries ({metrics.app_file_retries}) exceeds threshold ({settings.value.retryAlertThreshold})</span>
	</div>
{/if}

<div class="flex min-h-screen bg-gray-100">
	<Sidebar open={sidebarOpen} toggle={toggleSidebar} />

	<div class="flex flex-1 flex-col">
		<Topbar {sidebarOpen} {toggleSidebar} />

		<main class={mainClass}>
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
				<div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
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
					<MetricGraph
						label="Next Tick"
						value={round2((metrics.app_time_til_next_tick ?? 0) / 60)}
						unit="min"
						color={colors[7]}
						maxHistory={settings.value.analyticsHistoryWindow}
					/>
				</div>
			</section>

			<!-- File I/O -->
			<section>
				<h2 class="mb-3 text-xs font-semibold tracking-wider uppercase {headingClass}">
					File I/O
				</h2>
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

			<!-- Activity Counters -->
			<section>
				<h2 class="mb-3 text-xs font-semibold tracking-wider uppercase {headingClass}">
					Activity
				</h2>
				<div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
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
						label="GC Cycles"
						value={metrics.app_gc_cycles ?? 0}
						color={colors[11]}
						maxHistory={settings.value.analyticsHistoryWindow}
					/>
				</div>
			</section>

			<!-- Network -->
			<section>
				<h2 class="mb-3 text-xs font-semibold tracking-wider uppercase {headingClass}">
					Network
				</h2>
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
		</main>
	</div>
</div>
