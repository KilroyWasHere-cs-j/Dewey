import { env } from '$env/dynamic/private';

// Shared by every +server.ts proxy route so the localhost fallback (used in
// local dev when BACKEND_URL isn't set) only lives in one place (issue #219).
export function backendUrl(): string {
	return env.BACKEND_URL ?? 'http://localhost:8080';
}

// Files/machines management routes require a password (issue #332). The
// browser sends it as a header on its request to this SvelteKit route;
// this just passes it through unchanged to the backend request, so the
// actual secret never has to live in this server's own env.
export function forwardAuthHeader(request: Request): HeadersInit {
	return { 'X-Dewey-Password': request.headers.get('X-Dewey-Password') ?? '' };
}
