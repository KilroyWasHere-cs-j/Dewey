import { json } from '@sveltejs/kit';
import { backendUrl } from '$lib/server/backend';
import { proxyError, backendErrorMessage } from '$lib/server/apiError';
import type { RequestHandler } from './$types';

// /fileview/* is only gated by the backend's known_machines IP allowlist,
// not requirePassword (issue #389) — unlike /core/files/*, no auth header
// needs forwarding here.
export const GET: RequestHandler = async () => {
	try {
		const res = await fetch(`${backendUrl()}/fileview/viewLogDir`);
		if (!res.ok) {
			const message = await backendErrorMessage(res, 'Failed to list log files');
			return json({ error: message }, { status: res.status });
		}
		return json(await res.json());
	} catch (error) {
		return proxyError('Failed to list log files', error);
	}
};
