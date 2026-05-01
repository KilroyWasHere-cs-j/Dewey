<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Topbar from '$lib/components/Topbar.svelte';
	import { Button, Tooltip, Spinner } from "flowbite-svelte";

	/// Page data
	let sidebarOpen = $state(true);
	let pageRefreshing = $state(false);

	/// Page functions
	function handleRefresh() {
		pageRefreshing = true;
		statusSystemLog = '';
		statusSystemLog = "Loading logs...";
		
		// Simulate data fetching
		setTimeout(() => {
			pageRefreshing = false;
			// Update status variables with new data here
		}, 2000);
	}

	const toggleSidebar = () => {
		sidebarOpen = !sidebarOpen;
	};

	/// Status variables
	let statusSystemLog = $state('No logs available.');
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
			<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
				<!-- Wide Card -->
				<div class="bg-white p-4 rounded-2xl shadow-sm col-span-1 sm:col-span-2 lg:col-span-3">
					<h2 class="text-lg text-zinc-700 font-semibold mb-2">Log</h2>
					<div class="bg-zinc-950 rounded-lg p-4 text-sm text-green-400 h-64 overflow-y-auto">
						{statusSystemLog ?? (pageRefreshing ? "Loading logs..." : "No logs available.")}
					</div>
				</div>
			</div>
		</main>
	</div>
</div>



