import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';

// GET ?meta=true  → returns MetaData JSON from the backend
// GET ?meta=false → streams the raw file back as an attachment download
export const GET: RequestHandler = async ({ params, url }) => {
	const backendUrl = env.BACKEND_URL ?? 'http://localhost:8080';
	const meta = url.searchParams.get('meta') === 'true' ? 'true' : 'false';
	const filename = params.filename;

	try {
		const res = await fetch(`${backendUrl}/files/${encodeURIComponent(filename)}/${meta}`);

		if (!res.ok) {
			return json({ error: 'File not found' }, { status: res.status });
		}

		if (meta === 'true') {
			return json(await res.json());
		}

		// Stream the binary file through with a download header
		return new Response(res.body, {
			headers: {
				'Content-Type': res.headers.get('Content-Type') ?? 'application/octet-stream',
				'Content-Disposition': `attachment; filename="${filename}"`
			}
		});
	} catch (error) {
		return json({ error: 'Request failed', details: String(error) }, { status: 500 });
	}
};

export const DELETE: RequestHandler = async ({ params }) => {
	const backendUrl = env.BACKEND_URL ?? 'http://localhost:8080';
	const filename = params.filename;

	try {
		const res = await fetch(`${backendUrl}/files/${encodeURIComponent(filename)}`, {
			method: 'DELETE'
		});

		if (!res.ok) {
			return json({ error: 'Delete failed' }, { status: res.status });
		}

		return json({ ok: true });
	} catch (error) {
		return json({ error: 'Request failed', details: String(error) }, { status: 500 });
	}
};
