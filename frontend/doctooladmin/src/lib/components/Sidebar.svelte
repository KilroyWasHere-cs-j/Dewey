<script lang="ts">
	import { page } from '$app/state';
	import { settings, ACCENT } from '$lib/stores/settings.svelte';

	const { open = true, toggle } = $props<{
		open?: boolean;
		toggle?: () => void;
	}>();

	const links = [
		{ href: '/analytics', label: 'Analytics' },
		{ href: '/settings', label: 'Settings' }
	];

	// Accent hex for the active link's left border (inline style — avoids Tailwind purge)
	let accentHex = $derived(ACCENT[settings.value.accentColor].hex);
</script>

<aside
	aria-label="Sidebar"
	class={`bg-gray-900 text-white w-64 p-4 space-y-4
		${open ? 'block' : 'hidden'} md:block`}
>
	<h2 class="mb-6 text-2xl font-bold">{settings.value.portalName}</h2>

	<nav aria-label="Main navigation" class="space-y-1">
		{#each links as link}
			{@const active = page.url.pathname === link.href}
			<a
				href={link.href}
				aria-current={active ? 'page' : undefined}
				class="block rounded px-3 py-2 text-sm transition-colors {active
					? 'border-l-4 bg-gray-800 pl-2 font-medium text-white'
					: 'border-l-4 border-transparent text-gray-400 hover:bg-gray-700 hover:text-white'}"
				style={active ? `border-color: ${accentHex}` : ''}
				onclick={() => toggle?.()}
			>
				{link.label}
			</a>
		{/each}
	</nav>
</aside>
