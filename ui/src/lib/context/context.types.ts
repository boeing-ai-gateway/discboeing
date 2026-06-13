import type {
	AgentCommand,
	AgentCommandCredentialRequest,
	CodexAuthorizeResponse,
	CodexCallbackStatusRequest,
	CodexCallbackStatusResponse,
	CodexDeviceCodeResponse,
	CodexExchangeRequest,
	CodexExchangeResponse,
	CodexPollRequest,
	CodexPollResponse,
	CreateCredentialRequest,
	CreateThreadRequest,
	CredentialInfo,
	CredentialType,
	GitHubAuthorizeRequest,
	GitHubAuthorizeResponse,
	GitHubCallbackStatusRequest,
	GitHubCallbackStatusResponse,
	GitHubDeviceCodeRequest,
	GitHubDeviceCodeResponse,
	GitHubExchangeRequest,
	GitHubExchangeResponse,
	GitHubPollRequest,
	GitHubPollResponse,
	OAuthAuthorizeResponse,
	OAuthExchangeRequest,
	OAuthExchangeResponse,
	StartChatRequest,
	SessionCredentialAssignment,
	SessionDiffFileEntry,
	SessionDiffStats,
	ThemeColorScheme,
	ChatMessage,
	UpdateThreadRequest,
	UpdateQueuedPromptRequest,
	Workspace,
	WorkspaceValidationResult,
} from "$lib/api-types";
import type { ConversationComment } from "$lib/conversation-helpers";
import type { SwitcherCommitModifier } from "$lib/shortcuts/global-shortcuts";
import type { IdeOption, PreferredIde } from "$lib/shell/ide-options";
import type { RecentThreadEntry } from "$lib/context/view/thread-switcher";

export type { ConversationComment } from "$lib/conversation-helpers";
import type {
	DesktopRuntimeKind,
	WindowControlsSide,
} from "$lib/desktop/types";
import type { ResourceStatus } from "$lib/context/cache";
import type { ResolvedTheme, ThemeMetadata, ThemeMode } from "$lib/theme";

type NormalizedSessionDiffFilesResponse = {
	files: SessionDiffFileEntry[];
	stats: SessionDiffStats;
};
import type { DiffStyle } from "$lib/pierre-diff";
import type { CredentialsState } from "$lib/context/domains/credentials";
import type { ModelsState } from "$lib/context/domains/models";
import type {
	CreateSandboxProviderInput,
	SandboxProviderMutationInput,
	SandboxProvidersState,
} from "$lib/context/domains/sandbox-providers";
import type { SessionsState } from "$lib/context/domains/sessions";
import type { StartupTasksState } from "$lib/context/domains/startup-tasks";
import type { SupportInfoState } from "$lib/context/domains/support-info";
import type { WorkspacesState } from "$lib/context/domains/workspaces";
import type { DebugState } from "$lib/context/debug";

export type CommandOptions = {
	wait?: boolean;
};

export type Context = {
	data: DataState;
	view: ViewState;
	commands: Commands;
};

export type ChatWidthMode = "full" | "constrained";

export type SettingsDialogTab =
	| "appearance"
	| "chat"
	| "providers"
	| "update"
	| "credentials";

export type SessionDockViewKind =
	| "terminal"
	| "desktop"
	| "vscode"
	| "file"
	| "diff-review"
	| "services";

export type UpdateStatus =
	| "idle"
	| "checking"
	| "downloading"
	| "ready"
	| "installing"
	| "error";

export type DataState = {
	environment: {
		apiBase: string;
		runtime: DesktopRuntimeKind;
		isDesktop: boolean;
		supportsNativeWindowControls: boolean;
		supportsAppUpdates: boolean;
		windowControlsSide: WindowControlsSide;
		windowControls: string[];
	};
	project: {
		id: string;
		status: ResourceStatus;
	};
	sessions: SessionsState;
	workspaces: WorkspacesState;
	startupTasks: StartupTasksState;
	models: ModelsState;
	credentials: CredentialsState;
	sandboxProviders: SandboxProvidersState;
	supportInfo: SupportInfoState;
};

export type ViewState = {
	app: AppViewState;
	selection: {
		sessionId: string | null;
		threadId: string | null;
		pendingSessionId: string;
		requestedThreadIdBySessionId: Record<string, string>;
	};
	navigation: {
		desktopSidebarOpen: boolean;
		mobileSidebarOpen: boolean;
		hasSavedDesktopSidebarLayout: boolean;
		mountedSessionIds: string[];
	};
	sessions: Record<string, SessionViewState | undefined>;
};

export type AppViewState = {
	environment: {
		isMobile: boolean;
		isMacPlatform: boolean;
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
		theme: string;
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
	diffReview: {
		approvals: Record<string, Record<string, string>>;
		style: DiffStyle;
	};
	startupTasks: {
		visibleIds: string[];
		hasActiveTasks: boolean;
	};
	updates: {
		showBadge: boolean;
		status: UpdateStatus;
		availableVersion: string | null;
		error: string | null;
		downloadedBytes: number;
		totalBytes: number | null;
		isIgnored: boolean;
		canTrackPrereleases: boolean;
	};
	projectEvents: {
		connected: boolean;
	};
	lastSessionWorkspaceSelection: string | null;
	debug: DebugState;
};

export type SessionViewState = {
	sessionId: string;
	workspace: {
		activeView: string;
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
		diffTarget: string;
		diffFilesByTarget: Record<string, NormalizedSessionDiffFilesResponse>;
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
		activeViewMode: "preview" | "logs";
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
	threads: Record<string, ThreadViewState>;
};

export type ThreadViewState = {
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
		stickToBottom: boolean;
	};
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

export type CreateSessionInput = {
	id: string;
	workspaceId?: string;
	providerId?: string;
	model?: string;
	reasoning?: string;
};

export type SendMessageInput = Pick<
	StartChatRequest,
	"messages" | "model" | "reasoning" | "serviceTier" | "runAfter" | "trigger"
>;

export type Commands = {
	lifecycle: {
		startup(options?: CommandOptions): Promise<void>;
		shutdown(): void;
	};
	projects: {
		activateProject(projectId: string, options?: CommandOptions): Promise<void>;
	};
	sessions: {
		activateSession(sessionId: string, options?: CommandOptions): Promise<void>;
		deactivateSession(sessionId: string): Promise<void>;
		refreshSession(sessionId: string, options?: CommandOptions): Promise<void>;
		createSession(
			input: CreateSessionInput,
			options?: CommandOptions,
		): Promise<void>;
		renameSession(
			sessionId: string,
			name: string,
			options?: CommandOptions,
		): Promise<void>;
		stopSession(sessionId: string, options?: CommandOptions): Promise<void>;
		deleteSession(sessionId: string, options?: CommandOptions): Promise<void>;
	};
	threads: {
		activateThread(
			sessionId: string,
			threadId: string,
			options?: CommandOptions,
		): Promise<void>;
		deactivateThread(sessionId: string, threadId: string): Promise<void>;
		createThread(
			sessionId: string,
			input: CreateThreadRequest,
			options?: CommandOptions,
		): Promise<void>;
		renameThread(
			sessionId: string,
			threadId: string,
			name: string,
			options?: CommandOptions,
		): Promise<void>;
		updateThread(
			sessionId: string,
			threadId: string,
			input: UpdateThreadRequest,
			options?: CommandOptions,
		): Promise<void>;
		deleteThread(
			sessionId: string,
			threadId: string,
			options?: CommandOptions,
		): Promise<void>;
		sendMessage(
			sessionId: string,
			threadId: string,
			input: SendMessageInput,
			options?: CommandOptions,
		): Promise<void>;
	};
	view: {
		mountSessionView(sessionId: string): Promise<void>;
		mountThreadView(sessionId: string, threadId: string): Promise<void>;
		setSessionHooksExpanded(
			sessionId: string,
			expanded: boolean,
		): Promise<void>;
		setPendingWorkspaceSandboxProviderId(
			sessionId: string,
			providerId: string,
		): Promise<void>;
		setLastSessionWorkspaceSelection(option: string): Promise<void>;
		resetPendingWorkspaceSetup(sessionId: string): Promise<void>;
	};
	navigation: {
		setDesktopSidebarOpen(open: boolean): Promise<void>;
		setMobileSidebarOpen(open: boolean): Promise<void>;
		toggleMobileSidebarOpen(): Promise<void>;
		startNewSession(): Promise<void>;
		completePendingSession(
			pendingSessionId: string,
			sessionId: string,
		): Promise<void>;
		selectSession(sessionId: string): Promise<void>;
		openThread(sessionId: string, threadId: string): Promise<void>;
		toggleSelectedSessionView(viewKind: SessionDockViewKind): Promise<void>;
	};
	dialogs: {
		setSettingsDialogOpen(open: boolean): Promise<void>;
		setSettingsDialogTab(tab: SettingsDialogTab): Promise<void>;
		openSettingsDialog(tab?: SettingsDialogTab): Promise<void>;
		closeSettingsDialog(): Promise<void>;
		openCredentialsDialog(credentialId?: string): Promise<void>;
		openGitHubCredentialFlow(): Promise<void>;
		clearCredentialsDialogTarget(): Promise<void>;
		clearCredentialFlowIntent(): Promise<void>;
		openSupportInfoDialog(): Promise<void>;
		closeSupportInfoDialog(): Promise<void>;
		setKeyboardShortcutsOpen(open: boolean): Promise<void>;
		toggleKeyboardShortcutsOpen(): Promise<void>;
		setRecentThreadSwitcherOpen(open: boolean): Promise<void>;
		setRecentThreadSwitcherSelectedKey(key: string | null): Promise<void>;
		setRecentThreadSwitcherCommitModifier(
			modifier: SwitcherCommitModifier | null,
		): Promise<void>;
		closeKeyboardShortcutOverlays(): Promise<void>;
	};
	supportInfo: {
		fetchSupportInfo(): Promise<void>;
	};
	preferences: {
		setPreferredIde(preferredIde: PreferredIde): Promise<void>;
		setTheme(theme: ThemeMode): Promise<void>;
		setColorScheme(colorScheme: ThemeColorScheme): Promise<void>;
		setRecentThreadsVisibleLimit(value: number): Promise<void>;
		setShowRefreshButton(show: boolean): Promise<void>;
		setTopBarIconOnly(iconOnly: boolean): Promise<void>;
		setDefaultModel(modelId: string): Promise<void>;
		setDefaultReasoning(reasoning: string): Promise<void>;
		setDefaultServiceTier(serviceTier: string): Promise<void>;
		setChatWidthMode(mode: ChatWidthMode): Promise<void>;
		setAutoScrollOnStream(enabled: boolean): Promise<void>;
		setSidebarRecentOpen(open: boolean): Promise<void>;
		setSidebarAllOpen(open: boolean): Promise<void>;
		setSidebarAllGroupedByWorkspace(grouped: boolean): Promise<void>;
		setDiffReviewApprovals(
			approvals: Record<string, Record<string, string>>,
		): Promise<void>;
		setDiffReviewStyle(style: DiffStyle): Promise<void>;
		addPromptToHistory(prompt: string): Promise<void>;
		removePromptFromHistory(prompt: string): Promise<void>;
		pinPrompt(prompt: string): Promise<void>;
		unpinPrompt(prompt: string): Promise<void>;
	};
	threadComposer: {
		setComposerDraft(sessionId: string, value: string): Promise<void>;
		clearComposerDraft(
			sessionId: string,
			threadId: string,
			storageKey?: string,
		): Promise<void>;
		movePendingComposerDraftToThread(
			threadId: string,
			nextThreadId: string,
			value: string,
		): Promise<void>;
		setThreadNextModelId(
			sessionId: string,
			threadId: string,
			modelId: string | null | undefined,
		): Promise<void>;
		setThreadNextReasoning(
			sessionId: string,
			threadId: string,
			reasoning: string | undefined,
		): Promise<void>;
		setThreadNextServiceTier(
			sessionId: string,
			threadId: string,
			serviceTier: string | null | undefined,
		): Promise<void>;
		clearThreadNextComposerValues(
			sessionId: string,
			threadId: string,
		): Promise<void>;
		addThreadPendingComment(
			sessionId: string,
			threadId: string,
			comment: Omit<ConversationComment, "id">,
		): Promise<void>;
		removeThreadPendingComment(
			sessionId: string,
			threadId: string,
			commentId: string,
		): Promise<void>;
		clearThreadPendingComments(
			sessionId: string,
			threadId: string,
		): Promise<void>;
		setConversationScrollTop(
			sessionId: string,
			threadId: string,
			scrollTop: number,
		): Promise<void>;
		setConversationStickToBottom(
			sessionId: string,
			threadId: string,
			stickToBottom: boolean,
		): Promise<void>;
		addToolApprovalResponse(
			sessionId: string,
			threadId: string,
			payload: { id: string; approved: boolean; reason?: string },
		): Promise<void>;
		refreshThread(
			sessionId: string,
			threadId: string,
			options?: CommandOptions,
		): Promise<void>;
		submitThread(
			sessionId: string,
			threadId: string,
			payload: {
				parts: ChatMessage["parts"];
				workspaceId?: string;
				providerId?: string;
				workspaceType?: "local" | "git" | null;
				workspacePath?: string | null;
				allowEmptyPendingMessage?: boolean;
				runAfter?: string;
			},
		): Promise<{ sessionId: string; threadId: string } | void>;
		cancelThread(sessionId: string, threadId: string): Promise<void>;
		deleteQueuedPrompt(
			sessionId: string,
			threadId: string,
			queueId: string,
		): Promise<void>;
		updateQueuedPrompt(
			sessionId: string,
			threadId: string,
			queueId: string,
			payload: UpdateQueuedPromptRequest,
		): Promise<void>;
	};
	agentCommands: {
		runAgentCommand(sessionId: string, command: AgentCommand): Promise<void>;
		closeCommandCredentialDialog(sessionId: string): Promise<void>;
		confirmCommandCredentialDialog(sessionId: string): Promise<void>;
		selectCommandCredentialOption(
			sessionId: string,
			envVar: string,
			value: string,
		): Promise<void>;
		setCommandCredentialCreateName(
			sessionId: string,
			envVar: string,
			value: string,
		): Promise<void>;
		setCommandCredentialCreateSecret(
			sessionId: string,
			envVar: string,
			value: string,
		): Promise<void>;
		setCommandCredentialValidityPreset(
			sessionId: string,
			envVar: string,
			value: CredentialValidityPreset,
		): Promise<void>;
		setCommandCredentialValidityValue(
			sessionId: string,
			envVar: string,
			value: string,
		): Promise<void>;
		setCommandCredentialValidityUnit(
			sessionId: string,
			envVar: string,
			value: CredentialValidityUnit,
		): Promise<void>;
		launchCommandCredentialOAuthWizard(
			sessionId: string,
			envVar: string,
		): Promise<void>;
		refreshCommandCredentialDialogCredentials(sessionId: string): Promise<void>;
	};
	credentials: {
		refreshCredentials(options?: CommandOptions): Promise<void>;
		createCredential(input: CreateCredentialRequest): Promise<CredentialInfo>;
		deleteCredential(credentialId: string): Promise<void>;
		toggleCredentialInactive(credential: CredentialInfo): Promise<void>;
		codexAuthorize(): Promise<CodexAuthorizeResponse>;
		codexDeviceCode(): Promise<CodexDeviceCodeResponse>;
		codexCallbackStatus(
			input: CodexCallbackStatusRequest,
		): Promise<CodexCallbackStatusResponse>;
		codexPoll(input: CodexPollRequest): Promise<CodexPollResponse>;
		codexExchange(input: CodexExchangeRequest): Promise<CodexExchangeResponse>;
		githubAuthorize(
			input: GitHubAuthorizeRequest,
		): Promise<GitHubAuthorizeResponse>;
		githubDeviceCode(
			input: GitHubDeviceCodeRequest,
		): Promise<GitHubDeviceCodeResponse>;
		githubCallbackStatus(
			input: GitHubCallbackStatusRequest,
		): Promise<GitHubCallbackStatusResponse>;
		githubPoll(input: GitHubPollRequest): Promise<GitHubPollResponse>;
		githubExchange(
			input: GitHubExchangeRequest,
		): Promise<GitHubExchangeResponse>;
		anthropicAuthorize(): Promise<OAuthAuthorizeResponse>;
		anthropicExchange(
			input: OAuthExchangeRequest,
		): Promise<OAuthExchangeResponse>;
	};
	sessionCredentials: {
		refreshSessionCredentials(
			sessionId: string,
			options?: CommandOptions,
		): Promise<void>;
		setSessionCredentialAssignments(
			sessionId: string,
			assignments: SessionCredentialAssignment[],
			options?: CommandOptions,
		): Promise<void>;
	};
	sandboxProviders: {
		refreshSandboxProviders(options?: CommandOptions): Promise<void>;
		createSandboxProvider(
			input: CreateSandboxProviderInput,
			options?: CommandOptions,
		): Promise<void>;
		updateSandboxProvider(
			id: string,
			input: SandboxProviderMutationInput,
			options?: CommandOptions,
		): Promise<void>;
		deleteSandboxProvider(id: string, options?: CommandOptions): Promise<void>;
		updateDefaultSandboxProvider(
			providerId: string,
			options?: CommandOptions,
		): Promise<void>;
	};
	workspaces: {
		renameWorkspace(workspaceId: string, displayName: string): Promise<void>;
		deleteWorkspace(workspaceId: string): Promise<void>;
	};
	files: {
		refreshFileSubtree(
			sessionId: string,
			path: string,
			options?: CommandOptions,
		): Promise<void>;
		openFile(
			sessionId: string,
			path?: string,
			options?: CommandOptions,
		): Promise<void>;
		openFilesPanel(sessionId: string): Promise<void>;
		setDiffTarget(sessionId: string, target: string): Promise<void>;
		saveFile(
			sessionId: string,
			path: string,
			content: string,
			options?: CommandOptions & {
				encoding?: "utf8" | "base64";
				originalContent?: string;
			},
		): Promise<void>;
		renameFile(
			sessionId: string,
			from: string,
			to: string,
			options?: CommandOptions,
		): Promise<void>;
		deleteFile(
			sessionId: string,
			path: string,
			options?: CommandOptions,
		): Promise<void>;
	};
	hooks: {
		rerunHook(
			sessionId: string,
			hookId: string,
			options?: CommandOptions,
		): Promise<void>;
		pauseHooks(
			sessionId: string,
			paused: boolean,
			options?: CommandOptions,
		): Promise<void>;
		pauseHook(
			sessionId: string,
			hookId: string,
			paused: boolean,
			options?: CommandOptions,
		): Promise<void>;
	};
	services: {
		openServicePanel(
			sessionId: string,
			serviceId: string,
			viewMode?: "preview" | "logs",
		): Promise<void>;
		startService(
			sessionId: string,
			serviceId: string,
			options?: CommandOptions,
		): Promise<void>;
		stopService(
			sessionId: string,
			serviceId: string,
			options?: CommandOptions,
		): Promise<void>;
		bindServiceLocalhost(
			sessionId: string,
			serviceId: string,
			port: number,
			options?: CommandOptions,
		): Promise<void>;
		unbindServiceLocalhost(
			sessionId: string,
			serviceId: string,
			options?: CommandOptions,
		): Promise<void>;
	};
	updates: {
		checkForUpdates(): Promise<void>;
		setTrackPrereleases(track: boolean): Promise<void>;
		installUpdateAndRelaunch(): Promise<void>;
		ignoreUpdate(): Promise<void>;
	};
};

export type Bootstrap = {
	projectId?: string;
	selectedSessionId?: string;
	selectedThreadId?: string;
	workspaces?: Workspace[];
	ideOptions?: IdeOption[];
	windowControls?: string[];
	environment?: {
		apiBase?: string;
		runtime?: DesktopRuntimeKind;
		isDesktop?: boolean;
		supportsNativeWindowControls?: boolean;
		supportsAppUpdates?: boolean;
		windowControlsSide?: WindowControlsSide;
	};
};
