import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import { proxyError } from '$lib/server/apiError';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async () => {
	const backendUrl = env.BACKEND_URL ?? 'http://localhost:8080';

	try {
		const res = await fetch(`${backendUrl}/machines`);
		if (!res.ok) throw new Error(`HTTP ${res.status}`);
		return json(await res.json());
	} catch (error) {
		return proxyError('Failed to list machines', error);
	}
};

export const POST: RequestHandler = async ({ request }) => {
	const backendUrl = env.BACKEND_URL ?? 'http://localhost:8080';

	try {
		const body = await request.json();
		const res = await fetch(`${backendUrl}/machines`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(body)
		});

		const data = await res.json();
		return json(data, { status: res.status });
	} catch (error) {
		return proxyError('Failed to add machine', error);
	}
};
