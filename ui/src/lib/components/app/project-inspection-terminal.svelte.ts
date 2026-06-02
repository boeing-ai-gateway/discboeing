import type {
	ILink,
	ILinkProvider,
	ITheme,
	Terminal as GhosttyTerminal,
} from "ghostty-web";
import { appendAuthToken, getWsBase } from "$lib/api-config";
import { openUrl } from "$lib/shell";

export type ConnectionStatus =
	| "disconnected"
	| "connecting"
	| "connected"
	| "error";

type ProjectInspectionTerminalOptions = {
	open: () => boolean;
	projectId: () => string;
	providerId: () => string | undefined;
	terminalHost: () => HTMLDivElement | null;
};

const MIN_TERMINAL_ROWS = 20;
const MIN_TERMINAL_COLS = 80;
const RESIZE_DEBOUNCE_MS = 150;

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
	const background = readThemeValue(style, "--terminal-bg", "oklch(0.08 0 0)");
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

export function createProjectInspectionTerminal({
	open,
	projectId,
	providerId,
	terminalHost,
}: ProjectInspectionTerminalOptions) {
	let connectionStatus = $state<ConnectionStatus>("disconnected");
	let terminalReady = $state(false);

	let terminal: GhosttyTerminal | null = null;
	let socket: WebSocket | null = null;
	let resizeTimeout: ReturnType<typeof setTimeout> | null = null;
	let dataSubscription: { dispose(): void } | null = null;
	let resizeSubscription: { dispose(): void } | null = null;
	let windowResizeHandler: (() => void) | null = null;
	let lastSize: { rows: number; cols: number } | null = null;

	const statusClass = $derived.by(() => {
		if (connectionStatus === "connected") return "bg-green-500";
		if (connectionStatus === "connecting") return "bg-yellow-500";
		if (connectionStatus === "error") return "bg-red-500";
		return "bg-muted-foreground/50";
	});

	const overlayMessage = $derived.by(() => {
		if (connectionStatus === "connecting") {
			return "Connecting…";
		}
		if (connectionStatus === "error") {
			return "Connection error — retry to reconnect to the inspection shell.";
		}
		if (connectionStatus === "disconnected") {
			return "Disconnected";
		}
		return "";
	});

	function updateConnectionStatus(status: ConnectionStatus) {
		connectionStatus = status;
	}

	function clearResizeTimer() {
		if (resizeTimeout) {
			clearTimeout(resizeTimeout);
			resizeTimeout = null;
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

			socket.send(JSON.stringify({ type: "resize", data: { rows, cols } }));
		}, RESIZE_DEBOUNCE_MS);
	}

	function connect(nextProjectId: string, nextProviderId: string | undefined) {
		if (!terminal) {
			return;
		}

		closeSocket();
		clearResizeTimer();
		updateConnectionStatus("connecting");
		const rows = terminal.rows;
		const cols = terminal.cols;
		const projectWsBase = getWsBase().replace(
			/\/projects\/[^/]+$/,
			`/projects/${encodeURIComponent(nextProjectId)}`,
		);
		const terminalPath = nextProviderId
			? `/sandbox-providers/${encodeURIComponent(nextProviderId)}/inspection/terminal/ws`
			: "/inspection/terminal/ws";
		const wsUrl = appendAuthToken(
			`${projectWsBase}${terminalPath}?rows=${rows}&cols=${cols}`,
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

	function reconnectTerminal() {
		if (!terminalReady || !terminal) {
			return;
		}
		terminal.clear();
		lastSize = null;
		connect(projectId(), providerId());
	}

	$effect(() => {
		const nextTerminalHost = terminalHost();
		if (!open() || !nextTerminalHost) {
			return;
		}

		let cancelled = false;
		void (async () => {
			const { FitAddon, OSC8LinkProvider, Terminal, UrlRegexProvider, init } =
				await import("ghostty-web");
			await init();

			const createOpenUrlLinkProvider = (
				provider: ILinkProvider,
			): ILinkProvider => ({
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
			});

			if (cancelled || terminalHost() !== nextTerminalHost || terminal) {
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
			nextTerminal.open(nextTerminalHost);
			nextTerminal.registerLinkProvider(
				createOpenUrlLinkProvider(new OSC8LinkProvider(nextTerminal)),
			);
			nextTerminal.registerLinkProvider(
				createOpenUrlLinkProvider(new UrlRegexProvider(nextTerminal)),
			);
			nextFitAddon.fit();
			nextFitAddon.observeResize();

			terminal = nextTerminal;
			dataSubscription = nextTerminal.onData((data) => {
				sendInput(data);
			});
			resizeSubscription = nextTerminal.onResize(({ cols, rows }) => {
				sendResize(rows, cols);
			});
			windowResizeHandler = () => {
				try {
					nextFitAddon.fit();
				} catch {
					// Ignore fit errors during rapid resizing.
				}
			};
			window.addEventListener("resize", windowResizeHandler);
			nextTerminal.focus();
			terminalReady = true;
		})();

		return () => {
			cancelled = true;
			terminalReady = false;
			lastSize = null;
			updateConnectionStatus("disconnected");
			clearResizeTimer();
			closeSocket();
			dataSubscription?.dispose();
			dataSubscription = null;
			resizeSubscription?.dispose();
			resizeSubscription = null;
			if (windowResizeHandler) {
				window.removeEventListener("resize", windowResizeHandler);
				windowResizeHandler = null;
			}
			terminal?.dispose();
			terminal = null;
		};
	});

	$effect(() => {
		const nextProjectId = projectId();
		const nextProviderId = providerId();
		const nextOpen = open();
		if (!nextOpen || !terminalReady || !terminal) {
			return;
		}

		terminal.clear();
		lastSize = null;
		connect(nextProjectId, nextProviderId);

		return () => {
			clearResizeTimer();
			closeSocket();
		};
	});

	return {
		get connectionStatus() {
			return connectionStatus;
		},
		get overlayMessage() {
			return overlayMessage;
		},
		get statusClass() {
			return statusClass;
		},
		reconnectTerminal,
	};
}
