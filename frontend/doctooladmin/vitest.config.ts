import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

// Separate from vite.config.ts (rather than merged) so `vitest` doesn't pull
// in the Tailwind plugin — CSS processing isn't needed to run unit tests,
// only SvelteKit's module resolution ($lib/$app aliases, .svelte.ts rune
// compilation) is.
export default defineConfig({
	plugins: [sveltekit()],
	test: {
		environment: 'jsdom'
	}
});
