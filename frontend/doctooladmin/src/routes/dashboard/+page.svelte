<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Topbar from '$lib/components/Topbar.svelte';
	import { Button, Tooltip, Spinner } from "flowbite-svelte";

	// Page data
	let sidebarOpen = $state(true);
	let pageRefreshing = $state(false);

	function handleRefresh() {
		pageRefreshing = true;
		// Simulate data fetching
		setTimeout(() => {
			pageRefreshing = false;
			// Update status variables with new data here
		}, 2000);
	}

	const statusColorMap = {
		Good: "text-green-500 bg-green-600",
		Slow: "text-yellow-500 bg-yellow-400",
		Warning: "text-red-500 bg-orange-400",
		Critical: "text-red-500 bg-red-500",
		Dead: "text-gray-500 bg-slate-950",
		Unknown: "text-gray-500 bg-gray-100"

	} as const;
	
	const SystemHealth = {
		Good: 'Good',
		Slow: 'Slow',
		Warning: 'Warning',
		Critical: 'Critical',
		Dead: 'Dead',
		Uknown: 'Unknown'
	} as const;

	type SystemHealth = typeof SystemHealth[keyof typeof SystemHealth];

	let statusSystemHealth: SystemHealth = SystemHealth.Good;
	let statusUpTime = '00:00:00';
	let statusTotalStoredFiles = 0;
	let statusErrorsThrow = 0;
	let statusCacheCount = 0;
	let statusTimeTillTick = '00:00:00';

	const toggleSidebar = () => {
		sidebarOpen = !sidebarOpen;
	};
</script>

<div class="min-h-screen bg-gray-100 flex">
    <!-- Sidebar Component -->
	<Sidebar open={sidebarOpen} toggle={toggleSidebar} />

	<!-- Main Content -->
	<div class="flex-1 flex flex-col">

		<Topbar {sidebarOpen} {toggleSidebar} />

		<div class="bg-white shadow-sm m-4 rounded-lg">
			<div class="max-w-7xl mx-auto py-4 px-6">
				<Button color="purple" onclick={handleRefresh} disabled={pageRefreshing}>
					{pageRefreshing ? "Refreshing..." : "Refresh"}
				</Button>
				<Tooltip>Refresh dashboard</Tooltip>
				{#if pageRefreshing}
					<Spinner type="orbit" color="rose" />
				{/if}
			</div>
		</div>

		<!-- Content Grid -->
		<main class="p-6">
			<div
				class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6"
			>
				<!-- Card -->
				<div class="${statusColorMap[statusSystemHealth]} p-4 rounded-2xl shadow-md">
					<h2 class="text-lg text-zinc-700 font-semibold mb-2">System Health</h2>
					<p class="text-zinc-600 text-sm">
						{statusSystemHealth}
					</p>
				</div>
				<Tooltip>System health status</Tooltip>

				<div class="bg-yellow-400 p-4 rounded-2xl shadow-md">
					<h2 class="text-lg text-zinc-700 font-semibold mb-2">System Uptime</h2>
					<p class="text-zinc-600 text-sm">{statusUpTime} h:m:s</p>
				</div>
				<Tooltip>System uptime in hours, minutes, and seconds</Tooltip>

				<div class="bg-yellow-400 p-4 rounded-2xl shadow-md">
					<h2 class="text-lg text-zinc-700 font-semibold mb-2">Total Stored Files</h2>
					<p class="text-zinc-600 text-sm">{statusTotalStoredFiles}</p>
				</div>
				<Tooltip>Total number of files stored in the system</Tooltip>

				<div class="bg-yellow-400 p-4 rounded-2xl shadow-md">
					<h2 class="text-lg text-zinc-700 font-semibold mb-2">Cache Count</h2>
					<p class="text-zinc-600 text-sm">{statusCacheCount}</p>
				</div>
				<Tooltip>Total number of files currently in the cache</Tooltip>

				<div class="bg-yellow-400 p-4 rounded-2xl shadow-md">
					<h2 class="text-lg text-zinc-700 font-semibold mb-2">Time till tick</h2>
					<p class="text-zinc-600 text-sm">{statusTimeTillTick}</p>
				</div>
				<Tooltip>Time remaining until the next system tick</Tooltip>

				<div class="bg-yellow-400 p-4 rounded-2xl shadow-md">
					<h2 class="text-lg text-zinc-700 font-semibold mb-2">Errors Throw</h2>
					<p class="text-zinc-600 text-sm">{statusErrorsThrow}</p>
				</div>
				<Tooltip>Total number of errors thrown by the system</Tooltip>

				<div class="bg-yellow-400 p-4 rounded-2xl shadow-md">
					<h2 class="text-lg text-zinc-700 font-semibold mb-2">Errors Throw</h2>
					<p class="text-zinc-600 text-sm">{statusErrorsThrow}</p>
				</div>
				<Tooltip>Total number of errors thrown by the system</Tooltip>

				<div class="bg-yellow-400 p-4 rounded-2xl shadow-md">
					<h2 class="text-lg text-zinc-700 font-semibold mb-2">Errors Throw</h2>
					<p class="text-zinc-600 text-sm">{statusErrorsThrow}</p>
				</div>
				<Tooltip>Total number of errors thrown by the system</Tooltip>

				<!-- Wide Card -->
				<div class="bg-white p-4 rounded-2xl shadow-sm col-span-1 sm:col-span-2 lg:col-span-3">
					<h2 class="text-lg text-zinc-700 font-semibold mb-2">Wide Section</h2>
					<p class="text-zinc-600 text-sm">
						Useful for charts, tables, logs, etc.
					</p>
				</div>

				<!-- Wide Card -->
				<div class="bg-white p-4 rounded-2xl shadow-sm col-span-1 sm:col-span-2 lg:col-span-3">
					<h2 class="text-lg text-zinc-700 font-semibold mb-2">Wide Section</h2>
					<p class="text-zinc-600 text-sm">
						Useful for charts, tables, logs, etc.
					</p>
				</div>
			</div>
		</main>
	</div>
</div>