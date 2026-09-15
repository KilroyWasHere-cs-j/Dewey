import { json } from '@sveltejs/kit';
import { backendUrl, forwardSessionHeader } from '$lib/server/backend';
import { proxyError } from '$lib/server/apiError';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request }) => {
	try {
		const res = await fetch(`${backendUrl()}/core/machines`, { headers: forwardSessionHeader(request) });
		if (!res.ok) throw new Error(`HTTP ${res.status}`);
		return json(await res.json());
	} catch (error) {
		return proxyError('Failed to list machines', error);
	}
};

export const POST: RequestHandler = async ({ request }) => {
	try {
		const body = await request.json();
		const res = await fetch(`${backendUrl()}/core/machines`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json', ...forwardSessionHeader(request) },
			body: JSON.stringify(body)
		});

		const data = await res.json();
		return json(data, { status: res.status });
	} catch (error) {
		return proxyError('Failed to add machine', error);
	}
};
