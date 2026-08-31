import { beforeEach, describe, expect, it, vi } from 'vitest';

// $app/environment's `browser` export is resolved by SvelteKit's vite
// plugin based on build target — mocked explicitly here (vi.mock is
// hoisted above the import below) rather than relied on, so these tests
// exercise the same "browser" code path regardless of how that resolves
// under vitest.
vi.mock('$app/environment', () => ({ browser: true }));

// Tests run under vitest's default node environment (no jsdom — see
// vitest.config.ts), which has no real localStorage. Stubbed with a
// minimal, correct in-memory Storage instead.
class MemoryStorage implements Storage {
	private store = new Map<string, string>();
	get length() {
		return this.store.size;
	}
	clear() {
		this.store.clear();
	}
	getItem(key: string) {
		return this.store.has(key) ? this.store.get(key)! : null;
	}
	key(index: number) {
		return [...this.store.keys()][index] ?? null;
	}
	removeItem(key: string) {
		this.store.delete(key);
	}
	setItem(key: string, value: string) {
		this.store.set(key, value);
	}
}
vi.stubGlobal('localStorage', new MemoryStorage());

import { createSettingsStore, DEFAULTS, resolveTileOrder } from './settings.svelte';

const STORAGE_KEY = 'dewey-settings';

describe('settings store', () => {
	beforeEach(() => {
		localStorage.clear();
	});

	it('loads defaults when localStorage is empty', () => {
		const store = createSettingsStore();
		expect(store.value).toEqual(DEFAULTS);
	});

	it('falls back to defaults when localStorage holds corrupt JSON', () => {
		localStorage.setItem(STORAGE_KEY, '{not valid json');
		const store = createSettingsStore();
		expect(store.value).toEqual(DEFAULTS);
	});

	it('merges stored values over defaults on load', () => {
		localStorage.setItem(STORAGE_KEY, JSON.stringify({ darkMode: true, pollIntervalMs: 9000 }));
		const store = createSettingsStore();
		expect(store.value).toEqual({ ...DEFAULTS, darkMode: true, pollIntervalMs: 9000 });
	});

	it('update() merges a patch into the current value and persists it', () => {
		const store = createSettingsStore();

		store.update({ darkMode: true });

		expect(store.value).toEqual({ ...DEFAULTS, darkMode: true });
		expect(JSON.parse(localStorage.getItem(STORAGE_KEY)!)).toEqual({ ...DEFAULTS, darkMode: true });
	});

	it('reset() restores defaults and clears localStorage', () => {
		const store = createSettingsStore();

		store.update({ darkMode: true, pollIntervalMs: 1234 });
		store.reset();

		expect(store.value).toEqual(DEFAULTS);
		expect(localStorage.getItem(STORAGE_KEY)).toBeNull();
	});
});

describe('resolveTileOrder', () => {
	it('returns the default order when nothing is saved', () => {
		expect(resolveTileOrder(['a', 'b', 'c'], undefined)).toEqual(['a', 'b', 'c']);
		expect(resolveTileOrder(['a', 'b', 'c'], [])).toEqual(['a', 'b', 'c']);
	});

	it('uses the saved order when it matches the default set', () => {
		expect(resolveTileOrder(['a', 'b', 'c'], ['c', 'a', 'b'])).toEqual(['c', 'a', 'b']);
	});

	it('drops saved ids no longer present in the defaults', () => {
		expect(resolveTileOrder(['a', 'b'], ['a', 'removed', 'b'])).toEqual(['a', 'b']);
	});

	it('appends default ids missing from a stale saved order', () => {
		expect(resolveTileOrder(['a', 'b', 'c'], ['b', 'a'])).toEqual(['b', 'a', 'c']);
	});

	it('survives a save (settings.update) and reload (new store instance), same as the page uses it', () => {
		localStorage.clear();
		const store = createSettingsStore();

		// Mirrors persistTileOrder() in analytics/+page.svelte
		store.update({ tileOrder: { ...store.value.tileOrder, 'system-resources': ['open-fds', 'ram-usage'] } });

		// A fresh store instance simulates a page reload, reading back from localStorage
		const reloaded = createSettingsStore();
		expect(reloaded.value.tileOrder['system-resources']).toEqual(['open-fds', 'ram-usage']);
		expect(
			resolveTileOrder(
				['ram-usage', 'heap-usage', 'goroutines', 'open-fds'],
				reloaded.value.tileOrder['system-resources']
			)
		).toEqual(['open-fds', 'ram-usage', 'heap-usage', 'goroutines']);
	});
});
