import { env } from '$env/dynamic/private';

// Shared by every +server.ts proxy route so the localhost fallback (used in
// local dev when BACKEND_URL isn't set) only lives in one place (issue #219).
export function backendUrl(): string {
	return env.BACKEND_URL ?? 'http://localhost:8080';
}
