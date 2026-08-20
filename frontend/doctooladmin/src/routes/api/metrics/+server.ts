import { json } from '@sveltejs/kit';
import { backendUrl } from '$lib/server/backend';
import { proxyError } from '$lib/server/apiError';
import type { RequestHandler } from './$types';
import type { AppMetrics } from '$lib/types';

// Metrics whose labels should be collapsed into a Record<label, value>
const LABELED_METRICS = new Set([
	'file_io_ops_total',
	'file_io_bytes_total',
	'file_io_duration_seconds_bucket',
	'go_gc_duration_seconds', // has quantile labels
	'gin_request_size_bytes_bucket',
	'gin_response_size_bytes_bucket',
	'promhttp_metric_handler_requests_total', // has code/method labels
	// new in #104
	'app_upload_rejections_total',
	'app_uploads_by_type_total'
]);

export const GET: RequestHandler = async () => {
	try {
		const response = await fetch(`${backendUrl()}/metrics`);
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
		return proxyError('Failed to fetch metrics', error);
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
