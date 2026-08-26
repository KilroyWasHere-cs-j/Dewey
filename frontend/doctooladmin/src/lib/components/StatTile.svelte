<script lang="ts">
	interface Props {
		label: string;
		value: number;
		unit?: string;
		// Optional custom formatter for the displayed value
		formatter?: (v: number) => string;
		// When true, `value` is treated as seconds-remaining and ticked down
		// client-side every second instead of only updating on poll (issue #304)
		countdown?: boolean;
	}

	let { label, value = 0, unit = '', formatter, countdown = false }: Props = $props();

	// Local tick-down state, resynced to the polled `value` every time it changes
	// so client-side ticking can't drift from the server's authoritative number
	let remaining = $state(value);
	$effect(() => {
		remaining = value;
	});

	$effect(() => {
		if (!countdown) return;
		const id = setInterval(() => {
			remaining = Math.max(0, remaining - 1);
		}, 1000);
		return () => clearInterval(id);
	});

	function fmtCountdown(totalSeconds: number): string {
		const s = Math.floor(totalSeconds);
		const hh = Math.floor(s / 3600);
		const mm = Math.floor((s % 3600) / 60);
		const ss = s % 60;
		const pad = (n: number) => String(n).padStart(2, '0');
		return hh > 0 ? `${pad(hh)}:${pad(mm)}:${pad(ss)}` : `${pad(mm)}:${pad(ss)}`;
	}

	function fmt(v: number): string {
		if (countdown) return fmtCountdown(v);
		if (formatter) return formatter(v);
		return unit ? `${v} ${unit}` : String(v);
	}

	// Last-15-seconds urgency state — New Year's-style flourish for the cache
	// wipe countdown (issue #304). Only kicks in for countdown tiles.
	let urgent = $derived(countdown && remaining > 0 && remaining <= 15);
</script>

<!-- Same card shell as MetricGraph, minus the sparkline — for values like a
     countdown or uptime where a history graph isn't a meaningful trend to
     look at (issue #279). -->
<div
	class="rounded-2xl bg-white p-4 shadow-sm transition-colors dark:bg-gray-800"
	class:ring-2={urgent}
	class:ring-red-500={urgent}
	class:animate-pulse={urgent}
>
	<div class="flex items-baseline justify-between">
		<h3 class="text-sm font-medium text-gray-500 dark:text-gray-400">{label}</h3>
		<span
			class="text-xl font-bold text-gray-800 dark:text-white"
			class:text-red-500={urgent}
			class:dark:text-red-400={urgent}
		>
			{fmt(remaining)}
		</span>
	</div>
</div>
