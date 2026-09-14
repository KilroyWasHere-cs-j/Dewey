<script lang="ts">
	import { Chart } from '@flowbite-svelte-plugins/chart';
	import type { ApexOptions } from 'apexcharts';

	interface Props {
		label: string;
		// Keys become x-axis categories; values become bar heights
		data: Record<string, number>;
		colors?: string[];
		unit?: string;
		class?: string;
		// Whether the chart should animate when its data updates — disabled
		// at fast poll intervals (issue #427), same reasoning as MetricGraph.
		animate?: boolean;
	}

	let {
		label,
		data,
		colors = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6'],
		unit = '',
		class: className = '',
		animate = true
	}: Props = $props();

	// Recomputes whenever `data` prop changes — the Chart action calls updateOptions().
	// `distributed: true` lets each bar take its own color from the colors array.
	let options = $derived<ApexOptions>({
		chart: {
			type: 'bar',
			height: 160,
			toolbar: { show: false },
			animations: { enabled: animate, speed: 300 }
		},
		plotOptions: {
			bar: {
				horizontal: false,
				borderRadius: 4,
				distributed: true
			}
		},
		series: [{ name: label, data: Object.values(data) }],
		xaxis: {
			categories: Object.keys(data),
			labels: { style: { fontSize: '11px' } }
		},
		colors,
		dataLabels: { enabled: false },
		legend: { show: false },
		tooltip: {
			y: { formatter: (v) => (unit ? `${v} ${unit}` : String(v)) }
		}
	});
</script>

<div class="rounded-2xl bg-white p-4 shadow-sm dark:bg-gray-800 {className}">
	<h3 class="mb-2 text-sm font-medium text-gray-500 dark:text-gray-400">{label}</h3>
	{#if Object.keys(data).length > 0}
		<Chart {options} />
	{:else}
		<div class="flex h-40 items-center justify-center text-sm text-gray-400 dark:text-gray-500">No data yet</div>
	{/if}
</div>
