<script lang="ts">
	import { onMount } from 'svelte';
	import { slide, fade } from 'svelte/transition';
	import AppShell from '$lib/components/AppShell.svelte';
	import { credentials } from '$lib/stores/credentials.svelte';
	import { toasts } from '$lib/stores/toasts.svelte';
	import { ApiError, apiFetch, fetchJson } from '$lib/fetchJson';

	interface MetaData {
		claim_number: string;
		claimant_name: string;
		date_of_injury: string;
		employer: string;
		adjuster: string;
		support: string;
		claim_type: string;
		jurisdiction: string;
		policy_number: string;
		acts_id: string;
	}

	// ── File list ──────────────────────────────────────────────────────────────

	let files = $state<string[]>([]);
	let listLoading = $state(true);
	let listError = $state<string | null>(null);

	// Set true only once loadFiles() actually succeeds against the backend,
	// so a cancelled or wrong password (issue #345) never renders anything
	// but the prompt screen below. Doesn't by itself mean the page should
	// stay unlocked forever, though — see `authorized` below.
	let backendVerified = $state(false);

	// Gates the whole page's content, not just the file list. Re-derives off
	// the credentials store rather than staying a one-way latch, so an idle
	// timeout (issue #351) that clears the cached password immediately
	// re-locks this page behind the prompt screen, instead of leaving stale
	// content visible until the next manual action happens to notice.
	let authorized = $derived(backendVerified && credentials.hasFilesPassword);

	// ── Search ────────────────────────────────────────────────────────────────

	// searchQuery drives the input directly, so typing always feels
	// immediate. debouncedQuery only catches up 200ms after the user
	// pauses, so filteredFiles isn't recomputed on every keystroke — a
	// full array .filter() on each keystroke is wasted work once the file
	// count grows past trivial (issue #428).
	let searchQuery = $state('');
	let debouncedQuery = $state('');

	const SEARCH_DEBOUNCE_MS = 200;

	$effect(() => {
		const query = searchQuery;
		const timer = setTimeout(() => {
			debouncedQuery = query;
		}, SEARCH_DEBOUNCE_MS);
		// Cancels the pending update if searchQuery changes again (or the
		// component unmounts) before the timer fires.
		return () => clearTimeout(timer);
	});

	// Case-insensitive substring match against filename
	let filteredFiles = $derived(
		debouncedQuery.trim() === ''
			? files
			: files.filter((f) => f.toLowerCase().includes(debouncedQuery.trim().toLowerCase()))
	);

	async function loadFiles() {
		listLoading = true;
		listError = null;
		try {
			const data = await fetchJson<{ files?: string[] }>('/api/files', {
				headers: { 'X-Dewey-Password': await credentials.getFilesPassword() }
			});
			files = data.files ?? [];
			backendVerified = true;
		} catch (e) {
			// A rotated backend password (issue #414) means the cached value is
			// permanently wrong — clear it so the next attempt re-prompts
			// instead of resending the same stale password forever.
			if (e instanceof ApiError && e.status === 401) credentials.resetFilesPassword();
			listError = e instanceof Error ? e.message : String(e);
			toasts.push({ kind: 'error', title: 'Failed to load files', detail: listError });
		} finally {
			listLoading = false;
		}
	}

	onMount(loadFiles);

	// Clears the cached (wrong/cancelled) password so the prompt reappears,
	// then retries — used by the "Enter Password" button on the blocked screen.
	function retryAuth() {
		credentials.resetFilesPassword();
		loadFiles();
	}

	// ── Metadata expansion ────────────────────────────────────────────────────

	// Each entry is the loaded MetaData, 'loading', or 'error'
	let metaState = $state<Record<string, MetaData | 'loading' | 'error'>>({});

	async function toggleMeta(filename: string) {
		// Collapse if already shown
		if (metaState[filename]) {
			const next = { ...metaState };
			delete next[filename];
			metaState = next;
			return;
		}

		metaState = { ...metaState, [filename]: 'loading' };
		try {
			const meta = await fetchJson<MetaData>(
				`/api/files/${encodeURIComponent(filename)}?meta=true`,
				{ headers: { 'X-Dewey-Password': await credentials.getFilesPassword() } }
			);
			metaState = { ...metaState, [filename]: meta };
		} catch (e) {
			// Rotated password (issue #414) — clear the cache so the next
			// attempt re-prompts instead of resending the stale value.
			if (e instanceof ApiError && e.status === 401) credentials.resetFilesPassword();
			metaState = { ...metaState, [filename]: 'error' };
		}
	}

	// ── Download ──────────────────────────────────────────────────────────────

	let downloadError = $state<string | null>(null);

	// A plain <a href download> can't carry a custom header, and downloads
	// now require the files password (issue #332) — so this fetches the
	// file with the header attached and triggers the save via a temporary
	// object URL instead of letting the browser navigate directly.
	async function downloadFile(filename: string) {
		downloadError = null;
		try {
			const res = await apiFetch(`/api/files/${encodeURIComponent(filename)}?meta=false`, {
				headers: { 'X-Dewey-Password': await credentials.getFilesPassword() }
			});
			const blob = await res.blob();
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = filename;
			a.click();
			URL.revokeObjectURL(url);
		} catch (e) {
			// Rotated password (issue #414) — clear the cache so the next
			// attempt re-prompts instead of resending the stale value.
			if (e instanceof ApiError && e.status === 401) credentials.resetFilesPassword();
			downloadError = e instanceof Error ? e.message : String(e);
			toasts.push({ kind: 'error', title: 'Failed to download file', detail: downloadError });
		}
	}

	// ── Delete ────────────────────────────────────────────────────────────────

	let pendingDelete = $state<string | null>(null);
	let deleting = $state(false);
	let deleteError = $state<string | null>(null);

	async function confirmDelete() {
		if (!pendingDelete) return;
		deleting = true;
		deleteError = null;
		const target = pendingDelete;
		try {
			await apiFetch(`/api/files/${encodeURIComponent(target)}`, {
				method: 'DELETE',
				headers: { 'X-Dewey-Password': await credentials.getFilesPassword() }
			});
			files = files.filter((f) => f !== target);
			// Clean up any cached metadata for the deleted file
			const next = { ...metaState };
			delete next[target];
			metaState = next;
			pendingDelete = null;

			// The store copy is kept on the backend (issue #324) specifically so
			// this Undo can restore the actual file, not just a DB pointer.
			toasts.push({
				kind: 'info',
				title: 'File deleted',
				detail: target,
				action: { label: 'Undo', onClick: () => undeleteFile(target) }
			});
		} catch (e) {
			// Rotated password (issue #414) — clear the cache so the next
			// attempt re-prompts instead of resending the stale value.
			if (e instanceof ApiError && e.status === 401) credentials.resetFilesPassword();
			// Inline alert next to the row's Confirm/Cancel buttons (issue #241),
			// matching uploadError/listError elsewhere on this page instead of a
			// blocking native alert().
			deleteError = e instanceof Error ? e.message : String(e);
			toasts.push({ kind: 'error', title: 'Failed to delete file', detail: deleteError });
		} finally {
			deleting = false;
		}
	}

	// Reverses a delete within the toast's visible window — POSTs to the same
	// /api/files/:filename resource the GET/DELETE calls above use, then
	// refreshes the list so the restored file reappears.
	async function undeleteFile(filename: string) {
		try {
			await apiFetch(`/api/files/${encodeURIComponent(filename)}`, {
				method: 'POST',
				headers: { 'X-Dewey-Password': await credentials.getFilesPassword() }
			});
			await loadFiles();
			toasts.push({ kind: 'info', title: 'File restored', detail: filename });
		} catch (e) {
			// Rotated password (issue #414) — clear the cache so the next
			// attempt re-prompts instead of resending the stale value.
			if (e instanceof ApiError && e.status === 401) credentials.resetFilesPassword();
			toasts.push({
				kind: 'error',
				title: 'Failed to undo delete',
				detail: e instanceof Error ? e.message : String(e)
			});
		}
	}

	// ── Upload ────────────────────────────────────────────────────────────────

	let uploadOpen = $state(false);
	let uploading = $state(false);
	let uploadError = $state<string | null>(null);
	let uploadSuccess = $state<string | null>(null);

	// Bound form field values
	let fileInput = $state<HTMLInputElement | null>(null);
	let uploadFields = $state({
		claim_number: '',
		claimant_name: '',
		date_of_injury: '',
		employer: '',
		adjuster: '',
		support: '',
		claim_type: '',
		jurisdiction: '',
		policy_number: '',
		acts_id: '',
		data: ''
	});

	// Resets the file picker and every metadata field back to empty.
	// Shared by the submit-success path and the "Clear" button so both
	// stay in sync instead of duplicating the same reset object.
	function resetUploadForm() {
		if (fileInput) fileInput.value = '';
		uploadFields = {
			claim_number: '',
			claimant_name: '',
			date_of_injury: '',
			employer: '',
			adjuster: '',
			support: '',
			claim_type: '',
			jurisdiction: '',
			policy_number: '',
			acts_id: '',
			data: ''
		};
		uploadError = null;
		uploadSuccess = null;
	}

	async function submitUpload() {
		if (!fileInput?.files?.[0]) {
			uploadError = 'Please select a file.';
			return;
		}
		// date_of_injury is a NOT NULL column on the backend (issue #365) — a
		// blank value here previously reached the DB insert silently, since the
		// upload response is sent before that insert even runs.
		if (!uploadFields.date_of_injury.trim()) {
			uploadError = 'Date of Injury is required.';
			return;
		}

		uploadError = null;
		uploadSuccess = null;
		uploading = true;

		const form = new FormData();
		form.append('file', fileInput.files[0]);
		for (const [key, value] of Object.entries(uploadFields)) {
			form.append(key, value);
		}

		try {
			// Body-size-limit rejections (and similar) return a non-JSON
			// response — fetchJson's error-body parsing already guards
			// against that on the failure path, so no special-casing needed
			// here.
			const body = await fetchJson<{ filename?: string }>('/api/files', {
				method: 'POST',
				body: form,
				headers: { 'X-Dewey-Password': await credentials.getFilesPassword() }
			});
			const uploadedFilename = body.filename ?? 'File uploaded successfully.';
			resetUploadForm();
			uploadSuccess = uploadedFilename;
			// Refresh list to include the new file
			await loadFiles();
		} catch (e) {
			// Rotated password (issue #414) — clear the cache so the next
			// attempt re-prompts instead of resending the stale value.
			if (e instanceof ApiError && e.status === 401) credentials.resetFilesPassword();
			uploadError = e instanceof Error ? e.message : String(e);
			toasts.push({ kind: 'error', title: 'Failed to upload file', detail: uploadError });
		} finally {
			uploading = false;
		}
	}
</script>

<svelte:head>
	<title>File Management — Dewey</title>
</svelte:head>

<AppShell>
	{#snippet children({ headingClass })}
	{#if !authorized}
		<!-- Nothing below renders until loadFiles() succeeds with the correct
		     password — a wrong or cancelled attempt lands here instead (#345). -->
		<div
			class="flex flex-col items-center justify-center gap-3 rounded-2xl bg-white p-12 text-center shadow-sm dark:bg-gray-800"
		>
			<h1 class="text-lg font-semibold text-gray-800 dark:text-white">Password Required</h1>
			<p class="text-sm text-gray-500 dark:text-gray-400">
				{listError
					? 'Incorrect password.'
					: listLoading
						? 'Checking…'
						: 'Enter the files management password to continue.'}
			</p>
			<button
				class="rounded-lg bg-gray-900 px-6 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-gray-700 dark:hover:bg-gray-600"
				onclick={retryAuth}
			>
				Enter Password
			</button>
		</div>
	{:else}
		<!-- ── Header ─────────────────────────────────────────────────────── -->
		<div class="flex items-center justify-between">
			<h1 class="text-2xl font-bold text-gray-800 dark:text-white">File Management</h1>
			<div class="flex gap-2">
				<button
					class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-600 shadow-sm hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
					onclick={loadFiles}
				>
					Refresh
				</button>
				<button
					class="rounded-lg px-4 py-2 text-sm font-medium text-white shadow-sm transition-colors {uploadOpen
						? 'bg-gray-600 hover:bg-gray-700'
						: 'bg-gray-900 hover:bg-gray-800 dark:bg-gray-700 dark:hover:bg-gray-600'}"
					onclick={() => {
						uploadOpen = !uploadOpen;
						uploadError = null;
						uploadSuccess = null;
					}}
				>
					{uploadOpen ? 'Cancel Upload' : 'Upload File'}
				</button>
			</div>
		</div>

		<!-- ── Upload Panel ───────────────────────────────────────────────── -->
		{#if uploadOpen}
			<section
				transition:slide={{ duration: 200 }}
				class="rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800"
			>
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">
					Upload New File
				</h2>

				<form
					onsubmit={(e) => {
						e.preventDefault();
						submitUpload();
					}}
					class="space-y-4"
				>
					<!-- File picker -->
					<div>
						<label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
							File <span class="text-red-500">*</span>
						</label>
						<input
							type="file"
							bind:this={fileInput}
							class="block w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-300"
						/>
					</div>

					<!-- Metadata fields — 2-column grid -->
					<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
						{#each [{ key: 'claim_number', label: 'Claim Number' }, { key: 'claimant_name', label: 'Claimant Name' }, { key: 'date_of_injury', label: 'Date of Injury', type: 'date', required: true }, { key: 'employer', label: 'Employer' }, { key: 'adjuster', label: 'Adjuster' }, { key: 'support', label: 'Support Level' }, { key: 'claim_type', label: 'Claim Type' }, { key: 'jurisdiction', label: 'Jurisdiction' }, { key: 'policy_number', label: 'Policy Number' }, { key: 'acts_id', label: 'ACTs ID' }, { key: 'data', label: 'Data / Tag' }] as field}
							<div>
								<label
									for="upload-{field.key}"
									class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
								>
									{field.label}
									{#if field.required}<span class="text-red-500">*</span>{/if}
								</label>
								<input
									id="upload-{field.key}"
									type={field.type ?? 'text'}
									autocomplete="off"
									required={field.required ?? false}
									class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
									bind:value={uploadFields[field.key as keyof typeof uploadFields]}
								/>
							</div>
						{/each}
					</div>

					<!-- Status messages -->
					{#if uploadError}
						<p role="alert" class="text-sm text-red-600 dark:text-red-400">{uploadError}</p>
					{/if}
					{#if uploadSuccess}
						<p role="status" class="text-sm text-green-600 dark:text-green-400">
							Uploaded: {uploadSuccess}
						</p>
					{/if}

					<div class="flex gap-2">
						<button
							type="submit"
							disabled={uploading}
							class="rounded-lg bg-gray-900 px-6 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-50 dark:bg-gray-700 dark:hover:bg-gray-600"
						>
							{uploading ? 'Uploading…' : 'Upload'}
						</button>
						<button
							type="button"
							disabled={uploading}
							onclick={resetUploadForm}
							class="rounded-lg border border-gray-300 bg-white px-6 py-2 text-sm font-medium text-gray-600 shadow-sm hover:bg-gray-50 disabled:opacity-50 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
						>
							Clear
						</button>
					</div>
				</form>
			</section>
		{/if}

		<!-- ── File List ──────────────────────────────────────────────────── -->
		<section class="rounded-2xl bg-white shadow-sm dark:bg-gray-800">
			<div
				class="flex flex-wrap items-center gap-3 border-b border-gray-100 px-6 py-4 dark:border-gray-700"
			>
				<h2 class="shrink-0 text-xs font-semibold tracking-wider uppercase {headingClass}">
					Stored Files
				</h2>

				<!-- Search input -->
				<input
					type="search"
					placeholder="Search filenames…"
					aria-label="Search filenames"
					bind:value={searchQuery}
					class="min-w-0 flex-1 rounded-lg border border-gray-200 bg-gray-50 px-3 py-1.5 text-sm text-gray-700 placeholder-gray-400 focus:border-transparent focus:ring-2 focus:ring-gray-400 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 dark:placeholder-gray-500 dark:focus:ring-gray-500"
				/>

				{#if !listLoading}
					<span class="shrink-0 text-xs text-gray-400 dark:text-gray-500">
						{#if searchQuery.trim()}
							{filteredFiles.length} of {files.length}
						{:else}
							{files.length} {files.length === 1 ? 'file' : 'files'}
						{/if}
					</span>
				{/if}
			</div>

			{#if downloadError}
				<p role="alert" class="px-6 pt-4 text-sm text-red-600 dark:text-red-400">
					Download failed: {downloadError}
				</p>
			{/if}

			{#if listLoading}
				<p class="px-6 py-8 text-sm text-gray-500 dark:text-gray-400">Loading…</p>
			{:else if listError}
				<p role="alert" class="px-6 py-8 text-sm text-red-600 dark:text-red-400">
					Failed to load files: {listError}
				</p>
			{:else if files.length === 0}
				<p class="px-6 py-8 text-sm text-gray-500 dark:text-gray-400">No files stored.</p>
			{:else if filteredFiles.length === 0}
				<p class="px-6 py-8 text-sm text-gray-500 dark:text-gray-400">
					No files match <span class="font-medium text-gray-700 dark:text-gray-300"
						>"{searchQuery}"</span
					>.
				</p>
			{:else}
				<div class="overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr
								class="border-b border-gray-100 bg-gray-50 text-left dark:border-gray-700 dark:bg-gray-900/40"
							>
								<th class="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">Filename</th>
								<th class="px-6 py-3 text-right font-medium text-gray-500 dark:text-gray-400"
									>Actions</th
								>
							</tr>
						</thead>
						<tbody>
							{#each filteredFiles as filename (filename)}
								<!-- Main file row -->
								<tr
									class="border-b border-gray-50 hover:bg-gray-50/50 dark:border-gray-700/50 dark:hover:bg-gray-700/20"
								>
									<td class="px-6 py-3 font-mono text-gray-800 dark:text-gray-200">
										{filename}
									</td>
									<td class="px-6 py-3">
										<div class="flex items-center justify-end gap-2">
											<!-- Download: fetched with the password header, then saved via a temporary object URL -->
											<button
												class="rounded-md px-3 py-1.5 text-xs font-medium text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-900/20"
												onclick={() => downloadFile(filename)}
											>
												Download
											</button>

											<!-- Metadata toggle -->
											<button
												class="rounded-md px-3 py-1.5 text-xs font-medium transition-colors {metaState[
													filename
												]
													? 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200'
													: 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700'}"
												onclick={() => toggleMeta(filename)}
											>
												{metaState[filename] ? 'Hide Info' : 'View Info'}
											</button>

											<!-- Delete: request confirmation first -->
											{#if pendingDelete === filename}
												<span
													class="flex items-center gap-2"
													transition:fade={{ duration: 150 }}
												>
													<span class="text-xs text-gray-500 dark:text-gray-400">Are you sure?</span
													>
													<button
														disabled={deleting}
														class="rounded-md px-3 py-1.5 text-xs font-medium text-red-600 hover:bg-red-50 disabled:opacity-50 dark:text-red-400 dark:hover:bg-red-900/20"
														onclick={confirmDelete}
													>
														{deleting ? 'Deleting…' : 'Confirm'}
													</button>
													<button
														class="rounded-md px-3 py-1.5 text-xs font-medium text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700"
														onclick={() => {
															pendingDelete = null;
															deleteError = null;
														}}
													>
														Cancel
													</button>
													{#if deleteError}
														<span role="alert" class="text-xs text-red-600 dark:text-red-400"
															>{deleteError}</span
														>
													{/if}
												</span>
											{:else}
												<button
													class="rounded-md px-3 py-1.5 text-xs font-medium text-red-500 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
													onclick={() => {
														pendingDelete = filename;
														deleteError = null;
													}}
												>
													Delete
												</button>
											{/if}
										</div>
									</td>
								</tr>

								<!-- Expanded metadata row -->
								{#if metaState[filename]}
									<tr
										class="border-b border-gray-100 bg-gray-50/70 dark:border-gray-700 dark:bg-gray-900/20"
									>
										<td colspan="2" class="px-6 py-4">
											{#if metaState[filename] === 'loading'}
												<p class="text-xs text-gray-400">Loading metadata…</p>
											{:else if metaState[filename] === 'error'}
												<p class="text-xs text-red-500">Failed to load metadata.</p>
											{:else}
												{@const meta = metaState[filename] as MetaData}
												<dl class="grid grid-cols-2 gap-x-8 gap-y-2 sm:grid-cols-3 lg:grid-cols-5">
													{#each [{ label: 'Claim #', value: meta.claim_number }, { label: 'Claimant', value: meta.claimant_name }, { label: 'Date of Injury', value: meta.date_of_injury }, { label: 'Employer', value: meta.employer }, { label: 'Adjuster', value: meta.adjuster }, { label: 'Support', value: meta.support }, { label: 'Claim Type', value: meta.claim_type }, { label: 'Jurisdiction', value: meta.jurisdiction }, { label: 'Policy #', value: meta.policy_number }, { label: 'ACTs ID', value: meta.acts_id }] as item}
														<div>
															<dt class="text-xs font-medium text-gray-400 dark:text-gray-500">
																{item.label}
															</dt>
															<dd class="text-sm text-gray-700 dark:text-gray-200">
																{item.value || '—'}
															</dd>
														</div>
													{/each}
												</dl>
											{/if}
										</td>
									</tr>
								{/if}
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</section>
	{/if}
	{/snippet}
</AppShell>
