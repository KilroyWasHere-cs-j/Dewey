<script lang="ts">
	const { open = true, toggle } = $props<{
		open?: boolean;
		toggle?: () => void;
	}>();
</script>

<aside
	class={`bg-gray-900 text-white w-64 p-4 space-y-4
		${open ? 'block' : 'hidden'} md:block`}
>
	<h2 class="text-2xl font-bold mb-6">Dashboard</h2>

	<nav class="space-y-2">
		<a
			href="/dashboard"
			class="block px-3 py-2 rounded hover:bg-gray-700"
			on:click={() => toggle?.()}
		>
			Home
		</a>

		<a
			href="/analytics"
			class="block px-3 py-2 rounded hover:bg-gray-700"
			on:click={() => toggle?.()}
		>
			Analytics
		</a>

		<a
			href="/files"
			class="block px-3 py-2 rounded hover:bg-gray-700"
			on:click={() => toggle?.()}
		>
			Files
		</a>

		<a
			href="/settings"
			class="block px-3 py-2 rounded hover:bg-gray-700"
			on:click={() => toggle?.()}
		>
			Settings
		</a>

		<a
			href="/dev"
			class="block px-3 py-2 rounded hover:bg-gray-700"
			on:click={() => toggle?.()}
		>
			Dev
		</a>
	</nav>
</aside>