import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import { proxyError } from '$lib/server/apiError';
import type { RequestHandler } from './$types';

export interface AppVersionInfo {
	version: string;
	git_branch: string;
}

export const GET: RequestHandler = async () => {
	const backendUrl = env.BACKEND_URL ?? 'http://localhost:8080';

	try {
		const res = await fetch(`${backendUrl}/version`);
		if (!res.ok) throw new Error(`HTTP ${res.status}`);
		return json(await res.json());
	} catch (error) {
		return proxyError('Failed to fetch version info', error);
	}
};
