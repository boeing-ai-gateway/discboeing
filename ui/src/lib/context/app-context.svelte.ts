import { getContext, hasContext, setContext } from "svelte";

import { api } from "$lib/api-client";
import type {
	AppChatRequest,
	AppContext,
	AppContextBootstrap,
	AppStores,
} from "$lib/app/app-context.types";
import { createAppCredentialsDomain } from "$lib/app/domains/app-credentials.svelte";
import { createAppEnvironmentDomain } from "$lib/app/domains/app-environment";
import { createAppModelsDomain } from "$lib/app/domains/app-models.svelte";
import { createAppPreferencesDomain } from "$lib/app/domains/app-preferences.svelte";
import { createAppSessionsDomain } from "$lib/app/domains/app-sessions.svelte";
import { createAppStartupStatusDomain } from "$lib/app/domains/app-startup-status.svelte";
import { createAppSupportInfoDomain } from "$lib/app/domains/app-support-info.svelte";
import { createAppUpdatesDomain } from "$lib/app/domains/app-updates.svelte";
import { createAppWorkspacesDomain } from "$lib/app/domains/app-workspaces.svelte";
import { createAppViewState } from "$lib/app/view/create-app-view-state.svelte";
import type { StartupTask } from "$lib/api-types";
import { createChatStreamManager } from "$lib/thread/chat-stream-manager";
import { SessionStore } from "$lib/store/sessions.store.svelte";
import { WorkspaceStore } from "$lib/store/workspaces.store.svelte";
import { ModelStore } from "$lib/store/models.store.svelte";
import { CredentialStore } from "$lib/store/credentials.store.svelte";
import { StartupTaskStore } from "$lib/store/startup-tasks.store.svelte";

export type {
	AppContext,
	AppContextBootstrap,
	AppCredential,
	ChatWidthMode,
	SettingsDialogTab,
	UpdateStatus,
} from "$lib/app/app-context.types";

const APP_CONTEXT_KEY = Symbol.for("discobot-ui-app-context");

type ProjectEvent<TData> = {
	id: string;
	type: string;
	timestamp: string;
	data: TData;
};

type SessionUpdatedEventData = {
	sessionId: string;
	status: string;
};

type ThreadUpdatedEventData = {
	sessionId: string;
	threadId: string;
	name?: string;
};

type WorkspaceUpdatedEventData = {
	workspaceId: string;
	status: string;
};

function startProjectEventsSubscription(app: AppContext) {
	const subscription = app.chatStreams.subscribeProjectEvents({
		onError: (error) => {
			console.error("[WS] Project events connection error:", error);
		},
	});

	const handleSessionUpdated = (event: MessageEvent<string>) => {
		try {
			const payload = JSON.parse(
				event.data,
			) as ProjectEvent<SessionUpdatedEventData>;
			const sessionData = payload.data;
			if (!sessionData?.sessionId) {
				return;
			}

			if (sessionData.status === "removed") {
				app.sessions.removeFromMemory(sessionData.sessionId);
				return;
			}

			void app.sessions.reloadSession(sessionData.sessionId);
		} catch (error) {
			console.error("[WS] Failed to parse session_updated event:", error);
		}
	};

	const handleConnected = () => {
		console.debug("[WS] Connected to project events stream");
		void app.sessions.refresh().catch((error) => {
			console.error(
				"[WS] Failed to refresh sessions after connecting to project events stream:",
				error,
			);
		});
		void app.startup.refresh().catch((error) => {
			console.error(
				"[WS] Failed to refresh startup tasks after connecting to project events stream:",
				error,
			);
		});
	};

	const handleWorkspaceUpdated = (event: MessageEvent<string>) => {
		try {
			const payload = JSON.parse(
				event.data,
			) as ProjectEvent<WorkspaceUpdatedEventData>;
			if (!payload.data?.workspaceId) {
				return;
			}

			void app.workspaces.reloadWorkspace(payload.data.workspaceId);
		} catch (error) {
			console.error("[WS] Failed to parse workspace_updated event:", error);
		}
	};

	const handleStartupTaskUpdated = (event: MessageEvent<string>) => {
		try {
			const payload = JSON.parse(event.data) as ProjectEvent<StartupTask>;
			if (!payload.data?.id) {
				return;
			}

			void app.stores.startup.fetch();
		} catch (error) {
			console.error("[WS] Failed to parse startup_task_updated event:", error);
		}
	};

	subscription.eventSource.addEventListener(
		"session_updated",
		handleSessionUpdated,
	);
	subscription.eventSource.addEventListener("connected", handleConnected);
	subscription.eventSource.addEventListener(
		"workspace_updated",
		handleWorkspaceUpdated,
	);
	subscription.eventSource.addEventListener(
		"startup_task_updated",
		handleStartupTaskUpdated,
	);

	return () => {
		subscription.eventSource.removeEventListener(
			"session_updated",
			handleSessionUpdated,
		);
		subscription.eventSource.removeEventListener("connected", handleConnected);
		subscription.eventSource.removeEventListener(
			"workspace_updated",
			handleWorkspaceUpdated,
		);
		subscription.eventSource.removeEventListener(
			"startup_task_updated",
			handleStartupTaskUpdated,
		);
		subscription.unsubscribe();
	};
}

function createAppContext(bootstrap: AppContextBootstrap): AppContext {
	const preferences = createAppPreferencesDomain({ bootstrap });
	const environment = createAppEnvironmentDomain({ bootstrap });
	const updates = createAppUpdatesDomain();

	const stores: AppStores = {
		sessions: new SessionStore(),
		workspaces: new WorkspaceStore(),
		models: new ModelStore(),
		credentials: new CredentialStore(),
		startup: new StartupTaskStore(),
	};

	const workspaces = createAppWorkspacesDomain({ store: stores.workspaces });
	const startup = createAppStartupStatusDomain({ store: stores.startup });
	const models = createAppModelsDomain({ store: stores.models });
	const supportInfo = createAppSupportInfoDomain();
	const chatStreams = createChatStreamManager();
	const credentials = createAppCredentialsDomain({
		store: stores.credentials,
		refreshModels: models.refresh,
	});
	const sessions = createAppSessionsDomain({
		store: stores.sessions,
		initialSelectedSessionId: bootstrap.selectedSessionId,
	});
	const ui = createAppViewState({ sessions, preferences });

	const refresh = async () => {
		await Promise.all([
			sessions.refresh(),
			workspaces.refresh(),
			startup.refresh(),
			models.refresh(),
		]);
	};

	let stopProjectEvents: (() => void) | null = null;

	const findWorkspaceBySourceAndPath = (
		path: string,
		sourceType: "local" | "git",
	) => {
		const normalizedPath = path.trim();
		if (!normalizedPath) {
			return null;
		}

		return (
			workspaces.list.find((workspace) => {
				return (
					workspace.sourceType === sourceType &&
					workspace.path.trim() === normalizedPath
				);
			}) ?? null
		);
	};

	const chat = async ({
		sessionId,
		threadId,
		workspaceId,
		workspaceType,
		workspacePath,
		...rest
	}: AppChatRequest) => {
		const resolvedSessionId = sessionId ?? sessions.pendingId;
		let resolvedWorkspaceId = workspaceId ?? undefined;

		if (
			!resolvedWorkspaceId &&
			resolvedSessionId === sessions.pendingId &&
			workspaceType &&
			workspacePath
		) {
			const normalizedWorkspacePath = workspacePath.trim();
			if (normalizedWorkspacePath) {
				let workspace = findWorkspaceBySourceAndPath(
					normalizedWorkspacePath,
					workspaceType,
				);
				if (!workspace) {
					await workspaces.refresh();
					workspace = findWorkspaceBySourceAndPath(
						normalizedWorkspacePath,
						workspaceType,
					);
				}

				if (!workspace) {
					workspace = await workspaces.create({
						path: normalizedWorkspacePath,
						sourceType: workspaceType,
					});
				}

				resolvedWorkspaceId = workspace.id;
			}
		}

		return api.startChat({
			...rest,
			sessionId: resolvedSessionId,
			threadId,
			...(resolvedWorkspaceId ? { workspaceId: resolvedWorkspaceId } : {}),
		});
	};

	const context: AppContext = {
		stores,
		ui,
		preferences,
		environment,
		sessions,
		workspaces,
		startup,
		models,
		credentials,
		supportInfo,
		chatStreams,
		chat,
		refresh,
		connectProjectEvents: () => {
			if (!stopProjectEvents) {
				stopProjectEvents = startProjectEventsSubscription(context);
			}
			return () => {
				stopProjectEvents?.();
				stopProjectEvents = null;
			};
		},
		updates,
	};

	return context;
}

export function setAppContext(bootstrap: AppContextBootstrap): AppContext {
	const context = createAppContext(bootstrap);
	setContext(APP_CONTEXT_KEY, context);
	return context;
}

export function useAppContext(): AppContext {
	const context = getContext<AppContext | undefined>(APP_CONTEXT_KEY);
	if (!context) {
		throw new Error("useAppContext must be used within AppContext provider");
	}
	return context;
}

export function getAppContextIfPresent(): AppContext | undefined {
	if (!hasContext(APP_CONTEXT_KEY)) {
		return undefined;
	}
	return getContext<AppContext | undefined>(APP_CONTEXT_KEY);
}
