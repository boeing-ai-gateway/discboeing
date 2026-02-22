/**
 * HTTP Reverse Proxy
 *
 * Provides HTTP reverse proxy to service ports using fetch.
 * Supports HTTP and SSE streaming without buffering.
 * Respects x-forwarded-* headers and rewrites paths based on x-forwarded-path.
 *
 * Note: WebSocket upgrades are handled at the Bun.serve level in index.ts.
 */

import type { Context } from "hono";

const DEBUG = process.env.DEBUG_PROXY === "true";
const MAX_BODY_LOG_SIZE = 4096;

function log(message: string, data?: Record<string, unknown>): void {
	if (!DEBUG) return;
	const timestamp = new Date().toISOString();
	if (data) {
		console.log(
			`[${timestamp}] [proxy] ${message}`,
			JSON.stringify(data, null, 2),
		);
	} else {
		console.log(`[${timestamp}] [proxy] ${message}`);
	}
}

function truncateBody(body: string): string {
	if (body.length <= MAX_BODY_LOG_SIZE) {
		return body;
	}
	return (
		body.slice(0, MAX_BODY_LOG_SIZE) +
		`... [truncated, ${body.length} bytes total]`
	);
}

/**
 * Proxy an HTTP request to a local port.
 * Handles regular HTTP and SSE streaming.
 *
 * @param c - Hono context
 * @param port - Target port to proxy to
 */
export async function proxyHttpRequest(
	c: Context,
	port: number,
): Promise<Response> {
	const req = c.req.raw;

	// Note: WebSocket upgrades are handled at the Bun.serve level in index.ts
	// They should not reach this function, but if they do, return 501
	const upgradeHeader = c.req.header("upgrade")?.toLowerCase();
	if (upgradeHeader === "websocket") {
		log("WebSocket upgrade reached HTTP proxy (should be handled by Bun)", {
			port,
		});
		return new Response(
			JSON.stringify({
				error: "websocket_not_supported",
				message:
					"WebSocket proxying requires Bun runtime. Upgrade requests should be handled at the server level.",
			}),
			{
				status: 501,
				headers: { "content-type": "application/json" },
			},
		);
	}

	// Get the path to use - prefer x-forwarded-path, fall back to original path
	const forwardedPath = c.req.header("x-forwarded-path");
	const originalUrl = new URL(req.url);

	// Build the target URL
	const targetUrl = new URL(`http://localhost:${port}`);
	targetUrl.pathname = forwardedPath || originalUrl.pathname;
	targetUrl.search = originalUrl.search;

	log("Incoming request", {
		method: req.method,
		originalUrl: req.url,
		forwardedPath,
		targetUrl: targetUrl.toString(),
		port,
	});

	// Build headers for the proxied request
	const headers = new Headers();

	// Headers to exclude from request
	const excludeRequestHeaders = new Set([
		"connection",
		"keep-alive",
		"proxy-authenticate",
		"proxy-authorization",
		"te",
		"trailers",
		"transfer-encoding",
		"upgrade",
		"host", // We'll set this to the target
	]);

	// Headers to exclude from response (fetch auto-decompresses)
	const excludeResponseHeaders = new Set([
		"connection",
		"keep-alive",
		"proxy-authenticate",
		"proxy-authorization",
		"te",
		"trailers",
		"transfer-encoding",
		"upgrade",
		"content-encoding",
		"content-length",
	]);

	for (const [key, value] of req.headers.entries()) {
		const lowerKey = key.toLowerCase();
		if (!excludeRequestHeaders.has(lowerKey)) {
			headers.set(key, value);
		}
	}

	// Set/update x-forwarded-* headers
	const clientIp =
		c.req.header("x-forwarded-for") || c.req.header("x-real-ip") || "127.0.0.1";
	headers.set("x-forwarded-for", clientIp);

	const forwardedHost =
		c.req.header("x-forwarded-host") || c.req.header("host") || "localhost";
	headers.set("x-forwarded-host", forwardedHost);

	const forwardedProto = c.req.header("x-forwarded-proto") || "http";
	headers.set("x-forwarded-proto", forwardedProto);

	// Set host header: use X-Service-Host if present (for nested Discobot routing),
	// replacing the port with the target service port.
	const serviceHost = c.req.header("x-service-host");
	if (serviceHost) {
		const hostWithoutPort = serviceHost.replace(/:\d+$/, "");
		headers.set("host", `${hostWithoutPort}:${port}`);
	} else {
		headers.set("host", `localhost:${port}`);
	}

	// Make the proxied request
	const proxyReq: RequestInit = {
		method: req.method,
		headers,
		redirect: "manual",
	};

	// Include body for methods that support it
	if (req.method !== "GET" && req.method !== "HEAD" && req.body) {
		proxyReq.body = req.body;
		(proxyReq as RequestInit & { duplex: string }).duplex = "half";
	}

	// Log request
	const reqHeaders: Record<string, string> = {};
	headers.forEach((value, key) => {
		reqHeaders[key] = value;
	});
	log("Proxy request", { headers: reqHeaders });

	try {
		const proxyRes = await fetch(targetUrl.toString(), proxyReq);

		// Log response headers
		const resHeaders: Record<string, string> = {};
		proxyRes.headers.forEach((value, key) => {
			resHeaders[key] = value;
		});

		log("Proxy response", {
			status: proxyRes.status,
			statusText: proxyRes.statusText,
			headers: resHeaders,
		});

		// Build response headers
		const responseHeaders = new Headers();
		for (const [key, value] of proxyRes.headers.entries()) {
			const lowerKey = key.toLowerCase();
			if (!excludeResponseHeaders.has(lowerKey)) {
				responseHeaders.set(key, value);
			}
		}

		// If no body, return as-is
		if (!proxyRes.body) {
			return new Response(null, {
				status: proxyRes.status,
				statusText: proxyRes.statusText,
				headers: responseHeaders,
			});
		}

		// Stream the response body directly without buffering
		// Only add logging transform when DEBUG is enabled
		let responseBody: ReadableStream<Uint8Array> = proxyRes.body;

		if (DEBUG) {
			// Create a transform stream that logs chunks as they pass through
			// This is only used for debugging and may affect streaming performance
			const loggedBody = new TransformStream<Uint8Array, Uint8Array>({
				transform(chunk, controller) {
					const text = new TextDecoder().decode(chunk);
					log("Response chunk", {
						size: chunk.length,
						content: truncateBody(text),
					});
					controller.enqueue(chunk);
				},
				flush() {
					log("Response stream ended");
				},
			});
			responseBody = proxyRes.body.pipeThrough(loggedBody);
		}

		return new Response(responseBody, {
			status: proxyRes.status,
			statusText: proxyRes.statusText,
			headers: responseHeaders,
		});
	} catch (err) {
		const message = err instanceof Error ? err.message : "Proxy error";
		log("Proxy error", { error: message, targetUrl: targetUrl.toString() });

		// Check if this is a connection refused error
		const isConnectionRefused =
			message.includes("ECONNREFUSED") ||
			message.includes("Connection refused") ||
			message.includes("connect ECONNREFUSED") ||
			message.includes("Unable to connect");

		// Check if the client accepts HTML (browser/iframe request)
		const acceptHeader = c.req.header("accept") || "";
		const wantsHtml =
			acceptHeader.includes("text/html") ||
			acceptHeader.includes("application/xhtml+xml");

		if (isConnectionRefused) {
			if (wantsHtml) {
				// Return an HTML page that auto-refreshes for browser/iframe requests
				return new Response(connectionRefusedHtml(port), {
					status: 503,
					headers: { "content-type": "text/html; charset=utf-8" },
				});
			}
			return new Response(
				JSON.stringify({
					error: "connection_refused",
					message: `Unable to connect to service on port ${port}`,
					port,
				}),
				{
					status: 503,
					headers: { "content-type": "application/json" },
				},
			);
		}

		if (wantsHtml) {
			return new Response(proxyErrorHtml(port, message), {
				status: 502,
				headers: { "content-type": "text/html; charset=utf-8" },
			});
		}

		return new Response(
			JSON.stringify({ error: "proxy_error", message, port }),
			{
				status: 502,
				headers: { "content-type": "application/json" },
			},
		);
	}
}

/**
 * Generate HTML page for connection refused errors.
 * The page auto-refreshes every 5 seconds and shows status updates.
 */
function connectionRefusedHtml(port: number): string {
	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<title>Connecting to service...</title>
	<style>
		body {
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
			display: flex;
			justify-content: center;
			align-items: center;
			min-height: 100vh;
			margin: 0;
			background: #f5f5f5;
			color: #333;
		}
		@media (prefers-color-scheme: dark) {
			body { background: #1a1a1a; color: #e0e0e0; }
		}
		.container {
			text-align: center;
			padding: 2rem;
		}
		.spinner {
			width: 40px;
			height: 40px;
			border: 3px solid #ddd;
			border-top-color: #666;
			border-radius: 50%;
			animation: spin 1s linear infinite;
			margin: 0 auto 1rem;
		}
		@media (prefers-color-scheme: dark) {
			.spinner { border-color: #444; border-top-color: #aaa; }
		}
		@keyframes spin { to { transform: rotate(360deg); } }
		h1 { font-size: 1.25rem; font-weight: 500; margin: 0 0 0.5rem; }
		p { font-size: 0.875rem; color: #666; margin: 0; }
		@media (prefers-color-scheme: dark) {
			p { color: #999; }
		}
		.status {
			font-size: 0.75rem;
			color: #999;
			margin-top: 1rem;
			font-family: monospace;
		}
		.status.error { color: #c00; }
		.status.success { color: #0a0; }
		@media (prefers-color-scheme: dark) {
			.status { color: #666; }
			.status.error { color: #f66; }
			.status.success { color: #6f6; }
		}
	</style>
	<script>
		let retryCount = 0;
		let retryTimeout;

		async function checkConnection() {
			retryCount++;
			const statusEl = document.getElementById('status');

			try {
				const response = await fetch(window.location.href, {
					method: 'HEAD',
					cache: 'no-cache'
				});

				if (response.ok) {
					statusEl.textContent = 'HTTP ' + response.status + ' ' + response.statusText + ' - Connection established!';
					statusEl.className = 'status success';
					// Reload the page to show the actual content
					setTimeout(() => window.location.reload(), 500);
				} else {
					statusEl.textContent = 'HTTP ' + response.status + ' ' + response.statusText + ' - Retrying... (attempt ' + retryCount + ')';
					statusEl.className = 'status error';
					retryTimeout = setTimeout(checkConnection, 2000);
				}
			} catch (err) {
				statusEl.textContent = 'Connection refused - Retrying... (attempt ' + retryCount + ')';
				statusEl.className = 'status error';
				retryTimeout = setTimeout(checkConnection, 2000);
			}
		}

		window.addEventListener('load', () => {
			checkConnection();
		});

		window.addEventListener('beforeunload', () => {
			if (retryTimeout) clearTimeout(retryTimeout);
		});
	</script>
</head>
<body>
	<div class="container">
		<div class="spinner"></div>
		<h1>Waiting for service on port ${port}</h1>
		<p>Checking connection status...</p>
		<div id="status" class="status">Connecting...</div>
	</div>
</body>
</html>`;
}

/**
 * Generate HTML page for generic proxy errors.
 */
function proxyErrorHtml(port: number, message: string): string {
	const escapedMessage = message
		.replace(/&/g, "&amp;")
		.replace(/</g, "&lt;")
		.replace(/>/g, "&gt;");

	return `<!DOCTYPE html>
<html>
<head>
	<meta charset="utf-8">
	<title>Proxy Error</title>
	<style>
		body {
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
			display: flex;
			justify-content: center;
			align-items: center;
			min-height: 100vh;
			margin: 0;
			background: #f5f5f5;
			color: #333;
		}
		@media (prefers-color-scheme: dark) {
			body { background: #1a1a1a; color: #e0e0e0; }
		}
		.container {
			text-align: center;
			padding: 2rem;
			max-width: 400px;
		}
		h1 { font-size: 1.25rem; font-weight: 500; margin: 0 0 0.5rem; color: #c00; }
		@media (prefers-color-scheme: dark) {
			h1 { color: #f66; }
		}
		p { font-size: 0.875rem; color: #666; margin: 0.5rem 0; }
		@media (prefers-color-scheme: dark) {
			p { color: #999; }
		}
		code {
			font-size: 0.75rem;
			background: #eee;
			padding: 0.25rem 0.5rem;
			border-radius: 4px;
			display: block;
			margin-top: 1rem;
			word-break: break-all;
		}
		@media (prefers-color-scheme: dark) {
			code { background: #333; }
		}
	</style>
</head>
<body>
	<div class="container">
		<h1>Unable to connect to service</h1>
		<p>Failed to proxy request to port ${port}</p>
		<code>${escapedMessage}</code>
	</div>
</body>
</html>`;
}
