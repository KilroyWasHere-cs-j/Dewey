<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Topbar from '$lib/components/Topbar.svelte';

	let sidebarOpen = $state(true);

	const toggleSidebar = () => {
		sidebarOpen = !sidebarOpen;
	};

	import { onMount } from 'svelte';
	import type { AppMetrics } from '../api/metrics/+server';

	let metrics: Partial<AppMetrics> = {};
	let loading = true;

	async function refreshMetrics() {
		const res = await fetch('/api/metrics');
		metrics = await res.json();
		loading = false;
	}

	onMount(() => {
		refreshMetrics();
		// Refresh every 5 seconds
		const interval = setInterval(refreshMetrics, 5000);
		return () => clearInterval(interval);
	});
</script>

<div class="flex min-h-screen bg-slate-100">
	<!-- Sidebar -->
	<Sidebar open={sidebarOpen} toggle={toggleSidebar} />

	<!-- Main Area -->
	<div class="flex flex-1 flex-col">
		<!-- Topbar -->
		<Topbar {sidebarOpen} {toggleSidebar} />

		<!-- Content -->
		<main class="p-6">
			<!-- Analytics Grid -->
			<div class="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
				<!-- Stat Cards -->
				<div class="rounded-2xl bg-white p-4 shadow-sm">
					<h2 class="mb-2 text-lg font-semibold">Users</h2>
					<p class="text-sm text-gray-600">1,284 active users</p>
				</div>

				<div class="rounded-2xl bg-white p-4 shadow-sm">
					<h2 class="mb-2 text-lg font-semibold">Sessions</h2>
					<p class="text-sm text-gray-600">3,912 today</p>
				</div>

				<div class="rounded-2xl bg-white p-4 shadow-sm">
					<h2 class="mb-2 text-lg font-semibold">Errors</h2>
					<p class="text-sm text-gray-600">12 logged</p>
				</div>

				<div class="rounded-2xl bg-white p-4 shadow-sm">
					<h2 class="mb-2 text-lg font-semibold">Latency</h2>
					<p class="text-sm text-gray-600">142ms avg</p>
				</div>

				<!-- Wide Analytics Panel -->
				<div
					class="col-span-1 rounded-2xl bg-white p-4 shadow-sm sm:col-span-2 lg:col-span-3 xl:col-span-4"
				>
					<h2 class="mb-2 text-lg font-semibold">System Overview</h2>
					<p class="mb-4 text-sm text-gray-600">
						This will later hold charts (Prometheus, logs, etc.)
					</p>

					<div class="flex h-48 items-center justify-center rounded-xl bg-gray-100 text-gray-400">
						Chart Placeholder
					</div>
				</div>
			</div>
		</main>
	</div>
</div>
