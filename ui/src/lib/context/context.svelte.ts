import {
	getContext as getSvelteContext,
	setContext as setSvelteContext,
} from "svelte";

import type { Context, ContextBootstrap } from "$lib/context/context.types";
import { getAvailableThemes } from "$lib/theme";
import { getAppEnvironment } from "$lib/app/app-helpers";

const CONTEXT_KEY = Symbol.for("discobot-ui-context");

let currentContext: Context | null = null;

export function createContext(bootstrap: ContextBootstrap): Context {
	const environment = getAppEnvironment();
	const context = $state<Context>({
		view: {
			app: {
				navigation: {
					desktopSidebarOpen: false,
					mobileSidebarOpen: false,
					mountedSessionIds: [],
				},
				selection: {
					sessionId: bootstrap.selectedSessionId ?? null,
					threadId: bootstrap.selectedThreadId ?? null,
					pendingSessionId: "",
					requestedThreadIdBySessionId: {},
				},
				dialogs: {
					settings: {
						open: false,
						tab: "appearance",
					},
					credentials: {
						open: false,
						targetId: null,
						flowIntent: null,
					},
					supportInfo: {
						open: false,
					},
					keyboardShortcuts: {
						open: false,
					},
					recentThreadSwitcher: {
						open: false,
						selectedKey: null,
						commitModifier: null,
					},
				},
				preferences: {
					theme: "system",
					resolvedTheme: "dark",
					colorScheme: "default",
					availableThemes: getAvailableThemes("dark"),
					promptHistory: [],
					pinnedPrompts: [],
					preferredIde: "cursor",
					ideOptions: bootstrap.ideOptions,
					chatWidthMode: "constrained",
					defaultModel: "",
					recentThreadsVisibleLimit: 5,
					sidebarRecentOpen: true,
					sidebarAllOpen: true,
					sidebarAllGroupedByWorkspace: true,
					showRefreshButton: true,
					topBarIconOnly: false,
					autoScrollOnStream: true,
					ignoredUpdateVersion: null,
					trackPrereleases: false,
				},
				recentThreads: {
					visibleItems: [],
				},
				startupTasks: {
					visibleIds: [],
					hasActiveTasks: false,
				},
				updates: {
					showBadge: false,
				},
				projectEvents: {
					connected: false,
				},
			},
			sessions: {},
		},
		data: {
			environment: {
				apiBase: environment.apiBase,
				runtime: environment.runtime,
				isDesktop: environment.isDesktop,
				supportsNativeWindowControls: environment.supportsNativeWindowControls,
				supportsAppUpdates: environment.supportsAppUpdates,
				windowControlsSide: environment.windowControlsSide,
				windowControls: bootstrap.windowControls,
			},
			sessions: {
				items: [],
				byId: {},
				status: "idle",
				error: null,
				recentThreads: [],
			},
			threads: {
				bySessionId: {},
			},
			conversations: {
				byThreadId: {},
			},
			workspaces: {
				items: [],
				byId: {},
				status: "idle",
				error: null,
			},
			models: {
				items: [],
				byId: {},
				status: "idle",
				error: null,
			},
			credentials: {
				items: [],
				byId: {},
				types: [],
				status: "idle",
				error: null,
			},
			startupTasks: {
				items: [],
				byId: {},
				status: "idle",
				error: null,
			},
			files: {
				bySessionId: {},
			},
			hooks: {
				bySessionId: {},
			},
			services: {
				bySessionId: {},
			},
			commands: {
				bySessionId: {},
			},
			supportInfo: {
				value: null,
				status: "idle",
				error: null,
			},
			updates: {
				status: "idle",
				availableVersion: null,
				error: null,
				downloadedBytes: 0,
				totalBytes: null,
				isIgnored: false,
				canTrackPrereleases: false,
			},
		},
		actions: {},
	});

	return context;
}

export function setDiscobotContext(bootstrap: ContextBootstrap): Context {
	const context = createContext(bootstrap);
	currentContext = context;
	setSvelteContext(CONTEXT_KEY, context);
	return context;
}

export function useContext(): Context {
	const context = getSvelteContext<Context | undefined>(CONTEXT_KEY);
	if (!context) {
		throw new Error("useContext must be used within Context provider");
	}
	return context;
}

export function getContextForCommand(): Context {
	if (!currentContext) {
		throw new Error("Context has not been initialized");
	}
	return currentContext;
}
