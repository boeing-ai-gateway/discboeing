import type {
	AgentCommand,
	AgentCommandCredentialRequest,
	ChatMessage,
	CredentialInfo,
	CredentialType,
	FileStatus,
	HooksStatusResponse,
	QueuedPrompt,
	Session,
	SessionCredentialAssignment,
	SessionDiffFileEntry,
	SessionDiffStats,
	Thread,
} from "$lib/api-types";
import type { ThreadStore } from "$lib/store/threads.store.svelte";
import type { SessionViewState } from "$lib/session/view/create-session-view-state.svelte";
import type {
	AsyncStatus,
	HooksStatus,
	PlanEntry,
	ServiceItem,
	ThreadSummary,
} from "$lib/shell-types";

export type SessionStores = {
	threads: ThreadStore;
};

export type HookOutputState = {
	output: string;
	sizeBytes: number;
	displayedBytes: number;
	tooLarge: boolean;
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
	selectedId: string | null;
	selected: Thread | null;
	refresh: () => Promise<void>;
	select: (threadId: string | null) => void;
	create: (name?: string) => void;
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
	status: AsyncStatus | "streaming";
	error: string | null;
	hasPendingQuestion: boolean;
	pendingQuestionId: string | null;
	cancel: () => Promise<void>;
	refresh: () => Promise<void>;
};

export type ThreadSubmitResult = {
	sessionId: string;
	threadId: string;
	materialized: boolean;
	queued?: boolean;
};

export type ThreadContextValue = {
	threadId: string;
	thread: ThreadSummary | null;
	mode: "build" | "plan";
	modelId: string | null;
	reasoning: string | undefined;
	nextMode: "build" | "plan" | undefined;
	nextModelId: string | null | undefined;
	nextReasoning: string | undefined;
	setNextMode: (mode: "build" | "plan" | undefined) => void;
	setNextModelId: (modelId: string | null | undefined) => void;
	setNextReasoning: (reasoning: string | undefined) => void;
	clearNextComposerValues: () => void;
	messages: ChatMessage[];
	planEntries: PlanEntry[];
	promptQueue: QueuedPrompt[];
	status: AsyncStatus | "streaming";
	error: string | null;
	hasPendingQuestion: boolean;
	pendingQuestionId: string | null;
	clearComposerDraft: (storageKey?: string) => void;
	submit: (payload: {
		parts: ChatMessage["parts"];
		workspaceId?: string;
		workspaceType?: "local" | "git" | null;
		workspacePath?: string | null;
		allowEmptyPendingMessage?: boolean;
		runAfter?: string;
	}) => Promise<ThreadSubmitResult | void>;
	cancel: () => Promise<void>;
	connect: () => Promise<void>;
	disconnect: () => void;
	refresh: () => Promise<void>;
	addToolApprovalResponse: (payload: {
		id: string;
		approved: boolean;
		reason?: string;
	}) => void;
	deleteQueuedPrompt: (queueId: string) => Promise<void>;
	updateQueuedPrompt: (
		queueId: string,
		payload: { runAfter?: string; clearRunAfter?: boolean },
	) => Promise<void>;
	dispose: () => void;
	editorFiles: string[];
	fileContents: Record<string, string>;
};

export type SessionContextValue = {
	sessionId: string;
	isPending: boolean;
	current: Session | null;
	dispose: () => void;
	stores: SessionStores;
	ui: SessionViewState;
	threads: SessionThreadsService;
	hooks: SessionHooksService;
	files: SessionFilesDomain;
	services: SessionServicesDomain;
	commands: SessionCommandsDomain;
	threadContexts: Map<string, ThreadContextValue>;
	conversationScrollTopByThreadId: Map<string, number>;
};
