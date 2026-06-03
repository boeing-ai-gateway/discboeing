import type {
	AgentCommand,
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
	SessionDiffFileEntry,
	SessionDiffStats,
	StartupTask,
	SupportInfoResponse,
	Thread,
	Workspace,
} from "$lib/api-types";
import type { UpdateStatus } from "$lib/app/app-context.types";
import type { IdeOption } from "$lib/app/ide-options";
import type { RecentThreadEntry } from "$lib/app/thread-switcher";
import type { ContextView } from "$lib/context/context-view.types";
import type {
	DesktopRuntimeKind,
	WindowControlsSide,
} from "$lib/desktop/types";
import type { AsyncStatus } from "$lib/resource/types";

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

export type * from "$lib/context/context-view.types";

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
