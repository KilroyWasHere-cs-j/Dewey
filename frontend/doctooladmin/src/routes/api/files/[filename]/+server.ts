import { json } from '@sveltejs/kit';
import { backendUrl, forwardAuthHeader } from '$lib/server/backend';
import { proxyError } from '$lib/server/apiError';
import type { RequestHandler } from './$types';

// GET ?meta=true  → returns MetaData JSON from the backend
// GET ?meta=false → streams the raw file back as an attachment download
export const GET: RequestHandler = async ({ params, url, request }) => {
	const meta = url.searchParams.get('meta') === 'true' ? 'true' : 'false';
	const filename = params.filename;

	try {
		const res = await fetch(`${backendUrl()}/core/files/${encodeURIComponent(filename)}/${meta}`, {
			headers: forwardAuthHeader(request)
		});

		if (!res.ok) {
			return json({ error: 'File not found' }, { status: res.status });
		}

		if (meta === 'true') {
			return json(await res.json());
		}

		// Escape backslashes/quotes (RFC 2616 quoted-string) so a filename
		// containing a `"` can't break out of the quoted param and inject
		// extra Content-Disposition directives (issue #208).
		const safeFilename = filename.replace(/[\\"]/g, '\\$&');

		// Stream the binary file through with a download header
		return new Response(res.body, {
			headers: {
				'Content-Type': res.headers.get('Content-Type') ?? 'application/octet-stream',
				'Content-Disposition': `attachment; filename="${safeFilename}"`
			}
		});
	} catch (error) {
		return proxyError('Request failed', error);
	}
};

export const DELETE: RequestHandler = async ({ params, request }) => {
	const filename = params.filename;

	try {
		const res = await fetch(`${backendUrl()}/core/files/${encodeURIComponent(filename)}`, {
			method: 'DELETE',
			headers: forwardAuthHeader(request)
		});

		if (!res.ok) {
			return json({ error: 'Delete failed' }, { status: res.status });
		}

		return json({ ok: true });
	} catch (error) {
		return proxyError('Request failed', error);
	}
};
