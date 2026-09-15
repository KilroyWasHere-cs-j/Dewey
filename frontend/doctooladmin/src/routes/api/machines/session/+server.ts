import { json } from '@sveltejs/kit';
import { backendUrl, forwardPasswordHeader } from '$lib/server/backend';
import { proxyError, backendErrorMessage } from '$lib/server/apiError';
import type { RequestHandler } from './$types';

// Exchanges the machines password for a session token (issue #409) — the
// only /api/machines/* route that still forwards X-Dewey-Password rather
// than X-Dewey-Session-Token.
export const POST: RequestHandler = async ({ request }) => {
	try {
		const res = await fetch(`${backendUrl()}/core/machines/session`, {
			method: 'POST',
			headers: forwardPasswordHeader(request)
		});
		if (!res.ok) {
			const message = await backendErrorMessage(res, 'Failed to authenticate');
			return json({ error: message }, { status: res.status });
		}
		return json(await res.json());
	} catch (error) {
		return proxyError('Failed to authenticate', error);
	}
};
