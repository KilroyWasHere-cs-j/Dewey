import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import { proxyError } from '$lib/server/apiError';
import type { RequestHandler } from './$types';

export const DELETE: RequestHandler = async ({ params }) => {
	const backendUrl = env.BACKEND_URL ?? 'http://localhost:8080';
	const ip = params.ip;

	try {
		const res = await fetch(`${backendUrl}/machines/${encodeURIComponent(ip)}`, {
			method: 'DELETE'
		});

		if (!res.ok) {
			return json({ error: 'Delete failed' }, { status: res.status });
		}

		return json({ ok: true });
	} catch (error) {
		return proxyError('Request failed', error);
	}
};
