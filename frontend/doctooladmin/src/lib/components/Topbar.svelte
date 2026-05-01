<script lang="ts">
	let { sidebarOpen, toggleSidebar } = $props<{
		sidebarOpen: boolean;
		toggleSidebar: () => void;
	}>();
</script>

<header class="bg-white shadow px-6 py-4 flex items-center justify-between">

	<div class="flex items-center gap-4">
		<button
			class="md:hidden text-gray-700"
			on:click={toggleSidebar}
		>
			☰
		</button>

		<h1 class="text-xl font-semibold">Analytics</h1>
	</div>

	<div class="flex items-center gap-4">
		<span class="text-sm text-gray-600">User</span>
		<div class="w-8 h-8 bg-gray-300 rounded-full"></div>
	</div>

</header>