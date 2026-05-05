<script lang="ts">
  import { Avatar } from "flowbite-svelte";
  import { BugOutline } from "flowbite-svelte-icons";

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
		<h1 class="text-xl font-semibold text-gray-800">Dewey Dashboard</h1>
	</div>

	<div class="flex items-center gap-4">
		<span class="text-sm text-gray-600">User</span>
		<Avatar>
			<BugOutline />
		</Avatar>
	</div>

</header>
