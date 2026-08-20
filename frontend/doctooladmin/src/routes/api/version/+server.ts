import { json } from '@sveltejs/kit';
import { backendUrl } from '$lib/server/backend';
import { proxyError } from '$lib/server/apiError';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async () => {
	try {
		const res = await fetch(`${backendUrl()}/version`);
		if (!res.ok) throw new Error(`HTTP ${res.status}`);
		return json(await res.json());
	} catch (error) {
		return proxyError('Failed to fetch version info', error);
	}
};
