<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Topbar from '$lib/components/Topbar.svelte';

	let sidebarOpen = true;

	const toggleSidebar = () => {
		sidebarOpen = !sidebarOpen;
	};
</script>

<div class="min-h-screen bg-slate-100 flex">

	<!-- Sidebar -->
	<Sidebar open={sidebarOpen} toggle={toggleSidebar} />

	<!-- Main Area -->
	<div class="flex-1 flex flex-col">

		<!-- Topbar -->
		<Topbar {sidebarOpen} {toggleSidebar} />

		<!-- Content -->
		<main class="p-6">

			<!-- Analytics Grid -->
			<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">

				<!-- Stat Cards -->
				<div class="bg-white p-4 rounded-2xl shadow-sm">
					<h2 class="text-lg font-semibold mb-2">Users</h2>
					<p class="text-gray-600 text-sm">1,284 active users</p>
				</div>

				<div class="bg-white p-4 rounded-2xl shadow-sm">
					<h2 class="text-lg font-semibold mb-2">Sessions</h2>
					<p class="text-gray-600 text-sm">3,912 today</p>
				</div>

				<div class="bg-white p-4 rounded-2xl shadow-sm">
					<h2 class="text-lg font-semibold mb-2">Errors</h2>
					<p class="text-gray-600 text-sm">12 logged</p>
				</div>

				<div class="bg-white p-4 rounded-2xl shadow-sm">
					<h2 class="text-lg font-semibold mb-2">Latency</h2>
					<p class="text-gray-600 text-sm">142ms avg</p>
				</div>

				<!-- Wide Analytics Panel -->
				<div class="bg-white p-4 rounded-2xl shadow-sm col-span-1 sm:col-span-2 lg:col-span-3 xl:col-span-4">
					<h2 class="text-lg font-semibold mb-2">System Overview</h2>
					<p class="text-gray-600 text-sm mb-4">
						This will later hold charts (Prometheus, logs, etc.)
					</p>

					<div class="h-48 bg-gray-100 rounded-xl flex items-center justify-center text-gray-400">
						Chart Placeholder
					</div>
				</div>

			</div>

		</main>
	</div>
</div>