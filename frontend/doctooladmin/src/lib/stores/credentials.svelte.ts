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

export function createCredentialsStore() {
	let filesPassword = $state<string | null>(null);
	let machinesPassword = $state<string | null>(null);

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

		async getFilesPassword(): Promise<string> {
			if (filesPassword === null) {
				filesPassword = await promptForPassword('Files management password');
			}
			return filesPassword;
		},

		async getMachinesPassword(): Promise<string> {
			if (machinesPassword === null) {
				machinesPassword = await promptForPassword('Machine management password');
			}
			return machinesPassword;
		}
	};
}

// Singleton — all imports share the same reactive state instance
export const credentials = createCredentialsStore();
