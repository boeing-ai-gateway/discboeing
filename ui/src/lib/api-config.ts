import {
	getDesktopAuthToken,
	getDesktopServerConfig,
	isDesktopShell,
} from "$lib/shell";

// Default project ID for anonymous user mode (matches Go backend)
export const PROJECT_ID = "local";

declare global {
	interface Window {
		__DISCBOEING_CONFIG__?: {
			apiRoot?: string;
		};
	}
}

const DEFAULT_SSH_PORT = 3333;
const DEFAULT_HTTP_PORT = 3001;
const desktopLocalhost = "localhost";
const sameOriginAPIPath = "/api";
const viteApiRoot = import.meta.env.VITE_DISCBOEING_API_ROOT;

// Server config (fetched from backend)
let sshPort = DEFAULT_SSH_PORT;
let serverConfig: {
	httpPort: number;
	httpsPort: number | null;
	httpsTLSMode: "ephemeral" | "static" | "acme" | null;
} | null = null;

/**
 * Append the desktop auth token to a URL when a desktop runtime has provided one.
 * Used for WebSocket and SSE URLs that need authentication.
 */
export function appendAuthToken(url: string): string {
	const token = getDesktopAuthToken();
	if (!token) {
		return url;
	}
	const separator = url.includes("?") ? "&" : "?";
	return `${url}${separator}token=${encodeURIComponent(token)}`;
}

function getInjectedApiRootBase(): string | null {
	if (typeof window === "undefined") {
		return null;
	}
	const apiRoot = window.__DISCBOEING_CONFIG__?.apiRoot;
	if (!apiRoot) {
		return null;
	}
	return new URL(apiRoot, window.location.origin).toString().replace(/\/$/, "");
}

/**
 * Get the backend API root URL (without project path).
 *
 * - In standalone embedded mode: uses the Go server's injected runtime config
 * - In a desktop shell with initialized config: connects directly to the bundled Go server
 * - In browser with *-svc-ui.* or *-svc-ui-svelte.* hostname: routes to corresponding *-svc-api.* host
 * - In Vite dev: calls the Go backend directly on port 3001
 * - Otherwise: uses the current origin's /api endpoint
 */
export function getApiRootBase() {
	if (viteApiRoot) {
		if (typeof window === "undefined") {
			return viteApiRoot.replace(/\/$/, "");
		}
		return new URL(viteApiRoot, window.location.origin)
			.toString()
			.replace(/\/$/, "");
	}

	if (typeof window === "undefined") {
		// Server-side rendering - call backend directly
		return "http://localhost:3001/api";
	}

	const injectedApiRoot = getInjectedApiRootBase();
	if (injectedApiRoot) {
		return injectedApiRoot;
	}

	// Check if hostname matches *-svc-ui.* or *-svc-ui-svelte.* pattern
	const hostname = window.location.hostname;
	const svcUiHostToken = hostname.includes("-svc-ui-svelte.")
		? "-svc-ui-svelte."
		: hostname.includes("-svc-ui.")
			? "-svc-ui."
			: null;
	if (svcUiHostToken) {
		const apiHostname = hostname.replace(svcUiHostToken, "-svc-api.");
		const protocol = window.location.protocol;
		const preferred = getPreferredBrowserAPIOrigin();
		const port =
			protocol === "https:"
				? (preferred?.port ?? window.location.port)
				: window.location.port;
		const apiHost = port ? `${apiHostname}:${port}` : apiHostname;
		return `${protocol}//${apiHost}/api`;
	}

	const desktopServerConfig = getDesktopServerConfig();
	if (desktopServerConfig) {
		return `http://${desktopLocalhost}:${desktopServerConfig.port}/api`;
	}

	if (import.meta.env.DEV) {
		const preferred = getPreferredBrowserAPIOrigin();
		if (preferred) {
			return `${preferred.protocol}//${window.location.hostname}:${preferred.port}/api`;
		}
		return `http://localhost:${DEFAULT_HTTP_PORT}/api`;
	}

	return `${window.location.origin}${sameOriginAPIPath}`;
}

/**
 * Get the backend API base URL (with project path).
 */
export function getApiBase() {
	return `${getApiRootBase()}/projects/${PROJECT_ID}`;
}

/**
 * Get the backend WebSocket base URL.
 * Includes the desktop auth token when available.
 *
 * - In a desktop shell with initialized config: connects directly to the bundled Go server with token
 * - In browser with *-svc-ui.* or *-svc-ui-svelte.* hostname: routes to corresponding *-svc-api.* host
 * - In Vite dev: connects to the Go backend directly on port 3001
 * - Otherwise: connects to the current origin's /api endpoint
 */
export function getWsBase() {
	const url = getApiRootBase();
	return `${url.replace(/^http/, "ws")}/projects/${PROJECT_ID}`;
}

/**
 * Initialize server config by fetching from the backend.
 * Call after the backend is ready.
 */
export async function initServerConfig(): Promise<void> {
	try {
		const resp = await fetch(
			appendAuthToken(`${getApiRootBase()}/server-config`),
			{ credentials: "include" },
		);
		if (resp.ok) {
			const config = await resp.json();
			if (typeof config.ssh_port === "number" && config.ssh_port > 0) {
				sshPort = config.ssh_port;
			}
			serverConfig = {
				httpPort:
					typeof config.http_port === "number" && config.http_port > 0
						? config.http_port
						: DEFAULT_HTTP_PORT,
				httpsPort:
					typeof config.https_port === "number" && config.https_port > 0
						? config.https_port
						: null,
				httpsTLSMode:
					config.https_tls_mode === "ephemeral" ||
					config.https_tls_mode === "static" ||
					config.https_tls_mode === "acme"
						? config.https_tls_mode
						: null,
			};
		}
	} catch {
		// Fall back to default SSH port
	}
}

/**
 * Get the SSH port configured on the server. Defaults to 3333.
 */
export function getSSHPort(): number {
	return sshPort;
}

function getPreferredBrowserAPIOrigin(): {
	protocol: "https:";
	port: string;
} | null {
	if (
		typeof window === "undefined" ||
		isDesktopShell() ||
		!serverConfig?.httpsPort
	) {
		return null;
	}
	if (window.location.protocol === "https:") {
		return { protocol: "https:", port: String(serverConfig.httpsPort) };
	}
	if (
		serverConfig.httpsTLSMode === "static" ||
		serverConfig.httpsTLSMode === "acme"
	) {
		return { protocol: "https:", port: String(serverConfig.httpsPort) };
	}
	return null;
}
