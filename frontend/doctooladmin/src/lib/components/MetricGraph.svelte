<script lang="ts">
	import { Chart } from '@flowbite-svelte-plugins/chart';

	interface Props {
		value: number | undefined;
		label: string;
		color?: string;
		// How many data points to keep in the rolling window (default: 60)
		windowSize?: number;
	}

	let { value, label, color = '#3b82f6', windowSize = 60 }: Props = $props();

	// Rolling time-series maintained client-side across polls
	let history = $state<number[]>([]);

	$effect(() => {
		// Each time the parent updates `value`, append it and trim the window
		history = [...history.slice(-(windowSize - 1)), value ?? 0];
	});

	let options = $derived({
		chart: {
			type: 'line' as const,
			sparkline: { enabled: true },
			height: 56,
			// Disable animations so the chart snaps immediately on each poll
			animations: { enabled: false }
		},
		series: [{ name: label, data: history }],
		stroke: { curve: 'smooth' as const, width: 2 },
		colors: [color],
		tooltip: {
			x: { show: false },
			y: { formatter: (v: number) => v.toFixed(0) }
		}
	});
</script>

<div class="rounded-2xl bg-white p-4 shadow-sm">
	<p class="text-xs font-medium tracking-wide text-gray-400 uppercase">{label}</p>
	<p class="mt-1 text-2xl font-semibold text-gray-800">{value ?? 0}</p>
	{#if history.length > 1}
		<div class="mt-2">
			<Chart {options} />
		</div>
	{/if}
</div>
