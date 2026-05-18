import {
	FileDiff,
	VirtualizedFileDiff,
	Virtualizer,
	parseDiffFromFile,
	type FileContents,
	type FileDiff as PierreFileDiffInstance,
	type FileDiffMetadata,
	type FileDiffOptions,
	type SupportedLanguages,
} from "@pierre/diffs";
import { getOrCreateWorkerPoolSingleton } from "@pierre/diffs/worker";

const WORKER_URL = "/assets/pierre-diff-worker.js";
const UI_GO_SESSION_PARAM = "ui_go_session_id";
const UI_GO_SESSION_HEADER = "X-UI-Go-Session";

const DIFF_THEME = {
	light: "github-light",
	dark: "github-dark",
} as const;
const THEME_KEY = "theme";
const COLOR_SCHEME_KEY = "theme.colorScheme";
const colorSchemeKey = (mode: ResolvedTheme) => `${COLOR_SCHEME_KEY}.${mode}`;
const THEME_MODES = ["light", "dark", "system"] as const;
const COLOR_SCHEMES = [
	"default",
	"flexoki",
	"nord",
	"tokyo-night",
	"solarized",
	"dracula",
	"catppuccin-mocha",
	"catppuccin-macchiato",
	"catppuccin-frappe",
	"alucard",
	"catppuccin-latte",
] as const;
const COLOR_SCHEME_NAMES: Record<ThemeColorScheme, string> = {
	default: "Default",
	flexoki: "Flexoki",
	nord: "Nord",
	"tokyo-night": "Tokyo Night",
	solarized: "Solarized",
	dracula: "Dracula",
	"catppuccin-mocha": "Catppuccin Mocha",
	"catppuccin-macchiato": "Catppuccin Macchiato",
	"catppuccin-frappe": "Catppuccin Frappé",
	alucard: "Alucard",
	"catppuccin-latte": "Catppuccin Latte",
};
const COLOR_SCHEME_MODES: Record<ThemeColorScheme, ResolvedTheme | "system"> = {
	default: "system",
	flexoki: "system",
	nord: "dark",
	"tokyo-night": "dark",
	solarized: "light",
	dracula: "dark",
	"catppuccin-mocha": "dark",
	"catppuccin-macchiato": "dark",
	"catppuccin-frappe": "dark",
	alucard: "light",
	"catppuccin-latte": "light",
};
const DIFF_GUTTER_UTILITY_CSS = `
	[data-gutter-utility-slot] {
		left: 0;
		right: auto;
		justify-content: flex-start;
	}

	[data-utility-button] {
		margin-left: 0.5ch;
		margin-right: 0;
	}
`;
const LANGUAGE_MAP: Record<string, SupportedLanguages> = {
	js: "javascript",
	jsx: "javascript",
	ts: "typescript",
	tsx: "typescript",
	py: "python",
	rb: "ruby",
	go: "go",
	rs: "rust",
	java: "java",
	c: "c",
	cpp: "cpp",
	h: "c",
	hpp: "cpp",
	cs: "csharp",
	php: "php",
	swift: "swift",
	kt: "kotlin",
	html: "html",
	css: "css",
	scss: "scss",
	json: "json",
	xml: "xml",
	yaml: "yaml",
	yml: "yaml",
	md: "markdown",
	sql: "sql",
	sh: "bash",
	bash: "bash",
	zsh: "bash",
	dockerfile: "docker",
	makefile: "make",
	toml: "toml",
	graphql: "graphql",
	gql: "graphql",
	svelte: "svelte",
};
const DIFF_WORKER_LANGUAGES = Array.from(
	new Set(Object.values(LANGUAGE_MAP)),
) satisfies SupportedLanguages[];
const VIRTUALIZER_CONFIG = {
	overscrollSize: 1000,
	intersectionObserverMargin: 4000,
	resizeDebugging: false,
};

type DiffStyle = "split" | "unified";
type ResolvedTheme = "light" | "dark";
type ThemeMode = ResolvedTheme | "system";
type ThemeColorScheme = (typeof COLOR_SCHEMES)[number];
type PatchLine = {
	marker: string;
	content: string;
};
type PatchHunk = {
	lines: PatchLine[];
};
type DiffMountPayload = {
	path: string;
	oldPath?: string;
	patch: string;
	commitHash?: string;
	diffStyle?: DiffStyle;
	virtualized?: boolean;
};
type DiffRendererParams = {
	diffStyle: DiffStyle;
	resolvedTheme: ResolvedTheme;
	oldFile: FileContents;
	newFile: FileContents;
	fileDiff?: FileDiffMetadata;
	virtualized: boolean;
};
type DiffMountState = {
	instance: PierreFileDiffInstance | null;
	virtualizer: Virtualizer | null;
	identity: string | null;
};

function uiGoSessionID(): string {
	return (
		document
			.querySelector<HTMLMetaElement>("meta[name='ui-go-session-id']")
			?.content.trim() ?? ""
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

function installSessionTransportFallback() {
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

installSessionTransportFallback();

function getLanguageFromPath(path: string): SupportedLanguages | undefined {
	const filename = path.split("/").at(-1)?.toLowerCase() ?? "";
	if (filename === "dockerfile") return "docker";
	if (filename === "makefile") return "make";
	const extension = path.split(".").at(-1)?.toLowerCase() ?? "";
	return LANGUAGE_MAP[extension];
}

function buildDiffCacheKey(
	path: string,
	content: string,
	cacheKey: string | null,
	language = getLanguageFromPath(path),
): string {
	const languageKey = language ?? "text";
	const contentKey = cacheKey ?? `${content.length}`;
	return `${path}:${languageKey}:${contentKey}`;
}

function buildDiffFileContents(
	path: string,
	content: string,
	cacheKey: string | null,
): FileContents {
	const language = getLanguageFromPath(path);
	return {
		name: path,
		contents: content,
		lang: language,
		cacheKey: buildDiffCacheKey(path, content, cacheKey, language),
	};
}

function workerFactory(): Worker {
	return new Worker(WORKER_URL, { type: "module" });
}

function getDiffWorkerPool() {
	return getOrCreateWorkerPoolSingleton({
		poolOptions: { workerFactory },
		highlighterOptions: {
			theme: DIFF_THEME,
			langs: DIFF_WORKER_LANGUAGES,
			lineDiffType: "word",
		},
	});
}

function getDiffRendererOptions(
	style: DiffStyle,
	theme: ResolvedTheme,
): FileDiffOptions<undefined> {
	return {
		diffStyle: style,
		theme: DIFF_THEME,
		themeType: theme === "dark" ? "dark" : "light",
		disableFileHeader: true,
		hunkSeparators: "line-info",
		expandUnchanged: false,
		collapsedContextThreshold: 3,
		expansionLineCount: 20,
		lineDiffType: "word",
		overflow: "scroll",
		unsafeCSS: DIFF_GUTTER_UTILITY_CSS,
		enableGutterUtility: true,
		lineHoverHighlight: "number",
	};
}

function isThemeMode(value: string | null): value is ThemeMode {
	return THEME_MODES.includes(value as ThemeMode);
}

function isColorScheme(value: string | null): value is ThemeColorScheme {
	return COLOR_SCHEMES.includes(value as ThemeColorScheme);
}

function preferredTheme(): ResolvedTheme {
	return window.matchMedia("(prefers-color-scheme: dark)").matches
		? "dark"
		: "light";
}

function storedThemeMode(): ThemeMode {
	const stored = window.localStorage.getItem(THEME_KEY);
	return isThemeMode(stored) ? stored : "system";
}

function storedColorScheme(mode = resolvedTheme()): ThemeColorScheme {
	const storedForMode = window.localStorage.getItem(colorSchemeKey(mode));
	if (isColorScheme(storedForMode) && colorSchemeAvailableForMode(storedForMode, mode)) {
		return storedForMode;
	}
	return "default";
}

function resolveThemeMode(mode: ThemeMode): ResolvedTheme {
	return mode === "system" ? preferredTheme() : mode;
}

function colorSchemeAvailableForMode(
	scheme: ThemeColorScheme,
	mode: ResolvedTheme,
): boolean {
	const schemeMode = COLOR_SCHEME_MODES[scheme];
	return schemeMode === "system" || schemeMode === mode;
}

function colorSchemeForResolvedMode(
	scheme: ThemeColorScheme,
	mode: ResolvedTheme,
): ThemeColorScheme {
	return colorSchemeAvailableForMode(scheme, mode)
		? scheme
		: storedColorScheme(mode);
}

function resolvedTheme(): ResolvedTheme {
	const documentTheme = document.documentElement.classList.contains("dark")
		? "dark"
		: "light";
	return documentTheme;
}

function applyTheme(mode: ThemeMode) {
	window.localStorage.setItem(THEME_KEY, mode);
	document.documentElement.classList.toggle(
		"dark",
		resolveThemeMode(mode) === "dark",
	);
}

function applyColorScheme(scheme: ThemeColorScheme, mode = resolvedTheme()) {
	window.localStorage.setItem(colorSchemeKey(mode), scheme);
	document.documentElement.setAttribute("data-theme", scheme);
}

function applyStoredTheme() {
	const mode = storedThemeMode();
	const resolved = resolveThemeMode(mode);
	applyTheme(mode);
	applyColorScheme(storedColorScheme(resolved), resolved);
}

function colorSchemeName(scheme: ThemeColorScheme): string {
	return COLOR_SCHEME_NAMES[scheme] ?? COLOR_SCHEME_NAMES.default;
}

function themeOptionAvailable(
	mode: ResolvedTheme | "system" | undefined,
	resolved: ResolvedTheme,
): boolean {
	return mode === "system" || mode === resolved;
}

const themeWindow = window as typeof window & {
	uiGoThemeApply: typeof applyColorSchemeFromMode;
	uiGoThemeColorScheme: typeof storedColorScheme;
	uiGoThemeColorSchemeForMode: typeof colorSchemeForResolvedMode;
	uiGoThemeColorSchemeName: typeof colorSchemeName;
	uiGoThemeMode: typeof storedThemeMode;
	uiGoThemeOptionAvailable: typeof themeOptionAvailable;
	uiGoThemeResolve: typeof resolveThemeMode;
};

function applyColorSchemeFromMode(mode: ThemeMode, scheme: ThemeColorScheme) {
	const resolved = resolveThemeMode(mode);
	applyTheme(mode);
	applyColorScheme(colorSchemeForResolvedMode(scheme, resolved), resolved);
}

themeWindow.uiGoThemeApply = applyColorSchemeFromMode;
themeWindow.uiGoThemeColorScheme = storedColorScheme;
themeWindow.uiGoThemeColorSchemeForMode = colorSchemeForResolvedMode;
themeWindow.uiGoThemeColorSchemeName = colorSchemeName;
themeWindow.uiGoThemeMode = storedThemeMode;
themeWindow.uiGoThemeOptionAvailable = themeOptionAvailable;
themeWindow.uiGoThemeResolve = resolveThemeMode;

function composerFormData(form: HTMLFormElement | null): FormData {
	const data = new FormData();
	const textarea = form?.querySelector<HTMLTextAreaElement>(
		"textarea[name='prompt']",
	);
	data.set("prompt", textarea?.value ?? "");
	return data;
}

async function postComposerAttachments(input: HTMLInputElement) {
	const files = Array.from(input.files ?? []);
	if (files.length === 0) {
		return;
	}
	const form = input.closest<HTMLFormElement>("form");
	const data = composerFormData(form);
	for (const file of files) {
		data.append("attachments", file, file.name);
	}
	input.value = "";
	await fetch("/ui/commands/composer-attachments", {
		method: "POST",
		body: data,
	});
}

async function removeComposerAttachmentByID(
	form: HTMLFormElement | null,
	id: string,
) {
	const formData = composerFormData(form);
	const data = new URLSearchParams();
	data.set("prompt", String(formData.get("prompt") ?? ""));
	data.set("id", id);
	await fetch("/ui/commands/composer-attachment-remove", {
		method: "POST",
		body: data,
	});
}

async function removeComposerAttachment(button: HTMLButtonElement) {
	const id = button.dataset.composerAttachmentRemove;
	if (!id) {
		return;
	}
	await removeComposerAttachmentByID(button.closest<HTMLFormElement>("form"), id);
}

function composerAttachmentInput(root: ParentNode = document) {
	return root.querySelector<HTMLInputElement>(
		"input[data-composer-attachment-input]",
	);
}

function closeComposerAttachmentMenus(except?: HTMLElement) {
	for (const menu of document.querySelectorAll<HTMLElement>(
		"[data-composer-attachment-menu]",
	)) {
		if (except && menu === except) {
			continue;
		}
		menu.classList.add("hidden");
		const trigger = menu
			.closest<HTMLElement>("[data-composer-attachment-button]")
			?.querySelector<HTMLButtonElement>("[data-composer-attachment-trigger]");
		trigger?.setAttribute("aria-expanded", "false");
	}
}

function toggleComposerAttachmentMenu(trigger: HTMLButtonElement) {
	const wrapper = trigger.closest<HTMLElement>("[data-composer-attachment-button]");
	if (!wrapper || wrapper.dataset.composerAttachmentDisabled === "true") {
		return;
	}
	const menu = wrapper.querySelector<HTMLElement>(
		"[data-composer-attachment-menu]",
	);
	if (!menu) {
		return;
	}
	const open = menu.classList.contains("hidden");
	closeComposerAttachmentMenus(menu);
	menu.classList.toggle("hidden", !open);
	trigger.setAttribute("aria-expanded", String(open));
}

function parseUnifiedDiff(patch: string): PatchHunk[] {
	const hunks: PatchHunk[] = [];
	let current: PatchHunk | null = null;
	for (const line of patch.replaceAll("\r\n", "\n").split("\n")) {
		if (line.startsWith("@@")) {
			current = { lines: [] };
			hunks.push(current);
			continue;
		}
		if (!current || line.startsWith("diff ") || line.startsWith("--- ") || line.startsWith("+++ ")) {
			continue;
		}
		if (line.startsWith("+")) {
			current.lines.push({ marker: "+", content: line.slice(1) });
		} else if (line.startsWith("-")) {
			current.lines.push({ marker: "-", content: line.slice(1) });
		} else if (line.startsWith(" ")) {
			current.lines.push({ marker: " ", content: line.slice(1) });
		} else if (line === "") {
			current.lines.push({ marker: " ", content: "" });
		}
	}
	return hunks;
}

function buildPreviewDiffParams(
	payload: DiffMountPayload,
	style: DiffStyle,
): DiffRendererParams | null {
	if (!payload.patch) {
		return null;
	}

	const hunks = parseUnifiedDiff(payload.patch);
	if (hunks.length === 0) {
		return null;
	}

	const oldLines: string[] = [];
	const newLines: string[] = [];
	for (const [index, hunk] of hunks.entries()) {
		if (index > 0) {
			oldLines.push("", "⋯");
			newLines.push("", "⋯");
		}
		for (const line of hunk.lines) {
			if (line.marker !== "+") {
				oldLines.push(line.content);
			}
			if (line.marker !== "-") {
				newLines.push(line.content);
			}
		}
	}

	const oldPath = payload.oldPath || payload.path;
	const oldFile = buildDiffFileContents(
		oldPath,
		oldLines.join("\n"),
		`${payload.commitHash ?? "diff"}:${oldPath}:old`,
	);
	const newFile = buildDiffFileContents(
		payload.path,
		newLines.join("\n"),
		`${payload.commitHash ?? "diff"}:${payload.path}:new`,
	);
	return {
		diffStyle: style,
		resolvedTheme: resolvedTheme(),
		oldFile,
		newFile,
		fileDiff: parseDiffFromFile(oldFile, newFile),
		virtualized: payload.virtualized ?? false,
	};
}

function rendererIdentity(params: DiffRendererParams): string {
	return [
		params.virtualized ? "virtualized" : "standard",
		params.diffStyle,
		params.resolvedTheme,
		params.oldFile.name,
		params.oldFile.lang ?? "text",
		params.oldFile.cacheKey,
		params.newFile.name,
		params.newFile.lang ?? "text",
		params.newFile.cacheKey,
	].join("|");
}

function cleanup(state: DiffMountState) {
	state.instance?.cleanUp();
	state.instance = null;
	state.virtualizer?.cleanUp();
	state.virtualizer = null;
	state.identity = null;
}

function createRenderer(
	mount: HTMLElement,
	state: DiffMountState,
	params: DiffRendererParams,
) {
	const workerPool = getDiffWorkerPool();
	if (params.virtualized) {
		const scrollRoot = mount.querySelector<HTMLElement>("[data-pierre-scroll-root]");
		const scrollContent = mount.querySelector<HTMLElement>(
			"[data-pierre-scroll-content]",
		);
		if (!scrollRoot || !scrollContent) {
			return null;
		}
		const virtualizer = new Virtualizer(VIRTUALIZER_CONFIG);
		virtualizer.setup(scrollRoot, scrollContent);
		state.virtualizer = virtualizer;
		return new VirtualizedFileDiff(
			getDiffRendererOptions(params.diffStyle, params.resolvedTheme),
			virtualizer,
			undefined,
			workerPool,
		);
	}
	return new FileDiff(
		getDiffRendererOptions(params.diffStyle, params.resolvedTheme),
		workerPool,
	);
}

async function renderMount(mount: HTMLElement, style?: DiffStyle) {
	const data = mount.querySelector<HTMLScriptElement>(
		'script[type="application/json"][data-pierre-diff-data]',
	);
	const host = mount.querySelector<HTMLElement>("[data-pierre-diff-host]");
	const fallback = mount.querySelector<HTMLElement>("[data-pierre-diff-fallback]");
	const loading = mount.querySelector<HTMLElement>("[data-pierre-diff-loading]");
	if (!data || !host) {
		return;
	}

	let payload: DiffMountPayload;
	try {
		payload = JSON.parse(data.textContent ?? "{}");
	} catch (error) {
		console.error("[ui-go] Invalid Pierre diff payload", error);
		return;
	}

	const diffStyle = style ?? payload.diffStyle ?? "unified";
	const params = buildPreviewDiffParams(payload, diffStyle);
	if (!params) {
		return;
	}

	const state = ((mount as HTMLElement & { __pierreDiffState?: DiffMountState })
		.__pierreDiffState ??= {
		instance: null,
		virtualizer: null,
		identity: null,
	});
	const identity = rendererIdentity(params);
	if (!state.instance || state.identity !== identity) {
		cleanup(state);
		host.replaceChildren();
		state.instance = createRenderer(mount, state, params);
		state.identity = identity;
	}
	if (!state.instance) {
		return;
	}

	loading?.classList.remove("hidden");
	try {
		state.instance.setOptions(
			getDiffRendererOptions(params.diffStyle, params.resolvedTheme),
		);
		await Promise.resolve(
			state.instance.render({
				oldFile: params.oldFile,
				newFile: params.newFile,
				fileDiff: params.fileDiff,
				containerWrapper: params.virtualized
					? (mount.querySelector<HTMLElement>("[data-pierre-scroll-content]") ??
						undefined)
					: host,
			}),
		);
		fallback?.classList.add("hidden");
		host.classList.remove("hidden");
	} catch (error) {
		console.error("[ui-go] Failed to render Pierre diff", error);
		fallback?.classList.remove("hidden");
	} finally {
		loading?.classList.add("hidden");
	}
}

function enhancePierreDiffs(root: ParentNode = document) {
	for (const mount of root.querySelectorAll<HTMLElement>("[data-pierre-diff]")) {
		void renderMount(mount);
	}
}

function setDiffStyle(button: HTMLButtonElement) {
	const style = button.dataset.diffStyle as DiffStyle | undefined;
	if (style !== "split" && style !== "unified") {
		return;
	}
	const container = button.closest<HTMLElement>("[data-pierre-diff-viewer]");
	if (!container) {
		return;
	}
	for (const item of container.querySelectorAll<HTMLElement>("[data-pierre-diff]")) {
		void renderMount(item, style);
	}
	for (const nextButton of container.querySelectorAll<HTMLButtonElement>(
		"button[data-diff-style]",
	)) {
		nextButton.dataset.active = String(nextButton.dataset.diffStyle === style);
	}
}

document.addEventListener("click", (event) => {
	const attachmentTrigger = (event.target as Element | null)?.closest<HTMLButtonElement>(
		"button[data-composer-attachment-trigger]",
	);
	if (attachmentTrigger) {
		event.preventDefault();
		toggleComposerAttachmentMenu(attachmentTrigger);
		return;
	}

	const addFilesButton = (event.target as Element | null)?.closest<HTMLButtonElement>(
		"button[data-composer-attachment-add-files]",
	);
	if (addFilesButton) {
		event.preventDefault();
		closeComposerAttachmentMenus();
		composerAttachmentInput(addFilesButton.closest("form") ?? document)?.click();
		return;
	}

	const removeAttachmentButton = (
		event.target as Element | null
	)?.closest<HTMLButtonElement>("button[data-composer-attachment-remove]");
	if (removeAttachmentButton) {
		event.preventDefault();
		void removeComposerAttachment(removeAttachmentButton);
		return;
	}

	if (!(event.target as Element | null)?.closest("[data-composer-attachment-button]")) {
		closeComposerAttachmentMenus();
	}

	const button = (event.target as Element | null)?.closest<HTMLButtonElement>(
		"button[data-diff-style]",
	);
	if (!button) {
		return;
	}
	event.preventDefault();
	setDiffStyle(button);
});

document.addEventListener("change", (event) => {
	const input = (event.target as Element | null)?.closest<HTMLInputElement>(
		"input[data-composer-attachment-input]",
	);
	if (!input) {
		return;
	}
	void postComposerAttachments(input);
});

document.addEventListener("keydown", (event) => {
	const textarea = (event.target as Element | null)?.closest<HTMLTextAreaElement>(
		"textarea[data-composer-textarea]",
	);
	if (
		!textarea ||
		event.key !== "Backspace" ||
		textarea.value.length > 0 ||
		textarea.dataset.composerBackspaceRemovesLastAttachment !== "true"
	) {
		return;
	}
	const form = textarea.closest<HTMLFormElement>("form");
	const attachments = form?.querySelectorAll<HTMLElement>(
		"[data-composer-attachment-id]",
	);
	const lastAttachment = attachments?.item(attachments.length - 1);
	const id = lastAttachment?.dataset.composerAttachmentId;
	if (!id) {
		return;
	}
	event.preventDefault();
	void removeComposerAttachmentByID(form, id);
});

document.addEventListener("paste", (event) => {
	const textarea = (event.target as Element | null)?.closest<HTMLTextAreaElement>(
		"textarea[data-composer-textarea]",
	);
	const files = Array.from(event.clipboardData?.files ?? []);
	if (!textarea || files.length === 0) {
		return;
	}
	const input = composerAttachmentInput(textarea.closest("form") ?? document);
	if (!input) {
		return;
	}
	const dataTransfer = new DataTransfer();
	for (const file of files) {
		dataTransfer.items.add(file);
	}
	input.files = dataTransfer.files;
	void postComposerAttachments(input);
});

window
	.matchMedia("(prefers-color-scheme: dark)")
	.addEventListener("change", () => {
		if (storedThemeMode() !== "system") {
			return;
		}
		applyStoredTheme();
		window.dispatchEvent(new CustomEvent("ui-go-theme-system-change"));
		enhancePierreDiffs();
	});

document.addEventListener("DOMContentLoaded", () => {
	applyStoredTheme();
	enhancePierreDiffs();
});
document.addEventListener("datastar-patched", () => {
	enhancePierreDiffs();
});
