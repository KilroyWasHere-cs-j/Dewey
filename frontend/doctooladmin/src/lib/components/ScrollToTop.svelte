<script lang="ts">
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import { ChevronDoubleUpOutline } from 'flowbite-svelte-icons';

	// Threshold (px) past which the button appears — low enough to be useful
	// on the shorter admin docs page, but not so low it shows up after a
	// trivial scroll.
	const SHOW_THRESHOLD = 400;

	let visible = $state(false);

	onMount(() => {
		const onScroll = () => {
			visible = window.scrollY > SHOW_THRESHOLD;
		};
		onScroll();
		window.addEventListener('scroll', onScroll, { passive: true });
		return () => window.removeEventListener('scroll', onScroll);
	});

	function scrollToTop() {
		window.scrollTo({ top: 0, behavior: 'smooth' });
	}
</script>

{#if visible}
	<button
		aria-label="Scroll to top"
		onclick={scrollToTop}
		transition:fade={{ duration: 150 }}
		class="fixed bottom-6 right-6 z-40 rounded-full bg-white p-3 text-gray-500 shadow-lg transition-colors hover:bg-gray-100 hover:text-gray-700 dark:bg-gray-800 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-200"
	>
		<ChevronDoubleUpOutline class="h-5 w-5" />
	</button>
{/if}
