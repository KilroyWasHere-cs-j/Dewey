import { json } from '@sveltejs/kit';
import { backendUrl } from '$lib/server/backend';
import { proxyError } from '$lib/server/apiError';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async () => {
	try {
		const res = await fetch(`${backendUrl()}/files`);
		if (!res.ok) throw new Error(`HTTP ${res.status}`);
		return json(await res.json());
	} catch (error) {
		return proxyError('Failed to list files', error);
	}
};

// Forward the multipart upload to the backend as-is.
// The Content-Type header (including the multipart boundary) must be passed through unchanged.
export const POST: RequestHandler = async ({ request }) => {
	try {
		const res = await fetch(`${backendUrl()}/upload`, {
			method: 'POST',
			body: request.body,
			headers: { 'Content-Type': request.headers.get('Content-Type') ?? '' },
			// @ts-expect-error — Node fetch needs duplex for streamed bodies
			duplex: 'half'
		});

		const body = await res.json();
		return json(body, { status: res.status });
	} catch (error) {
		return proxyError('Upload failed', error);
	}
};
