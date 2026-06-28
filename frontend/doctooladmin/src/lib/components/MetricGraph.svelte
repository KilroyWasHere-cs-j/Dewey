<script lang="ts">
	import { untrack } from 'svelte';
	import { Chart } from '@flowbite-svelte-plugins/chart';
	import type { ApexOptions } from 'apexcharts';

	interface Props {
		label: string;
		value: number;
		unit?: string;
		color?: string;
		// How many historical readings to keep in the sparkline window
		maxHistory?: number;
		// Optional custom formatter for the displayed value and tooltip
		formatter?: (v: number) => string;
	}

	let {
		label,
		value = 0,
		unit = '',
		color = '#3b82f6',
		maxHistory = 40,
		formatter
	}: Props = $props();

	let history = $state<number[]>([]);
	let timestamps = $state<string[]>([]);

	// Push each new reading into the rolling window.
	// untrack() is used to read history/timestamps without creating a dependency —
	// otherwise this effect would re-trigger itself every time it writes.
	$effect(() => {
		if (typeof value !== 'number' || isNaN(value)) return;

		const v = value;
		const now = new Date().toLocaleTimeString([], {
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		});

		history = [...untrack(() => history).slice(-(maxHistory - 1)), v];
		timestamps = [...untrack(() => timestamps).slice(-(maxHistory - 1)), now];
	});

	function fmt(v: number): string {
		if (formatter) return formatter(v);
		return unit ? `${v} ${unit}` : String(v);
	}

	// Recomputes whenever history changes — the Chart action picks up new options
	// and calls chart.updateOptions() automatically via the use:initChart directive.
	let options = $derived<ApexOptions>({
		chart: {
			type: 'area',
			height: 80,
			sparkline: { enabled: true },
			animations: {
				enabled: true,
				speed: 300,
				animateGradually: { enabled: false }
			}
		},
		series: [{ name: label, data: [...history] }],
		xaxis: { categories: [...timestamps] },
		colors: [color],
		stroke: { curve: 'smooth', width: 2 },
		fill: {
			type: 'gradient',
			gradient: {
				shadeIntensity: 1,
				opacityFrom: 0.35,
				opacityTo: 0,
				stops: [0, 100]
			}
		},
		tooltip: {
			fixed: { enabled: false },
			x: { show: true },
			y: { formatter: (v) => fmt(v) },
			marker: { show: false }
		}
	});
</script>

<div class="rounded-2xl bg-white p-4 shadow-sm dark:bg-gray-800">
	<div class="mb-1 flex items-baseline justify-between">
		<h3 class="text-sm font-medium text-gray-500 dark:text-gray-400">{label}</h3>
		<span class="text-xl font-bold text-gray-800 dark:text-white">{fmt(value)}</span>
	</div>
	<Chart {options} />
</div>
