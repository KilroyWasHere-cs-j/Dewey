import { json } from '@sveltejs/kit';
import { backendUrl, forwardSessionHeader } from '$lib/server/backend';
import { proxyError, backendErrorMessage } from '$lib/server/apiError';
import type { RequestHandler } from './$types';

// GET ?meta=true  → returns MetaData JSON from the backend
// GET ?meta=false → streams the raw file back as an attachment download
export const GET: RequestHandler = async ({ params, url, request }) => {
	const meta = url.searchParams.get('meta') === 'true' ? 'true' : 'false';
	const filename = params.filename;

	try {
		// meta is a query param on the backend now, not a path segment — a path
		// segment there would collide with filenames that contain their own "/"
		// (category-nested paths from listFiles, e.g. "Swedish/image/foo.png").
		const res = await fetch(
			`${backendUrl()}/core/files/${encodeURIComponent(filename)}?meta=${meta}`,
			{ headers: forwardSessionHeader(request) }
		);

		if (!res.ok) {
			const message = await backendErrorMessage(res, 'File not found');
			return json({ error: message }, { status: res.status });
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
			headers: forwardSessionHeader(request)
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

// POST → undelete: clears the backend record's soft-delete flag (issue #324).
// Used by the file manager page's post-delete "Undo" toast.
export const POST: RequestHandler = async ({ params, request }) => {
	const filename = params.filename;

	try {
		const res = await fetch(
			`${backendUrl()}/core/files/undelete/${encodeURIComponent(filename)}`,
			{ method: 'POST', headers: forwardSessionHeader(request) }
		);

		if (!res.ok) {
			const message = await backendErrorMessage(res, 'Undelete failed');
			return json({ error: message }, { status: res.status });
		}

		return json({ ok: true });
	} catch (error) {
		return proxyError('Request failed', error);
	}
};
