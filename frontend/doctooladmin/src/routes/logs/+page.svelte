<script lang="ts">
	import { onMount } from 'svelte';
	import AppShell from '$lib/components/AppShell.svelte';
	import { toasts } from '$lib/stores/toasts.svelte';

	// ── Log file list (populated from /fileview/viewLogDir) ────────────────────

	let logFiles = $state<string[]>([]);
	let logFilesLoading = $state(true);
	let logFilesError = $state<string | null>(null);

	async function loadLogFiles() {
		logFilesLoading = true;
		logFilesError = null;
		try {
			const res = await fetch('/api/fileview/dir');
			if (!res.ok) {
				const body = await res.json().catch(() => ({}));
				throw new Error(body.error ?? `HTTP ${res.status}`);
			}
			const data = await res.json();
			logFiles = data.files ?? [];
		} catch (e) {
			logFilesError = String(e);
			toasts.push({ kind: 'error', title: 'Failed to list log files', detail: logFilesError });
		} finally {
			logFilesLoading = false;
		}
	}

	onMount(loadLogFiles);

	// ── Selection ────────────────────────────────────────────────────────────

	type FileType = 'log' | 'config';

	let fileType = $state<FileType>('log');
	let selectedLogFile = $state('');
	let selectedFile = $derived(fileType === 'config' ? 'config.json' : selectedLogFile);

	// ── Content fetch, with a temp in-memory cache ──────────────────────────────
	// Keyed by "fileType/file" so re-selecting something already viewed this
	// session serves the cached copy instead of hitting the backend again.
	// Cache lives only in this page's memory — cleared on reload, and never
	// persisted (unlike the settings store), since log/config content isn't
	// something that should survive across sessions as stale data.

	const contentCache = new Map<string, string>();

	let content = $state<string | null>(null);
	let contentLoading = $state(false);
	let contentError = $state<string | null>(null);

	async function loadContent(forceRefresh = false) {
		if (!selectedFile) {
			content = null;
			return;
		}

		const cacheKey = `${fileType}/${selectedFile}`;

		if (!forceRefresh && contentCache.has(cacheKey)) {
			content = contentCache.get(cacheKey)!;
			contentError = null;
			return;
		}

		contentLoading = true;
		contentError = null;
		try {
			const res = await fetch(
				`/api/fileview/${encodeURIComponent(fileType)}/${encodeURIComponent(selectedFile)}`
			);
			if (!res.ok) {
				const body = await res.json().catch(() => ({}));
				throw new Error(body.error ?? `HTTP ${res.status}`);
			}
			const text = await res.text();
			contentCache.set(cacheKey, text);
			content = text;
		} catch (e) {
			contentError = String(e);
			content = null;
			toasts.push({ kind: 'error', title: 'Failed to load file', detail: contentError });
		} finally {
			contentLoading = false;
		}
	}

	// Refetch whenever the effective selection changes (fileType or, for
	// "log", which file is picked) — but not on every keystroke, since
	// selection only changes via the dropdowns below.
	$effect(() => {
		selectedFile;
		loadContent();
	});

	// Config has exactly one file, so pick it automatically once fileType
	// switches to "config" instead of leaving a dropdown with one option.
	$effect(() => {
		if (fileType === 'log' && !selectedLogFile && logFiles.length > 0) {
			selectedLogFile = logFiles[0];
		}
	});

	// Logs read top-down (newest first) — config is a single static blob with
	// no chronological order, so only reverse for the log branch. A log file
	// normally ends in a trailing newline; stripped first so reversing
	// doesn't leave a blank line at the top.
	let displayContent = $derived(
		fileType === 'log' && content !== null
			? content.replace(/\n+$/, '').split('\n').reverse().join('\n')
			: content
	);
</script>

<svelte:head>
	<title>Logs & Config — Dewey</title>
</svelte:head>

<AppShell>
	{#snippet children({ headingClass })}
		<div class="flex items-center justify-between">
			<h1 class="text-2xl font-bold text-gray-800 dark:text-white">Logs &amp; Config</h1>
		</div>

		<!-- ── Controls ───────────────────────────────────────────────────── -->
		<section class="rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
			<div class="flex flex-wrap items-end gap-4">
				<div>
					<label
						for="file-type-select"
						class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
					>
						File Type
					</label>
					<select
						id="file-type-select"
						bind:value={fileType}
						class="w-40 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700 focus:border-transparent focus:ring-2 focus:ring-gray-400 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-white dark:focus:ring-gray-500"
					>
						<option value="log">Log</option>
						<option value="config">Config</option>
					</select>
				</div>

				{#if fileType === 'log'}
					<div class="min-w-[16rem] flex-1">
						<label
							for="log-file-select"
							class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
						>
							Log File
						</label>
						{#if logFilesLoading}
							<p class="py-2 text-sm text-gray-500 dark:text-gray-400">Loading log files…</p>
						{:else if logFilesError}
							<p role="alert" class="py-2 text-sm text-red-600 dark:text-red-400">
								{logFilesError}
							</p>
						{:else if logFiles.length === 0}
							<p class="py-2 text-sm text-gray-500 dark:text-gray-400">No log files found.</p>
						{:else}
							<select
								id="log-file-select"
								bind:value={selectedLogFile}
								class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700 focus:border-transparent focus:ring-2 focus:ring-gray-400 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-white dark:focus:ring-gray-500"
							>
								{#each logFiles as f (f)}
									<option value={f}>{f}</option>
								{/each}
							</select>
						{/if}
					</div>

					<!-- Config is a single static file baked into the image, so it
					     can't go stale between requests the way a growing log can —
					     Refresh only makes sense for logs. -->
					<button
						type="button"
						onclick={() => loadContent(true)}
						disabled={!selectedFile || contentLoading}
						class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-600 shadow-sm hover:bg-gray-50 disabled:opacity-50 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
					>
						{contentLoading ? 'Refreshing…' : 'Refresh'}
					</button>
				{/if}
			</div>
		</section>

		<!-- ── Content ────────────────────────────────────────────────────── -->
		<section class="rounded-2xl bg-white shadow-sm dark:bg-gray-800">
			<div
				class="flex flex-wrap items-center justify-between gap-2 border-b border-gray-100 px-6 py-4 dark:border-gray-700"
			>
				<h2 class="font-mono text-xs font-semibold tracking-wider uppercase {headingClass}">
					{fileType === 'log' ? selectedFile || 'Log' : 'config.json'}
				</h2>
				{#if fileType === 'log' && content !== null}
					<span class="text-xs text-gray-400 dark:text-gray-500">Newest first</span>
				{/if}
			</div>

			<div class="p-6">
				{#if contentLoading}
					<p class="text-sm text-gray-500 dark:text-gray-400">Loading…</p>
				{:else if contentError}
					<p role="alert" class="text-sm text-red-600 dark:text-red-400">{contentError}</p>
				{:else if displayContent !== null}
					<pre
						class="max-h-[32rem] overflow-auto rounded-lg bg-gray-50 p-4 font-mono text-xs whitespace-pre-wrap text-gray-800 dark:bg-gray-900/40 dark:text-gray-200">{displayContent}</pre>
				{:else}
					<p class="text-sm text-gray-500 dark:text-gray-400">
						Select a file to view its contents.
					</p>
				{/if}
			</div>
		</section>
	{/snippet}
</AppShell>
