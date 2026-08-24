import { json } from '@sveltejs/kit';
import { backendUrl } from '$lib/server/backend';
import { proxyError } from '$lib/server/apiError';
import { parsePrometheusMetrics } from '$lib/server/metricsParser';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async () => {
	try {
		const response = await fetch(`${backendUrl()}/metrics`);
		if (!response.ok) throw new Error(`HTTP ${response.status}`);

		const text = await response.text();
		return json(parsePrometheusMetrics(text));
	} catch (error) {
		return proxyError('Failed to fetch metrics', error);
	}
};
