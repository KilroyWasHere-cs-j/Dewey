import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export interface AppMetrics {
	// App-specific
	app_cache_size: number;
	app_cpu_count: number;
	app_exe_count: number;
	app_file_copys: number;
	app_file_retries: number;
	app_file_sorts: number;
	app_files_in_backup: number;
	app_files_in_store: number;
	app_gc_cycles: number;
	app_heap_usage: number;
	app_ram_usage: number;
	app_time_til_next_tick: number;
	app_uptime_seconds: number;

	// File IO (labeled)
	file_io_ops_total: Record<string, number>;
	file_io_bytes_total: Record<string, number>;
	file_io_duration_seconds_bucket: Record<string, number>;
	file_io_duration_seconds_sum: number;
	file_io_duration_seconds_count: number;

	// Go runtime
	go_gc_duration_seconds: number;
	go_gc_duration_seconds_sum: number;
	go_gc_duration_seconds_count: number;
	go_gc_gogc_percent: number;
	go_gc_gomemlimit_bytes: number;
	go_goroutines: number;
	go_info: number;
	go_threads: number;
	go_memstats_alloc_bytes: number;
	go_memstats_alloc_bytes_total: number;
	go_memstats_buck_hash_sys_bytes: number;
	go_memstats_frees_total: number;
	go_memstats_gc_sys_bytes: number;
	go_memstats_heap_alloc_bytes: number;
	go_memstats_heap_idle_bytes: number;
	go_memstats_heap_inuse_bytes: number;
	go_memstats_heap_objects: number;
	go_memstats_heap_released_bytes: number;
	go_memstats_heap_sys_bytes: number;
	go_memstats_last_gc_time_seconds: number;
	go_memstats_mallocs_total: number;
	go_memstats_mcache_inuse_bytes: number;
	go_memstats_mcache_sys_bytes: number;
	go_memstats_mspan_inuse_bytes: number;
	go_memstats_mspan_sys_bytes: number;
	go_memstats_next_gc_bytes: number;
	go_memstats_other_sys_bytes: number;
	go_memstats_stack_inuse_bytes: number;
	go_memstats_stack_sys_bytes: number;
	go_memstats_sys_bytes: number;
	go_sched_gomaxprocs_threads: number;

	// Gin HTTP
	gin_request_size_bytes_sum: number;
	gin_request_size_bytes_count: number;
	gin_response_size_bytes_sum: number;
	gin_response_size_bytes_count: number;

	// Process
	process_cpu_seconds_total: number;
	process_max_fds: number;
	process_network_receive_bytes_total: number;
	process_network_transmit_bytes_total: number;
	process_open_fds: number;
	process_resident_memory_bytes: number;
	process_start_time_seconds: number;
	process_virtual_memory_bytes: number;
	process_virtual_memory_max_bytes: number;

	// Prometheus HTTP handler
	promhttp_metric_handler_requests_in_flight: number;
	promhttp_metric_handler_requests_total: number;

	// Index signature for any unlisted labeled metrics
	[key: string]: number | Record<string, number> | undefined;
}

// Metrics whose labels should be collapsed into a Record<label, value>
const LABELED_METRICS = new Set([
	'file_io_ops_total',
	'file_io_bytes_total',
	'file_io_duration_seconds_bucket',
	'go_gc_duration_seconds', // has quantile labels
	'gin_request_size_bytes_bucket',
	'gin_response_size_bytes_bucket',
	'promhttp_metric_handler_requests_total' // has code/method labels
]);

export const GET: RequestHandler = async () => {
	try {
		const response = await fetch('http://localhost:8080/metrics');
		if (!response.ok) throw new Error(`HTTP ${response.status}`);

		const text = await response.text();
		const metrics: Partial<AppMetrics> = {};

		for (const line of text.split('\n')) {
			const trimmed = line.trim();
			if (!trimmed || trimmed.startsWith('#')) continue;

			const spaceIndex = trimmed.lastIndexOf(' ');
			if (spaceIndex === -1) continue;

			const keyPart = trimmed.slice(0, spaceIndex).trim();
			const valueStr = trimmed.slice(spaceIndex + 1).trim();
			const value = parseFloat(valueStr);
			if (isNaN(value)) continue;

			// Split metric name from labels
			const braceStart = keyPart.indexOf('{');
			const metricName = braceStart > 0 ? keyPart.slice(0, braceStart) : keyPart;
			const cleanName = metricName.replace(/^myapp_/, '');

			const labelMatch = keyPart.match(/\{(.+)\}/);

			if (labelMatch && LABELED_METRICS.has(cleanName)) {
				// Build a label key string like "op=read" or "quantile=0.5,code=200"
				const labelKey = buildLabelKey(labelMatch[1]);

				if (!(metrics[cleanName] as Record<string, number>)) {
					(metrics as any)[cleanName] = {};
				}
				(metrics[cleanName] as Record<string, number>)[labelKey] = value;
			} else if (!labelMatch) {
				// Plain scalar metric
				(metrics as any)[cleanName] = value;
			}
			// Labeled metrics NOT in LABELED_METRICS are silently skipped —
			// add them to the set above if you need them.
		}

		return json(metrics);
	} catch (error) {
		console.error('Failed to fetch metrics:', error);
		return json(
			{
				error: 'Failed to fetch metrics',
				details: error instanceof Error ? error.message : String(error)
			},
			{ status: 500 }
		);
	}
};

/**
 * Converts a Prometheus label string like `op="read",foo="bar"`
 * into a compact key like `op=read` (single label) or `op=read,foo=bar` (multi).
 */
function buildLabelKey(labelStr: string): string {
	const pairs = [...labelStr.matchAll(/(\w+)="([^"]+)"/g)];
	if (pairs.length === 1) {
		// Single label — just use the value for cleaner keys e.g. "read", "write"
		return pairs[0][2];
	}
	return pairs.map(([, k, v]) => `${k}=${v}`).join(',');
}
