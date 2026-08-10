<script lang="ts">
	import type { Snippet } from 'svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Topbar from '$lib/components/Topbar.svelte';
	import { settings, ACCENT } from '$lib/stores/settings.svelte';

	// Wraps the Sidebar/Topbar/outer-div shell that was previously duplicated
	// verbatim across files/machines/settings/analytics (issue #219).
	//
	// maxWidth and spacious cover the two ways pages' non-compact spacing
	// already differed before this change (settings needed max-w-3xl,
	// analytics used space-y-8 instead of space-y-6) — passing them through
	// keeps each page's exact prior layout instead of silently unifying it.
	let {
		maxWidth,
		spacious = 'space-y-6 p-6',
		children
	}: {
		maxWidth?: string;
		spacious?: string;
		children: Snippet<[{ headingClass: string }]>;
	} = $props();

	let sidebarOpen = $state(settings.value.defaultSidebarOpen);
	const toggleSidebar = () => {
		sidebarOpen = !sidebarOpen;
	};

	let headingClass = $derived(ACCENT[settings.value.accentColor].text);
	let mainClass = $derived(
		[maxWidth, settings.value.layoutDensity === 'compact' ? 'space-y-4 p-4' : spacious]
			.filter(Boolean)
			.join(' ')
	);
</script>

<div class="flex min-h-screen bg-gray-100 dark:bg-gray-900">
	<Sidebar open={sidebarOpen} toggle={toggleSidebar} />

	<div class="flex flex-1 flex-col">
		<Topbar {sidebarOpen} {toggleSidebar} />

		<main id="main-content" class={mainClass}>
			{@render children({ headingClass })}
		</main>
	</div>
</div>
