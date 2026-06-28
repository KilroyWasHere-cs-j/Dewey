<script lang="ts">
	import { page } from '$app/stores';

	// Map known status codes to human-readable titles and descriptions
	const ERROR_COPY: Record<number, { title: string; description: string }> = {
		404: {
			title: 'Page not found',
			description: "The page you're looking for doesn't exist or has been moved."
		},
		500: {
			title: 'Internal server error',
			description: 'Something went wrong on our end. Try refreshing, or check the logs.'
		},
		403: {
			title: 'Access denied',
			description: "You don't have permission to view this page."
		}
	};

	let status = $derived($page.status);
	let copy = $derived(ERROR_COPY[status] ?? { title: 'Unexpected error', description: $page.error?.message ?? 'An unknown error occurred.' });
</script>

<svelte:head>
	<title>{status} — Dewey</title>
</svelte:head>

<!--
	Full-screen dark page consistent with the app's #0f172a body background.
	The large blurred status code behind the content is purely decorative.
-->
<div class="relative flex min-h-screen flex-col items-center justify-center bg-[#0f172a] px-6 text-center overflow-hidden">

	<!-- Blurred background code — decorative depth layer -->
	<span
		aria-hidden="true"
		class="pointer-events-none absolute select-none text-[22rem] font-black leading-none text-white/[0.03] blur-sm"
	>
		{status}
	</span>

	<!-- Status badge -->
	<div class="mb-4 inline-flex items-center rounded-full border border-white/10 bg-white/5 px-4 py-1.5 text-xs font-semibold tracking-widest text-slate-400 uppercase">
		{#if status === 404}
			Not Found
		{:else if status === 500}
			Server Error
		{:else}
			HTTP {status}
		{/if}
	</div>

	<!-- Large code number -->
	<h1 class="text-8xl font-black tabular-nums text-white sm:text-9xl">{status}</h1>

	<!-- Title -->
	<p class="mt-4 text-2xl font-semibold text-slate-200">{copy.title}</p>

	<!-- Description -->
	<p class="mt-2 max-w-sm text-sm leading-relaxed text-slate-400">{copy.description}</p>

	<!-- Actions -->
	<div class="mt-8 flex flex-col items-center gap-3 sm:flex-row">
		<a
			href="/dashboard"
			class="rounded-lg bg-white px-5 py-2.5 text-sm font-semibold text-gray-900 shadow-sm transition hover:bg-slate-100"
		>
			Go to dashboard
		</a>
		<button
			onclick={() => history.back()}
			class="rounded-lg border border-white/10 bg-white/5 px-5 py-2.5 text-sm font-semibold text-slate-300 transition hover:bg-white/10"
		>
			Go back
		</button>
	</div>

	<!-- Debug message when available (non-404 errors often carry one) -->
	{#if $page.error?.message && status !== 404}
		<p class="mt-6 rounded-lg bg-white/5 px-4 py-2 font-mono text-xs text-slate-500">
			{$page.error.message}
		</p>
	{/if}
</div>
