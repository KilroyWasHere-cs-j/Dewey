export interface Toast {
	id: number;
	kind: 'error' | 'warning' | 'info';
	title: string;
	detail?: string | null;
	// Optional single action button (e.g. "Undo") rendered next to dismiss.
	action?: { label: string; onClick: () => void };
}

// Non-error toasts clear themselves after this long; errors stay until dismissed
// since they usually need the user to actually read and act on them.
const AUTO_DISMISS_MS = 6000;

// Toasts with an action button (e.g. "Undo") get longer before clearing —
// reading the message and physically clicking a button takes more time than
// just reading it.
const ACTION_AUTO_DISMISS_MS = 15000;

export function createToastStore() {
	let items = $state<Toast[]>([]);
	let nextId = 0;

	function push(t: Omit<Toast, 'id'>): number {
		const id = nextId++;
		items = [...items, { ...t, id }];

		if (t.kind !== 'error') {
			const delay = t.action ? ACTION_AUTO_DISMISS_MS : AUTO_DISMISS_MS;
			setTimeout(() => dismiss(id), delay);
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
