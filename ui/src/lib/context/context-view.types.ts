import type {
	AgentCommand,
	AgentCommandCredentialRequest,
	CredentialInfo,
	CredentialType,
	SessionCredentialAssignment,
	ThemeColorScheme,
	WorkspaceValidationResult,
} from "$lib/api-types";
import type {
	ChatWidthMode,
	SettingsDialogTab,
} from "$lib/app/app-context.types";
import type { SwitcherCommitModifier } from "$lib/app/global-shortcuts";
import type { RecentThreadEntry } from "$lib/app/thread-switcher";
import type { IdeOption, PreferredIde } from "$lib/app/ide-options";
import type { SessionActiveView } from "$lib/session/session-view.types";
import type { ThemeMetadata, ThemeMode, ResolvedTheme } from "$lib/theme";

export type ContextView = {
	app: AppView;
	sessions: Record<string, SessionView>;
};

export type AppView = {
	navigation: {
		desktopSidebarOpen: boolean;
		mobileSidebarOpen: boolean;
		mountedSessionIds: string[];
	};
	selection: {
		sessionId: string | null;
		threadId: string | null;
		pendingSessionId: string;
		requestedThreadIdBySessionId: Record<string, string>;
	};
	dialogs: {
		settings: {
			open: boolean;
			tab: SettingsDialogTab;
		};
		credentials: {
			open: boolean;
			targetId: string | null;
			flowIntent: "github-git" | "codex" | null;
		};
		supportInfo: {
			open: boolean;
		};
		keyboardShortcuts: {
			open: boolean;
		};
		recentThreadSwitcher: {
			open: boolean;
			selectedKey: string | null;
			commitModifier: SwitcherCommitModifier | null;
		};
	};
	preferences: {
		theme: ThemeMode;
		resolvedTheme: ResolvedTheme;
		colorScheme: ThemeColorScheme;
		availableThemes: ThemeMetadata[];
		promptHistory: string[];
		pinnedPrompts: string[];
		preferredIde: PreferredIde;
		ideOptions: IdeOption[];
		chatWidthMode: ChatWidthMode;
		defaultModel: string;
		recentThreadsVisibleLimit: number;
		sidebarRecentOpen: boolean;
		sidebarAllOpen: boolean;
		sidebarAllGroupedByWorkspace: boolean;
		showRefreshButton: boolean;
		topBarIconOnly: boolean;
		autoScrollOnStream: boolean;
		ignoredUpdateVersion: string | null;
		trackPrereleases: boolean;
	};
	recentThreads: {
		visibleItems: RecentThreadEntry[];
	};
	startupTasks: {
		visibleIds: string[];
		hasActiveTasks: boolean;
	};
	updates: {
		showBadge: boolean;
	};
	projectEvents: {
		connected: boolean;
	};
};

export type SessionView = {
	sessionId: string;
	workspace: {
		activeView: SessionActiveView;
		selectedFile: string;
		activeServiceId: string | null;
		terminalRootEnabled: boolean;
		dockMaximized: boolean;
	};
	composer: {
		draft: string;
	};
	files: {
		selected: string;
		activePath: string;
		openPaths: string[];
		showChangedOnly: boolean;
		expandedPaths: string[];
		loadingPaths: Record<string, boolean>;
		buffers: Record<string, SessionFileBufferState>;
		editorModels: Record<string, unknown>;
		editorViewStates: Record<string, unknown>;
	};
	hooks: {
		expanded: boolean;
		dialog: {
			open: boolean;
			selectedHookId: string | null;
		};
	};
	services: {
		activeServiceId: string | null;
	};
	commands: {
		credentialDialog: SessionCommandCredentialDialogView;
	};
	queue: {
		expanded: boolean;
	};
	pendingWorkspace: {
		option: string;
		branch: string;
		sourceInput: string;
		validation: WorkspaceValidationResult | null;
		validating: boolean;
		setupMessage: string | null;
		sandboxProviderId: string;
	};
	threads: Record<string, ThreadView>;
};

export type ThreadView = {
	sessionId: string;
	threadId: string;
	composer: {
		nextModelId: string | null | undefined;
		nextReasoning: string | undefined;
		nextServiceTier: string | null | undefined;
		pendingComments: ConversationComment[];
	};
	conversation: {
		scrollTop: number;
	};
};

export type ConversationComment = {
	id: string;
	snippet: string;
	comment: string;
};

export type SessionFileBufferState = {
	content: string;
	originalContent: string;
	encoding: "utf8" | "base64";
	isDirty: boolean;
	isSaving: boolean;
	saveError: string | null;
	hasConflict: boolean;
	conflictContent: string | null;
	fromBase: boolean;
};

export type SessionCommandCredentialDialogView = {
	open: boolean;
	command: AgentCommand | null;
	requests: AgentCommandCredentialRequest[];
	projectCredentials: CredentialInfo[];
	credentialTypes: CredentialType[];
	sessionAssignments: SessionCredentialAssignment[];
	selectedOptionByEnvVar: Record<string, string>;
	createCredentialNamesByEnvVar: Record<string, string>;
	createCredentialSecretsByEnvVar: Record<string, string>;
	validityPresetByEnvVar: Record<string, CredentialValidityPreset>;
	validityValueByEnvVar: Record<string, string>;
	validityUnitByEnvVar: Record<string, CredentialValidityUnit>;
	error: string | null;
};

export type CredentialValidityPreset =
	| "15_minutes"
	| "1_hour"
	| "1_day"
	| "1_week"
	| "custom";

export type CredentialValidityUnit = "hours" | "days" | "weeks" | "never";
