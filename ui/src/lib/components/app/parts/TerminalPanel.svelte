<script lang="ts">
	import "@wterm/dom/css";
	import "@xterm/xterm/css/xterm.css";
	import CheckIcon from "@lucide/svelte/icons/check";
	import CopyIcon from "@lucide/svelte/icons/copy";
	import RotateCcwIcon from "@lucide/svelte/icons/rotate-ccw";
	import { appendAuthToken, getSSHPort, getWsBase } from "$lib/api-config";
	import { openUrl, readClipboardText, writeClipboardText } from "$lib/shell";
	import DockWindowChrome from "$lib/components/app/parts/DockWindowChrome.svelte";
	import { Button } from "$lib/components/ui/button";
	import {
		ContextMenu,
		ContextMenuContent,
		ContextMenuItem,
		ContextMenuTrigger,
	} from "$lib/components/ui/context-menu";
	import { Switch } from "$lib/components/ui/switch";
	import type {
		ILink,
		ILinkProvider,
		ITheme,
		Terminal as GhosttyTerminal,
	} from "ghostty-web";
	import type { WTerm } from "@wterm/dom";
	import type { Terminal as XtermTerminal } from "@xterm/xterm";

	type ConnectionStatus = "disconnected" | "connecting" | "connected" | "error";
	type TerminalRenderer = "ghostty" | "xterm" | "wterm";
	type XtermLikeTerminal = GhosttyTerminal | XtermTerminal;
	type TerminalAdapter = {
		rows: number;
		cols: number;
		write(data: string): void;
		writeln(data: string): void;
		clear(): void;
		getSelection(): string;
		hasSelection(): boolean;
		focus(): void;
		dispose(): void;
	};

	type Props = {
		dockMaximized: boolean;
		onClose: () => void;
		onRootEnabledChange: (value: boolean) => void;
		onToggleDockMaximized: () => void;
		rootEnabled: boolean;
		sessionId: string | null;
		shiftWindowControlsForSidebar?: boolean;
	};

	const MIN_TERMINAL_ROWS = 20;
	const MIN_TERMINAL_COLS = 80;
	const RESIZE_DEBOUNCE_MS = 150;
	const COPY_RESET_MS = 2000;
	const GIT_PULL_WORKSPACE_PATH = "/home/discobot/workspace";
	const TERMINAL_RENDERERS: TerminalRenderer[] = ["ghostty", "wterm", "xterm"];

	let {
		dockMaximized,
		onClose,
		onRootEnabledChange,
		onToggleDockMaximized,
		rootEnabled,
		sessionId,
		shiftWindowControlsForSidebar = false,
	}: Props = $props();

	let terminalHost = $state<HTMLDivElement | null>(null);
	let connectionStatus = $state<ConnectionStatus>("disconnected");
	let copiedCommand = $state<"ssh" | "pull" | null>(null);
	let terminalCanCopy = $state(false);
	let terminalContextMenuOpen = $state(false);
	let terminalReady = $state(false);
	let terminalRenderer = $state<TerminalRenderer>("ghostty");

	let terminal: TerminalAdapter | null = null;
	let socket: WebSocket | null = null;
	let resizeTimeout: ReturnType<typeof setTimeout> | null = null;
	let copyTimeout: ReturnType<typeof setTimeout> | null = null;
	let dataSubscription: { dispose(): void } | null = null;
	let resizeSubscription: { dispose(): void } | null = null;
	let terminalResizeObserver: ResizeObserver | null = null;
	let windowResizeHandler: (() => void) | null = null;
	let lastSize: { rows: number; cols: number } | null = null;

	const statusClass = $derived.by(() => {
		if (connectionStatus === "connected") return "bg-terminal-fg";
		if (connectionStatus === "connecting") return "bg-ring";
		if (connectionStatus === "error") return "bg-destructive";
		return "bg-muted-foreground/50";
	});

	const sessionLabel = $derived.by(() => {
		if (!sessionId) return "No session";
		return `Session: ${sessionId}`;
	});

	const sshCommand = $derived.by(() => {
		if (!sessionId) return null;
		return `ssh -p ${getSSHPort()} ${sessionId}@${getSSHHost()}`;
	});

	const pullCommand = $derived.by(() => {
		if (!sessionId) return null;
		return `git pull "ssh://${sessionId}@${getSSHHost()}:${getSSHPort()}${GIT_PULL_WORKSPACE_PATH}" HEAD`;
	});

	const overlayMessage = $derived.by(() => {
		if (!sessionId) {
			return "No session selected.";
		}
		if (connectionStatus === "connecting") {
			return "Connecting…";
		}
		if (connectionStatus === "error") {
			return "Connection error — reopen the terminal panel to retry.";
		}
		if (connectionStatus === "disconnected") {
			return "Disconnected";
		}
		return "";
	});

	const maximizeTitle = $derived.by(() =>
		dockMaximized ? "Restore split view" : "Maximize terminal panel",
	);

	function getSSHHost(): string {
		if (typeof window === "undefined") return "localhost";
		const { hostname } = window.location;
		if (hostname === "127.0.0.1" || hostname === "::1") {
			return "localhost";
		}
		return hostname;
	}

	function readThemeValue(
		style: CSSStyleDeclaration,
		property: string,
		fallback: string,
	): string {
		const value = style.getPropertyValue(property).trim();
		return value.length > 0 ? value : fallback;
	}

	function getTerminalTheme(): ITheme {
		if (typeof window === "undefined") {
			return {
				background: "oklch(0.08 0 0)",
				foreground: "oklch(0.75 0.15 145)",
			};
		}

		const style = window.getComputedStyle(document.documentElement);
		const background = readThemeValue(
			style,
			"--terminal-bg",
			"oklch(0.08 0 0)",
		);
		const foreground = readThemeValue(
			style,
			"--terminal-fg",
			"oklch(0.75 0.15 145)",
		);
		const selectionBackground = readThemeValue(
			style,
			"--tree-selected",
			"oklch(0.65 0.15 250 / 0.2)",
		);

		return {
			background,
			foreground,
			cursor: foreground,
			cursorAccent: background,
			selectionBackground,
			selectionForeground: foreground,
			black: "#1e1e2e",
			red: "#f38ba8",
			green: "#a6e3a1",
			yellow: "#f9e2af",
			blue: "#89b4fa",
			magenta: "#cba6f7",
			cyan: "#94e2d5",
			white: "#cdd6f4",
			brightBlack: "#585b70",
			brightRed: "#f38ba8",
			brightGreen: "#a6e3a1",
			brightYellow: "#f9e2af",
			brightBlue: "#89b4fa",
			brightMagenta: "#cba6f7",
			brightCyan: "#94e2d5",
			brightWhite: "#a6adc8",
		};
	}

	function updateConnectionStatus(status: ConnectionStatus) {
		connectionStatus = status;
	}

	function clearResizeTimer() {
		if (resizeTimeout) {
			clearTimeout(resizeTimeout);
			resizeTimeout = null;
		}
	}

	function clearCopyTimer() {
		if (copyTimeout) {
			clearTimeout(copyTimeout);
			copyTimeout = null;
		}
	}

	function closeSocket() {
		if (socket) {
			socket.close();
			socket = null;
		}
	}

	function sendInput(data: string) {
		if (socket?.readyState !== WebSocket.OPEN) {
			return;
		}
		socket.send(JSON.stringify({ type: "input", data }));
	}

	function sendResize(rows: number, cols: number) {
		if (rows < MIN_TERMINAL_ROWS || cols < MIN_TERMINAL_COLS) {
			return;
		}

		if (lastSize?.rows === rows && lastSize?.cols === cols) {
			return;
		}

		clearResizeTimer();
		resizeTimeout = setTimeout(() => {
			if (lastSize?.rows === rows && lastSize?.cols === cols) {
				return;
			}

			lastSize = { rows, cols };
			if (socket?.readyState !== WebSocket.OPEN) {
				return;
			}

			socket.send(
				JSON.stringify({
					type: "resize",
					data: { rows, cols },
				}),
			);
		}, RESIZE_DEBOUNCE_MS);
	}

	function connect(nextSessionId: string | null, nextRootEnabled: boolean) {
		if (!terminal) {
			return;
		}

		closeSocket();
		clearResizeTimer();

		if (!nextSessionId) {
			updateConnectionStatus("disconnected");
			terminal.writeln(
				"\x1b[33mNo session selected. Select a session to connect to the terminal.\x1b[0m",
			);
			return;
		}

		updateConnectionStatus("connecting");
		const rows = terminal.rows;
		const cols = terminal.cols;
		const rootParam = nextRootEnabled ? "&root=true" : "";
		const wsUrl = appendAuthToken(
			`${getWsBase()}/sessions/${nextSessionId}/terminal/ws?rows=${rows}&cols=${cols}&workdir=workspace${rootParam}`,
		);
		const nextSocket = new WebSocket(wsUrl);
		socket = nextSocket;

		nextSocket.onopen = () => {
			if (socket !== nextSocket) {
				return;
			}
			updateConnectionStatus("connected");
		};

		nextSocket.onmessage = (event) => {
			if (socket !== nextSocket) {
				return;
			}

			try {
				const message = JSON.parse(event.data as string) as {
					type?: string;
					data?: string;
				};
				if (message.type === "output" && typeof message.data === "string") {
					terminal?.write(message.data);
					return;
				}
				if (message.type === "error" && typeof message.data === "string") {
					terminal?.writeln(`\x1b[31mError: ${message.data}\x1b[0m`);
				}
			} catch {
				if (typeof event.data === "string") {
					terminal?.write(event.data);
				}
			}
		};

		nextSocket.onerror = () => {
			if (socket !== nextSocket) {
				return;
			}
			updateConnectionStatus("error");
		};

		nextSocket.onclose = (event) => {
			if (socket !== nextSocket) {
				return;
			}
			socket = null;
			updateConnectionStatus(event.wasClean ? "disconnected" : "error");
		};
	}

	function getTerminalCopyShortcut(
		event: KeyboardEvent,
	): "ctrl-shift" | "meta" | null {
		const key = event.key.toLowerCase();
		if (key !== "c" || event.altKey) {
			return null;
		}

		if (event.ctrlKey && event.shiftKey && !event.metaKey) {
			return "ctrl-shift";
		}

		if (event.metaKey && !event.ctrlKey && !event.shiftKey) {
			return "meta";
		}

		return null;
	}

	function copyTerminalSelection() {
		const selection = terminal?.getSelection() ?? "";
		if (selection.length === 0) {
			return;
		}

		void writeClipboardText(selection);
	}

	async function pasteClipboardIntoTerminal() {
		if (!terminalReady || !terminal) {
			return;
		}

		terminal.focus();
		const text = await readClipboardText();
		if (text.length > 0) {
			sendInput(text);
		}
	}

	$effect(() => {
		if (!terminalContextMenuOpen) {
			return;
		}

		terminalCanCopy = terminal?.hasSelection() ?? false;
	});

	async function copyCommand(command: string | null, kind: "ssh" | "pull") {
		if (!command) {
			return;
		}

		await writeClipboardText(command);
		copiedCommand = kind;
		clearCopyTimer();
		copyTimeout = setTimeout(() => {
			copiedCommand = null;
		}, COPY_RESET_MS);
	}

	function copySshCommand() {
		return copyCommand(sshCommand, "ssh");
	}

	function copyPullCommand() {
		return copyCommand(pullCommand, "pull");
	}

	function reconnectTerminal() {
		if (!terminalReady || !terminal) {
			return;
		}

		terminal.clear();
		lastSize = null;
		connect(sessionId, rootEnabled);
	}

	function getHostSelection(host: HTMLElement): string {
		const selection = document.getSelection();
		if (
			!selection ||
			selection.rangeCount === 0 ||
			selection.toString().length === 0
		) {
			return "";
		}

		const anchorNode = selection.anchorNode;
		const focusNode = selection.focusNode;
		if (
			(anchorNode && host.contains(anchorNode)) ||
			(focusNode && host.contains(focusNode))
		) {
			return selection.toString();
		}

		return "";
	}

	function createXtermLikeAdapter(term: XtermLikeTerminal): TerminalAdapter {
		return {
			get rows() {
				return term.rows;
			},
			get cols() {
				return term.cols;
			},
			write: (data) => {
				term.write(data);
			},
			writeln: (data) => {
				term.writeln(data);
			},
			clear: () => {
				term.clear();
			},
			getSelection: () => term.getSelection(),
			hasSelection: () => term.hasSelection(),
			focus: () => {
				term.focus();
			},
			dispose: () => {
				term.dispose();
			},
		};
	}

	function createWtermAdapter(
		term: WTerm,
		host: HTMLDivElement,
	): TerminalAdapter {
		return {
			get rows() {
				return term.rows;
			},
			get cols() {
				return term.cols;
			},
			write: (data) => {
				term.write(data);
			},
			writeln: (data) => {
				term.write(`${data}\r\n`);
			},
			clear: () => {
				term.write("\x1b[2J\x1b[H");
			},
			getSelection: () => getHostSelection(host),
			hasSelection: () => getHostSelection(host).length > 0,
			focus: () => {
				term.focus();
			},
			dispose: () => {
				term.destroy();
			},
		};
	}

	function createOpenUrlLinkProvider(provider: ILinkProvider): ILinkProvider {
		return {
			provideLinks: (y, callback) => {
				provider.provideLinks(y, (links) => {
					callback(
						links?.map(
							(link): ILink => ({
								...link,
								activate: (event: MouseEvent) => {
									event.preventDefault();
									void openUrl(link.text);
								},
							}),
						),
					);
				});
			},
			dispose: () => {
				provider.dispose?.();
			},
		};
	}

	async function setupGhosttyTerminal(
		host: HTMLDivElement,
		cancelled: () => boolean,
	) {
		const { FitAddon, OSC8LinkProvider, Terminal, UrlRegexProvider, init } =
			await import("ghostty-web");
		await init();

		if (cancelled() || !terminalHost || terminal) {
			return;
		}

		const nextTerminal = new Terminal({
			cursorBlink: true,
			cursorStyle: "block",
			fontFamily:
				'"JetBrains Mono", "Fira Code", "SF Mono", Monaco, "Cascadia Code", "Roboto Mono", Consolas, "Liberation Mono", Menlo, Courier, monospace',
			fontSize: 13,
			scrollback: 5000,
			theme: getTerminalTheme(),
		});
		const nextFitAddon = new FitAddon();

		nextTerminal.loadAddon(nextFitAddon);
		nextTerminal.open(host);
		nextTerminal.registerLinkProvider(
			createOpenUrlLinkProvider(new OSC8LinkProvider(nextTerminal)),
		);
		nextTerminal.registerLinkProvider(
			createOpenUrlLinkProvider(new UrlRegexProvider(nextTerminal)),
		);
		nextFitAddon.fit();
		nextFitAddon.observeResize();

		installGhosttyCopyKeyHandler(nextTerminal);
		activateTerminalTransport(nextTerminal, () => {
			try {
				nextFitAddon.fit();
			} catch {
				// Ignore fit errors during rapid resizing.
			}
		});
	}

	async function setupXtermTerminal(
		host: HTMLDivElement,
		cancelled: () => boolean,
	) {
		const [{ FitAddon }, { WebLinksAddon }, { Terminal }] = await Promise.all([
			import("@xterm/addon-fit"),
			import("@xterm/addon-web-links"),
			import("@xterm/xterm"),
		]);

		if (cancelled() || !terminalHost || terminal) {
			return;
		}

		const nextTerminal = new Terminal({
			cursorBlink: true,
			cursorStyle: "block",
			fontFamily:
				'"JetBrains Mono", "Fira Code", "SF Mono", Monaco, "Cascadia Code", "Roboto Mono", Consolas, "Liberation Mono", Menlo, Courier, monospace',
			fontSize: 13,
			scrollback: 5000,
			theme: getTerminalTheme(),
		});
		const nextFitAddon = new FitAddon();

		nextTerminal.loadAddon(nextFitAddon);
		nextTerminal.loadAddon(
			new WebLinksAddon((event, uri) => {
				event.preventDefault();
				void openUrl(uri);
			}),
		);
		nextTerminal.open(host);
		nextFitAddon.fit();

		installXtermCopyKeyHandler(nextTerminal);
		activateTerminalTransport(nextTerminal, () => {
			try {
				nextFitAddon.fit();
			} catch {
				// Ignore fit errors during rapid resizing.
			}
		});
		terminalResizeObserver = new ResizeObserver(() => {
			try {
				nextFitAddon.fit();
			} catch {
				// Ignore fit errors while the container is being remeasured.
			}
		});
		terminalResizeObserver.observe(host);
	}

	async function setupWtermTerminal(
		host: HTMLDivElement,
		cancelled: () => boolean,
	) {
		const [{ WTerm }, { GhosttyCore }] = await Promise.all([
			import("@wterm/dom"),
			import("@wterm/ghostty"),
		]);
		const core = await GhosttyCore.load();

		if (cancelled() || !terminalHost || terminal) {
			return;
		}

		const nextTerminal = new WTerm(host, {
			autoResize: true,
			cols: MIN_TERMINAL_COLS,
			core,
			cursorBlink: true,
			onData: (data) => {
				sendInput(data);
			},
			onResize: (cols, rows) => {
				sendResize(rows, cols);
			},
			rows: MIN_TERMINAL_ROWS,
		});
		await nextTerminal.init();

		if (cancelled() || !terminalHost || terminal) {
			nextTerminal.destroy();
			return;
		}

		terminal = createWtermAdapter(nextTerminal, host);
		nextTerminal.focus();
		terminalReady = true;
	}

	function installGhosttyCopyKeyHandler(nextTerminal: GhosttyTerminal) {
		nextTerminal.attachCustomKeyEventHandler((event) => {
			const shortcut = getTerminalCopyShortcut(event);
			if (!shortcut) {
				return false;
			}

			if (shortcut === "ctrl-shift") {
				if (terminal?.hasSelection()) {
					event.preventDefault();
					event.stopPropagation();
					copyTerminalSelection();
					return true;
				}
				return false;
			}

			if (!terminal?.hasSelection()) {
				return false;
			}

			event.preventDefault();
			event.stopPropagation();
			copyTerminalSelection();
			return true;
		});
	}

	function installXtermCopyKeyHandler(nextTerminal: XtermTerminal) {
		nextTerminal.attachCustomKeyEventHandler((event) => {
			const shortcut = getTerminalCopyShortcut(event);
			if (!shortcut) {
				return true;
			}

			if (shortcut === "ctrl-shift") {
				if (terminal?.hasSelection()) {
					event.preventDefault();
					event.stopPropagation();
					copyTerminalSelection();
					return false;
				}
				return true;
			}

			if (!terminal?.hasSelection()) {
				return true;
			}

			event.preventDefault();
			event.stopPropagation();
			copyTerminalSelection();
			return false;
		});
	}

	function activateTerminalTransport(
		nextTerminal: XtermLikeTerminal,
		fitTerminal: () => void,
	) {
		terminal = createXtermLikeAdapter(nextTerminal);
		dataSubscription = nextTerminal.onData((data) => {
			sendInput(data);
		});
		resizeSubscription = nextTerminal.onResize(({ cols, rows }) => {
			sendResize(rows, cols);
		});
		windowResizeHandler = () => {
			fitTerminal();
		};
		window.addEventListener("resize", windowResizeHandler);
		nextTerminal.focus();
		terminalReady = true;
	}

	$effect(() => {
		if (!terminalHost) {
			return;
		}

		const host = terminalHost;
		const renderer = terminalRenderer;
		let cancelled = false;
		void (async () => {
			if (renderer === "ghostty") {
				await setupGhosttyTerminal(host, () => cancelled);
				return;
			}

			if (renderer === "xterm") {
				await setupXtermTerminal(host, () => cancelled);
				return;
			}

			await setupWtermTerminal(host, () => cancelled);
		})();

		return () => {
			cancelled = true;
			terminalReady = false;
			copiedCommand = null;
			lastSize = null;
			updateConnectionStatus("disconnected");
			clearResizeTimer();
			clearCopyTimer();
			closeSocket();
			dataSubscription?.dispose();
			dataSubscription = null;
			resizeSubscription?.dispose();
			resizeSubscription = null;
			if (windowResizeHandler) {
				window.removeEventListener("resize", windowResizeHandler);
				windowResizeHandler = null;
			}
			terminalResizeObserver?.disconnect();
			terminalResizeObserver = null;
			terminal?.dispose();
			terminal = null;
		};
	});

	$effect(() => {
		const nextSessionId = sessionId;
		const nextRootEnabled = rootEnabled;
		if (!terminalReady || !terminal) {
			return;
		}

		terminal.clear();
		lastSize = null;
		connect(nextSessionId, nextRootEnabled);

		return () => {
			clearResizeTimer();
			closeSocket();
		};
	});
</script>

<DockWindowChrome
	{dockMaximized}
	{onClose}
	{onToggleDockMaximized}
	{shiftWindowControlsForSidebar}
	closeLabel="Close terminal panel"
	minimizeLabel="Minimize terminal panel"
	{maximizeTitle}
	shellClass="min-h-[28rem]"
	contentClass="min-h-0 min-w-0 flex-1 overflow-hidden"
>
	{#snippet title()}
		<span class="truncate text-xs text-sidebar-foreground/70"
			>{sessionLabel}</span
		>
		<div
			aria-label={`Terminal connection status: ${connectionStatus}`}
			class={`size-2 shrink-0 rounded-full ${statusClass}`}
			role="status"
			title={connectionStatus}
		></div>
	{/snippet}

	{#snippet actions()}
		<fieldset
			class="flex items-center rounded-md border border-sidebar-border p-0.5 text-xs text-sidebar-foreground/70"
			title="Switch terminal renderer"
		>
			<legend class="sr-only">Terminal renderer</legend>
			{#each TERMINAL_RENDERERS as renderer (renderer)}
				<label class="relative">
					<input
						class="peer sr-only"
						type="radio"
						name={`terminal-renderer-${sessionId ?? "default"}`}
						value={renderer}
						bind:group={terminalRenderer}
					/>
					<span
						class="flex h-6 cursor-pointer items-center rounded-sm px-2 text-[0.6875rem] capitalize transition-colors peer-checked:bg-sidebar-accent peer-checked:text-sidebar-accent-foreground peer-focus-visible:ring-2 peer-focus-visible:ring-ring peer-focus-visible:ring-offset-1 peer-focus-visible:ring-offset-background"
					>
						{renderer === "xterm" ? "Xterm.js" : renderer}
					</span>
				</label>
			{/each}
		</fieldset>
		<label class="flex items-center gap-2 text-xs text-sidebar-foreground/70">
			<span>root</span>
			<Switch
				checked={rootEnabled}
				disabled={!sessionId}
				onCheckedChange={(checked) => onRootEnabledChange(checked === true)}
			/>
		</label>
		{#if sshCommand}
			<Button
				variant="ghost"
				size="xs"
				onclick={copySshCommand}
				aria-label="Copy SSH command"
				class="gap-2 text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
				title={`Copy SSH command: ${sshCommand}`}
			>
				{#if copiedCommand === "ssh"}
					<CheckIcon class="size-3.5" />
				{:else}
					<CopyIcon class="size-3.5" />
				{/if}
				<span class="hidden sm:inline"
					>{copiedCommand === "ssh" ? "Copied!" : "Copy SSH"}</span
				>
			</Button>
		{/if}
		{#if pullCommand}
			<Button
				variant="ghost"
				size="xs"
				onclick={copyPullCommand}
				aria-label="Copy pull command"
				class="gap-2 text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
				title={`Copy pull command: ${pullCommand}`}
			>
				{#if copiedCommand === "pull"}
					<CheckIcon class="size-3.5" />
				{:else}
					<CopyIcon class="size-3.5" />
				{/if}
				<span class="hidden sm:inline"
					>{copiedCommand === "pull" ? "Copied!" : "Copy pull cmd"}</span
				>
			</Button>
		{/if}
	{/snippet}

	<div class="relative h-full min-h-0 min-w-0 overflow-hidden p-3">
		{#if connectionStatus !== "connected"}
			<div
				class="absolute inset-3 z-10 flex items-center justify-center rounded-md bg-background/80"
			>
				<div class="flex flex-col items-center gap-3 text-center">
					<span class="text-foreground/70 text-xs">{overlayMessage}</span>
					{#if connectionStatus === "disconnected" && sessionId}
						<Button
							variant="outline"
							size="xs"
							onclick={reconnectTerminal}
							class="gap-2"
						>
							<RotateCcwIcon class="size-3.5" />
							Reconnect
						</Button>
					{/if}
				</div>
			</div>
		{/if}

		<ContextMenu bind:open={terminalContextMenuOpen}>
			<ContextMenuTrigger class="block h-full w-full">
				<div
					bind:this={terminalHost}
					class="h-full w-full cursor-text overflow-hidden rounded-md border border-border bg-terminal-bg p-3 outline-none [caret-color:transparent]"
				></div>
			</ContextMenuTrigger>
			<ContextMenuContent class="w-32">
				<ContextMenuItem
					disabled={!terminalCanCopy}
					onclick={copyTerminalSelection}
				>
					Copy
				</ContextMenuItem>
				<ContextMenuItem
					disabled={!terminalReady}
					onclick={() => {
						void pasteClipboardIntoTerminal();
					}}
				>
					Paste
				</ContextMenuItem>
			</ContextMenuContent>
		</ContextMenu>
	</div>
</DockWindowChrome>
