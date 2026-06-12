<script lang="ts">
	import type { GlobalShortcut } from "$lib/shortcuts/global-shortcuts";
	import { Kbd, KbdGroup } from "$lib/components/ui/kbd";

	type Props = {
		open: boolean;
		shortcuts: GlobalShortcut[];
	};

	let { open, shortcuts }: Props = $props();
</script>

{#if open}
	<div
		class="pointer-events-none absolute inset-0 z-40 flex items-start justify-center bg-background/20 px-4 pt-24 backdrop-blur-[2px]"
	>
		<div
			role="dialog"
			aria-modal="true"
			aria-labelledby="keyboard-shortcuts-title"
			class="pointer-events-auto w-full max-w-xl overflow-hidden rounded-2xl border border-border/80 bg-background/95 shadow-2xl"
		>
			<div class="border-b border-border/70 px-4 py-3">
				<h2
					id="keyboard-shortcuts-title"
					class="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground"
				>
					Keyboard shortcuts
				</h2>
			</div>

			<div class="space-y-1 p-2">
				{#each shortcuts as shortcut (shortcut.id)}
					<div
						class="flex items-center justify-between gap-4 rounded-xl px-3 py-3"
					>
						<span class="text-sm text-foreground/85">{shortcut.label}</span>
						<div class="flex flex-wrap items-center justify-end gap-2">
							{#each shortcut.keyGroups as keyGroup, index (keyGroup.join("+"))}
								<KbdGroup>
									{#each keyGroup as key (key)}
										<Kbd>{key}</Kbd>
									{/each}
								</KbdGroup>
								{#if index < shortcut.keyGroups.length - 1}
									<span class="text-xs text-muted-foreground">or</span>
								{/if}
							{/each}
						</div>
					</div>
				{/each}
			</div>
		</div>
	</div>
{/if}
