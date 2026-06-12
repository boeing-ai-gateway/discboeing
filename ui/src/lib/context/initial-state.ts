import { generateId } from "ai";
import type {
	Bootstrap,
	DataState,
	SessionCommandCredentialDialogView,
	SessionViewState,
	ThreadViewState,
	ViewState,
} from "$lib/context/context.types";
import { getAppEnvironment } from "$lib/app-environment";
import {
	getAvailableThemes,
	getColorScheme,
	getThemeMode,
	resolveThemeMode,
} from "$lib/theme";
import { createIdleStatus } from "$lib/context/cache";
import { createCredentialsState } from "$lib/context/domains/credentials";
import { createDebugState } from "$lib/context/debug";
import { createModelsState } from "$lib/context/domains/models";
import { uiStateStore } from "$lib/context/domains/preferences";
import { createSandboxProvidersState } from "$lib/context/domains/sandbox-providers";
import { createSessionsState } from "$lib/context/domains/sessions";
import { createStartupTasksState } from "$lib/context/domains/startup-tasks";
import { createSupportInfoState } from "$lib/context/domains/support-info";
import { createWorkspacesState } from "$lib/context/domains/workspaces";
import { diffReviewPreferencesStore } from "$lib/context/stores/diff-review-preferences";
import { lastSessionWorkspaceStore } from "$lib/context/stores/last-session-workspace";
import { sidebarLayoutStore } from "$lib/context/stores/sidebar-layout";
import { threadSelectionStore } from "$lib/context/stores/thread-selection";

export const DEFAULT_PROJECT_ID = "local";

export function createInitialDataState(bootstrap: Bootstrap = {}): DataState {
	const environment = getAppEnvironment();
	return {
		environment: {
			apiBase: bootstrap.environment?.apiBase ?? environment.apiBase,
			runtime: bootstrap.environment?.runtime ?? environment.runtime,
			isDesktop: bootstrap.environment?.isDesktop ?? environment.isDesktop,
			supportsNativeWindowControls:
				bootstrap.environment?.supportsNativeWindowControls ??
				environment.supportsNativeWindowControls,
			supportsAppUpdates:
				bootstrap.environment?.supportsAppUpdates ??
				environment.supportsAppUpdates,
			windowControlsSide:
				bootstrap.environment?.windowControlsSide ??
				environment.windowControlsSide,
			windowControls: bootstrap.windowControls ?? [],
		},
		project: {
			id: bootstrap.projectId ?? DEFAULT_PROJECT_ID,
			status: createIdleStatus(),
		},
		sessions: createSessionsState(),
		workspaces: createWorkspacesState(),
		startupTasks: createStartupTasksState(),
		models: createModelsState(),
		credentials: createCredentialsState(),
		sandboxProviders: createSandboxProvidersState(),
		supportInfo: createSupportInfoState(),
	};
}

export function createInitialViewState(bootstrap: Bootstrap = {}): ViewState {
	const pendingSessionId = generateId();
	const projectId = bootstrap.projectId ?? DEFAULT_PROJECT_ID;
	const storedSelection = threadSelectionStore.readInitial();
	const selectedSessionId =
		bootstrap.selectedSessionId ?? storedSelection?.sessionId ?? null;
	const selectedThreadId =
		bootstrap.selectedThreadId ?? storedSelection?.threadId ?? null;
	const theme = getThemeMode();
	const resolvedTheme = resolveThemeMode(theme);
	const colorScheme = getColorScheme();
	return {
		app: {
			environment: {
				isMobile: false,
				isMacPlatform: false,
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
				theme,
				resolvedTheme,
				colorScheme,
				availableThemes: getAvailableThemes(resolvedTheme),
				promptHistory: uiStateStore.promptHistory,
				pinnedPrompts: uiStateStore.pinnedPrompts,
				preferredIde: uiStateStore.preferredIde,
				ideOptions: bootstrap.ideOptions ?? [],
				chatWidthMode: uiStateStore.chatWidthMode,
				defaultModel: uiStateStore.defaultModel,
				defaultReasoning: uiStateStore.defaultReasoning,
				defaultServiceTier: uiStateStore.defaultServiceTier,
				recentThreadsVisibleLimit: uiStateStore.recentThreadsVisibleLimit,
				sidebarRecentOpen: uiStateStore.sidebarRecentOpen,
				sidebarAllOpen: uiStateStore.sidebarAllOpen,
				sidebarAllGroupedByWorkspace: uiStateStore.sidebarAllGroupedByWorkspace,
				showRefreshButton: uiStateStore.showRefreshButton,
				topBarIconOnly: uiStateStore.topBarIconOnly,
				autoScrollOnStream: uiStateStore.autoScrollOnStream,
				ignoredUpdateVersion: uiStateStore.ignoredUpdateVersion,
				trackPrereleases: uiStateStore.trackPrereleases,
			},
			recentThreads: {
				visibleItems: [],
			},
			diffReview: {
				approvals: diffReviewPreferencesStore.readApprovals(),
				style: diffReviewPreferencesStore.readStyle(),
			},
			startupTasks: {
				visibleIds: [],
				hasActiveTasks: false,
			},
			updates: {
				showBadge: false,
				status: "idle",
				availableVersion: null,
				error: null,
				downloadedBytes: 0,
				totalBytes: null,
				isIgnored: false,
				canTrackPrereleases: false,
			},
			projectEvents: {
				connected: false,
			},
			lastSessionWorkspaceSelection:
				lastSessionWorkspaceStore.readInitial(projectId),
			debug: createDebugState(),
		},
		selection: {
			sessionId: selectedSessionId,
			threadId: selectedThreadId,
			pendingSessionId,
			requestedThreadIdBySessionId: {},
		},
		navigation: {
			desktopSidebarOpen: false,
			mobileSidebarOpen: false,
			hasSavedDesktopSidebarLayout: sidebarLayoutStore.hasSavedDesktopLayout(),
			mountedSessionIds: [selectedSessionId, pendingSessionId].filter(
				(sessionId): sessionId is string => !!sessionId,
			),
		},
		sessions: {},
	};
}

export function createInitialSessionViewState(
	sessionId: string,
): SessionViewState {
	return {
		sessionId,
		workspace: {
			activeView: "conversation",
			selectedFile: "",
			activeServiceId: null,
			terminalRootEnabled: false,
			dockMaximized: false,
		},
		composer: {
			draft: "",
		},
		files: {
			selected: "",
			activePath: "",
			openPaths: [],
			showChangedOnly: false,
			expandedPaths: [],
			loadingPaths: {},
			buffers: {},
			editorModels: {},
			editorViewStates: {},
			diffTarget: "",
			diffFilesByTarget: {},
		},
		hooks: {
			expanded: false,
			dialog: {
				open: false,
				selectedHookId: null,
			},
		},
		services: {
			activeServiceId: null,
			activeViewMode: "preview",
		},
		commands: {
			credentialDialog: createInitialCommandCredentialDialogView(),
		},
		queue: {
			expanded: false,
		},
		pendingWorkspace: {
			option: "new-workspace",
			branch: "",
			sourceInput: "",
			validation: null,
			validating: false,
			setupMessage: null,
			sandboxProviderId: "",
		},
		threads: {},
	};
}

export function createInitialThreadViewState(
	sessionId: string,
	threadId: string,
): ThreadViewState {
	return {
		sessionId,
		threadId,
		composer: {
			nextModelId: undefined,
			nextReasoning: undefined,
			nextServiceTier: undefined,
			pendingComments: [],
		},
		conversation: {
			scrollTop: 0,
		},
	};
}

function createInitialCommandCredentialDialogView(): SessionCommandCredentialDialogView {
	return {
		open: false,
		command: null,
		requests: [],
		projectCredentials: [],
		credentialTypes: [],
		sessionAssignments: [],
		selectedOptionByEnvVar: {},
		createCredentialNamesByEnvVar: {},
		createCredentialSecretsByEnvVar: {},
		validityPresetByEnvVar: {},
		validityValueByEnvVar: {},
		validityUnitByEnvVar: {},
		error: null,
	};
}
