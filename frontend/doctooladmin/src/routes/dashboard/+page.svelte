<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Topbar from '$lib/components/Topbar.svelte';
	import { Tooltip } from 'flowbite-svelte';

	import { onMount } from 'svelte';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	let metrics = $state(data.metrics);
	let prometheusDown = $state(data.prometheusError ?? false);

	const POLL_INTERVAL_MS = 1500;

	onMount(() => {
		const interval = setInterval(async () => {
			try {
				const res = await fetch('/api/metrics');
				if (res.ok) {
					metrics = await res.json();
					prometheusDown = false; // Recovered — clear the alert
				} else {
					prometheusDown = true;
				}
			} catch (e) {
				console.error('Failed to poll metrics:', e);
				prometheusDown = true;
			}
		}, POLL_INTERVAL_MS);

		return () => clearInterval(interval);
	});
	// Page data
	let sidebarOpen = $state(true);

	const statusColorMap = {
		Good: 'text-green-500 bg-green-600',
		Slow: 'text-yellow-500 bg-yellow-400',
		Warning: 'text-red-500 bg-orange-400',
		Critical: 'text-red-500 bg-red-500',
		Dead: 'text-gray-500 bg-slate-950',
		Unknown: 'text-gray-500 bg-gray-100'
	} as const;

	const SystemHealth = {
		Good: 'Good',
		Slow: 'Slow',
		Warning: 'Warning',
		Critical: 'Critical',
		Dead: 'Dead',
		Uknown: 'Unknown'
	} as const;

	type SystemHealth = (typeof SystemHealth)[keyof typeof SystemHealth];

	let statusSystemHealth: SystemHealth = SystemHealth.Good;
	let statusErrorsThrow = 0;
	let statusFileTypesRejected = 0;

	const toggleSidebar = () => {
		sidebarOpen = !sidebarOpen;
	};
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

<div class="flex min-h-screen bg-gray-100">
	<!-- Sidebar Component -->
	<Sidebar open={sidebarOpen} toggle={toggleSidebar} />

	<!-- Main Content -->
	<div class="flex flex-1 flex-col">
		<Topbar {sidebarOpen} {toggleSidebar} />

		<!-- Content Grid -->
		<main class="p-6">
			<div class="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
				<!-- Card -->
				<div class="${statusColorMap[statusSystemHealth]} rounded-2xl p-4 shadow-md">
					<h2 class="mb-2 text-lg font-semibold text-zinc-700">System Health</h2>
					<p class="text-sm text-zinc-600">
						{statusSystemHealth}
					</p>
				</div>
				<Tooltip
					>System health status. Normal day to day should be indicated as "Good". Periodic "Slow" is
					acceptable. Anyother status indicates a problem.</Tooltip
				>

				<div class="rounded-2xl bg-yellow-400 p-4 shadow-md">
					<h2 class="mb-2 text-lg font-semibold text-zinc-700">System Uptime</h2>
					<p class="text-sm text-zinc-600">{metrics.app_uptime_seconds / 60} minutes</p>
				</div>
				<Tooltip>System uptime in hours, minutes, and seconds</Tooltip>

				<div class="rounded-2xl bg-yellow-400 p-4 shadow-md">
					<h2 class="mb-2 text-lg font-semibold text-zinc-700">Total Stored Files</h2>
					<p class="text-sm text-zinc-600">{metrics.app_files_in_store} files</p>
				</div>
				<Tooltip
					>Total number of files stored in the system. Useful for monitoring storage usage.</Tooltip
				>

				<div class="rounded-2xl bg-yellow-400 p-4 shadow-md">
					<h2 class="mb-2 text-lg font-semibold text-zinc-700">Cache Count</h2>
					<p class="text-sm text-zinc-600">{metrics.app_cache_size} files</p>
				</div>
				<Tooltip
					>Total number of files currently in the cache. Every set interval, the cache is cleared.</Tooltip
				>

				<div class="rounded-2xl bg-yellow-400 p-4 shadow-md">
					<h2 class="mb-2 text-lg font-semibold text-zinc-700">Time till tick</h2>
					<p class="text-sm text-zinc-600">{metrics.app_time_til_next_tick / 60} minutes</p>
				</div>
				<Tooltip
					>Time remaining until the next system tick. Ticks trigger system selfcare routines. Such
					as cache dumps, backups, etc...</Tooltip
				>

				<div class="rounded-2xl bg-yellow-400 p-4 shadow-md">
					<h2 class="mb-2 text-lg font-semibold text-zinc-700">Errors Throw</h2>
					<p class="text-sm text-zinc-600">{statusErrorsThrow}</p>
				</div>
				<Tooltip
					>Total number of errors thrown by the system. This should be zero. Other numbers indicate
					issues. Please check logs for specific details.</Tooltip
				>

				<div class="rounded-2xl bg-yellow-400 p-4 shadow-md">
					<h2 class="mb-2 text-lg font-semibold text-zinc-700">Files rejected</h2>
					<p class="text-sm text-zinc-600">{statusFileTypesRejected}</p>
				</div>
				<Tooltip>Total number of file types rejected.</Tooltip>

				<div class="rounded-2xl bg-yellow-400 p-4 shadow-md">
					<h2 class="mb-2 text-lg font-semibold text-zinc-700">Executable Files Blocked</h2>
					<p class="text-sm text-zinc-600">{metrics.app_exe_count} files</p>
				</div>
				<Tooltip
					>Total number of executable files blocked. Each insidendent of a executable file being
					uploaded needs to be reviewed.</Tooltip
				>

				<!-- Wide Card -->
				<div class="col-span-1 rounded-2xl bg-white p-4 shadow-sm sm:col-span-2 lg:col-span-4">
					<h2 class="mb-2 text-lg font-semibold text-zinc-700">Wide Section</h2>
					<p class="text-sm text-zinc-600">Useful for charts, tables, logs, etc.</p>
				</div>

				<!-- Wide Card -->
				<div class="col-span-1 rounded-2xl bg-white p-4 shadow-sm sm:col-span-2 lg:col-span-4">
					<h2 class="mb-2 text-lg font-semibold text-zinc-700">Wide Section</h2>
					<p class="text-sm text-zinc-600">Useful for charts, tables, logs, etc.</p>
				</div>
			</div>
		</main>
	</div>
</div>
