// +page.ts
import type { AppMetrics } from './api/metrics/+server';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const res = await fetch('/api/metrics');
	if (!res.ok) throw new Error('Failed to load metrics');
	const metrics: Partial<AppMetrics> = await res.json();
	return { metrics };
};
