<script lang="ts">
  import { Avatar } from "flowbite-svelte";
  import { BugOutline } from "flowbite-svelte-icons";
  import { settings, ACCENT } from '$lib/stores/settings.svelte';

  let accentHex = $derived(ACCENT[settings.value.accentColor].hex);

	let { sidebarOpen, toggleSidebar } = $props<{
		sidebarOpen: boolean;
		toggleSidebar: () => void;
	}>();
</script>

<header class="bg-white shadow px-6 py-4 flex items-center justify-between" style="border-bottom: 3px solid {accentHex}">

	<div class="flex items-center gap-4">
		<button
			class="md:hidden text-gray-700"
			on:click={toggleSidebar}
		>
			☰
		</button>
		<h1 class="text-xl font-semibold text-gray-800">{settings.value.portalName}</h1>
	</div>

	<div class="flex items-center gap-4">
		<span class="text-sm text-gray-600">User</span>
		<Avatar>
			<BugOutline />
		</Avatar>
	</div>

</header>
