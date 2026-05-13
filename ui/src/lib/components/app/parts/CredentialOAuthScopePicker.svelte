<script lang="ts">
	import type { CredentialTypeOAuthScopeOption } from "$lib/api-types";
	import { Button } from "$lib/components/ui/button";
	import { Label } from "$lib/components/ui/label";

	type ScopeGroup = {
		group: string;
		scopes: CredentialTypeOAuthScopeOption[];
	};

	type Props = {
		label?: string;
		mode: "simple" | "advanced";
		simpleOptions: CredentialTypeOAuthScopeOption[];
		defaultOptions?: CredentialTypeOAuthScopeOption[];
		advancedGroups: ScopeGroup[];
		useBulletSummary?: boolean;
		onModeChange: (mode: "simple" | "advanced") => void;
		onResetToDefaults?: () => void;
		isEnabled: (scope: string) => boolean;
		onSetEnabled: (scope: string, enabled: boolean) => void;
	};

	let {
		label = "Requested scopes",
		mode,
		simpleOptions,
		defaultOptions = [],
		advancedGroups,
		useBulletSummary = false,
		onModeChange,
		onResetToDefaults,
		isEnabled,
		onSetEnabled,
	}: Props = $props();
</script>

<div class="space-y-2">
	<div class="flex items-center justify-between gap-2">
		<Label>{label}</Label>
		<div class="flex gap-2">
			{#if mode === "advanced" && onResetToDefaults}
				<Button variant="outline" size="xs" onclick={onResetToDefaults}>
					Back to defaults
				</Button>
			{/if}
			<Button
				variant={mode === "simple" ? "default" : "outline"}
				size="xs"
				onclick={() => onModeChange("simple")}
			>
				Simple
			</Button>
			<Button
				variant={mode === "advanced" ? "default" : "outline"}
				size="xs"
				onclick={() => onModeChange("advanced")}
			>
				Advanced
			</Button>
		</div>
	</div>

	{#if mode === "simple"}
		{#if useBulletSummary}
			<div class="space-y-2">
				<ul class="space-y-1 text-sm text-muted-foreground">
					{#each defaultOptions as scopeOption, __key0 (__key0)}
						<li class="flex gap-2">
							<span
								class="mt-[0.45rem] size-1 rounded-full bg-muted-foreground/60"
							></span>
							<div>
								<span class="font-medium text-foreground">
									{scopeOption.simpleLabel ?? scopeOption.label}
								</span>
								<span>
									— {scopeOption.simpleHelpText ??
										scopeOption.description ??
										scopeOption.label}
								</span>
							</div>
						</li>
					{/each}
				</ul>
				<div class="pt-1">
					<Button
						variant="outline"
						size="sm"
						onclick={() => onModeChange("advanced")}
					>
						Customize
					</Button>
				</div>
			</div>
		{:else}
			<div
				class="max-h-[min(24rem,50vh)] space-y-2 overflow-y-auto rounded-md border border-border bg-muted/40 p-3"
			>
				{#each simpleOptions as scopeOption, __key1 (__key1)}
					<label class="flex items-start gap-2 text-sm">
						<input
							type="checkbox"
							checked={isEnabled(scopeOption.value)}
							onchange={(event) =>
								onSetEnabled(
									scopeOption.value,
									(event.currentTarget as HTMLInputElement).checked,
								)}
						/>
						<div class="space-y-0.5">
							<div class="font-medium">
								{scopeOption.simpleLabel ?? scopeOption.label}
							</div>
							{#if scopeOption.simpleHelpText}
								<div class="text-muted-foreground">
									{scopeOption.simpleHelpText}
								</div>
							{/if}
						</div>
					</label>
				{/each}
			</div>
		{/if}
	{:else}
		<div
			class:text-sm={!useBulletSummary}
			class="max-h-[18rem] space-y-3 overflow-y-auto rounded-md border border-border bg-background p-3"
		>
			{#each advancedGroups as scopeGroup, __key2 (__key2)}
				<div class="space-y-2">
					<div
						class="text-xs font-medium uppercase tracking-wide text-muted-foreground"
					>
						{scopeGroup.group}
					</div>
					<div class="space-y-2">
						{#each scopeGroup.scopes as scopeOption, __key3 (__key3)}
							<label class="flex items-start gap-2 text-sm">
								<input
									type="checkbox"
									checked={isEnabled(scopeOption.value)}
									onchange={(event) =>
										onSetEnabled(
											scopeOption.value,
											(event.currentTarget as HTMLInputElement).checked,
										)}
								/>
								<div class="space-y-0.5">
									<div class="flex items-center gap-2">
										<div class="font-mono text-xs">{scopeOption.label}</div>
										{#if scopeOption.access}
											<div
												class="rounded border border-border px-1.5 py-0.5 text-[10px] uppercase tracking-wide text-muted-foreground"
											>
												{scopeOption.access}
											</div>
										{/if}
									</div>
									{#if scopeOption.description}
										<div class="text-muted-foreground">
											{scopeOption.description}
										</div>
									{/if}
								</div>
							</label>
						{/each}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>
