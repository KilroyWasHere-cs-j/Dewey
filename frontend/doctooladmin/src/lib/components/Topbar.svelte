<script lang="ts">
	import { onMount } from 'svelte';
	import { Avatar } from 'flowbite-svelte';
	import { BugOutline, MoonSolid, SunSolid } from 'flowbite-svelte-icons';
	import { settings, ACCENT } from '$lib/stores/settings.svelte';
	import type { AppVersionInfo } from '../../routes/api/version/+server';

	let accentHex = $derived(ACCENT[settings.value.accentColor].hex);
	let dark = $derived(settings.value.darkMode);

	let { sidebarOpen, toggleSidebar } = $props<{
		sidebarOpen: boolean;
		toggleSidebar: () => void;
	}>();

	// Build/deploy info (issue #66) — fetched once on mount since it's static
	// for the lifetime of the running backend, no need to poll it.
	let versionInfo = $state<AppVersionInfo | null>(null);

	onMount(async () => {
		try {
			const res = await fetch('/api/version');
			if (res.ok) versionInfo = await res.json();
		} catch {
			// Non-critical — just leave the badge blank if the backend is unreachable.
		}
	});
</script>

<header
	class="flex items-center justify-between bg-white px-6 py-4 shadow dark:bg-gray-800"
	style="border-bottom: 3px solid {accentHex}"
>
	<div class="flex items-center gap-4">
		<button
			aria-label="Toggle sidebar"
			aria-expanded={sidebarOpen}
			class="text-gray-700 dark:text-gray-300 md:hidden"
			onclick={toggleSidebar}
		>
			<span aria-hidden="true">☰</span>
		</button>
		<h1 class="text-xl font-semibold text-gray-800 dark:text-white">{settings.value.portalName}</h1>
		{#if versionInfo}
			<span
				class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-500 dark:bg-gray-700 dark:text-gray-400"
				title="Backend version and git branch"
			>
				v{versionInfo.version} &middot; {versionInfo.git_branch}
			</span>
		{/if}
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
		<!-- Decorative placeholder avatar — hidden from assistive tech -->
		<span aria-hidden="true">
			<Avatar>
				<BugOutline />
			</Avatar>
		</span>
	</div>
</header>
