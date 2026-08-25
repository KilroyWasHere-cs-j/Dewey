<script lang="ts">
	import { onMount } from 'svelte';
	import AppShell from '$lib/components/AppShell.svelte';
	import { credentials } from '$lib/stores/credentials.svelte';

	interface Machine {
		ip: string;
		label: string;
		added_at: string;
		last_seen_at: string;
	}

	// ── Machine list ──────────────────────────────────────────────────────────

	let machines = $state<Machine[]>([]);
	let listLoading = $state(true);
	let listError = $state<string | null>(null);

	async function loadMachines() {
		listLoading = true;
		listError = null;
		try {
			const res = await fetch('/api/machines', {
				headers: { 'X-Dewey-Password': credentials.getMachinesPassword() }
			});
			if (!res.ok) throw new Error(`HTTP ${res.status}`);
			const data = await res.json();
			machines = data.machines ?? [];
		} catch (e) {
			listError = String(e);
		} finally {
			listLoading = false;
		}
	}

	onMount(loadMachines);

	// ── Add ───────────────────────────────────────────────────────────────────

	let addOpen = $state(false);
	let adding = $state(false);
	let addError = $state<string | null>(null);
	let addFields = $state({ ip: '', label: '' });

	async function submitAdd() {
		if (!addFields.ip.trim() || !addFields.label.trim()) {
			addError = 'IP and label are both required.';
			return;
		}

		addError = null;
		adding = true;

		try {
			const res = await fetch('/api/machines', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					'X-Dewey-Password': credentials.getMachinesPassword()
				},
				body: JSON.stringify(addFields)
			});
			const body = await res.json();
			if (!res.ok) {
				addError = body.error ?? `HTTP ${res.status}`;
				return;
			}
			addFields = { ip: '', label: '' };
			addOpen = false;
			await loadMachines();
		} catch (e) {
			addError = String(e);
		} finally {
			adding = false;
		}
	}

	// ── Remove ────────────────────────────────────────────────────────────────

	let pendingRemove = $state<string | null>(null);
	let removing = $state(false);
	let removeError = $state<string | null>(null);

	async function confirmRemove() {
		if (!pendingRemove) return;
		removing = true;
		removeError = null;
		const target = pendingRemove;
		try {
			const res = await fetch(`/api/machines/${encodeURIComponent(target)}`, {
				method: 'DELETE',
				headers: { 'X-Dewey-Password': credentials.getMachinesPassword() }
			});
			if (!res.ok) throw new Error(`HTTP ${res.status}`);
			machines = machines.filter((m) => m.ip !== target);
			pendingRemove = null;
		} catch (e) {
			// Inline alert next to the row's Confirm/Cancel buttons (issue #241),
			// matching addError/listError elsewhere on this page instead of a
			// blocking native alert().
			removeError = String(e);
		} finally {
			removing = false;
		}
	}
</script>

<svelte:head>
	<title>Known Machines — Dewey</title>
</svelte:head>

<AppShell>
	{#snippet children({ headingClass })}
		<!-- ── Header ─────────────────────────────────────────────────────── -->
		<div class="flex items-center justify-between">
			<h1 class="text-2xl font-bold text-gray-800 dark:text-white">Known Machines</h1>
			<div class="flex gap-2">
				<button
					class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm text-gray-600 shadow-sm hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-300 dark:hover:bg-gray-600"
					onclick={loadMachines}
				>
					Refresh
				</button>
				<button
					class="rounded-lg px-4 py-2 text-sm font-medium text-white shadow-sm transition-colors {addOpen
						? 'bg-gray-600 hover:bg-gray-700'
						: 'bg-gray-900 hover:bg-gray-800 dark:bg-gray-700 dark:hover:bg-gray-600'}"
					onclick={() => {
						addOpen = !addOpen;
						addError = null;
					}}
				>
					{addOpen ? 'Cancel' : 'Add Machine'}
				</button>
			</div>
		</div>

		<!-- ── Add Panel ──────────────────────────────────────────────────── -->
		{#if addOpen}
			<section class="rounded-2xl bg-white p-6 shadow-sm dark:bg-gray-800">
				<h2 class="mb-4 text-xs font-semibold tracking-wider uppercase {headingClass}">
					Register New Machine
				</h2>

				<form
					onsubmit={(e) => {
						e.preventDefault();
						submitAdd();
					}}
					class="space-y-4"
				>
					<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
						<div>
							<label
								for="machine-ip"
								class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
							>
								IP Address <span class="text-red-500">*</span>
							</label>
							<input
								id="machine-ip"
								type="text"
								autocomplete="off"
								placeholder="10.0.0.5"
								class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 font-mono text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
								bind:value={addFields.ip}
							/>
						</div>
						<div>
							<label
								for="machine-label"
								class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300"
							>
								Label <span class="text-red-500">*</span>
							</label>
							<input
								id="machine-label"
								type="text"
								autocomplete="off"
								placeholder="vm-2"
								class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
								bind:value={addFields.label}
							/>
						</div>
					</div>

					{#if addError}
						<p role="alert" class="text-sm text-red-600 dark:text-red-400">{addError}</p>
					{/if}

					<button
						type="submit"
						disabled={adding}
						class="rounded-lg bg-gray-900 px-6 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:opacity-50 dark:bg-gray-700 dark:hover:bg-gray-600"
					>
						{adding ? 'Adding…' : 'Add'}
					</button>
				</form>
			</section>
		{/if}

		<!-- ── Machine List ───────────────────────────────────────────────── -->
		<section class="rounded-2xl bg-white shadow-sm dark:bg-gray-800">
			<div
				class="flex flex-wrap items-center gap-3 border-b border-gray-100 px-6 py-4 dark:border-gray-700"
			>
				<h2 class="shrink-0 text-xs font-semibold tracking-wider uppercase {headingClass}">
					Registered Machines
				</h2>

				{#if !listLoading}
					<span class="shrink-0 text-xs text-gray-400 dark:text-gray-500">
						{machines.length}
						{machines.length === 1 ? 'machine' : 'machines'}
					</span>
				{/if}
			</div>

			{#if listLoading}
				<p class="px-6 py-8 text-sm text-gray-500 dark:text-gray-400">Loading…</p>
			{:else if listError}
				<p role="alert" class="px-6 py-8 text-sm text-red-600 dark:text-red-400">
					Failed to load machines: {listError}
				</p>
			{:else if machines.length === 0}
				<p class="px-6 py-8 text-sm text-gray-500 dark:text-gray-400">No machines registered.</p>
			{:else}
				<div class="overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr
								class="border-b border-gray-100 bg-gray-50 text-left dark:border-gray-700 dark:bg-gray-900/40"
							>
								<th class="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">IP Address</th>
								<th class="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">Label</th>
								<th class="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">Added</th>
								<th class="px-6 py-3 font-medium text-gray-500 dark:text-gray-400">Last Seen</th>
								<th class="px-6 py-3 text-right font-medium text-gray-500 dark:text-gray-400"
									>Actions</th
								>
							</tr>
						</thead>
						<tbody>
							{#each machines as machine (machine.ip)}
								<tr
									class="border-b border-gray-50 hover:bg-gray-50/50 dark:border-gray-700/50 dark:hover:bg-gray-700/20"
								>
									<td class="px-6 py-3 font-mono text-gray-800 dark:text-gray-200">{machine.ip}</td>
									<td class="px-6 py-3 text-gray-700 dark:text-gray-300">{machine.label}</td>
									<td class="px-6 py-3 text-gray-500 dark:text-gray-400"
										>{machine.added_at || '—'}</td
									>
									<td class="px-6 py-3 text-gray-500 dark:text-gray-400"
										>{machine.last_seen_at || 'Never'}</td
									>
									<td class="px-6 py-3">
										<div class="flex items-center justify-end gap-2">
											{#if pendingRemove === machine.ip}
												<span class="text-xs text-gray-500 dark:text-gray-400">Are you sure?</span>
												<button
													disabled={removing}
													class="rounded-md px-3 py-1.5 text-xs font-medium text-red-600 hover:bg-red-50 disabled:opacity-50 dark:text-red-400 dark:hover:bg-red-900/20"
													onclick={confirmRemove}
												>
													{removing ? 'Removing…' : 'Confirm'}
												</button>
												<button
													class="rounded-md px-3 py-1.5 text-xs font-medium text-gray-500 hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-gray-700"
													onclick={() => {
														pendingRemove = null;
														removeError = null;
													}}
												>
													Cancel
												</button>
												{#if removeError}
													<span role="alert" class="text-xs text-red-600 dark:text-red-400"
														>{removeError}</span
													>
												{/if}
											{:else}
												<button
													class="rounded-md px-3 py-1.5 text-xs font-medium text-red-500 hover:bg-red-50 dark:text-red-400 dark:hover:bg-red-900/20"
													onclick={() => {
														pendingRemove = machine.ip;
														removeError = null;
													}}
												>
													Remove
												</button>
											{/if}
										</div>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			{/if}
		</section>
	{/snippet}
</AppShell>
