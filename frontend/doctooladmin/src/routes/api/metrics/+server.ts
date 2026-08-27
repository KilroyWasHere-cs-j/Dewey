import { json } from '@sveltejs/kit';
import { backendUrl } from '$lib/server/backend';
import { proxyError } from '$lib/server/apiError';
import { parsePrometheusMetrics } from '$lib/server/metricsParser';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async () => {
	try {
		const response = await fetch(`${backendUrl()}/metrics`);
		// Forward the backend's actual status (e.g. 429 from its rate limiter)
		// instead of collapsing every non-ok response to a generic 500, so the
		// frontend can tell "rate limited" apart from a real outage (issue #362).
		if (!response.ok) {
			return proxyError('Failed to fetch metrics', new Error(`HTTP ${response.status}`), response.status);
		}

		const text = await response.text();
		return json(parsePrometheusMetrics(text));
	} catch (error) {
		return proxyError('Failed to fetch metrics', error);
	}
};
