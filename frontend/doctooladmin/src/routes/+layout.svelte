<script lang="ts">
	import { browser } from '$app/environment';
	import { settings } from '$lib/stores/settings.svelte';
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';

	let { children } = $props();

	// Toggle the `dark` class on <html> whenever the setting changes.
	// The CSS custom variant `dark (&:where(.dark, .dark *))` in layout.css
	// makes all dark: Tailwind classes activate when this class is present.
	$effect(() => {
		if (!browser) return;
		document.documentElement.classList.toggle('dark', settings.value.darkMode);
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<!-- Visually hidden until focused — lets keyboard users skip past nav to page content -->
<a
	href="#main-content"
	class="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50 focus:rounded focus:bg-white focus:px-4 focus:py-2 focus:text-sm focus:font-semibold focus:text-gray-900 focus:shadow focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 dark:focus:bg-gray-800 dark:focus:text-white"
>
	Skip to main content
</a>

{@render children()}
