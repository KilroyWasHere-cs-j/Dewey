<script lang="ts">
	import { Avatar } from 'flowbite-svelte';
	import { BugOutline, MoonSolid, SunSolid } from 'flowbite-svelte-icons';
	import { settings, ACCENT } from '$lib/stores/settings.svelte';

	let accentHex = $derived(ACCENT[settings.value.accentColor].hex);
	let dark = $derived(settings.value.darkMode);

	let { sidebarOpen, toggleSidebar } = $props<{
		sidebarOpen: boolean;
		toggleSidebar: () => void;
	}>();
</script>

<header
	class="flex items-center justify-between bg-white px-6 py-4 shadow dark:bg-gray-800"
	style="border-bottom: 3px solid {accentHex}"
>
	<div class="flex items-center gap-4">
		<button class="text-gray-700 dark:text-gray-300 md:hidden" onclick={toggleSidebar}>
			☰
		</button>
		<h1 class="text-xl font-semibold text-gray-800 dark:text-white">{settings.value.portalName}</h1>
	</div>

	<div class="flex items-center gap-3">
		<!-- Dark mode toggle -->
		<button
			aria-label="Toggle dark mode"
			class="rounded-lg p-2 text-gray-500 transition-colors hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700"
			onclick={() => settings.update({ darkMode: !dark })}
		>
			{#if dark}
				<SunSolid class="h-4 w-4" />
			{:else}
				<MoonSolid class="h-4 w-4" />
			{/if}
		</button>

		<span class="text-sm text-gray-600 dark:text-gray-300">User</span>
		<Avatar>
			<BugOutline />
		</Avatar>
	</div>
</header>
