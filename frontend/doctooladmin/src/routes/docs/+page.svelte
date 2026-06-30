<script lang="ts">
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Topbar from '$lib/components/Topbar.svelte';
	import { settings, ACCENT } from '$lib/stores/settings.svelte';

	let mainClass = $derived(
		settings.value.layoutDensity === 'compact'
			? 'max-w-4xl space-y-4 p-4'
			: 'max-w-4xl space-y-6 p-6'
	);
	let headingClass = $derived(ACCENT[settings.value.accentColor].text);
	let sidebarOpen = $state(settings.value.defaultSidebarOpen);
	const toggleSidebar = () => (sidebarOpen = !sidebarOpen);
</script>

<svelte:head>
	<title>Documentation — Dewey</title>
</svelte:head>

<div class="flex min-h-screen bg-gray-100 dark:bg-gray-900">
	<Sidebar open={sidebarOpen} toggle={toggleSidebar} />

	<div class="flex flex-1 flex-col">
		<Topbar {sidebarOpen} {toggleSidebar} />

		<main id="main-content" class={mainClass}>
			<div>
				<h1 class="text-2xl font-bold text-gray-800 dark:text-white">Documentation</h1>
				<p class="text-sm text-gray-500 dark:text-gray-400">
					Choose a guide based on your role.
				</p>
			</div>

			<div class="grid gap-6 sm:grid-cols-2">
				<!-- Developer card -->
				<a
					href="/docs/developer"
					class="group rounded-2xl bg-white p-6 shadow-sm transition-shadow hover:shadow-md dark:bg-gray-800"
				>
					<h2 class="mb-2 text-lg font-semibold text-gray-800 group-hover:underline dark:text-white">
						Developer Guide
					</h2>
					<p class="mb-4 text-sm text-gray-500 dark:text-gray-400">
						Architecture, API reference, plugin system, database schema, logging, and deployment.
						For engineers building on or maintaining Dewey.
					</p>
					<ul class="space-y-1 text-xs text-gray-400 dark:text-gray-500">
						<li>→ System architecture & file pipeline</li>
						<li>→ REST API reference</li>
						<li>→ Writing Lua plugins</li>
						<li>→ Database schema</li>
						<li>→ Deployment with Podman</li>
					</ul>
					<span class="mt-4 inline-block text-sm font-medium {headingClass}">
						Read developer docs →
					</span>
				</a>

				<!-- Admin card -->
				<a
					href="/docs/admin"
					class="group rounded-2xl bg-white p-6 shadow-sm transition-shadow hover:shadow-md dark:bg-gray-800"
				>
					<h2 class="mb-2 text-lg font-semibold text-gray-800 group-hover:underline dark:text-white">
						Admin Guide
					</h2>
					<p class="mb-4 text-sm text-gray-500 dark:text-gray-400">
						Day-to-day operation of Dewey. Uploading files, managing settings, reading the
						analytics dashboard, and troubleshooting. For administrators and users.
					</p>
					<ul class="space-y-1 text-xs text-gray-400 dark:text-gray-500">
						<li>→ Uploading & retrieving files</li>
						<li>→ Settings reference</li>
						<li>→ Analytics & monitoring</li>
						<li>→ System configuration</li>
						<li>→ Troubleshooting</li>
					</ul>
					<span class="mt-4 inline-block text-sm font-medium {headingClass}">
						Read admin guide →
					</span>
				</a>
			</div>
		</main>
	</div>
</div>
