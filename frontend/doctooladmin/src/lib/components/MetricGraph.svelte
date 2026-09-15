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
		// Whether the chart should animate when its data updates — disabled
		// at fast poll intervals (issue #427) since ~20 tiles animating in
		// lockstep every second is real CPU/GPU cost for values that mostly
		// change slowly.
		animate?: boolean;
	}

	let {
		label,
		value = 0,
		unit = '',
		color = '#3b82f6',
		maxHistory = 40,
		formatter,
		animate = true
	}: Props = $props();

	let history = $state<number[]>([]);
	let timestamps = $state<string[]>([]);

	// Push each new reading into the rolling window — but only when it
	// actually differs from the last one (issue #427). A poll tick whose
	// value hasn't moved (e.g. a steady goroutine count) would otherwise
	// still append a duplicate point and force a full chart redraw for no
	// visible change. This does mean the sparkline reads as "last N actual
	// changes" rather than "last N poll ticks" — a flat metric shows fewer,
	// older points instead of a row of identical dots, which better reflects
	// what's actually happening.
	// untrack() is used to read history/timestamps without creating a dependency —
	// otherwise this effect would re-trigger itself every time it writes.
	$effect(() => {
		if (typeof value !== 'number' || isNaN(value)) return;

		const v = value;
		const lastValue = untrack(() => history[history.length - 1]);
		if (lastValue === v) return;

		const now = new Date().toLocaleTimeString([], {
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		});

		// maxHistory is read via untrack() too (issue #417) — otherwise it's
		// an unintended reactive dependency of this effect, and a change to
		// it alone (no new value) would re-run the effect and push a
		// phantom duplicate point. Only an actual change to `value` should
		// ever trigger a push; maxHistory just needs its current value at
		// push time, not to be watched for changes.
		const currentMaxHistory = untrack(() => maxHistory);
		history = [...untrack(() => history).slice(-(currentMaxHistory - 1)), v];
		timestamps = [...untrack(() => timestamps).slice(-(currentMaxHistory - 1)), now];
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
				enabled: animate,
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
