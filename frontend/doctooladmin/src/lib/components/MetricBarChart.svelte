<script lang="ts">
	import { Chart } from '@flowbite-svelte-plugins/chart';

	interface Props {
		data: Record<string, number> | undefined;
		title: string;
		color?: string;
	}

	let { data, title, color = '#3b82f6' }: Props = $props();

	let labels = $derived(Object.keys(data ?? {}));
	let values = $derived(Object.values(data ?? {}));

	let options = $derived({
		chart: {
			type: 'bar' as const,
			height: 200,
			toolbar: { show: false },
			animations: { enabled: false }
		},
		series: [{ name: title, data: values }],
		xaxis: { categories: labels },
		colors: [color],
		dataLabels: { enabled: false },
		plotOptions: {
			bar: { borderRadius: 4, horizontal: false }
		},
		grid: {
			borderColor: '#f1f5f9'
		}
	});
</script>

<div class="rounded-2xl bg-white p-4 shadow-sm">
	<h3 class="mb-3 text-sm font-semibold text-gray-700">{title}</h3>
	{#if labels.length > 0}
		<Chart {options} />
	{:else}
		<div class="flex h-48 items-center justify-center text-sm text-gray-400">
			No data yet — waiting for events
		</div>
	{/if}
</div>
