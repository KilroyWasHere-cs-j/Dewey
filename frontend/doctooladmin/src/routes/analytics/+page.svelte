<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Topbar from '$lib/components/Topbar.svelte';
	import MetricGraph from '$lib/components/MetricGraph.svelte';
	import MetricBarChart from '$lib/components/MetricBarChart.svelte';

	import { onMount } from 'svelte';
	import type { AppMetrics } from '../api/metrics/+server';

	let sidebarOpen = $state(true);
	let metrics: Partial<AppMetrics> = $state({});
	let loading = $state(true);
	let prometheusDown = $state(false);

	const toggleSidebar = () => {
		sidebarOpen = !sidebarOpen;
	};

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
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		refreshMetrics();
		const interval = setInterval(refreshMetrics, 5000);
		return () => clearInterval(interval);
	});
</script>

{#if prometheusDown}
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

<div class="flex min-h-screen bg-slate-100">
	<Sidebar open={sidebarOpen} toggle={toggleSidebar} />

	<div class="flex flex-1 flex-col">
		<Topbar {sidebarOpen} {toggleSidebar} />

		<main class="p-6 space-y-8">
			{#if loading}
				<div class="flex items-center justify-center py-24 text-sm text-gray-400">
					Loading metrics…
				</div>
			{:else}
				<!-- ─── Activity ───────────────────────────────────────────────── -->
				<section>
					<h2 class="mb-3 text-base font-semibold text-gray-700">Activity</h2>
					<div class="grid grid-cols-2 gap-4 sm:grid-cols-4 xl:grid-cols-5">
						<MetricGraph label="File Sorts" value={metrics.app_file_sorts} />
						<MetricGraph label="File Copies" value={metrics.app_file_copys} />
						<MetricGraph label="File Retrievals" value={metrics.app_file_retries} />
						<MetricGraph label="File Deletions" value={metrics.app_file_deletions} color="#ef4444" />
						<MetricGraph label="Filter Loads" value={metrics.app_filters_loadings} color="#8b5cf6" />
					</div>
				</section>

				<!-- ─── Upload Activity ────────────────────────────────────────── -->
				<section>
					<h2 class="mb-3 text-base font-semibold text-gray-700">Upload Activity</h2>
					<div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
						<MetricBarChart
							title="Rejections by Reason"
							data={metrics.app_upload_rejections_total}
							color="#f97316"
						/>
						<MetricBarChart
							title="Accepted Uploads by File Type"
							data={metrics.app_uploads_by_type_total}
							color="#22c55e"
						/>
					</div>
				</section>

				<!-- ─── Processing Pipeline ───────────────────────────────────── -->
				<section>
					<h2 class="mb-3 text-base font-semibold text-gray-700">Processing Pipeline</h2>
					<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 xl:grid-cols-5">
						<MetricGraph
							label="Barcode Successes"
							value={metrics.app_barcode_successes}
							color="#22c55e"
						/>
						<MetricGraph
							label="Barcode Failures"
							value={metrics.app_barcode_failures}
							color="#ef4444"
						/>
						<MetricGraph label="Plugin Runs" value={metrics.app_plugin_runs} color="#3b82f6" />
						<MetricGraph
							label="Plugin Errors"
							value={metrics.app_plugin_errors}
							color="#f97316"
						/>
						<MetricGraph label="DB Errors" value={metrics.app_db_errors} color="#dc2626" />
					</div>
				</section>
			{/if}
		</main>
	</div>
</div>
