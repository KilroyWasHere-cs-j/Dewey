<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { Alert } from 'flowbite-svelte';
	import { Progressradial } from 'flowbite-svelte';
	import { sineOut } from 'svelte/easing';

	let failedToFetchMetrics = $state(false);
	let loading = $state(true);
	let progress = $state(0);

	onMount(() => {
		async function testMetricsRoute() {
			progress = 25;
			try {
				const res = await fetch('/api/metrics');
				progress = 75;
				if (res.ok) {
					progress = 100;
					failedToFetchMetrics = false;
					goto('/dashboard');
				} else {
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
