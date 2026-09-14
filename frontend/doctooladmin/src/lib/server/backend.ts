import { env } from '$env/dynamic/private';

// Shared by every +server.ts proxy route so the localhost fallback (used in
// local dev when BACKEND_URL isn't set) only lives in one place (issue #219).
export function backendUrl(): string {
	return env.BACKEND_URL ?? 'http://localhost:8080';
}

// Files/machines management routes require a session token (issue #409 —
// exchanged once for the password, rather than resending the password
// itself on every request). The browser sends it as a header on its
// request to this SvelteKit route; this just passes it through unchanged
// to the backend request, so the token never has to live in this
// server's own env.
export function forwardSessionHeader(request: Request): HeadersInit {
	return { 'X-Dewey-Session-Token': request.headers.get('X-Dewey-Session-Token') ?? '' };
}

// Used only by the two /api/*/session exchange routes (issue #409) — the
// one place the actual password still crosses the wire, once per login,
// to be traded for a session token.
export function forwardPasswordHeader(request: Request): HeadersInit {
	return { 'X-Dewey-Password': request.headers.get('X-Dewey-Password') ?? '' };
}
