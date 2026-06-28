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

{@render children()}
