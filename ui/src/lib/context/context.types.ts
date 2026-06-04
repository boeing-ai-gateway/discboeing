import type {
	AgentCommand,
	AgentCommandCredentialRequest,
	BrowserEventChunkData,
	ChatMessage,
	CredentialInfo,
	CredentialType,
	FileStatus,
	HookRunStatus as ApiHookRunStatus,
	ModelInfo,
	QueuedPrompt,
	ServiceLocalhostBind,
	ServiceStatus,
	Session,
	SessionCredentialAssignment,
	SessionDiffFileEntry,
	SessionDiffStats,
	StartupTask,
	SupportInfoResponse,
	ThemeColorScheme,
	Thread,
	Workspace,
	WorkspaceValidationResult,
} from "$lib/api-types";
import type {
	ChatWidthMode,
	SettingsDialogTab,
	UpdateStatus,
} from "$lib/app/app-context.types";
import type { SwitcherCommitModifier } from "$lib/app/global-shortcuts";
import type { IdeOption, PreferredIde } from "$lib/app/ide-options";
import type { RecentThreadEntry } from "$lib/app/thread-switcher";
import type {
	DesktopRuntimeKind,
	WindowControlsSide,
} from "$lib/desktop/types";
import type { AsyncStatus } from "$lib/resource/types";
import type { SessionActiveView } from "$lib/session/session-view.types";
import type { ThemeMetadata, ThemeMode, ResolvedTheme } from "$lib/theme";

export type Context = {
	view: ContextView;
	data: ContextData;
	actions: ContextActions;
};

export type ContextBootstrap = {
	ideOptions: IdeOption[];
	selectedSessionId?: string;
	selectedThreadId?: string;
	windowControls: string[];
};

export type ContextActions = Record<string, never>;

export type ContextView = {
	app: AppView;
	sessions: Record<string, SessionView>;
};

export type AppView = {
	environment: {
		isMobile: boolean;
		isMacPlatform: boolean;
	};
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
		defaultReasoning: string;
		defaultServiceTier: string;
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

export type ContextData = {
	environment: EnvironmentData;
	sessions: SessionsData;
	threads: ThreadsData;
	conversations: ConversationsData;
	workspaces: WorkspacesData;
	models: ModelsData;
	credentials: CredentialsData;
	startupTasks: StartupTasksData;
	files: FilesData;
	hooks: HooksData;
	services: ServicesData;
	commands: CommandsData;
	supportInfo: SupportInfoData;
	updates: UpdatesData;
};

export type EnvironmentData = {
	apiBase: string;
	runtime: DesktopRuntimeKind;
	isDesktop: boolean;
	supportsNativeWindowControls: boolean;
	supportsAppUpdates: boolean;
	windowControlsSide: WindowControlsSide;
	windowControls: string[];
};

export type SessionsData = {
	items: Session[];
	byId: Record<string, Session>;
	status: AsyncStatus;
	error: string | null;
	recentThreads: RecentThreadEntry[];
};

export type ThreadsData = {
	bySessionId: Record<string, SessionThreadsData>;
};

export type SessionThreadsData = {
	items: Thread[];
	byId: Record<string, Thread>;
	status: AsyncStatus;
	error: string | null;
};

export type ConversationsData = {
	byThreadId: Record<string, ConversationData>;
};

export type ConversationData = {
	sessionId: string;
	threadId: string;
	messages: ChatMessage[];
	browserEventsByTurnId: Record<string, BrowserEventChunkData[]>;
	status: AsyncStatus;
	error: string | null;
	isStreaming: boolean;
	hasPendingQuestion: boolean;
	pendingQuestionId: string | null;
	promptQueue: QueuedPrompt[];
};

export type WorkspacesData = {
	items: Workspace[];
	byId: Record<string, Workspace>;
	status: AsyncStatus;
	error: string | null;
};

export type ModelsData = {
	items: ModelInfo[];
	byId: Record<string, ModelInfo>;
	status: AsyncStatus;
	error: string | null;
};

export type CredentialsData = {
	items: CredentialInfo[];
	byId: Record<string, CredentialInfo>;
	types: CredentialType[];
	status: AsyncStatus;
	error: string | null;
};

export type StartupTasksData = {
	items: StartupTask[];
	byId: Record<string, StartupTask>;
	status: AsyncStatus;
	error: string | null;
};

export type FilesData = {
	bySessionId: Record<string, SessionFilesData>;
};

export type SessionFilesData = {
	list: string[];
	searchable: string[];
	diff: SessionDiffFileEntry[];
	diffStats: SessionDiffStats;
	diffTarget: string;
	contents: Record<string, SessionFileRecord>;
	tree: SessionFileTreeNode[];
	status: AsyncStatus;
	error: string | null;
};

export type SessionFileTreeNode = {
	name: string;
	path: string;
	type: "file" | "directory";
	size?: number;
	changed?: boolean;
	status?: FileStatus;
	children?: SessionFileTreeNode[];
};

export type SessionFileRecord = {
	path: string;
	content: string;
	encoding: "utf8" | "base64";
	size: number;
	fromBase: boolean;
};

export type HooksData = {
	bySessionId: Record<string, SessionHooksData>;
};

export type SessionHooksData = {
	status: HooksStatus;
	outputById: Record<string, HookOutputState>;
	resourceStatus: AsyncStatus;
	error: string | null;
	isRefreshing: boolean;
	isStale: boolean;
	fetchedAt: number | null;
};

export type HooksStatus = {
	hooks: HookRunStatus[];
	pendingHookIds: string[];
	executionPaused: boolean;
};

export type HookRunStatus = Pick<
	ApiHookRunStatus,
	"hookId" | "hookName" | "type" | "lastResult" | "runCount" | "failCount"
> & {
	command?: string;
	lastRunAt?: string;
	lastExitCode?: number;
	executionPaused: boolean;
};

export type HookOutputState = {
	output: string;
	sizeBytes: number;
	displayedBytes: number;
	tooLarge: boolean;
};

export type ServicesData = {
	bySessionId: Record<string, SessionServicesData>;
};

export type SessionServicesData = {
	items: ServiceItem[];
	byId: Record<string, ServiceItem>;
	status: AsyncStatus;
	error: string | null;
	isRefreshing: boolean;
	isStale: boolean;
	fetchedAt: number | null;
};

export type ServiceItem = {
	id: string;
	label: string;
	target: string;
	description?: string;
	order?: number;
	http?: number;
	https?: number;
	urlPath?: string;
	status: ServiceStatus;
	passive?: boolean;
	exitCode?: number;
	localhost?: ServiceLocalhostBind;
};

export type CommandsData = {
	bySessionId: Record<string, SessionCommandsData>;
};

export type SessionCommandsData = {
	items: AgentCommand[];
	visibleItems: AgentCommand[];
	status: AsyncStatus;
	error: string | null;
	isRefreshing: boolean;
	isStale: boolean;
	fetchedAt: number | null;
	isSubmitting: boolean;
};

export type SupportInfoData = {
	value: SupportInfoResponse | null;
	status: AsyncStatus;
	error: string | null;
};

export type UpdatesData = {
	status: UpdateStatus;
	availableVersion: string | null;
	error: string | null;
	downloadedBytes: number;
	totalBytes: number | null;
	isIgnored: boolean;
	canTrackPrereleases: boolean;
};
