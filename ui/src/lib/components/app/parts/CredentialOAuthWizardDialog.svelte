<script lang="ts">
	import CopyIcon from "@lucide/svelte/icons/copy";
	import ExternalLinkIcon from "@lucide/svelte/icons/external-link";
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import LogInIcon from "@lucide/svelte/icons/log-in";
	import type {
		CredentialOAuthKind,
		CredentialTypeOAuthScopeOption,
	} from "$lib/api-types";
	import { Button } from "$lib/components/ui/button";
	import * as Dialog from "$lib/components/ui/dialog";
	import { Input } from "$lib/components/ui/input";
	import CredentialOAuthScopePicker from "./CredentialOAuthScopePicker.svelte";

	type ScopeGroup = {
		group: string;
		scopes: CredentialTypeOAuthScopeOption[];
	};

	type Props = {
		open: boolean;
		title: string;
		providerName: string;
		openVerificationLabel: string;
		waitingForProviderLabel: string;
		deviceIntroLine1: string;
		deviceIntroLine2: string;
		deviceReturnText: string;
		closeLabel?: string;
		selectedOAuthKind: CredentialOAuthKind | null;
		hasScopeOptions: boolean;
		oauthScopePickerMode: "simple" | "advanced";
		defaultOAuthScopeOptions: CredentialTypeOAuthScopeOption[];
		advancedOAuthScopeGroups: ScopeGroup[];
		startingOAuth: boolean;
		pollingOAuth: boolean;
		copiedOAuthCode: boolean;
		copiedOAuthAuthUrl: boolean;
		oauthAuthUrl: string;
		oauthInputDraft: string;
		oauthVerifierDraft: string;
		oauthVerificationUrl: string;
		oauthUserCodeDraft: string;
		errorMessage: string | null;
		onOpenChange: (open: boolean) => void;
		onSelectOAuthKind: (kind: CredentialOAuthKind) => void;
		onSetScopePickerMode: (mode: "simple" | "advanced") => void;
		onResetScopesToDefaults: () => void;
		isOAuthScopeEnabled: (scope: string) => boolean;
		onSetOAuthScopeEnabled: (scope: string, enabled: boolean) => void;
		onOpenOAuthAuthUrl: () => void | Promise<void>;
		onCopyOAuthAuthUrl: () => void | Promise<void>;
		onSetOAuthInputDraft: (value: string) => void;
		onSubmitOAuthAuthorizationCode: () => void | Promise<void>;
		onStartOAuthAuthorization: () => void | Promise<void>;
		onOpenVerificationUrl: () => void | Promise<void>;
		onCopyOAuthCode: () => void | Promise<void>;
		onStartOAuthPolling: () => void | Promise<void>;
	};

	let {
		open,
		title,
		providerName,
		openVerificationLabel,
		waitingForProviderLabel,
		deviceIntroLine1,
		deviceIntroLine2,
		deviceReturnText,
		closeLabel = "Close",
		selectedOAuthKind,
		hasScopeOptions,
		oauthScopePickerMode,
		defaultOAuthScopeOptions,
		advancedOAuthScopeGroups,
		startingOAuth,
		pollingOAuth,
		copiedOAuthCode,
		copiedOAuthAuthUrl,
		oauthAuthUrl,
		oauthInputDraft,
		oauthVerifierDraft,
		oauthVerificationUrl,
		oauthUserCodeDraft,
		errorMessage,
		onOpenChange,
		onSelectOAuthKind,
		onSetScopePickerMode,
		onResetScopesToDefaults,
		isOAuthScopeEnabled,
		onSetOAuthScopeEnabled,
		onOpenOAuthAuthUrl,
		onCopyOAuthAuthUrl,
		onSetOAuthInputDraft,
		onSubmitOAuthAuthorizationCode,
		onStartOAuthAuthorization,
		onOpenVerificationUrl,
		onCopyOAuthCode,
		onStartOAuthPolling,
	}: Props = $props();
</script>

<Dialog.Root {open} {onOpenChange}>
	<Dialog.Content class="sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
		</Dialog.Header>
		<div class="space-y-4">
			<div class="rounded-md border border-border bg-muted/30 p-4">
				<div class="mb-2 text-sm font-medium">1. Choose a sign-in flow</div>
				<div class="grid gap-3 md:grid-cols-2">
					<button
						type="button"
						class={`rounded-lg border p-4 text-left transition-colors ${
							selectedOAuthKind === "authorization_code"
								? "border-primary bg-primary/5"
								: "border-border hover:bg-muted/60"
						}`}
						onclick={() => onSelectOAuthKind("authorization_code")}
					>
						<div class="flex items-center justify-between gap-2">
							<div class="font-medium">Redirect sign-in</div>
							<div class="text-xs text-muted-foreground">Recommended</div>
						</div>
						<p class="mt-1 text-sm text-muted-foreground">
							Open {providerName} in your browser and come back automatically after
							approval.
						</p>
					</button>
					<button
						type="button"
						class={`rounded-lg border p-4 text-left transition-colors ${
							selectedOAuthKind === "device_code"
								? "border-primary bg-primary/5"
								: "border-border hover:bg-muted/60"
						}`}
						onclick={() => onSelectOAuthKind("device_code")}
					>
						<div class="font-medium">Device code</div>
						<p class="mt-1 text-sm text-muted-foreground">
							Use a short code instead if the redirect flow does not work for
							you.
						</p>
					</button>
				</div>
			</div>

			{#if hasScopeOptions}
				<div class="rounded-md border border-border bg-muted/30 p-4">
					<div class="mb-2 text-sm font-medium">
						2. Choose {providerName} access
					</div>
					<CredentialOAuthScopePicker
						label=""
						mode={oauthScopePickerMode}
						simpleOptions={defaultOAuthScopeOptions}
						defaultOptions={defaultOAuthScopeOptions}
						advancedGroups={advancedOAuthScopeGroups}
						useBulletSummary={true}
						onModeChange={onSetScopePickerMode}
						onResetToDefaults={onResetScopesToDefaults}
						isEnabled={isOAuthScopeEnabled}
						onSetEnabled={onSetOAuthScopeEnabled}
					/>
				</div>
			{/if}

			<div class="rounded-md border border-border bg-muted/30 p-4">
				<div class="mb-2 text-sm font-medium">
					{hasScopeOptions ? "3." : "2."} Complete the {providerName} authorization
				</div>
				{#if selectedOAuthKind === "authorization_code"}
					<div class="space-y-3">
						<div class="flex flex-wrap gap-2">
							<Button
								variant="default"
								size="sm"
								class="gap-2"
								disabled={startingOAuth || pollingOAuth}
								onclick={() => void onOpenOAuthAuthUrl()}
							>
								{#if startingOAuth}
									<Loader2Icon class="size-4 animate-spin" />
									Preparing…
								{:else}
									<ExternalLinkIcon class="size-4" />
									Open Sign-On Link
								{/if}
							</Button>
							<Button
								variant="outline"
								size="sm"
								onclick={() => onSelectOAuthKind("device_code")}
							>
								Use device code instead
							</Button>
						</div>
						{#if copiedOAuthAuthUrl}
							<p class="text-xs text-muted-foreground">
								Sign-in URL copied to clipboard.
							</p>
						{/if}
						{#if oauthAuthUrl}
							<div class="rounded-md border border-border bg-background p-3">
								<div
									class="text-xs font-medium uppercase tracking-wide text-muted-foreground"
								>
									Sign-in URL
								</div>
								<div class="mt-2 flex items-start gap-2">
									<div
										class="min-w-0 flex-1 break-all font-mono text-xs text-muted-foreground"
									>
										{oauthAuthUrl}
									</div>
									<Button
										variant="ghost"
										size="icon"
										class="h-7 w-7 shrink-0"
										disabled={startingOAuth || pollingOAuth}
										aria-label="Copy sign-in URL"
										onclick={() => void onCopyOAuthAuthUrl()}
									>
										<CopyIcon class="size-4" />
									</Button>
								</div>
							</div>
						{/if}
						<div
							class="space-y-3 rounded-md border border-border bg-background p-3"
						>
							<div class="space-y-1">
								<label
									class="text-sm font-medium"
									for="oauth-authorization-input"
								>
									Paste the redirect URL or authorization code
								</label>
								<p class="text-sm text-muted-foreground">
									If the browser does not return here automatically, paste the
									full callback URL or just the <code>code</code> value and Discobot
									will extract the authorization code.
								</p>
							</div>
							<Input
								id="oauth-authorization-input"
								placeholder="http://127.0.0.1:1455/auth/callback?... or code"
								value={oauthInputDraft}
								oninput={(event) =>
									onSetOAuthInputDraft(event.currentTarget.value)}
							/>
							<div class="flex flex-wrap gap-2">
								<Button
									size="sm"
									disabled={pollingOAuth || !oauthVerifierDraft.trim()}
									onclick={() => void onSubmitOAuthAuthorizationCode()}
								>
									{#if pollingOAuth}
										<Loader2Icon class="size-4 animate-spin" />
										Connecting…
									{:else}
										Connect with pasted code
									{/if}
								</Button>
							</div>
						</div>
					</div>
				{:else}
					<div class="space-y-3">
						<ul class="list-disc space-y-2 pl-5 text-sm text-muted-foreground">
							<li>{deviceIntroLine1}</li>
							<li>{deviceIntroLine2}</li>
							<li>{deviceReturnText}</li>
						</ul>
						<div class="flex flex-wrap gap-2">
							<Button
								variant="default"
								size="sm"
								class="gap-2"
								disabled={startingOAuth || pollingOAuth}
								onclick={() => void onStartOAuthAuthorization()}
							>
								{#if startingOAuth}
									<Loader2Icon class="size-4 animate-spin" />
									Starting…
								{:else}
									<LogInIcon class="size-4" />
									Get device code
								{/if}
							</Button>
							{#if oauthVerificationUrl}
								<Button
									variant="outline"
									size="sm"
									class="gap-2"
									disabled={pollingOAuth}
									onclick={() => void onOpenVerificationUrl()}
								>
									<ExternalLinkIcon class="size-4" />
									{openVerificationLabel}
								</Button>
							{/if}
						</div>
						{#if oauthUserCodeDraft}
							<div
								class="space-y-3 rounded-md border border-border bg-background p-4"
							>
								<div class="text-sm text-muted-foreground">
									Enter this code at
									<code class="mx-1 font-mono">{oauthVerificationUrl}</code>.
								</div>
								<div class="flex items-center gap-2">
									<div class="flex-1 rounded-lg bg-muted/40 p-4 text-center">
										<code class="text-xl font-bold tracking-[0.3em]">
											{oauthUserCodeDraft}
										</code>
									</div>
									<Button
										variant="outline"
										size="icon"
										class="h-14 w-14"
										disabled={pollingOAuth}
										aria-label="Copy device code"
										onclick={() => void onCopyOAuthCode()}
									>
										<CopyIcon class="size-5" />
									</Button>
								</div>
								{#if copiedOAuthCode}
									<p class="text-xs text-center text-muted-foreground">
										Copied to clipboard
									</p>
								{/if}
								<div class="flex flex-wrap items-center justify-between gap-2">
									<div class="text-sm text-muted-foreground">
										After you approve the device in {providerName}, come back
										here.
									</div>
									<Button
										size="sm"
										disabled={pollingOAuth}
										onclick={() => void onStartOAuthPolling()}
									>
										{#if pollingOAuth}
											<Loader2Icon class="size-4 animate-spin" />
											{waitingForProviderLabel}
										{:else}
											I entered the code
										{/if}
									</Button>
								</div>
								{#if pollingOAuth}
									<div class="flex items-center gap-2 text-sm text-foreground">
										<Loader2Icon class="size-4 animate-spin" />
										Waiting for {providerName} to finish authorization.
									</div>
								{/if}
							</div>
						{/if}
					</div>
				{/if}
				{#if errorMessage}
					<p class="mt-3 text-sm text-destructive">{errorMessage}</p>
				{/if}
			</div>
		</div>
		<Dialog.Footer>
			<Button variant="ghost" size="sm" onclick={() => onOpenChange(false)}>
				{closeLabel}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
