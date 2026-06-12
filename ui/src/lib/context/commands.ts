import type {
	CommandOptions,
	Commands,
	Context,
} from "$lib/context/context.types";
import {
	closeCommandCredentialDialog,
	confirmCommandCredentialDialog,
	launchCommandCredentialOAuthWizard,
	refreshCommandCredentialDialogCredentials,
	runAgentCommand,
	selectCommandCredentialOption,
	setCommandCredentialCreateName,
	setCommandCredentialCreateSecret,
	setCommandCredentialValidityPreset,
	setCommandCredentialValidityUnit,
	setCommandCredentialValidityValue,
} from "$lib/context/domains/agent-commands";
import {
	anthropicAuthorize,
	anthropicExchange,
	codexAuthorize,
	codexCallbackStatus,
	codexDeviceCode,
	codexExchange,
	codexPoll,
	createCredential,
	deleteCredential,
	githubAuthorize,
	githubCallbackStatus,
	githubDeviceCode,
	githubExchange,
	githubPoll,
	loadCredentialsIntoCache,
	toggleCredentialInactive,
} from "$lib/context/domains/credentials";
import {
	deleteFile,
	loadFileSubtreeIntoCache,
	openFile,
	openFilesPanel,
	renameFile,
	saveFile,
	setDiffTarget,
} from "$lib/context/domains/files";
import {
	clearCredentialFlowIntent,
	closeKeyboardShortcutOverlays,
	closeSettingsDialog,
	closeSupportInfoDialog,
	clearCredentialsDialogTarget,
	openCredentialsDialog,
	openGitHubCredentialFlow,
	openSettingsDialog,
	openSupportInfoDialog,
	setRecentThreadSwitcherCommitModifier,
	setRecentThreadSwitcherOpen,
	setRecentThreadSwitcherSelectedKey,
	setKeyboardShortcutsOpen,
	setSettingsDialogOpen,
	setSettingsDialogTab,
	toggleKeyboardShortcutsOpen,
} from "$lib/context/domains/dialogs";
import { pauseHooks, pauseHook, rerunHook } from "$lib/context/domains/hooks";
import {
	openThread,
	selectSession,
	setDesktopSidebarOpen,
	setMobileSidebarOpen,
	startNewSession,
	toggleMobileSidebarOpen,
	toggleSelectedSessionView,
} from "$lib/context/domains/navigation";
import {
	addPromptToHistory,
	pinPrompt,
	removePromptFromHistory,
	setAutoScrollOnStream,
	setChatWidthMode,
	setColorScheme,
	setDefaultModel,
	setDefaultReasoning,
	setDefaultServiceTier,
	setDiffReviewApprovals,
	setDiffReviewStyle,
	setPreferredIde,
	setRecentThreadsVisibleLimit,
	setSidebarAllGroupedByWorkspace,
	setSidebarAllOpen,
	setSidebarRecentOpen,
	setShowRefreshButton,
	setTheme,
	setTopBarIconOnly,
	unpinPrompt,
} from "$lib/context/domains/preferences";
import {
	createSandboxProvider,
	deleteSandboxProvider,
	loadSandboxProvidersIntoCache,
	updateDefaultSandboxProvider,
	updateSandboxProvider,
} from "$lib/context/domains/sandbox-providers";
import {
	loadSessionCredentialsIntoCache,
	replaceSessionCredentialAssignments,
} from "$lib/context/domains/session-credentials";
import {
	activateSessionUsingProjectSocket,
	createSession,
	deactivateSession,
	deleteSessionWithThreadDeactivation,
	loadSessionIntoCache,
	renameSession,
	stopSession,
} from "$lib/context/domains/sessions";
import {
	bindServiceLocalhost,
	openServicePanel,
	startService,
	stopService,
	unbindServiceLocalhost,
} from "$lib/context/domains/services";
import { fetchSupportInfo } from "$lib/context/domains/support-info";
import {
	checkForUpdates,
	ignoreUpdate,
	installUpdateAndRelaunch,
	setTrackPrereleases,
} from "$lib/context/domains/updates";
import {
	activateThreadUsingProjectSocket,
	createThread,
	deactivateThread,
	deleteThreadWithDeactivation,
	renameThread,
	sendMessage,
	updateThread,
} from "$lib/context/domains/threads";
import {
	mountSessionView,
	mountThreadView,
	resetPendingWorkspaceSetup,
	setPendingWorkspaceSandboxProviderId,
	setSessionHooksExpanded,
} from "$lib/context/domains/view";
import {
	addThreadPendingComment,
	addToolApprovalResponse,
	cancelThread,
	clearComposerDraft,
	clearThreadNextComposerValues,
	clearThreadPendingComments,
	deleteQueuedPrompt,
	movePendingComposerDraftToThread,
	refreshThread as refreshRuntimeThread,
	removeThreadPendingComment,
	setComposerDraft,
	setConversationScrollTop,
	setThreadNextModelId,
	setThreadNextReasoning,
	setThreadNextServiceTier,
	submitThread,
	updateQueuedPrompt,
} from "$lib/context/domains/thread-composer";
import { activateProject } from "$lib/context/domains/projects";
import {
	deleteWorkspace,
	renameWorkspace,
} from "$lib/context/domains/workspaces";
import {
	logDebugCommandFinish,
	logDebugCommandStart,
} from "$lib/context/debug";
import { shutdown, startup } from "$lib/context/domains/lifecycle";

type DomainCommand<Args extends unknown[], Return> = (
	context: Context,
	...args: Args
) => Return | Promise<Awaited<Return>>;
type DomainCommandFor<T> = T extends (...args: infer Args) => infer Return
	? DomainCommand<Args, Return>
	: never;
type CommandRegistrationSpec<T> = {
	[Group in keyof T]: {
		[Command in keyof T[Group]]: DomainCommandFor<T[Group][Command]>;
	};
};

export function createCommands(context: Context): Commands {
	return register(context, {
		lifecycle: {
			startup,
			shutdown,
		},
		projects: {
			activateProject,
		},
		sessions: {
			activateSession: activateSessionUsingProjectSocket,
			deactivateSession,
			refreshSession: loadSessionIntoCache,
			createSession,
			renameSession,
			stopSession,
			deleteSession: deleteSessionWithThreadDeactivation,
		},
		threads: {
			activateThread: activateThreadUsingProjectSocket,
			deactivateThread,
			createThread,
			renameThread,
			updateThread,
			deleteThread: deleteThreadWithDeactivation,
			sendMessage,
		},
		view: {
			mountSessionView,
			mountThreadView,
			setSessionHooksExpanded,
			setPendingWorkspaceSandboxProviderId,
			resetPendingWorkspaceSetup,
		},
		navigation: {
			setDesktopSidebarOpen,
			setMobileSidebarOpen,
			toggleMobileSidebarOpen,
			startNewSession,
			selectSession,
			openThread,
			toggleSelectedSessionView,
		},
		dialogs: {
			setSettingsDialogOpen,
			setSettingsDialogTab,
			openSettingsDialog,
			closeSettingsDialog,
			openCredentialsDialog,
			openGitHubCredentialFlow,
			clearCredentialsDialogTarget,
			clearCredentialFlowIntent,
			openSupportInfoDialog,
			closeSupportInfoDialog,
			setKeyboardShortcutsOpen,
			toggleKeyboardShortcutsOpen,
			setRecentThreadSwitcherOpen,
			setRecentThreadSwitcherSelectedKey,
			setRecentThreadSwitcherCommitModifier,
			closeKeyboardShortcutOverlays,
		},
		supportInfo: {
			fetchSupportInfo,
		},
		preferences: {
			setPreferredIde,
			setTheme,
			setColorScheme,
			setRecentThreadsVisibleLimit,
			setShowRefreshButton,
			setTopBarIconOnly,
			setDefaultModel,
			setDefaultReasoning,
			setDefaultServiceTier,
			setChatWidthMode,
			setAutoScrollOnStream,
			setSidebarRecentOpen,
			setSidebarAllOpen,
			setSidebarAllGroupedByWorkspace,
			setDiffReviewApprovals,
			setDiffReviewStyle,
			addPromptToHistory,
			removePromptFromHistory,
			pinPrompt,
			unpinPrompt,
		},
		threadComposer: {
			setComposerDraft,
			clearComposerDraft,
			movePendingComposerDraftToThread,
			setThreadNextModelId,
			setThreadNextReasoning,
			setThreadNextServiceTier,
			clearThreadNextComposerValues,
			addThreadPendingComment,
			removeThreadPendingComment,
			clearThreadPendingComments,
			setConversationScrollTop,
			addToolApprovalResponse,
			refreshThread: refreshRuntimeThread,
			submitThread,
			cancelThread,
			deleteQueuedPrompt,
			updateQueuedPrompt,
		},
		agentCommands: {
			runAgentCommand,
			closeCommandCredentialDialog,
			confirmCommandCredentialDialog,
			selectCommandCredentialOption,
			setCommandCredentialCreateName,
			setCommandCredentialCreateSecret,
			setCommandCredentialValidityPreset,
			setCommandCredentialValidityValue,
			setCommandCredentialValidityUnit,
			launchCommandCredentialOAuthWizard,
			refreshCommandCredentialDialogCredentials,
		},
		credentials: {
			refreshCredentials: loadCredentialsIntoCache,
			createCredential,
			deleteCredential,
			toggleCredentialInactive,
			codexAuthorize,
			codexDeviceCode,
			codexCallbackStatus,
			codexPoll,
			codexExchange,
			githubAuthorize,
			githubDeviceCode,
			githubCallbackStatus,
			githubPoll,
			githubExchange,
			anthropicAuthorize,
			anthropicExchange,
		},
		sessionCredentials: {
			refreshSessionCredentials: loadSessionCredentialsIntoCache,
			setSessionCredentialAssignments: replaceSessionCredentialAssignments,
		},
		sandboxProviders: {
			refreshSandboxProviders: loadSandboxProvidersIntoCache,
			createSandboxProvider,
			updateSandboxProvider,
			deleteSandboxProvider,
			updateDefaultSandboxProvider,
		},
		workspaces: {
			renameWorkspace,
			deleteWorkspace,
		},
		files: {
			refreshFileSubtree: loadFileSubtreeIntoCache,
			openFile,
			openFilesPanel,
			setDiffTarget,
			saveFile,
			renameFile,
			deleteFile,
		},
		hooks: {
			rerunHook,
			pauseHooks,
			pauseHook,
		},
		services: {
			openServicePanel,
			startService,
			stopService,
			bindServiceLocalhost,
			unbindServiceLocalhost,
		},
		updates: {
			checkForUpdates,
			setTrackPrereleases,
			installUpdateAndRelaunch,
			ignoreUpdate,
		},
	} satisfies CommandRegistrationSpec<Commands>);
}
type RegisteredCommandGroups<T> = {
	[Group in keyof T]: {
		[Command in keyof T[Group]]: T[Group][Command] extends DomainCommand<
			infer Args,
			infer Return
		>
			? (...args: Args) => Promise<Awaited<Return>>
			: never;
	};
};

function register<T extends CommandRegistrationSpec<Commands>>(
	context: Context,
	commands: T,
): RegisteredCommandGroups<T> {
	const registered = {} as RegisteredCommandGroups<T>;

	for (const groupName in commands) {
		const group = commands[groupName];
		const registeredGroup = {} as RegisteredCommandGroups<T>[typeof groupName];

		for (const commandName in group) {
			const command = group[commandName] as DomainCommand<unknown[], unknown>;
			registeredGroup[commandName] = registerCommand(
				context,
				`${groupName}.${commandName}`,
				command,
			) as RegisteredCommandGroups<T>[typeof groupName][typeof commandName];
		}

		registered[groupName] = registeredGroup;
	}

	return registered;
}
function registerCommand<Args extends unknown[], Return>(
	context: Context,
	commandName: string,
	command: DomainCommand<Args, Return>,
): (...args: Args) => Promise<Awaited<Return>> {
	return async (...args: Args): Promise<Awaited<Return>> => {
		const debugCommandId = logDebugCommandStart(context, commandName, args);
		try {
			const result = await schedule(
				Promise.resolve(command(context, ...args)),
				getCommandOptions(command, args),
			);
			logDebugCommandFinish(context, debugCommandId, "success");
			return result;
		} catch (error) {
			logDebugCommandFinish(context, debugCommandId, "error", error);
			throw error;
		}
	};
}

function getCommandOptions(
	command: { length: number },
	args: unknown[],
): CommandOptions | undefined {
	const expectedArgCount = Math.max(command.length - 1, 0);
	if (args.length > expectedArgCount) {
		return args[expectedArgCount] as CommandOptions | undefined;
	}

	const lastArg = args.at(-1);
	if (isCommandOptions(lastArg)) {
		return lastArg;
	}
}

function isCommandOptions(value: unknown): value is CommandOptions {
	return typeof value === "object" && value !== null && "wait" in value;
}

async function schedule<T>(
	work: Promise<T>,
	options: CommandOptions | undefined,
): Promise<T> {
	if (options?.wait) {
		return await work;
	}
	return await work;
}
