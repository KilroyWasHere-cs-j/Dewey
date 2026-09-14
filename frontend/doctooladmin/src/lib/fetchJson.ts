// Shared client-side fetch helper (issue #432). Files, Machines, and Logs
// each hand-rolled their own "fetch → check res.ok → best-effort parse the
// error body → fallback to a bare status code" block, and the copies had
// already drifted (e.g. one call site skipped the error-body parse
// entirely, another didn't guard against a non-JSON error body). Routing
// every call through here keeps that behavior consistent by construction.
//
// Mirrors $lib/server/apiError.ts's backendErrorMessage, which does the
// same parse-with-fallback for the SvelteKit proxy routes server-side —
// this is the client-side equivalent for the pages themselves.

// Thrown by apiFetch/fetchJson on a non-ok response. Carries the parsed
// backend message (when available) as .message, and the HTTP status so
// callers can still special-case things like 401 (a rotated password,
// issue #414) without re-parsing anything themselves.
export class ApiError extends Error {
	status: number;

	constructor(message: string, status: number) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
	}
}

// Reads a non-ok response's JSON body and returns its `error` field if
// present, otherwise falls back. Guards against a non-JSON or empty body
// (e.g. a body-size-limit rejection) rather than assuming every error
// response is JSON.
async function parseErrorMessage(res: Response, fallback: string): Promise<string> {
	try {
		const body = await res.json();
		if (body && typeof body.error === 'string') return body.error;
	} catch {
		// not JSON, or empty body — fall through to the generic message
	}
	return fallback;
}

// Does fetch + the res.ok check + error-body parsing. Returns the raw
// Response on success so callers can read it as JSON, a blob, or text as
// their endpoint requires; throws ApiError on failure.
export async function apiFetch(url: string, init?: RequestInit): Promise<Response> {
	const res = await fetch(url, init);
	if (!res.ok) {
		const message = await parseErrorMessage(res, `HTTP ${res.status}`);
		throw new ApiError(message, res.status);
	}
	return res;
}

// Convenience wrapper for the common case of an endpoint whose success
// response body is JSON.
export async function fetchJson<T>(url: string, init?: RequestInit): Promise<T> {
	const res = await apiFetch(url, init);
	return res.json() as Promise<T>;
}
