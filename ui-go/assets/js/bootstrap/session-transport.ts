const UI_GO_SESSION_PARAM = "ui_go_session_id";
const UI_GO_SESSION_HEADER = "X-UI-Go-Session";

function uiGoSessionID(): string {
	return (
		document.querySelector<HTMLMetaElement>("meta[name='ui-go-session-id']")?.content.trim() ?? ""
	);
}

function isUiGoSessionRequest(url: URL): boolean {
	return url.origin === window.location.origin && url.pathname.startsWith("/ui/");
}

function uiGoSessionURL(input: string | URL): string {
	const id = uiGoSessionID();
	if (!id) {
		return String(input);
	}
	const url = new URL(input, window.location.href);
	if (!isUiGoSessionRequest(url)) {
		return String(input);
	}
	if (!url.searchParams.has(UI_GO_SESSION_PARAM)) {
		url.searchParams.set(UI_GO_SESSION_PARAM, id);
	}
	return url.href;
}

export function installSessionTransportFallback() {
	const originalFetch = window.fetch.bind(window);
	window.fetch = (input: RequestInfo | URL, init?: RequestInit) => {
		const id = uiGoSessionID();
		const url = new URL(input instanceof Request ? input.url : input, window.location.href);
		const headers = new Headers(init?.headers);
		if (id && isUiGoSessionRequest(url) && !headers.has(UI_GO_SESSION_HEADER)) {
			headers.set(UI_GO_SESSION_HEADER, id);
		}
		if (input instanceof Request) {
			return originalFetch(new Request(uiGoSessionURL(input.url), input), {
				...init,
				headers,
			});
		}
		return originalFetch(uiGoSessionURL(input), { ...init, headers });
	};

	const OriginalEventSource = window.EventSource;
	window.EventSource = class extends OriginalEventSource {
		constructor(url: string | URL, eventSourceInitDict?: EventSourceInit) {
			super(uiGoSessionURL(url), eventSourceInitDict);
		}
	};
}
