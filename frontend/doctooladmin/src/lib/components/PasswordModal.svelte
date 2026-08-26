<script lang="ts">
	import { credentials } from '$lib/stores/credentials.svelte';

	let value = $state('');

	function submit() {
		credentials.request?.submit(value);
		value = '';
	}

	function cancel() {
		credentials.request?.cancel();
		value = '';
	}
</script>

{#if credentials.request}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
		<div class="w-80 border border-gray-300 bg-white p-4 dark:border-gray-600 dark:bg-gray-800">
			<p class="mb-2 text-sm text-gray-700 dark:text-gray-300">{credentials.request.label}</p>
			<form
				onsubmit={(e) => {
					e.preventDefault();
					submit();
				}}
			>
				<input
					type="password"
					autofocus
					bind:value
					class="mb-3 w-full border border-gray-300 px-2 py-1 text-sm dark:border-gray-600 dark:bg-gray-700 dark:text-white"
				/>
				<div class="flex justify-end gap-2">
					<button
						type="button"
						onclick={cancel}
						class="px-3 py-1 text-sm text-gray-600 dark:text-gray-300"
					>
						Cancel
					</button>
					<button type="submit" class="px-3 py-1 text-sm text-gray-900 dark:text-white">
						Submit
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}
