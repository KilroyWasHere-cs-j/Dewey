// Files and machine management (issue #332) each require their own
// password, sent as the X-Dewey-Password header. Prompted once per
// browser session and held only in memory — never written to storage —
// so closing the tab or reloading requires re-entering it.

export function createCredentialsStore() {
	let filesPassword = $state<string | null>(null);
	let machinesPassword = $state<string | null>(null);

	return {
		getFilesPassword(): string {
			if (filesPassword === null) {
				filesPassword = window.prompt('Files management password:') ?? '';
			}
			return filesPassword;
		},

		getMachinesPassword(): string {
			if (machinesPassword === null) {
				machinesPassword = window.prompt('Machine management password:') ?? '';
			}
			return machinesPassword;
		}
	};
}

// Singleton — all imports share the same reactive state instance
export const credentials = createCredentialsStore();
