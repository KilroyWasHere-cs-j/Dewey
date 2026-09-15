// Files and machine management (issue #332) each require their own
// password. Rather than resending that password as a header on every
// single request (issue #409 — visible in the Network tab for every
// action, not just once), the password is exchanged once for a
// short-lived session token, and every subsequent request sends the
// token (X-Dewey-Session-Token) instead. Both the prompted password and
// the resulting token are held only in memory — never written to
// storage — so closing the tab or reloading requires re-entering the
// password.
//
// Prompting is done via a styled in-app modal (PasswordModal.svelte)
// rather than window.prompt() (issue #346), so getting a password is now
// async — the returned promise resolves once the modal is submitted or
// cancelled, same as window.prompt()'s blocking-until-dismissed behavior
// but without a native, unstyleable dialog.

import { ApiError, fetchJson } from '$lib/fetchJson';

export interface PasswordRequest {
	label: string;
	submit: (value: string) => void;
	cancel: () => void;
}

// Issue #351: an authenticated session shouldn't live forever just because
// the tab stays open. One shared timer covers both tokens rather than a
// timer per token — activity anywhere in the app (not just the page
// currently gated by a given token) counts as "still here," so working on
// Files doesn't let a Machines session quietly expire in the background.
// Independent of (and tighter than) the backend's own 5-minute sliding
// session expiry (issue #409) — this is purely a client-side UX nicety
// that clears the cached token early so a walked-away-from tab re-prompts
// promptly; the backend enforces the real expiry regardless.
const IDLE_TIMEOUT_MS = 90_000;

// Exchanges password for a session token via endpoint (issue #409).
// Throws the same ApiError every fetchJson caller elsewhere in the app
// throws — including on a wrong password (401) — so a caller's existing
// `if (e instanceof ApiError && e.status === 401)` handling covers a
// failed exchange exactly the same way it already covers a token that
// turned out to be stale.
async function exchangeForToken(endpoint: string, password: string): Promise<string> {
	const data = await fetchJson<{ token?: string }>(endpoint, {
		method: 'POST',
		headers: { 'X-Dewey-Password': password }
	});
	if (!data.token) throw new ApiError('Session exchange returned no token', 500);
	return data.token;
}

export function createCredentialsStore() {
	let filesToken = $state<string | null>(null);
	let machinesToken = $state<string | null>(null);

	let idleTimer: ReturnType<typeof setTimeout> | undefined;

	// Re-locks both pages by clearing their cached tokens — clearing an
	// already-null value is a harmless no-op, so this doesn't need to know
	// which token(s) are actually set.
	function expireTokens() {
		filesToken = null;
		machinesToken = null;
	}

	function resetIdleTimer() {
		clearTimeout(idleTimer);
		idleTimer = setTimeout(expireTokens, IDLE_TIMEOUT_MS);
	}

	// Guard for SSR — this module is imported during server rendering too,
	// where `document` doesn't exist.
	if (typeof document !== 'undefined') {
		document.addEventListener('mousemove', resetIdleTimer);
		document.addEventListener('keydown', resetIdleTimer);
		resetIdleTimer();
	}

	// The modal (mounted once, in +layout.svelte) reads this to know what to
	// show; null means no prompt is currently pending.
	let request = $state<PasswordRequest | null>(null);

	function promptForPassword(label: string): Promise<string> {
		return new Promise((resolve) => {
			request = {
				label,
				submit: (value: string) => {
					request = null;
					resolve(value);
				},
				// Mirrors window.prompt()'s cancel behavior: resolves to '',
				// which the backend's session exchange endpoint then
				// correctly rejects (same fail-closed-on-empty check the old
				// per-request password comparison always had).
				cancel: () => {
					request = null;
					resolve('');
				}
			};
		});
	}

	return {
		get request() {
			return request;
		},

		// Reactive presence checks — pages derive their "authorized" state from
		// these (rather than latching a one-time success flag) so an idle-timer
		// expiry re-locks an already-authorized page immediately, not just on
		// the next manual retry.
		get hasFilesToken() {
			return filesToken !== null;
		},

		get hasMachinesToken() {
			return machinesToken !== null;
		},

		async getFilesToken(): Promise<string> {
			if (filesToken === null) {
				const password = await promptForPassword('Files management password');
				filesToken = await exchangeForToken('/api/files/session', password);
			}
			return filesToken;
		},

		// Clears the cached token so the next getFilesToken() call re-prompts
		// and re-exchanges, instead of reusing a stale or cancelled ('')
		// attempt forever.
		resetFilesToken() {
			filesToken = null;
		},

		async getMachinesToken(): Promise<string> {
			if (machinesToken === null) {
				const password = await promptForPassword('Machine management password');
				machinesToken = await exchangeForToken('/api/machines/session', password);
			}
			return machinesToken;
		},

		resetMachinesToken() {
			machinesToken = null;
		}
	};
}

// Singleton — all imports share the same reactive state instance
export const credentials = createCredentialsStore();
