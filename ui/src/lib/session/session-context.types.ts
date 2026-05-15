import type {
	AgentCommand,
	BrowserEventChunkData,
	AgentCommandCredentialRequest,
	ChatMessage,
	CredentialInfo,
	CredentialType,
	FileStatus,
	HookRunStatus as ApiHookRunStatus,
	HooksStatusResponse,
	QueuedPrompt,
	ServiceStatus,
	Session,
	SessionCredentialAssignment,
	SessionDiffFileEntry,
	SessionDiffStats,
	StartChatResponse,
	Thread,
	UpdateQueuedPromptRequest,
} from "$lib/api-types";
import type { DynamicToolPart } from "$lib/components/ai/types";
import type { SessionViewState } from "$lib/session/view/create-session-view-state.svelte";
import type { AsyncStatus } from "$lib/resource/types";

export type HookOutputState = {
	output: string;
	sizeBytes: number;
	displayedBytes: number;
	tooLarge: boolean;
};

export type HookRunStatus = Pick<
	ApiHookRunStatus,
	"hookId" | "hookName" | "type" | "lastResult" | "runCount" | "failCount"
> & {
	command?: string;
	lastRunAt?: string;
	lastExitCode?: number;
};

export type HooksStatus = {
	hooks: HookRunStatus[];
	pendingHookIds: string[];
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
};

export type SessionHooksService = {
	status: HooksStatus;
	outputById: Record<string, HookOutputState>;
	resourceStatus: AsyncStatus;
	error: string | null;
	isRefreshing: boolean;
	isStale: boolean;
	fetchedAt: number | null;
	rerun: (hookId: string) => void;
	refresh: () => Promise<void>;
	invalidate: () => void;
	applyStatusUpdate: (status: HooksStatusResponse) => Promise<void>;
};

export type SessionThreadsService = {
	list: Thread[];
	status: AsyncStatus;
	selectedId: string;
	selected: Thread | null;
	get: (threadId: string) => Thread | null;
	upsert: (thread: Thread) => void;
	refresh: () => Promise<void>;
	select: (threadId: string | null) => void;
	create: (name?: string) => Promise<string | null>;
	rename: (threadId: string, nextName: string) => Promise<boolean>;
	remove: (threadId: string) => Promise<boolean>;
	refreshThread: (threadId: string) => Promise<void>;
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

export type SessionFilesDomain = {
	list: string[];
	searchable: string[];
	diff: SessionDiffFileEntry[];
	diffStats: SessionDiffStats;
	diffTarget: string;
	contents: Record<string, string>;
	selected: string;
	activePath: string;
	openPaths: string[];
	tree: SessionFileTreeNode[];
	showChangedOnly: boolean;
	expandedPaths: string[];
	getRecord: (path: string) => SessionFileRecord | null;
	getBuffer: (path: string) => SessionFileBufferState | null;
	isPathLoading: (path: string) => boolean;
	hasDirtyChanges: (path: string) => boolean;
	open: (file?: string) => Promise<void>;
	close: (file: string) => void;
	refresh: () => Promise<void>;
	setDiffTarget: (target: string) => Promise<void>;
	toggleChangedOnly: () => Promise<void>;
	toggleDirectory: (path: string) => Promise<void>;
	expandAll: () => Promise<void>;
	collapseAll: () => void;
	rename: (path: string, nextName: string) => Promise<boolean>;
	remove: (path: string) => Promise<boolean>;
	updateBuffer: (path: string, content: string) => void;
	discard: (path: string) => void;
	save: (path: string) => Promise<boolean>;
	acceptConflict: (path: string) => void;
	forceSave: (path: string) => Promise<boolean>;
	getEditorModel: (path: string) => unknown | null;
	setEditorModel: (path: string, model: unknown | null) => void;
	getEditorViewState: (path: string) => unknown | null;
	setEditorViewState: (path: string, viewState: unknown | null) => void;
	dispose: () => void;
};

export type SessionServicesDomain = {
	list: ServiceItem[];
	active: ServiceItem | null;
	status: AsyncStatus;
	error: string | null;
	isRefreshing: boolean;
	isStale: boolean;
	fetchedAt: number | null;
	open: (serviceId: string) => void;
	start: (serviceId: string) => Promise<void>;
	stop: (serviceId: string) => Promise<void>;
	refresh: () => Promise<void>;
	invalidate: () => void;
};

export type SessionCommandCredentialDialogState = {
	open: boolean;
	command: AgentCommand | null;
	requests: AgentCommandCredentialRequest[];
	projectCredentials: CredentialInfo[];
	credentialTypes: CredentialType[];
	sessionAssignments: SessionCredentialAssignment[];
	selectedOptionByEnvVar: Record<string, string>;
	createCredentialNamesByEnvVar: Record<string, string>;
	createCredentialSecretsByEnvVar: Record<string, string>;
	validityPresetByEnvVar: Record<
		string,
		"15_minutes" | "1_hour" | "1_day" | "1_week" | "custom"
	>;
	validityValueByEnvVar: Record<string, string>;
	validityUnitByEnvVar: Record<string, "hours" | "days" | "weeks" | "never">;
	error: string | null;
	selectOption: (envVar: string, value: string) => void;
	setCreateCredentialName: (envVar: string, value: string) => void;
	setCreateCredentialSecret: (envVar: string, value: string) => void;
	setValidityPreset: (
		envVar: string,
		value: "15_minutes" | "1_hour" | "1_day" | "1_week" | "custom",
	) => void;
	setValidityValue: (envVar: string, value: string) => void;
	setValidityUnit: (
		envVar: string,
		value: "hours" | "days" | "weeks" | "never",
	) => void;
	launchOAuthWizard: (envVar: string) => Promise<void>;
	refreshAvailableCredentials: () => Promise<void>;
	close: () => void;
	confirm: () => Promise<void>;
};

export type SessionCommandsDomain = {
	list: AgentCommand[];
	uiVisible: AgentCommand[];
	status: AsyncStatus;
	error: string | null;
	isRefreshing: boolean;
	isStale: boolean;
	fetchedAt: number | null;
	isSubmitting: boolean;
	credentialDialog: SessionCommandCredentialDialogState;
	refresh: () => Promise<void>;
	invalidate: () => void;
	run: (command: AgentCommand) => Promise<void>;
};

export type SessionConversationDomain = {
	messages: ChatMessage[];
	browserEventsByTurnId: Record<string, BrowserEventChunkData[]>;
	status: AsyncStatus;
	isStreaming: boolean;
	error: string | null;
	cancel: () => Promise<void>;
};

export type ThreadSubmitResult = {
	sessionId: string;
	threadId: string;
	queued?: boolean;
};

export type SubmitPromptOptions = {
	threadId?: string | null;
};

export type SubmitPromptResult = StartChatResponse | ThreadSubmitResult | void;

export type SelectionComment = {
	snippet: string;
	comment: string;
};

export type ThreadContextValue = {
	threadId: string;
	thread: Thread | null;
	modelId: string | null;
	reasoning: string | undefined;
	serviceTier: string | undefined;
	nextModelId: string | null | undefined;
	nextReasoning: string | undefined;
	nextServiceTier: string | null | undefined;
	setNextModelId: (modelId: string | null | undefined) => void;
	setNextReasoning: (reasoning: string | undefined) => void;
	setNextServiceTier: (serviceTier: string | null | undefined) => void;
	clearNextComposerValues: () => void;
	messages: ChatMessage[];
	browserEventsByTurnId: Record<string, BrowserEventChunkData[]>;
	promptQueue: QueuedPrompt[];
	status: AsyncStatus;
	isStreaming: boolean;
	error: string | null;
	hasPendingQuestion: boolean;
	pendingQuestionToolPart: DynamicToolPart | null;
	pendingQuestionLoading: boolean;
	pendingQuestionError: string | null;
	clearComposerDraft: (storageKey?: string) => void;
	submit: (payload: {
		parts: ChatMessage["parts"];
		workspaceId?: string;
		providerId?: string;
		workspaceType?: "local" | "git" | null;
		workspacePath?: string | null;
		runAfter?: string;
	}) => Promise<ThreadSubmitResult | void>;
	cancel: () => Promise<void>;
	start: () => Promise<void>;
	refresh: () => Promise<void>;
	addToolApprovalResponse: (payload: {
		id: string;
		approved: boolean;
		reason?: string;
	}) => void;
	deleteQueuedPrompt: (queueId: string) => Promise<void>;
	updateQueuedPrompt: (
		queueId: string,
		payload: UpdateQueuedPromptRequest,
	) => Promise<void>;
	dispose: () => void;
};

export type SessionContextValue = {
	sessionId: string;
	isPending: boolean;
	current: Session | null;
	dispose: () => void;
	ensureThread: (threadId: string) => ThreadContextValue;
	ui: SessionViewState;
	threads: SessionThreadsService;
	hooks: SessionHooksService;
	files: SessionFilesDomain;
	services: SessionServicesDomain;
	commands: SessionCommandsDomain;
	submit: (
		text: string,
		options?: SubmitPromptOptions,
	) => Promise<SubmitPromptResult>;
	threadContexts: Map<string, ThreadContextValue>;
	conversationScrollTopByThreadId: Map<string, number>;
};
