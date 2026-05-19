<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { Alert } from 'flowbite-svelte';
	import { Progressradial } from 'flowbite-svelte';
	import { sineOut } from 'svelte/easing';

	let failedToFetchMetrics = $state(false);
	let loading = $state(true);
	let progress = $state(0);

	// Artificial delay to simulate loading hehehe, remove this for amazing and incredible "performance" boosts
	async function sleep(ms: number): Promise<void> {
		return new Promise((resolve) => setTimeout(resolve, ms));
	}

	onMount(() => {
		async function testMetricsRoute() {
			progress = 0;
			try {
				progress = 25;
				await sleep(1000);
				const res = await fetch('/api/metrics');
				progress = 50;
				await sleep(1000);
				if (res.ok) {
					progress = 75;
					await sleep(1000);
					progress = 100;
					failedToFetchMetrics = false;
					goto('/dashboard');
				} else {
					await sleep(1000);
					failedToFetchMetrics = true;
					loading = false;
				}
			} catch (e) {
				alert('Failed to poll metrics: ' + e);
			}
		}
		testMetricsRoute();
	});
</script>

{#if loading}
	<Progressradial
		{progress}
		animate
		precision={1}
		labelOutside="Animation"
		labelInside
		tweenDuration={1000}
		easing={sineOut}
	/>
{/if}

{#if failedToFetchMetrics}
	<Alert class="m-4 bg-red-600">
		<span class="font-medium">Failed to fetch metrics</span>
		We checked it seems the metrics backend is not responding. Please contact your administrator.
	</Alert>
{/if}
