import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async () => {
	const backendUrl = env.BACKEND_URL ?? 'http://localhost:8080';

	try {
		const res = await fetch(`${backendUrl}/files`);
		if (!res.ok) throw new Error(`HTTP ${res.status}`);
		return json(await res.json());
	} catch (error) {
		return json({ error: 'Failed to list files', details: String(error) }, { status: 500 });
	}
};

// Forward the multipart upload to the backend as-is.
// The Content-Type header (including the multipart boundary) must be passed through unchanged.
export const POST: RequestHandler = async ({ request }) => {
	const backendUrl = env.BACKEND_URL ?? 'http://localhost:8080';

	try {
		const res = await fetch(`${backendUrl}/upload`, {
			method: 'POST',
			body: request.body,
			headers: { 'Content-Type': request.headers.get('Content-Type') ?? '' },
			// @ts-expect-error — Node fetch needs duplex for streamed bodies
			duplex: 'half'
		});

		const body = await res.json();
		return json(body, { status: res.status });
	} catch (error) {
		return json({ error: 'Upload failed', details: String(error) }, { status: 500 });
	}
};
