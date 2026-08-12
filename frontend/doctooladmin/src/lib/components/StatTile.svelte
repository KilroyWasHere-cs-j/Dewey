<script lang="ts">
	interface Props {
		label: string;
		value: number;
		unit?: string;
		// Optional custom formatter for the displayed value
		formatter?: (v: number) => string;
	}

	let { label, value = 0, unit = '', formatter }: Props = $props();

	function fmt(v: number): string {
		if (formatter) return formatter(v);
		return unit ? `${v} ${unit}` : String(v);
	}
</script>

<!-- Same card shell as MetricGraph, minus the sparkline — for values like a
     countdown or uptime where a history graph isn't a meaningful trend to
     look at (issue #279). -->
<div class="rounded-2xl bg-white p-4 shadow-sm dark:bg-gray-800">
	<div class="flex items-baseline justify-between">
		<h3 class="text-sm font-medium text-gray-500 dark:text-gray-400">{label}</h3>
		<span class="text-xl font-bold text-gray-800 dark:text-white">{fmt(value)}</span>
	</div>
</div>
