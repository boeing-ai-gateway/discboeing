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
const DIFF_THEME = {
	light: "github-light",
	dark: "github-dark",
} as const;
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

function resolvedTheme(): ResolvedTheme {
	return document.documentElement.classList.contains("dark") ? "dark" : "light";
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
					? (mount.querySelector<HTMLElement>("[data-pierre-scroll-content]") ?? undefined)
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

export const pierreDiffViewer = {
	enhance(root: ParentNode = document) {
		for (const mount of root.querySelectorAll<HTMLElement>("[data-pierre-diff]")) {
			void renderMount(mount);
		}
	},

	setStyle(button: HTMLButtonElement) {
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
	},
};
