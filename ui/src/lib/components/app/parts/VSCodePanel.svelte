<script lang="ts">
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import { api } from "$lib/api-client";
	import { appendAuthToken, getApiRootBase } from "$lib/api-config";
	import DockWindowChrome from "$lib/components/app/parts/DockWindowChrome.svelte";
	import { Button } from "$lib/components/ui/button";
	import type { ResolvedTheme } from "$lib/theme";
	import type { ServiceItem } from "$lib/shell-types";

	type Props = {
		dockMaximized: boolean;
		onClose: () => void;
		onToggleDockMaximized: () => void;
		resolvedTheme: ResolvedTheme;
		sessionId: string;
		service: ServiceItem | null;
	};

	const VSCODE_THEME_FILE_PATH = ".discobot/.vscode-theme.json";
	const GIT_HEAD_PATH = ".git/HEAD";
	const GIT_EXCLUDE_PATH = ".git/info/exclude";
	const GIT_EXCLUDE_ENTRY = `${VSCODE_THEME_FILE_PATH}\n`;

	let {
		dockMaximized,
		onClose,
		onToggleDockMaximized,
		resolvedTheme,
		sessionId,
		service,
	}: Props = $props();

	let iframeElement = $state<HTMLIFrameElement | null>(null);
	let isLoading = $state(true);
	let error = $state<string | null>(null);
	let refreshKey = $state(0);
	let lastSyncedThemeKey = $state<string | null>(null);

	const maximizeTitle = $derived.by(() =>
		dockMaximized ? "Restore split view" : "Maximize editor panel",
	);
	const serviceUrl = $derived.by(() =>
		service ? buildServiceUrl(sessionId, service, service.urlPath ?? "/") : "",
	);
	const iframeKey = $derived.by(
		() => `${service?.id ?? "vscode"}-${refreshKey}-${service?.urlPath ?? "/"}`,
	);

	function normalizePath(path?: string): string {
		const trimmed = path?.trim() ?? "";
		if (!trimmed) {
			return "/";
		}
		return trimmed.startsWith("/") ? trimmed : `/${trimmed}`;
	}

	function buildServiceUrl(
		nextSessionId: string,
		nextService: ServiceItem,
		nextPath: string,
	): string {
		if (typeof window === "undefined") {
			return "";
		}

		const apiRoot = getApiRootBase();
		const parsed = new URL(apiRoot);
		const subdomain = `${nextSessionId}-svc-${nextService.id}`;
		const protocol =
			typeof nextService.https === "number" ? "https:" : parsed.protocol;
		const path = normalizePath(nextPath);
		return appendAuthToken(`${protocol}//${subdomain}.${parsed.host}${path}`);
	}

	function buildThemePayload(nextTheme: ResolvedTheme): string {
		return JSON.stringify({ theme: nextTheme }, null, "\t");
	}

	async function ensureThemeFileIsGitIgnored(nextSessionId: string) {
		try {
			await api.readSessionFile(nextSessionId, GIT_HEAD_PATH);
		} catch {
			return;
		}

		let currentExclude = "";
		try {
			const response = await api.readSessionFile(
				nextSessionId,
				GIT_EXCLUDE_PATH,
			);
			currentExclude = response.content;
		} catch {
			currentExclude = "";
		}

		if (currentExclude.includes(VSCODE_THEME_FILE_PATH)) {
			return;
		}

		const separator =
			currentExclude.length > 0 && !currentExclude.endsWith("\n") ? "\n" : "";
		await api.writeSessionFile(nextSessionId, {
			path: GIT_EXCLUDE_PATH,
			content: `${currentExclude}${separator}${GIT_EXCLUDE_ENTRY}`,
		});
	}

	function refreshPreview() {
		isLoading = true;
		error = null;
		refreshKey += 1;
	}

	function handleIframeLoad() {
		isLoading = false;
		error = null;
	}

	function handleIframeError() {
		isLoading = false;
		error = "Failed to load editor";
	}

	$effect(() => {
		void serviceUrl;
		isLoading = true;
		error = null;
	});

	$effect(() => {
		void service;
		void resolvedTheme;
		void sessionId;

		if (typeof window === "undefined") {
			return;
		}

		const themeKey = `${sessionId}:${resolvedTheme}`;
		if (lastSyncedThemeKey === themeKey) {
			return;
		}

		let cancelled = false;
		void (async () => {
			try {
				await ensureThemeFileIsGitIgnored(sessionId);
				if (cancelled) {
					return;
				}
				await api.writeSessionFile(sessionId, {
					path: VSCODE_THEME_FILE_PATH,
					content: buildThemePayload(resolvedTheme),
				});
				if (!cancelled) {
					lastSyncedThemeKey = themeKey;
				}
			} catch (syncError) {
				if (!cancelled) {
					console.error("Failed to sync editor theme", syncError);
				}
			}
		})();

		return () => {
			cancelled = true;
		};
	});
</script>

<DockWindowChrome
	{dockMaximized}
	{onClose}
	{onToggleDockMaximized}
	closeLabel="Close editor panel"
	minimizeLabel="Minimize editor panel"
	{maximizeTitle}
	shellClass="min-h-[28rem]"
	contentClass="min-h-0 min-w-0 flex-1 overflow-hidden"
>
	{#snippet title()}
		<p class="truncate text-sm font-medium">Editor</p>
	{/snippet}

	<div class="flex h-full min-h-0 min-w-0 flex-col p-0">
		{#if service}
			<div
				class="relative min-h-0 flex-1 overflow-hidden border border-sidebar-border bg-background"
			>
				{#if isLoading}
					<div
						class="absolute inset-0 z-10 flex items-center justify-center bg-background/80 backdrop-blur-sm"
					>
						<div class="flex items-center gap-2 text-sm text-muted-foreground">
							<Loader2Icon class="size-4 animate-spin" />
							<span>Loading editor…</span>
						</div>
					</div>
				{/if}

				{#if error}
					<div
						class="absolute inset-0 z-20 flex items-center justify-center bg-background/90"
					>
						<div class="space-y-3 text-center text-sm text-muted-foreground">
							<p>{error}</p>
							<div class="flex items-center justify-center gap-2">
								<Button variant="outline" size="sm" onclick={refreshPreview}
									>Retry</Button
								>
							</div>
						</div>
					</div>
				{/if}

				{#key iframeKey}
					<iframe
						bind:this={iframeElement}
						src={serviceUrl}
						class="size-full border-0"
						onload={handleIframeLoad}
						onerror={handleIframeError}
						title="Editor"
						allow="clipboard-read; clipboard-write; fullscreen"
						referrerpolicy="no-referrer"
					></iframe>
				{/key}
			</div>
		{:else}
			<div
				class="flex h-full items-center justify-center rounded-md border border-dashed border-sidebar-border bg-sidebar/30 p-6 text-center"
			>
				<div class="space-y-2 text-sm text-muted-foreground">
					<p class="font-medium text-sidebar-foreground">
						Editor is not available for this session.
					</p>
					<p>
						Install <code class="rounded bg-muted px-1 py-0.5 text-xs"
							>code-server</code
						>
						in the sandbox image to enable the built-in editor service.
					</p>
				</div>
			</div>
		{/if}
	</div>
</DockWindowChrome>
