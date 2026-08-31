import { json } from '@sveltejs/kit';

// Logs the full error server-side and returns a generic message to the
// client, so internal details (backend host/port, connection-failure
// semantics, stack traces) never leak through a proxy route (issue #209).
export function proxyError(message: string, error: unknown, status = 500) {
	console.error(message, error);
	return json({ error: message }, { status });
}

// Reads a non-ok backend response's JSON body and returns its `error`
// field if it's a clean string, otherwise falls back to a generic message.
// Only ever forwards that one field — never the raw body — so real
// backend-supplied reasons (e.g. "invalid credentials") reach the client
// without also leaking whatever else the backend response might contain
// (issue #353, keeping #209's intent for anything that isn't that shape).
export async function backendErrorMessage(res: Response, fallback: string): Promise<string> {
	try {
		const body = await res.json();
		if (body && typeof body.error === 'string') return body.error;
	} catch {
		// not JSON, or empty body — fall through to the generic message
	}
	return fallback;
}
