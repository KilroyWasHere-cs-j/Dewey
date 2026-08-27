// Files and machine management (issue #332) each require their own
// password, sent as the X-Dewey-Password header. Prompted once per
// browser session and held only in memory — never written to storage —
// so closing the tab or reloading requires re-entering it.
//
// Prompting is done via a styled in-app modal (PasswordModal.svelte)
// rather than window.prompt() (issue #346), so getting a password is now
// async — the returned promise resolves once the modal is submitted or
// cancelled, same as window.prompt()'s blocking-until-dismissed behavior
// but without a native, unstyleable dialog.

export interface PasswordRequest {
	label: string;
	submit: (value: string) => void;
	cancel: () => void;
}

// Issue #351: an authenticated password shouldn't live forever just because
// the tab stays open. One shared timer covers both passwords rather than a
// timer per password — activity anywhere in the app (not just the page
// currently gated by a given password) counts as "still here," so working
// on Files doesn't let a Machines password quietly expire in the background.
const IDLE_TIMEOUT_MS = 90_000;

export function createCredentialsStore() {
	let filesPassword = $state<string | null>(null);
	let machinesPassword = $state<string | null>(null);

	let idleTimer: ReturnType<typeof setTimeout> | undefined;

	// Re-locks both pages by clearing their cached passwords — clearing an
	// already-null value is a harmless no-op, so this doesn't need to know
	// which password(s) are actually set.
	function expirePasswords() {
		filesPassword = null;
		machinesPassword = null;
	}

	function resetIdleTimer() {
		clearTimeout(idleTimer);
		idleTimer = setTimeout(expirePasswords, IDLE_TIMEOUT_MS);
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
				// which the backend's requirePassword then correctly rejects.
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
		get hasFilesPassword() {
			return filesPassword !== null;
		},

		get hasMachinesPassword() {
			return machinesPassword !== null;
		},

		async getFilesPassword(): Promise<string> {
			if (filesPassword === null) {
				filesPassword = await promptForPassword('Files management password');
			}
			return filesPassword;
		},

		// Clears the cached value so the next getFilesPassword() call prompts
		// again instead of reusing a wrong or cancelled ('') attempt forever.
		resetFilesPassword() {
			filesPassword = null;
		},

		async getMachinesPassword(): Promise<string> {
			if (machinesPassword === null) {
				machinesPassword = await promptForPassword('Machine management password');
			}
			return machinesPassword;
		},

		resetMachinesPassword() {
			machinesPassword = null;
		}
	};
}

// Singleton — all imports share the same reactive state instance
export const credentials = createCredentialsStore();
