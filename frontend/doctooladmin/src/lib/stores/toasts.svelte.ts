export interface Toast {
	id: number;
	kind: 'error' | 'warning' | 'info';
	title: string;
	detail?: string | null;
}

// Non-error toasts clear themselves after this long; errors stay until dismissed
// since they usually need the user to actually read and act on them.
const AUTO_DISMISS_MS = 6000;

export function createToastStore() {
	let items = $state<Toast[]>([]);
	let nextId = 0;

	function push(t: Omit<Toast, 'id'>): number {
		const id = nextId++;
		items = [...items, { ...t, id }];

		if (t.kind !== 'error') {
			setTimeout(() => dismiss(id), AUTO_DISMISS_MS);
		}

		return id;
	}

	function dismiss(id: number) {
		items = items.filter((t) => t.id !== id);
	}

	return {
		get items(): Toast[] {
			return items;
		},
		push,
		dismiss
	};
}

// Singleton — all imports share the same reactive state instance
export const toasts = createToastStore();
