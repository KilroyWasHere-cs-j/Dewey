import { json } from '@sveltejs/kit';
import { backendUrl, forwardAuthHeader } from '$lib/server/backend';
import { proxyError, backendErrorMessage } from '$lib/server/apiError';
import type { RequestHandler } from './$types';

export const DELETE: RequestHandler = async ({ params, request }) => {
	const ip = params.ip;

	try {
		const res = await fetch(`${backendUrl()}/core/machines/${encodeURIComponent(ip)}`, {
			method: 'DELETE',
			headers: forwardAuthHeader(request)
		});

		if (!res.ok) {
			const message = await backendErrorMessage(res, 'Delete failed');
			return json({ error: message }, { status: res.status });
		}

		return json({ ok: true });
	} catch (error) {
		return proxyError('Request failed', error);
	}
};
