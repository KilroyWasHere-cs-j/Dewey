// Shared response-shape types for the backend API routes.
// Lives in $lib so components can depend on it without reaching into
// routes/api/* (routes are supposed to depend on $lib, not the reverse).

export interface AppVersionInfo {
	version: string;
	git_branch: string;
}

export interface AppMetrics {
	// App-specific
	app_cache_size: number;
	app_cpu_count: number;
	app_exe_count: number;
	app_file_copys: number;
	app_file_deletions: number;
	app_file_retrievals: number;
	app_file_sorts: number;
	app_filters_loadings: number;
	app_files_in_backup: number;
	app_files_in_store: number;
	app_gc_cycles: number;
	app_heap_usage: number;
	app_ram_usage: number;
	app_time_til_next_tick: number;
	app_uptime_seconds: number;

	// Upload tracking (new in #104)
	app_upload_rejections_total: Record<string, number>;
	app_uploads_by_type_total: Record<string, number>;
	app_upload_size_bytes_sum: number;
	app_upload_size_bytes_count: number;

	// Processing pipeline (new in #104)
	app_barcode_successes: number;
	app_barcode_failures: number;
	app_plugin_runs: number;
	app_plugin_errors: number;
	app_db_errors: number;

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
