import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, apiFetch, fetchJson } from './fetchJson';

// Builds a minimal Response-like stub — real Response works fine under
// vitest's node environment, this just keeps each test's setup terse.
function stubResponse(ok: boolean, status: number, body: unknown, isJson = true) {
	return {
		ok,
		status,
		json: async () => {
			if (!isJson) throw new SyntaxError('Unexpected token');
			return body;
		}
	} as Response;
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('fetchJson', () => {
	it('returns the parsed JSON body on success', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => stubResponse(true, 200, { files: ['a.pdf'] }))
		);
		await expect(fetchJson('/api/files')).resolves.toEqual({ files: ['a.pdf'] });
	});

	it('throws ApiError with the backend-provided message on failure', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => stubResponse(false, 401, { error: 'invalid credentials' }))
		);
		await expect(fetchJson('/api/files')).rejects.toMatchObject({
			message: 'invalid credentials',
			status: 401
		});
	});

	it('falls back to a bare status code when the error body is not JSON', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => stubResponse(false, 413, null, false))
		);
		await expect(fetchJson('/api/files')).rejects.toMatchObject({
			message: 'HTTP 413',
			status: 413
		});
	});

	it('falls back to a bare status code when the error body has no .error field', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => stubResponse(false, 500, { unrelated: true }))
		);
		await expect(fetchJson('/api/files')).rejects.toMatchObject({
			message: 'HTTP 500',
			status: 500
		});
	});

	it('rejected calls throw an instance of ApiError', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => stubResponse(false, 404, { error: 'not found' }))
		);
		await expect(fetchJson('/api/files')).rejects.toBeInstanceOf(ApiError);
	});
});

describe('apiFetch', () => {
	it('returns the raw Response on success, for callers that need .blob()/.text()', async () => {
		const res = stubResponse(true, 200, {});
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => res)
		);
		await expect(apiFetch('/api/files/report.pdf')).resolves.toBe(res);
	});
});
