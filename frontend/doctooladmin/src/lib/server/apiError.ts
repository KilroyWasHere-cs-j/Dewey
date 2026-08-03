import { json } from '@sveltejs/kit';

// Logs the full error server-side and returns a generic message to the
// client, so internal details (backend host/port, connection-failure
// semantics, stack traces) never leak through a proxy route (issue #209).
export function proxyError(message: string, error: unknown, status = 500) {
	console.error(message, error);
	return json({ error: message }, { status });
}
