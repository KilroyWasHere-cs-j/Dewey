<script lang="ts">
	import { onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import { flip } from 'svelte/animate';
	import {
		dragHandleZone,
		dragHandle,
		SHADOW_ITEM_MARKER_PROPERTY_NAME,
		setFeatureFlag,
		FEATURE_FLAG_NAMES,
		type DndEvent,
		type Item as DndItem
	} from 'svelte-dnd-action';
	// Every dndzone here is a CSS grid (not flex), which svelte-dnd-action's
	// default bounding-rect-based position tracking has a known lag with
	// (library issues #454/#470) — tiles can visually "not lock" into the
	// grid cell they're dropped over. This flag switches it to computed-style
	// based tracking instead, which the library recommends for grid layouts.
	setFeatureFlag(FEATURE_FLAG_NAMES.USE_COMPUTED_STYLE_INSTEAD_OF_BOUNDING_RECT, true);
	import AppShell from '$lib/components/AppShell.svelte';
	import MetricGraph from '$lib/components/MetricGraph.svelte';
	import MetricBarChart from '$lib/components/MetricBarChart.svelte';
	import StatTile from '$lib/components/StatTile.svelte';
	import { settings, CHART_THEMES, resolveTileOrder } from '$lib/stores/settings.svelte';
	import { toasts } from '$lib/stores/toasts.svelte';
	import type { AppMetrics } from '$lib/types';

	let metrics = $state<Partial<AppMetrics>>({});
	// 'rateLimited' (backend's own rate limiter returned 429) is kept distinct
	// from 'unreachable' (network failure or any other non-ok status) so the
	// banner/toast wording doesn't claim a real outage when it's just being
	// throttled (issue #362).
	let metricsStatus = $state<'ok' | 'rateLimited' | 'unreachable'>('ok');

	async function refreshMetrics() {
		const previousStatus = metricsStatus;

		try {
			const res = await fetch('/api/metrics');
			if (res.ok) {
				metrics = await res.json();
				metricsStatus = 'ok';
			} else if (res.status === 429) {
				metricsStatus = 'rateLimited';
			} else {
				metricsStatus = 'unreachable';
			}
		} catch {
			metricsStatus = 'unreachable';
		}

		// Only toast on a state change, not every poll tick — this fires every
		// pollIntervalMs (default 5s), so toasting unconditionally would spam
		// the same message over and over for as long as the condition holds.
		if (metricsStatus !== previousStatus && metricsStatus !== 'ok') {
			toasts.push(
				metricsStatus === 'rateLimited'
					? {
							kind: 'warning',
							title: 'Metrics polling rate-limited',
							detail: 'The backend is throttling requests — metrics will resume shortly.'
						}
					: {
							kind: 'error',
							title: 'Prometheus unreachable',
							detail: 'Metrics may be stale or unavailable until the connection recovers.'
						}
			);
		}
	}

	// Initial fetch on mount
	onMount(refreshMetrics);

	// Recreate the polling interval whenever pollIntervalMs changes in settings.
	// The effect cleanup (return fn) clears the old interval before starting a new one.
	$effect(() => {
		const id = setInterval(refreshMetrics, settings.value.pollIntervalMs);
		return () => clearInterval(id);
	});

	// Sum app_upload_rejections_total across every reason label (pe_blocked,
	// elf_blocked, content_mismatch, pdf_js_blocked, etc.) into one running
	// count for the sparkline — the per-reason breakdown isn't shown here.
	let uploadRejectionsTotal = $derived(
		Object.values((metrics.app_upload_rejections_total as Record<string, number>) ?? {}).reduce(
			(sum, v) => sum + v,
			0
		)
	);

	// Aggregate file_io_ops_total (3 labels: op, result, file_group) by op for a clean bar chart
	let ioOpsByType = $derived(
		Object.entries((metrics.file_io_ops_total as Record<string, number>) ?? {}).reduce(
			(acc, [key, val]) => {
				const op =
					key
						.split(',')
						.find((p) => p.startsWith('op='))
						?.split('=')[1] ?? key;
				acc[op] = (acc[op] ?? 0) + val;
				return acc;
			},
			{} as Record<string, number>
		)
	);

	// Convenience shortcuts into the current theme's color array
	let colors = $derived(CHART_THEMES[settings.value.chartColorTheme]);

	// Alert conditions derived from live metrics + user thresholds
	let ramAlert = $derived((metrics.app_ram_usage ?? 0) > settings.value.ramAlertThresholdMb);
	let retrievalAlert = $derived(
		(metrics.app_file_retrievals ?? 0) > settings.value.retryAlertThreshold
	);

	function round2(v: number) {
		return Math.round(v * 100) / 100;
	}

	// --- Tile reordering (issue #343) ---------------------------------------
	//
	// Every tile is keyed by a stable id and grouped into a fixed section, so
	// drag-and-drop can reorder tiles within a section without touching which
	// section they belong to (design call: within-section only, not free
	// cross-section dragging — see issue #343).

	interface SectionDef {
		id: string;
		title: string;
		// Tailwind grid-cols classes — each section has its own tuned column
		// count depending on how many tiles it holds.
		gridClass: string;
		defaultTileIds: string[];
	}

	const SECTIONS: SectionDef[] = [
		{
			id: 'system-resources',
			title: 'System Resources',
			gridClass: 'grid-cols-2 lg:grid-cols-4',
			defaultTileIds: ['ram-usage', 'heap-usage', 'goroutines', 'open-fds']
		},
		{
			id: 'application',
			title: 'Application',
			gridClass: 'grid-cols-2 lg:grid-cols-4 xl:grid-cols-5',
			defaultTileIds: ['files-in-store', 'files-in-backup', 'cache-size', 'next-tick', 'uptime']
		},
		{
			id: 'file-io',
			title: 'File I/O',
			gridClass: 'grid-cols-1 lg:grid-cols-2',
			defaultTileIds: ['io-bytes', 'io-ops']
		},
		{
			id: 'activity',
			title: 'Activity',
			gridClass: 'grid-cols-2 lg:grid-cols-3 xl:grid-cols-6',
			defaultTileIds: [
				'file-copies',
				'file-retrievals',
				'file-sorts',
				'file-deletions',
				'filter-loads',
				'gc-cycles'
			]
		},
		{
			id: 'upload-activity',
			title: 'Upload Activity',
			gridClass: 'grid-cols-1 lg:grid-cols-2',
			defaultTileIds: ['upload-rejections-by-reason', 'uploads-by-type']
		},
		{
			id: 'processing-pipeline',
			title: 'Processing Pipeline',
			gridClass: 'grid-cols-2 lg:grid-cols-5 xl:grid-cols-6',
			defaultTileIds: [
				'barcode-successes',
				'barcode-failures',
				'plugin-runs',
				'plugin-errors',
				'db-errors',
				'upload-rejections-total'
			]
		},
		{
			id: 'network',
			title: 'Network',
			gridClass: 'grid-cols-1 lg:grid-cols-2',
			defaultTileIds: ['network-receive', 'network-transmit']
		}
	];

	// Extends svelte-dnd-action's own Item type (Record<string, any>) rather
	// than a plain { id: string } — during a drag, dndzone injects a shadow
	// placeholder into the items array carrying SHADOW_ITEM_MARKER_PROPERTY_NAME,
	// which needs to be checked in the template below.
	interface TileItem extends DndItem {
		id: string;
	}

	// Per-section drag order, seeded from the saved settings (falling back to
	// each section's default order). This is local, mutable $state — dndzone
	// needs to update it live during a drag, and the result is persisted back
	// into the settings store on drop (see handleFinalize below). Read once at
	// component init rather than kept reactive to settings changes, since a
	// later settings.reset() is expected to take effect on next page load
	// (this route unmounts/remounts on navigation), not fight the user's own
	// in-progress drag.
	let sectionOrders = $state<Record<string, TileItem[]>>(
		Object.fromEntries(
			SECTIONS.map((section) => [
				section.id,
				resolveTileOrder(section.defaultTileIds, settings.value.tileOrder[section.id]).map(
					(id) => ({ id })
				)
			])
		)
	);

	function persistTileOrder(sectionId: string, items: TileItem[]) {
		settings.update({
			tileOrder: { ...settings.value.tileOrder, [sectionId]: items.map((item) => item.id) }
		});
	}

	// dndzone requires both consider (live, during drag) and finalize (on
	// drop) to update the items list — only finalize also persists, so a
	// drag that's cancelled mid-flight doesn't write a half-done order.
	function handleConsider(sectionId: string, e: CustomEvent<DndEvent<TileItem>>) {
		sectionOrders[sectionId] = e.detail.items;
	}

	function handleFinalize(sectionId: string, e: CustomEvent<DndEvent<TileItem>>) {
		sectionOrders[sectionId] = e.detail.items;
		persistTileOrder(sectionId, e.detail.items);
	}

	// Dynamic-dispatch table: which component + props each tile id renders.
	// `props` is intentionally `any` rather than a per-component union — the
	// renderer is chosen at runtime by id, so there's no single static shape
	// TypeScript could check it against (Record<string, unknown> fails: the
	// union of MetricGraph/MetricBarChart/StatTile's Props all require fields
	// like `label`/`data` that `unknown` can't satisfy).
	type TileComponent = typeof MetricGraph | typeof MetricBarChart | typeof StatTile;
	// eslint-disable-next-line @typescript-eslint/no-explicit-any -- see comment above
	let tileRenderers = $derived<Record<string, { component: TileComponent; props: any }>>({
		'ram-usage': {
			component: MetricGraph,
			props: {
				label: 'RAM Usage',
				value: metrics.app_ram_usage ?? 0,
				unit: 'MB',
				color: colors[0],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'heap-usage': {
			component: MetricGraph,
			props: {
				label: 'Heap Usage',
				value: metrics.app_heap_usage ?? 0,
				unit: 'MB',
				color: colors[1],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		goroutines: {
			component: MetricGraph,
			props: {
				label: 'Goroutines',
				value: metrics.go_goroutines ?? 0,
				color: colors[2],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'open-fds': {
			component: MetricGraph,
			props: {
				label: 'Open File Descriptors',
				value: metrics.process_open_fds ?? 0,
				color: colors[3],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'files-in-store': {
			component: MetricGraph,
			props: {
				label: 'Files in Store',
				value: metrics.app_files_in_store ?? 0,
				color: colors[4],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'files-in-backup': {
			component: MetricGraph,
			props: {
				label: 'Files in Backup',
				value: metrics.app_files_in_backup ?? 0,
				color: colors[5],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'cache-size': {
			component: MetricGraph,
			props: {
				label: 'Cache Size',
				value: metrics.app_cache_size ?? 0,
				color: colors[6],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		// Plain stat tiles, not sparkline graphs — a countdown and an
		// ever-increasing uptime counter don't have a meaningful trend
		// to plot (issue #279).
		'next-tick': {
			component: StatTile,
			props: {
				label: 'Dumping the cache in',
				value: metrics.app_time_til_next_tick ?? 0,
				countdown: true
			}
		},
		uptime: {
			component: StatTile,
			props: {
				label: 'Uptime',
				value: round2((metrics.app_uptime_seconds ?? 0) / 3600),
				unit: 'hr'
			}
		},
		// file_io_bytes_total has a single "op" label, so keys are just "read" / "write"
		'io-bytes': {
			component: MetricBarChart,
			props: {
				label: 'I/O Bytes by Operation',
				data: (metrics.file_io_bytes_total as Record<string, number>) ?? {},
				unit: 'bytes',
				colors: colors.slice(0, 4)
			}
		},
		// Aggregated from the 3-label ops metric by summing across result + file_group
		'io-ops': {
			component: MetricBarChart,
			props: { label: 'I/O Operations by Type', data: ioOpsByType, colors: colors.slice(4, 8) }
		},
		'file-copies': {
			component: MetricGraph,
			props: {
				label: 'File Copies',
				value: metrics.app_file_copys ?? 0,
				color: colors[8],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'file-retrievals': {
			component: MetricGraph,
			props: {
				label: 'File Retrievals',
				value: metrics.app_file_retrievals ?? 0,
				color: colors[9],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'file-sorts': {
			component: MetricGraph,
			props: {
				label: 'File Sorts',
				value: metrics.app_file_sorts ?? 0,
				color: colors[10],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'file-deletions': {
			component: MetricGraph,
			props: {
				label: 'File Deletions',
				value: metrics.app_file_deletions ?? 0,
				color: '#ef4444',
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'filter-loads': {
			component: MetricGraph,
			props: {
				label: 'Filter Loads',
				value: metrics.app_filters_loadings ?? 0,
				color: colors[11],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'gc-cycles': {
			component: MetricGraph,
			props: {
				label: 'GC Cycles',
				value: metrics.app_gc_cycles ?? 0,
				color: colors[0],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'upload-rejections-by-reason': {
			component: MetricBarChart,
			props: {
				label: 'Rejections by Reason',
				data: (metrics.app_upload_rejections_total as Record<string, number>) ?? {},
				colors: ['#f97316', '#ef4444', '#dc2626']
			}
		},
		'uploads-by-type': {
			component: MetricBarChart,
			props: {
				label: 'Accepted Uploads by File Type',
				data: (metrics.app_uploads_by_type_total as Record<string, number>) ?? {},
				colors: colors.slice(0, 8)
			}
		},
		'barcode-successes': {
			component: MetricGraph,
			props: {
				label: 'Barcode Successes',
				value: metrics.app_barcode_successes ?? 0,
				color: '#22c55e',
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'barcode-failures': {
			component: MetricGraph,
			props: {
				label: 'Barcode Failures',
				value: metrics.app_barcode_failures ?? 0,
				color: '#ef4444',
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'plugin-runs': {
			component: MetricGraph,
			props: {
				label: 'Plugin Runs',
				value: metrics.app_plugin_runs ?? 0,
				color: colors[2],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'plugin-errors': {
			component: MetricGraph,
			props: {
				label: 'Plugin Errors',
				value: metrics.app_plugin_errors ?? 0,
				color: '#f97316',
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'db-errors': {
			component: MetricGraph,
			props: {
				label: 'DB Errors',
				value: metrics.app_db_errors ?? 0,
				color: '#dc2626',
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'upload-rejections-total': {
			component: MetricGraph,
			props: {
				label: 'Upload Rejections',
				value: uploadRejectionsTotal,
				color: '#a855f7',
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'network-receive': {
			component: MetricGraph,
			props: {
				label: 'Network Receive',
				value: round2((metrics.process_network_receive_bytes_total ?? 0) / 1024 / 1024),
				unit: 'MB',
				color: colors[12],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		},
		'network-transmit': {
			component: MetricGraph,
			props: {
				label: 'Network Transmit',
				value: round2((metrics.process_network_transmit_bytes_total ?? 0) / 1024 / 1024),
				unit: 'MB',
				color: colors[0],
				maxHistory: settings.value.analyticsHistoryWindow
			}
		}
	});
</script>

<!-- Alert banners — stacked via flex column + gap so the browser handles spacing
     instead of hand-computed margin offsets (issue #242). -->
<div class="fixed top-4 left-1/2 z-50 flex -translate-x-1/2 flex-col items-center gap-2">
	<!-- Prometheus unreachable / rate-limited banner (toggled in settings) -->
	{#if metricsStatus !== 'ok' && settings.value.showPrometheusAlert}
		<div
			role="alert"
			transition:fly={{ y: -16, duration: 200 }}
			class="flex items-center gap-3 rounded-lg border border-red-500/40 bg-red-950/90 px-5 py-3 text-sm text-red-300 shadow-lg backdrop-blur"
		>
			<svg
				aria-hidden="true"
				class="h-4 w-4 shrink-0 text-red-400"
				fill="none"
				viewBox="0 0 24 24"
				stroke="currentColor"
				stroke-width="2"
			>
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					d="M12 9v3m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"
				/>
			</svg>
			<span>
				{metricsStatus === 'rateLimited'
					? 'Metrics polling is being rate-limited — retrying shortly'
					: 'Prometheus is unreachable — metrics may be stale or unavailable'}
			</span>
		</div>
	{/if}

	<!-- RAM alert -->
	{#if ramAlert}
		<div
			role="alert"
			transition:fly={{ y: -16, duration: 200 }}
			class="flex items-center gap-3 rounded-lg border border-amber-500/40 bg-amber-950/90 px-5 py-3 text-sm text-amber-300 shadow-lg backdrop-blur"
		>
			<span
				>⚠ RAM usage ({metrics.app_ram_usage} MB) exceeds threshold ({settings.value
					.ramAlertThresholdMb} MB)</span
			>
		</div>
	{/if}

	<!-- Retrieval alert -->
	{#if retrievalAlert}
		<div
			role="alert"
			transition:fly={{ y: -16, duration: 200 }}
			class="flex items-center gap-3 rounded-lg border border-red-500/40 bg-red-950/90 px-5 py-3 text-sm text-red-300 shadow-lg backdrop-blur"
		>
			<span
				>⚠ File retrievals ({metrics.app_file_retrievals}) exceeds threshold ({settings.value
					.retryAlertThreshold})</span
			>
		</div>
	{/if}
</div>

<svelte:head>
	<title>Analytics — Dewey</title>
</svelte:head>

<AppShell spacious="space-y-8 p-6">
	{#snippet children({ headingClass })}
		{#each SECTIONS as section (section.id)}
			<section>
				<h2 class="mb-3 text-xs font-semibold tracking-wider uppercase {headingClass}">
					{section.title}
				</h2>
				<!-- No transition:fade on this element (unlike the rest of the app's
				     section grids) — a Svelte transition on the same element as
				     use:dragHandleZone conflicts with the library's own layout
				     measurements during drag, causing tiles to not snap cleanly
				     into their grid cell (confirmed community report:
				     https://github.com/isaacHagoel/svelte-dnd-action/issues/466). -->
				<div
					use:dragHandleZone={{
						items: sectionOrders[section.id],
						flipDurationMs: 200,
						// Zones with the same type can drop into each other (library
						// default: one shared type for every zone) — giving each
						// section its own type is what actually enforces the
						// within-section-only design call from issue #343.
						type: section.id
					}}
					onconsider={(e: CustomEvent<DndEvent<TileItem>>) => handleConsider(section.id, e)}
					onfinalize={(e: CustomEvent<DndEvent<TileItem>>) => handleFinalize(section.id, e)}
					class="grid gap-4 {section.gridClass}"
				>
					{#each sectionOrders[section.id] as item (item.id + (item[SHADOW_ITEM_MARKER_PROPERTY_NAME] ? '_shadow' : ''))}
						<!-- animate:flip must sit on the each block's single direct child,
						     so the shadow-vs-real-tile branch lives inside this div instead
						     of replacing it. -->
						<div animate:flip={{ duration: 200 }} class="relative">
							{#if item[SHADOW_ITEM_MARKER_PROPERTY_NAME]}
								<!-- Placeholder shown where the dragged tile would land — not a
								     real tile, so it isn't in tileRenderers. -->
								<div
									class="h-full min-h-24 rounded-2xl border-2 border-dashed border-gray-300 dark:border-gray-600"
								></div>
							{:else}
								{@const tile = tileRenderers[item.id]}
								<!-- Drag handle: functional placement for now, styling/polish TBD together. -->
								<div
									use:dragHandle
									aria-label="Drag to reorder {tile.props.label}"
									class="absolute top-2 right-2 z-10 cursor-grab rounded p-1 text-gray-400 active:cursor-grabbing dark:text-gray-500"
								>
									⠿
								</div>
								<tile.component {...tile.props} />
							{/if}
						</div>
					{/each}
				</div>
			</section>
		{/each}
	{/snippet}
</AppShell>
