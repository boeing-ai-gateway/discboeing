<script lang="ts">
	import Loader2Icon from "@lucide/svelte/icons/loader-2";
	import MonitorIcon from "@lucide/svelte/icons/monitor";
	import RotateCcwIcon from "@lucide/svelte/icons/rotate-ccw";
	import { appendAuthToken, getApiRootBase } from "$lib/api-config";
	import DockWindowChrome from "$lib/components/app/parts/DockWindowChrome.svelte";
	import { Button } from "$lib/components/ui/button";
	import { readClipboardText, writeClipboardText } from "$lib/shell";
	import { cn } from "$lib/utils";

	type Props = {
		dockMaximized: boolean;
		sessionId: string;
		desktopAvailable: boolean;
		onClose: () => void;
		onToggleDockMaximized: () => void;
		shiftWindowControlsForSidebar?: boolean;
	};

	type ConnectionStatus =
		| "connecting"
		| "connected"
		| "disconnected"
		| "unavailable";

	type RFBEventMap = {
		connect: CustomEvent;
		disconnect: CustomEvent<{ clean: boolean }>;
		clipboard: CustomEvent<{ text: string }>;
		desktopname: CustomEvent<{ name: string }>;
	};

	type RFBInstance = {
		scaleViewport: boolean;
		resizeSession: boolean;
		background: string;
		disconnect: () => void;
		clipboardPasteFrom: (text: string) => void;
		sendKey: (keysym: number, code: string | null, down?: boolean) => void;
		addEventListener: <K extends keyof RFBEventMap>(
			type: K,
			listener: (event: RFBEventMap[K]) => void,
		) => void;
		removeEventListener: <K extends keyof RFBEventMap>(
			type: K,
			listener: (event: RFBEventMap[K]) => void,
		) => void;
	};

	type RFBConstructor = new (
		target: HTMLElement,
		urlOrChannel: string | WebSocket,
	) => RFBInstance;

	const DESKTOP_CONNECT_RETRY_DELAYS = [250, 750, 1500];

	let {
		sessionId,
		desktopAvailable,
		dockMaximized,
		onClose,
		onToggleDockMaximized,
		shiftWindowControlsForSidebar = false,
	}: Props = $props();

	let desktopHost = $state<HTMLDivElement | null>(null);
	let connectionStatus = $state<ConnectionStatus>("connecting");
	let desktopName = $state("");
	let reconnectVersion = $state(0);

	const maximizeTitle = $derived.by(() =>
		dockMaximized ? "Restore split view" : "Maximize desktop panel",
	);
	const statusLabel = $derived.by(() => {
		switch (connectionStatus) {
			case "connected":
				return "Connected";
			case "disconnected":
				return "Disconnected";
			case "unavailable":
				return "Unavailable";
			default:
				return "Connecting";
		}
	});
	const statusDotClass = $derived.by(() =>
		cn(
			"size-2 shrink-0 rounded-full",
			connectionStatus === "connected" && "bg-chart-2",
			connectionStatus === "connecting" && "bg-chart-5",
			connectionStatus === "disconnected" && "bg-destructive",
			connectionStatus === "unavailable" && "bg-sidebar-foreground/30",
		),
	);
	const overlayMessage = $derived.by(() => {
		if (!desktopAvailable) {
			return "Desktop is not available for this session.";
		}

		switch (connectionStatus) {
			case "connected":
				return "";
			case "disconnected":
				return "Desktop disconnected";
			default:
				return "Connecting to desktop...";
		}
	});
	const canReconnect = $derived.by(
		() => desktopAvailable && connectionStatus === "disconnected",
	);
	const titleLabel = $derived.by(() => desktopName.trim() || statusLabel);

	function getDesktopWsUrl(sessionId: string, host: HTMLElement) {
		const apiRoot = getApiRootBase();
		const parsed = new URL(apiRoot);
		const subdomain = `${sessionId}-svc-discobot-desktop`;
		const protocol = parsed.protocol === "https:" ? "wss:" : "ws:";
		const url = new URL(`${protocol}//${subdomain}.${parsed.host}`);
		const width = Math.floor(host.clientWidth);
		const height = Math.floor(host.clientHeight);
		if (width > 0 && height > 0) {
			url.searchParams.set("x", String(width));
			url.searchParams.set("y", String(height));
		}
		return appendAuthToken(url.toString());
	}

	function clearDesktopHost(host: HTMLDivElement) {
		while (host.firstChild) {
			host.removeChild(host.firstChild);
		}
	}

	function reconnectDesktop() {
		reconnectVersion += 1;
	}

	$effect(() => {
		const host = desktopHost;
		const currentSessionId = sessionId;
		const available = desktopAvailable;
		const reconnectAttempt = reconnectVersion;

		if (!host) {
			return;
		}

		clearDesktopHost(host);
		desktopName = "";

		if (!available) {
			connectionStatus = "unavailable";
			return;
		}

		connectionStatus = "connecting";

		let disposed = false;
		let retryTimer: ReturnType<typeof setTimeout> | null = null;
		let socket: WebSocket | null = null;
		let rfb: RFBInstance | null = null;
		let rfbConnected = false;
		let rfbDisconnected = false;

		const onConnect = () => {
			if (disposed) {
				return;
			}
			rfbConnected = true;
			connectionStatus = "connected";
		};

		const onDisconnect = () => {
			rfbDisconnected = true;
			if (disposed) {
				return;
			}
			connectionStatus = "disconnected";
		};

		const onClipboard = (event: CustomEvent<{ text: string }>) => {
			void writeClipboardText(event.detail.text).catch(() => {});
		};

		const onDesktopName = (event: CustomEvent<{ name: string }>) => {
			if (disposed) {
				return;
			}
			desktopName = event.detail.name;
		};

		const onKeyDown = (event: KeyboardEvent) => {
			if (!rfb) {
				return;
			}
			if ((event.ctrlKey || event.metaKey) && event.key === "v") {
				event.stopPropagation();
				void readClipboardText()
					.then((text) => {
						if (disposed || !rfb) {
							return;
						}
						rfb.clipboardPasteFrom(text);
						rfb.sendKey(0xffe3, "ControlLeft", true);
						rfb.sendKey(0x76, "KeyV", true);
						rfb.sendKey(0x76, "KeyV", false);
						rfb.sendKey(0xffe3, "ControlLeft", false);
					})
					.catch(() => {});
			}
		};

		const scheduleConnect = (connect: () => void, delay: number) => {
			retryTimer = setTimeout(() => {
				retryTimer = null;
				connect();
			}, delay);
		};

		void import("@novnc/novnc")
			.then((module) => {
				const RFB = module.default as unknown as RFBConstructor;
				const connect = (attempt: number) => {
					if (disposed || reconnectAttempt !== reconnectVersion) {
						return;
					}

					const nextSocket = new WebSocket(
						getDesktopWsUrl(currentSessionId, host),
					);
					socket = nextSocket;

					nextSocket.addEventListener("open", () => {
						if (
							disposed ||
							reconnectAttempt !== reconnectVersion ||
							socket !== nextSocket
						) {
							nextSocket.close();
							return;
						}

						socket = null;
						rfbDisconnected = false;
						rfb = new RFB(host, nextSocket);
						rfb.scaleViewport = true;
						rfb.resizeSession = true;
						rfb.background = "var(--background)";
						rfb.addEventListener("connect", onConnect);
						rfb.addEventListener("disconnect", onDisconnect);
						rfb.addEventListener("clipboard", onClipboard);
						rfb.addEventListener("desktopname", onDesktopName);
						host.addEventListener("keydown", onKeyDown, true);
					});

					nextSocket.addEventListener("close", () => {
						if (
							disposed ||
							reconnectAttempt !== reconnectVersion ||
							socket !== nextSocket
						) {
							return;
						}

						socket = null;
						const retryDelay = DESKTOP_CONNECT_RETRY_DELAYS[attempt];
						if (!rfbConnected && retryDelay !== undefined) {
							scheduleConnect(() => connect(attempt + 1), retryDelay);
							return;
						}

						connectionStatus = "disconnected";
					});
				};

				connect(0);
			})
			.catch((error) => {
				if (disposed) {
					return;
				}
				console.error("Failed to initialize desktop viewer", error);
				connectionStatus = "disconnected";
			});

		return () => {
			disposed = true;
			if (retryTimer) {
				clearTimeout(retryTimer);
			}
			if (socket) {
				socket.close();
				socket = null;
			}
			host.removeEventListener("keydown", onKeyDown, true);
			if (rfb) {
				rfb.removeEventListener("connect", onConnect);
				rfb.removeEventListener("disconnect", onDisconnect);
				rfb.removeEventListener("clipboard", onClipboard);
				rfb.removeEventListener("desktopname", onDesktopName);
				if (!rfbDisconnected) {
					rfb.disconnect();
				}
				rfb = null;
			}
			clearDesktopHost(host);
		};
	});
</script>

<DockWindowChrome
	{dockMaximized}
	{onClose}
	{onToggleDockMaximized}
	{shiftWindowControlsForSidebar}
	closeLabel="Close desktop panel"
	minimizeLabel="Minimize desktop panel"
	{maximizeTitle}
	shellClass="min-h-[28rem]"
	contentClass="min-h-0 min-w-0 flex-1 overflow-hidden"
>
	{#snippet title()}
		<div class="flex min-w-0 items-center gap-2 text-xs">
			<p class="truncate text-sm font-medium">Desktop</p>
			<span class="truncate text-sidebar-foreground/70">{titleLabel}</span>
			<div class={statusDotClass} aria-hidden="true"></div>
			<span class="sr-only" aria-live="polite"
				>Desktop status: {statusLabel}</span
			>
		</div>
	{/snippet}

	<div
		class="relative flex h-full min-h-0 min-w-0 items-center justify-center overflow-hidden p-3"
	>
		{#if connectionStatus !== "connected"}
			<div
				class="absolute inset-3 z-10 flex items-center justify-center rounded-md bg-background/80 backdrop-blur-sm"
			>
				<div class="flex max-w-xs flex-col items-center gap-3 px-4 text-center">
					{#if connectionStatus === "connecting"}
						<Loader2Icon class="size-8 animate-spin text-foreground/70" />
					{:else}
						<MonitorIcon class="size-8 text-foreground/70" />
					{/if}
					<span class="text-xs text-foreground/70">{overlayMessage}</span>
					{#if canReconnect}
						<Button
							variant="outline"
							size="xs"
							onclick={reconnectDesktop}
							class="gap-2"
						>
							<RotateCcwIcon class="size-3.5" />
							Reconnect
						</Button>
					{/if}
				</div>
			</div>
		{/if}

		<div
			bind:this={desktopHost}
			class="desktop-vnc-host h-full w-full overflow-hidden rounded-md border border-border bg-background"
		></div>
	</div>
</DockWindowChrome>

<style>
	.desktop-vnc-host > :global(div) {
		overflow: hidden !important;
		align-items: flex-start !important;
		justify-content: flex-start !important;
	}

	.desktop-vnc-host :global(canvas) {
		display: block !important;
		margin: 0 !important;
	}
</style>
