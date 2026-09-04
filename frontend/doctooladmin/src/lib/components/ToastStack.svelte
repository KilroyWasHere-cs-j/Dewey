<script lang="ts">
	import { fly } from 'svelte/transition';
	import { toasts } from '$lib/stores/toasts.svelte';
	import type { Toast } from '$lib/stores/toasts.svelte';

	// Same border/bg/text recipe as the analytics page's alert banners
	// (red/amber-950 cards with a matching border and text tint) so toasts
	// read as the same design language instead of a separate one.
	const KIND_CLASSES: Record<Toast['kind'], string> = {
		error: 'border-red-500/40 bg-red-950/90 text-red-300',
		warning: 'border-amber-500/40 bg-amber-950/90 text-amber-300',
		info: 'border-blue-500/40 bg-blue-950/90 text-blue-300'
	};

	const ICON_CLASSES: Record<Toast['kind'], string> = {
		error: 'text-red-400',
		warning: 'text-amber-400',
		info: 'text-blue-400'
	};
</script>

<!-- Bottom-right stack, newest toast nearest the corner (flex-col-reverse)
     since that's the conventional toast placement/growth direction. -->
<div class="fixed right-4 bottom-4 z-50 flex w-full max-w-sm flex-col-reverse gap-2">
	{#each toasts.items as toast (toast.id)}
		<div
			role="alert"
			transition:fly={{ x: 24, duration: 200 }}
			class="flex items-start gap-3 rounded-lg border px-4 py-3 text-sm shadow-lg backdrop-blur {KIND_CLASSES[
				toast.kind
			]}"
		>
			<svg
				aria-hidden="true"
				class="mt-0.5 h-4 w-4 shrink-0 {ICON_CLASSES[toast.kind]}"
				fill="none"
				viewBox="0 0 24 24"
				stroke="currentColor"
				stroke-width="2"
			>
				<path
					stroke-linecap="round"
					stroke-linejoin="round"
					d="M12 9v3m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z"
				/>
			</svg>
			<div class="min-w-0 flex-1">
				<p class="font-medium">{toast.title}</p>
				{#if toast.detail}
					<p class="mt-0.5 text-xs opacity-80">{toast.detail}</p>
				{/if}
				{#if toast.action}
					<button
						onclick={() => {
							toast.action?.onClick();
							toasts.dismiss(toast.id);
						}}
						class="mt-1.5 text-xs font-semibold underline underline-offset-2 opacity-90 hover:opacity-100"
					>
						{toast.action.label}
					</button>
				{/if}
			</div>
			<button
				aria-label="Dismiss"
				onclick={() => toasts.dismiss(toast.id)}
				class="shrink-0 rounded p-0.5 opacity-70 hover:opacity-100"
			>
				<svg
					aria-hidden="true"
					class="h-3.5 w-3.5"
					fill="none"
					viewBox="0 0 24 24"
					stroke="currentColor"
					stroke-width="2"
				>
					<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
				</svg>
			</button>
		</div>
	{/each}
</div>
