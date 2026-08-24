<script lang="ts">
	import AppShell from '$lib/components/AppShell.svelte';
	import { settings, CHART_THEMES, type Settings } from '$lib/stores/settings.svelte';

	let saved = $state(false);

	// Apply a partial update and flash the "Saved" indicator
	function update<K extends keyof Settings>(key: K, value: Settings[K]) {
		settings.update({ [key]: value } as Partial<Settings>);
		saved = true;
		setTimeout(() => (saved = false), 1500);
	}

	function reset() {
		settings.reset();
		saved = true;
		setTimeout(() => (saved = false), 1500);
	}

	const pollOptions = [
		{ label: '1s', value: 1000 },
		{ label: '5s', value: 5000 },
		{ label: '15s', value: 15000 },
		{ label: '30s', value: 30000 }
	];

	const historyOptions = [
		{ label: '20', value: 20 },
		{ label: '40', value: 40 },
		{ label: '80', value: 80 },
		{ label: '120', value: 120 }
	];

	// Full class strings for Tailwind scanner — no dynamic concatenation
	const accentOptions: { label: string; value: Settings['accentColor']; swatch: string }[] = [
		{ label: 'Yellow', value: 'yellow', swatch: 'bg-yellow-400' },
		{ label: 'Blue', value: 'blue', swatch: 'bg-blue-400' },
		{ label: 'Green', value: 'green', swatch: 'bg-emerald-400' },
		{ label: 'Purple', value: 'purple', swatch: 'bg-purple-400' },
		{ label: 'Slate', value: 'slate', swatch: 'bg-slate-500' }
	];

	const themeOptions: { label: string; value: Settings['chartColorTheme']; preview: string[] }[] = [
		{ label: 'Default', value: 'default', preview: CHART_THEMES.default.slice(0, 4) },
		{ label: 'Cool', value: 'cool', preview: CHART_THEMES.cool.slice(0, 4) },
		{ label: 'Warm', value: 'warm', preview: CHART_THEMES.warm.slice(0, 4) },
		{ label: 'Mono', value: 'mono', preview: CHART_THEMES.mono.slice(0, 4) }
	];
</script>

<svelte:head>
	<title>Settings — Dewey</title>
</svelte:head>

<AppShell maxWidth="max-w-3xl">
	{#snippet children({ headingClass })}
		<!-- Header -->
		<div class="flex items-center justify-between">
			<div>
				<h1 class="text-2xl font-bold text-gray-800 dark:text-white">Settings</h1>
				<p class="text-sm text-gray-500 dark:text-gray-400">
					Changes save instantly to your browser.
				</p>
			</div>
			<div class="flex items-center gap-3">
				<!-- aria-live keeps this region in the DOM so screen readers catch the transition -->
				<span aria-live="polite" class="text-sm font-medium text-green-600 dark:text-green-400">
					{#if saved}Saved ✓{/if}
				</span>
				<button
					class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-600 shadow-sm hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
					onclick={reset}
				>
					Reset to defaults
				</button>
			</div>
		</div>

		<!-- ── Behavior ── -->
		<section class="space-y-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
			<h2 class="text-xs font-semibold tracking-wider uppercase {headingClass}">Behavior</h2>

			<div class="flex items-start justify-between gap-4">
				<div>
					<p class="font-medium text-gray-700 dark:text-gray-200">Metrics Poll Interval</p>
					<p class="text-sm text-gray-500 dark:text-gray-400">
						How often the dashboard and analytics pages refresh.
					</p>
				</div>
				<div
					role="group"
					aria-label="Metrics poll interval"
					class="flex shrink-0 overflow-hidden rounded-lg border border-gray-200 bg-gray-50 dark:border-gray-600 dark:bg-gray-700"
				>
					{#each pollOptions as opt}
						<button
							aria-pressed={settings.value.pollIntervalMs === opt.value}
							class="px-4 py-2 text-sm font-medium transition-colors {settings.value
								.pollIntervalMs === opt.value
								? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900'
								: 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-600'}"
							onclick={() => update('pollIntervalMs', opt.value)}
						>
							{opt.label}
						</button>
					{/each}
				</div>
			</div>

			<div class="flex items-start justify-between gap-4">
				<div>
					<p class="font-medium text-gray-700 dark:text-gray-200">Analytics History Window</p>
					<p class="text-sm text-gray-500 dark:text-gray-400">
						How many data points the sparkline charts keep.
					</p>
				</div>
				<div
					role="group"
					aria-label="Analytics history window"
					class="flex shrink-0 overflow-hidden rounded-lg border border-gray-200 bg-gray-50 dark:border-gray-600 dark:bg-gray-700"
				>
					{#each historyOptions as opt}
						<button
							aria-pressed={settings.value.analyticsHistoryWindow === opt.value}
							class="px-4 py-2 text-sm font-medium transition-colors {settings.value
								.analyticsHistoryWindow === opt.value
								? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900'
								: 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-600'}"
							onclick={() => update('analyticsHistoryWindow', opt.value)}
						>
							{opt.label}
						</button>
					{/each}
				</div>
			</div>

			<div class="flex items-center justify-between">
				<div>
					<p class="font-medium text-gray-700 dark:text-gray-200">Prometheus Alert Banner</p>
					<p class="text-sm text-gray-500 dark:text-gray-400">
						Show the red banner when Prometheus is unreachable.
					</p>
				</div>
				<button
					role="switch"
					aria-label="Prometheus Alert Banner"
					aria-checked={settings.value.showPrometheusAlert}
					class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full transition-colors {settings
						.value.showPrometheusAlert
						? 'bg-gray-900 dark:bg-blue-500'
						: 'bg-gray-300 dark:bg-gray-600'}"
					onclick={() => update('showPrometheusAlert', !settings.value.showPrometheusAlert)}
				>
					<span
						class="inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform {settings
							.value.showPrometheusAlert
							? 'translate-x-6'
							: 'translate-x-1'}"
					></span>
				</button>
			</div>

			<div class="flex items-center justify-between">
				<div>
					<p class="font-medium text-gray-700 dark:text-gray-200">Default Sidebar State</p>
					<p class="text-sm text-gray-500 dark:text-gray-400">
						Whether the sidebar starts open or collapsed.
					</p>
				</div>
				<button
					role="switch"
					aria-label="Default Sidebar State"
					aria-checked={settings.value.defaultSidebarOpen}
					class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full transition-colors {settings
						.value.defaultSidebarOpen
						? 'bg-gray-900 dark:bg-blue-500'
						: 'bg-gray-300 dark:bg-gray-600'}"
					onclick={() => update('defaultSidebarOpen', !settings.value.defaultSidebarOpen)}
				>
					<span
						class="inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform {settings
							.value.defaultSidebarOpen
							? 'translate-x-6'
							: 'translate-x-1'}"
					></span>
				</button>
			</div>

			<div class="flex items-start justify-between gap-4">
				<div>
					<p class="font-medium text-gray-700 dark:text-gray-200">Layout Density</p>
					<p class="text-sm text-gray-500 dark:text-gray-400">
						Controls padding and spacing throughout the app.
					</p>
				</div>
				<div
					role="group"
					aria-label="Layout density"
					class="flex shrink-0 overflow-hidden rounded-lg border border-gray-200 bg-gray-50 dark:border-gray-600 dark:bg-gray-700"
				>
					{#each ['comfortable', 'compact'] as const as opt}
						<button
							aria-pressed={settings.value.layoutDensity === opt}
							class="px-4 py-2 text-sm font-medium capitalize transition-colors {settings.value
								.layoutDensity === opt
								? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900'
								: 'text-gray-600 hover:bg-gray-100 dark:text-gray-300 dark:hover:bg-gray-600'}"
							onclick={() => update('layoutDensity', opt)}
						>
							{opt}
						</button>
					{/each}
				</div>
			</div>
		</section>

		<!-- ── Alerts ── -->
		<section class="space-y-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
			<h2 class="text-xs font-semibold tracking-wider uppercase {headingClass}">Alerts</h2>

			<div class="flex items-center justify-between gap-4">
				<div>
					<p class="font-medium text-gray-700 dark:text-gray-200">RAM Alert Threshold</p>
					<p class="text-sm text-gray-500 dark:text-gray-400">
						Warn when RAM usage exceeds this value.
					</p>
				</div>
				<div class="flex shrink-0 items-center gap-2">
					<input
						type="number"
						min="0"
						aria-label="RAM alert threshold in megabytes"
						autocomplete="off"
						class="w-24 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-right text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
						value={settings.value.ramAlertThresholdMb}
						oninput={(e) =>
							update('ramAlertThresholdMb', Number((e.target as HTMLInputElement).value))}
					/>
					<span class="text-sm text-gray-500 dark:text-gray-400">MB</span>
				</div>
			</div>

			<div class="flex items-center justify-between gap-4">
				<div>
					<p class="font-medium text-gray-700 dark:text-gray-200">Retrieval Alert Threshold</p>
					<p class="text-sm text-gray-500 dark:text-gray-400">
						Warn when file retrievals exceed this count.
					</p>
				</div>
				<div class="flex shrink-0 items-center gap-2">
					<input
						type="number"
						min="0"
						aria-label="File retrieval alert threshold"
						autocomplete="off"
						class="w-24 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-right text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
						value={settings.value.retryAlertThreshold}
						oninput={(e) =>
							update('retryAlertThreshold', Number((e.target as HTMLInputElement).value))}
					/>
					<span class="text-sm text-gray-500 dark:text-gray-400">retrievals</span>
				</div>
			</div>
		</section>

		<!-- ── Customization ── -->
		<section class="space-y-6 rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
			<h2 class="text-xs font-semibold tracking-wider uppercase {headingClass}">Customization</h2>

			<div class="flex items-center justify-between">
				<div>
					<p class="font-medium text-gray-700 dark:text-gray-200">Dark Mode</p>
					<p class="text-sm text-gray-500 dark:text-gray-400">
						Switch the entire app to a dark color scheme.
					</p>
				</div>
				<button
					role="switch"
					aria-label="Dark Mode"
					aria-checked={settings.value.darkMode}
					class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full transition-colors {settings
						.value.darkMode
						? 'bg-gray-900 dark:bg-blue-500'
						: 'bg-gray-300 dark:bg-gray-600'}"
					onclick={() => update('darkMode', !settings.value.darkMode)}
				>
					<span
						class="inline-block h-4 w-4 transform rounded-full bg-white shadow transition-transform {settings
							.value.darkMode
							? 'translate-x-6'
							: 'translate-x-1'}"
					></span>
				</button>
			</div>

			<div class="flex items-start justify-between gap-4">
				<div>
					<p class="font-medium text-gray-700 dark:text-gray-200">Accent Color</p>
					<p class="text-sm text-gray-500 dark:text-gray-400">
						Used for dashboard cards and highlights.
					</p>
				</div>
				<div role="group" aria-label="Accent color" class="flex shrink-0 gap-2">
					{#each accentOptions as opt}
						<button
							title={opt.label}
							aria-label={opt.label}
							aria-pressed={settings.value.accentColor === opt.value}
							class="h-8 w-8 rounded-full transition-transform hover:scale-110 {opt.swatch} {settings
								.value.accentColor === opt.value
								? 'scale-110 ring-2 ring-gray-900 ring-offset-2 dark:ring-white dark:ring-offset-gray-800'
								: ''}"
							onclick={() => update('accentColor', opt.value)}
						></button>
					{/each}
				</div>
			</div>

			<div class="flex items-start justify-between gap-4">
				<div>
					<p class="font-medium text-gray-700 dark:text-gray-200">Chart Color Theme</p>
					<p class="text-sm text-gray-500 dark:text-gray-400">
						Color palette used by analytics charts.
					</p>
				</div>
				<div role="group" aria-label="Chart color theme" class="flex shrink-0 gap-2">
					{#each themeOptions as opt}
						<button
							aria-label="{opt.label} theme"
							aria-pressed={settings.value.chartColorTheme === opt.value}
							class="flex flex-col items-center gap-1.5 rounded-xl border-2 p-2 transition-colors {settings
								.value.chartColorTheme === opt.value
								? 'border-gray-900 bg-gray-50 dark:border-white dark:bg-gray-700'
								: 'border-transparent hover:border-gray-200 dark:hover:border-gray-600'}"
							onclick={() => update('chartColorTheme', opt.value)}
						>
							<div class="flex gap-0.5">
								{#each opt.preview as hex}
									<span class="h-4 w-4 rounded-sm" style="background:{hex}"></span>
								{/each}
							</div>
							<span class="text-xs text-gray-500 dark:text-gray-400">{opt.label}</span>
						</button>
					{/each}
				</div>
			</div>
		</section>
	{/snippet}
</AppShell>
