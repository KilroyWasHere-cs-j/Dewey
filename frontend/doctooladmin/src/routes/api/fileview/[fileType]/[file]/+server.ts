import { text, json } from '@sveltejs/kit';
import { backendUrl } from '$lib/server/backend';
import { proxyError, backendErrorMessage } from '$lib/server/apiError';
import type { RequestHandler } from './$types';

// Backend's GET /fileview/viewFile/:fileType/:file returns raw text on
// success (c.String, not JSON) — passed through as-is rather than wrapped.
export const GET: RequestHandler = async ({ params }) => {
	try {
		const res = await fetch(
			`${backendUrl()}/fileview/viewFile/${encodeURIComponent(params.fileType)}/${encodeURIComponent(params.file)}`
		);
		if (!res.ok) {
			const message = await backendErrorMessage(res, 'Failed to load file');
			return json({ error: message }, { status: res.status });
		}
		return text(await res.text());
	} catch (error) {
		return proxyError('Failed to load file', error);
	}
};
