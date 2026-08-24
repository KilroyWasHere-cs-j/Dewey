import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

// Separate from vite.config.ts (rather than merged) so `vitest` doesn't pull
// in the Tailwind plugin — CSS processing isn't needed to run unit tests,
// only SvelteKit's module resolution ($lib/$app aliases, .svelte.ts rune
// compilation) is.
export default defineConfig({
	plugins: [sveltekit()],
	// Default 'node' environment — nothing under test touches real DOM
	// globals (settings.test.ts stubs its own minimal localStorage rather
	// than depend on jsdom's), and jsdom's own Node-version requirements
	// (a transitive dependency needs Node >=22) are otherwise incompatible
	// with CI's pinned Node 20.
	test: {}
});
