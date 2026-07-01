<script lang="ts">
	import { onMount } from 'svelte';
	import Sidebar from '$lib/components/Sidebar.svelte';
	import Topbar from '$lib/components/Topbar.svelte';
	import { settings, ACCENT } from '$lib/stores/settings.svelte';

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

	let sidebarOpen = $state(settings.value.defaultSidebarOpen);
	const toggleSidebar = () => { sidebarOpen = !sidebarOpen; };

	let headingClass = $derived(ACCENT[settings.value.accentColor].text);
	let mainClass = $derived(
		settings.value.layoutDensity === 'compact' ? 'space-y-4 p-4' : 'space-y-6 p-6'
	);

	// ── File list ──────────────────────────────────────────────────────────────

	let files = $state<string[]>([]);
	let listLoading = $state(true);
	let listError = $state<string | null>(null);

	async function loadFiles() {
		listLoading = true;
		listError = null;
		try {
			const res = await fetch('/api/files');
			if (!res.ok) throw new Error(`HTTP ${res.status}`);
			const data = await res.json();
			files = data.files ?? [];
		} catch (e) {
			listError = String(e);
		} finally {
			listLoading = false;
		}
	}

	onMount(loadFiles);

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
			const res = await fetch(`/api/files/${encodeURIComponent(filename)}?meta=true`);
			if (!res.ok) throw new Error(`HTTP ${res.status}`);
			metaState = { ...metaState, [filename]: await res.json() };
		} catch {
			metaState = { ...metaState, [filename]: 'error' };
		}
	}

	// ── Delete ────────────────────────────────────────────────────────────────

	let pendingDelete = $state<string | null>(null);
	let deleting = $state(false);

	async function confirmDelete() {
		if (!pendingDelete) return;
		deleting = true;
		const target = pendingDelete;
		try {
			const res = await fetch(`/api/files/${encodeURIComponent(target)}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(`HTTP ${res.status}`);
			files = files.filter((f) => f !== target);
			// Clean up any cached metadata for the deleted file
			const next = { ...metaState };
			delete next[target];
			metaState = next;
			pendingDelete = null;
		} catch (e) {
			alert('Delete failed: ' + e);
		} finally {
			deleting = false;
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

	async function submitUpload() {
		if (!fileInput?.files?.[0]) {
			uploadError = 'Please select a file.';
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
			const res = await fetch('/api/files', { method: 'POST', body: form });
			const body = await res.json();
			if (!res.ok) {
				uploadError = body.error ?? `HTTP ${res.status}`;
				return;
			}
			uploadSuccess = body.filename ?? 'File uploaded successfully.';
			// Reset file input and fields
			if (fileInput) fileInput.value = '';
			uploadFields = {
				claim_number: '', claimant_name: '', date_of_injury: '',
				employer: '', adjuster: '', support: '', claim_type: '',
				jurisdiction: '', policy_number: '', acts_id: '', data: ''
			};
			// Refresh list to include the new file
			await loadFiles();
		} catch (e) {
			uploadError = String(e);
		} finally {
			uploading = false;
		}
	}
</script>

<svelte:head>
	<title>File Management — Dewey</title>
</svelte:head>

<div class="flex min-h-screen bg-gray-100 dark:bg-gray-900">
	<Sidebar open={sidebarOpen} toggle={toggleSidebar} />

	<div class="flex flex-1 flex-col">
		<Topbar {sidebarOpen} {toggleSidebar} />

		<main id="main-content" class={mainClass}>

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
						onclick={() => { uploadOpen = !uploadOpen; uploadError = null; uploadSuccess = null; }}
					>
						{uploadOpen ? 'Cancel Upload' : 'Upload File'}
					</button>
				</div>
			</div>

			<!-- ── Upload Panel ───────────────────────────────────────────────── -->
			{#if uploadOpen}
				<section class="rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
					<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">
						Upload New File
					</h2>

					<form
						onsubmit={(e) => { e.preventDefault(); submitUpload(); }}
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
							{#each [
								{ key: 'claim_number',   label: 'Claim Number' },
								{ key: 'claimant_name',  label: 'Claimant Name' },
								{ key: 'date_of_injury', label: 'Date of Injury', type: 'date' },
								{ key: 'employer',       label: 'Employer' },
								{ key: 'adjuster',       label: 'Adjuster' },
								{ key: 'support',        label: 'Support Level' },
								{ key: 'claim_type',     label: 'Claim Type' },
								{ key: 'jurisdiction',   label: 'Jurisdiction' },
								{ key: 'policy_number',  label: 'Policy Number' },
								{ key: 'acts_id',        label: 'ACTs ID' },
								{ key: 'data',           label: 'Data / Tag' }
							] as field}
								<div>
									<label
										for="upload-{field.key}"
										class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
									>
										{field.label}
									</label>
									<input
										id="upload-{field.key}"
										type={field.type ?? 'text'}
										autocomplete="off"
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

						<button
							type="submit"
							disabled={uploading}
							class="rounded-lg bg-gray-900 px-6 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-50 dark:bg-gray-700 dark:hover:bg-gray-600"
						>
							{uploading ? 'Uploading…' : 'Upload'}
						</button>
					</form>
				</section>
			{/if}

			<!-- ── File List ──────────────────────────────────────────────────── -->
			<section class="rounded-2xl bg-white shadow-sm dark:bg-gray-800">
				<div class="flex items-center justify-between border-b border-gray-100 px-6 py-4 dark:border-gray-700">
					<h2 class="text-xs font-semibold tracking-wider uppercase {headingClass}">
						Stored Files
					</h2>
					{#if !listLoading}
						<span class="text-xs text-gray-400 dark:text-gray-500">
							{files.length} {files.length === 1 ? 'file' : 'files'}
						</span>
					{/if}
				</div>

				{#if listLoading}
					<p class="px-6 py-8 text-sm text-gray-500 dark:text-gray-400">Loading…</p>

				{:else if listError}
					<p role="alert" class="px-6 py-8 text-sm text-red-600 dark:text-red-400">
						Failed to load files: {listError}
					</p>

				{:else if files.length === 0}
					<p class="px-6 py-8 text-sm text-gray-500 dark:text-gray-400">No files stored.</p>

				{:else}
					<div class="overflow-x-auto">
						<table class="w-full text-sm">
							<thead>
								<tr class="border-b border-gray-100 bg-gray-50 text-left dark:border-gray-700 dark:bg-gray-900/40">
									<th class="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">Filename</th>
									<th class="px-6 py-3 text-right font-medium text-gray-500 dark:text-gray-400">Actions</th>
								</tr>
							</thead>
							<tbody>
								{#each files as filename (filename)}
									<!-- Main file row -->
									<tr class="border-b border-gray-50 hover:bg-gray-50/50 dark:border-gray-700/50 dark:hover:bg-gray-700/20">
										<td class="px-6 py-3 font-mono text-gray-800 dark:text-gray-200">
											{filename}
										</td>
										<td class="px-6 py-3">
											<div class="flex items-center justify-end gap-2">
												<!-- Download: browser follows the link, Content-Disposition triggers save dialog -->
												<a
													href="/api/files/{encodeURIComponent(filename)}?meta=false"
													download={filename}
													class="rounded-md px-3 py-1.5 text-xs font-medium text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-900/20"
												>
													Download
												</a>

												<!-- Metadata toggle -->
												<button
													class="rounded-md px-3 py-1.5 text-xs font-medium transition-colors {metaState[filename]
														? 'bg-gray-100 text-gray-700 dark:bg-gray-700 dark:text-gray-200'
														: 'text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700'}"
													onclick={() => toggleMeta(filename)}
												>
													{metaState[filename] ? 'Hide Info' : 'View Info'}
												</button>

												<!-- Delete: request confirmation first -->
												{#if pendingDelete === filename}
													<span class="text-xs text-gray-500 dark:text-gray-400">Are you sure?</span>
													<button
														disabled={deleting}
														class="rounded-md px-3 py-1.5 text-xs font-medium text-red-600 hover:bg-red-50 disabled:opacity-50 dark:text-red-400 dark:hover:bg-red-900/20"
														onclick={confirmDelete}
													>
														{deleting ? 'Deleting…' : 'Confirm'}
													</button>
													<button
														class="rounded-md px-3 py-1.5 text-xs font-medium text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700"
														onclick={() => { pendingDelete = null; }}
													>
														Cancel
													</button>
												{:else}
													<button
														class="rounded-md px-3 py-1.5 text-xs font-medium text-red-500 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
														onclick={() => { pendingDelete = filename; }}
													>
														Delete
													</button>
												{/if}
											</div>
										</td>
									</tr>

									<!-- Expanded metadata row -->
									{#if metaState[filename]}
										<tr class="border-b border-gray-100 bg-gray-50/70 dark:border-gray-700 dark:bg-gray-900/20">
											<td colspan="2" class="px-6 py-4">
												{#if metaState[filename] === 'loading'}
													<p class="text-xs text-gray-400">Loading metadata…</p>

												{:else if metaState[filename] === 'error'}
													<p class="text-xs text-red-500">Failed to load metadata.</p>

												{:else}
													{@const meta = metaState[filename] as MetaData}
													<dl class="grid grid-cols-2 gap-x-8 gap-y-2 sm:grid-cols-3 lg:grid-cols-5">
														{#each [
															{ label: 'Claim #',       value: meta.claim_number },
															{ label: 'Claimant',      value: meta.claimant_name },
															{ label: 'Date of Injury',value: meta.date_of_injury },
															{ label: 'Employer',      value: meta.employer },
															{ label: 'Adjuster',      value: meta.adjuster },
															{ label: 'Support',       value: meta.support },
															{ label: 'Claim Type',    value: meta.claim_type },
															{ label: 'Jurisdiction',  value: meta.jurisdiction },
															{ label: 'Policy #',      value: meta.policy_number },
															{ label: 'ACTs ID',       value: meta.acts_id }
														] as item}
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
		</main>
	</div>
</div>
